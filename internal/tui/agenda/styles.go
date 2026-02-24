package agenda

import (
	"github.com/charmbracelet/lipgloss"
	"wydo/internal/tui/theme"
)

// -- day.go styles --
var (
	titleStyle       = theme.Title
	sectionStyle     = theme.Subtitle
	emptyStyle       = lipgloss.NewStyle().Foreground(theme.TextMuted).Italic(true)
	searchLabelStyle = lipgloss.NewStyle().Foreground(theme.Secondary).Bold(true)
)

// -- item_line.go styles --
var (
	reasonDueStyle     = lipgloss.NewStyle().Foreground(theme.Warning).Bold(true)
	reasonSchedStyle   = lipgloss.NewStyle().Foreground(theme.Primary)
	reasonNoteStyle    = theme.Muted
	overdueHeaderStyle = theme.Error
	projectStyle       = theme.Project
	boardInfoStyle     = lipgloss.NewStyle().Foreground(theme.Accent)
	notePathStyle      = theme.Muted
	selectedStyle      = lipgloss.NewStyle().Bold(true).Foreground(theme.TextBright).Background(theme.Primary)
	cursorStyle        = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
	normalStyle        = lipgloss.NewStyle()
	completedStyle     = lipgloss.NewStyle().Foreground(theme.TextMuted).Strikethrough(true)
	completedTagStyle  = theme.Muted
)

// -- week.go styles --
var (
	weekDayHeaderStyle = theme.Subtitle
	weekTodayStyle     = theme.Ok
	weekCountStyle     = theme.Muted
)

// -- month.go styles --
var (
	calDayHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(theme.TextMuted).Width(5).Align(lipgloss.Center)
	calDayStyle        = lipgloss.NewStyle().Width(5).Align(lipgloss.Center)
	calTodayStyle      = lipgloss.NewStyle().Width(5).Align(lipgloss.Center).Bold(true).Foreground(theme.Success)
	calCursorStyle     = lipgloss.NewStyle().Width(5).Align(lipgloss.Center).Bold(true).Foreground(theme.TextBright).Background(theme.Primary)
	calHasItemsStyle   = lipgloss.NewStyle().Width(5).Align(lipgloss.Center).Foreground(theme.Warning)
	calEmptyStyle      = lipgloss.NewStyle().Width(5).Align(lipgloss.Center).Foreground(theme.TextMuted)
	calMonthTitleStyle = theme.Title
	detailHeaderStyle  = theme.Subtitle
)
