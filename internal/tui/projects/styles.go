package projects

import "github.com/charmbracelet/lipgloss"

var (
	// Colors - Using terminal's native ANSI palette (0-15)
	colorPrimary = lipgloss.Color("4") // Blue
	colorMuted   = lipgloss.Color("8") // Bright black (dark gray)
	colorWarning = lipgloss.Color("3") // Yellow / selected
	colorAccent  = lipgloss.Color("6") // Cyan
	colorSuccess = lipgloss.Color("2") // Green
	colorDanger  = lipgloss.Color("1") // Red

	// Title
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 1)

	// List items
	listItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7")).
			Padding(0, 2)

	selectedListItemStyle = lipgloss.NewStyle().
				Foreground(colorWarning).
				Bold(true).
				Padding(0, 2)

	// Muted / path
	pathStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	virtualBadgeStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Italic(true)

	// Help
	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(1, 2)

	// Error
	errorStyle = lipgloss.NewStyle().
			Foreground(colorDanger).
			Bold(true)

	// Section header for detail view
	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent)

	sectionActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWarning)

	// Detail item
	detailItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7")).
			Padding(0, 2)

	selectedDetailItemStyle = lipgloss.NewStyle().
				Foreground(colorWarning).
				Bold(true).
				Padding(0, 2)

	// Search
	searchLabelStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)
)
