package messages

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"wydo/internal/workspace"
)

// StatusLevel indicates the severity of a status message shown in the top bar.
type StatusLevel int

const (
	LevelSuccess StatusLevel = iota
	LevelWarning
	LevelError
)

// StatusMsg carries a one-line feedback message to be shown in the right side of the tab bar.
type StatusMsg struct {
	Text  string
	Level StatusLevel
}

// ClearStatusMsg is fired by the auto-clear timer to dismiss the status message.
type ClearStatusMsg struct{}

// StatusCmd returns a Cmd that delivers a StatusMsg to the root model.
func StatusCmd(text string, level StatusLevel) tea.Cmd {
	return func() tea.Msg {
		return StatusMsg{Text: text, Level: level}
	}
}

// ClearStatusAfter returns a Cmd that fires ClearStatusMsg after d.
func ClearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return ClearStatusMsg{}
	})
}

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

// FocusSidebarMsg is sent by content views (board, project detail) to shift
// focus to the sidebar within their parent combined view.
type FocusSidebarMsg struct{}

// RequestExitMsg is sent by child views when the user wants to quit
type RequestExitMsg struct{}

func SwitchView(v ViewType) tea.Cmd {
	return func() tea.Msg {
		return SwitchViewMsg{View: v}
	}
}
