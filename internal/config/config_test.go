package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReturnsDefaultsWhenFileMissing(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DefaultRemote != "upstream" {
		t.Fatalf("DefaultRemote = %q, want %q", cfg.DefaultRemote, "upstream")
	}
	if cfg.DefaultPushRemote != "origin" {
		t.Fatalf("DefaultPushRemote = %q, want %q", cfg.DefaultPushRemote, "origin")
	}
	if cfg.Editor != "code --wait" {
		t.Fatalf("Editor = %q, want %q", cfg.Editor, "code --wait")
	}
}

func TestLoadMergesYAMLConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".patchflow.yml")
	if err := os.WriteFile(path, []byte(`
default_remote: origin
editor: vim
versioning:
  suggested_files:
    - CUSTOM
`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DefaultRemote != "origin" {
		t.Fatalf("DefaultRemote = %q, want %q", cfg.DefaultRemote, "origin")
	}
	if cfg.Editor != "vim" {
		t.Fatalf("Editor = %q, want %q", cfg.Editor, "vim")
	}
	if len(cfg.Versioning.SuggestedFiles) != 1 || cfg.Versioning.SuggestedFiles[0] != "CUSTOM" {
		t.Fatalf("SuggestedFiles = %#v, want [CUSTOM]", cfg.Versioning.SuggestedFiles)
	}
}
