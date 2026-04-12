package projects

import (
	"wydo/internal/tui/messages"
	"wydo/internal/tui/shared"
	"wydo/internal/tui/theme"
	"wydo/internal/workspace"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const projectSidebarWidth = 22

// CombinedModel wraps ProjectsModel (sidebar) and DetailModel (content area) side by side.
type CombinedModel struct {
	picker         ProjectsModel
	detail         DetailModel
	detailLoaded   bool
	sidebarFocused bool
	width          int
	height         int
}

func NewCombinedModel(workspaces []*workspace.Workspace) CombinedModel {
	return CombinedModel{
		picker:         NewProjectsModel(workspaces),
		sidebarFocused: true,
	}
}

func (m *CombinedModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.picker.SetSize(projectSidebarWidth, height)
	if m.detailLoaded {
		m.detail.SetSize(m.detailContentWidth(), height)
	}
}

func (m CombinedModel) detailContentWidth() int {
	w := m.width - projectSidebarWidth - 1
	if w < 0 {
		return 0
	}
	return w
}

func (m *CombinedModel) SetData(workspaces []*workspace.Workspace) {
	m.picker.SetData(workspaces)
}

// LoadDetail sets the detail view and shifts focus to it.
func (m *CombinedModel) LoadDetail(detail DetailModel) {
	detail.SetSize(m.detailContentWidth(), m.height)
	m.detail = detail
	m.detailLoaded = true
	m.sidebarFocused = false
}

// FocusDetail shifts focus to the detail area (e.g. when returning via P key).
func (m *CombinedModel) FocusDetail() {
	if m.detailLoaded {
		m.sidebarFocused = false
	}
}

// ActiveProjectName returns the name of the currently open project, or "".
func (m CombinedModel) ActiveProjectName() string {
	if m.detailLoaded {
		name, _ := m.detail.OpenInfo()
		return name
	}
	return ""
}

// OpenInfo returns the project name and workspace dir of the active detail, for re-opening.
func (m CombinedModel) OpenInfo() (name, wsDir string) {
	return m.detail.OpenInfo()
}

// IsTyping returns true when the sidebar is focused and has an active text input.
func (m CombinedModel) IsTyping() bool {
	if m.sidebarFocused {
		return m.picker.IsTyping()
	}
	return false
}

// IsModal returns true when the detail is focused and has an active modal.
func (m CombinedModel) IsModal() bool {
	if !m.sidebarFocused && m.detailLoaded {
		return m.detail.IsModal()
	}
	return false
}

func (m CombinedModel) Init() tea.Cmd {
	return nil
}

func (m CombinedModel) Update(msg tea.Msg) (CombinedModel, tea.Cmd) {
	switch msg.(type) {
	case messages.FocusSidebarMsg:
		m.sidebarFocused = true
		return m, nil
	}

	if m.sidebarFocused {
		// Intercept esc in list mode when detail is loaded → focus detail instead of going to agenda
		if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
			if m.picker.ShouldFocusDetail() && m.detailLoaded {
				m.sidebarFocused = false
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd
	}

	if m.detailLoaded {
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m CombinedModel) View() string {
	// Show any modal UI as a centered full-screen popup instead of in the sidebar
	if m.picker.mode != modeList {
		p := m.picker
		p.width = m.width
		p.height = m.height
		return p.View()
	}

	sidebar := m.picker.viewSidebar(m.height, m.ActiveProjectName(), m.sidebarFocused)
	sep := shared.RenderSeparatorColumn(m.height)

	var detailContent string
	if m.detailLoaded {
		detailContent = m.detail.View()
	} else {
		detailContent = lipgloss.NewStyle().
			Width(m.detailContentWidth()).
			Height(m.height).
			Foreground(theme.TextMuted).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Select a project from the sidebar")
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, sep, detailContent)
}
