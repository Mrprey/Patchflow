package git

import (
	"context"

	"patchflow/internal/execx"
)

func CherryPick(r execx.Runner, dir, sha string) (string, error) {
	return r.Run(context.Background(), dir, "git", "cherry-pick", sha)
}

func CherryPickContinue(r execx.Runner, dir string) (string, error) {
	return r.Run(context.Background(), dir, "git", "cherry-pick", "--continue")
}

func CherryPickSkip(r execx.Runner, dir string) (string, error) {
	return r.Run(context.Background(), dir, "git", "cherry-pick", "--skip")
}

func CherryPickAbort(r execx.Runner, dir string) (string, error) {
	return r.Run(context.Background(), dir, "git", "cherry-pick", "--abort")
}
