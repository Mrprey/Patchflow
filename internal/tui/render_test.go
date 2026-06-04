package tui

import (
	"regexp"
	"strings"
	"testing"
)

func TestRenderScreenAddsAppChrome(t *testing.T) {
	got := stripANSI(renderScreen("Select remote", "Remote: upstream", "origin\nupstream", []string{"enter  continue", "q  quit"}))
	if !strings.Contains(got, "PATCHFLOW") {
		t.Fatalf("renderScreen() = %q, want app title", got)
	}
	if !strings.Contains(got, "Select remote") || !strings.Contains(got, "Remote: upstream") {
		t.Fatalf("renderScreen() = %q, want title and subtitle", got)
	}
	if !strings.Contains(got, "origin") || !strings.Contains(got, "upstream") {
		t.Fatalf("renderScreen() = %q, want body", got)
	}
	if !strings.Contains(got, "enter  continue") || !strings.Contains(got, "q  quit") {
		t.Fatalf("renderScreen() = %q, want footer", got)
	}
}

func stripANSI(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}
