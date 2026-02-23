package projects

import (
	"github.com/charmbracelet/lipgloss"
	"wydo/internal/tui/theme"
)

var (
	// Title
	titleStyle = theme.Title.Padding(0, 1)

	// List items
	listItemStyle = lipgloss.NewStyle().
			Foreground(theme.Text).
			Padding(0, 2)

	selectedListItemStyle = lipgloss.NewStyle().
				Foreground(theme.Warning).
				Bold(true).
				Padding(0, 2)

	// Muted / path
	pathStyle = theme.Muted

	virtualBadgeStyle = lipgloss.NewStyle().
				Foreground(theme.TextMuted).
				Italic(true)

	// Error
	errorStyle = theme.Error

	// Section header for detail view
	sectionHeaderStyle = theme.Subtitle

	sectionActiveStyle = theme.Selected

	// Detail item
	detailItemStyle = lipgloss.NewStyle().
			Foreground(theme.Text).
			Padding(0, 2)

	selectedDetailItemStyle = lipgloss.NewStyle().
				Foreground(theme.Warning).
				Bold(true).
				Padding(0, 2)

	// Search
	searchLabelStyle = lipgloss.NewStyle().
				Foreground(theme.Secondary).
				Bold(true)
)
