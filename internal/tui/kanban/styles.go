package kanban

import (
	"github.com/charmbracelet/lipgloss"
	"wydo/internal/tui/theme"
)

const (
	// Layout constants
	columnWidth             = 40
	columnPaddingHorizontal = 2
	cardPaddingHorizontal   = 1
	cardBorderWidth         = 1
)

var (
	// Title styles
	titleStyle = theme.Title.Padding(0, 1)

	// Column styles
	columnStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(1, columnPaddingHorizontal).
			Width(columnWidth)

	columnTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(theme.Primary).
				Align(lipgloss.Center)

	selectedColumnTitleStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(theme.Warning).
					Background(theme.Surface).
					Underline(true).
					Align(lipgloss.Center)

	selectedColumnStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.BorderFocused).
				Padding(1, columnPaddingHorizontal).
				Width(columnWidth)

	// Card styles
	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), false, false, false, true).
			BorderForeground(theme.Border).
			Padding(0, cardPaddingHorizontal).
			MarginBottom(1)

	selectedCardStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder(), false, false, false, true).
				BorderForeground(theme.BorderFocused).
				Background(theme.Surface).
				Padding(0, cardPaddingHorizontal).
				MarginBottom(1).
				Bold(true)

	moveSelectedCardStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder(), false, false, false, true).
				BorderForeground(theme.Warning).
				Background(lipgloss.Color("54")).
				Padding(0, cardPaddingHorizontal).
				MarginBottom(1).
				Bold(true)

	cardTitleStyle = lipgloss.NewStyle().
			Foreground(theme.Primary).
			Bold(true)

	cardTagStyle = lipgloss.NewStyle().
			Foreground(theme.Accent).
			Italic(true)

	cardProjectStyle = lipgloss.NewStyle().
				Foreground(theme.Secondary).
				Italic(true)

	cardPreviewStyle = lipgloss.NewStyle().
				Foreground(theme.TextMuted)

	// Help styles
	helpStyle = theme.Muted.Padding(1, 2)

	// List styles
	listItemStyle = lipgloss.NewStyle().
			Foreground(theme.Text).
			Padding(0, 2)

	selectedListItemStyle = lipgloss.NewStyle().
				Foreground(theme.Warning).
				Bold(true).
				Padding(0, 2)

	// Message styles
	errorStyle   = theme.Error
	warningStyle = theme.Warn
	successStyle = theme.Ok

	// Tag picker styles
	tagPickerBoxStyle = theme.ModalBox.Width(50)

	tagPickerTitleStyle = theme.ModalTitle

	tagItemStyle = lipgloss.NewStyle().
			Foreground(theme.Text)

	tagItemSelectedStyle = lipgloss.NewStyle().
				Foreground(theme.Secondary).
				Bold(true)

	tagItemHighlightStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("0")).
				Foreground(theme.Warning)

	tagCreateNewStyle = lipgloss.NewStyle().
				Foreground(theme.Success).
				Italic(true)

	// Tmux session indicator style
	cardTmuxStyle = lipgloss.NewStyle().
			Background(theme.Warning).
			Foreground(lipgloss.Color("16"))

	// Scroll indicator style
	scrollIndicatorStyle = lipgloss.NewStyle().
				Foreground(theme.Primary).
				Italic(true).
				Align(lipgloss.Center)

	// Path style for dimmed directory display
	pathStyle = theme.Muted

	// Column editor styles
	columnEditorBoxStyle = theme.ModalBox.Width(60)

	columnEditorTitleStyle = theme.ModalTitle.Align(lipgloss.Center)

	columnEditorPromptStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(theme.Primary)

	columnEditorItemStyle = lipgloss.NewStyle().
				Foreground(theme.Text)

	columnEditorItemHighlightStyle = lipgloss.NewStyle().
					Background(theme.Surface).
					Foreground(theme.Warning).
					Bold(true)

	columnEditorItemImmutableStyle = lipgloss.NewStyle().
					Foreground(theme.TextMuted).
					Italic(true)

	// URL input modal styles
	urlInputBoxStyle = theme.ModalBox.Width(60)

	urlInputTitleStyle = theme.ModalTitle.Align(lipgloss.Center)

	// Priority input modal styles
	priorityInputBoxStyle = theme.ModalBox.Width(60)

	priorityInputTitleStyle = theme.ModalTitle.Align(lipgloss.Center)

	// Filter indicator style
	filterIndicatorStyle = lipgloss.NewStyle().
				Foreground(theme.Warning).
				Bold(true)
)

// modeIndicatorStyle returns a bold style with the given foreground color for mode badges.
func modeIndicatorStyle(color lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(color)
}
