package git

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestLocalBranchesRemoteBranchesAndTags(t *testing.T) {
	r := fakeRunner{outputs: map[string]string{
		"git for-each-ref --format=%(refname:short) refs/heads":            "main\nfeature/login\n",
		"git for-each-ref --format=%(refname:short) refs/remotes/upstream": "upstream/HEAD\nupstream/main\nupstream/release/1.2.0\n",
		"git tag --list": "v1.2.0\nv1.2.1\n",
		"git tag --list v1.2.1": "v1.2.1\n",
		"git ls-remote --tags origin v1.2.1": "deadbeef\trefs/tags/v1.2.1\n",
		"git branch --list feature/login": "  feature/login\n",
	}}
	c := NewClient(r)

	heads, err := c.LocalBranches(context.Background(), "/tmp/repo")
	if err != nil {
		t.Fatalf("LocalBranches() error = %v", err)
	}
	if len(heads) != 2 || heads[0] != "main" || heads[1] != "feature/login" {
		t.Fatalf("heads = %#v", heads)
	}

	remotes, err := c.RemoteBranches(context.Background(), "/tmp/repo", "upstream")
	if err != nil {
		t.Fatalf("RemoteBranches() error = %v", err)
	}
	if len(remotes) != 2 || remotes[0] != "upstream/main" || remotes[1] != "upstream/release/1.2.0" {
		t.Fatalf("remotes = %#v", remotes)
	}

	tags, err := c.Tags(context.Background(), "/tmp/repo")
	if err != nil {
		t.Fatalf("Tags() error = %v", err)
	}
	if len(tags) != 2 || tags[0] != "v1.2.0" || tags[1] != "v1.2.1" {
		t.Fatalf("tags = %#v", tags)
	}

	localExists, err := c.TagExists(context.Background(), "/tmp/repo", "v1.2.1")
	if err != nil {
		t.Fatalf("TagExists() error = %v", err)
	}
	if !localExists {
		t.Fatal("TagExists() = false, want true")
	}

	remoteExists, err := c.RemoteTagExists(context.Background(), "/tmp/repo", "origin", "v1.2.1")
	if err != nil {
		t.Fatalf("RemoteTagExists() error = %v", err)
	}
	if !remoteExists {
		t.Fatal("RemoteTagExists() = false, want true")
	}

	branchExists, err := c.BranchExists(context.Background(), "/tmp/repo", "feature/login")
	if err != nil {
		t.Fatalf("BranchExists() error = %v", err)
	}
	if !branchExists {
		t.Fatal("BranchExists() = false, want true")
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
