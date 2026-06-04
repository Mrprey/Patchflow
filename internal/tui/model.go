package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"patchflow/internal/browser"
	"patchflow/internal/config"
	"patchflow/internal/git"
	"patchflow/internal/github"
	"patchflow/internal/logging"
)

type Options struct {
	WorkDir    string
	ConfigPath string
	Config     config.Config
	Git        git.Client
	GitHub     github.Client
	Browser    browser.Interface
	Logger     logging.Logger
	Stdout     interface{}
}

type Model struct {
	WorkDir    string
	ConfigPath string
	Config     config.Config
	Git        git.Client
	GitHub     github.Client
	Browser    browser.Interface
	Logger     logging.Logger

	screen           string
	history          []string
	loading          bool
	loadingMessage   string
	loadingToken     int
	spinner          spinner.Model
	Remotes          []git.Remote
	RemoteIndex      int
	RefOptions       []string
	RefIndex         int
	CompareFrom      string
	Commits          []git.Commit
	CommitIndex      int
	CheckoutOptions  []string
	CheckoutIndex    int
	CherryPickIndex  int
	CherryPickCommit git.Commit
	CherryPickErr    error
	NewBranchName    string
	NewBranchBase    string
	BranchName       string
	PushRemote       string
	TagName          string
	ReleaseName      string
	ReleaseNotesPath string
	Err              error
}

func New(opts Options) Model {
	return Model{
		WorkDir:    opts.WorkDir,
		ConfigPath: opts.ConfigPath,
		Config:     opts.Config,
		Git:        opts.Git,
		GitHub:     opts.GitHub,
		Browser:    opts.Browser,
		Logger:     opts.Logger,
		screen:     "welcome",
		spinner:    newLoadingSpinner(),
	}
}

type remoteOptionsLoadedMsg struct {
	token   int
	remote  string
	options []string
	err     error
}

type commitsLoadedMsg struct {
	token       int
	compareFrom string
	commits     []git.Commit
	err         error
}

type checkoutOptionsLoadedMsg struct {
	token   int
	options []string
	err     error
}

type cherryPickAppliedMsg struct {
	token   int
	commits []git.Commit
	err     error
}

type cherryPickConflictMsg struct {
	token  int
	index  int
	commit git.Commit
	err    error
}

func (m Model) ScreenName() string {
	return m.screen
}

func (m *Model) setScreen(next string) {
	if next == "" || m.screen == next {
		return
	}
	m.history = append(m.history, m.screen)
	m.screen = next
}

func (m *Model) backScreen() bool {
	if len(m.history) == 0 {
		return false
	}
	prev := m.history[len(m.history)-1]
	m.history = m.history[:len(m.history)-1]
	m.screen = prev
	return true
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	persist := true
	defer func() {
		if persist {
			_ = m.saveState()
		}
	}()

	if m.loading {
		switch msg := msg.(type) {
		case spinner.TickMsg:
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			persist = false
			return m, cmd
		case remoteOptionsLoadedMsg:
			if msg.token != m.loadingToken {
				persist = false
				return m, nil
			}
			m.loading = false
			m.loadingMessage = ""
			if msg.err != nil {
				m.Err = msg.err
				return m, nil
			}
			m.RefOptions = msg.options
			m.RemoteIndex = 0
			m.RefIndex = 0
			m.setScreen("ref_select")
			return m, nil
		case commitsLoadedMsg:
			if msg.token != m.loadingToken {
				persist = false
				return m, nil
			}
			m.loading = false
			m.loadingMessage = ""
			if msg.err != nil {
				m.Err = msg.err
				return m, nil
			}
			m.CompareFrom = msg.compareFrom
			m.Commits = msg.commits
			m.CommitIndex = 0
			m.setScreen("commit_select")
			return m, nil
		case checkoutOptionsLoadedMsg:
			if msg.token != m.loadingToken {
				persist = false
				return m, nil
			}
			m.loading = false
			m.loadingMessage = ""
			if msg.err != nil {
				m.Err = msg.err
				return m, nil
			}
			m.CheckoutOptions = msg.options
			m.CheckoutIndex = 0
			m.setScreen("checkout_select")
			return m, nil
		case cherryPickAppliedMsg:
			if msg.token != m.loadingToken {
				persist = false
				return m, nil
			}
			m.loading = false
			m.loadingMessage = ""
			if msg.err != nil {
				m.Err = msg.err
				return m, nil
			}
			if len(msg.commits) > 0 {
				m.Commits = msg.commits
			}
			m.Err = nil
			m.setScreen("versioning")
			return m, nil
		case cherryPickConflictMsg:
			if msg.token != m.loadingToken {
				persist = false
				return m, nil
			}
			m.loading = false
			m.loadingMessage = ""
			m.CherryPickIndex = msg.index
			m.CherryPickCommit = msg.commit
			m.CherryPickErr = msg.err
			m.Err = nil
			m.setScreen("cherry_pick_conflict")
			return m, nil
		case tea.KeyMsg:
			if msg.Type == tea.KeyEsc {
				m.cancelLoading()
				return m, nil
			}
			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}
			persist = false
			return m, nil
		default:
			persist = false
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" {
			return m, tea.Quit
		}
		if msg.Type == tea.KeyEsc {
			if m.backScreen() {
				return m, nil
			}
			return m, nil
		}
		if m.screen == "commit_select" && m.isOpenPRShortcut(msg) {
			if err := m.openFocusedPR(context.Background()); err != nil {
				m.Err = err
			}
			return m, nil
		}
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.screen == "welcome" {
				remotes, err := m.Git.RemoteList(context.Background(), m.WorkDir)
				if err != nil {
					m.Err = err
					return m, nil
				}
				m.Remotes = remotes
				m.Err = nil
				m.setScreen("remote_select")
				m.RemoteIndex = 0
				return m, nil
			}
			if m.screen == "remote_select" {
				remote := m.selectedRemote()
				if strings.TrimSpace(remote) == "" {
					m.Err = errors.New("no git remotes found")
					return m, nil
				}
				token := m.beginLoading(fmt.Sprintf("Fetching %s and loading refs...", remote))
				return m, tea.Batch(m.spinner.Tick, m.loadRemoteOptionsCmd(token, remote))
			}
			if m.screen == "ref_select" {
				if len(m.RefOptions) == 0 {
					m.Err = errors.New("no compare refs found")
					return m, nil
				}
				compareFrom := m.selectedRef()
				token := m.beginLoading(fmt.Sprintf("Loading commits from %s...", compareFrom))
				return m, tea.Batch(m.spinner.Tick, m.loadCommitsCmd(token, compareFrom))
			}
			if m.screen == "commit_select" {
				token := m.beginLoading("Loading checkout targets...")
				return m, tea.Batch(m.spinner.Tick, m.loadCheckoutOptionsCmd(token))
			}
			if m.screen == "checkout_select" {
				if err := m.handleCheckoutEnter(); err != nil {
					m.Err = err
					return m, nil
				}
				selected := m.selectedCommits()
				if len(selected) == 0 {
					m.Err = nil
					m.setScreen("versioning")
					return m, nil
				}
				m.setScreen("cherry_pick_progress")
				token := m.beginLoading("Applying selected commits...")
				return m, tea.Batch(m.spinner.Tick, m.applySelectedCommitsCmd(token, 0))
			}
			if m.screen == "cherry_pick_progress" {
				token := m.beginLoading("Applying selected commits...")
				return m, tea.Batch(m.spinner.Tick, m.applySelectedCommitsCmd(token, 0))
			}
			if m.screen == "cherry_pick_conflict" {
				if err := m.cherryPickContinueSelectedCommit(); err != nil {
					m.Err = err
					return m, nil
				}
				if len(m.selectedCommits()) == 0 {
					m.Err = nil
					m.setScreen("versioning")
					return m, nil
				}
				m.setScreen("cherry_pick_progress")
				token := m.beginLoading("Applying selected commits...")
				return m, tea.Batch(m.spinner.Tick, m.applySelectedCommitsCmd(token, 0))
			}
			if m.screen == "versioning" {
				m.Err = nil
				m.setScreen("push_select")
				return m, nil
			}
			if m.screen == "push_select" {
				if err := m.handlePushEnter(); err != nil {
					m.Err = err
					return m, nil
				}
				m.Err = nil
				if strings.TrimSpace(m.TagName) == "" {
					m.TagName = "v1.2.1"
				}
				m.ReleaseName = m.TagName
				m.setScreen("tag_select")
				return m, nil
			}
			if m.screen == "tag_select" {
				if err := m.handleTagEnter(); err != nil {
					m.Err = err
					return m, nil
				}
				m.Err = nil
				m.ReleaseName = m.TagName
				m.setScreen("release_select")
				return m, nil
			}
			if m.screen == "release_select" {
				if err := m.handleReleaseEnter(); err != nil {
					m.Err = err
					return m, nil
				}
				m.Err = nil
				m.setScreen("done")
				return m, nil
			}
		case tea.KeyUp:
			if m.screen == "remote_select" && len(m.Remotes) > 0 {
				m.RemoteIndex--
				if m.RemoteIndex < 0 {
					m.RemoteIndex = len(m.Remotes) - 1
				}
			}
			if m.screen == "ref_select" && len(m.RefOptions) > 0 {
				m.RefIndex--
				if m.RefIndex < 0 {
					m.RefIndex = len(m.RefOptions) - 1
				}
			}
			if m.screen == "commit_select" && len(m.Commits) > 0 {
				m.CommitIndex--
				if m.CommitIndex < 0 {
					m.CommitIndex = len(m.Commits) - 1
				}
			}
		case tea.KeyDown:
			if m.screen == "remote_select" && len(m.Remotes) > 0 {
				m.RemoteIndex++
				if m.RemoteIndex >= len(m.Remotes) {
					m.RemoteIndex = 0
				}
			}
			if m.screen == "ref_select" && len(m.RefOptions) > 0 {
				m.RefIndex++
				if m.RefIndex >= len(m.RefOptions) {
					m.RefIndex = 0
				}
			}
			if m.screen == "commit_select" && len(m.Commits) > 0 {
				m.CommitIndex++
				if m.CommitIndex >= len(m.Commits) {
					m.CommitIndex = 0
				}
			}
			if m.screen == "checkout_select" && len(m.CheckoutOptions) > 0 {
				m.CheckoutIndex++
				if m.CheckoutIndex >= len(m.CheckoutOptions) {
					m.CheckoutIndex = 0
				}
			}
		case tea.KeySpace:
			if m.screen == "commit_select" && len(m.Commits) > 0 {
				m.Commits[m.CommitIndex].Selected = !m.Commits[m.CommitIndex].Selected
			}
		case tea.KeyCtrlA:
			if m.screen == "commit_select" {
				for i := range m.Commits {
					m.Commits[i].Selected = true
				}
			}
		case tea.KeyCtrlD:
			if m.screen == "commit_select" {
				for i := range m.Commits {
					m.Commits[i].Selected = false
				}
			}
		case tea.KeyBackspace, tea.KeyDelete:
			if m.screen == "tag_select" {
				m.TagName = deleteLastRune(m.TagName)
				return m, nil
			}
			if m.screen == "release_select" {
				m.ReleaseName = deleteLastRune(m.ReleaseName)
				return m, nil
			}
		case tea.KeyRunes:
			switch msg.String() {
			case "e":
				if m.screen == "versioning" {
					if err := m.openEditor(context.Background()); err != nil {
						m.Err = err
					}
					return m, nil
				}
			case "r":
				if m.screen == "versioning" {
					if err := m.runValidation(context.Background()); err != nil {
						m.Err = err
					}
					return m, nil
				}
			case "p":
				if m.screen == "versioning" {
					m.setScreen("push_select")
					return m, nil
				}
			case "t":
				if m.screen == "push_select" {
					m.setScreen("tag_select")
					return m, nil
				}
			case "l":
				if m.screen == "tag_select" {
					m.setScreen("release_select")
					return m, nil
				}
			case "o":
				if m.screen == "commit_select" {
					if err := m.openFocusedPR(context.Background()); err != nil {
						m.Err = err
					}
					return m, nil
				}
			case "c":
				if m.screen == "cherry_pick_conflict" {
					if err := m.cherryPickContinueSelectedCommit(); err != nil {
						m.Err = err
						return m, nil
					}
					if len(m.selectedCommits()) == 0 {
						m.Err = nil
						m.setScreen("versioning")
						return m, nil
					}
					m.setScreen("cherry_pick_progress")
					token := m.beginLoading("Applying selected commits...")
					return m, tea.Batch(m.spinner.Tick, m.applySelectedCommitsCmd(token, 0))
				}
			case "s":
				if m.screen == "cherry_pick_conflict" {
					if err := m.cherryPickSkipSelectedCommit(); err != nil {
						m.Err = err
						return m, nil
					}
					if len(m.selectedCommits()) == 0 {
						m.Err = nil
						m.setScreen("versioning")
						return m, nil
					}
					m.setScreen("cherry_pick_progress")
					token := m.beginLoading("Applying selected commits...")
					return m, tea.Batch(m.spinner.Tick, m.applySelectedCommitsCmd(token, 0))
				}
			case "a":
				if m.screen == "cherry_pick_conflict" {
					if err := m.cherryPickAbort(); err != nil {
						m.Err = err
						return m, nil
					}
					return m, nil
				}
			}
			if m.screen == "tag_select" {
				m.TagName += msg.String()
				return m, nil
			}
			if m.screen == "release_select" {
				m.ReleaseName += msg.String()
				return m, nil
			}
		default:
			if msg.String() == "q" {
				return m, tea.Quit
			}
		}
	case error:
		m.Err = msg
	}
	return m, nil
}

func (m Model) View() string {
	if m.Err != nil && !m.loading {
		return renderScreen(
			"Error",
			"Something went wrong.",
			renderError(m.Err),
			[]string{"esc  back", "q  quit"},
		)
	}
	if m.loading {
		return m.renderLoadingView()
	}
	switch m.screen {
	case "welcome":
		return renderScreen(
			"Welcome",
			"Press enter to start the patch flow.",
			"Ready to inspect remotes, compare refs, and select commits.",
			[]string{"enter  start", "q  quit"},
		)
	case "remote_select":
		if len(m.Remotes) == 0 {
			return renderScreen(
				"Select remote",
				"Pick the remote to fetch before comparing refs.",
				"No remotes found.",
				[]string{"q  quit"},
			)
		}
		rows := make([]string, 0, len(m.Remotes))
		for i, remote := range m.Remotes {
			rows = append(rows, renderRow(i == m.RemoteIndex, fmt.Sprintf("  %s", remote.Name)))
		}
		return renderScreen(
			"Select remote",
			"Pick the remote to fetch before comparing refs.",
			strings.Join(rows, "\n"),
			[]string{"↑↓  move", "enter  continue", "q  quit"},
		)
	case "ref_select":
		if len(m.RefOptions) == 0 {
			return renderScreen(
				"Select compare base",
				fmt.Sprintf("Remote: %s", m.selectedRemote()),
				"No compare refs found.",
				[]string{"q  quit"},
			)
		}
		rows := make([]string, 0, len(m.RefOptions))
		for i, ref := range m.RefOptions {
			rows = append(rows, renderRow(i == m.RefIndex, fmt.Sprintf("  %s", ref)))
		}
		return renderScreen(
			"Select compare base",
			fmt.Sprintf("Remote: %s", m.selectedRemote()),
			strings.Join(rows, "\n"),
			[]string{"↑↓  move", "enter  continue", "q  quit"},
		)
	case "commit_select":
		if len(m.Commits) == 0 {
			return renderScreen(
				"Select commits",
				fmt.Sprintf("Selected: %d/%d", m.selectedCommitCount(), len(m.Commits)),
				"No commits found.",
				[]string{"q  quit"},
			)
		}
		rows := make([]string, 0, len(m.Commits))
		for i, commit := range m.Commits {
			check := "☐"
			if commit.Selected {
				check = "☑"
			}
			line := fmt.Sprintf("%s %s %s%s%s", check, commit.ShortSHA, commit.Title, m.commitPRSuffix(commit), m.commitLabelSuffix(commit))
			rows = append(rows, renderRow(i == m.CommitIndex, "  "+line))
		}
		return renderScreen(
			"Select commits",
			fmt.Sprintf("Selected: %d/%d", m.selectedCommitCount(), len(m.Commits)),
			strings.Join(rows, "\n"),
			[]string{"space  toggle", "ctrl+a  all", "ctrl+d  none", "ctrl+enter  open PR", "o  open PR", "enter  continue"},
		)
	case "checkout_select":
		if len(m.CheckoutOptions) == 0 {
			return renderScreen(
				"Select checkout target",
				"Choose the branch or tag to apply commits on.",
				"No checkout targets found.",
				[]string{"q  quit"},
			)
		}
		rows := make([]string, 0, len(m.CheckoutOptions))
		for i, option := range m.CheckoutOptions {
			rows = append(rows, renderRow(i == m.CheckoutIndex, fmt.Sprintf("  %s", option)))
		}
		return renderScreen(
			"Select checkout target",
			"Choose the branch or tag to apply commits on.",
			strings.Join(rows, "\n"),
			[]string{"↑↓  move", "enter  checkout", "q  quit"},
		)
	case "cherry_pick_progress":
		return renderScreen(
			"Cherry-pick",
			"Applying selected commits to the checkout target.",
			"Loading overlay shows the current operation while Git works.",
			[]string{"esc  back", "q  quit"},
		)
	case "cherry_pick_conflict":
		conflictText := "The cherry-pick stopped on a conflict."
		if m.CherryPickErr != nil {
			conflictText = conflictText + "\n\n" + renderError(m.CherryPickErr)
		}
		if strings.TrimSpace(m.CherryPickCommit.ShortSHA) != "" || strings.TrimSpace(m.CherryPickCommit.Title) != "" {
			conflictText += fmt.Sprintf("\n\n%s %s", m.CherryPickCommit.ShortSHA, m.CherryPickCommit.Title)
		}
		return renderScreen(
			"Cherry-pick conflict",
			"Resolve the files manually, then continue, skip, or abort.",
			conflictText,
			[]string{"e  editor", "c  continue", "s  skip", "a  abort", "esc  back"},
		)
	case "versioning":
		return renderScreen(
			"Versioning",
			"Update versioning files before continuing.",
			"Press e to open the editor, r to run validation, or enter to continue.",
			[]string{"e  editor", "r  validate", "enter  continue", "esc  back"},
		)
	case "push_select":
		return renderScreen(
			"Push",
			"Push the current branch to the configured remote.",
			"Press enter to run git push.",
			[]string{"enter  push", "esc  back", "q  quit"},
		)
	case "tag_select":
		return renderScreen(
			"Tag",
			"Create and push the release tag.",
			fmt.Sprintf("Tag name: %s\n\nType to edit the tag name, then press enter.", m.TagName),
			[]string{"type  edit", "backspace  delete", "enter  tag", "esc  back", "q  quit"},
		)
	case "release_select":
		return renderScreen(
			"Release",
			"Generate release notes and create the GitHub draft.",
			fmt.Sprintf("Release title: %s\nTag: %s\n\nType to edit the release title, then press enter.", m.releaseTitle(), m.TagName),
			[]string{"type  edit", "backspace  delete", "enter  release", "esc  back", "q  quit"},
		)
	case "done":
		return renderScreen(
			"Done",
			"UI flow complete.",
			"All requested steps finished.",
			[]string{"q  quit"},
		)
	default:
		return renderScreen(
			"Patchflow",
			"",
			"Press q to quit.",
			[]string{"q  quit"},
		)
	}
}

func (m Model) renderLoadingView() string {
	return renderScreen(
		"Loading",
		m.loadingMessage,
		strings.TrimSpace(m.spinner.View())+"\n\nPress esc to cancel.",
		[]string{"esc  back", "q  quit", "ctrl+c  quit"},
	)
}

func newLoadingSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return s
}

func (m *Model) beginLoading(message string) int {
	m.loading = true
	m.loadingMessage = message
	m.Err = nil
	m.spinner = newLoadingSpinner()
	m.loadingToken++
	return m.loadingToken
}

func (m *Model) cancelLoading() {
	m.loading = false
	m.loadingMessage = ""
	m.loadingToken++
}

func (m Model) loadRemoteOptionsCmd(token int, remote string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.Git.Fetch(ctx, m.WorkDir, remote); err != nil {
			return remoteOptionsLoadedMsg{token: token, remote: remote, err: err}
		}
		options, err := m.loadRefOptions(ctx, remote)
		return remoteOptionsLoadedMsg{token: token, remote: remote, options: options, err: err}
	}
}

func (m Model) loadCommitsCmd(token int, compareFrom string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		commits, err := m.Git.LogRange(ctx, m.WorkDir, compareFrom, "HEAD")
		if err == nil {
			commits = m.enrichCommitsWithPRs(commits)
		}
		return commitsLoadedMsg{token: token, compareFrom: compareFrom, commits: commits, err: err}
	}
}

func (m Model) loadCheckoutOptionsCmd(token int) tea.Cmd {
	return func() tea.Msg {
		options, err := m.loadCheckoutOptions(context.Background())
		return checkoutOptionsLoadedMsg{token: token, options: options, err: err}
	}
}

func (m Model) selectedRemote() string {
	if len(m.Remotes) == 0 {
		return ""
	}
	if m.RemoteIndex < 0 || m.RemoteIndex >= len(m.Remotes) {
		return m.Remotes[0].Name
	}
	return m.Remotes[m.RemoteIndex].Name
}

func (m Model) selectedRef() string {
	if len(m.RefOptions) == 0 {
		return ""
	}
	if m.RefIndex < 0 || m.RefIndex >= len(m.RefOptions) {
		return m.RefOptions[0]
	}
	return m.RefOptions[m.RefIndex]
}

func (m Model) selectedCommitCount() int {
	count := 0
	for _, commit := range m.Commits {
		if commit.Selected {
			count++
		}
	}
	return count
}

func (m Model) commitPRSuffix(commit git.Commit) string {
	if commit.PRNumber == 0 {
		return ""
	}
	suffix := fmt.Sprintf(" #%d", commit.PRNumber)
	if strings.TrimSpace(commit.PRURL) != "" {
		suffix += " ↗"
	}
	return suffix
}

func (m Model) commitLabelSuffix(commit git.Commit) string {
	if len(commit.Labels) == 0 {
		return ""
	}
	return " [" + strings.Join(commit.Labels, ", ") + "]"
}

func (m Model) enrichCommitsWithPRs(commits []git.Commit) []git.Commit {
	if m.GitHub.Runner == nil {
		return commits
	}
	for i := range commits {
		pr, err := m.GitHub.PullRequestForCommit(context.Background(), m.WorkDir, commits[i].SHA)
		if err != nil || pr == nil {
			continue
		}
		commits[i].PRNumber = pr.Number
		commits[i].PRTitle = pr.Title
		commits[i].PRURL = pr.URL
		commits[i].Labels = append([]string(nil), pr.Labels...)
	}
	return commits
}

func (m Model) openFocusedPR(ctx context.Context) error {
	if len(m.Commits) == 0 {
		return nil
	}
	commit := m.Commits[m.CommitIndex]
	if strings.TrimSpace(commit.PRURL) == "" {
		return nil
	}
	if m.Browser == nil {
		return nil
	}
	return m.Browser.Open(ctx, commit.PRURL)
}

func (m Model) isOpenPRShortcut(msg tea.KeyMsg) bool {
	return msg.String() == "ctrl+enter" || msg.String() == "o"
}

func (m *Model) handleCheckoutEnter() error {
	target := m.selectedCheckoutOption()
	if strings.TrimSpace(target) == "" {
		return errors.New("no checkout target selected")
	}
	if _, err := m.runLogged(context.Background(), "git", "checkout", target); err != nil {
		return err
	}
	m.BranchName = target
	return nil
}

func (m Model) selectedCheckoutOption() string {
	if len(m.CheckoutOptions) == 0 {
		return ""
	}
	if m.CheckoutIndex < 0 || m.CheckoutIndex >= len(m.CheckoutOptions) {
		return m.CheckoutOptions[0]
	}
	return m.CheckoutOptions[m.CheckoutIndex]
}

func (m Model) openEditor(ctx context.Context) error {
	if strings.TrimSpace(m.Config.Editor) == "" {
		return nil
	}
	return runShellCommand(ctx, m.Git.Runner, m.Logger, m.WorkDir, m.Config.Editor)
}

func (m Model) runValidation(ctx context.Context) error {
	if strings.TrimSpace(m.Config.PostCherryPickCmd) == "" {
		return nil
	}
	return runShellCommand(ctx, m.Git.Runner, m.Logger, m.WorkDir, m.Config.PostCherryPickCmd)
}

func (m Model) loadRefOptions(ctx context.Context, remote string) ([]string, error) {
	branches, err := m.Git.RemoteBranches(ctx, m.WorkDir, remote)
	if err != nil {
		return nil, err
	}
	tags, err := m.Git.Tags(ctx, m.WorkDir)
	if err != nil {
		return nil, err
	}
	return mergeUnique(branches, tags), nil
}

func (m Model) loadCheckoutOptions(ctx context.Context) ([]string, error) {
	branches, err := m.Git.LocalBranches(ctx, m.WorkDir)
	if err != nil {
		return nil, err
	}
	tags, err := m.Git.Tags(ctx, m.WorkDir)
	if err != nil {
		return nil, err
	}
	return mergeUnique(branches, tags), nil
}

func (m Model) applySelectedCommitsCmd(token int, startSelectedIndex int) tea.Cmd {
	return func() tea.Msg {
		commits := append([]git.Commit(nil), m.Commits...)
		selectedIndex := 0
		for i := range commits {
			if !commits[i].Selected {
				continue
			}
			if selectedIndex < startSelectedIndex {
				selectedIndex++
				continue
			}
			if _, err := m.runLogged(context.Background(), "git", "cherry-pick", commits[i].SHA); err != nil {
				commits[i].Status = "failed"
				return cherryPickConflictMsg{
					token:  token,
					index:  selectedIndex,
					commit: commits[i],
					err:    err,
				}
			}
			commits[i].Status = "applied"
			commits[i].Selected = false
			selectedIndex++
		}
		return cherryPickAppliedMsg{token: token, commits: commits}
	}
}

func (m *Model) cherryPickContinueSelectedCommit() error {
	_, err := m.runLogged(context.Background(), "git", "cherry-pick", "--continue")
	if err != nil {
		return err
	}
	m.markCommitBySHA(m.CherryPickCommit.SHA, "applied", false)
	m.CherryPickErr = nil
	m.loadingMessage = ""
	m.Err = nil
	return nil
}

func (m *Model) cherryPickSkipSelectedCommit() error {
	_, err := m.runLogged(context.Background(), "git", "cherry-pick", "--skip")
	if err != nil {
		return err
	}
	m.markCommitBySHA(m.CherryPickCommit.SHA, "skipped", false)
	m.CherryPickErr = nil
	m.Err = nil
	return nil
}

func (m *Model) cherryPickAbort() error {
	_, err := m.runLogged(context.Background(), "git", "cherry-pick", "--abort")
	if err != nil {
		return err
	}
	m.CherryPickErr = nil
	m.Err = nil
	m.setScreen("checkout_select")
	return nil
}

func (m *Model) markCommitBySHA(sha, status string, selected bool) {
	for i := range m.Commits {
		if m.Commits[i].SHA != sha {
			continue
		}
		m.Commits[i].Status = status
		m.Commits[i].Selected = selected
		return
	}
}

func (m *Model) handlePushEnter() error {
	if m.PushRemote == "" {
		m.PushRemote = "origin"
	}
	branch := m.BranchName
	if branch == "" {
		branch = m.NewBranchName
	}
	if branch == "" {
		branch = m.selectedCheckoutOption()
	}
	_, err := m.runLogged(context.Background(), "git", "push", m.PushRemote, branch)
	return err
}

func (m *Model) handleTagEnter() error {
	if m.PushRemote == "" {
		m.PushRemote = "origin"
	}
	if m.TagName == "" {
		m.TagName = "v1.2.1"
	}
	if _, err := m.runLogged(context.Background(), "git", "tag", "-a", m.TagName, "-m", m.TagName); err != nil {
		return err
	}
	branch := m.BranchName
	if branch == "" {
		branch = m.NewBranchName
	}
	if branch == "" {
		branch = m.selectedCheckoutOption()
	}
	_, err := m.runLogged(context.Background(), "git", "push", m.PushRemote, m.TagName)
	if err != nil {
		return err
	}
	m.BranchName = branch
	return nil
}

func (m *Model) handleReleaseEnter() error {
	if m.TagName == "" {
		m.TagName = "v1.2.1"
	}
	if strings.TrimSpace(m.ReleaseName) == "" {
		m.ReleaseName = m.TagName
	}
	if m.ReleaseNotesPath == "" {
		m.ReleaseNotesPath = ".patchflow/release-notes.md"
	}
	notes := m.releaseNotes()
	if err := writeTextFile(m.ReleaseNotesPath, notes); err != nil {
		return err
	}
	branch := m.BranchName
	if branch == "" {
		branch = m.NewBranchName
	}
	if branch == "" {
		branch = m.selectedCheckoutOption()
	}
	_, err := m.runLogged(context.Background(), "gh", "release", "create", m.TagName, "--target", branch, "--title", m.ReleaseName, "--notes-file", m.ReleaseNotesPath, "--draft")
	return err
}

func (m Model) releaseTitle() string {
	if strings.TrimSpace(m.ReleaseName) != "" {
		return m.ReleaseName
	}
	if strings.TrimSpace(m.TagName) != "" {
		return m.TagName
	}
	return "v1.2.1"
}

func deleteLastRune(value string) string {
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[:len(runes)-1])
}

func (m Model) releaseNotes() string {
	selected := m.selectedCommits()
	var b strings.Builder
	b.WriteString("## Changes\n\n")
	for _, commit := range selected {
		if commit.PRNumber > 0 {
			fmt.Fprintf(&b, "- %s (#%d)", commit.Title, commit.PRNumber)
			if len(commit.Labels) > 0 {
				fmt.Fprintf(&b, " [%s]", strings.Join(commit.Labels, ", "))
			}
			b.WriteString("\n")
			continue
		}
		fmt.Fprintf(&b, "- %s\n", commit.Title)
	}
	b.WriteString("\n## Cherry-picked commits\n\n")
	for _, commit := range selected {
		fmt.Fprintf(&b, "- %s %s\n", commit.ShortSHA, commit.Title)
	}
	return b.String()
}

func (m Model) selectedCommits() []git.Commit {
	selected := make([]git.Commit, 0, len(m.Commits))
	for _, commit := range m.Commits {
		if commit.Selected {
			selected = append(selected, commit)
		}
	}
	return selected
}

func mergeUnique(lists ...[]string) []string {
	seen := make(map[string]struct{})
	merged := make([]string, 0)
	for _, list := range lists {
		for _, item := range list {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			merged = append(merged, item)
		}
	}
	return merged
}

func writeTextFile(path string, content string) error {
	if err := ensureDir(path); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func ensureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func (m Model) saveState() error {
	if strings.TrimSpace(m.WorkDir) == "" {
		return nil
	}
	state := stateSnapshot{
		CurrentStep:     m.screen,
		CurrentCommit:   m.currentCommitSHA(),
		TagName:         m.TagName,
		ReleaseName:     m.ReleaseName,
		SelectedCommits: m.selectedStateCommits(),
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	path := m.resolveStatePath()
	if err := ensureDir(path); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (m Model) resolveStatePath() string {
	return filepath.Join(m.WorkDir, ".patchflow", "state.json")
}

type stateSnapshot struct {
	CurrentStep     string        `json:"currentStep"`
	CurrentCommit   string        `json:"currentCommit"`
	TagName         string        `json:"tagName,omitempty"`
	ReleaseName     string        `json:"releaseName,omitempty"`
	SelectedCommits []stateCommit `json:"selectedCommits"`
}

type stateCommit struct {
	SHA      string   `json:"sha"`
	ShortSHA string   `json:"shortSha"`
	Title    string   `json:"title"`
	PRNumber int      `json:"prNumber,omitempty"`
	PRTitle  string   `json:"prTitle,omitempty"`
	PRURL    string   `json:"prUrl,omitempty"`
	Labels   []string `json:"labels,omitempty"`
	Status   string   `json:"status"`
}

func (m Model) currentCommitSHA() string {
	if len(m.Commits) == 0 || m.CommitIndex < 0 || m.CommitIndex >= len(m.Commits) {
		return ""
	}
	return m.Commits[m.CommitIndex].SHA
}

func (m Model) selectedStateCommits() []stateCommit {
	selected := make([]stateCommit, 0, len(m.Commits))
	for _, commit := range m.Commits {
		if !commit.Selected {
			continue
		}
		selected = append(selected, stateCommit{
			SHA:      commit.SHA,
			ShortSHA: commit.ShortSHA,
			Title:    commit.Title,
			PRNumber: commit.PRNumber,
			PRTitle:  commit.PRTitle,
			PRURL:    commit.PRURL,
			Labels:   append([]string(nil), commit.Labels...),
			Status:   commit.Status,
		})
	}
	return selected
}

func runShellCommand(ctx context.Context, runner interface {
	Run(context.Context, string, string, ...string) (string, error)
}, logger logging.Logger, dir, command string) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	if runtime.GOOS == "windows" {
		out, err := runner.Run(ctx, dir, "cmd", "/c", command)
		if logger.Dir != "" {
			_ = logger.Write("cmd /c "+command, out)
		}
		return err
	}
	out, err := runner.Run(ctx, dir, "sh", "-c", command)
	if logger.Dir != "" {
		_ = logger.Write("sh -c "+command, out)
	}
	return err
}

func (m Model) runLogged(ctx context.Context, name string, args ...string) (string, error) {
	out, err := m.Git.Runner.Run(ctx, m.WorkDir, name, args...)
	if m.Logger.Dir != "" {
		_ = m.Logger.Write(strings.TrimSpace(name+" "+strings.Join(args, " ")), out)
	}
	return out, err
}
