package reexec

import (
	"context"
	"os/exec"
	"syscall"
)

func Command(args ...string) *exec.Cmd {
	return CommandContext(context.Background(), args...)
}

// CommandContext returns an [*exec.Cmd] which has Path as current binary which,
// on Linux, is set to the in-memory version (/proc/self/exe) of the current
// binary, it is thus safe to delete or replace the on-disk binary (os.Args[0]).
//
// On Linux, the Pdeathsig of [*exec.Cmd.SysProcAttr] is set to SIGTERM.
// This signal will be sent to the process when the OS thread which created
// the process dies.
//
// It is the caller's responsibility to ensure that the creating thread is
// not terminated prematurely. See https://go.dev/issue/27505 for more details.
func CommandContext(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "")
	cmd.Path = Self()
	cmd.Args = args
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}
	return cmd
}
