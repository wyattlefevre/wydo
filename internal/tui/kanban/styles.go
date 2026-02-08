package kanban

import "github.com/charmbracelet/lipgloss"

const (
	// Layout constants
	columnWidth             = 40
	columnPaddingHorizontal = 2
	cardPaddingHorizontal   = 1
	cardBorderWidth         = 1
)

var (
	// Colors - Using terminal's native ANSI palette (0-15)
	colorPrimary   = lipgloss.Color("6") // Cyan
	colorSecondary = lipgloss.Color("5") // Magenta
	colorMuted     = lipgloss.Color("8") // Bright black (dark gray)
	colorSuccess   = lipgloss.Color("2") // Green
	colorDanger    = lipgloss.Color("1") // Red
	colorWarning   = lipgloss.Color("3") // Yellow
	colorAccent    = lipgloss.Color("4") // Blue

	// Title styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSecondary).
			Padding(0, 1)

	// Column styles
	columnStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorMuted).
			Padding(1, columnPaddingHorizontal).
			Width(columnWidth)

	columnTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary).
				Align(lipgloss.Center)

	selectedColumnTitleStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(colorWarning).
					Align(lipgloss.Center)

	selectedColumnStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(colorWarning).
				Padding(1, columnPaddingHorizontal).
				Width(columnWidth)

	// Card styles
	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorMuted).
			Padding(0, cardPaddingHorizontal).
			MarginBottom(1)

	selectedCardStyle = lipgloss.NewStyle().
				Border(lipgloss.ThickBorder(), false, false, false, true).
				BorderForeground(colorWarning).
				Background(lipgloss.Color("236")).
				Padding(0, cardPaddingHorizontal).
				MarginBottom(1).
				Bold(true)

	cardTitleStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	cardTagStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Italic(true)

	cardProjectStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Italic(true)

	cardPreviewStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8")) // Bright black (dim text)

	// Help styles
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Padding(1, 2)

	// List styles
	listItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7")).
			Padding(0, 2)

	selectedListItemStyle = lipgloss.NewStyle().
				Foreground(colorWarning).
				Bold(true).
				Padding(0, 2)

	// Message styles
	errorStyle = lipgloss.NewStyle().
			Foreground(colorDanger).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(colorWarning).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	// Tag picker styles
	tagPickerBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(colorSecondary).
				Padding(1, 2).
				Width(50)

	tagPickerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWarning)

	tagItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	tagItemSelectedStyle = lipgloss.NewStyle().
				Foreground(colorSecondary).
				Bold(true)

	tagItemHighlightStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("0")).
				Foreground(colorWarning)

	tagCreateNewStyle = lipgloss.NewStyle().
				Foreground(colorSuccess).
				Italic(true)

	// Scroll indicator style
	scrollIndicatorStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Italic(true).
				Align(lipgloss.Center)

	// Path style for dimmed directory display
	pathStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Column editor styles
	columnEditorBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(colorPrimary).
				Padding(1, 2).
				Width(60)

	columnEditorTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWarning).
				Align(lipgloss.Center)

	columnEditorPromptStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary)

	columnEditorItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("7"))

	columnEditorItemHighlightStyle = lipgloss.NewStyle().
					Background(lipgloss.Color("236")).
					Foreground(colorWarning).
					Bold(true)

	columnEditorItemImmutableStyle = lipgloss.NewStyle().
					Foreground(colorMuted).
					Italic(true)

	// URL input modal styles
	urlInputBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(colorPrimary).
				Padding(1, 2).
				Width(60)

	urlInputTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWarning).
				Align(lipgloss.Center)

	// Priority input modal styles
	priorityInputBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(colorPrimary).
				Padding(1, 2).
				Width(60)

	priorityInputTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWarning).
				Align(lipgloss.Center)

	// Date picker modal styles
	datePickerBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(colorSecondary).
				Padding(1, 2).
				Width(50)

	datePickerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWarning).
				Align(lipgloss.Center)

	datePickerMonthStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorSecondary).
				Align(lipgloss.Center)

	datePickerDayHeaderStyle = lipgloss.NewStyle().
					Foreground(colorMuted).
					Bold(true)

	datePickerDayStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("7"))

	datePickerTodayStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	datePickerCursorStyle = lipgloss.NewStyle().
				Background(colorWarning).
				Foreground(lipgloss.Color("0")).
				Bold(true)

	datePickerExamplesStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Italic(true)

	// Filter indicator style
	filterIndicatorStyle = lipgloss.NewStyle().
				Foreground(colorWarning).
				Bold(true)
)
