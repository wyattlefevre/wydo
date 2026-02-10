package tasks

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/logs"
	"wydo/internal/tasks/data"
	"wydo/internal/tasks/service"
	"wydo/internal/tui/shared"
)

var (
	groupHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")).MarginTop(1)
	cursorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
)

// FileViewMode determines which file(s) to display tasks from
type FileViewMode int

const (
	FileViewAll FileViewMode = iota
	FileViewTodoOnly
	FileViewDoneOnly
)

// TaskUpdateMsg is sent when a task is updated
type TaskUpdateMsg struct {
	Task data.Task
}

// TaskEditorOpenMsg is sent to open the task editor
type TaskEditorOpenMsg struct {
	Task *data.Task
}

// ToggleFileViewMsg is sent to cycle file view mode
type ToggleFileViewMsg struct{}

// StartArchiveMsg is sent to start the archive flow
type StartArchiveMsg struct{}

// ArchiveRequestMsg is sent to request archiving tasks
type ArchiveRequestMsg struct {
	Count int
}

// TaskDeleteMsg is sent when a task should be deleted
type TaskDeleteMsg struct {
	TaskID string
}

// ArchiveCompleteMsg is sent when archive operation completes
type ArchiveCompleteMsg struct {
	Count int
}

// MoveTaskToBoardMsg is sent when a task should be moved to a kanban board
type MoveTaskToBoardMsg struct {
	Task      data.Task
	BoardPath string
}

// TaskManagerModel manages the task list view with filtering, sorting, and grouping
type TaskManagerModel struct {
	// Data
	taskSvc        service.TaskService
	workspaceRoots []string
	boards         []kanbanmodels.Board
	tasks          []data.Task
	displayTasks   []data.Task
	taskGroups     []TaskGroup

	// Navigation
	cursor       int
	scrollOffset int

	// State
	inputContext InputModeContext
	filterState  FilterState
	sortState    SortState
	groupState   GroupState

	// Sub-components
	infoBar           InfoBarModel
	fuzzyPicker       *FuzzyPickerModel
	textInput         *TextInputModel
	taskEditor        *TaskEditorModel
	confirmationModal *ConfirmationModal

	// File view mode
	fileViewMode FileViewMode

	// Pending delete (for confirmation modal)
	pendingDeleteTaskID string

	// Inline search
	searchActive     bool
	searchFilterMode bool // true when actively typing in search filter
	searchInput      textinput.Model

	// Cached data for pickers
	allProjects []string
	allContexts []string
	allFiles    []string

	// Picker context (what are we picking for)
	pickerContext string // "filter-project", "filter-context", "filter-file", etc.

	// Dimensions
	width  int
	height int
}

// NewTaskManagerModel creates a new task manager model
func NewTaskManagerModel(taskSvc service.TaskService, workspaceRoots []string, boards []kanbanmodels.Board) TaskManagerModel {
	m := TaskManagerModel{
		taskSvc:        taskSvc,
		workspaceRoots: workspaceRoots,
		boards:         boards,
		inputContext:   NewInputModeContext(),
		filterState:    NewFilterState(),
		sortState:      NewSortState(),
		groupState:     GroupState{Field: GroupByFile, Ascending: false},
		infoBar:        NewInfoBar(),
		fileViewMode:   FileViewAll,
	}
	m.loadTasks()
	return m
}

// SetSize updates the dimensions
func (m *TaskManagerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.infoBar.Width = width
	m.ensureCursorVisible()
}

// SetData refreshes data from the task service
func (m *TaskManagerModel) SetData(taskSvc service.TaskService) {
	m.taskSvc = taskSvc
	m.loadTasks()
}

// SetBoards updates the available boards
func (m *TaskManagerModel) SetBoards(boards []kanbanmodels.Board) {
	m.boards = boards
}

// FocusTask moves the cursor to a specific task by ID
func (m *TaskManagerModel) FocusTask(taskID string) {
	for i, task := range m.displayTasks {
		if task.ID == taskID {
			m.cursor = i
			m.ensureCursorVisible()
			return
		}
	}
}

func (m *TaskManagerModel) loadTasks() {
	tasks, err := m.taskSvc.List()
	if err != nil {
		logs.Logger.Printf("Error loading tasks: %v", err)
		return
	}
	m.tasks = tasks
	m.allProjects = ExtractUniqueProjects(tasks)
	m.allContexts = ExtractUniqueContexts(tasks)
	m.allFiles = ExtractUniqueFiles(tasks, m.workspaceRoots)
	m.refreshDisplayTasks()
}

// Update handles messages for the task manager
func (m TaskManagerModel) Update(msg tea.Msg) (TaskManagerModel, tea.Cmd) {
	// Handle sub-component results first
	switch msg := msg.(type) {
	case FuzzyPickerResultMsg:
		// If task editor has its own fuzzy picker, forward to it
		if m.taskEditor != nil && m.taskEditor.fuzzyPicker != nil {
			_, cmd := m.taskEditor.Update(msg)
			return m, cmd
		}
		return m.handlePickerResult(msg)
	case TextInputResultMsg:
		return m.handleTextInputResult(msg)
	case TaskEditorResultMsg:
		return m.handleEditorResult(msg)
	case ToggleFileViewMsg:
		m.cycleFileViewMode()
		m.refreshDisplayTasks()
		return m, nil
	case StartArchiveMsg:
		return m.handleStartArchive()
	case ConfirmationResultMsg:
		return m.handleConfirmationResult(msg)
	case ArchiveCompleteMsg:
		m.confirmationModal = nil
		m.loadTasks()
		return m, tea.Printf("Archived %d tasks to done.txt", msg.Count)
	}

	// Handle inline search mode (before other sub-components)
	if m.searchActive {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			return m.handleSearchMode(msg)
		default:
			// Forward non-key messages (like blink) to searchInput
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, cmd
		}
	}

	// Handle sub-component updates
	if m.confirmationModal != nil {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			cmd := m.confirmationModal.Update(keyMsg)
			return m, cmd
		}
	}
	if m.fuzzyPicker != nil {
		var cmd tea.Cmd
		_, cmd = m.fuzzyPicker.Update(msg)
		return m, cmd
	}
	if m.textInput != nil {
		var cmd tea.Cmd
		_, cmd = m.textInput.Update(msg)
		return m, cmd
	}
	if m.taskEditor != nil {
		var cmd tea.Cmd
		_, cmd = m.taskEditor.Update(msg)
		return m, cmd
	}

	// Handle key messages based on mode
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			return m.handleEscape()
		}

		switch m.inputContext.Mode {
		case ModeNormal:
			return m.handleNormalMode(msg)
		case ModeFilterSelect:
			return m.handleFilterSelect(msg)
		case ModeSortSelect:
			return m.handleSortSelect(msg)
		case ModeGroupSelect:
			return m.handleGroupSelect(msg)
		case ModeSortDirection:
			return m.handleSortDirection(msg)
		case ModeGroupDirection:
			return m.handleGroupDirection(msg)
		}
	}

	return m, nil
}

// View renders the task manager
func (m TaskManagerModel) View() string {
	var b strings.Builder

	// Update info bar with current state
	m.infoBar.SetContext(&m.inputContext, &m.filterState, &m.sortState, &m.groupState, m.filterState.SearchQuery, m.fileViewMode)

	// Info bar (always visible)
	b.WriteString(m.infoBar.View())
	b.WriteString("\n\n")

	// Sub-component overlays (except search - which is inline)
	if m.confirmationModal != nil {
		modal := m.confirmationModal.View()
		// Center the modal on screen
		return lipgloss.Place(
			m.infoBar.Width, 30,
			lipgloss.Center, lipgloss.Center,
			modal,
			lipgloss.WithWhitespaceChars(" "),
		)
	}
	if m.fuzzyPicker != nil {
		b.WriteString(m.fuzzyPicker.View())
		return b.String()
	}
	if m.textInput != nil {
		b.WriteString(m.textInput.View())
		return b.String()
	}
	if m.taskEditor != nil {
		b.WriteString(m.taskEditor.View())
		return b.String()
	}

	// Inline search line (when active)
	if m.searchActive {
		searchLine := searchStyle.Render("/") + m.searchInput.View()
		b.WriteString(searchLine)
		b.WriteString("\n")
	}

	// Task list
	if m.groupState.IsActive() && len(m.taskGroups) > 0 {
		b.WriteString(m.renderGroupedTasks())
	} else {
		b.WriteString(m.renderFlatTasks())
	}

	// Compute hints for bottom
	var hintsText string
	if m.searchActive {
		if m.searchFilterMode {
			hintsText = "[enter] done  [esc] clear"
		} else {
			if m.filterState.SearchQuery != "" {
				hintsText = "[/] filter  [j/k] navigate  [enter] done  [esc] clear"
			} else {
				hintsText = "[/] filter  [j/k] navigate  [enter] done  [esc] cancel"
			}
		}
		hintsText = hintStyle.Render(hintsText)
	} else {
		hintsText = m.infoBar.RenderHints()
	}
	hints := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, hintsText)

	return shared.CenterWithBottomHints(b.String(), hints, m.height)
}

func (m *TaskManagerModel) renderFlatTasks() string {
	var b strings.Builder

	if len(m.displayTasks) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("No tasks found."))
		return b.String()
	}

	visible := m.visibleTaskRows()
	end := m.scrollOffset + visible
	if end > len(m.displayTasks) {
		end = len(m.displayTasks)
	}

	for i := m.scrollOffset; i < end; i++ {
		task := m.displayTasks[i]
		prefix := "  "
		if i == m.cursor {
			prefix = cursorStyle.Render("> ")
		}
		b.WriteString(prefix + shared.StyledTaskLine(task) + "\n")
	}

	return b.String()
}

func (m *TaskManagerModel) renderGroupedTasks() string {
	var b strings.Builder

	visible := m.visibleTaskRows()
	linesRendered := 0
	taskIndex := 0

	for _, group := range m.taskGroups {
		groupStart := taskIndex
		groupEnd := taskIndex + len(group.Tasks)

		// Skip groups entirely before the scroll window
		if groupEnd <= m.scrollOffset {
			taskIndex = groupEnd
			continue
		}

		// Stop if we've filled the visible area
		if linesRendered >= visible {
			break
		}

		// Emit group header if any task in this group is visible
		if taskIndex >= m.scrollOffset || (groupStart < m.scrollOffset && groupEnd > m.scrollOffset) {
			b.WriteString(groupHeaderStyle.Render("-- " + group.Label + " --"))
			b.WriteString("\n")
			linesRendered++
		}

		for _, task := range group.Tasks {
			if linesRendered >= visible {
				break
			}
			if taskIndex >= m.scrollOffset {
				prefix := "  "
				if taskIndex == m.cursor {
					prefix = cursorStyle.Render("> ")
				}
				b.WriteString(prefix + shared.StyledTaskLine(task) + "\n")
				linesRendered++
			}
			taskIndex++
		}
	}

	return b.String()
}

// Input handlers

func (m TaskManagerModel) handleNormalMode(msg tea.KeyMsg) (TaskManagerModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case "enter":
		return m.openTaskEditor()
	case "f":
		m.inputContext.TransitionTo(ModeFilterSelect)
		m.inputContext.Category = "filter"
	case "s":
		m.inputContext.TransitionTo(ModeSortSelect)
		m.inputContext.Category = "sort"
	case "g":
		m.inputContext.TransitionTo(ModeGroupSelect)
		m.inputContext.Category = "group"
	case "F":
		return m.startFileFilter()
	case "/":
		return m.startSearch()
	case " ":
		return m.toggleTaskDone()
	case "n":
		return m.startNewTask()
	case "D":
		return m.handleStartDelete()
	case "m":
		return m.startMoveToBoard()
	}
	return m, nil
}

func (m TaskManagerModel) handleFilterSelect(msg tea.KeyMsg) (TaskManagerModel, tea.Cmd) {
	switch msg.String() {
	case "/":
		return m.startSearch()
	case "d":
		return m.startDateFilter()
	case "p":
		return m.startProjectFilter()
	case "P":
		m.cyclePriorityFilter()
		m.inputContext.Reset()
	case "t", "c":
		return m.startContextFilter()
	case "s":
		m.filterState.CycleStatusFilter()
		m.refreshDisplayTasks()
		m.inputContext.Reset()
	case "f":
		return m.startFileFilter()
	}
	return m, nil
}

func (m TaskManagerModel) handleSortSelect(msg tea.KeyMsg) (TaskManagerModel, tea.Cmd) {
	switch msg.String() {
	case "d":
		m.inputContext.Field = "date"
		m.inputContext.TransitionTo(ModeSortDirection)
	case "p":
		m.inputContext.Field = "project"
		m.inputContext.TransitionTo(ModeSortDirection)
	case "P":
		m.inputContext.Field = "priority"
		m.inputContext.TransitionTo(ModeSortDirection)
	case "t", "c":
		m.inputContext.Field = "context"
		m.inputContext.TransitionTo(ModeSortDirection)
	}
	return m, nil
}

func (m TaskManagerModel) handleGroupSelect(msg tea.KeyMsg) (TaskManagerModel, tea.Cmd) {
	switch msg.String() {
	case "d":
		m.inputContext.Field = "date"
		m.inputContext.TransitionTo(ModeGroupDirection)
	case "p":
		m.inputContext.Field = "project"
		m.inputContext.TransitionTo(ModeGroupDirection)
	case "P":
		m.inputContext.Field = "priority"
		m.inputContext.TransitionTo(ModeGroupDirection)
	case "t", "c":
		m.inputContext.Field = "context"
		m.inputContext.TransitionTo(ModeGroupDirection)
	case "f":
		m.inputContext.Field = "file"
		m.inputContext.TransitionTo(ModeGroupDirection)
	}
	return m, nil
}

func (m TaskManagerModel) handleSortDirection(msg tea.KeyMsg) (TaskManagerModel, tea.Cmd) {
	switch msg.String() {
	case "a":
		m.applySortField(true)
	case "d":
		m.applySortField(false)
	}
	return m, nil
}

func (m TaskManagerModel) handleGroupDirection(msg tea.KeyMsg) (TaskManagerModel, tea.Cmd) {
	switch msg.String() {
	case "a":
		m.applyGroupField(true)
	case "d":
		m.applyGroupField(false)
	}
	return m, nil
}

func (m TaskManagerModel) handleSearchMode(msg tea.KeyMsg) (TaskManagerModel, tea.Cmd) {
	// Handle filter typing mode
	if m.searchFilterMode {
		switch msg.String() {
		case "enter":
			// Exit filter mode, keep query, stay in search mode
			m.searchFilterMode = false
			m.searchInput.Blur()
			return m, nil

		case "esc":
			// Clear query, exit filter mode, stay in search mode
			m.searchInput.SetValue("")
			m.filterState.SearchQuery = ""
			m.searchFilterMode = false
			m.searchInput.Blur()
			m.refreshDisplayTasks()
			return m, nil

		default:
			// Forward all keys to textinput (including j/k)
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			// Live filter on every keystroke
			m.filterState.SearchQuery = m.searchInput.Value()
			m.refreshDisplayTasks()
			return m, cmd
		}
	}

	// Navigation mode (not typing in filter)
	switch msg.String() {
	case "/":
		// Re-enter filter typing mode
		m.searchFilterMode = true
		return m, m.searchInput.Focus()

	case "enter":
		// Confirm search: exit search mode entirely
		m.searchActive = false
		m.searchFilterMode = false
		m.inputContext.Reset()
		return m, nil

	case "esc":
		// If query exists, clear it; otherwise exit search mode
		if m.filterState.SearchQuery != "" {
			m.searchInput.SetValue("")
			m.filterState.SearchQuery = ""
			m.refreshDisplayTasks()
			return m, nil
		}
		// Exit search mode
		m.searchActive = false
		m.searchFilterMode = false
		m.inputContext.Reset()
		return m, nil

	case "up", "k":
		m.moveCursor(-1)
		return m, nil

	case "down", "j":
		m.moveCursor(1)
		return m, nil

	case " ":
		// Allow toggling tasks while in search navigation mode
		return m.toggleTaskDone()
	}

	return m, nil
}

func (m TaskManagerModel) handleEscape() (TaskManagerModel, tea.Cmd) {
	// Close any open sub-component
	if m.confirmationModal != nil {
		m.confirmationModal = nil
		m.inputContext.Reset()
		return m, nil
	}
	if m.fuzzyPicker != nil {
		m.fuzzyPicker = nil
		m.inputContext.Reset()
		return m, nil
	}
	if m.textInput != nil {
		m.textInput = nil
		m.inputContext.Reset()
		return m, nil
	}
	if m.taskEditor != nil {
		m.taskEditor = nil
		m.inputContext.Reset()
		return m, nil
	}

	// Go back or reset
	if m.inputContext.Mode != ModeNormal {
		m.inputContext.Back()
		if m.inputContext.Mode == ModeNormal {
			m.inputContext.Reset()
		}
		return m, nil
	}

	// In normal mode, clear filters and file view mode, restore default grouping
	m.filterState.Reset()
	m.sortState.Reset()
	m.groupState = GroupState{Field: GroupByFile, Ascending: true}
	m.fileViewMode = FileViewAll
	m.refreshDisplayTasks()
	return m, nil
}

// Actions

func (m TaskManagerModel) startSearch() (TaskManagerModel, tea.Cmd) {
	// Use inline search mode with lightweight textinput
	m.searchInput = textinput.New()
	m.searchInput.Placeholder = "type to filter..."
	m.searchInput.CharLimit = 256
	m.searchInput.Width = 40
	m.searchInput.SetValue(m.filterState.SearchQuery)
	m.searchActive = true
	m.searchFilterMode = true // Start in filter typing mode
	m.inputContext.TransitionTo(ModeSearch)
	m.ensureCursorVisible()
	return m, m.searchInput.Focus()
}

func (m TaskManagerModel) startNewTask() (TaskManagerModel, tea.Cmd) {
	// Prompt for task name using text input
	m.textInput = NewTextInput("New Task Name", "Enter task description...", nil)
	m.inputContext.TransitionTo(ModeCreateTask)
	return m, m.textInput.Focus()
}

func (m TaskManagerModel) createNewTaskAndOpenEditor(taskName string) (TaskManagerModel, tea.Cmd) {
	if strings.TrimSpace(taskName) == "" {
		m.inputContext.Reset()
		return m, nil
	}

	// Generate a unique ID for the new task
	timestamp := time.Now().Format("20060102150405")
	randomPart := fmt.Sprintf("%d", time.Now().UnixNano()%10000)
	newID := data.HashTaskLine(timestamp + randomPart)

	// Create new task (File will be set by Add when persisted)
	newTask := &data.Task{
		ID:       newID,
		Name:     taskName,
		Projects: []string{},
		Contexts: []string{},
		Done:     false,
		Tags:     make(map[string]string),
		Priority: data.PriorityNone,
	}

	// Open editor with the new task
	m.taskEditor = NewTaskEditor(newTask, m.allProjects, m.allContexts)
	m.taskEditor.Width = m.width
	m.taskEditor.Height = m.height
	m.inputContext.TransitionTo(ModeTaskEditor)
	return m, nil
}

func (m TaskManagerModel) startDateFilter() (TaskManagerModel, tea.Cmd) {
	m.textInput = NewDateInput("Due date filter")
	m.inputContext.TransitionTo(ModeDateInput)
	return m, m.textInput.Focus()
}

func (m TaskManagerModel) startProjectFilter() (TaskManagerModel, tea.Cmd) {
	m.fuzzyPicker = NewFuzzyPicker(m.allProjects, "Filter by Project", true, false)
	m.fuzzyPicker.PreSelect(m.filterState.ProjectFilter)
	m.pickerContext = "filter-project"
	m.inputContext.TransitionTo(ModeFuzzyPicker)
	return m, nil
}

func (m TaskManagerModel) startContextFilter() (TaskManagerModel, tea.Cmd) {
	m.fuzzyPicker = NewFuzzyPicker(m.allContexts, "Filter by Context", true, false)
	m.fuzzyPicker.PreSelect(m.filterState.ContextFilter)
	m.pickerContext = "filter-context"
	m.inputContext.TransitionTo(ModeFuzzyPicker)
	return m, nil
}

func (m TaskManagerModel) startFileFilter() (TaskManagerModel, tea.Cmd) {
	m.fuzzyPicker = NewFuzzyPicker(m.allFiles, "Filter by File", true, false)
	m.fuzzyPicker.PreSelect(m.filterState.FileFilter)
	m.pickerContext = "filter-file"
	m.inputContext.TransitionTo(ModeFuzzyPicker)
	return m, nil
}

func (m *TaskManagerModel) cyclePriorityFilter() {
	priorities := []data.Priority{
		data.PriorityA, data.PriorityB, data.PriorityC,
		data.PriorityD, data.PriorityE, data.PriorityF,
	}

	if len(m.filterState.PriorityFilter) == 0 {
		m.filterState.PriorityFilter = []data.Priority{data.PriorityA}
	} else {
		current := m.filterState.PriorityFilter[0]
		nextIdx := -1
		for i, p := range priorities {
			if p == current {
				nextIdx = i + 1
				break
			}
		}
		if nextIdx >= len(priorities) {
			m.filterState.PriorityFilter = nil
		} else {
			m.filterState.PriorityFilter = []data.Priority{priorities[nextIdx]}
		}
	}
	m.refreshDisplayTasks()
}

func (m *TaskManagerModel) applySortField(ascending bool) {
	var field SortField
	switch m.inputContext.Field {
	case "date":
		field = SortByDueDate
	case "project":
		field = SortByProject
	case "priority":
		field = SortByPriority
	case "context":
		field = SortByContext
	}

	m.sortState.Field = field
	m.sortState.Ascending = ascending
	m.refreshDisplayTasks()
	m.inputContext.Reset()
}

func (m *TaskManagerModel) applyGroupField(ascending bool) {
	var field GroupField
	switch m.inputContext.Field {
	case "date":
		field = GroupByDueDate
	case "project":
		field = GroupByProject
	case "priority":
		field = GroupByPriority
	case "context":
		field = GroupByContext
	case "file":
		field = GroupByFile
	}

	m.groupState.Field = field
	m.groupState.Ascending = ascending
	m.refreshDisplayTasks()
	m.inputContext.Reset()
}

func (m TaskManagerModel) openTaskEditor() (TaskManagerModel, tea.Cmd) {
	task := m.selectedTask()
	if task == nil {
		return m, nil
	}

	m.taskEditor = NewTaskEditor(task, m.allProjects, m.allContexts)
	m.taskEditor.Width = m.width
	m.taskEditor.Height = m.height
	m.inputContext.TransitionTo(ModeTaskEditor)
	return m, nil
}

func (m TaskManagerModel) toggleTaskDone() (TaskManagerModel, tea.Cmd) {
	logs.Logger.Println("space pressed")
	task := m.selectedTask()
	if task == nil {
		logs.Logger.Println("no selected task")
		return m, nil
	}

	task.Done = !task.Done
	return m, func() tea.Msg {
		return TaskUpdateMsg{Task: *task}
	}
}

// Result handlers

func (m TaskManagerModel) handlePickerResult(msg FuzzyPickerResultMsg) (TaskManagerModel, tea.Cmd) {
	m.fuzzyPicker = nil

	if msg.Cancelled {
		m.inputContext.Reset()
		return m, nil
	}

	switch m.pickerContext {
	case "filter-project":
		m.filterState.ProjectFilter = msg.Selected
	case "filter-context":
		m.filterState.ContextFilter = msg.Selected
	case "filter-file":
		m.filterState.FileFilter = msg.Selected
	case "move-to-board":
		if len(msg.Selected) > 0 {
			boardName := msg.Selected[0]
			for _, b := range m.boards {
				if b.Name == boardName {
					task := m.selectedTask()
					if task != nil {
						t := *task
						m.refreshDisplayTasks()
						m.inputContext.Reset()
						m.pickerContext = ""
						return m, func() tea.Msg {
							return MoveTaskToBoardMsg{Task: t, BoardPath: b.Path}
						}
					}
					break
				}
			}
		}
	}

	m.refreshDisplayTasks()
	m.inputContext.Reset()
	m.pickerContext = ""
	return m, nil
}

func (m TaskManagerModel) handleTextInputResult(msg TextInputResultMsg) (TaskManagerModel, tea.Cmd) {
	m.textInput = nil

	if msg.Cancelled {
		m.inputContext.Reset()
		return m, nil
	}

	if m.inputContext.Mode == ModeSearch {
		m.filterState.SearchQuery = msg.Value
		m.refreshDisplayTasks()
	} else if m.inputContext.Mode == ModeCreateTask {
		// Create new task and open editor
		return m.createNewTaskAndOpenEditor(msg.Value)
	}

	m.inputContext.Reset()
	return m, nil
}

func (m TaskManagerModel) handleEditorResult(msg TaskEditorResultMsg) (TaskManagerModel, tea.Cmd) {
	m.taskEditor = nil
	m.inputContext.Reset()

	if msg.Cancelled {
		return m, nil
	}

	// Send update message
	return m, func() tea.Msg {
		return TaskUpdateMsg{Task: msg.Task}
	}
}

// Helpers

func (m *TaskManagerModel) refreshDisplayTasks() {
	// Apply filters
	filtered := ApplyFilters(m.tasks, m.filterState)

	// Apply file view filter
	filtered = m.applyFileViewFilter(filtered)

	// Apply sort
	sorted := ApplySort(filtered, m.sortState)

	// Apply grouping
	if m.groupState.IsActive() {
		m.taskGroups = ApplyGroups(sorted, m.groupState, m.workspaceRoots)
		// Flatten for cursor navigation
		m.displayTasks = nil
		for _, g := range m.taskGroups {
			m.displayTasks = append(m.displayTasks, g.Tasks...)
		}
	} else {
		m.displayTasks = sorted
		m.taskGroups = nil
	}

	// Clamp cursor
	if m.cursor >= len(m.displayTasks) {
		m.cursor = len(m.displayTasks) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureCursorVisible()
}

func (m *TaskManagerModel) moveCursor(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.displayTasks) {
		m.cursor = len(m.displayTasks) - 1
	}
	m.ensureCursorVisible()
}

func (m *TaskManagerModel) selectedTask() *data.Task {
	if m.cursor >= 0 && m.cursor < len(m.displayTasks) {
		return &m.displayTasks[m.cursor]
	}
	return nil
}

// visibleTaskRows returns the number of task lines that fit in the viewport.
// The info bar uses 4 lines (3 content + border), gap uses 1 line, hints use 1 line.
func (m *TaskManagerModel) visibleTaskRows() int {
	used := 6 // info bar (4) + gap (1) + hints (1)
	if m.searchActive {
		used++ // search input line
	}
	visible := m.height - used
	if visible < 1 {
		visible = 1
	}
	return visible
}

// ensureCursorVisible adjusts scrollOffset so the cursor is within the visible window.
func (m *TaskManagerModel) ensureCursorVisible() {
	visible := m.visibleTaskRows()
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visible {
		m.scrollOffset = m.cursor - visible + 1
	}
}

// handleStartArchive initiates the archive flow
func (m TaskManagerModel) handleStartArchive() (TaskManagerModel, tea.Cmd) {
	// Count completed tasks not yet in done.txt
	count := 0
	for _, task := range m.tasks {
		if task.Done && !strings.HasSuffix(task.File, "done.txt") {
			count++
		}
	}

	if count == 0 {
		return m, tea.Printf("No completed tasks to archive")
	}

	// Show confirmation modal
	m.confirmationModal = NewConfirmationModal(
		fmt.Sprintf("Archive %d completed task(s)?", count),
		"This will move completed tasks from todo.txt to done.txt",
		50,
	)
	m.inputContext.TransitionTo(ModeConfirmation)
	return m, nil
}

// handleStartDelete initiates the delete flow for the selected task
func (m TaskManagerModel) handleStartDelete() (TaskManagerModel, tea.Cmd) {
	task := m.selectedTask()
	if task == nil {
		return m, nil
	}

	m.pendingDeleteTaskID = task.ID
	m.confirmationModal = NewConfirmationModal(
		"Delete task?",
		task.Name,
		50,
	)
	m.inputContext.TransitionTo(ModeConfirmation)
	return m, nil
}

// startMoveToBoard initiates the move-to-board flow
func (m TaskManagerModel) startMoveToBoard() (TaskManagerModel, tea.Cmd) {
	task := m.selectedTask()
	if task == nil {
		return m, nil
	}
	if task.Done {
		return m, tea.Printf("Cannot move completed tasks to a board")
	}
	if len(m.boards) == 0 {
		return m, tea.Printf("No boards available")
	}

	// Single board — skip picker
	if len(m.boards) == 1 {
		t := *task
		boardPath := m.boards[0].Path
		return m, func() tea.Msg {
			return MoveTaskToBoardMsg{Task: t, BoardPath: boardPath}
		}
	}

	// Multiple boards — open picker
	boardNames := make([]string, len(m.boards))
	for i, b := range m.boards {
		boardNames[i] = b.Name
	}
	m.fuzzyPicker = NewFuzzyPicker(boardNames, "Move to Board", false, false)
	m.pickerContext = "move-to-board"
	m.inputContext.TransitionTo(ModeBoardPicker)
	return m, nil
}

// handleConfirmationResult processes the confirmation modal result
func (m TaskManagerModel) handleConfirmationResult(msg ConfirmationResultMsg) (TaskManagerModel, tea.Cmd) {
	m.confirmationModal = nil
	m.inputContext.Reset()

	if !msg.Confirmed {
		m.pendingDeleteTaskID = ""
		return m, nil
	}

	// Delete flow
	if m.pendingDeleteTaskID != "" {
		taskID := m.pendingDeleteTaskID
		m.pendingDeleteTaskID = ""
		return m, func() tea.Msg {
			return TaskDeleteMsg{TaskID: taskID}
		}
	}

	// Archive flow
	count := 0
	for _, task := range m.tasks {
		if task.Done && !strings.HasSuffix(task.File, "done.txt") {
			count++
		}
	}
	return m, func() tea.Msg {
		return ArchiveRequestMsg{Count: count}
	}
}

// IsInModalState returns true if the task manager is in a mode that should
// block global key handling (editor, picker, input, search, or any non-normal mode)
func (m *TaskManagerModel) IsInModalState() bool {
	if m.taskEditor != nil || m.fuzzyPicker != nil || m.textInput != nil || m.searchActive || m.confirmationModal != nil {
		return true
	}
	return m.inputContext.Mode != ModeNormal
}

// cycleFileViewMode cycles through file view modes: All -> TodoOnly -> DoneOnly -> All
func (m *TaskManagerModel) cycleFileViewMode() {
	m.fileViewMode = (m.fileViewMode + 1) % 3
	m.cursor = 0       // Reset cursor position
	m.scrollOffset = 0 // Reset scroll position
}

// applyFileViewFilter filters tasks based on the current file view mode
func (m *TaskManagerModel) applyFileViewFilter(tasks []data.Task) []data.Task {
	if m.fileViewMode == FileViewAll {
		return tasks
	}

	var filtered []data.Task
	for _, task := range tasks {
		if m.fileViewMode == FileViewTodoOnly && !task.Done {
			filtered = append(filtered, task)
		} else if m.fileViewMode == FileViewDoneOnly && task.Done {
			filtered = append(filtered, task)
		}
	}
	return filtered
}
