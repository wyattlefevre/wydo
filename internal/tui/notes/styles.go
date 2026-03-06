package notes

import (
	"github.com/charmbracelet/lipgloss"
	"wydo/internal/tui/theme"
)

var (
	titleStyle = theme.Title.Padding(0, 1)

	listItemStyle = lipgloss.NewStyle().
			Foreground(theme.Text).
			Padding(0, 2)

	selectedListItemStyle = lipgloss.NewStyle().
				Foreground(theme.Warning).
				Bold(true).
				Padding(0, 2)

	pathStyle = theme.Muted

	sectionHeaderStyle = lipgloss.NewStyle().
				Foreground(theme.Accent).
				Bold(true)

	confirmUnpinBoxStyle = theme.ModalBox.Padding(1, 2)

	confirmUnpinTitleStyle = lipgloss.NewStyle().
				Foreground(theme.Danger).
				Bold(true)

	confirmUnpinHelpStyle = theme.ModalHelp
)
