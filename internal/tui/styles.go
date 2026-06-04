package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	AppTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#8BDB81")).
			Padding(0, 1)

	SectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7CC7FF"))

	MutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8A9099"))

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#2E3440")).
			Padding(1, 2)

	SelectedRowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0B0F14")).
				Background(lipgloss.Color("#8BDB81")).
				Bold(true)

	RowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E6EAF0"))

	DimRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8A9099"))

	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8A9099"))

	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B6B"))
)

func renderScreen(title, subtitle, body string, footer []string) string {
	parts := []string{
		AppTitleStyle.Render("PATCHFLOW"),
	}
	if strings.TrimSpace(title) != "" {
		parts = append(parts, SectionStyle.Render(title))
	}
	if strings.TrimSpace(subtitle) != "" {
		parts = append(parts, MutedStyle.Render(subtitle))
	}
	parts = append(parts, "", PanelStyle.Render(body))
	if len(footer) > 0 {
		parts = append(parts, "", HelpStyle.Render(strings.Join(footer, "   ")))
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func renderRow(selected bool, text string) string {
	if selected {
		return SelectedRowStyle.Render(text)
	}
	return RowStyle.Render(text)
}

func renderDimRow(text string) string {
	return DimRowStyle.Render(text)
}

func renderError(err error) string {
	if err == nil {
		return ""
	}
	return ErrorStyle.Render("Error: " + err.Error())
}
