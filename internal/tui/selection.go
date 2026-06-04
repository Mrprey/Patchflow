package tui

import (
	"strconv"
	"strings"

	"github.com/sahilm/fuzzy"
)

type CommitOption struct {
	SHA      string
	ShortSHA string
	Title    string
	PRNumber int
	Labels   []string
	Selected bool
}

type SelectionModel struct {
	Items  []CommitOption
	Filter string
	Focus  int
}

func (m *SelectionModel) Toggle(index int) {
	if index < 0 || index >= len(m.Items) {
		return
	}
	m.Items[index].Selected = !m.Items[index].Selected
}

func (m *SelectionModel) SelectAll() {
	for i := range m.Items {
		m.Items[i].Selected = true
	}
}

func (m *SelectionModel) ClearAll() {
	for i := range m.Items {
		m.Items[i].Selected = false
	}
}

func (m SelectionModel) SelectedCount() int {
	count := 0
	for _, item := range m.Items {
		if item.Selected {
			count++
		}
	}
	return count
}

func (m SelectionModel) FilteredItems() []CommitOption {
	query := strings.ToLower(strings.TrimSpace(m.Filter))
	if query == "" {
		return append([]CommitOption(nil), m.Items...)
	}
	labels := make([]string, 0, len(m.Items))
	for _, item := range m.Items {
		labels = append(labels, searchableCommitOption(item))
	}
	matches := fuzzy.Find(query, labels)
	filtered := make([]CommitOption, 0, len(matches))
	for _, match := range matches {
		filtered = append(filtered, m.Items[match.Index])
	}
	return filtered
}

func searchableCommitOption(item CommitOption) string {
	parts := []string{item.SHA, item.ShortSHA, item.Title, strconv.Itoa(item.PRNumber)}
	parts = append(parts, item.Labels...)
	return strings.Join(parts, " ")
}
