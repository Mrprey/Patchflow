package git

import "testing"

func TestParseRemotesDeduplicatesAndSorts(t *testing.T) {
	got := ParseRemotes("origin\tgit@github.com:a.git (fetch)\nupstream\tgit@github.com:b.git (fetch)\norigin\tgit@github.com:a.git (push)\n")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Name != "origin" || got[1].Name != "upstream" {
		t.Fatalf("order = %#v", got)
	}
}

func TestParseLog(t *testing.T) {
	got := ParseLog("sha1\tshort\tAna\t2024-01-01\tFix bug\n")
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].SHA != "sha1" || got[0].Title != "Fix bug" {
		t.Fatalf("got = %#v", got[0])
	}
}
