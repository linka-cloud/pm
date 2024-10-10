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
	"io"
	"os"
	"syscall"
)

type cmdOpts struct {
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer
	env         []string
	extraFiles  []*os.File
	dir         string
	sysProcAttr *syscall.SysProcAttr
}

type CmdOpt func(o *cmdOpts)

func WithStdin(r io.Reader) CmdOpt {
	return func(o *cmdOpts) {
		o.stdin = r
	}
}

func WithStdout(w io.Writer) CmdOpt {
	return func(o *cmdOpts) {
		o.stdout = w
	}
}

func WithStderr(w io.Writer) CmdOpt {
	return func(o *cmdOpts) {
		o.stderr = w
	}
}

func WithEnv(env []string) CmdOpt {
	return func(o *cmdOpts) {
		o.env = env
	}
}

func WithDir(dir string) CmdOpt {
	return func(o *cmdOpts) {
		o.dir = dir
	}
}

func WithExtraFiles(files []*os.File) CmdOpt {
	return func(o *cmdOpts) {
		o.extraFiles = files
	}
}

func WithSysProcAttr(attr *syscall.SysProcAttr) CmdOpt {
	return func(o *cmdOpts) {
		o.sysProcAttr = attr
	}
}
