package tui

import "wydo/internal/tui/messages"

// Re-export types from messages package for convenience
type ViewType = messages.ViewType

const (
	ViewAgendaDay    = messages.ViewAgendaDay
	ViewAgendaWeek   = messages.ViewAgendaWeek
	ViewAgendaMonth  = messages.ViewAgendaMonth
	ViewKanbanPicker = messages.ViewKanbanPicker
	ViewKanbanBoard  = messages.ViewKanbanBoard
	ViewTaskManager  = messages.ViewTaskManager
	ViewProjects     = messages.ViewProjects
)

type SwitchViewMsg = messages.SwitchViewMsg
type OpenBoardMsg = messages.OpenBoardMsg
type FocusTaskMsg = messages.FocusTaskMsg
type DataRefreshMsg = messages.DataRefreshMsg
