package main

import (
	"context"
	"os"
	"strings"

	"patchflow/internal/config"
	"patchflow/internal/app"
)

func main() {
	repo, configPath, args := parseCLIArgs(os.Args[1:])
	opts := app.Options{
		Args:    args,
		WorkDir: repo,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Config:  configPath,
	}
	if err := app.New(opts).Run(context.Background()); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func parseCLIArgs(argv []string) (string, string, []string) {
	repo := "."
	configPath := config.DefaultConfigPath
	args := make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		switch {
		case arg == "--repo":
			if i+1 < len(argv) {
				repo = argv[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--repo="):
			repo = strings.TrimPrefix(arg, "--repo=")
		case arg == "--config":
			if i+1 < len(argv) {
				configPath = argv[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
		default:
			args = append(args, arg)
		}
	}
	if repo == "" {
		repo = "."
	}
	if configPath == "" {
		configPath = config.DefaultConfigPath
	}
	return repo, configPath, args
}
