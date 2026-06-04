package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"patchflow/internal/config"
	"patchflow/internal/github"
	"patchflow/internal/git"
)

func TestNewSetsWelcomeScreen(t *testing.T) {
	m := New(Options{})
	if m.ScreenName() != "welcome" {
		t.Fatalf("ScreenName() = %q, want %q", m.ScreenName(), "welcome")
	}
}

func TestNewKeepsInjectedContext(t *testing.T) {
	m := New(Options{WorkDir: "/tmp/repo", ConfigPath: ".patchflow.yml"})
	if m.WorkDir != "/tmp/repo" {
		t.Fatalf("WorkDir = %q", m.WorkDir)
	}
	if m.ConfigPath != ".patchflow.yml" {
		t.Fatalf("ConfigPath = %q", m.ConfigPath)
	}
}

func TestEnterMovesWelcomeToRemoteSelect(t *testing.T) {
	m := New(Options{Config: configDefaultForTest()})
	next, _ := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "remote_select" {
		t.Fatalf("ScreenName() = %q, want remote_select", got.ScreenName())
	}
}

func TestViewShowsWelcomeAndRemoteSelect(t *testing.T) {
	m := New(Options{Config: configDefaultForTest()})
	if view := m.View(); view != "Patchflow\n\nPress enter to start.\n" {
		t.Fatalf("View() = %q", view)
	}
	m.screen = "remote_select"
	if view := m.View(); view != "Patchflow\n\nSelect remote:\n\n> upstream\n" {
		t.Fatalf("View() = %q", view)
	}
}

func TestEnterLoadsRemotesIntoRemoteSelect(t *testing.T) {
	m := New(Options{
		Config: configDefaultForTest(),
		Git: git.NewClient(fakeRunner{outputs: map[string]string{
			"git remote -v": "origin\tgit@github.com:a.git (fetch)\nupstream\tgit@github.com:b.git (fetch)\n",
		}}),
	})

	next, _ := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "remote_select" {
		t.Fatalf("ScreenName() = %q, want remote_select", got.ScreenName())
	}
	if len(got.Remotes) != 2 || got.Remotes[0].Name != "origin" || got.Remotes[1].Name != "upstream" {
		t.Fatalf("Remotes = %#v", got.Remotes)
	}
	if view := got.View(); !strings.Contains(view, "origin") || !strings.Contains(view, "upstream") {
		t.Fatalf("View() = %q", view)
	}
}

func TestEnterShowsErrorWhenRemoteListFails(t *testing.T) {
	m := New(Options{
		Config: configDefaultForTest(),
		Git: git.NewClient(fakeRunner{errs: map[string]error{
			"git remote -v": errors.New("boom"),
		}}),
	})

	next, _ := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.Err == nil || !strings.Contains(got.Err.Error(), "boom") {
		t.Fatalf("Err = %v", got.Err)
	}
}

func TestRemoteSelectMovesFocusAndFetchesSelectedRemote(t *testing.T) {
	runner := &recordingRunner{outputs: map[string]string{
		"git fetch upstream --tags --prune": "",
	}}
	m := New(Options{
		WorkDir: "/tmp/repo",
		Config:  configDefaultForTest(),
		Git:     git.NewClient(runner),
	})
	m.screen = "remote_select"
	m.Remotes = []git.Remote{{Name: "origin"}, {Name: "upstream"}}

	next, _ := m.Update(keyMsgDown())
	got := next.(Model)
	if got.RemoteIndex != 1 {
		t.Fatalf("RemoteIndex = %d, want 1", got.RemoteIndex)
	}

	next, _ = got.Update(keyMsgEnter())
	got = next.(Model)
	if got.ScreenName() != "ref_select" {
		t.Fatalf("ScreenName() = %q, want ref_select", got.ScreenName())
	}
	if len(runner.calls) != 1 || runner.calls[0] != "git fetch upstream --tags --prune" {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestRefSelectShowsAvailableRefs(t *testing.T) {
	m := New(Options{
		Config: configDefaultForTest(),
	})
	m.screen = "ref_select"
	m.Remotes = []git.Remote{{Name: "upstream"}}
	m.RefOptions = []string{"upstream/master", "upstream/main", "v1.2.0"}
	if view := m.View(); !strings.Contains(view, "upstream/master") || !strings.Contains(view, "v1.2.0") {
		t.Fatalf("View() = %q", view)
	}
}

func TestRefSelectMovesAndChoosesBase(t *testing.T) {
	m := New(Options{Config: configDefaultForTest()})
	m.screen = "ref_select"
	m.RefOptions = []string{"upstream/master", "upstream/main", "v1.2.0"}

	next, _ := m.Update(keyMsgDown())
	got := next.(Model)
	if got.RefIndex != 1 {
		t.Fatalf("RefIndex = %d, want 1", got.RefIndex)
	}

	next, _ = got.Update(keyMsgEnter())
	got = next.(Model)
	if got.CompareFrom != "upstream/main" {
		t.Fatalf("CompareFrom = %q, want upstream/main", got.CompareFrom)
	}
	if got.ScreenName() != "commit_select" {
		t.Fatalf("ScreenName() = %q, want commit_select", got.ScreenName())
	}
}

func TestRefSelectLoadsCommitsIntoCommitSelect(t *testing.T) {
	runner := &recordingRunner{outputs: map[string]string{
		"git log --pretty=format:%H%x09%h%x09%an%x09%ad%x09%s --date=short upstream/main...HEAD": "sha1\tshort1\tAna\t2024-01-01\tFix login\nsha2\tshort2\tBob\t2024-01-02\tUpdate docs\n",
	}}
	m := New(Options{
		WorkDir: "/tmp/repo",
		Config:  configDefaultForTest(),
		Git:     git.NewClient(runner),
	})
	m.screen = "ref_select"
	m.RefOptions = []string{"upstream/main"}
	m.RefIndex = 0
	m.CompareFrom = "upstream/main"

	next, _ := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "commit_select" {
		t.Fatalf("ScreenName() = %q, want commit_select", got.ScreenName())
	}
	if len(got.Commits) != 2 || got.Commits[0].Title != "Fix login" || got.Commits[1].ShortSHA != "short2" {
		t.Fatalf("Commits = %#v", got.Commits)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestRefSelectEnrichesCommitsWithPRData(t *testing.T) {
	ghRunner := &recordingRunner{outputs: map[string]string{
		"gh pr list --state all --search sha1 --json number,title,url,labels": `[{"number":123,"title":"Fix login","url":"https://github.com/owner/repo/pull/123","labels":[{"name":"bug"},{"name":"mobile"}]}]`,
	}}
	m := New(Options{
		WorkDir: "/tmp/repo",
		Config:  configDefaultForTest(),
		Git: git.NewClient(&recordingRunner{outputs: map[string]string{
			"git log --pretty=format:%H%x09%h%x09%an%x09%ad%x09%s --date=short upstream/main...HEAD": "sha1\tshort1\tAna\t2024-01-01\tFix login\n",
		}}),
		GitHub: github.NewClient(ghRunner),
	})
	m.screen = "ref_select"
	m.RefOptions = []string{"upstream/main"}
	m.CompareFrom = "upstream/main"

	next, _ := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.Commits[0].PRNumber != 123 || got.Commits[0].PRURL == "" || len(got.Commits[0].Labels) != 2 {
		t.Fatalf("Commits = %#v", got.Commits[0])
	}
}

func TestCommitSelectShowsPRMetadata(t *testing.T) {
	m := New(Options{Config: configDefaultForTest()})
	m.screen = "commit_select"
	m.Commits = []git.Commit{{
		ShortSHA: "short1",
		Title:    "Fix login",
		PRNumber: 123,
		Labels:   []string{"bug", "mobile"},
		PRURL:    "https://github.com/owner/repo/pull/123",
	}}

	view := m.View()
	if !strings.Contains(view, "#123") || !strings.Contains(view, "bug, mobile") || !strings.Contains(view, "↗") {
		t.Fatalf("View() = %q", view)
	}
}

func TestCommitSelectTogglesAndBulkSelects(t *testing.T) {
	m := New(Options{Config: configDefaultForTest()})
	m.screen = "commit_select"
	m.Commits = []git.Commit{
		{ShortSHA: "short1", Title: "Fix login"},
		{ShortSHA: "short2", Title: "Update docs"},
	}

	next, _ := m.Update(keyMsgSpace())
	got := next.(Model)
	if !got.Commits[0].Selected || got.Commits[1].Selected {
		t.Fatalf("Commits = %#v", got.Commits)
	}

	next, _ = got.Update(keyMsgCtrlA())
	got = next.(Model)
	if !got.Commits[0].Selected || !got.Commits[1].Selected {
		t.Fatalf("Commits = %#v", got.Commits)
	}

	next, _ = got.Update(keyMsgCtrlD())
	got = next.(Model)
	if got.Commits[0].Selected || got.Commits[1].Selected {
		t.Fatalf("Commits = %#v", got.Commits)
	}
}

func TestCommitSelectEnterAdvancesToCheckoutSelect(t *testing.T) {
	m := New(Options{Config: configDefaultForTest()})
	m.screen = "commit_select"
	m.Commits = []git.Commit{{ShortSHA: "short1", Title: "Fix login", Selected: true}}

	next, _ := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "checkout_select" {
		t.Fatalf("ScreenName() = %q, want checkout_select", got.ScreenName())
	}
}

func TestCommitSelectCtrlEnterOpensFocusedPR(t *testing.T) {
	browserCalls := []string{}
	m := New(Options{
		Config:  configDefaultForTest(),
		Browser: fakeBrowser{open: func(_ context.Context, url string) error {
			browserCalls = append(browserCalls, url)
			return nil
		}},
	})
	m.screen = "commit_select"
	m.Commits = []git.Commit{{ShortSHA: "short1", Title: "Fix login", PRURL: "https://github.com/owner/repo/pull/123"}}

	next, _ := m.Update(keyMsgCtrlEnter())
	got := next.(Model)
	if got.ScreenName() != "commit_select" {
		t.Fatalf("ScreenName() = %q, want commit_select", got.ScreenName())
	}
	if len(browserCalls) != 1 || browserCalls[0] != "https://github.com/owner/repo/pull/123" {
		t.Fatalf("browserCalls = %#v", browserCalls)
	}
}

func TestCommitSelectOOpensFocusedPR(t *testing.T) {
	browserCalls := []string{}
	m := New(Options{
		Config:  configDefaultForTest(),
		Browser: fakeBrowser{open: func(_ context.Context, url string) error {
			browserCalls = append(browserCalls, url)
			return nil
		}},
	})
	m.screen = "commit_select"
	m.Commits = []git.Commit{{ShortSHA: "short1", Title: "Fix login", PRURL: "https://github.com/owner/repo/pull/123"}}

	next, _ := m.Update(keyMsgO())
	got := next.(Model)
	if got.ScreenName() != "commit_select" {
		t.Fatalf("ScreenName() = %q, want commit_select", got.ScreenName())
	}
	if len(browserCalls) != 1 {
		t.Fatalf("browserCalls = %#v", browserCalls)
	}
}

func TestCheckoutSelectShowsTargets(t *testing.T) {
	m := New(Options{Config: configDefaultForTest()})
	m.screen = "checkout_select"
	m.CheckoutOptions = []string{"release/1.2.0", "Create new branch from tag"}
	if view := m.View(); !strings.Contains(view, "release/1.2.0") || !strings.Contains(view, "Create new branch from tag") {
		t.Fatalf("View() = %q", view)
	}
}

func TestCheckoutSelectChoosesExistingBranch(t *testing.T) {
	runner := &recordingRunner{outputs: map[string]string{
		"git checkout release/1.2.0": "",
	}}
	m := New(Options{
		WorkDir: "/tmp/repo",
		Config:  configDefaultForTest(),
		Git:     git.NewClient(runner),
	})
	m.screen = "checkout_select"
	m.CheckoutOptions = []string{"release/1.2.0", "Create new branch from tag"}

	next, _ := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "done" {
		t.Fatalf("ScreenName() = %q, want done", got.ScreenName())
	}
	if len(runner.calls) != 1 || runner.calls[0] != "git checkout release/1.2.0" {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestCheckoutSelectCreatesNewBranch(t *testing.T) {
	runner := &recordingRunner{outputs: map[string]string{
		"git checkout -b release/1.2.1 v1.2.0": "",
	}}
	m := New(Options{
		WorkDir: "/tmp/repo",
		Config:  configDefaultForTest(),
		Git:     git.NewClient(runner),
	})
	m.screen = "checkout_select"
	m.CheckoutOptions = []string{"release/1.2.0", "Create new branch from tag"}
	m.CheckoutIndex = 1
	m.NewBranchName = "release/1.2.1"
	m.NewBranchBase = "v1.2.0"

	next, _ := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "done" {
		t.Fatalf("ScreenName() = %q, want done", got.ScreenName())
	}
	if len(runner.calls) != 1 || runner.calls[0] != "git checkout -b release/1.2.1 v1.2.0" {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func configDefaultForTest() config.Config {
	return config.Config{DefaultRemote: "upstream"}
}

func keyMsgEnter() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEnter}
}

func keyMsgDown() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyDown}
}

func keyMsgSpace() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeySpace}
}

func keyMsgCtrlA() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlA}
}

func keyMsgCtrlD() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlD}
}

func keyMsgCtrlEnter() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ctrl+enter")}
}

func keyMsgO() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")}
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

type recordingRunner struct {
	outputs map[string]string
	calls   []string
}

func (r *recordingRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	key := strings.TrimSpace(name + " " + strings.Join(args, " "))
	r.calls = append(r.calls, key)
	if out, ok := r.outputs[key]; ok {
		return out, nil
	}
	return "", errors.New("unexpected command: " + key)
}

type fakeBrowser struct {
	open func(context.Context, string) error
}

func (f fakeBrowser) Open(ctx context.Context, url string) error {
	return f.open(ctx, url)
}
