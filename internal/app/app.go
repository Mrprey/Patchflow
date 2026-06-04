package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"patchflow/internal/browser"
	"patchflow/internal/config"
	"patchflow/internal/execx"
	"patchflow/internal/git"
	"patchflow/internal/github"
	"patchflow/internal/logging"
	"patchflow/internal/release"
	"patchflow/internal/tui"
)

type Options struct {
	Args    []string
	WorkDir string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Config  string
	Runner  execx.Runner
}

type App struct {
	args       []string
	workDir    string
	in         io.Reader
	out        io.Writer
	errOut     io.Writer
	cfg        config.Config
	cfgPath    string
	runner     execx.Runner
	newProgram func(tea.Model, ...tea.ProgramOption) teaProgram
}

func New(opts Options) *App {
	r := opts.Runner
	if r == nil {
		r = execx.CommandRunner{}
	}
	return &App{
		args:    opts.Args,
		workDir: opts.WorkDir,
		in:      opts.Stdin,
		out:     opts.Stdout,
		errOut:  opts.Stderr,
		cfgPath: opts.Config,
		runner:  r,
		newProgram: func(model tea.Model, opts ...tea.ProgramOption) teaProgram {
			return tea.NewProgram(model, opts...)
		},
	}
}

func (a *App) Run(ctx context.Context) error {
	if a.out == nil {
		a.out = os.Stdout
	}
	if a.errOut == nil {
		a.errOut = os.Stderr
	}
	cfg, err := config.Load(a.cfgPath)
	if err != nil {
		return err
	}
	a.cfg = cfg

	cmd := "start"
	if len(a.args) > 0 {
		cmd = a.args[0]
	}

	switch cmd {
	case "start":
		return a.runTUI()
	case "doctor":
		return a.runDoctor(ctx)
	case "config":
		return a.printConfig()
	case "resume":
		return a.resume()
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func (a *App) runTUI() error {
	p := a.newProgram(tui.New(tui.Options{
		WorkDir:    a.workDir,
		ConfigPath: a.cfgPath,
		Config:     a.cfg,
		Git:        a.GitClient(),
		GitHub:     a.GitHubClient(),
		Browser:    a.Browser(),
		Logger:     a.Logger(),
		Stdout:     a.out,
	}), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (a *App) runDoctor(ctx context.Context) error {
	gitClient := git.NewClient(a.runner)
	ghClient := github.NewClient(a.runner)

	top, err := gitClient.TopLevel(ctx, a.workDir)
	if err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			return errors.New("This directory is not a Git repository.")
		}
		return err
	}

	status, err := gitClient.StatusPorcelain(ctx, top)
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) != "" {
		return errors.New("Your working tree has uncommitted changes.")
	}

	if err := ghClient.AuthStatus(ctx); err != nil {
		return err
	}
	return nil
}

func (a *App) printConfig() error {
	data, err := yaml.Marshal(a.cfg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(a.out, string(data))
	return err
}

func (a *App) resume() error {
	path := a.resolvePath(".patchflow", "state.json")
	state, err := LoadState(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_, writeErr := fmt.Fprintln(a.out, "No saved state.")
			return writeErr
		}
		return err
	}
	_, err = fmt.Fprintf(a.out, "Current step: %s\n", state.CurrentStep)
	if err != nil {
		return err
	}
	if state.Remote != "" {
		if _, err = fmt.Fprintf(a.out, "Remote: %s\n", state.Remote); err != nil {
			return err
		}
	}
	if state.CompareFrom != "" || state.CompareTo != "" {
		if _, err = fmt.Fprintf(a.out, "Compare: %s...%s\n", state.CompareFrom, state.CompareTo); err != nil {
			return err
		}
	}
	if state.BranchName != "" {
		if _, err = fmt.Fprintf(a.out, "Branch: %s\n", state.BranchName); err != nil {
			return err
		}
	}
	if state.TagName != "" {
		if _, err = fmt.Fprintf(a.out, "Tag: %s\n", state.TagName); err != nil {
			return err
		}
	}
	if len(state.SelectedCommits) > 0 {
		if _, err = fmt.Fprintf(a.out, "Selected commits: %d\n", len(state.SelectedCommits)); err != nil {
			return err
		}
	}
	return err
}

func (a *App) GitClient() git.Client {
	return git.NewClient(a.runner)
}

func (a *App) GitHubClient() github.Client {
	return github.NewClient(a.runner)
}

func (a *App) Logger() logging.Logger {
	return logging.New(a.resolvePath(".patchflow", "logs"))
}

func (a *App) Browser() browser.Opener {
	return browser.New(a.runner)
}

func (a *App) BuildReleaseNotes(commits []git.Commit) string {
	return release.BuildNotes(commits)
}

type teaProgram interface {
	Run() (tea.Model, error)
}

func (a *App) resolvePath(parts ...string) string {
	all := append([]string{}, parts...)
	if a.workDir != "" {
		return filepath.Join(append([]string{a.workDir}, all...)...)
	}
	return filepath.Join(all...)
}
