package git

import (
	"context"
	"fmt"
	"strings"

	"patchflow/internal/execx"
)

type Client struct {
	Runner execx.Runner
}

func NewClient(r execx.Runner) Client {
	if r == nil {
		r = execx.CommandRunner{}
	}
	return Client{Runner: r}
}

func (c Client) TopLevel(ctx context.Context, dir string) (string, error) {
	if c.Runner == nil {
		return "", nil
	}
	out, err := c.Runner.Run(ctx, dir, "git", "rev-parse", "--show-toplevel")
	return strings.TrimSpace(out), err
}

func (c Client) StatusPorcelain(ctx context.Context, dir string) (string, error) {
	if c.Runner == nil {
		return "", nil
	}
	return c.Runner.Run(ctx, dir, "git", "status", "--porcelain")
}

func (c Client) RemoteList(ctx context.Context, dir string) ([]Remote, error) {
	if c.Runner == nil {
		return nil, nil
	}
	out, err := c.Runner.Run(ctx, dir, "git", "remote", "-v")
	if err != nil {
		return nil, err
	}
	return ParseRemotes(out), nil
}

func (c Client) Fetch(ctx context.Context, dir, remote string) error {
	if c.Runner == nil {
		return nil
	}
	_, err := c.Runner.Run(ctx, dir, "git", "fetch", remote, "--tags", "--prune")
	return err
}

func (c Client) LogRange(ctx context.Context, dir, base, target string) ([]Commit, error) {
	if c.Runner == nil {
		return nil, nil
	}
	spec := fmt.Sprintf("%s...%s", base, target)
	out, err := c.Runner.Run(ctx, dir, "git", "log", "--pretty=format:%H%x09%h%x09%an%x09%ad%x09%s", "--date=short", spec)
	if err != nil {
		return nil, err
	}
	return ParseLog(out), nil
}
