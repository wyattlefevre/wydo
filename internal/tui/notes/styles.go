package notes

import (
	"github.com/charmbracelet/lipgloss"
	"wydo/internal/tui/theme"
)

var (
	titleStyle = theme.Title.Padding(0, 1)

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
