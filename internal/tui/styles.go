package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	ColorPrimary   = lipgloss.Color("4")
	ColorSecondary = lipgloss.Color("6")
	ColorSuccess   = lipgloss.Color("2")
	ColorWarning   = lipgloss.Color("3")
	ColorDanger    = lipgloss.Color("1")
	ColorMuted     = lipgloss.Color("8")

	// Title styles
	TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)

	// Status bar
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(ColorMuted)

	// Help text
	HelpStyle = lipgloss.NewStyle().Foreground(ColorMuted)
)
