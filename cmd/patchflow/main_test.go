package main

import "testing"

func TestParseCLIArgsSupportsRepoAndConfigFlagsAnywhere(t *testing.T) {
	repo, configPath, args := parseCLIArgs([]string{"start", "--repo", "/tmp/other", "--config", "/tmp/other/.patchflow.yml"})
	if repo != "/tmp/other" {
		t.Fatalf("repo = %q, want /tmp/other", repo)
	}
	if configPath != "/tmp/other/.patchflow.yml" {
		t.Fatalf("configPath = %q, want /tmp/other/.patchflow.yml", configPath)
	}
	if len(args) != 1 || args[0] != "start" {
		t.Fatalf("args = %#v, want [start]", args)
	}
}

func TestParseCLIArgsDefaultsToCurrentRepo(t *testing.T) {
	repo, configPath, args := parseCLIArgs([]string{"doctor"})
	if repo != "." {
		t.Fatalf("repo = %q, want .", repo)
	}
	if configPath != ".patchflow.yml" {
		t.Fatalf("configPath = %q, want .patchflow.yml", configPath)
	}
	if len(args) != 1 || args[0] != "doctor" {
		t.Fatalf("args = %#v, want [doctor]", args)
	}
}
