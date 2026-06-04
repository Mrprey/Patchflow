package github

import (
	"context"
	"encoding/json"

	"patchflow/internal/execx"
)

type Client struct {
	Runner execx.Runner
}

type PullRequest struct {
	Number int
	Title  string
	URL    string
	Labels []string
}

func NewClient(r execx.Runner) Client {
	if r == nil {
		r = execx.CommandRunner{}
	}
	return Client{Runner: r}
}

func (c Client) AuthStatus(ctx context.Context) error {
	_, err := c.Runner.Run(ctx, "", "gh", "auth", "status")
	return err
}

func (c Client) PullRequestForCommit(ctx context.Context, dir, sha string) (*PullRequest, error) {
	out, err := c.Runner.Run(ctx, dir, "gh", "pr", "list", "--state", "all", "--search", sha, "--json", "number,title,url,labels")
	if err != nil {
		return nil, err
	}
	var prs []struct {
		Number int `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"url"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return nil, err
	}
	if len(prs) == 0 {
		return nil, nil
	}
	pr := &PullRequest{Number: prs[0].Number, Title: prs[0].Title, URL: prs[0].URL}
	for _, label := range prs[0].Labels {
		pr.Labels = append(pr.Labels, label.Name)
	}
	return pr, nil
}

func (c Client) ReleaseCreate(ctx context.Context, repo, tag, target, title, notesFile string) error {
	_, err := c.Runner.Run(ctx, "", "gh", "release", "create", tag, "--repo", repo, "--target", target, "--title", title, "--notes-file", notesFile, "--draft")
	return err
}
