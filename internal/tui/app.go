package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"time"
	agendapkg "wydo/internal/agenda"
	"wydo/internal/config"
	"wydo/internal/kanban/fs"
	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/kanban/operations"
	"wydo/internal/logs"
	"wydo/internal/notes"
	"wydo/internal/scanner"
	"wydo/internal/tasks/service"
	agendaview "wydo/internal/tui/agenda"
	kanbanview "wydo/internal/tui/kanban"
	"wydo/internal/tui/messages"
	notesview "wydo/internal/tui/notes"
	projectsview "wydo/internal/tui/projects"
	"wydo/internal/tui/shared"
	taskview "wydo/internal/tui/tasks"
	"wydo/internal/tui/theme"
	"wydo/internal/workspace"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const maxContentWidth = 120

// AppModel is the root model that dispatches to child views
type AppModel struct {
	cfg         *config.Config
	workspaces  []*workspace.Workspace
	taskSvc     service.TaskService // combined task service across all workspaces
	boards      []kanbanmodels.Board
	allNotes    []notes.Note
	currentView    ViewType
	lastAgendaView ViewType
	dayView        agendaview.DayModel
	weekView    agendaview.WeekModel
	monthView   agendaview.MonthModel
	kanbanView kanbanview.CombinedModel
	taskManagerView     taskview.TaskManagerModel
	projectsView projectsview.CombinedModel
	notesView           notesview.NotesModel
	showHelp       bool
	exitConfirming bool
	width          int
	height         int
	ready          bool
	statusMessage string
	statusLevel   StatusLevel
	statusExpiry  time.Time
}

// NewAppModel creates the root application model
func NewAppModel(cfg *config.Config, workspaces []*workspace.Workspace) AppModel {
	// Aggregate boards and notes from all workspaces for display
	var allBoards []kanbanmodels.Board
	var allNotes []notes.Note
	var allTaskDirs []scanner.TaskDirInfo

	for _, ws := range workspaces {
		allBoards = append(allBoards, ws.Boards...)
		allNotes = append(allNotes, ws.Notes...)
		allTaskDirs = append(allTaskDirs, ws.TaskDirs...)
	}

	// Build combined task service
	var taskSvc service.TaskService
	if len(allTaskDirs) > 0 {
		var err error
		taskSvc, err = service.NewTaskService(allTaskDirs)
		if err != nil {
			logs.Logger.Printf("Warning: could not create task service: %v", err)
		}
	}

	view := ViewTaskManager
	switch cfg.DefaultView {
	case "day":
		view = ViewAgendaDay
	case "week":
		view = ViewAgendaWeek
	case "month":
		view = ViewAgendaMonth
	case "tasks":
		view = ViewTaskManager
	case "boards":
		view = ViewKanbanPicker
	case "projects":
		view = ViewProjects
	}

	// Compute available boards/ directories for the picker.
	// Always use <workspace>/boards/ for each workspace.
	seen := make(map[string]bool)
	var availableDirs []string

	for _, ws := range workspaces {
		dir := filepath.Join(ws.RootDir, "boards")
		if !seen[dir] {
			seen[dir] = true
			availableDirs = append(availableDirs, dir)
		}
	}
	sort.Strings(availableDirs)

	defaultDir := ""
	if len(availableDirs) > 0 {
		defaultDir = availableDirs[0]
	}

	projDates := collectProjectDates(workspaces)

	app := AppModel{
		cfg:             cfg,
		workspaces:      workspaces,
		taskSvc:         taskSvc,
		boards:          allBoards,
		allNotes:        allNotes,
		currentView:     view,
		lastAgendaView:  ViewAgendaDay,
		dayView:         agendaview.NewDayModel(taskSvc, allBoards, allNotes, projDates),
		weekView:        agendaview.NewWeekModel(taskSvc, allBoards, allNotes, projDates),
		monthView:       agendaview.NewMonthModel(taskSvc, allBoards, allNotes, projDates),
		kanbanView:      kanbanview.NewCombinedModel(allBoards, defaultDir, availableDirs),
		taskManagerView: taskview.NewTaskManagerModel(taskSvc, cfg.Workspaces, allBoards, collectAllProjects(workspaces)),
		projectsView:    projectsview.NewCombinedModel(workspaces),
		notesView:       notesview.NewNotesModel(workspaces),
	}

	// If a specific board was requested, find and open it directly
	if cfg.DefaultBoard != "" {
		if board, ok := findBoard(allBoards, cfg.DefaultBoard); ok {
			if loaded, err := fs.ReadBoardFull(board.Path, board.WSRoot); err == nil {
				app.kanbanView.LoadBoard(loaded, collectAllProjects(workspaces), allBoards, projectsForBoard(workspaces, board.Path))
				app.currentView = ViewKanbanPicker
			}
		}
	}

	return app
}

func (m AppModel) Init() tea.Cmd {
	return m.kanbanView.Init()
}

// setStatus sets a transient status message in the tab bar and returns an auto-clear timer.
func (m *AppModel) setStatus(text string, level StatusLevel) tea.Cmd {
	m.statusMessage = text
	m.statusLevel = level
	m.statusExpiry = time.Now().Add(4 * time.Second)
	return messages.ClearStatusAfter(4 * time.Second)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case StatusMsg:
		return m, m.setStatus(msg.Text, msg.Level)

	case ClearStatusMsg:
		if time.Now().After(m.statusExpiry) {
			m.statusMessage = ""
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		contentHeight := msg.Height - 2 // 2 for tab bar
		contentWidth := min(msg.Width, maxContentWidth)
		m.dayView.SetSize(contentWidth, contentHeight-2) // -2 for agenda sub-bar
		m.weekView.SetSize(contentWidth, contentHeight-2)
		m.monthView.SetSize(contentWidth, contentHeight-2)
		m.kanbanView.SetSize(msg.Width, contentHeight)
		m.taskManagerView.SetSize(contentWidth, contentHeight)
		m.projectsView.SetSize(msg.Width, contentHeight)
		m.notesView.SetSize(msg.Width, contentHeight)
		return m, nil

	case OpenBoardMsg:
		// Find the board from already-loaded workspaces (they have full columns)
		var board *kanbanmodels.Board
		for i := range m.boards {
			if m.boards[i].Path == msg.BoardPath {
				board = &m.boards[i]
				break
			}
		}
		if board == nil {
			return m, nil
		}
		m.kanbanView.LoadBoard(*board, collectAllProjects(m.workspaces), m.boards, projectsForBoard(m.workspaces, msg.BoardPath))
		if msg.ColIndex > 0 || msg.CardIndex > 0 {
			m.kanbanView.NavigateTo(msg.ColIndex, msg.CardIndex)
		}
		m.currentView = ViewKanbanPicker
		return m, m.kanbanView.Init()

	case OpenProjectMsg:
		// Find workspace by RootDir, build detail model, load into combined view
		for _, ws := range m.workspaces {
			if ws.RootDir == msg.WorkspaceRootDir {
				proj := ws.Projects.Get(msg.ProjectName)
				projTasks := ws.Projects.TasksForProject(msg.ProjectName, ws.Tasks)
				projCards := ws.Projects.TaskNotesForProject(msg.ProjectName, ws.Boards)
				projBoards := ws.Projects.BoardsForProject(msg.ProjectName, ws.Boards)
				children := ws.Projects.ChildrenOf(msg.ProjectName)
				allProjectItems := collectAllProjects(m.workspaces)
				allTags := taskview.ExtractUniqueTags(ws.Tasks)
				detail := projectsview.NewDetailModel(
					msg.ProjectName, msg.WorkspaceRootDir,
					projTasks, projCards, projBoards, ws.Boards,
					proj, ws.Projects, children, ws.Tasks,
					allProjectItems, allTags,
				)
				m.projectsView.LoadDetail(detail)
				m.currentView = ViewProjects
				break
			}
		}
		return m, nil

	case SwitchViewMsg:
		m.currentView = msg.View
		// Refresh data when switching to certain views
		switch msg.View {
		case ViewAgendaDay:
			m.lastAgendaView = ViewAgendaDay
			m.refreshData()
			m.dayView.SetData(m.taskSvc, m.boards, m.allNotes, collectProjectDates(m.workspaces))
		case ViewAgendaWeek:
			m.lastAgendaView = ViewAgendaWeek
			m.refreshData()
			m.weekView.SetData(m.taskSvc, m.boards, m.allNotes, collectProjectDates(m.workspaces))
		case ViewAgendaMonth:
			m.lastAgendaView = ViewAgendaMonth
			m.refreshData()
			m.monthView.SetData(m.taskSvc, m.boards, m.allNotes, collectProjectDates(m.workspaces))
		case ViewKanbanPicker:
			m.refreshData()
			m.kanbanView.SetBoards(m.boards)
		case ViewProjects:
			m.refreshData()
			m.projectsView.SetData(m.workspaces)
		case ViewNotes:
			m.refreshData()
			m.notesView.SetData(m.workspaces)
		}
		return m, nil

	case FocusTaskMsg:
		m.currentView = ViewTaskManager
		m.taskManagerView.SetData(m.taskSvc)
		m.taskManagerView.FocusTask(msg.TaskID)
		return m, nil

	case taskview.TaskUpdateMsg:
		// A task was updated in the task manager — persist it
		if msg.Task.File == "" {
			added, err := m.taskSvc.Add(msg.Task.String())
			if err != nil {
				logs.Logger.Printf("Error adding new task: %v", err)
			}
			m.taskManagerView.SetData(m.taskSvc)
			var cmds []tea.Cmd
			if added != nil {
				cmds = append(cmds, m.setStatus(fmt.Sprintf("%q added to %s", msg.Task.Name, added.File), LevelSuccess))
			}
			if m.currentView == ViewProjects {
				if projName := m.projectsView.ActiveProjectName(); projName != "" {
					_, wsDir := m.projectsView.OpenInfo()
					m.refreshData()
					m.projectsView.SetData(m.workspaces)
					cmds = append(cmds, func() tea.Msg {
						return OpenProjectMsg{ProjectName: projName, WorkspaceRootDir: wsDir}
					})
				}
			}
			return m, tea.Batch(cmds...)
		}
		if err := m.taskSvc.Update(msg.Task); err != nil {
			logs.Logger.Printf("Error updating task: %v", err)
		}
		m.taskManagerView.SetData(m.taskSvc)
		return m, nil

	case taskview.TaskDeleteMsg:
		if err := m.taskSvc.Delete(msg.TaskID); err != nil {
			logs.Logger.Printf("Error deleting task: %v", err)
		}
		m.taskManagerView.SetData(m.taskSvc)
		return m, nil

	case taskview.MoveTaskToBoardMsg:
		// Find the board from already-loaded workspaces
		var boardPtr *kanbanmodels.Board
		for i := range m.boards {
			if m.boards[i].Path == msg.BoardPath {
				boardPtr = &m.boards[i]
				break
			}
		}
		if boardPtr == nil {
			return m, m.setStatus(fmt.Sprintf("Board not found: %s", msg.BoardPath), LevelError)
		}
		board := *boardPtr

		// Parse dates from task tags
		var dueDate, scheduledDate *time.Time
		if d := msg.Task.GetDueDate(); d != "" {
			if t, err := time.Parse("2006-01-02", d); err == nil {
				dueDate = &t
			}
		}
		if d := msg.Task.GetScheduledDate(); d != "" {
			if t, err := time.Parse("2006-01-02", d); err == nil {
				scheduledDate = &t
			}
		}

		priority := operations.TaskPriorityToTaskNotePriority(rune(msg.Task.Priority))

		// Merge board projects into task projects
		projects := msg.Task.Projects
		for _, bp := range projectsForBoard(m.workspaces, msg.BoardPath) {
			found := false
			for _, p := range projects {
				if strings.EqualFold(p, bp) {
					found = true
					break
				}
			}
			if !found {
				projects = append(projects, bp)
			}
		}

		// Create the card
		_, err := operations.CreateTaskNoteFromTask(&board, msg.Task.Name, projects, msg.Task.Tags, dueDate, scheduledDate, priority)
		if err != nil {
			return m, m.setStatus(fmt.Sprintf("Error creating card: %v", err), LevelError)
		}

		// Delete the task (prefer duplication over data loss — card already created)
		if err := m.taskSvc.Delete(msg.Task.ID); err != nil {
			logs.Logger.Printf("Warning: card created but task deletion failed: %v", err)
			m.taskManagerView.SetData(m.taskSvc)
			return m, m.setStatus(fmt.Sprintf("Card created but could not delete task: %v", err), LevelWarning)
		}

		m.taskManagerView.SetData(m.taskSvc)
		return m, m.setStatus(fmt.Sprintf("Moved \"%s\" to board \"%s\"", msg.Task.Name, board.Name), LevelSuccess)

	case taskview.ArchiveSelectionRequestMsg:
		// Archive specifically selected tasks
		if err := m.taskSvc.ArchiveByIDs(msg.IDs); err != nil {
			logs.Logger.Printf("Error archiving selected tasks: %v", err)
			return m, nil
		}
		m.taskManagerView.SetData(m.taskSvc)
		return m, func() tea.Msg {
			return taskview.ArchiveCompleteMsg{Count: len(msg.IDs)}
		}

	case taskview.DeleteSelectionRequestMsg:
		// Permanently delete specifically selected tasks in one write+reload
		if err := m.taskSvc.DeleteByIDs(msg.IDs); err != nil {
			logs.Logger.Printf("Error deleting tasks: %v", err)
			return m, nil
		}
		m.taskManagerView.SetData(m.taskSvc)
		return m, func() tea.Msg {
			return taskview.DeleteCompleteMsg{Count: len(msg.IDs)}
		}

	case taskview.ConvertToComplexSelectionMsg:
		var boardPtr *kanbanmodels.Board
		for i := range m.boards {
			if m.boards[i].Path == msg.BoardPath {
				boardPtr = &m.boards[i]
				break
			}
		}
		if boardPtr == nil {
			if msg.BoardPath == "" {
				return m, m.setStatus("No workspace available for conversion", LevelError)
			}
			return m, m.setStatus(fmt.Sprintf("Board not found: %s", msg.BoardPath), LevelError)
		}
		board := *boardPtr

		converted := 0
		failed := 0
		var idsToDelete []string
		for _, task := range msg.Tasks {
			var dueDate, scheduledDate *time.Time
			if d := task.GetDueDate(); d != "" {
				if t, err := time.Parse("2006-01-02", d); err == nil {
					dueDate = &t
				}
			}
			if d := task.GetScheduledDate(); d != "" {
				if t, err := time.Parse("2006-01-02", d); err == nil {
					scheduledDate = &t
				}
			}

			priority := operations.TaskPriorityToTaskNotePriority(rune(task.Priority))

			projects := mergeProjects(task.Projects, projectsForBoard(m.workspaces, msg.BoardPath))

			_, err := operations.CreateTaskNoteFromTask(&board, task.Name, projects, task.Tags, dueDate, scheduledDate, priority)
			if err != nil {
				logs.Logger.Printf("Error creating card for task %q: %v", task.Name, err)
				failed++
				continue
			}

			idsToDelete = append(idsToDelete, task.ID)
			converted++
		}

		if len(idsToDelete) > 0 {
			if err := m.taskSvc.DeleteByIDs(idsToDelete); err != nil {
				logs.Logger.Printf("Error deleting converted tasks: %v", err)
			}
		}

		m.taskManagerView.SetData(m.taskSvc)
		if failed > 0 {
			return m, m.setStatus(fmt.Sprintf("Converted %d task(s), %d failed", converted, failed), LevelWarning)
		}
		if board.Path == "" {
			return m, m.setStatus(fmt.Sprintf("Converted %d task(s) to tasknotes", converted), LevelSuccess)
		}
		return m, m.setStatus(fmt.Sprintf("Converted %d task(s) to tasknotes on board \"%s\"", converted, board.Name), LevelSuccess)

	case taskview.ArchiveRequestMsg:
		// Archive all completed tasks
		if err := m.taskSvc.Archive(); err != nil {
			logs.Logger.Printf("Error archiving tasks: %v", err)
			return m, nil
		}
		m.taskManagerView.SetData(m.taskSvc)
		return m, func() tea.Msg {
			return taskview.ArchiveCompleteMsg{Count: msg.Count}
		}

	case CreateSubProjectMsg:
		for _, ws := range m.workspaces {
			if ws.RootDir == msg.WsDir {
				projectsDir := filepath.Dir(msg.ParentProject.FilePath)
				if err := os.MkdirAll(projectsDir, 0o755); err != nil {
					logs.Logger.Printf("Error creating projects dir: %v", err)
					break
				}
				projectFile := filepath.Join(projectsDir, msg.Name+".md")
				if err := os.WriteFile(projectFile, []byte("# "+msg.Name+"\n"), 0o644); err != nil {
					logs.Logger.Printf("Error writing project file: %v", err)
				}
				break
			}
		}
		return m, func() tea.Msg { return DataRefreshMsg{} }

	case DataRefreshMsg:
		m.refreshData()
		// Push fresh data into every loaded model, not just the active view.
		projDates := collectProjectDates(m.workspaces)
		m.kanbanView.SetBoards(m.boards)
		m.taskManagerView.SetBoards(m.boards)
		m.taskManagerView.SetData(m.taskSvc)
		m.projectsView.SetData(m.workspaces)
		m.notesView.SetData(m.workspaces)
		m.dayView.SetData(m.taskSvc, m.boards, m.allNotes, projDates)
		m.weekView.SetData(m.taskSvc, m.boards, m.allNotes, projDates)
		m.monthView.SetData(m.taskSvc, m.boards, m.allNotes, projDates)
		if boardPath := m.kanbanView.BoardPath(); boardPath != "" {
			// Find the reloaded board from workspaces (which have full columns)
			var found *kanbanmodels.Board
			for i := range m.boards {
				if m.boards[i].Path == boardPath {
					found = &m.boards[i]
					break
				}
			}
			if found != nil {
				m.kanbanView.SetBoard(*found)
				m.kanbanView.SetAllProjects(collectAllProjects(m.workspaces))
				m.kanbanView.SetBoardProjects(projectsForBoard(m.workspaces, boardPath))
			} else {
				// Board no longer exists (e.g. was deleted) — unload it
				logs.Logger.Printf("DataRefreshMsg: board not found at %s, unloading", boardPath)
				m.kanbanView.UnloadBoard()
			}
		}
		if projName := m.projectsView.ActiveProjectName(); projName != "" && m.currentView == ViewProjects {
			_, wsDir := m.projectsView.OpenInfo()
			return m, func() tea.Msg {
				return OpenProjectMsg{ProjectName: projName, WorkspaceRootDir: wsDir}
			}
		}
		return m, nil

	case RequestExitMsg:
		m.exitConfirming = true
		return m, nil

	case tea.KeyMsg:
		// Exit confirmation modal intercepts all keys
		if m.exitConfirming {
			switch msg.String() {
			case "y", "enter", "ctrl+c":
				return m, tea.Quit
			case "esc", "n":
				m.exitConfirming = false
				return m, nil
			}
			return m, nil
		}

		// Global keys: ctrl+c triggers exit confirmation
		if msg.String() == "ctrl+c" {
			m.exitConfirming = true
			return m, nil
		}

		// Dismiss help overlay on any key
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		// Global view-switching (uppercase) — works in all views when not in modal/typing state
		if !m.isChildInputActive() {
			switch msg.String() {
			case "N":
				m.currentView = ViewNotes
				m.refreshData()
				m.notesView.SetData(m.workspaces)
				return m, nil
			case "P":
				m.refreshData()
				m.currentView = ViewProjects
				m.projectsView.SetData(m.workspaces)
				if projName := m.projectsView.ActiveProjectName(); projName != "" {
					m.projectsView.FocusDetail()
				}
				return m, nil
			case "B":
				m.refreshData()
				m.currentView = ViewKanbanPicker
				m.kanbanView.SetBoards(m.boards)
				if boardPath := m.kanbanView.BoardPath(); boardPath != "" {
					var found *kanbanmodels.Board
					for i := range m.boards {
						if m.boards[i].Path == boardPath {
							found = &m.boards[i]
							break
						}
					}
					if found != nil {
						m.kanbanView.SetBoard(*found)
					} else {
						logs.Logger.Printf("B key: board not found at %s", boardPath)
					}
					m.kanbanView.SetAllProjects(collectAllProjects(m.workspaces))
					m.kanbanView.FocusBoard()
				}
				return m, nil
			case "A":
				m.refreshData()
				m.currentView = m.lastAgendaView
				switch m.lastAgendaView {
				case ViewAgendaWeek:
					m.weekView.SetData(m.taskSvc, m.boards, m.allNotes, collectProjectDates(m.workspaces))
				case ViewAgendaMonth:
					m.monthView.SetData(m.taskSvc, m.boards, m.allNotes, collectProjectDates(m.workspaces))
				default:
					m.dayView.SetData(m.taskSvc, m.boards, m.allNotes, collectProjectDates(m.workspaces))
				}
				return m, nil
			case "T":
				if m.currentView != ViewTaskManager {
					m.currentView = ViewTaskManager
					m.taskManagerView.SetData(m.taskSvc)
					m.taskManagerView.SetBoards(m.boards)
				}
				return m, nil
			}
		}

		// Global ? help — works in all views when not in modal/typing state
		if msg.String() == "?" && !m.isChildInputActive() {
			m.showHelp = true
			return m, nil
		}

		// For board/picker views and task manager in modal state,
		// let the child view handle all keys.
		if m.currentView == ViewKanbanBoard || m.currentView == ViewKanbanPicker || m.currentView == ViewProjects {
			// Don't intercept keys — let child view handle everything
		} else if m.currentView == ViewTaskManager && m.taskManagerView.IsInModalState() {
			// Task manager is in a modal state (editor, picker, search, etc.)
			// Let it handle all keys
		} else if m.currentView == ViewNotes && m.notesView.IsTyping() {
			// Notes view has active text input (file picker, label input)
			// Let it handle all keys
		} else if m.currentView == ViewAgendaDay && m.dayView.IsSearching() {
			// Day agenda search is active — let it handle all keys
		} else if m.currentView == ViewAgendaWeek && m.weekView.IsSearching() {
			// Week agenda search is active — let it handle all keys
		} else {
			// Global navigation keys for agenda/task views
			switch msg.String() {
			case "q":
				m.exitConfirming = true
				return m, nil
			}

			// Agenda sub-view switching — only when already in an agenda view
			if m.currentView == ViewAgendaDay || m.currentView == ViewAgendaWeek || m.currentView == ViewAgendaMonth {
				switch msg.String() {
				case "d", "D":
					m.currentView = ViewAgendaDay
					m.lastAgendaView = ViewAgendaDay
					m.refreshData()
					m.dayView.SetData(m.taskSvc, m.boards, m.allNotes, collectProjectDates(m.workspaces))
					return m, nil
				case "w", "W":
					m.currentView = ViewAgendaWeek
					m.lastAgendaView = ViewAgendaWeek
					m.refreshData()
					m.weekView.SetData(m.taskSvc, m.boards, m.allNotes, collectProjectDates(m.workspaces))
					return m, nil
				case "m", "M":
					m.currentView = ViewAgendaMonth
					m.lastAgendaView = ViewAgendaMonth
					m.refreshData()
					m.monthView.SetData(m.taskSvc, m.boards, m.allNotes, collectProjectDates(m.workspaces))
					return m, nil
				}
			}
		}
	}

	// Dispatch to current child view
	var cmd tea.Cmd
	switch m.currentView {
	case ViewAgendaDay:
		m.dayView, cmd = m.dayView.Update(msg)
		return m, cmd
	case ViewAgendaWeek:
		m.weekView, cmd = m.weekView.Update(msg)
		return m, cmd
	case ViewAgendaMonth:
		m.monthView, cmd = m.monthView.Update(msg)
		return m, cmd
	case ViewKanbanPicker, ViewKanbanBoard:
		m.kanbanView, cmd = m.kanbanView.Update(msg)
		return m, cmd
	case ViewTaskManager:
		m.taskManagerView, cmd = m.taskManagerView.Update(msg)
		return m, cmd
	case ViewProjects, ViewProjectDetail:
		m.projectsView, cmd = m.projectsView.Update(msg)
		return m, cmd
	case ViewNotes:
		m.notesView, cmd = m.notesView.Update(msg)
		return m, cmd
	}

	return m, nil
}

// refreshData rescans workspaces and refreshes aggregated data
func (m *AppModel) refreshData() {
	var allBoards []kanbanmodels.Board
	var allNotes []notes.Note
	var allTaskDirs []scanner.TaskDirInfo
	var freshWorkspaces []*workspace.Workspace

	for _, wsDir := range m.cfg.Workspaces {
		scan, err := scanner.ScanWorkspace(wsDir)
		if err != nil {
			continue
		}
		ws, err := workspace.Load(scan)
		if err != nil {
			continue
		}
		freshWorkspaces = append(freshWorkspaces, ws)
		allBoards = append(allBoards, ws.Boards...)
		allNotes = append(allNotes, ws.Notes...)
		allTaskDirs = append(allTaskDirs, scan.TaskDirs...)
	}

	m.workspaces = freshWorkspaces
	m.boards = allBoards
	m.allNotes = allNotes

	if len(allTaskDirs) > 0 {
		if svc, err := service.NewTaskService(allTaskDirs); err == nil {
			m.taskSvc = svc
		}
	}
}

// isChildInputActive returns true when the current child view has an active text input
// or modal that should receive uppercase keys instead of the global view-switcher.
func (m *AppModel) isChildInputActive() bool {
	if m.exitConfirming {
		return true
	}
	switch m.currentView {
	case ViewKanbanPicker, ViewKanbanBoard:
		return m.kanbanView.IsTyping() || m.kanbanView.IsModal()
	case ViewTaskManager:
		return m.taskManagerView.IsInModalState()
	case ViewProjects, ViewProjectDetail:
		return m.projectsView.IsTyping() || m.projectsView.IsModal()
	case ViewNotes:
		return m.notesView.IsTyping()
	case ViewAgendaDay:
		return m.dayView.IsSearching()
	case ViewAgendaWeek:
		return m.weekView.IsSearching()
	default:
		return false
	}
}

// collectProjectDates collects all labeled project dates from all workspaces.
func collectProjectDates(workspaces []*workspace.Workspace) []agendapkg.ProjectDateSource {
	var result []agendapkg.ProjectDateSource
	for _, ws := range workspaces {
		if ws.Projects == nil {
			continue
		}
		for _, p := range ws.Projects.List() {
			for _, d := range p.Dates {
				result = append(result, agendapkg.ProjectDateSource{
					ProjectName: p.Name,
					Label:       d.Label,
					Date:        d.Date,
				})
			}
		}
	}
	return result
}

// collectAllProjects returns projects in hierarchical DFS order with depth metadata,
// from all workspaces (directories, task +tags, and card frontmatter).
func collectAllProjects(workspaces []*workspace.Workspace) []kanbanview.ProjectPickerItem {
	// Collect all unique projects across workspaces
	seen := make(map[string]*workspace.Project)
	for _, ws := range workspaces {
		if ws.Projects == nil {
			continue
		}
		for _, p := range ws.Projects.List() {
			if _, exists := seen[p.Name]; !exists {
				seen[p.Name] = p
			}
		}
	}

	// Build parent -> children adjacency (sorted)
	children := make(map[string][]string)
	for name, p := range seen {
		children[p.Parent] = append(children[p.Parent], name)
	}
	for k := range children {
		sort.Strings(children[k])
	}

	// DFS from roots to build ordered list with depths
	var result []kanbanview.ProjectPickerItem
	visited := make(map[string]bool)

	var dfs func(name string, depth int)
	dfs = func(name string, depth int) {
		if visited[name] {
			return
		}
		visited[name] = true
		p := seen[name]
		dirPath := ""
		if p != nil {
			dirPath = p.FilePath
		}
		result = append(result, kanbanview.ProjectPickerItem{Name: name, Depth: depth, FilePath: dirPath})
		for _, child := range children[name] {
			dfs(child, depth+1)
		}
	}

	// Visit roots (Parent == "") in sorted order
	roots := children[""]
	for _, root := range roots {
		dfs(root, 0)
	}

	// Append any unvisited projects (virtual projects without matching parent) at depth 0
	remaining := make([]string, 0)
	for name := range seen {
		if !visited[name] {
			remaining = append(remaining, name)
		}
	}
	sort.Strings(remaining)
	for _, name := range remaining {
		p := seen[name]
		dirPath := ""
		if p != nil {
			dirPath = p.FilePath
		}
		result = append(result, kanbanview.ProjectPickerItem{Name: name, Depth: 0, FilePath: dirPath})
	}

	return result
}

// mergeProjects returns a new slice containing all items from base plus any
// items in extra that are not already present (case-insensitive).
func mergeProjects(base, extra []string) []string {
	result := append([]string(nil), base...)
	for _, e := range extra {
		found := false
		for _, b := range result {
			if strings.EqualFold(b, e) {
				found = true
				break
			}
		}
		if !found {
			result = append(result, e)
		}
	}
	return result
}

// projectsForBoard returns the project names (immediate + ancestors) linked to
// the board at boardPath via the board's project frontmatter field, or nil if none.
func projectsForBoard(workspaces []*workspace.Workspace, boardPath string) []string {
	for _, ws := range workspaces {
		if ws.Projects == nil {
			continue
		}
		if names := ws.Projects.ProjectsForBoard(boardPath, ws.Boards); len(names) > 0 {
			return names
		}
	}
	return nil
}

// findBoard looks up a board by name or directory basename (case-insensitive).
func findBoard(boards []kanbanmodels.Board, query string) (kanbanmodels.Board, bool) {
	q := strings.ToLower(query)
	for _, b := range boards {
		if strings.ToLower(b.Name) == q || strings.ToLower(filepath.Base(b.Path)) == q {
			return b, true
		}
	}
	return kanbanmodels.Board{}, false
}

func (m AppModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.showHelp {
		return m.renderHelpOverlay()
	}

	bg := m.renderBackground()

	if m.exitConfirming {
		d := shared.Dialog{
			Title: "Quit wydo?",
			Hints: theme.Ok.Render("[y/enter]") + " Confirm  " + theme.Error.Render("[esc]") + " Cancel",
			Width: 40,
		}
		return shared.PlaceOverlay(bg, d.View(), m.width, m.height)
	}

	if m.currentView == ViewTaskManager {
		if overlay := m.taskManagerView.OverlayView(); overlay != "" {
			return shared.PlaceOverlay(bg, overlay, m.width, m.height)
		}
	}

	return bg
}

func (m AppModel) renderBackground() string {
	var content string
	centerContent := false

	switch m.currentView {
	case ViewAgendaDay:
		content = m.dayView.View()
		centerContent = true
	case ViewAgendaWeek:
		content = m.weekView.View()
		centerContent = true
	case ViewAgendaMonth:
		content = m.monthView.View()
		centerContent = true
	case ViewKanbanPicker, ViewKanbanBoard:
		content = m.kanbanView.View()
	case ViewTaskManager:
		content = m.taskManagerView.View()
		centerContent = true
	case ViewProjects, ViewProjectDetail:
		content = m.projectsView.View()
	case ViewNotes:
		content = m.notesView.View()
	}

	if centerContent && m.width > maxContentWidth {
		leftPad := strings.Repeat(" ", (m.width-maxContentWidth)/2)
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			lines[i] = leftPad + line
		}
		content = strings.Join(lines, "\n")
	}

	tabBar := m.renderTabBar()
	rows := []string{tabBar}
	if subBar := m.renderAgendaSubBar(); subBar != "" {
		rows = append(rows, subBar)
	}
	rows = append(rows, content)
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderAgendaSubBar renders a secondary indicator row below the tab bar
// showing the active agenda sub-view (Day / Week / Month). Returns empty
// string when not in an agenda view.
func (m AppModel) renderAgendaSubBar() string {
	subViews := []struct {
		label string
		view  ViewType
	}{
		{"Day", ViewAgendaDay},
		{"Week", ViewAgendaWeek},
		{"Month", ViewAgendaMonth},
	}

	var found bool
	for _, sv := range subViews {
		if m.currentView == sv.view {
			found = true
			break
		}
	}
	if !found {
		return ""
	}

	var parts []string
	for _, sv := range subViews {
		if m.currentView == sv.view {
			parts = append(parts, theme.NavActive.Render(sv.label))
		} else {
			parts = append(parts, theme.NavInactive.Render(sv.label))
		}
	}
	style := lipgloss.NewStyle().Width(m.width).PaddingLeft(1)
	return style.Render(strings.Join(parts, "  "))
}

// renderTabBar renders the top tab bar with the active view highlighted.
// The right side shows a transient status/alert message when present.
func (m AppModel) renderTabBar() string {
	tabs := []string{"Board", "Agenda", "Tasks", "Projects", "Notes"}

	// Map current view to active tab index
	activeIdx := -1
	switch m.currentView {
	case ViewKanbanPicker, ViewKanbanBoard:
		activeIdx = 0
	case ViewAgendaDay, ViewAgendaWeek, ViewAgendaMonth:
		activeIdx = 1
	case ViewTaskManager:
		activeIdx = 2
	case ViewProjects, ViewProjectDetail:
		activeIdx = 3
	case ViewNotes:
		activeIdx = 4
	}

	var parts []string
	for i, label := range tabs {
		if i == activeIdx {
			parts = append(parts, theme.TabActive.Render(label))
		} else {
			parts = append(parts, theme.TabInactive.Render(label))
		}
	}
	tabContent := strings.Join(parts, " ")

	// Right-side status message
	var statusContent string
	if m.statusMessage != "" {
		var sty lipgloss.Style
		switch m.statusLevel {
		case LevelError:
			sty = theme.Error
		case LevelWarning:
			sty = theme.Warn
		default:
			sty = theme.Ok
		}
		statusContent = sty.Render(m.statusMessage)
	}

	// theme.TabBar has PaddingLeft(1); account for it so status is truly right-aligned
	tabWidth := lipgloss.Width(tabContent)
	statusWidth := lipgloss.Width(statusContent)
	gap := (m.width - 1) - tabWidth - statusWidth
	if gap < 1 {
		gap = 1
	}

	line := tabContent + strings.Repeat(" ", gap) + statusContent
	return theme.TabBar.Width(m.width).Render(line)
}


func (m AppModel) renderHelpOverlay() string {
	globalNav := shared.HelpSection{
		Title: "Global Navigation",
		Binds: []shared.HelpBind{
			{"N", "Notes"},
			{"P", "Projects"},
			{"B", "Board picker"},
			{"A", "Agenda (day view)"},
			{"T", "Task manager"},
			{"D / W / M", "Day / week / month"},
			{"?", "Show this help"},
			{"q", "Quit"},
		},
	}

	var sections []shared.HelpSection
	sections = append(sections, globalNav)

	switch m.currentView {
	case ViewTaskManager:
		sections = append(sections, shared.HelpSection{
			Title: "Task Manager",
			Binds: []shared.HelpBind{
				{"j / k", "Navigate tasks"},
				{"enter", "Open task editor"},
				{"space", "Toggle done"},
				{"d", "Due date"},
				{"s", "Scheduled date"},
				{"t", "Contexts"},
				{"p", "Projects"},
				{"i", "Cycle priority"},
				{"U", "Edit URLs"},
				{"u", "Open URL"},
				{"n", "New task"},
				{"D", "Delete task"},
				{"m", "Move to board"},
				{"/", "Search"},
				{"f", "Filter options"},
				{"S", "Sort options"},
				{"g", "Group options"},
				{"F", "File view"},
				{"W", "Workspace filter"},
			},
		})
	case ViewKanbanBoard:
		sections = append(sections, shared.HelpSection{
			Title: "Board",
			Binds: []shared.HelpBind{
				{"h / l", "Navigate columns"},
				{"j / k", "Navigate cards"},
				{"enter", "Edit card"},
				{"n", "New card"},
				{"d", "Due date"},
				{"s", "Scheduled date"},
				{"t", "Tags"},
				{"p", "Projects"},
				{"i", "Priority"},
				{"U", "Edit URLs"},
				{"u", "Open URL"},
				{"m / space", "Move card"},
				{"M", "Move to board"},
				{"D", "Delete card"},
				{"c", "Edit columns"},
				{"/", "Filter"},
				{"x", "Start work / switch to session"},
				{"X", "Link tmux session"},
				{"ctrl+j", "Link Jira board"},
				{"J", "Link Jira issue to card"},
				{"a", "Archive / unarchive card"},
				{"ctrl+a", "Toggle show archived"},
				{"esc / q", "Back"},
			},
		})
	case ViewKanbanPicker:
		sections = append(sections, shared.HelpSection{
			Title: "Board Picker",
			Binds: []shared.HelpBind{
				{"j / k", "Navigate"},
				{"enter", "Open board"},
				{"/", "Search"},
				{"n", "New board"},
				{"a", "Archive / unarchive board"},
				{"ctrl+a", "Toggle show archived"},
			},
		})
	case ViewAgendaDay, ViewAgendaWeek:
		sections = append(sections, shared.HelpSection{
			Title: "Agenda",
			Binds: []shared.HelpBind{
				{"h / l", "Previous / next period"},
				{"j / k", "Navigate items"},
				{"t", "Jump to today"},
				{"enter", "Open selected item"},
				{"/", "Search"},
			},
		})
	case ViewAgendaMonth:
		sections = append(sections, shared.HelpSection{
			Title: "Month View",
			Binds: []shared.HelpBind{
				{"h / l", "Previous / next day"},
				{"j / k", "Previous / next week"},
				{"H / L", "Previous / next month"},
				{"t", "Jump to today"},
				{"enter", "Enter detail panel"},
				{"esc", "Back to calendar"},
			},
		})
	case ViewNotes:
		sections = append(sections, shared.HelpSection{
			Title: "Notes",
			Binds: []shared.HelpBind{
				{"j / k", "Navigate"},
				{"enter", "Open note in editor"},
				{"p", "Pin a new note"},
				{"d", "Unpin selected note"},
				{"esc", "Back"},
			},
		})
	case ViewProjects, ViewProjectDetail:
		sections = append(sections, shared.HelpSection{
			Title: "Projects",
			Binds: []shared.HelpBind{
				{"j / k", "Navigate"},
				{"enter", "Open project"},
				{"space / tab / l / →", "Expand"},
				{"←", "Collapse"},
				{"/", "Search"},
				{"n", "New project"},
				{"r", "Rename project"},
				{"p", "Reparent project"},
				{"a", "Archive / unarchive"},
				{"ctrl+a", "Toggle show archived"},
				{"esc", "Focus detail / back"},
			},
		})
		sections = append(sections, shared.HelpSection{
			Title: "Project Detail",
			Binds: []shared.HelpBind{
				{"[", "Go to parent project"},
				{"]", "Pick child project"},
				{"h / l", "Navigate columns"},
				{"j / k", "Navigate items"},
				{"space / enter", "Expand / open item"},
				{"u", "Open URL(s)"},
				{"U", "Edit URLs"},
				{"d", "Edit dates"},
				{"esc / q", "Back to sidebar"},
			},
		})
	}

	return shared.RenderHelpPopup(sections, m.width, m.height)
}

func (m AppModel) renderPlaceholder(title, subtitle string) string {
	titleStr := TitleStyle.Render(title)
	subtitleStr := HelpStyle.Render(subtitle)
	return lipgloss.JoinVertical(lipgloss.Left, "", titleStr, subtitleStr, "")
}

