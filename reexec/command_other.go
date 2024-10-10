//go:build freebsd || darwin || windows

package reexec

import (
	"context"
	"os/exec"
)

func Command(args ...string) *exec.Cmd {
	return CommandContext(context.Background(), args...)
}

// CommandContext returns *exec.Cmd with its Path set to the path of the current
// binary using the result of [Self]. For example if current binary is
// "my-binary" at "/usr/bin/" (or "my-binary.exe" at "C:\" on Windows),
// then cmd.Path is set to "/usr/bin/my-binary" and "C:\my-binary.exe"
// respectively.
func CommandContext(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "")
	cmd.Path = Self()
	cmd.Args = args
	return cmd
}
