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
	"os"
	"os/exec"

	"go.linka.cloud/grpc-toolkit/logger"
)

var _ Service = (*Cmd)(nil)

type Cmd struct {
	name string
	bin  string
	args []string
	cmd  *exec.Cmd
	o    cmdOpts
}

func NewCmd(name, bin string, args ...string) *Cmd {
	return &Cmd{name: name, bin: bin, args: args}
}

func (c *Cmd) WithOpts(opts ...CmdOpt) *Cmd {
	for _, opt := range opts {
		opt(&c.o)
	}
	return c
}

func (c *Cmd) Serve(ctx context.Context) error {
	log := logger.C(ctx).Logger().WithField("service", c.String())
	Notify(ctx, StatusStarting)
	c.cmd = exec.CommandContext(ctx, c.bin, c.args...)
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
	if err := c.cmd.Start(); err != nil {
		Notify(ctx, StatusError)
		return err
	}
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
	return c.cmd.Process.Signal(sig)
}

func (c *Cmd) String() string {
	return c.name
}
