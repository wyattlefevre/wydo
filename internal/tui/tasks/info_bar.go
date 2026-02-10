package tasks

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	modeStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	hintStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	filterStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	searchStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	infoBarStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(lipgloss.Color("8"))
)

// InfoBarModel displays mode, keybinds, and active filters
type InfoBarModel struct {
	InputContext *InputModeContext
	FilterState  *FilterState
	SortState    *SortState
	GroupState   *GroupState
	SearchQuery  string
	Message      string
	Width        int
	FileViewMode FileViewMode
}

// NewInfoBar creates a new info bar
func NewInfoBar() InfoBarModel {
	return InfoBarModel{
		Width: 80,
	}
}

// SetContext updates the info bar with current state
func (m *InfoBarModel) SetContext(ctx *InputModeContext, filter *FilterState, sortState *SortState, groupState *GroupState, searchQuery string, fileViewMode FileViewMode) {
	m.InputContext = ctx
	m.FilterState = filter
	m.SortState = sortState
	m.GroupState = groupState
	m.SearchQuery = searchQuery
	m.FileViewMode = fileViewMode
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
	return m.getHintsForMode()
}

func (m *InfoBarModel) getHintsForMode() string {
	if m.InputContext == nil {
		return hintStyle.Render("n:new  f:filter  s:sort  g:group  /:search  enter:edit  space:toggle")
	}

	switch m.InputContext.Mode {
	case ModeNormal:
		return hintStyle.Render("n:new  f:filter  s:sort  g:group  /:search  F:files  A:archive  D:delete  enter:edit  space:toggle")

	case ModeFilterSelect:
		return hintStyle.Render("/:search  d:date  p:project  P:priority  t:context  s:status  f:file  esc:back")

	case ModeSortSelect:
		return hintStyle.Render("d:date  p:project  P:priority  t:context  esc:back")

	case ModeGroupSelect:
		return hintStyle.Render("d:date  p:project  P:priority  t:context  f:file  esc:back")

	case ModeSortDirection, ModeGroupDirection:
		return hintStyle.Render("a:ascending  d:descending  esc:back")

	case ModeSearch:
		return hintStyle.Render("type to filter  j/k:navigate  enter:confirm  esc:clear")

	case ModeDateInput:
		return hintStyle.Render("format: yyyy-MM-dd  enter:apply  esc:cancel")

	case ModeFuzzyPicker:
		return hintStyle.Render("j/k:navigate  enter:select  esc:cancel")

	case ModeTaskEditor:
		return hintStyle.Render("d:due  p:project  t:context  P:priority  enter:save  esc:cancel")

	case ModeEditDueDate:
		return hintStyle.Render("format: yyyy-MM-dd  enter:save  esc:cancel")

	case ModeEditProject, ModeEditContext:
		return hintStyle.Render("j/k:navigate  enter:select  space:toggle  esc:cancel")

	case ModeConfirmation:
		return hintStyle.Render("y/enter:yes  n/esc:no")
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
			Foreground(lipgloss.Color("6")).
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
