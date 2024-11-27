// Copyright 2024 Linka Cloud  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pm

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/thejerf/suture/v4"
	"go.linka.cloud/grpc-toolkit/logger"
	pubsub "go.linka.cloud/pubsub/typed"
	"go.uber.org/multierr"
	"golang.org/x/sync/errgroup"
)

var (
	ErrAlreadyRunning = errors.New("already running")
	ErrNotExist       = errors.New("service does not exist")
)

type Manager interface {
	Add(s Service) error
	AddFunc(name string, f ServiceFunc) error
	Stop(s NamedService) error
	Run(ctx context.Context) error
	RunBackground(ctx context.Context) <-chan error
	Status(...string) map[string]Status
	Watch(ctx context.Context, fn func(map[string]Status))
	Close() error
}

type manager struct {
	name    string
	ctx     context.Context
	m       sync.Mutex
	rctx    context.Context
	rcancel context.CancelFunc
	g       *errgroup.Group
	procs   map[string]*process
	pm      sync.Mutex
	s       *suture.Supervisor
	pub     pubsub.Publisher[map[string]Status]
}

func New(ctx context.Context, name string) Manager {
	return (&manager{
		name:  name,
		ctx:   ctx,
		procs: make(map[string]*process),
		pub:   pubsub.NewPublisher[map[string]Status](time.Second, 1),
	}).setupSupervisor()
}

func (m *manager) setupSupervisor() *manager {
	m.s = suture.New(m.name, suture.Spec{
		FailureBackoff: 5 * time.Second,
		EventHook: func(event suture.Event) {
			switch e := event.(type) {
			case suture.EventResume:
				m.setStatus(e.SupervisorName, StatusRunning)
			case suture.EventBackoff:
				logger.C(m.ctx).WithFields("service", e.SupervisorName, "status", StatusCrashLoop).Warnf("%v", e)
				m.setStatus(e.SupervisorName, StatusCrashLoop)
			case suture.EventServiceTerminate:
				if e.Err == nil {
					m.setStatus(e.SupervisorName, StatusStopped)
				} else {
					m.setStatus(e.SupervisorName, StatusError)
				}
			case suture.EventServicePanic:
				logger.C(m.ctx).WithFields(
					"service", e.SupervisorName,
					"status", StatusError,
					"error", e.PanicMsg,
					"stacktrace", e.Stacktrace,
					"restarting", e.Restarting,
				).Error("panic")
				m.setStatus(e.SupervisorName, StatusError)
			case suture.EventStopTimeout:
				m.setStatus(e.SupervisorName, StatusUnknown)
			}
		},
	})
	return m
}

func (m *manager) Run(ctx context.Context) error {
	m.m.Lock()
	if m.rcancel != nil {
		m.m.Unlock()
		return ErrAlreadyRunning
	}
	ctx, cancel := context.WithCancel(ctx)
	g, ctx := errgroup.WithContext(ctx)
	m.rctx, m.rcancel, m.g = ctx, cancel, g
	m.m.Unlock()
	g.Go(func() error {
		return m.s.Serve(ctx)
	})
	return g.Wait()
}

func (m *manager) RunBackground(ctx context.Context) <-chan error {
	errs := make(chan error, 1)
	go func() {
		errs <- m.Run(ctx)
		close(errs)
	}()
	return errs
}

func (m *manager) Add(s Service) error {
	s = &wrapper{s: s, m: m}
	m.pm.Lock()
	defer m.pm.Unlock()
	v, ok := m.procs[s.String()]
	if ok {
		if v.status.Load() == uint32(StatusRunning) {
			return fmt.Errorf("%s: %w", s.String(), ErrAlreadyRunning)
		}
		if err := m.s.Remove(v.tk); err != nil {
			return err
		}
	}
	m.procs[s.String()] = m.newProcess(s.String(), s)
	return nil
}

func (m *manager) AddFunc(name string, f ServiceFunc) error {
	return m.Add(NewServiceFunc(name, f))
}

func (m *manager) Stop(s NamedService) error {
	m.pm.Lock()
	defer m.pm.Unlock()
	p, ok := m.procs[s.String()]
	if !ok {
		return fmt.Errorf("%s: %w", s.String(), ErrNotExist)
	}
	return m.s.Remove(p.tk)
}

func (m *manager) Status(ss ...string) map[string]Status {
	m.pm.Lock()
	defer m.pm.Unlock()
	status := make(map[string]Status, len(m.procs))
	if len(ss) == 0 {
		for k, v := range m.procs {
			status[k] = v.Status()
		}
		return status
	}
	for _, s := range ss {
		if p, ok := m.procs[s]; ok {
			status[s] = p.Status()
		}
	}
	return status
}

func (m *manager) Watch(ctx context.Context, fn func(map[string]Status)) {
	ch := m.pub.Subscribe()
	go func() {
		defer m.pub.Evict(ch)
		for {
			select {
			case s := <-ch:
				fn(s)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (m *manager) Close() error {
	m.m.Lock()
	defer m.m.Unlock()
	if m.rcancel == nil {
		return nil
	}
	m.rcancel()
	defer func() {
		m.rctx, m.rcancel, m.g = nil, nil, nil
	}()
	if err := m.g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	g := errgroup.Group{}
	for _, v := range m.procs {
		v := v
		g.Go(func() error {
			return m.s.Remove(v.tk)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	var err error
	m.pm.Lock()
	defer m.pm.Unlock()
	for _, v := range m.procs {
		err = multierr.Append(err, m.s.Remove(v.tk))
	}
	// clean up
	m.procs = make(map[string]*process)
	// we need to re-setup the supervisor to prevent its liveness channel from being double closed
	m.setupSupervisor()
	return err
}

func (m *manager) newProcess(name string, s Service) *process {
	p := &process{
		sup: suture.NewSimple(name),
	}
	p.sup.Add(s)
	p.tk = m.s.Add(p.sup)
	return p
}

func (m *manager) doSetStatus(name string, status Status) {
	if p, ok := m.procs[name]; ok {
		if p.Status() == status {
			return
		}
		if p.Status() == StatusCrashLoop && status == StatusError {
			return
		}
		p.setStatus(status)
	}
	c := make(map[string]Status, len(m.procs))
	for k, v := range m.procs {
		c[k] = v.Status()
	}
	m.pub.Publish(c)
}

func (m *manager) setStatus(name string, status Status) {
	m.pm.Lock()
	defer m.pm.Unlock()
	m.doSetStatus(name, status)
}
