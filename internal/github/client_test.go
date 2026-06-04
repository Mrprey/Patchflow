package github

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPullRequestForCommitParsesGhJson(t *testing.T) {
	c := NewClient(fakeRunner{outputs: map[string]string{
		"gh pr list --state all --search abc123 --json number,title,url,labels": `[{"number":123,"title":"Fix","url":"https://example.com","labels":[{"name":"bug"},{"name":"mobile"}]}]`,
	}})

	pr, err := c.PullRequestForCommit(context.Background(), "/tmp/repo", "abc123")
	if err != nil {
		t.Fatalf("PullRequestForCommit() error = %v", err)
	}
	if pr == nil || pr.Number != 123 || len(pr.Labels) != 2 {
		t.Fatalf("PullRequestForCommit() = %#v", pr)
	}
}

func TestPullRequestForCommitReturnsNilWhenNoMatch(t *testing.T) {
	c := NewClient(fakeRunner{outputs: map[string]string{
		"gh pr list --state all --search abc123 --json number,title,url,labels": `[]`,
	}})

	pr, err := c.PullRequestForCommit(context.Background(), "/tmp/repo", "abc123")
	if err != nil {
		t.Fatalf("PullRequestForCommit() error = %v", err)
	}
	if pr != nil {
		t.Fatalf("PullRequestForCommit() = %#v, want nil", pr)
	}
}

func TestAuthStatusFailsOnError(t *testing.T) {
	c := NewClient(fakeRunner{errs: map[string]error{
		"gh auth status": errors.New("not authenticated"),
	}})
	if err := c.AuthStatus(context.Background()); err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("AuthStatus() error = %v", err)
	}
}

type fakeRunner struct {
	outputs map[string]string
	errs    map[string]error
}

func (f fakeRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	key := strings.TrimSpace(name + " " + strings.Join(args, " "))
	if err, ok := f.errs[key]; ok {
		return f.outputs[key], err
	}
	if out, ok := f.outputs[key]; ok {
		return out, nil
	}
	return "", errors.New("unexpected command: " + key)
}
