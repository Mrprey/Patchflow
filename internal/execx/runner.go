package execx

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) (string, error)
}

type CommandRunner struct{}

func (CommandRunner) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return buf.String(), fmt.Errorf("%s %v: %w: %s", name, args, err, buf.String())
	}
	return buf.String(), nil
}
