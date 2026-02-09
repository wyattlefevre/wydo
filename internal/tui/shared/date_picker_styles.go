package shared

import "github.com/charmbracelet/lipgloss"

var (
	DatePickerBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(lipgloss.Color("5")).
				Padding(1, 2).
				Width(50)

	DatePickerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("3")).
				Align(lipgloss.Center)

	DatePickerMonthStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("5")).
				Align(lipgloss.Center)

	DatePickerDayHeaderStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("8")).
					Bold(true)

	DatePickerDayStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("7"))

	DatePickerTodayStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("4")).
				Bold(true)

	DatePickerCursorStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("3")).
				Foreground(lipgloss.Color("0")).
				Bold(true)

	DatePickerExamplesStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8")).
				Italic(true)

	DatePickerHelpStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8")).
				Padding(0, 0)
)
