package tasks

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"wydo/internal/tui/theme"
)

var (
	modeStyle    = theme.NavActive
	hintStyle    = theme.HelpHint
	filterStyle  = lipgloss.NewStyle().Foreground(theme.Warning)
	searchStyle  = lipgloss.NewStyle().Foreground(theme.Success)
	infoBarStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(theme.Border)
)

// InfoBarModel displays mode, keybinds, and active filters
type InfoBarModel struct {
	InputContext   *InputModeContext
	FilterState    *FilterState
	SortState      *SortState
	GroupState     *GroupState
	SearchQuery    string
	Message        string
	Width          int
	MultiWorkspace bool
}

// NewInfoBar creates a new info bar
func NewInfoBar() InfoBarModel {
	return InfoBarModel{
		Width: 80,
	}
}

// SetContext updates the info bar with current state
func (m *InfoBarModel) SetContext(ctx *InputModeContext, filter *FilterState, sortState *SortState, groupState *GroupState, searchQuery string, multiWorkspace bool) {
	m.InputContext = ctx
	m.FilterState = filter
	m.SortState = sortState
	m.GroupState = groupState
	m.SearchQuery = searchQuery
	m.MultiWorkspace = multiWorkspace
}

// View renders the info bar as a single line with a bottom border.
func (m *InfoBarModel) View() string {
	return infoBarStyle.Width(m.Width).Render(m.renderStatusLine())
}

// RenderModeBar renders the mode indicator and keybind hints as a full-width bottom bar.
func (m *InfoBarModel) RenderModeBar(width int) string {
	left := m.renderModeLine()
	right := hintStyle.Render(m.RenderHintsRaw())
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	content := left + strings.Repeat(" ", gap) + right
	return theme.StatusBar.Width(width).Render(content)
}

// renderStatusLine combines filters, sort, group, view, and search/message into one line.
func (m *InfoBarModel) renderStatusLine() string {
	var parts []string

	filterText := "none"
	if m.FilterState != nil && !m.FilterState.IsEmpty() {
		filterText = m.FilterState.Summary()
	}
	parts = append(parts, filterStyle.Render("Filters: "+filterText))

	sortText := "none"
	if m.SortState != nil && m.SortState.IsActive() {
		sortText = m.SortState.String()
	}
	parts = append(parts, filterStyle.Render("Sort: "+sortText))

	groupText := "none"
	if m.GroupState != nil && m.GroupState.IsActive() {
		groupText = m.GroupState.String()
	}
	parts = append(parts, filterStyle.Render("Group: "+groupText))

	if m.Message != "" {
		parts = append(parts, hintStyle.Render(m.Message))
	} else if m.SearchQuery != "" {
		parts = append(parts, searchStyle.Render("Search: \""+m.SearchQuery+"\""))
	}

	return strings.Join(parts, "  |  ")
}

func (m *InfoBarModel) renderModeLine() string {
	if m.InputContext == nil {
		return modeStyle.Render("[Normal]")
	}
	return modeStyle.Render("[" + m.InputContext.String() + "]")
}

// RenderHints returns the styled keybind hints for the current mode.
func (m *InfoBarModel) RenderHints() string {
	return hintStyle.Render(m.RenderHintsRaw())
}

// RenderHintsRaw returns the raw (unstyled) keybind hints for the current mode.
func (m *InfoBarModel) RenderHintsRaw() string {
	if m.InputContext == nil {
		return "?:help  /:search  enter:details  space:done"
	}

	switch m.InputContext.Mode {
	case ModeNormal:
		hint := "?:help  /:search  enter:details  space:done  r:rename"
		if m.MultiWorkspace {
			hint = "?:help  /:search  enter:details  space:done  r:rename  W:workspace"
		}
		return hint

	case ModeFilterSelect:
		hint := "/:search  d:date  p:project  P:priority  t:context  s:status  f:file  esc:back"
		if m.MultiWorkspace {
			hint = "/:search  d:date  p:project  P:priority  t:context  s:status  f:file  w:workspace  esc:back"
		}
		return hint

	case ModeSortSelect:
		return "d:date  p:project  P:priority  t:context  esc:back"

	case ModeGroupSelect:
		return "d:date  p:project  P:priority  t:context  esc:back"

	case ModeSortDirection, ModeGroupDirection:
		return "a:ascending  d:descending  esc:back"

	case ModeSearch:
		return "type to filter  j/k:navigate  enter:confirm  esc:clear"

	case ModeDateInput:
		return "format: yyyy-MM-dd  enter:apply  esc:cancel"

	case ModeFuzzyPicker:
		return "j/k:navigate  enter:select  esc:cancel"

	case ModeTaskEditor:
		return "d:due  s:sched  p:project  t:context  i:priority  enter:save  esc:cancel"

	case ModeEditDueDate:
		return "format: yyyy-MM-dd  enter:save  esc:cancel"

	case ModeEditProject, ModeEditContext:
		return "j/k:navigate  enter:select  space:toggle  esc:cancel"

	case ModeArchive:
		return "space:select  a:select all  enter:archive  esc:cancel"

	case ModeDelete:
		return "space:select  a:select all  enter:delete  esc:cancel"

	case ModeConfirmation:
		return "y/enter:yes  n/esc:no"

	case ModeBoardPicker:
		return "j/k:navigate  enter:select  esc:cancel"
	}

	return ""
}

