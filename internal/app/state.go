package app

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type State struct {
	Remote          string       `json:"remote"`
	PushRemote      string       `json:"pushRemote"`
	CompareFrom     string       `json:"compareFrom"`
	CompareTo       string       `json:"compareTo"`
	CheckoutTarget  string       `json:"checkoutTarget"`
	BranchName      string       `json:"branchName"`
	TagName         string       `json:"tagName"`
	CurrentStep     string       `json:"currentStep"`
	CurrentCommit   string       `json:"currentCommit"`
	SelectedCommits []StateCommit `json:"selectedCommits"`
}

type StateCommit struct {
	SHA      string   `json:"sha"`
	ShortSHA string   `json:"shortSha"`
	Title    string   `json:"title"`
	PRNumber int      `json:"prNumber,omitempty"`
	PRTitle  string   `json:"prTitle,omitempty"`
	PRURL    string   `json:"prUrl,omitempty"`
	Labels   []string `json:"labels,omitempty"`
	Status   string   `json:"status"`
}

func LoadState(path string) (State, error) {
	var state State
	data, err := os.ReadFile(path)
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

func SaveState(path string, state State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
