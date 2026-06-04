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
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"patchflow/internal/browser"
	"patchflow/internal/config"
	"patchflow/internal/git"
	"patchflow/internal/github"
	"patchflow/internal/logging"
)

const createNewBranchOption = "Create new branch from branch/tag"

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

	screen                     string
	history                    []string
	loading                    bool
	loadingMessage             string
	loadingCommand             string
	loadingStatus              string
	loadingStartedAt           time.Time
	loadingToken               int
	Width                      int
	Height                     int
	spinner                    spinner.Model
	Remotes                    []git.Remote
	RemoteIndex                int
	RemoteFilter               string
	SelectedRemote             string
	RefOptions                 []string
	RefIndex                   int
	RefFilter                  string
	CompareFrom                string
	CompareTo                  string
	CompareToOptions           []string
	CompareToIndex             int
	CompareToFilter            string
	Commits                    []git.Commit
	CommitIndex                int
	CommitFilter               string
	CommitRangeAnchor          int
	CheckoutOptions            []string
	CheckoutIndex              int
	CheckoutFilter             string
	CherryPickIndex            int
	CherryPickCommit           git.Commit
	CherryPickErr              error
	CherryPickValidationIndex  int
	CherryPickValidationCommit git.Commit
	CherryPickValidationErr    error
	NewBranchName              string
	NewBranchBase              string
	NewBranchBaseFilter        string
	BranchName                 string
	PushRemote                 string
	TagName                    string
	ReleaseName                string
	ReleaseNotesPath           string
	Err                        error
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
	compareTo   string
	commits     []git.Commit
	err         error
}

type compareToOptionsLoadedMsg struct {
	token       int
	compareFrom string
	options     []string
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

type cherryPickValidationMsg struct {
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
			m.loadingStatus = m.loadingRuntimeStatus()
			persist = false
			return m, cmd
		case remoteOptionsLoadedMsg:
			if msg.token != m.loadingToken {
				persist = false
				return m, nil
			}
			m.finishLoading()
			if msg.err != nil {
				m.Err = msg.err
				return m, nil
			}
			m.SelectedRemote = msg.remote
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
			m.finishLoading()
			if msg.err != nil {
				m.Err = msg.err
				return m, nil
			}
			m.CompareFrom = msg.compareFrom
			m.CompareTo = msg.compareTo
			m.Commits = msg.commits
			m.CommitIndex = 0
			m.CommitRangeAnchor = 0
			m.CommitFilter = ""
			m.setScreen("commit_select")
			return m, nil
		case compareToOptionsLoadedMsg:
			if msg.token != m.loadingToken {
				persist = false
				return m, nil
			}
			m.finishLoading()
			if msg.err != nil {
				m.Err = msg.err
				return m, nil
			}
			m.CompareFrom = msg.compareFrom
			m.CompareToOptions = msg.options
			m.CompareToIndex = 0
			if len(m.CompareToOptions) > 0 {
				m.CompareTo = m.CompareToOptions[0]
			}
			m.setScreen("compare_to_select")
			return m, nil
		case checkoutOptionsLoadedMsg:
			if msg.token != m.loadingToken {
				persist = false
				return m, nil
			}
			m.finishLoading()
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
			m.finishLoading()
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
			m.finishLoading()
			m.CherryPickIndex = msg.index
			m.CherryPickCommit = msg.commit
			m.CherryPickErr = msg.err
			m.Err = nil
			m.setScreen("cherry_pick_conflict")
			return m, nil
		case cherryPickValidationMsg:
			if msg.token != m.loadingToken {
				persist = false
				return m, nil
			}
			m.finishLoading()
			m.CherryPickIndex = msg.index
			m.CherryPickCommit = msg.commit
			m.CherryPickValidationErr = msg.err
			m.CherryPickErr = nil
			m.Err = nil
			m.setScreen("cherry_pick_validation")
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
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		persist = false
		return m, nil
	case tea.KeyMsg:
		if m.isFilterScreen() && !msg.Alt {
			if msg.Type == tea.KeyEsc {
				if m.currentFilter() != "" {
					m.setCurrentFilter("")
					m.ensureVisibleFocus()
					return m, nil
				}
			}
			if msg.Type == tea.KeyBackspace || msg.Type == tea.KeyDelete {
				if current := m.currentFilter(); current != "" {
					m.setCurrentFilter(deleteLastRune(current))
					m.ensureVisibleFocus()
					return m, nil
				}
			}
			if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
				m.setCurrentFilter(m.currentFilter() + msg.String())
				m.ensureVisibleFocus()
				return m, nil
			}
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
				m.SelectedRemote = remote
				token := m.beginLoading(
					fmt.Sprintf("Fetching %s and loading refs...", remote),
					fmt.Sprintf("git fetch %s --tags --prune", remote),
				)
				return m, tea.Batch(m.spinner.Tick, m.loadRemoteOptionsCmd(token, remote))
			}
			if m.screen == "ref_select" {
				if len(m.RefOptions) == 0 {
					m.Err = errors.New("no compare refs found")
					return m, nil
				}
				compareFrom := m.selectedRef()
				token := m.beginLoading(
					fmt.Sprintf("Loading compare targets for %s...", compareFrom),
					"git for-each-ref --format=%(refname:short) refs/heads && git tag --list",
				)
				return m, tea.Batch(m.spinner.Tick, m.loadCompareToOptionsCmd(token, compareFrom))
			}
			if m.screen == "compare_to_select" {
				if len(m.CompareToOptions) == 0 {
					m.Err = errors.New("no compare targets found")
					return m, nil
				}
				compareTo := m.selectedCompareTo()
				token := m.beginLoading(
					fmt.Sprintf("Loading commits from %s...%s", m.CompareFrom, compareTo),
					fmt.Sprintf("git log --pretty=format:%%H%%x09%%h%%x09%%an%%x09%%ad%%x09%%s --date=short %s...%s", m.CompareFrom, compareTo),
				)
				return m, tea.Batch(m.spinner.Tick, m.loadCommitsCmd(token, m.CompareFrom, compareTo))
			}
			if m.screen == "commit_select" {
				token := m.beginLoading(
					"Loading checkout targets...",
					"git for-each-ref --format=%(refname:short) refs/heads && git tag --list",
				)
				return m, tea.Batch(m.spinner.Tick, m.loadCheckoutOptionsCmd(token))
			}
			if m.screen == "checkout_select" {
				if m.selectedCheckoutOption() == createNewBranchOption {
					m.NewBranchBase = ""
					m.NewBranchName = ""
					m.Err = nil
					m.CheckoutIndex = 0
					m.setScreen("new_branch_base_select")
					return m, nil
				}
				if err := m.handleCheckoutEnter(); err != nil {
					m.Err = err
					return m, nil
				}
				return m.continueAfterCheckout()
			}
			if m.screen == "new_branch_base_select" {
				if len(m.CheckoutOptions) == 0 {
					m.Err = errors.New("no branch or tag bases found")
					return m, nil
				}
				m.NewBranchBase = m.selectedCheckoutBase()
				m.NewBranchName = ""
				m.Err = nil
				m.setScreen("new_branch_name_select")
				return m, nil
			}
			if m.screen == "new_branch_name_select" {
				if err := m.handleNewBranchEnter(); err != nil {
					m.Err = err
					return m, nil
				}
				return m.continueAfterCheckout()
			}
			if m.screen == "cherry_pick_progress" {
				token := m.beginLoading(
					"Applying selected commits...",
					"git cherry-pick <selected commits>",
				)
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
				token := m.beginLoading(
					"Applying selected commits...",
					"git cherry-pick <selected commits>",
				)
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
			if m.screen == "cherry_pick_validation" {
				m.Err = nil
				m.CherryPickValidationErr = nil
				m.setScreen("versioning")
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
			if m.screen == "commit_select" {
				m.moveCommitFocus(-1)
			}
			if m.screen == "remote_select" {
				m.moveRemoteFocus(-1)
			}
			if m.screen == "ref_select" {
				m.moveRefFocus(-1)
			}
			if m.screen == "compare_to_select" {
				m.moveCompareToFocus(-1)
			}
			if m.screen == "checkout_select" {
				m.moveCheckoutFocus(-1)
			}
			if m.screen == "new_branch_base_select" {
				m.moveNewBranchBaseFocus(-1)
			}
		case tea.KeyDown:
			if m.screen == "remote_select" && len(m.Remotes) > 0 {
				m.moveRemoteFocus(1)
			}
			if m.screen == "ref_select" && len(m.RefOptions) > 0 {
				m.moveRefFocus(1)
			}
			if m.screen == "commit_select" {
				m.moveCommitFocus(1)
			}
			if m.screen == "checkout_select" {
				m.moveCheckoutFocus(1)
			}
			if m.screen == "compare_to_select" {
				m.moveCompareToFocus(1)
			}
			if m.screen == "new_branch_base_select" {
				m.moveNewBranchBaseFocus(1)
			}
		case tea.KeySpace:
			if m.screen == "commit_select" && len(m.Commits) > 0 {
				m.toggleFocusedCommit()
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
		case tea.KeyShiftUp:
			if m.screen == "commit_select" {
				m.moveCommitFocus(-1)
				m.selectCommitRangeToFocus()
			}
		case tea.KeyShiftDown:
			if m.screen == "commit_select" {
				m.moveCommitFocus(1)
				m.selectCommitRangeToFocus()
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
			if m.screen == "new_branch_name_select" {
				m.NewBranchName = deleteLastRune(m.NewBranchName)
				return m, nil
			}
		case tea.KeyRunes:
			if msg.Alt {
				switch msg.String() {
				case "alt+a":
					if m.screen == "commit_select" {
						for _, idx := range m.filteredCommitIndexes() {
							m.Commits[idx].Selected = true
						}
					}
				case "alt+d":
					if m.screen == "commit_select" {
						for _, idx := range m.filteredCommitIndexes() {
							m.Commits[idx].Selected = false
						}
					}
				}
				return m, nil
			}
			switch msg.String() {
			case "e":
				if m.screen == "versioning" || m.screen == "cherry_pick_validation" {
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
				if m.screen == "cherry_pick_validation" {
					if err := m.runValidation(context.Background()); err != nil {
						m.Err = err
						return m, nil
					}
					return m.resumeCherryPickAfterValidation()
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
					token := m.beginLoading(
						"Applying selected commits...",
						"git cherry-pick <selected commits>",
					)
					return m, tea.Batch(m.spinner.Tick, m.applySelectedCommitsCmd(token, 0))
				}
				if m.screen == "cherry_pick_validation" {
					return m.resumeCherryPickAfterValidation()
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
					token := m.beginLoading(
						"Applying selected commits...",
						"git cherry-pick <selected commits>",
					)
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
				if m.screen == "cherry_pick_validation" {
					m.CherryPickValidationErr = nil
					m.CherryPickErr = nil
					m.setScreen("versioning")
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
			if m.screen == "new_branch_name_select" {
				m.NewBranchName += msg.String()
				return m, nil
			}
		default:
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
			[]string{"esc  back", "ctrl+c  quit"},
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
			[]string{"enter  start", "ctrl+c  quit"},
		)
	case "remote_select":
		if len(m.Remotes) == 0 {
			return renderScreen(
				"Select remote",
				m.listSubtitle("Pick the remote to fetch before comparing refs.", m.RemoteFilter),
				"No remotes found.",
				[]string{"ctrl+c  quit"},
			)
		}
		filtered := filteredStringIndexes(m.remoteOptionNames(), m.RemoteFilter)
		if len(filtered) == 0 {
			return renderScreen(
				"Select remote",
				m.listSubtitle("Pick the remote to fetch before comparing refs.", m.RemoteFilter),
				"No remotes match the current filter.",
				[]string{"type  filter", "esc  clear", "ctrl+c  quit"},
			)
		}
		rows := make([]string, 0, len(filtered))
		focus := indexOfInt(filtered, m.RemoteIndex)
		for pos, idx := range filtered {
			rows = append(rows, renderRow(pos == focus, fmt.Sprintf("  %s", m.Remotes[idx].Name)))
		}
		return renderScreen(
			"Select remote",
			m.listSubtitle("Pick the remote to fetch before comparing refs.", m.RemoteFilter),
			m.renderVisibleRows(rows, focus),
			[]string{"↑↓  move", "enter  continue", "type  filter", "ctrl+c  quit"},
		)
	case "ref_select":
		if len(m.RefOptions) == 0 {
			return renderScreen(
				"Select compare base",
				m.listSubtitle(fmt.Sprintf("Remote: %s", m.compareRemoteName()), m.RefFilter),
				"No compare refs found.",
				[]string{"ctrl+c  quit"},
			)
		}
		filtered := filteredStringIndexes(m.RefOptions, m.RefFilter)
		if len(filtered) == 0 {
			return renderScreen(
				"Select compare base",
				m.listSubtitle(fmt.Sprintf("Remote: %s", m.compareRemoteName()), m.RefFilter),
				"No compare refs match the current filter.",
				[]string{"type  filter", "esc  clear", "ctrl+c  quit"},
			)
		}
		rows := make([]string, 0, len(filtered))
		focus := indexOfInt(filtered, m.RefIndex)
		for pos, idx := range filtered {
			rows = append(rows, renderRow(pos == focus, fmt.Sprintf("  %s", m.RefOptions[idx])))
		}
		return renderScreen(
			"Select compare base",
			m.listSubtitle(fmt.Sprintf("Remote: %s", m.compareRemoteName()), m.RefFilter),
			m.renderVisibleRows(rows, focus),
			[]string{"↑↓  move", "enter  continue", "type  filter", "ctrl+c  quit"},
		)
	case "compare_to_select":
		if len(m.CompareToOptions) == 0 {
			return renderScreen(
				"Select compare target",
				m.listSubtitle(fmt.Sprintf("Compare from: %s", m.CompareFrom), m.CompareToFilter),
				"No compare targets found.",
				[]string{"ctrl+c  quit"},
			)
		}
		filtered := filteredStringIndexes(m.CompareToOptions, m.CompareToFilter)
		if len(filtered) == 0 {
			return renderScreen(
				"Select compare target",
				m.listSubtitle(fmt.Sprintf("Compare from: %s", m.CompareFrom), m.CompareToFilter),
				"No compare targets match the current filter.",
				[]string{"type  filter", "esc  clear", "ctrl+c  quit"},
			)
		}
		rows := make([]string, 0, len(filtered))
		focus := indexOfInt(filtered, m.CompareToIndex)
		for pos, idx := range filtered {
			rows = append(rows, renderRow(pos == focus, fmt.Sprintf("  %s", m.CompareToOptions[idx])))
		}
		return renderScreen(
			"Select compare target",
			m.listSubtitle(fmt.Sprintf("Compare from: %s", m.CompareFrom), m.CompareToFilter),
			m.renderVisibleRows(rows, focus),
			[]string{"↑↓  move", "enter  continue", "type  filter", "ctrl+c  quit"},
		)
	case "commit_select":
		filtered := m.filteredCommitIndexes()
		if len(m.Commits) == 0 {
			return renderScreen(
				"Select commits",
				m.listSubtitle(fmt.Sprintf("Selected: %d/%d", m.selectedCommitCount(), len(m.Commits)), m.CommitFilter),
				"No commits found.",
				[]string{"ctrl+c  quit"},
			)
		}
		if len(filtered) == 0 {
			return renderScreen(
				"Select commits",
				m.listSubtitle(fmt.Sprintf("Selected: %d/%d", m.selectedCommitCount(), len(m.Commits)), m.CommitFilter),
				"No commits match the current filter.",
				[]string{"type  filter", "esc  clear", "ctrl+c  quit"},
			)
		}
		rows := make([]string, 0, len(filtered))
		focus := indexOfInt(filtered, m.CommitIndex)
		for pos, idx := range filtered {
			commit := m.Commits[idx]
			check := "☐"
			if commit.Selected {
				check = "☑"
			}
			line := fmt.Sprintf("%s %s %s%s%s", check, commit.ShortSHA, commit.Title, m.commitPRSuffix(commit), m.commitLabelSuffix(commit))
			rows = append(rows, renderRow(pos == focus, "  "+line))
		}
		filterText := "Filter: (off)"
		if strings.TrimSpace(m.CommitFilter) != "" {
			filterText = fmt.Sprintf("Filter: %s", m.CommitFilter)
		}
		return renderScreen(
			"Select commits",
			fmt.Sprintf("Selected: %d/%d\n%s", m.selectedCommitCount(), len(m.Commits), filterText),
			m.renderVisibleRows(rows, focus),
			[]string{"space  toggle", "ctrl+a  all", "ctrl+d  none", "alt+a  visible all", "alt+d  visible none", "shift+↑/↓  range", "ctrl+enter  open PR", "type  filter", "enter  continue", "ctrl+c  quit"},
		)
	case "checkout_select":
		if len(m.CheckoutOptions) == 0 {
			return renderScreen(
				"Select checkout target",
				"Choose the branch or tag to apply commits on.",
				"No checkout targets found.",
				[]string{"ctrl+c  quit"},
			)
		}
		filtered := filteredStringIndexes(m.CheckoutOptions, m.CheckoutFilter)
		rows := make([]string, 0, len(filtered)+1)
		focus := indexOfInt(filtered, m.CheckoutIndex)
		for pos, idx := range filtered {
			rows = append(rows, renderRow(pos == focus, fmt.Sprintf("  %s", m.CheckoutOptions[idx])))
		}
		specialFocus := m.CheckoutIndex == len(m.CheckoutOptions)
		rows = append(rows, renderRow(specialFocus, fmt.Sprintf("  %s", createNewBranchOption)))
		if specialFocus {
			focus = len(rows) - 1
		}
		return renderScreen(
			"Select checkout target",
			m.listSubtitle("Choose the branch or tag to apply commits on.", m.CheckoutFilter),
			m.renderVisibleRows(rows, focus),
			[]string{"↑↓  move", "enter  checkout", "type  filter", "ctrl+c  quit"},
		)
	case "new_branch_base_select":
		if len(m.CheckoutOptions) == 0 {
			return renderScreen(
				"Select new branch base",
				"No branch or tag bases found.",
				"",
				[]string{"ctrl+c  quit"},
			)
		}
		filtered := filteredStringIndexes(m.CheckoutOptions, m.NewBranchBaseFilter)
		if len(filtered) == 0 {
			return renderScreen(
				"Select new branch base",
				m.listSubtitle("Pick the branch or tag to branch from.", m.NewBranchBaseFilter),
				"No branch or tag bases match the current filter.",
				[]string{"esc  back", "type  filter", "ctrl+c  quit"},
			)
		}
		rows := make([]string, 0, len(filtered))
		focus := indexOfInt(filtered, m.CheckoutIndex)
		for pos, idx := range filtered {
			rows = append(rows, renderRow(pos == focus, fmt.Sprintf("  %s", m.CheckoutOptions[idx])))
		}
		return renderScreen(
			"Select new branch base",
			m.listSubtitle("Pick the branch or tag to branch from.", m.NewBranchBaseFilter),
			m.renderVisibleRows(rows, focus),
			[]string{"↑↓  move", "enter  continue", "esc  back", "type  filter", "ctrl+c  quit"},
		)
	case "new_branch_name_select":
		return renderScreen(
			"Create branch",
			fmt.Sprintf("Base ref: %s", m.NewBranchBase),
			fmt.Sprintf("Branch name: %s\n\nType to edit the branch name, then press enter.", m.NewBranchName),
			[]string{"type  edit", "backspace  delete", "enter  create", "esc  back", "ctrl+c  quit"},
		)
	case "cherry_pick_validation":
		validationText := "The post cherry-pick command failed."
		if m.CherryPickValidationErr != nil {
			validationText = validationText + "\n\n" + renderError(m.CherryPickValidationErr)
		}
		if strings.TrimSpace(m.CherryPickCommit.ShortSHA) != "" || strings.TrimSpace(m.CherryPickCommit.Title) != "" {
			validationText += fmt.Sprintf("\n\n%s %s", m.CherryPickCommit.ShortSHA, m.CherryPickCommit.Title)
		}
		return renderScreen(
			"Validation failed",
			"Run the editor, retry, continue anyway, or abort.",
			validationText,
			[]string{"e  editor", "r  retry", "c  continue", "a  abort", "esc  back"},
		)
	case "cherry_pick_progress":
		return renderScreen(
			"Cherry-pick",
			"Applying selected commits to the checkout target.",
			"Loading overlay shows the current operation while Git works.",
			[]string{"esc  back", "ctrl+c  quit"},
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
			[]string{"enter  push", "esc  back", "ctrl+c  quit"},
		)
	case "tag_select":
		return renderScreen(
			"Tag",
			"Create and push the release tag.",
			fmt.Sprintf("Tag name: %s\n\nType to edit the tag name, then press enter.", m.TagName),
			[]string{"type  edit", "backspace  delete", "enter  tag", "esc  back", "ctrl+c  quit"},
		)
	case "release_select":
		return renderScreen(
			"Release",
			"Generate release notes and create the GitHub draft.",
			fmt.Sprintf("Release title: %s\nTag: %s\n\nType to edit the release title, then press enter.", m.releaseTitle(), m.TagName),
			[]string{"type  edit", "backspace  delete", "enter  release", "esc  back", "ctrl+c  quit"},
		)
	case "done":
		return renderScreen(
			"Done",
			"UI flow complete.",
			"All requested steps finished.",
			[]string{"ctrl+c  quit"},
		)
	default:
		return renderScreen(
			"Patchflow",
			"",
			"Press ctrl+c to quit.",
			[]string{"ctrl+c  quit"},
		)
	}
}

func (m Model) renderLoadingView() string {
	parts := []string{strings.TrimSpace(m.spinner.View())}
	if strings.TrimSpace(m.loadingMessage) != "" {
		parts = append(parts, m.loadingMessage)
	}
	if strings.TrimSpace(m.loadingStatus) != "" {
		parts = append(parts, MutedStyle.Render(m.loadingStatus))
	}
	parts = append(parts, renderCompactTerminal("Command", m.loadingCommand))
	return renderScreen(
		"Loading",
		"Press esc to cancel.",
		strings.Join(parts, "\n\n"),
		[]string{"esc  back", "ctrl+c  quit"},
	)
}

func renderCompactTerminal(title, command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		command = "(idle)"
	}
	lines := []string{
		SectionStyle.Render(title),
		"",
		command,
	}
	return PanelStyle.Render(strings.Join(lines, "\n"))
}

func newLoadingSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return s
}

func (m *Model) beginLoading(message, command string) int {
	m.loading = true
	m.loadingMessage = message
	m.loadingCommand = command
	m.loadingStatus = "Starting..."
	m.loadingStartedAt = time.Now()
	m.Err = nil
	m.spinner = newLoadingSpinner()
	m.loadingToken++
	return m.loadingToken
}

func (m *Model) cancelLoading() {
	m.loading = false
	m.loadingMessage = ""
	m.loadingCommand = ""
	m.loadingStatus = ""
	m.loadingStartedAt = time.Time{}
	m.loadingToken++
}

func (m *Model) finishLoading() {
	m.loading = false
	m.loadingMessage = ""
	m.loadingCommand = ""
	m.loadingStatus = ""
	m.loadingStartedAt = time.Time{}
}

func (m Model) loadingRuntimeStatus() string {
	if !m.loading || m.loadingStartedAt.IsZero() {
		return ""
	}
	return fmt.Sprintf("Running for %s", formatDuration(time.Since(m.loadingStartedAt)))
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		seconds := d.Seconds()
		if seconds < 10 {
			return fmt.Sprintf("%.1fs", seconds)
		}
		return fmt.Sprintf("%ds", int(seconds))
	}
	minutes := int(d / time.Minute)
	seconds := int((d % time.Minute) / time.Second)
	return fmt.Sprintf("%dm%02ds", minutes, seconds)
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

func (m Model) loadCompareToOptionsCmd(token int, compareFrom string) tea.Cmd {
	return func() tea.Msg {
		options, err := m.loadCompareTargets(context.Background())
		return compareToOptionsLoadedMsg{token: token, compareFrom: compareFrom, options: options, err: err}
	}
}

func (m Model) loadCommitsCmd(token int, compareFrom, compareTo string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		commits, err := m.Git.LogRange(ctx, m.WorkDir, compareFrom, compareTo)
		if err == nil {
			commits = m.enrichCommitsWithPRs(commits)
		}
		return commitsLoadedMsg{token: token, compareFrom: compareFrom, compareTo: compareTo, commits: commits, err: err}
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

func (m Model) compareRemoteName() string {
	if strings.TrimSpace(m.SelectedRemote) != "" {
		return m.SelectedRemote
	}
	return m.selectedRemote()
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
	return msg.String() == "ctrl+enter"
}

func (m *Model) handleCheckoutEnter() error {
	target := m.selectedCheckoutOption()
	if target == createNewBranchOption {
		return nil
	}
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
	if m.CheckoutIndex == len(m.CheckoutOptions) {
		return createNewBranchOption
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

func (m Model) loadCompareTargets(ctx context.Context) ([]string, error) {
	branches, err := m.Git.LocalBranches(ctx, m.WorkDir)
	if err != nil {
		return nil, err
	}
	tags, err := m.Git.Tags(ctx, m.WorkDir)
	if err != nil {
		return nil, err
	}
	return mergeUnique([]string{"HEAD"}, branches, tags), nil
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
			if strings.TrimSpace(m.Config.PostCherryPickCmd) != "" {
				if err := runShellCommand(context.Background(), m.Git.Runner, m.Logger, m.WorkDir, m.Config.PostCherryPickCmd); err != nil {
					return cherryPickValidationMsg{
						token:  token,
						index:  selectedIndex,
						commit: commits[i],
						err:    err,
					}
				}
			}
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
	m.loadingCommand = ""
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

func (m *Model) resumeCherryPickAfterValidation() (tea.Model, tea.Cmd) {
	m.CherryPickValidationErr = nil
	m.CherryPickErr = nil
	if len(m.selectedCommits()) == 0 {
		m.setScreen("versioning")
		return Model(*m), nil
	}
	m.setScreen("cherry_pick_progress")
	token := m.beginLoading(
		"Applying selected commits...",
		"git cherry-pick <selected commits>",
	)
	return Model(*m), tea.Batch(m.spinner.Tick, m.applySelectedCommitsCmd(token, m.CherryPickIndex+1))
}

func (m *Model) continueAfterCheckout() (tea.Model, tea.Cmd) {
	selected := m.selectedCommits()
	if len(selected) == 0 {
		m.Err = nil
		m.setScreen("versioning")
		return Model(*m), nil
	}
	m.setScreen("cherry_pick_progress")
	token := m.beginLoading(
		"Applying selected commits...",
		"git cherry-pick <selected commits>",
	)
	return Model(*m), tea.Batch(m.spinner.Tick, m.applySelectedCommitsCmd(token, 0))
}

func (m Model) selectedCompareTo() string {
	if len(m.CompareToOptions) == 0 {
		return "HEAD"
	}
	if m.CompareToIndex < 0 || m.CompareToIndex >= len(m.CompareToOptions) {
		return m.CompareToOptions[0]
	}
	return m.CompareToOptions[m.CompareToIndex]
}

func (m Model) selectedCheckoutBase() string {
	if len(m.CheckoutOptions) == 0 {
		return ""
	}
	if m.CheckoutIndex < 0 || m.CheckoutIndex >= len(m.CheckoutOptions) {
		return m.CheckoutOptions[0]
	}
	return m.CheckoutOptions[m.CheckoutIndex]
}

func (m *Model) handleNewBranchEnter() error {
	base := strings.TrimSpace(m.NewBranchBase)
	if base == "" {
		return errors.New("no branch base selected")
	}
	branch := strings.TrimSpace(m.NewBranchName)
	if branch == "" {
		return errors.New("new branch name is required")
	}
	exists, err := m.Git.BranchExists(context.Background(), m.WorkDir, branch)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("branch %s already exists", branch)
	}
	if _, err := m.runLogged(context.Background(), "git", "checkout", "-b", branch, base); err != nil {
		return err
	}
	m.BranchName = branch
	return nil
}

func (m Model) isFilterScreen() bool {
	switch m.screen {
	case "remote_select", "ref_select", "compare_to_select", "commit_select", "checkout_select", "new_branch_base_select":
		return true
	default:
		return false
	}
}

func (m Model) currentFilter() string {
	switch m.screen {
	case "remote_select":
		return m.RemoteFilter
	case "ref_select":
		return m.RefFilter
	case "compare_to_select":
		return m.CompareToFilter
	case "commit_select":
		return m.CommitFilter
	case "checkout_select":
		return m.CheckoutFilter
	case "new_branch_base_select":
		return m.NewBranchBaseFilter
	default:
		return ""
	}
}

func (m *Model) setCurrentFilter(value string) {
	switch m.screen {
	case "remote_select":
		m.RemoteFilter = value
	case "ref_select":
		m.RefFilter = value
	case "compare_to_select":
		m.CompareToFilter = value
	case "commit_select":
		m.CommitFilter = value
	case "checkout_select":
		m.CheckoutFilter = value
	case "new_branch_base_select":
		m.NewBranchBaseFilter = value
	}
}

func (m *Model) ensureVisibleFocus() {
	switch m.screen {
	case "remote_select":
		m.RemoteIndex = ensureFilteredIndex(filteredStringIndexes(m.remoteOptionNames(), m.RemoteFilter), m.RemoteIndex)
	case "ref_select":
		m.RefIndex = ensureFilteredIndex(filteredStringIndexes(m.RefOptions, m.RefFilter), m.RefIndex)
	case "compare_to_select":
		m.CompareToIndex = ensureFilteredIndex(filteredStringIndexes(m.CompareToOptions, m.CompareToFilter), m.CompareToIndex)
	case "commit_select":
		m.ensureVisibleCommitFocus()
	case "checkout_select":
		m.CheckoutIndex = ensureCheckoutIndex(m, m.CheckoutIndex)
	case "new_branch_base_select":
		m.CheckoutIndex = ensureFilteredIndex(filteredStringIndexes(m.CheckoutOptions, m.NewBranchBaseFilter), m.CheckoutIndex)
	}
}

func (m Model) remoteOptionNames() []string {
	names := make([]string, 0, len(m.Remotes))
	for _, remote := range m.Remotes {
		names = append(names, remote.Name)
	}
	return names
}

func (m *Model) moveRemoteFocus(delta int) {
	indexes := filteredStringIndexes(m.remoteOptionNames(), m.RemoteFilter)
	m.RemoteIndex = moveFilteredIndex(indexes, m.RemoteIndex, delta)
}

func (m *Model) moveRefFocus(delta int) {
	indexes := filteredStringIndexes(m.RefOptions, m.RefFilter)
	m.RefIndex = moveFilteredIndex(indexes, m.RefIndex, delta)
}

func (m *Model) moveCompareToFocus(delta int) {
	indexes := filteredStringIndexes(m.CompareToOptions, m.CompareToFilter)
	m.CompareToIndex = moveFilteredIndex(indexes, m.CompareToIndex, delta)
}

func (m *Model) moveCheckoutFocus(delta int) {
	m.CheckoutIndex = ensureCheckoutIndexMove(m, delta)
}

func (m *Model) moveNewBranchBaseFocus(delta int) {
	indexes := filteredStringIndexes(m.CheckoutOptions, m.NewBranchBaseFilter)
	m.CheckoutIndex = moveFilteredIndex(indexes, m.CheckoutIndex, delta)
}

func (m Model) listSubtitle(base, filter string) string {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return base
	}
	return fmt.Sprintf("%s\nFilter: %s", base, filter)
}

func (m Model) maxListBodyRows() int {
	if m.Height <= 0 {
		return 8
	}
	rows := m.Height - 10
	if rows < 3 {
		rows = 3
	}
	return rows
}

func (m Model) renderVisibleRows(rows []string, focus int) string {
	if len(rows) == 0 {
		return ""
	}
	limit := m.maxListBodyRows()
	if limit >= len(rows) {
		return strings.Join(rows, "\n")
	}
	if focus < 0 || focus >= len(rows) {
		focus = 0
	}
	start := focus - (limit / 2)
	if start < 0 {
		start = 0
	}
	if start+limit > len(rows) {
		start = len(rows) - limit
	}
	end := start + limit
	visible := make([]string, 0, limit+2)
	if start > 0 {
		visible = append(visible, renderDimRow(fmt.Sprintf("  ... %d more above", start)))
	}
	visible = append(visible, rows[start:end]...)
	if end < len(rows) {
		visible = append(visible, renderDimRow(fmt.Sprintf("  ... %d more below", len(rows)-end)))
	}
	return strings.Join(visible, "\n")
}

func filteredStringIndexes(items []string, query string) []int {
	return fuzzyMatchIndexes(query, items)
}

func ensureFilteredIndex(indexes []int, current int) int {
	if len(indexes) == 0 {
		return 0
	}
	if containsInt(indexes, current) {
		return current
	}
	return indexes[0]
}

func moveFilteredIndex(indexes []int, current, delta int) int {
	if len(indexes) == 0 {
		return 0
	}
	currentPos := indexOfInt(indexes, current)
	if currentPos < 0 {
		currentPos = 0
	}
	nextPos := currentPos + delta
	for nextPos < 0 {
		nextPos += len(indexes)
	}
	nextPos %= len(indexes)
	return indexes[nextPos]
}

func ensureCheckoutIndex(m *Model, current int) int {
	indexes := filteredStringIndexes(m.CheckoutOptions, m.CheckoutFilter)
	if current == len(m.CheckoutOptions) {
		return len(m.CheckoutOptions)
	}
	if len(indexes) == 0 {
		return 0
	}
	if containsInt(indexes, current) {
		return current
	}
	return indexes[0]
}

func ensureCheckoutIndexMove(m *Model, delta int) int {
	indexes := filteredStringIndexes(m.CheckoutOptions, m.CheckoutFilter)
	if len(indexes) == 0 {
		if delta >= 0 {
			return len(m.CheckoutOptions)
		}
		return 0
	}
	if m.CheckoutIndex == len(m.CheckoutOptions) {
		if delta < 0 {
			return indexes[len(indexes)-1]
		}
		return len(m.CheckoutOptions)
	}
	next := moveFilteredIndex(indexes, m.CheckoutIndex, delta)
	if delta > 0 && next == indexes[0] && m.CheckoutIndex == indexes[len(indexes)-1] {
		return len(m.CheckoutOptions)
	}
	if delta < 0 && m.CheckoutIndex == len(m.CheckoutOptions) {
		return indexes[len(indexes)-1]
	}
	return next
}

func (m Model) filteredCommitIndexes() []int {
	labels := make([]string, 0, len(m.Commits))
	for _, commit := range m.Commits {
		labels = append(labels, searchableCommit(commit))
	}
	return fuzzyMatchIndexes(m.CommitFilter, labels)
}

func searchableCommit(commit git.Commit) string {
	parts := []string{commit.SHA, commit.ShortSHA, commit.Title, commit.PRTitle}
	if commit.PRNumber > 0 {
		parts = append(parts, fmt.Sprintf("%d", commit.PRNumber))
	}
	parts = append(parts, commit.Labels...)
	return strings.Join(parts, " ")
}

func (m *Model) ensureVisibleCommitFocus() {
	indexes := m.filteredCommitIndexes()
	if len(indexes) == 0 {
		m.CommitIndex = 0
		return
	}
	if !containsInt(indexes, m.CommitIndex) {
		m.CommitIndex = indexes[0]
	}
	if !containsInt(indexes, m.CommitRangeAnchor) {
		m.CommitRangeAnchor = m.CommitIndex
	}
}

func (m *Model) moveCommitFocus(delta int) {
	indexes := m.filteredCommitIndexes()
	if len(indexes) == 0 {
		return
	}
	currentPos := indexOfInt(indexes, m.CommitIndex)
	if currentPos < 0 {
		currentPos = 0
	}
	nextPos := currentPos + delta
	for nextPos < 0 {
		nextPos += len(indexes)
	}
	nextPos = nextPos % len(indexes)
	m.CommitIndex = indexes[nextPos]
	if m.CommitRangeAnchor < 0 || !containsInt(indexes, m.CommitRangeAnchor) {
		m.CommitRangeAnchor = m.CommitIndex
	}
}

func (m *Model) toggleFocusedCommit() {
	indexes := m.filteredCommitIndexes()
	if len(indexes) == 0 {
		return
	}
	current := m.CommitIndex
	if !containsInt(indexes, current) {
		current = indexes[0]
		m.CommitIndex = current
	}
	m.Commits[current].Selected = !m.Commits[current].Selected
	m.CommitRangeAnchor = current
}

func (m *Model) selectCommitRangeToFocus() {
	indexes := m.filteredCommitIndexes()
	if len(indexes) == 0 {
		return
	}
	currentPos := indexOfInt(indexes, m.CommitIndex)
	if currentPos < 0 {
		currentPos = 0
		m.CommitIndex = indexes[0]
	}
	anchorPos := indexOfInt(indexes, m.CommitRangeAnchor)
	if anchorPos < 0 {
		anchorPos = currentPos
	}
	if anchorPos > currentPos {
		anchorPos, currentPos = currentPos, anchorPos
	}
	for i := anchorPos; i <= currentPos; i++ {
		m.Commits[indexes[i]].Selected = true
	}
}

func containsInt(items []int, value int) bool {
	return indexOfInt(items, value) >= 0
}

func indexOfInt(items []int, value int) int {
	for i, item := range items {
		if item == value {
			return i
		}
	}
	return -1
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
	if err := m.validateTagAvailability(context.Background(), m.PushRemote, m.TagName); err != nil {
		return err
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

func (m Model) validateTagAvailability(ctx context.Context, remote, tag string) error {
	exists, err := m.Git.TagExists(ctx, m.WorkDir, tag)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("tag %s already exists locally", tag)
	}
	remoteExists, err := m.Git.RemoteTagExists(ctx, m.WorkDir, remote, tag)
	if err != nil {
		return err
	}
	if remoteExists {
		return fmt.Errorf("tag %s already exists on %s", tag, remote)
	}
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
		Remote:          m.SelectedRemote,
		CompareFrom:     m.CompareFrom,
		CompareTo:       m.CompareTo,
		NewBranchName:   m.NewBranchName,
		NewBranchBase:   m.NewBranchBase,
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
	Remote          string        `json:"remote,omitempty"`
	CompareFrom     string        `json:"compareFrom,omitempty"`
	CompareTo       string        `json:"compareTo,omitempty"`
	NewBranchName   string        `json:"newBranchName,omitempty"`
	NewBranchBase   string        `json:"newBranchBase,omitempty"`
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
