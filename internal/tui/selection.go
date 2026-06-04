package tui

import (
	"strconv"
	"strings"
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
	Items   []CommitOption
	Filter  string
	Focus   int
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
	filtered := make([]CommitOption, 0, len(m.Items))
	for _, item := range m.Items {
		if matchesFilter(item, query) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func matchesFilter(item CommitOption, query string) bool {
	if strings.Contains(strings.ToLower(item.SHA), query) {
		return true
	}
	if strings.Contains(strings.ToLower(item.ShortSHA), query) {
		return true
	}
	if strings.Contains(strings.ToLower(item.Title), query) {
		return true
	}
	for _, label := range item.Labels {
		if strings.Contains(strings.ToLower(label), query) {
			return true
		}
	}
	if item.PRNumber > 0 && strings.HasPrefix(query, "#") {
		return strings.Contains(strconv.Itoa(item.PRNumber), strings.TrimPrefix(query, "#"))
	}
	return false
}
