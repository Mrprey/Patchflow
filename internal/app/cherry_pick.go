package app

import (
	"context"
	"runtime"
	"strings"

	"patchflow/internal/execx"
	"patchflow/internal/git"
)

type CherryPickEngine struct {
	Runner   execx.Runner
	WorkDir  string
	PostCmd  string
}

func (e CherryPickEngine) Run(ctx context.Context, commits []StateCommit) ([]StateCommit, error) {
	results := make([]StateCommit, len(commits))
	copy(results, commits)
	for i := range results {
		results[i].Status = "applying"
		if _, err := git.CherryPick(e.Runner, e.WorkDir, results[i].SHA); err != nil {
			results[i].Status = "failed"
			return results, err
		}
		results[i].Status = "applied"
		if strings.TrimSpace(e.PostCmd) != "" {
			if _, err := runShell(ctx, e.Runner, e.WorkDir, e.PostCmd); err != nil {
				return results, err
			}
		}
	}
	return results, nil
}

func runShell(ctx context.Context, runner execx.Runner, dir, command string) (string, error) {
	if runtime.GOOS == "windows" {
		return runner.Run(ctx, dir, "cmd", "/c", command)
	}
	return runner.Run(ctx, dir, "sh", "-c", command)
}
