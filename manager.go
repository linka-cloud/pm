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
	"fmt"
	"sync"
	"time"

	"github.com/thejerf/suture/v4"
	"go.linka.cloud/grpc-toolkit/logger"
	pubsub "go.linka.cloud/pubsub/typed"
)

type Manager interface {
	Add(s Service) error
	Stop(s Service) error
	Run(ctx context.Context) error
	Status(...string) map[string]Status
	Watch(ctx context.Context, fn func(map[string]Status))
}

type manager struct {
	m     sync.Mutex
	procs map[string]*process
	s     *suture.Supervisor
	pub   pubsub.Publisher[map[string]Status]
}

func New(ctx context.Context, name string) Manager {
	var m *manager
	m = &manager{
		procs: make(map[string]*process),
		pub:   pubsub.NewPublisher[map[string]Status](time.Second, 1),
		s: suture.New(name, suture.Spec{
			FailureBackoff: 5 * time.Second,
			EventHook: func(event suture.Event) {
				switch e := event.(type) {
				case suture.EventResume:
					m.setStatus(e.SupervisorName, StatusRunning)
				case suture.EventBackoff:
					logger.C(ctx).WithFields("service", e.SupervisorName, "status", StatusCrashLoop).Warnf("%v", e)
					m.setStatus(e.SupervisorName, StatusCrashLoop)
				case suture.EventServiceTerminate:
					if e.Err == nil {
						m.setStatus(e.SupervisorName, StatusStopped)
					} else {
						m.setStatus(e.SupervisorName, StatusError)
					}
				case suture.EventServicePanic:
					logger.C(ctx).WithFields(
						"service", e.SupervisorName,
						"status", StatusError,
						"error", e.PanicMsg,
						"stacktrace", e.Stacktrace,
						"restarting", e.Restarting,
					).Warn("panic")
					m.setStatus(e.SupervisorName, StatusError)
				case suture.EventStopTimeout:
					m.setStatus(e.SupervisorName, StatusUnknown)
				}
			},
		}),
	}
	return m
}

func (m *manager) Run(ctx context.Context) error {
	return m.s.Serve(ctx)
}

func (m *manager) Add(s Service) error {
	s = &wrapper{s: s, m: m}
	m.m.Lock()
	defer m.m.Unlock()
	v, ok := m.procs[s.String()]
	if ok {
		if v.status.Load() == uint32(StatusRunning) {
			return fmt.Errorf("service %s already exists and running", s.String())
		}
		if err := m.s.Remove(v.tk); err != nil {
			return err
		}
	}
	m.procs[s.String()] = m.newProcess(s.String(), s)
	return nil
}

func (m *manager) Stop(s Service) error {
	m.m.Lock()
	defer m.m.Unlock()
	p, ok := m.procs[s.String()]
	if !ok {
		return fmt.Errorf("service %s does not exist", s.String())
	}
	return m.s.Remove(p.tk)
}

func (m *manager) Status(ss ...string) map[string]Status {
	m.m.Lock()
	defer m.m.Unlock()
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
	m.m.Lock()
	defer m.m.Unlock()
	m.doSetStatus(name, status)
}
