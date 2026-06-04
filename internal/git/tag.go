package git

import (
	"context"

	"patchflow/internal/execx"
)

func CreateAnnotatedTag(r execx.Runner, dir, tag, message string) (string, error) {
	return r.Run(context.Background(), dir, "git", "tag", "-a", tag, "-m", message)
}

func PushTag(r execx.Runner, dir, remote, tag string) (string, error) {
	return r.Run(context.Background(), dir, "git", "push", remote, tag)
}
