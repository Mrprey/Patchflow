package app

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoadState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".patchflow", "state.json")
	want := State{
		CurrentStep: "cherry_pick",
		SelectedCommits: []StateCommit{{
			SHA:      "abc",
			ShortSHA: "abc",
			Title:    "Fix",
			Status:   "applied",
		}},
	}

	if err := SaveState(path, want); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	got, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if got.CurrentStep != want.CurrentStep {
		t.Fatalf("CurrentStep = %q, want %q", got.CurrentStep, want.CurrentStep)
	}
	if len(got.SelectedCommits) != 1 || got.SelectedCommits[0].SHA != "abc" {
		t.Fatalf("SelectedCommits = %#v", got.SelectedCommits)
	}
}
