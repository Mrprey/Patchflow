package config

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

const DefaultConfigPath = ".patchflow.yml"

type Config struct {
	DefaultRemote       string       `yaml:"default_remote"`
	DefaultPushRemote    string       `yaml:"default_push_remote"`
	Editor               string       `yaml:"editor"`
	PostCherryPickCmd    string       `yaml:"post_cherry_pick_command"`
	ReleaseNotesFile     string       `yaml:"release_notes_file"`
	Versioning           Versioning   `yaml:"versioning"`
}

type Versioning struct {
	SuggestedFiles []string `yaml:"suggested_files"`
}

func Default() Config {
	return Config{
		DefaultRemote:    "upstream",
		DefaultPushRemote: "origin",
		Editor:           "code --wait",
		PostCherryPickCmd: "flutter analyze",
		ReleaseNotesFile:  ".patchflow/release-notes.md",
		Versioning: Versioning{
			SuggestedFiles: []string{
				"pubspec.yaml",
				"package.json",
				"CHANGELOG.md",
				"ios/Runner/Info.plist",
				"android/app/build.gradle",
			},
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	normalize(&cfg)
	return cfg, nil
}

func normalize(cfg *Config) {
	if cfg.DefaultRemote == "" {
		cfg.DefaultRemote = Default().DefaultRemote
	}
	if cfg.DefaultPushRemote == "" {
		cfg.DefaultPushRemote = Default().DefaultPushRemote
	}
	if cfg.Editor == "" {
		cfg.Editor = Default().Editor
	}
	if cfg.PostCherryPickCmd == "" {
		cfg.PostCherryPickCmd = Default().PostCherryPickCmd
	}
	if cfg.ReleaseNotesFile == "" {
		cfg.ReleaseNotesFile = Default().ReleaseNotesFile
	}
	if len(cfg.Versioning.SuggestedFiles) == 0 {
		cfg.Versioning.SuggestedFiles = Default().Versioning.SuggestedFiles
	}
}
