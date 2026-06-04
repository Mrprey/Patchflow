package app

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCherryPickEngineRunsCommitsInOrder(t *testing.T) {
	runner := sequenceRunner{
		outputs: []string{
			"git cherry-pick abc",
			"git cherry-pick def",
		},
	}
	engine := CherryPickEngine{Runner: &runner, WorkDir: "/tmp/repo"}
	got, err := engine.Run(context.Background(), []StateCommit{
		{SHA: "abc"},
		{SHA: "def"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got[0].Status != "applied" || got[1].Status != "applied" {
		t.Fatalf("statuses = %#v", got)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestCherryPickEngineStopsOnError(t *testing.T) {
	runner := sequenceRunner{
		outputs: []string{
			"git cherry-pick abc",
		},
		errs: map[string]error{
			"git cherry-pick def": errors.New("conflict"),
		},
	}
	engine := CherryPickEngine{Runner: &runner, WorkDir: "/tmp/repo"}
	got, err := engine.Run(context.Background(), []StateCommit{
		{SHA: "abc"},
		{SHA: "def"},
	})
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("Run() error = %v", err)
	}
	if got[0].Status != "applied" || got[1].Status != "failed" {
		t.Fatalf("statuses = %#v", got)
	}
}

type sequenceRunner struct {
	outputs []string
	errs    map[string]error
	calls   []string
}

func (r *sequenceRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	key := strings.TrimSpace(name + " " + strings.Join(args, " "))
	r.calls = append(r.calls, key)
	if err, ok := r.errs[key]; ok {
		return "", err
	}
	if len(r.outputs) > 0 {
		out := r.outputs[0]
		r.outputs = r.outputs[1:]
		if out != key {
			return "", errors.New("unexpected command: " + key)
		}
	}
	return "", nil
}
