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

const fuzzyMinScore = 0

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
	indexes := fuzzyMatchIndexes(m.Filter, func() []string {
		labels := make([]string, 0, len(m.Items))
		for _, item := range m.Items {
			labels = append(labels, searchableCommitOption(item))
		}
		return labels
	}())
	filtered := make([]CommitOption, 0, len(indexes))
	for _, idx := range indexes {
		filtered = append(filtered, m.Items[idx])
	}
	return filtered
}

func fuzzyMatchIndexes(query string, items []string) []int {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		indexes := make([]int, 0, len(items))
		for i := range items {
			indexes = append(indexes, i)
		}
		return indexes
	}
	indexes := make([]int, 0, len(items))
	seen := make(map[int]struct{}, len(items))
	for i, item := range items {
		if strings.Contains(strings.ToLower(item), query) {
			indexes = append(indexes, i)
			seen[i] = struct{}{}
		}
	}
	matches := fuzzy.Find(query, items)
	for _, match := range matches {
		if match.Score < fuzzyMinScore {
			continue
		}
		if _, ok := seen[match.Index]; ok {
			continue
		}
		indexes = append(indexes, match.Index)
	}
	return indexes
}

func searchableCommitOption(item CommitOption) string {
	parts := []string{item.SHA, item.ShortSHA, item.Title, strconv.Itoa(item.PRNumber)}
	parts = append(parts, item.Labels...)
	return strings.Join(parts, " ")
}
