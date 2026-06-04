package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"patchflow/internal/git"
)

func TestBuildNotesIncludesPRAndLabels(t *testing.T) {
	got := BuildNotes([]git.Commit{{
		ShortSHA: "abc1234",
		Title:    "Fix login crash",
		PRNumber: 123,
		Labels:   []string{"bug", "mobile"},
	}})
	if !strings.Contains(got, "Fix login crash (#123)") {
		t.Fatalf("notes = %q", got)
	}
	if !strings.Contains(got, "[bug, mobile]") {
		t.Fatalf("notes = %q", got)
	}
}

func TestWriteNotesCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".patchflow", "release-notes.md")
	if err := WriteNotes(path, "hello"); err != nil {
		t.Fatalf("WriteNotes() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("content = %q", data)
	}
}
