package tui

import (
	"wydo/internal/config"
	"wydo/internal/kanban/fs"
	"wydo/internal/kanban/models"
	"wydo/internal/logs"
	"wydo/internal/notes"
	"wydo/internal/tasks/service"
	agendaview "wydo/internal/tui/agenda"
	kanbanview "wydo/internal/tui/kanban"
	taskview "wydo/internal/tui/tasks"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AppModel is the root model that dispatches to child views
type AppModel struct {
	cfg             *config.Config
	taskSvc         service.TaskService
	boards          []models.Board
	allNotes        []notes.Note
	currentView     ViewType
	dayView         agendaview.DayModel
	weekView        agendaview.WeekModel
	monthView       agendaview.MonthModel
	pickerView      kanbanview.PickerModel
	boardView       kanbanview.BoardModel
	boardLoaded     bool // true when boardView has a valid board
	taskManagerView taskview.TaskManagerModel
	todoFilePath    string
	doneFilePath    string
	showHelp        bool
	width           int
	height          int
	ready           bool
}

// NewAppModel creates the root application model
func NewAppModel(cfg *config.Config, taskSvc service.TaskService, todoFilePath, doneFilePath string) AppModel {
	boards, _ := fs.ScanAllBoards(cfg.Dirs, cfg.RecursiveDirs)
	allNotes := notes.ScanNotes(cfg.Dirs, cfg.RecursiveDirs)

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
	}

	// Compute available dirs for picker (all dirs + recursive dirs)
	availableDirs := append([]string{}, cfg.Dirs...)
	availableDirs = append(availableDirs, cfg.RecursiveDirs...)

	defaultDir := ""
	if len(availableDirs) > 0 {
		defaultDir = availableDirs[0]
	}

	return AppModel{
		cfg:             cfg,
		taskSvc:         taskSvc,
		boards:          boards,
		allNotes:        allNotes,
		currentView:     view,
		dayView:         agendaview.NewDayModel(taskSvc, boards, allNotes),
		weekView:        agendaview.NewWeekModel(taskSvc, boards, allNotes),
		monthView:       agendaview.NewMonthModel(taskSvc, boards, allNotes),
		pickerView:      kanbanview.NewPickerModel(boards, defaultDir, availableDirs),
		taskManagerView: taskview.NewTaskManagerModel(taskSvc, todoFilePath, doneFilePath),
		todoFilePath:    todoFilePath,
		doneFilePath:    doneFilePath,
	}
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
		m.dayView.SetSize(msg.Width, contentHeight)
		m.weekView.SetSize(msg.Width, contentHeight)
		m.monthView.SetSize(msg.Width, contentHeight)
		m.pickerView.SetSize(msg.Width, contentHeight)
		if m.boardLoaded {
			m.boardView.SetSize(msg.Width, contentHeight)
		}
		m.taskManagerView.SetSize(msg.Width, contentHeight)
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

	case SwitchViewMsg:
		m.currentView = msg.View
		// Refresh data when switching to certain views
		switch msg.View {
		case ViewAgendaDay:
			m.allNotes = notes.ScanNotes(m.cfg.Dirs, m.cfg.RecursiveDirs)
			m.dayView.SetData(m.taskSvc, m.boards, m.allNotes)
		case ViewAgendaWeek:
			m.allNotes = notes.ScanNotes(m.cfg.Dirs, m.cfg.RecursiveDirs)
			m.weekView.SetData(m.taskSvc, m.boards, m.allNotes)
		case ViewAgendaMonth:
			m.allNotes = notes.ScanNotes(m.cfg.Dirs, m.cfg.RecursiveDirs)
			m.monthView.SetData(m.taskSvc, m.boards, m.allNotes)
		case ViewKanbanPicker:
			// Rescan boards in case they changed
			boards, _ := fs.ScanAllBoards(m.cfg.Dirs, m.cfg.RecursiveDirs)
			m.boards = boards
			m.pickerView.SetBoards(boards)
		}
		return m, nil

	case FocusTaskMsg:
		m.currentView = ViewTaskManager
		m.taskManagerView.SetData(m.taskSvc)
		m.taskManagerView.FocusTask(msg.TaskID)
		return m, nil

	case taskview.TaskUpdateMsg:
		// A task was updated in the task manager — persist it
		if err := m.taskSvc.Update(msg.Task); err != nil {
			logs.Logger.Printf("Error updating task: %v", err)
		}
		m.taskManagerView.SetData(m.taskSvc)
		return m, nil

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
		// Refresh all data sources
		boards, _ := fs.ScanAllBoards(m.cfg.Dirs, m.cfg.RecursiveDirs)
		m.boards = boards
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

		// For board/picker views and task manager in modal state,
		// let the child view handle all keys.
		if m.currentView == ViewKanbanBoard || m.currentView == ViewKanbanPicker {
			// Don't intercept keys — let child view handle everything
		} else if m.currentView == ViewTaskManager && m.taskManagerView.IsInModalState() {
			// Task manager is in a modal state (editor, picker, search, etc.)
			// Let it handle all keys
		} else {
			// Global navigation keys for agenda/task views
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "1":
				m.currentView = ViewAgendaDay
				m.allNotes = notes.ScanNotes(m.cfg.Dirs, m.cfg.RecursiveDirs)
				m.dayView.SetData(m.taskSvc, m.boards, m.allNotes)
				return m, nil
			case "2":
				m.currentView = ViewAgendaWeek
				m.allNotes = notes.ScanNotes(m.cfg.Dirs, m.cfg.RecursiveDirs)
				m.weekView.SetData(m.taskSvc, m.boards, m.allNotes)
				return m, nil
			case "3":
				m.currentView = ViewAgendaMonth
				m.allNotes = notes.ScanNotes(m.cfg.Dirs, m.cfg.RecursiveDirs)
				m.monthView.SetData(m.taskSvc, m.boards, m.allNotes)
				return m, nil
			case "b":
				m.currentView = ViewKanbanPicker
				boards, _ := fs.ScanAllBoards(m.cfg.Dirs, m.cfg.RecursiveDirs)
				m.boards = boards
				m.pickerView.SetBoards(boards)
				return m, nil
			case "t":
				if m.currentView != ViewTaskManager {
					m.currentView = ViewTaskManager
					m.taskManagerView.SetData(m.taskSvc)
				}
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
	}

	return m, nil
}

func (m AppModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.showHelp {
		return m.renderHelpOverlay()
	}

	var content string

	switch m.currentView {
	case ViewAgendaDay:
		content = m.dayView.View()
	case ViewAgendaWeek:
		content = m.weekView.View()
	case ViewAgendaMonth:
		content = m.monthView.View()
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
	}

	// Status bar — show different hints based on view
	var statusText string
	switch m.currentView {
	case ViewKanbanBoard:
		statusText = "Board view | q/b: back to picker"
	case ViewKanbanPicker:
		statusText = "Board picker | esc: back"
	case ViewTaskManager:
		statusText = "Task manager | 1:day 2:week 3:month | b:boards | ?:help | q:quit"
	default:
		statusText = "1:day 2:week 3:month | b:boards t:tasks | ?:help | q:quit"
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
	content += line("1", "Day agenda view") + "\n"
	content += line("2", "Week agenda view") + "\n"
	content += line("3", "Month agenda view") + "\n"
	content += line("b", "Board picker") + "\n"
	content += line("t", "Task manager") + "\n"
	content += line("?", "Show this help") + "\n"
	content += line("q", "Quit") + "\n"
	content += line("ctrl+c", "Force quit") + "\n\n"

	content += sectionStyle.Render("Agenda Views (Day/Week)") + "\n"
	content += line("h / l", "Previous / next period") + "\n"
	content += line("j / k", "Navigate items") + "\n"
	content += line("T", "Jump to today") + "\n"
	content += line("enter", "Open selected item") + "\n\n"

	content += sectionStyle.Render("Month View") + "\n"
	content += line("h / l", "Previous / next day") + "\n"
	content += line("j / k", "Previous / next week") + "\n"
	content += line("H / L", "Previous / next month") + "\n"
	content += line("T", "Jump to today") + "\n"
	content += line("enter", "Enter detail panel") + "\n"
	content += line("esc", "Back to calendar") + "\n\n"

	content += sectionStyle.Render("Task Manager") + "\n"
	content += line("j / k", "Navigate tasks") + "\n"
	content += line("enter", "Edit task") + "\n"
	content += line("space", "Toggle done") + "\n"
	content += line("n", "New task") + "\n"
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
