package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"path/filepath"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewReturnsDoctorCommand(t *testing.T) {
	a := New(Options{})
	if a == nil {
		t.Fatal("New() returned nil")
	}
}

func TestRunDoctorReturnsRepoErrorWhenNotGitRepo(t *testing.T) {
	a := New(Options{Runner: fakeRunner{errs: map[string]error{
		"git rev-parse --show-toplevel": errors.New("fatal: not a git repository"),
	}}})

	err := a.runDoctor(context.Background())
	if err == nil || err.Error() != "This directory is not a Git repository." {
		t.Fatalf("runDoctor() error = %v", err)
	}
}

func TestRunDoctorReturnsDirtyTreeError(t *testing.T) {
	a := New(Options{Runner: fakeRunner{outputs: map[string]string{
		"git rev-parse --show-toplevel": "/tmp/repo",
		"git status --porcelain":        " M file.txt",
	}}})

	err := a.runDoctor(context.Background())
	if err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("runDoctor() error = %v", err)
	}
}

func TestRunDoctorPassesCleanRepo(t *testing.T) {
	a := New(Options{Runner: fakeRunner{outputs: map[string]string{
		"git rev-parse --show-toplevel": "/tmp/repo",
		"git status --porcelain":        "",
		"gh auth status":                 "",
	}}})

	if err := a.runDoctor(context.Background()); err != nil {
		t.Fatalf("runDoctor() error = %v", err)
	}
}

func TestRunTUIUsesInjectedFactory(t *testing.T) {
	called := false
	a := New(Options{})
	a.newProgram = func(model tea.Model, _ ...tea.ProgramOption) teaProgram {
		called = true
		if got, ok := model.(interface{ ScreenName() string }); !ok || got.ScreenName() != "welcome" {
			t.Fatalf("model = %#v", model)
		}
		return fakeProgram{}
	}

	if err := a.runTUI(); err != nil {
		t.Fatalf("runTUI() error = %v", err)
	}
	if !called {
		t.Fatal("factory not called")
	}
}

func TestResumePrintsCurrentStep(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, ".patchflow", "state.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(statePath, []byte(`{"currentStep":"versioning"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var out strings.Builder
	a := New(Options{WorkDir: dir, Stdout: &out})
	if err := a.resume(); err != nil {
		t.Fatalf("resume() error = %v", err)
	}
	if !strings.Contains(out.String(), "versioning") {
		t.Fatalf("output = %q", out.String())
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

type fakeProgram struct{}

func (fakeProgram) Run() (tea.Model, error) { return nil, nil }
