package github

import "context"

func (c Client) CreateDraftRelease(ctx context.Context, repo, tag, target, title, notesFile string) error {
	return c.ReleaseCreate(ctx, repo, tag, target, title, notesFile)
}
