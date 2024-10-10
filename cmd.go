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
	"os"
	"os/exec"
	"sync"

	"go.linka.cloud/grpc-toolkit/logger"

	"go.linka.cloud/pm/reexec"
)

var ErrAlreadyRunning = errors.New("already running")

var _ Service = (*Cmd)(nil)

type Cmd struct {
	name   string
	bin    string
	args   []string
	cmd    *exec.Cmd
	o      cmdOpts
	reexec bool
	m      sync.RWMutex
}

func Command(name, bin string, args ...string) *Cmd {
	return &Cmd{name: name, bin: bin, args: args}
}

func ReExec(name string, args ...string) *Cmd {
	return &Cmd{name: name, args: args, reexec: true}
}

func (c *Cmd) WithOpts(opts ...CmdOpt) *Cmd {
	for _, opt := range opts {
		opt(&c.o)
	}
	return c
}

func (c *Cmd) Serve(ctx context.Context) error {
	c.m.RLock()
	if c.cmd != nil {
		c.m.RUnlock()
		return ErrAlreadyRunning
	}
	c.m.RUnlock()
	c.m.Lock()
	log := logger.C(ctx).Logger().WithField("service", c.String())
	Notify(ctx, StatusStarting)
	if c.reexec {
		c.cmd = reexec.Command(c.args...)
	} else {
		c.cmd = exec.CommandContext(ctx, c.bin, c.args...)
	}
	if c.o.stdin != nil {
		c.cmd.Stdin = c.o.stdin
	}
	if c.o.stdout == nil {
		c.cmd.Stdout = log.WriterLevel(logger.InfoLevel)
	} else {
		c.cmd.Stdout = c.o.stdout
	}
	if c.o.stderr == nil {
		c.cmd.Stderr = log.WriterLevel(logger.ErrorLevel)
	} else {
		c.cmd.Stderr = c.o.stderr
	}
	if c.o.env != nil {
		c.cmd.Env = c.o.env
	}
	if len(c.o.extraFiles) != 0 {
		c.cmd.ExtraFiles = c.o.extraFiles
	}
	if c.o.dir != "" {
		c.cmd.Dir = c.o.dir
	}
	if c.o.sysProcAttr != nil {
		c.cmd.SysProcAttr = c.o.sysProcAttr
	}
	if err := c.cmd.Start(); err != nil {
		c.m.Unlock()
		Notify(ctx, StatusError)
		return err
	}
	c.m.Unlock()
	defer func() {
		c.m.Lock()
		c.cmd = nil
		c.m.Unlock()
	}()
	Notify(ctx, StatusRunning)
	if err := c.cmd.Wait(); err != nil {
		log.WithError(err).Error("exited")
		Notify(ctx, StatusError)
		return err
	}
	Notify(ctx, StatusStopped)
	return nil
}

func (c *Cmd) Signal(sig os.Signal) error {
	c.m.Lock()
	defer c.m.Unlock()
	if c.cmd.Process == nil {
		return nil
	}
	return c.cmd.Process.Signal(sig)
}

func (c *Cmd) Pid() int {
	c.m.RLocker()
	defer c.m.RUnlock()
	if c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}

func (c *Cmd) String() string {
	return c.name
}
