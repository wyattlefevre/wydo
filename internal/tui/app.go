package tui

import (
	"path/filepath"
	"strings"

	"time"
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
	projectsview "wydo/internal/tui/projects"
	taskview "wydo/internal/tui/tasks"
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
	currentView ViewType
	dayView     agendaview.DayModel
	weekView    agendaview.WeekModel
	monthView   agendaview.MonthModel
	pickerView  kanbanview.PickerModel
	boardView   kanbanview.BoardModel
	boardLoaded bool // true when boardView has a valid board
	taskManagerView    taskview.TaskManagerModel
	projectsView      projectsview.ProjectsModel
	projectDetailView projectsview.DetailModel
	projectDetailLoaded bool
	showHelp    bool
	width       int
	height      int
	ready       bool
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

	view := ViewAgendaDay
	switch cfg.DefaultView {
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

	// Compute available dirs for picker
	availableDirs := make([]string, 0, len(cfg.Workspaces))
	availableDirs = append(availableDirs, cfg.Workspaces...)

	defaultDir := ""
	if len(availableDirs) > 0 {
		defaultDir = availableDirs[0]
	}

	app := AppModel{
		cfg:             cfg,
		workspaces:      workspaces,
		taskSvc:         taskSvc,
		boards:          allBoards,
		allNotes:        allNotes,
		currentView:     view,
		dayView:         agendaview.NewDayModel(taskSvc, allBoards, allNotes),
		weekView:        agendaview.NewWeekModel(taskSvc, allBoards, allNotes),
		monthView:       agendaview.NewMonthModel(taskSvc, allBoards, allNotes),
		pickerView:      kanbanview.NewPickerModel(allBoards, defaultDir, availableDirs),
		taskManagerView: taskview.NewTaskManagerModel(taskSvc, cfg.Workspaces, allBoards),
		projectsView:   projectsview.NewProjectsModel(workspaces),
	}

	// If a specific board was requested, find and open it directly
	if cfg.DefaultBoard != "" {
		if board, ok := findBoard(allBoards, cfg.DefaultBoard); ok {
			loaded, err := fs.ReadBoard(board.Path)
			if err == nil {
				app.boardView = kanbanview.NewBoardModel(loaded)
				app.boardLoaded = true
				app.currentView = ViewKanbanBoard
			}
		}
	}

	return app
}

func (m AppModel) Init() tea.Cmd {
	return nil
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		contentHeight := msg.Height - 3 // Reserve space for status bar
		contentWidth := min(msg.Width, maxContentWidth)
		m.dayView.SetSize(contentWidth, contentHeight)
		m.weekView.SetSize(contentWidth, contentHeight)
		m.monthView.SetSize(contentWidth, contentHeight)
		m.pickerView.SetSize(msg.Width, contentHeight)
		if m.boardLoaded {
			m.boardView.SetSize(msg.Width, contentHeight)
		}
		m.taskManagerView.SetSize(contentWidth, contentHeight)
		m.projectsView.SetSize(msg.Width, contentHeight)
		if m.projectDetailLoaded {
			m.projectDetailView.SetSize(msg.Width, contentHeight)
		}
		return m, nil

	case OpenBoardMsg:
		// Load the board and switch to board view
		board, err := fs.ReadBoard(msg.BoardPath)
		if err != nil {
			// Stay on current view if board can't be loaded
			return m, nil
		}
		m.boardView = kanbanview.NewBoardModel(board)
		m.boardView.SetSize(m.width, m.height-3)
		if msg.ColIndex > 0 || msg.CardIndex > 0 {
			m.boardView.NavigateTo(msg.ColIndex, msg.CardIndex)
		}
		m.boardLoaded = true
		m.currentView = ViewKanbanBoard
		return m, nil

	case OpenProjectMsg:
		// Find workspace by RootDir
		for _, ws := range m.workspaces {
			if ws.RootDir == msg.WorkspaceRootDir {
				proj := ws.Projects.Get(msg.ProjectName)
				projNotes := ws.Projects.NotesForProject(msg.ProjectName, ws.Notes)
				projTasks := ws.Projects.TasksForProject(msg.ProjectName, ws.Tasks)
				projCards := ws.Projects.CardsForProject(msg.ProjectName, ws.Boards)
				projBoards := ws.Projects.BoardsForProject(msg.ProjectName, ws.Boards)
				m.projectDetailView = projectsview.NewDetailModel(
					msg.ProjectName, msg.WorkspaceRootDir,
					projNotes, projTasks, projCards, projBoards,
				)
				_ = proj // proj used for future enhancements
				m.projectDetailView.SetSize(m.width, m.height-3)
				m.projectDetailLoaded = true
				m.currentView = ViewProjectDetail
				break
			}
		}
		return m, nil

	case SwitchViewMsg:
		m.currentView = msg.View
		// Refresh data when switching to certain views
		switch msg.View {
		case ViewAgendaDay:
			m.refreshData()
			m.dayView.SetData(m.taskSvc, m.boards, m.allNotes)
		case ViewAgendaWeek:
			m.refreshData()
			m.weekView.SetData(m.taskSvc, m.boards, m.allNotes)
		case ViewAgendaMonth:
			m.refreshData()
			m.monthView.SetData(m.taskSvc, m.boards, m.allNotes)
		case ViewKanbanPicker:
			m.refreshData()
			m.pickerView.SetBoards(m.boards)
		case ViewProjects:
			m.refreshData()
			m.projectsView.SetData(m.workspaces)
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
			if _, err := m.taskSvc.Add(msg.Task.String()); err != nil {
				logs.Logger.Printf("Error adding new task: %v", err)
			}
		} else {
			if err := m.taskSvc.Update(msg.Task); err != nil {
				logs.Logger.Printf("Error updating task: %v", err)
			}
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
		// Load the board fresh from disk
		board, err := fs.ReadBoard(msg.BoardPath)
		if err != nil {
			return m, tea.Printf("Error loading board: %v", err)
		}

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

		priority := operations.TaskPriorityToCardPriority(rune(msg.Task.Priority))

		// Create the card
		_, err = operations.CreateCardFromTask(&board, msg.Task.Name, msg.Task.Projects, msg.Task.Contexts, dueDate, scheduledDate, priority)
		if err != nil {
			return m, tea.Printf("Error creating card: %v", err)
		}

		// Delete the task (prefer duplication over data loss — card already created)
		if err := m.taskSvc.Delete(msg.Task.ID); err != nil {
			logs.Logger.Printf("Warning: card created but task deletion failed: %v", err)
			m.taskManagerView.SetData(m.taskSvc)
			return m, tea.Printf("Card created but could not delete task: %v", err)
		}

		m.taskManagerView.SetData(m.taskSvc)
		return m, tea.Printf("Moved \"%s\" to board \"%s\"", msg.Task.Name, board.Name)

	case taskview.ArchiveRequestMsg:
		// Archive completed tasks
		if err := m.taskSvc.Archive(); err != nil {
			logs.Logger.Printf("Error archiving tasks: %v", err)
			return m, nil
		}
		m.taskManagerView.SetData(m.taskSvc)
		return m, func() tea.Msg {
			return taskview.ArchiveCompleteMsg{Count: msg.Count}
		}

	case DataRefreshMsg:
		m.refreshData()
		if m.currentView == ViewProjects {
			m.projectsView.SetData(m.workspaces)
		}
		return m, nil

	case tea.KeyMsg:
		// Global keys: ctrl+c always quits
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Dismiss help overlay on any key
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		// Global view-switching (uppercase) — works in all views when not in modal/typing state
		if !m.isChildInputActive() {
			switch msg.String() {
			case "P":
				m.currentView = ViewProjects
				m.refreshData()
				m.projectsView.SetData(m.workspaces)
				return m, nil
			case "B":
				m.currentView = ViewKanbanPicker
				m.refreshData()
				m.pickerView.SetBoards(m.boards)
				return m, nil
			case "A":
				m.currentView = ViewAgendaDay
				m.refreshData()
				m.dayView.SetData(m.taskSvc, m.boards, m.allNotes)
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

		// For board/picker views and task manager in modal state,
		// let the child view handle all keys.
		if m.currentView == ViewKanbanBoard || m.currentView == ViewKanbanPicker || m.currentView == ViewProjectDetail {
			// Don't intercept keys — let child view handle everything
		} else if m.currentView == ViewTaskManager && m.taskManagerView.IsInModalState() {
			// Task manager is in a modal state (editor, picker, search, etc.)
			// Let it handle all keys
		} else if m.currentView == ViewProjects && m.projectsView.IsTyping() {
			// Projects view has active text input (search, create, rename)
			// Let it handle all keys
		} else {
			// Global navigation keys for agenda/task views
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "1":
				m.currentView = ViewAgendaDay
				m.refreshData()
				m.dayView.SetData(m.taskSvc, m.boards, m.allNotes)
				return m, nil
			case "2":
				m.currentView = ViewAgendaWeek
				m.refreshData()
				m.weekView.SetData(m.taskSvc, m.boards, m.allNotes)
				return m, nil
			case "3":
				m.currentView = ViewAgendaMonth
				m.refreshData()
				m.monthView.SetData(m.taskSvc, m.boards, m.allNotes)
				return m, nil
			case "b":
				m.currentView = ViewKanbanPicker
				m.refreshData()
				m.pickerView.SetBoards(m.boards)
				return m, nil
			case "p":
				m.currentView = ViewProjects
				m.refreshData()
				m.projectsView.SetData(m.workspaces)
				return m, nil
			case "?":
				m.showHelp = true
				return m, nil
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
	case ViewKanbanPicker:
		m.pickerView, cmd = m.pickerView.Update(msg)
		return m, cmd
	case ViewKanbanBoard:
		if m.boardLoaded {
			m.boardView, cmd = m.boardView.Update(msg)
			return m, cmd
		}
	case ViewTaskManager:
		m.taskManagerView, cmd = m.taskManagerView.Update(msg)
		return m, cmd
	case ViewProjects:
		m.projectsView, cmd = m.projectsView.Update(msg)
		return m, cmd
	case ViewProjectDetail:
		if m.projectDetailLoaded {
			m.projectDetailView, cmd = m.projectDetailView.Update(msg)
			return m, cmd
		}
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
	switch m.currentView {
	case ViewKanbanBoard:
		return m.boardView.IsModal()
	case ViewKanbanPicker:
		return m.pickerView.IsTyping()
	case ViewTaskManager:
		return m.taskManagerView.IsInModalState()
	case ViewProjects:
		return m.projectsView.IsTyping()
	case ViewProjectDetail:
		return m.projectDetailView.IsModal()
	default:
		return false
	}
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
	case ViewKanbanPicker:
		content = m.pickerView.View()
	case ViewKanbanBoard:
		if m.boardLoaded {
			content = m.boardView.View()
		} else {
			content = m.renderPlaceholder("Board View", "No board loaded")
		}
	case ViewTaskManager:
		content = m.taskManagerView.View()
		centerContent = true
	case ViewProjects:
		content = m.projectsView.View()
	case ViewProjectDetail:
		if m.projectDetailLoaded {
			content = m.projectDetailView.View()
		} else {
			content = m.renderPlaceholder("Project Detail", "No project loaded")
		}
	}

	if centerContent && m.width > maxContentWidth {
		leftPad := strings.Repeat(" ", (m.width-maxContentWidth)/2)
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			lines[i] = leftPad + line
		}
		content = strings.Join(lines, "\n")
	}

	// Status bar — show different hints based on view
	var statusText string
	switch m.currentView {
	case ViewKanbanBoard:
		statusText = "Board view | esc/q/b: back | P:projects A:agenda T:tasks"
	case ViewKanbanPicker:
		statusText = "Board picker | esc: back | P:projects A:agenda T:tasks | q:quit"
	case ViewTaskManager:
		statusText = "Task manager | 1:day 2:week 3:month | B:boards P:projects A:agenda | ?:help | q:quit"
	case ViewProjects:
		statusText = "Projects | 1:day 2:week 3:month | B:boards T:tasks A:agenda | ?:help | q:quit"
	case ViewProjectDetail:
		statusText = "Project detail | tab/1/2/3/4: sections | esc/q: back to projects | P:projects A:agenda T:tasks"
	default:
		statusText = "1:day 2:week 3:month | B:boards T:tasks P:projects | ?:help | q:quit"
	}

	statusBar := StatusBarStyle.Width(m.width).Render(
		HelpStyle.Render(statusText),
	)

	return lipgloss.JoinVertical(lipgloss.Left, content, statusBar)
}

func (m AppModel) renderHelpOverlay() string {
	helpBoxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2)

	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

	line := func(key, desc string) string {
		return "  " + keyStyle.Width(14).Render(key) + descStyle.Render(desc)
	}

	var content string
	content += sectionStyle.Render("Wydo - Keyboard Shortcuts") + "\n\n"

	content += sectionStyle.Render("Global Navigation") + "\n"
	content += line("P", "Projects") + "\n"
	content += line("B", "Board picker") + "\n"
	content += line("A", "Agenda (day view)") + "\n"
	content += line("T", "Task manager") + "\n"
	content += line("1 / 2 / 3", "Day / week / month agenda") + "\n"
	content += line("b", "Board picker") + "\n"
	content += line("p", "Projects") + "\n"
	content += line("?", "Show this help") + "\n"
	content += line("q", "Quit") + "\n"
	content += line("ctrl+c", "Force quit") + "\n\n"

	content += sectionStyle.Render("Agenda Views (Day/Week)") + "\n"
	content += line("h / l", "Previous / next period") + "\n"
	content += line("j / k", "Navigate items") + "\n"
	content += line("t", "Jump to today") + "\n"
	content += line("enter", "Open selected item") + "\n\n"

	content += sectionStyle.Render("Month View") + "\n"
	content += line("h / l", "Previous / next day") + "\n"
	content += line("j / k", "Previous / next week") + "\n"
	content += line("H / L", "Previous / next month") + "\n"
	content += line("t", "Jump to today") + "\n"
	content += line("enter", "Enter detail panel") + "\n"
	content += line("esc", "Back to calendar") + "\n\n"

	content += sectionStyle.Render("Task Manager") + "\n"
	content += line("j / k", "Navigate tasks") + "\n"
	content += line("enter", "Edit task") + "\n"
	content += line("space", "Toggle done") + "\n"
	content += line("n", "New task") + "\n"
	content += line("m", "Move to board") + "\n"
	content += line("f", "Filter") + "\n"
	content += line("s", "Sort") + "\n"
	content += line("g", "Group") + "\n"
	content += line("/", "Search") + "\n\n"

	content += HelpStyle.Render("Press any key to close")

	box := helpBoxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m AppModel) renderPlaceholder(title, subtitle string) string {
	titleStr := TitleStyle.Render(title)
	subtitleStr := HelpStyle.Render(subtitle)
	return lipgloss.JoinVertical(lipgloss.Left, "", titleStr, subtitleStr, "")
}
