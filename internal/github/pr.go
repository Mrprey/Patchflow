package github

import "context"

func (c Client) PullRequestURL(ctx context.Context, repo, sha string) (string, error) {
	pr, err := c.PullRequestForCommit(ctx, repo, sha)
	if err != nil || pr == nil {
		return "", err
	}
	return pr.URL, nil
}
