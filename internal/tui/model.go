package tui

import (
	"context"
	"fmt"
	"strings"

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

	screen string
	Remotes []git.Remote
	RemoteIndex int
	RefOptions []string
	RefIndex int
	CompareFrom string
	Commits []git.Commit
	CommitIndex int
	CheckoutOptions []string
	CheckoutIndex int
	NewBranchName string
	NewBranchBase string
	Err    error
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
	}
}

func (m Model) ScreenName() string {
	return m.screen
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.screen == "commit_select" && (msg.String() == "ctrl+enter" || msg.String() == "o") {
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
				m.screen = "remote_select"
				if len(m.Remotes) == 0 {
					m.Remotes = []git.Remote{{Name: m.Config.DefaultRemote}}
				}
				m.RemoteIndex = 0
				return m, nil
			}
			if m.screen == "remote_select" {
				remote := m.selectedRemote()
				if err := m.Git.Fetch(context.Background(), m.WorkDir, remote); err != nil {
					m.Err = err
					return m, nil
				}
				m.RefOptions = []string{remote + "/master", remote + "/main", "v1.2.0"}
				m.screen = "ref_select"
				m.RefIndex = 0
				return m, nil
			}
			if m.screen == "ref_select" {
				m.CompareFrom = m.selectedRef()
				commits, err := m.Git.LogRange(context.Background(), m.WorkDir, m.CompareFrom, "HEAD")
				if err != nil {
					m.Err = err
					return m, nil
				}
				commits = m.enrichCommitsWithPRs(commits)
				m.Commits = commits
				m.CommitIndex = 0
				m.screen = "commit_select"
				return m, nil
			}
			if m.screen == "commit_select" {
				m.screen = "checkout_select"
				m.CheckoutOptions = []string{"release/1.2.0", "Create new branch from tag"}
				m.CheckoutIndex = 0
				return m, nil
			}
			if m.screen == "checkout_select" {
				if m.CheckoutIndex == 0 {
					_ = m.Git.Runner
				}
				return m.handleCheckoutEnter()
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
	switch m.screen {
	case "welcome":
		return "Patchflow\n\nPress enter to start.\n"
	case "remote_select":
		var b strings.Builder
		b.WriteString("Patchflow\n\nSelect remote:\n\n")
		if len(m.Remotes) == 0 {
			fmt.Fprintf(&b, "> %s\n", strings.TrimSpace(m.Config.DefaultRemote))
			return b.String()
		}
		for i, remote := range m.Remotes {
			prefix := "  "
			if i == m.RemoteIndex {
				prefix = "> "
			}
			fmt.Fprintf(&b, "%s%s\n", prefix, remote.Name)
		}
		return b.String()
	case "ref_select":
		var b strings.Builder
		fmt.Fprintf(&b, "Patchflow\n\nSelect compare base:\n\nRemote: %s\n\n", m.selectedRemote())
		if len(m.RefOptions) == 0 {
			b.WriteString("> upstream/master\n")
			return b.String()
		}
		for i, ref := range m.RefOptions {
			prefix := "  "
			if i == m.RefIndex {
				prefix = "> "
			}
			fmt.Fprintf(&b, "%s%s\n", prefix, ref)
		}
		return b.String()
	case "commit_select":
		var b strings.Builder
		fmt.Fprintf(&b, "Patchflow\n\nSelect commits to cherry-pick\n\nSelected: %d/%d\n\n", m.selectedCommitCount(), len(m.Commits))
		if len(m.Commits) == 0 {
			b.WriteString("No commits found.\n")
			return b.String()
		}
		for i, commit := range m.Commits {
			prefix := "  "
			if i == m.CommitIndex {
				prefix = "> "
			}
			check := "[ ]"
			if commit.Selected {
				check = "[x]"
			}
			fmt.Fprintf(&b, "%s%s %s %s%s%s\n", prefix, check, commit.ShortSHA, commit.Title, m.commitPRSuffix(commit), m.commitLabelSuffix(commit))
		}
		return b.String()
	case "checkout_select":
		var b strings.Builder
		b.WriteString("Patchflow\n\nSelect checkout target:\n\n")
		if len(m.CheckoutOptions) == 0 {
			b.WriteString("> release/1.2.0\n")
			return b.String()
		}
		for i, option := range m.CheckoutOptions {
			prefix := "  "
			if i == m.CheckoutIndex {
				prefix = "> "
			}
			fmt.Fprintf(&b, "%s%s\n", prefix, option)
		}
		return b.String()
	case "done":
		return "Patchflow\n\nDone.\n"
	default:
		if m.Err != nil {
			return fmt.Sprintf("Patchflow\n\nError: %v\n\nPress q to quit.\n", m.Err)
		}
		return "Patchflow\n\nPress q to quit.\n"
	}
}

func (m Model) selectedRemote() string {
	if len(m.Remotes) == 0 {
		return strings.TrimSpace(m.Config.DefaultRemote)
	}
	if m.RemoteIndex < 0 || m.RemoteIndex >= len(m.Remotes) {
		return m.Remotes[0].Name
	}
	return m.Remotes[m.RemoteIndex].Name
}

func (m Model) selectedRef() string {
	if len(m.RefOptions) == 0 {
		return "upstream/master"
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

func (m Model) handleCheckoutEnter() (tea.Model, tea.Cmd) {
	target := m.selectedCheckoutOption()
	switch target {
	case "Create new branch from tag":
		if m.NewBranchName == "" {
			m.NewBranchName = "release/1.2.1"
		}
		if m.NewBranchBase == "" {
			m.NewBranchBase = "v1.2.0"
		}
		if _, err := m.Git.Runner.Run(context.Background(), m.WorkDir, "git", "checkout", "-b", m.NewBranchName, m.NewBranchBase); err != nil {
			m.Err = err
			return m, nil
		}
	default:
		if _, err := m.Git.Runner.Run(context.Background(), m.WorkDir, "git", "checkout", target); err != nil {
			m.Err = err
			return m, nil
		}
	}
	m.screen = "done"
	return m, nil
}

func (m Model) selectedCheckoutOption() string {
	if len(m.CheckoutOptions) == 0 {
		return "release/1.2.0"
	}
	if m.CheckoutIndex < 0 || m.CheckoutIndex >= len(m.CheckoutOptions) {
		return m.CheckoutOptions[0]
	}
	return m.CheckoutOptions[m.CheckoutIndex]
}
