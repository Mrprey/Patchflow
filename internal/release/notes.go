package release

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"patchflow/internal/git"
)

func BuildNotes(commits []git.Commit) string {
	var b strings.Builder
	b.WriteString("## Changes\n\n")
	for _, commit := range commits {
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
	for _, commit := range commits {
		fmt.Fprintf(&b, "- %s %s\n", commit.ShortSHA, commit.Title)
	}
	return b.String()
}

func WriteNotes(path string, notes string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(notes), 0o600)
}
