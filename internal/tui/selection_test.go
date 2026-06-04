package tui

import "testing"

func TestSelectionModelToggleAndCounts(t *testing.T) {
	m := SelectionModel{Items: []CommitOption{{Title: "One"}, {Title: "Two"}}}
	m.Toggle(1)
	if m.SelectedCount() != 1 || !m.Items[1].Selected {
		t.Fatalf("unexpected selection state: %#v", m.Items)
	}
	m.Toggle(1)
	if m.SelectedCount() != 0 {
		t.Fatalf("SelectedCount() = %d, want 0", m.SelectedCount())
	}
}

func TestSelectionModelSelectAllAndClearAll(t *testing.T) {
	m := SelectionModel{Items: []CommitOption{{Title: "One"}, {Title: "Two"}}}
	m.SelectAll()
	if m.SelectedCount() != 2 {
		t.Fatalf("SelectedCount() = %d, want 2", m.SelectedCount())
	}
	m.ClearAll()
	if m.SelectedCount() != 0 {
		t.Fatalf("SelectedCount() = %d, want 0", m.SelectedCount())
	}
}

func TestSelectionModelFilteredItems(t *testing.T) {
	m := SelectionModel{
		Items: []CommitOption{
			{SHA: "abc123", ShortSHA: "abc123", Title: "Fix login crash", Labels: []string{"bug", "mobile"}},
			{SHA: "def456", ShortSHA: "def456", Title: "Update docs", Labels: []string{"docs"}},
		},
		Filter: "bug",
	}
	got := m.FilteredItems()
	if len(got) != 1 || got[0].Title != "Fix login crash" {
		t.Fatalf("FilteredItems() = %#v", got)
	}
}
