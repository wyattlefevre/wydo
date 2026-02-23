package shared

import (
	"github.com/charmbracelet/lipgloss"
	"wydo/internal/tui/theme"
)

var (
	DatePickerBoxStyle = theme.ModalBox.Width(50)

	DatePickerTitleStyle = theme.ModalTitle.Align(lipgloss.Center)

	DatePickerMonthStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(theme.Accent).
				Align(lipgloss.Center)

	DatePickerDayHeaderStyle = lipgloss.NewStyle().
					Foreground(theme.TextMuted).
					Bold(true)

	DatePickerDayStyle = lipgloss.NewStyle().
				Foreground(theme.Text)

	DatePickerTodayStyle = lipgloss.NewStyle().
				Foreground(theme.Primary).
				Bold(true)

	DatePickerCursorStyle = lipgloss.NewStyle().
				Background(theme.Warning).
				Foreground(lipgloss.Color("0")).
				Bold(true)

	DatePickerExamplesStyle = lipgloss.NewStyle().
				Foreground(theme.TextMuted).
				Italic(true)

	DatePickerHelpStyle = theme.ModalHelp
)
