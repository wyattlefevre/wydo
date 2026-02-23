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
	FileViewMode   FileViewMode
	MultiWorkspace bool
}

// NewInfoBar creates a new info bar
func NewInfoBar() InfoBarModel {
	return InfoBarModel{
		Width: 80,
	}
}

// SetContext updates the info bar with current state
func (m *InfoBarModel) SetContext(ctx *InputModeContext, filter *FilterState, sortState *SortState, groupState *GroupState, searchQuery string, fileViewMode FileViewMode, multiWorkspace bool) {
	m.InputContext = ctx
	m.FilterState = filter
	m.SortState = sortState
	m.GroupState = groupState
	m.SearchQuery = searchQuery
	m.FileViewMode = fileViewMode
	m.MultiWorkspace = multiWorkspace
}

// View renders the info bar (3 fixed lines)
func (m *InfoBarModel) View() string {
	var lines [3]string

	lines[0] = m.renderModeLine()
	lines[1] = m.renderFiltersLine()
	lines[2] = m.renderSearchLine()

	content := strings.Join(lines[:], "\n")
	return infoBarStyle.Width(m.Width).Render(content)
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
		hint := "?:help  /:search  enter:details  space:done"
		if m.MultiWorkspace {
			hint = "?:help  /:search  enter:details  space:done  W:workspace"
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
		return "d:date  p:project  P:priority  t:context  f:file  esc:back"

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

	case ModeConfirmation:
		return "y/enter:yes  n/esc:no"

	case ModeBoardPicker:
		return "j/k:navigate  enter:select  esc:cancel"
	}

	return ""
}

func (m *InfoBarModel) renderFiltersLine() string {
	var parts []string

	if m.FilterState != nil && !m.FilterState.IsEmpty() {
		parts = append(parts, filterStyle.Render("Filters: "+m.FilterState.Summary()))
	}

	if m.SortState != nil && m.SortState.IsActive() {
		parts = append(parts, filterStyle.Render("Sort: "+m.SortState.String()))
	}

	if m.GroupState != nil && m.GroupState.IsActive() {
		parts = append(parts, filterStyle.Render("Group: "+m.GroupState.String()))
	}

	if m.FileViewMode != FileViewTodoOnly {
		var viewMode string
		if m.FileViewMode == FileViewAll {
			viewMode = "View: todo.txt + done.txt"
		} else {
			viewMode = "View: done.txt"
		}
		parts = append(parts, lipgloss.NewStyle().
			Foreground(theme.Secondary).
			Render(viewMode))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "  |  ")
}

func (m *InfoBarModel) renderSearchLine() string {
	if m.Message != "" {
		return hintStyle.Render(m.Message)
	}

	if m.SearchQuery != "" {
		return searchStyle.Render("Search: \"" + m.SearchQuery + "\"")
	}

	return ""
}
