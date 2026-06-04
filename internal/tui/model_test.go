package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"patchflow/internal/config"
	"patchflow/internal/git"
	"patchflow/internal/github"
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
	if view := stripANSI(m.View()); !strings.Contains(view, "PATCHFLOW") || !strings.Contains(view, "Welcome") || !strings.Contains(view, "enter  start") {
		t.Fatalf("View() = %q", view)
	}
	m.screen = "remote_select"
	if view := stripANSI(m.View()); !strings.Contains(view, "Select remote") || !strings.Contains(view, "No remotes found.") || !strings.Contains(view, "q  quit") {
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

func TestRemoteSelectEnterShowsLoadingBeforeRefSelect(t *testing.T) {
	m := New(Options{
		Config: configDefaultForTest(),
		Git: git.NewClient(fakeRunner{outputs: map[string]string{
			"git fetch origin --tags --prune":                                "",
			"git for-each-ref --format=%(refname:short) refs/remotes/origin": "origin/main\n",
			"git tag --list": "v1.2.0\n",
		}}),
	})
	m.screen = "remote_select"
	m.Remotes = []git.Remote{{Name: "origin"}}

	next, _ := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "remote_select" {
		t.Fatalf("ScreenName() = %q, want remote_select while loading", got.ScreenName())
	}
	if view := stripANSI(got.View()); !strings.Contains(view, "loading refs") {
		t.Fatalf("View() = %q", view)
	}
}

func TestRefSelectEnterShowsLoadingBeforeCommitSelect(t *testing.T) {
	m := New(Options{
		Config: configDefaultForTest(),
		Git: git.NewClient(fakeRunner{outputs: map[string]string{
			"git log --pretty=format:%H%x09%h%x09%an%x09%ad%x09%s --date=short upstream/main...HEAD": "sha1\tshort1\tAna\t2024-01-01\tFix login\n",
		}}),
	})
	m.screen = "ref_select"
	m.RefOptions = []string{"upstream/main"}
	m.RefIndex = 0

	next, _ := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "ref_select" {
		t.Fatalf("ScreenName() = %q, want ref_select while loading", got.ScreenName())
	}
	if view := stripANSI(got.View()); !strings.Contains(view, "Loading commits") {
		t.Fatalf("View() = %q", view)
	}
}

func TestCommitSelectEnterShowsLoadingBeforeCheckoutSelect(t *testing.T) {
	m := New(Options{
		Config: configDefaultForTest(),
		Git: git.NewClient(fakeRunner{outputs: map[string]string{
			"git for-each-ref --format=%(refname:short) refs/heads": "main\nrelease/1.2.0\n",
			"git tag --list": "v1.2.0\n",
		}}),
	})
	m.screen = "commit_select"
	m.Commits = []git.Commit{{ShortSHA: "short1", Title: "Fix login", Selected: true}}

	next, _ := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "commit_select" {
		t.Fatalf("ScreenName() = %q, want commit_select while loading", got.ScreenName())
	}
	if view := stripANSI(got.View()); !strings.Contains(view, "Loading checkout targets") {
		t.Fatalf("View() = %q", view)
	}
}

func TestEscCancelsLoadingAndKeepsOriginScreen(t *testing.T) {
	m := New(Options{Config: configDefaultForTest()})
	m.screen = "remote_select"
	m.loading = true
	m.loadingMessage = "Fetching remote..."

	next, _ := m.Update(keyMsgEsc())
	got := next.(Model)
	if got.loading {
		t.Fatalf("loading = true, want false")
	}
	if got.ScreenName() != "remote_select" {
		t.Fatalf("ScreenName() = %q, want remote_select", got.ScreenName())
	}
}

func TestRuneQQuitsFromNormalScreens(t *testing.T) {
	m := New(Options{Config: configDefaultForTest()})

	_, cmd := m.Update(keyMsgRune("q"))
	if cmd == nil {
		t.Fatal("cmd = nil, want quit")
	}
}

func TestSpinnerTickWhileLoadingSchedulesNextFrame(t *testing.T) {
	m := New(Options{Config: configDefaultForTest()})
	m.loading = true
	m.loadingMessage = "Loading commits..."

	next, cmd := m.Update(spinner.TickMsg{})
	got := next.(Model)
	if got.ScreenName() != "welcome" {
		t.Fatalf("ScreenName() = %q, want welcome while loading", got.ScreenName())
	}
	if cmd == nil {
		t.Fatalf("cmd = nil, want next spinner tick")
	}
}

func TestEscOnWelcomeStaysOnWelcome(t *testing.T) {
	m := New(Options{Config: configDefaultForTest()})

	next, _ := m.Update(keyMsgEsc())
	got := next.(Model)
	if got.ScreenName() != "welcome" {
		t.Fatalf("ScreenName() = %q, want welcome", got.ScreenName())
	}
}

func TestEscReturnsBackThroughFlowWithoutLosingState(t *testing.T) {
	m := New(Options{
		Config: configDefaultForTest(),
		Git: git.NewClient(fakeRunner{outputs: map[string]string{
			"git remote -v":                   "origin\tgit@github.com:a.git (fetch)\nupstream\tgit@github.com:b.git (fetch)\n",
			"git fetch origin --tags --prune": "",
			"git for-each-ref --format=%(refname:short) refs/remotes/origin": "origin/HEAD\norigin/main\norigin/release/1.2.0\n",
			"git tag --list": "v1.2.0\nv1.2.1\n",
		}}),
	})

	next, _ := m.Update(keyMsgEnter())
	got := next.(Model)
	next, cmd := got.Update(keyMsgEnter())
	got = next.(Model)
	if got.ScreenName() != "remote_select" {
		t.Fatalf("ScreenName() = %q, want remote_select while loading", got.ScreenName())
	}
	got = applyCmd(t, got, cmd)
	if got.ScreenName() != "ref_select" {
		t.Fatalf("ScreenName() = %q, want ref_select", got.ScreenName())
	}

	got.RefOptions = []string{"upstream/main", "upstream/release/1.2.0"}
	got.RefIndex = 1

	next, _ = got.Update(keyMsgEsc())
	got = next.(Model)
	if got.ScreenName() != "remote_select" {
		t.Fatalf("ScreenName() = %q, want remote_select", got.ScreenName())
	}
	if got.RemoteIndex != 0 {
		t.Fatalf("RemoteIndex = %d, want 0", got.RemoteIndex)
	}
	if len(got.Remotes) != 2 {
		t.Fatalf("Remotes = %#v", got.Remotes)
	}

	next, _ = got.Update(keyMsgEsc())
	got = next.(Model)
	if got.ScreenName() != "welcome" {
		t.Fatalf("ScreenName() = %q, want welcome", got.ScreenName())
	}
}

func TestEscReturnsFromCherryPickProgressToPreviousScreen(t *testing.T) {
	m := New(Options{Config: configDefaultForTest()})
	m.screen = "cherry_pick_progress"
	m.history = []string{"checkout_select"}

	next, _ := m.Update(keyMsgEsc())
	got := next.(Model)
	if got.ScreenName() != "checkout_select" {
		t.Fatalf("ScreenName() = %q, want checkout_select", got.ScreenName())
	}
}

func TestRemoteSelectMovesFocusAndFetchesSelectedRemote(t *testing.T) {
	runner := &recordingRunner{outputs: map[string]string{
		"git fetch upstream --tags --prune":                                "",
		"git for-each-ref --format=%(refname:short) refs/remotes/upstream": "upstream/HEAD\nupstream/main\nupstream/release/1.2.0\n",
		"git tag --list": "v1.2.0\nv1.2.1\n",
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

	next, cmd := got.Update(keyMsgEnter())
	got = next.(Model)
	if got.ScreenName() != "remote_select" {
		t.Fatalf("ScreenName() = %q, want remote_select while loading", got.ScreenName())
	}
	got = applyCmd(t, got, cmd)
	if got.ScreenName() != "ref_select" {
		t.Fatalf("ScreenName() = %q, want ref_select", got.ScreenName())
	}
	if len(got.RefOptions) != 4 || got.RefOptions[0] != "upstream/main" || got.RefOptions[1] != "upstream/release/1.2.0" || got.RefOptions[2] != "v1.2.0" || got.RefOptions[3] != "v1.2.1" {
		t.Fatalf("RefOptions = %#v", got.RefOptions)
	}
	if len(runner.calls) != 3 || runner.calls[0] != "git fetch upstream --tags --prune" {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestRefSelectShowsAvailableRefs(t *testing.T) {
	m := New(Options{
		Config: configDefaultForTest(),
	})
	m.screen = "ref_select"
	m.RefOptions = []string{"upstream/main", "upstream/release/1.2.0", "v1.2.0"}
	if view := m.View(); !strings.Contains(view, "upstream/main") || !strings.Contains(view, "upstream/release/1.2.0") || !strings.Contains(view, "v1.2.0") {
		t.Fatalf("View() = %q", view)
	}
}

func TestRefSelectMovesAndChoosesBase(t *testing.T) {
	m := New(Options{Config: configDefaultForTest()})
	m.screen = "ref_select"
	m.RefOptions = []string{"upstream/main", "upstream/release/1.2.0", "v1.2.0"}

	next, _ := m.Update(keyMsgDown())
	got := next.(Model)
	if got.RefIndex != 1 {
		t.Fatalf("RefIndex = %d, want 1", got.RefIndex)
	}

	next, cmd := got.Update(keyMsgEnter())
	got = next.(Model)
	if got.ScreenName() != "ref_select" {
		t.Fatalf("ScreenName() = %q, want ref_select while loading", got.ScreenName())
	}
	got = applyCmd(t, got, cmd)
	if got.CompareFrom != "upstream/release/1.2.0" {
		t.Fatalf("CompareFrom = %q, want upstream/release/1.2.0", got.CompareFrom)
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

	next, cmd := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "ref_select" {
		t.Fatalf("ScreenName() = %q, want ref_select while loading", got.ScreenName())
	}
	got = applyCmd(t, got, cmd)
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

func TestRemoteSelectLoadsRealRefsAndAdvancesToRefSelect(t *testing.T) {
	runner := &recordingRunner{outputs: map[string]string{
		"git fetch upstream --tags --prune":                                "",
		"git for-each-ref --format=%(refname:short) refs/remotes/upstream": "upstream/HEAD\nupstream/main\nupstream/release/1.2.0\n",
		"git tag --list": "v1.2.0\nv1.2.1\n",
	}}
	m := New(Options{
		WorkDir: "/tmp/repo",
		Config:  configDefaultForTest(),
		Git:     git.NewClient(runner),
	})
	m.screen = "remote_select"
	m.Remotes = []git.Remote{{Name: "upstream"}}

	next, cmd := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "remote_select" {
		t.Fatalf("ScreenName() = %q, want remote_select while loading", got.ScreenName())
	}
	got = applyCmd(t, got, cmd)
	if got.ScreenName() != "ref_select" {
		t.Fatalf("ScreenName() = %q, want ref_select", got.ScreenName())
	}
	if len(got.RefOptions) != 4 || got.RefOptions[0] != "upstream/main" || got.RefOptions[1] != "upstream/release/1.2.0" || got.RefOptions[2] != "v1.2.0" || got.RefOptions[3] != "v1.2.1" {
		t.Fatalf("RefOptions = %#v", got.RefOptions)
	}
	if len(runner.calls) != 3 {
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

	next, cmd := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "ref_select" {
		t.Fatalf("ScreenName() = %q, want ref_select while loading", got.ScreenName())
	}
	got = applyCmd(t, got, cmd)
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

	next, cmd := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "commit_select" {
		t.Fatalf("ScreenName() = %q, want commit_select while loading", got.ScreenName())
	}
	got = applyCmd(t, got, cmd)
	if got.ScreenName() != "checkout_select" {
		t.Fatalf("ScreenName() = %q, want checkout_select", got.ScreenName())
	}
}

func TestCommitSelectCtrlEnterOpensFocusedPR(t *testing.T) {
	browserCalls := []string{}
	m := New(Options{
		Config: configDefaultForTest(),
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
		Config: configDefaultForTest(),
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
	m.CheckoutOptions = []string{"release/1.2.0", "v1.2.0"}
	if view := m.View(); !strings.Contains(view, "release/1.2.0") || !strings.Contains(view, "v1.2.0") {
		t.Fatalf("View() = %q", view)
	}
}

func TestCheckoutSelectChoosesExistingBranch(t *testing.T) {
	runner := &recordingRunner{outputs: map[string]string{
		"git for-each-ref --format=%(refname:short) refs/heads": "main\nrelease/1.2.0\n",
		"git tag --list":             "v1.2.0\nv1.2.1\n",
		"git checkout release/1.2.0": "",
	}}
	m := New(Options{
		WorkDir: "/tmp/repo",
		Config:  configDefaultForTest(),
		Git:     git.NewClient(runner),
	})
	m.screen = "checkout_select"
	m.CheckoutOptions = []string{"release/1.2.0", "v1.2.0"}

	next, _ := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "versioning" {
		t.Fatalf("ScreenName() = %q, want versioning", got.ScreenName())
	}
	if len(runner.calls) != 1 || runner.calls[0] != "git checkout release/1.2.0" {
		t.Fatalf("calls = %#v", runner.calls)
	}
	if got.BranchName != "release/1.2.0" {
		t.Fatalf("BranchName = %q", got.BranchName)
	}
}

func TestCommitSelectLoadsCheckoutTargetsBeforeAdvancing(t *testing.T) {
	runner := &recordingRunner{outputs: map[string]string{
		"git for-each-ref --format=%(refname:short) refs/heads": "main\nrelease/1.2.0\n",
		"git tag --list": "v1.2.0\nv1.2.1\n",
	}}
	m := New(Options{
		WorkDir: "/tmp/repo",
		Config:  configDefaultForTest(),
		Git:     git.NewClient(runner),
	})
	m.screen = "commit_select"
	m.Commits = []git.Commit{{ShortSHA: "short1", Title: "Fix login", Selected: true}}

	next, cmd := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "commit_select" {
		t.Fatalf("ScreenName() = %q, want commit_select while loading", got.ScreenName())
	}
	got = applyCmd(t, got, cmd)
	if got.ScreenName() != "checkout_select" {
		t.Fatalf("ScreenName() = %q, want checkout_select", got.ScreenName())
	}
	if len(got.CheckoutOptions) != 4 || got.CheckoutOptions[0] != "main" || got.CheckoutOptions[1] != "release/1.2.0" || got.CheckoutOptions[2] != "v1.2.0" || got.CheckoutOptions[3] != "v1.2.1" {
		t.Fatalf("CheckoutOptions = %#v", got.CheckoutOptions)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestCherryPickProgressAppliesSelectedCommitsAndMovesToVersioning(t *testing.T) {
	runner := &recordingRunner{outputs: map[string]string{
		"git cherry-pick sha1": "",
		"git cherry-pick sha2": "",
	}}
	m := New(Options{
		WorkDir: "/tmp/repo",
		Config:  configDefaultForTest(),
		Git:     git.NewClient(runner),
	})
	m.screen = "cherry_pick_progress"
	m.Commits = []git.Commit{
		{SHA: "sha1", ShortSHA: "short1", Title: "Fix login", Selected: true},
		{SHA: "sha2", ShortSHA: "short2", Title: "Update docs", Selected: true},
	}

	next, cmd := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "cherry_pick_progress" {
		t.Fatalf("ScreenName() = %q, want cherry_pick_progress while loading", got.ScreenName())
	}
	got = applyCmd(t, got, cmd)
	if got.ScreenName() != "versioning" {
		t.Fatalf("ScreenName() = %q, want versioning", got.ScreenName())
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestCherryPickConflictContinueResumesRemainingCommits(t *testing.T) {
	runner := &recordingRunner{
		outputs: map[string]string{
			"git cherry-pick --continue": "",
			"git cherry-pick sha2":       "",
		},
		errs: map[string]error{
			"git cherry-pick sha1": errors.New("merge conflict"),
		},
	}
	m := New(Options{
		WorkDir: "/tmp/repo",
		Config:  configDefaultForTest(),
		Git:     git.NewClient(runner),
	})
	m.screen = "cherry_pick_progress"
	m.Commits = []git.Commit{
		{SHA: "sha1", ShortSHA: "short1", Title: "Fix login", Selected: true},
		{SHA: "sha2", ShortSHA: "short2", Title: "Update docs", Selected: true},
	}

	next, cmd := m.Update(keyMsgEnter())
	got := next.(Model)
	got = applyCmd(t, got, cmd)
	if got.ScreenName() != "cherry_pick_conflict" {
		t.Fatalf("ScreenName() = %q, want cherry_pick_conflict", got.ScreenName())
	}
	if got.CherryPickCommit.SHA != "sha1" || got.CherryPickErr == nil {
		t.Fatalf("Conflict = %#v err=%v", got.CherryPickCommit, got.CherryPickErr)
	}

	next, cmd = got.Update(keyMsgRune("c"))
	got = next.(Model)
	if got.ScreenName() != "cherry_pick_progress" {
		t.Fatalf("ScreenName() = %q, want cherry_pick_progress while loading", got.ScreenName())
	}
	got = applyCmd(t, got, cmd)
	if got.ScreenName() != "versioning" {
		t.Fatalf("ScreenName() = %q, want versioning", got.ScreenName())
	}
	if len(runner.calls) != 3 {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestVersioningKeysAndContinue(t *testing.T) {
	runner := &recordingRunner{outputs: map[string]string{
		"sh -c code --wait":     "",
		"sh -c flutter analyze": "",
	}}
	m := New(Options{
		WorkDir: "/tmp/repo",
		Config: config.Config{
			Editor:            "code --wait",
			PostCherryPickCmd: "flutter analyze",
		},
		Git: git.NewClient(runner),
	})
	m.screen = "versioning"

	next, _ := m.Update(keyMsgRune("e"))
	got := next.(Model)
	if got.ScreenName() != "versioning" {
		t.Fatalf("ScreenName() = %q, want versioning", got.ScreenName())
	}

	next, _ = got.Update(keyMsgRune("r"))
	got = next.(Model)
	if got.ScreenName() != "versioning" {
		t.Fatalf("ScreenName() = %q, want versioning", got.ScreenName())
	}

	next, _ = got.Update(keyMsgEnter())
	got = next.(Model)
	if got.ScreenName() != "push_select" {
		t.Fatalf("ScreenName() = %q, want push_select", got.ScreenName())
	}
}

func TestPushTagAndReleaseFlow(t *testing.T) {
	runner := &recordingRunner{outputs: map[string]string{
		"git push origin release/1.2.1": "",
		"git tag -a v1.2.2 -m v1.2.2":   "",
		"git push origin v1.2.2":        "",
		"gh release create v1.2.2 --target release/1.2.1 --title v1.2.2-rc1 --notes-file /tmp/repo/.patchflow/release-notes.md --draft": "",
	}}
	m := New(Options{
		WorkDir: "/tmp/repo",
		Config: config.Config{
			DefaultPushRemote: "origin",
			ReleaseNotesFile:  "/tmp/repo/.patchflow/release-notes.md",
		},
		Git: git.NewClient(runner),
	})
	m.screen = "push_select"
	m.BranchName = "release/1.2.1"
	m.TagName = "v1.2.1"
	m.ReleaseNotesPath = "/tmp/repo/.patchflow/release-notes.md"
	m.Commits = []git.Commit{{ShortSHA: "short1", Title: "Fix login", Selected: true}}

	next, _ := m.Update(keyMsgEnter())
	got := next.(Model)
	if got.ScreenName() != "tag_select" {
		t.Fatalf("ScreenName() = %q, want tag_select", got.ScreenName())
	}

	next, _ = got.Update(keyMsgBackspace())
	got = next.(Model)
	if got.TagName != "v1.2." {
		t.Fatalf("TagName = %q, want v1.2.", got.TagName)
	}
	next, _ = got.Update(keyMsgRune("2"))
	got = next.(Model)
	if got.TagName != "v1.2.2" {
		t.Fatalf("TagName = %q, want v1.2.2", got.TagName)
	}

	next, _ = got.Update(keyMsgEnter())
	got = next.(Model)
	if got.ScreenName() != "release_select" {
		t.Fatalf("ScreenName() = %q, want release_select", got.ScreenName())
	}
	if got.TagName != "v1.2.2" {
		t.Fatalf("TagName = %q, want v1.2.2", got.TagName)
	}
	if got.ReleaseName != "v1.2.2" {
		t.Fatalf("ReleaseName = %q, want v1.2.2", got.ReleaseName)
	}

	next, _ = got.Update(keyMsgRune("-rc1"))
	got = next.(Model)
	if got.ReleaseName != "v1.2.2-rc1" {
		t.Fatalf("ReleaseName = %q, want v1.2.2-rc1", got.ReleaseName)
	}

	next, _ = got.Update(keyMsgEnter())
	got = next.(Model)
	if got.ScreenName() != "done" {
		t.Fatalf("ScreenName() = %q, want done", got.ScreenName())
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

func keyMsgEsc() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEsc}
}

func keyMsgBackspace() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyBackspace}
}

func keyMsgCtrlEnter() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ctrl+enter")}
}

func keyMsgO() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")}
}

func keyMsgRune(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func applyCmd(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	msg := cmd()
	switch msg := msg.(type) {
	case tea.BatchMsg:
		for _, sub := range msg {
			if sub == nil {
				continue
			}
			next, _ := m.Update(sub())
			m = next.(Model)
		}
		return m
	default:
		next, _ := m.Update(msg)
		return next.(Model)
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

type recordingRunner struct {
	outputs map[string]string
	errs    map[string]error
	calls   []string
}

func (r *recordingRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	key := strings.TrimSpace(name + " " + strings.Join(args, " "))
	r.calls = append(r.calls, key)
	if err, ok := r.errs[key]; ok {
		if out, ok := r.outputs[key]; ok {
			return out, err
		}
		return "", err
	}
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
