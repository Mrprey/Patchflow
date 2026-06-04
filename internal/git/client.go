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

func (c Client) LocalBranches(ctx context.Context, dir string) ([]string, error) {
	if c.Runner == nil {
		return nil, nil
	}
	out, err := c.Runner.Run(ctx, dir, "git", "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	return ParseRefList(out), nil
}

func (c Client) RemoteBranches(ctx context.Context, dir, remote string) ([]string, error) {
	if c.Runner == nil {
		return nil, nil
	}
	out, err := c.Runner.Run(ctx, dir, "git", "for-each-ref", "--format=%(refname:short)", "refs/remotes/"+remote)
	if err != nil {
		return nil, err
	}
	refs := ParseRefList(out)
	filtered := make([]string, 0, len(refs))
	for _, ref := range refs {
		if strings.HasSuffix(ref, "/HEAD") {
			continue
		}
		filtered = append(filtered, ref)
	}
	return filtered, nil
}

func (c Client) Tags(ctx context.Context, dir string) ([]string, error) {
	if c.Runner == nil {
		return nil, nil
	}
	out, err := c.Runner.Run(ctx, dir, "git", "tag", "--list")
	if err != nil {
		return nil, err
	}
	return ParseRefList(out), nil
}

func (c Client) TagExists(ctx context.Context, dir, tag string) (bool, error) {
	if c.Runner == nil {
		return false, nil
	}
	out, err := c.Runner.Run(ctx, dir, "git", "tag", "--list", tag)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (c Client) RemoteTagExists(ctx context.Context, dir, remote, tag string) (bool, error) {
	if c.Runner == nil {
		return false, nil
	}
	out, err := c.Runner.Run(ctx, dir, "git", "ls-remote", "--tags", remote, tag)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (c Client) BranchExists(ctx context.Context, dir, branch string) (bool, error) {
	if c.Runner == nil {
		return false, nil
	}
	out, err := c.Runner.Run(ctx, dir, "git", "branch", "--list", branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func ParseRefList(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	refs := make([]string, 0, len(lines))
	seen := make(map[string]struct{})
	for _, line := range lines {
		ref := strings.TrimSpace(strings.TrimPrefix(line, "* "))
		if ref == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}
	return refs
}
