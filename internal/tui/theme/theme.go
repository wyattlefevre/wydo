package theme

import "github.com/charmbracelet/lipgloss"

// ---------------------------------------------------------------------------
// Color palette — ANSI 0-15 + one 256-color accent
// ---------------------------------------------------------------------------

var (
	Text       = lipgloss.Color("7")
	TextMuted  = lipgloss.Color("8")
	TextBright = lipgloss.Color("15")

	Primary   = lipgloss.Color("4")  // blue
	Secondary = lipgloss.Color("6")  // cyan
	Accent    = lipgloss.Color("5")  // magenta
	Success   = lipgloss.Color("2")  // green
	Warning   = lipgloss.Color("3")  // yellow
	Danger    = lipgloss.Color("1")  // red
	Surface   = lipgloss.Color("236") // dark bg
	Border    = lipgloss.Color("8")  // dim
	BorderFocused = lipgloss.Color("4") // blue
)

// ---------------------------------------------------------------------------
// Semantic text styles
// ---------------------------------------------------------------------------

var (
	Title    = lipgloss.NewStyle().Bold(true).Foreground(Primary)
	Subtitle = lipgloss.NewStyle().Bold(true).Foreground(Secondary)
	Muted    = lipgloss.NewStyle().Foreground(TextMuted)
	Bold     = lipgloss.NewStyle().Bold(true)

	Error = lipgloss.NewStyle().Bold(true).Foreground(Danger)
	Warn  = lipgloss.NewStyle().Bold(true).Foreground(Warning)
	Ok    = lipgloss.NewStyle().Bold(true).Foreground(Success)

	Cursor     = lipgloss.NewStyle().Bold(true).Foreground(Success)
	Selected   = lipgloss.NewStyle().Bold(true).Foreground(Warning)
	SelectedBg = lipgloss.NewStyle().Foreground(TextBright).Background(Surface)

	Project  = lipgloss.NewStyle().Foreground(Secondary)
	Context  = lipgloss.NewStyle().Foreground(Accent)
	Tag      = lipgloss.NewStyle().Foreground(Warning)
	Priority = lipgloss.NewStyle().Bold(true).Foreground(Danger)
	Done     = lipgloss.NewStyle().Foreground(TextMuted)
)

// ---------------------------------------------------------------------------
// Reusable component helpers
// ---------------------------------------------------------------------------

var (
	ModalBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(1, 2)

	ModalTitle = lipgloss.NewStyle().Bold(true).Foreground(Warning)

	ModalHelp = lipgloss.NewStyle().Foreground(TextMuted)

	StatusBar = lipgloss.NewStyle().
			Foreground(TextMuted).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(Border)

	HelpHint = lipgloss.NewStyle().Foreground(TextMuted)

	NavActive   = lipgloss.NewStyle().Bold(true).Foreground(Primary)
	NavInactive = lipgloss.NewStyle().Foreground(TextMuted)

	TabActive   = lipgloss.NewStyle().Bold(true).Foreground(Primary)
	TabInactive = lipgloss.NewStyle().Foreground(TextMuted)
	TabBar      = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(Border).
			PaddingLeft(1)
)
