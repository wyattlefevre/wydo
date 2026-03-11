package messages

import (
	tea "github.com/charmbracelet/bubbletea"
	"wydo/internal/workspace"
)

// ViewType represents the different views in the application
type ViewType int

const (
	ViewAgendaDay ViewType = iota
	ViewAgendaWeek
	ViewAgendaMonth
	ViewKanbanPicker
	ViewKanbanBoard
	ViewTaskManager
	ViewProjects
	ViewProjectDetail
	ViewNotes
)

// SwitchViewMsg is sent by child views to switch to a different view
type SwitchViewMsg struct {
	View ViewType
}

// OpenBoardMsg requests opening a specific board at a specific position
type OpenBoardMsg struct {
	BoardPath string
	ColIndex  int
	CardIndex int
}

// FocusTaskMsg requests focusing on a specific task in the task manager
type FocusTaskMsg struct {
	TaskID string
}

// OpenProjectMsg requests opening a specific project detail view
type OpenProjectMsg struct {
	ProjectName     string
	WorkspaceRootDir string
}

// DataRefreshMsg signals that data should be reloaded
type DataRefreshMsg struct{}

// CreateSubProjectMsg requests creating a new sub-project under a parent project
type CreateSubProjectMsg struct {
	ParentProject *workspace.Project
	Name          string
	WsDir         string
}

// RequestExitMsg is sent by child views when the user wants to quit
type RequestExitMsg struct{}

func SwitchView(v ViewType) tea.Cmd {
	return func() tea.Msg {
		return SwitchViewMsg{View: v}
	}
}
