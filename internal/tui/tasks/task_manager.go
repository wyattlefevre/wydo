package tasks

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/kanban/operations"
	"wydo/internal/logs"
	"wydo/internal/tasks/data"
	"wydo/internal/tasks/service"
	"wydo/internal/tui/messages"
	"wydo/internal/tui/shared"
	"wydo/internal/tui/theme"
	kanbanview "wydo/internal/tui/kanban"
)

var (
	groupHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(theme.Accent)
	sectionHeaderStyle = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(theme.Primary)
	cursorStyle        = theme.Cursor
)


// TaskUpdateMsg is sent when a task is updated
type TaskUpdateMsg struct {
	Task data.Task
}

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

// DeleteSelectionRequestMsg is sent when selected tasks should be permanently deleted
type DeleteSelectionRequestMsg struct {
	IDs []string
}

// DeleteCompleteMsg is sent when a bulk delete operation completes
type DeleteCompleteMsg struct {
	Count int
}

// MoveTaskToBoardMsg is sent when a task should be moved to a kanban board
type MoveTaskToBoardMsg struct {
	Task      data.Task
	BoardPath string
}

// taskNoteEditorFinishedMsg is sent when the editor closes after editing a tasknote from the task view
type taskNoteEditorFinishedMsg struct{ err error }

// ArchiveSelectionRequestMsg is sent when selected tasks should be archived
type ArchiveSelectionRequestMsg struct {
	IDs []string
}

// ConvertToComplexSelectionMsg is sent when selected simple tasks should be converted to TaskNotes.
type ConvertToComplexSelectionMsg struct {
	Tasks     []data.Task
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
	taskSections   []TaskSection

	// Navigation
	cursor       int
	scrollOffset int

	// State
	inputContext InputModeContext
	filterState  FilterState
	sortState    SortState
	groupState   GroupState
	activePreset ViewPreset

	// Sub-components
	infoBar       InfoBarModel
	fuzzyPicker   *FuzzyPickerModel
	textInput     *TextInputModel
	taskEditor    *TaskEditorModel
	confirmDialog *shared.Dialog
	datePicker    *shared.DatePickerModel // for direct date editing
	projectPicker     *kanbanview.ProjectPickerModel

	// Direct edit state
	directEditTaskID string

	// Archive mode selection
	archiveSelection map[string]bool

	// Delete mode selection
	deleteSelection map[string]bool

	// Convert mode selection
	convertSelection map[string]bool
	convertBoardPath string

	// Pending delete (for confirmation modal)
	pendingDeleteTaskID string

	// Inline search
	searchActive     bool
	searchFilterMode bool // true when actively typing in search filter
	searchInput      textinput.Model

	// Cached data for pickers
	allProjects      []string
	allProjectItems  []kanbanview.ProjectPickerItem
	allTags          []string
	allFiles         []string
	allWorkspaces    []string

	// Picker context (what are we picking for)
	pickerContext string // "filter-project", "filter-tag", "filter-file", etc.

	// Dimensions
	width  int
	height int
}

// NewTaskManagerModel creates a new task manager model
func NewTaskManagerModel(taskSvc service.TaskService, workspaceRoots []string, boards []kanbanmodels.Board, allProjectItems []kanbanview.ProjectPickerItem) TaskManagerModel {
	m := TaskManagerModel{
		taskSvc:         taskSvc,
		workspaceRoots:  workspaceRoots,
		boards:          boards,
		allProjectItems: allProjectItems,
		inputContext:    NewInputModeContext(),
		filterState:     NewFilterState(),
		sortState:       NewSortState(),
		groupState: GroupState{Field: GroupByNone, Ascending: false},
		infoBar:    NewInfoBar(),
	}
	m.loadTasks()
	return m
}

// SetSize updates the dimensions.
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

	// Append TaskNotes from all boards
	for _, board := range m.boards {
		for _, col := range board.Columns {
			for _, tn := range col.TaskNotes {
				if tn.Archived {
					continue
				}
				tasks = append(tasks, TaskNoteToTask(tn, board, col))
			}
		}
	}

	m.tasks = tasks
	m.allProjects = ExtractUniqueProjects(tasks)
	m.allTags = ExtractUniqueTags(tasks)
	m.allFiles = ExtractUniqueFiles(tasks, m.workspaceRoots)
	if len(m.workspaceRoots) > 1 {
		m.allWorkspaces = ExtractUniqueWorkspaces(tasks, m.workspaceRoots)
	}
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
		// If task editor has its own URL input, forward to it
		if m.taskEditor != nil && m.taskEditor.urlInput != nil {
			_, cmd := m.taskEditor.Update(msg)
			return m, cmd
		}
		return m.handleTextInputResult(msg)
	case TaskEditorResultMsg:
		return m.handleEditorResult(msg)
	case StartArchiveMsg:
		return m.handleStartArchive()
	case ArchiveCompleteMsg:
		m.confirmDialog = nil
		m.loadTasks()
		return m, messages.StatusCmd(fmt.Sprintf("Archived %d tasks", msg.Count), messages.LevelSuccess)
	case DeleteCompleteMsg:
		m.confirmDialog = nil
		m.loadTasks()
		return m, messages.StatusCmd(fmt.Sprintf("Deleted %d tasks", msg.Count), messages.LevelSuccess)
	case taskNoteEditorFinishedMsg:
		if msg.err != nil {
			return m, messages.StatusCmd(fmt.Sprintf("Editor error: %v", msg.err), messages.LevelError)
		}
		return m, tea.Batch(
			func() tea.Msg { return messages.DataRefreshMsg{} },
			messages.StatusCmd("Task note updated", messages.LevelSuccess),
		)
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

	// Handle project picker for direct editing
	if m.projectPicker != nil {
		var cmd tea.Cmd
		var isDone bool
		var cancelled bool
		*m.projectPicker, cmd, isDone, cancelled = m.projectPicker.Update(msg)
		_ = cancelled // projectpicker uses key-based save detection
		if isDone {
			task := m.findTaskByID(m.directEditTaskID)
			if task != nil {
				task.Projects = m.projectPicker.GetSelectedProjects()
				m.refreshDisplayTasks()
				m.directEditTaskID = ""
				m.projectPicker = nil
				return m, func() tea.Msg { return TaskUpdateMsg{Task: *task} }
			}
			m.projectPicker = nil
			m.directEditTaskID = ""
		}
		return m, cmd
	}

	// Handle date picker for direct editing
	if m.datePicker != nil {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			return m.handleDatePickerUpdate(keyMsg)
		}
		return m, nil
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
		case ModeArchive:
			return m.handleArchiveMode(msg)
		case ModeDelete:
			return m.handleDeleteMode(msg)
		case ModeConvertComplex:
			return m.handleConvertMode(msg)
		case ModeConfirmation:
			switch msg.String() {
			case "y", "enter":
				return m.handleConfirmationResult(true)
			case "n":
				return m.handleConfirmationResult(false)
			}
			return m, nil
		}
	}

	return m, nil
}

// View renders the task manager
func (m TaskManagerModel) View() string {
	// Sub-components that replace the full view (not overlaid)
	if m.projectPicker != nil {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.projectPicker.View())
	}
	if m.datePicker != nil {
		return m.datePicker.View()
	}
	if m.confirmDialog != nil {
		return shared.PlaceOverlay(m.renderTaskListFull(), m.confirmDialog.View(), m.width, m.height)
	}
	if m.fuzzyPicker != nil {
		if m.pickerContext == "edit-tag" {
			return m.renderTaskListFull() // background; overlay handled by OverlayView
		}
		return m.renderTaskList() + m.fuzzyPicker.View()
	}
	if m.textInput != nil {
		return m.renderTaskListFull()
	}

	// Task editor: sub-pickers replace the view; main editor box overlays the background (via app.go)
	if m.taskEditor != nil {
		if m.taskEditor.HasActiveSubComponent() {
			return m.taskEditor.View()
		}
		return m.renderTaskListFull()
	}

	return m.renderTaskListFull()
}

// OverlayView returns the box that should be composited over the full screen by the caller,
// or "" if no screen-level overlay is active.
func (m TaskManagerModel) OverlayView() string {
	// Direct tag edit — picker floats over the task list
	if m.fuzzyPicker != nil && m.pickerContext == "edit-tag" {
		return m.fuzzyPicker.View()
	}
	if m.textInput != nil {
		return m.textInput.View()
	}
	if m.taskEditor != nil && !m.taskEditor.HasActiveSubComponent() {
		// Tag picker nested inside editor takes priority over the editor box
		if pickerView := m.taskEditor.TagPickerView(); pickerView != "" {
			return pickerView
		}
		return m.taskEditor.View()
	}
	return ""
}

// renderTaskListHeader returns just the info bar line (used when a text input replaces the body).
func (m TaskManagerModel) renderTaskListHeader() string {
	m.infoBar.SetTag(&m.inputContext, &m.filterState, &m.sortState, &m.groupState, m.filterState.SearchQuery, len(m.workspaceRoots) > 1, m.activePreset)
	return m.infoBar.View() + "\n"
}

// renderTaskList returns the info bar without the mode bar (used when a picker appends below it).
func (m TaskManagerModel) renderTaskList() string {
	return m.renderTaskListHeader()
}

// renderTaskListFull builds the complete task list view: info bar + tasks + mode bar.
func (m TaskManagerModel) renderTaskListFull() string {
	var b strings.Builder

	m.infoBar.SetTag(&m.inputContext, &m.filterState, &m.sortState, &m.groupState, m.filterState.SearchQuery, len(m.workspaceRoots) > 1, m.activePreset)
	b.WriteString(m.infoBar.View())
	b.WriteString("\n")

	// Inline search line (when active)
	if m.searchActive {
		searchLine := searchStyle.Render("/") + m.searchInput.View()
		b.WriteString(searchLine)
		b.WriteString("\n")
	}

	// Task list
	b.WriteString(m.renderSectionedTasks())

	// Pin mode bar to bottom
	content := strings.TrimRight(b.String(), "\n")
	modeBar := m.infoBar.RenderModeBar(m.width)
	modeBarLines := strings.Count(modeBar, "\n") + 1
	contentLines := strings.Count(content, "\n") + 1
	if content == "" {
		contentLines = 0
	}
	if padding := m.height - contentLines - modeBarLines; padding > 0 {
		content += strings.Repeat("\n", padding)
	}
	return content + "\n" + modeBar
}

func maxBoardNameLen(tasks []data.Task) int {
	max := 0
	for _, t := range tasks {
		if t.IsTaskNote && len(t.BoardName) > max {
			max = len(t.BoardName)
		}
	}
	return max
}

func (m *TaskManagerModel) renderFlatTasks() string {
	var b strings.Builder

	if len(m.displayTasks) == 0 {
		b.WriteString(theme.Muted.Render("No tasks found."))
		return b.String()
	}

	bnw := maxBoardNameLen(m.displayTasks)

	visible := m.visibleTaskRows()
	end := m.scrollOffset + visible
	if end > len(m.displayTasks) {
		end = len(m.displayTasks)
	}

	for i := m.scrollOffset; i < end; i++ {
		task := m.displayTasks[i]
		var prefix string
		if m.inputContext.Mode == ModeArchive {
			box := "[ ]"
			if m.archiveSelection[task.ID] {
				box = "[x]"
			}
			if i == m.cursor {
				prefix = cursorStyle.Render(box+" ")
			} else {
				prefix = box + " "
			}
		} else if m.inputContext.Mode == ModeDelete {
			box := "[ ]"
			if m.deleteSelection[task.ID] {
				box = "[x]"
			}
			if i == m.cursor {
				prefix = cursorStyle.Render(box+" ")
			} else {
				prefix = box + " "
			}
		} else if m.inputContext.Mode == ModeConvertComplex {
			box := "[ ]"
			if m.convertSelection[task.ID] {
				box = "[x]"
			}
			if i == m.cursor {
				prefix = cursorStyle.Render(box + " ")
			} else {
				prefix = box + " "
			}
		} else {
			prefix = "  "
			if i == m.cursor {
				prefix = cursorStyle.Render("> ")
			}
		}
		line := prefix + shared.StyledTaskLine(task, m.width-lipgloss.Width(prefix), bnw)
		if i == m.cursor {
			line = shared.ApplyRowBackground(line, m.width)
		}
		b.WriteString(line + "\n")
	}

	return b.String()
}

func (m *TaskManagerModel) renderGroupedTasks() string {
	var b strings.Builder

	var allGroupedTasks []data.Task
	for _, group := range m.taskGroups {
		allGroupedTasks = append(allGroupedTasks, group.Tasks...)
	}
	bnw := maxBoardNameLen(allGroupedTasks)

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
			// Blank separator before header (skip at very top of viewport)
			if linesRendered > 0 {
				if linesRendered >= visible {
					break
				}
				b.WriteString("\n")
				linesRendered++
			}
			if linesRendered >= visible {
				break
			}
			b.WriteString(groupHeaderStyle.Render(group.Label))
			b.WriteString("\n")
			linesRendered++
		}

		for _, task := range group.Tasks {
			if linesRendered >= visible {
				break
			}
			if taskIndex >= m.scrollOffset {
				var prefix string
				if m.inputContext.Mode == ModeArchive {
					box := "[ ]"
					if m.archiveSelection[task.ID] {
						box = "[x]"
					}
					if taskIndex == m.cursor {
						prefix = cursorStyle.Render(box + " ")
					} else {
						prefix = box + " "
					}
				} else if m.inputContext.Mode == ModeDelete {
					box := "[ ]"
					if m.deleteSelection[task.ID] {
						box = "[x]"
					}
					if taskIndex == m.cursor {
						prefix = cursorStyle.Render(box + " ")
					} else {
						prefix = box + " "
					}
				} else if m.inputContext.Mode == ModeConvertComplex {
					box := "[ ]"
					if m.convertSelection[task.ID] {
						box = "[x]"
					}
					if taskIndex == m.cursor {
						prefix = cursorStyle.Render(box + " ")
					} else {
						prefix = box + " "
					}
				} else {
					prefix = "  "
					if taskIndex == m.cursor {
						prefix = cursorStyle.Render("> ")
					}
				}
				line := prefix + shared.StyledTaskLine(task, m.width-lipgloss.Width(prefix), bnw)
				if taskIndex == m.cursor {
					line = shared.ApplyRowBackground(line, m.width)
				}
				b.WriteString(line + "\n")
				linesRendered++
			}
			taskIndex++
		}
	}

	return b.String()
}

func (m *TaskManagerModel) renderSectionedTasks() string {
	var b strings.Builder

	if len(m.displayTasks) == 0 {
		b.WriteString(theme.Muted.Render("No tasks found."))
		return b.String()
	}

	var allTasks []data.Task
	for _, section := range m.taskSections {
		for _, group := range section.Groups {
			allTasks = append(allTasks, group.Tasks...)
		}
	}
	bnw := maxBoardNameLen(allTasks)

	visible := m.visibleTaskRows()
	linesRendered := 0
	taskIndex := 0

	for _, section := range m.taskSections {
		sectionStart := taskIndex
		sectionTaskCount := 0
		for _, g := range section.Groups {
			sectionTaskCount += len(g.Tasks)
		}
		sectionEnd := sectionStart + sectionTaskCount

		// Skip sections entirely before the scroll window
		if sectionEnd <= m.scrollOffset {
			taskIndex = sectionEnd
			continue
		}
		if linesRendered >= visible {
			break
		}

		// Blank line before section (skip at very top of viewport)
		if linesRendered > 0 {
			if linesRendered >= visible {
				break
			}
			b.WriteString("\n")
			linesRendered++
		}
		if linesRendered >= visible {
			break
		}

		// Section header
		b.WriteString(sectionHeaderStyle.Render(section.Label))
		b.WriteString("\n")
		linesRendered++

		firstGroup := true
		for _, group := range section.Groups {
			groupEnd := taskIndex + len(group.Tasks)

			if groupEnd <= m.scrollOffset {
				taskIndex = groupEnd
				firstGroup = false
				continue
			}
			if linesRendered >= visible {
				break
			}

			// Sub-group header (only when label is non-empty)
			if group.Label != "" {
				if !firstGroup {
					if linesRendered >= visible {
						break
					}
					b.WriteString("\n")
					linesRendered++
				}
				if linesRendered >= visible {
					break
				}
				b.WriteString(groupHeaderStyle.Render(group.Label))
				b.WriteString("\n")
				linesRendered++
			}
			firstGroup = false

			for _, task := range group.Tasks {
				if linesRendered >= visible {
					break
				}
				if taskIndex >= m.scrollOffset {
					var prefix string
					if m.inputContext.Mode == ModeArchive {
						box := "[ ]"
						if m.archiveSelection[task.ID] {
							box = "[x]"
						}
						if taskIndex == m.cursor {
							prefix = cursorStyle.Render(box + " ")
						} else {
							prefix = box + " "
						}
					} else if m.inputContext.Mode == ModeDelete {
						box := "[ ]"
						if m.deleteSelection[task.ID] {
							box = "[x]"
						}
						if taskIndex == m.cursor {
							prefix = cursorStyle.Render(box + " ")
						} else {
							prefix = box + " "
						}
					} else if m.inputContext.Mode == ModeConvertComplex {
						box := "[ ]"
						if m.convertSelection[task.ID] {
							box = "[x]"
						}
						if taskIndex == m.cursor {
							prefix = cursorStyle.Render(box + " ")
						} else {
							prefix = box + " "
						}
					} else {
						prefix = "  "
						if taskIndex == m.cursor {
							prefix = cursorStyle.Render("> ")
						}
					}
					line := prefix + shared.StyledTaskLine(task, m.width-lipgloss.Width(prefix), bnw)
					if taskIndex == m.cursor {
						line = shared.ApplyRowBackground(line, m.width)
					}
					b.WriteString(line + "\n")
					linesRendered++
				}
				taskIndex++
			}
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
	case "t":
		return m.startDirectTagEdit()
	case "p":
		return m.startDirectProjectEdit()
	case "i":
		return m.directCyclePriority()
	case "r":
		return m.startDirectNameEdit()
	case "U":
		return m.startDirectURLEdit()
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
	case "W":
		if len(m.workspaceRoots) > 1 {
			return m.startWorkspaceFilter()
		}
	case "/":
		return m.startSearch()
	case " ":
		return m.toggleTaskDone()
	case "n":
		return m.startNewTask()
	case "u":
		return m.handleOpenURL()
	case "ctrl+u":
		m.moveCursor(-20)
		return m, nil
	case "ctrl+d":
		m.moveCursor(20)
		return m, nil
	case "m":
		return m.startMoveToBoard()
	case "b":
		task := m.selectedTask()
		if task == nil || !task.IsTaskNote {
			return m, messages.StatusCmd("Not a task note", messages.LevelError)
		}
		parts := strings.SplitN(task.ID, ":", 3)
		if len(parts) < 3 {
			return m, messages.StatusCmd("Invalid task note ID", messages.LevelError)
		}
		filename := parts[2]
		for _, b := range m.boards {
			if b.Name == task.BoardName {
				for ci, col := range b.Columns {
					for ki, card := range col.TaskNotes {
						if card.Filename == filename {
							return m, func() tea.Msg {
								return messages.OpenBoardMsg{
									BoardPath: b.Path,
									ColIndex:  ci,
									CardIndex: ki,
								}
							}
						}
					}
				}
				return m, func() tea.Msg {
					return messages.OpenBoardMsg{BoardPath: b.Path}
				}
			}
		}
		return m, messages.StatusCmd("Board not found: "+task.BoardName, messages.LevelError)
	case "a":
		m.archiveSelection = make(map[string]bool)
		m.inputContext.TransitionTo(ModeArchive)
	case "d":
		m.deleteSelection = make(map[string]bool)
		m.inputContext.TransitionTo(ModeDelete)
	case "c":
		m.convertSelection = make(map[string]bool)
		m.inputContext.TransitionTo(ModeConvertComplex)
		m.refreshDisplayTasks()
	case "1":
		m.togglePreset(PresetStack)
		return m, nil
	case "2":
		m.togglePreset(PresetCache)
		return m, nil
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
	case "i":
		m.cyclePriorityFilter()
		m.inputContext.Reset()
	case "t", "c":
		return m.startTagFilter()
	case "s":
		m.activePreset = PresetNone
		m.filterState.CycleStatusFilter()
		m.refreshDisplayTasks()
		m.inputContext.Reset()
	case "f":
		return m.startFileFilter()
	case "w":
		if len(m.workspaceRoots) > 1 {
			return m.startWorkspaceFilter()
		}
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
	case "i":
		m.inputContext.Field = "priority"
		m.inputContext.TransitionTo(ModeSortDirection)
	case "t", "c":
		m.inputContext.Field = "tag"
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
	case "i":
		m.inputContext.Field = "priority"
		m.inputContext.TransitionTo(ModeGroupDirection)
	case "t", "c":
		m.inputContext.Field = "tag"
		m.inputContext.TransitionTo(ModeGroupDirection)
	case "b":
		m.inputContext.Field = "board"
		m.applyGroupField(true) // always ascending, no direction prompt
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

func (m TaskManagerModel) handleArchiveMode(msg tea.KeyMsg) (TaskManagerModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case " ":
		task := m.selectedTask()
		if task != nil {
			m.archiveSelection[task.ID] = !m.archiveSelection[task.ID]
			if !m.archiveSelection[task.ID] {
				delete(m.archiveSelection, task.ID)
			}
		}
	case "a":
		// Select all if any unselected; deselect all if all selected
		allSelected := len(m.archiveSelection) == len(m.displayTasks) && len(m.displayTasks) > 0
		if allSelected {
			m.archiveSelection = make(map[string]bool)
		} else {
			for _, t := range m.displayTasks {
				m.archiveSelection[t.ID] = true
			}
		}
	case "enter":
		count := len(m.archiveSelection)
		if count == 0 {
			return m, messages.StatusCmd("No tasks selected to archive", messages.LevelWarning)
		}
		m.confirmDialog = &shared.Dialog{
			Title: fmt.Sprintf("Archive %d task(s)?", count),
			Body:  "Selected tasks will be moved to archive/tasks/todo.txt",
			Hints: theme.Ok.Render("[y/enter]") + " Confirm  " + theme.Error.Render("[n/esc]") + " Cancel",
			Width: 50,
		}
		m.inputContext.TransitionTo(ModeConfirmation)
	case "esc":
		m.archiveSelection = nil
		m.inputContext.Reset()
	}
	return m, nil
}

func (m TaskManagerModel) handleDeleteMode(msg tea.KeyMsg) (TaskManagerModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case " ":
		task := m.selectedTask()
		if task != nil && !task.IsTaskNote {
			m.deleteSelection[task.ID] = !m.deleteSelection[task.ID]
			if !m.deleteSelection[task.ID] {
				delete(m.deleteSelection, task.ID)
			}
		}
	case "a":
		// Select all if any unselected; deselect all if all selected
		allSelected := len(m.deleteSelection) == len(m.displayTasks) && len(m.displayTasks) > 0
		if allSelected {
			m.deleteSelection = make(map[string]bool)
		} else {
			for _, t := range m.displayTasks {
				m.deleteSelection[t.ID] = true
			}
		}
	case "enter":
		count := len(m.deleteSelection)
		if count == 0 {
			return m, messages.StatusCmd("No tasks selected to delete", messages.LevelWarning)
		}
		m.confirmDialog = &shared.Dialog{
			Title: fmt.Sprintf("Delete %d task(s)?", count),
			Body:  "This will permanently remove the selected tasks.",
			Hints: theme.Ok.Render("[y/enter]") + " Confirm  " + theme.Error.Render("[n/esc]") + " Cancel",
			Width: 50,
		}
		m.inputContext.TransitionTo(ModeConfirmation)
	case "esc":
		m.deleteSelection = nil
		m.inputContext.Reset()
	}
	return m, nil
}

func (m TaskManagerModel) handleConvertMode(msg tea.KeyMsg) (TaskManagerModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case " ":
		task := m.selectedTask()
		if task != nil {
			m.convertSelection[task.ID] = !m.convertSelection[task.ID]
			if !m.convertSelection[task.ID] {
				delete(m.convertSelection, task.ID)
			}
		}
	case "a":
		allSelected := len(m.convertSelection) == len(m.displayTasks) && len(m.displayTasks) > 0
		if allSelected {
			m.convertSelection = make(map[string]bool)
		} else {
			for _, t := range m.displayTasks {
				m.convertSelection[t.ID] = true
			}
		}
	case "enter":
		count := len(m.convertSelection)
		if count == 0 {
			return m, messages.StatusCmd("No tasks selected to convert", messages.LevelWarning)
		}
		if len(m.boards) == 0 {
			return m, messages.StatusCmd("No workspace available", messages.LevelWarning)
		}
		// Default: convert without board assignment
		m.convertBoardPath = ""
		m.confirmDialog = &shared.Dialog{
			Title: fmt.Sprintf("Convert %d task(s) to tasknotes?", count),
			Body:  "Tasks will be created as unassigned tasknotes",
			Hints: theme.Ok.Render("[y/enter]") + " Confirm  " + theme.Error.Render("[n/esc]") + " Cancel",
			Width: 50,
		}
		m.inputContext.TransitionTo(ModeConfirmation)
		return m, nil
	case "b":
		count := len(m.convertSelection)
		if count == 0 {
			return m, messages.StatusCmd("No tasks selected to convert", messages.LevelWarning)
		}
		var realBoards []kanbanmodels.Board
		for _, b := range m.boards {
			if b.Path != "" {
				realBoards = append(realBoards, b)
			}
		}
		if len(realBoards) == 0 {
			return m, messages.StatusCmd("No boards available", messages.LevelWarning)
		}
		if len(realBoards) == 1 {
			m.convertBoardPath = realBoards[0].Path
			m.confirmDialog = &shared.Dialog{
				Title: fmt.Sprintf("Convert %d task(s) to tasknotes?", count),
				Body:  fmt.Sprintf("Tasks will be assigned to board \"%s\"", realBoards[0].Name),
				Hints: theme.Ok.Render("[y/enter]") + " Confirm  " + theme.Error.Render("[n/esc]") + " Cancel",
				Width: 50,
			}
			m.inputContext.TransitionTo(ModeConfirmation)
			return m, nil
		}
		boardNames := make([]string, len(realBoards))
		for i, b := range realBoards {
			boardNames[i] = b.Name
		}
		m.fuzzyPicker = NewFuzzyPicker(boardNames, "Convert to Board", false, false)
		m.pickerContext = "convert-to-board"
		m.inputContext.TransitionTo(ModeBoardPicker)
		return m, nil
	case "esc":
		m.convertSelection = nil
		m.convertBoardPath = ""
		m.inputContext.Reset()
		m.refreshDisplayTasks()
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
	if m.confirmDialog != nil {
		return m.handleConfirmationResult(false)
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
		if m.inputContext.Mode == ModeArchive {
			m.archiveSelection = nil
		}
		if m.inputContext.Mode == ModeDelete {
			m.deleteSelection = nil
		}
		if m.inputContext.Mode == ModeConvertComplex {
			m.convertSelection = nil
			m.convertBoardPath = ""
			m.refreshDisplayTasks()
		}
		m.inputContext.Back()
		if m.inputContext.Mode == ModeNormal {
			m.inputContext.Reset()
		}
		return m, nil
	}

	// In normal mode, clear filters, restore default grouping
	m.activePreset = PresetNone
	m.filterState.Reset()
	m.sortState.Reset()
	m.groupState = GroupState{Field: GroupByNone, Ascending: true}
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
	m.textInput.SetWidth(60)
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
		Tags:     []string{},
		Done:     false,
		Properties:   make(map[string]string),
		Priority: data.PriorityNone,
	}

	// Open editor with the new task
	m.taskEditor = NewTaskEditor(newTask, m.allProjectItems, m.allTags)
	m.taskEditor.Width = m.width
	m.taskEditor.Height = m.height
	m.inputContext.TransitionTo(ModeTaskEditor)
	return m, nil
}

func (m TaskManagerModel) handleOpenURL() (TaskManagerModel, tea.Cmd) {
	task := m.selectedTask()
	if task == nil {
		return m, nil
	}
	if url := task.GetURL(); url != "" {
		operations.OpenURL(url)
	}
	return m, nil
}

func (m TaskManagerModel) startDateFilter() (TaskManagerModel, tea.Cmd) {
	m.textInput = NewDateInput("Due date filter")
	m.textInput.SetWidth(m.width)
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

func (m TaskManagerModel) startTagFilter() (TaskManagerModel, tea.Cmd) {
	m.fuzzyPicker = NewFuzzyPicker(m.allTags, "Filter by Tag", true, false)
	m.fuzzyPicker.PreSelect(m.filterState.TagFilter)
	m.pickerContext = "filter-tag"
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

func (m TaskManagerModel) startWorkspaceFilter() (TaskManagerModel, tea.Cmd) {
	m.fuzzyPicker = NewFuzzyPicker(m.allWorkspaces, "Filter by Workspace", true, false)
	m.fuzzyPicker.PreSelect(m.filterState.WorkspaceFilter)
	m.pickerContext = "filter-workspace"
	m.inputContext.TransitionTo(ModeFuzzyPicker)
	return m, nil
}

func (m *TaskManagerModel) applyPreset(preset ViewPreset) {
	m.filterState.Reset()
	m.sortState.Reset()
	m.activePreset = preset

	switch preset {
	case PresetStack:
		m.filterState.StatusFilter = StatusPending
		m.filterState.PriorityPresence = PriorityPresenceHas
		m.sortState = SortState{Field: SortByPriority, Ascending: true}
	case PresetCache:
		m.filterState.StatusFilter = StatusPending
		m.filterState.PriorityPresence = PriorityPresenceNone
		m.sortState = SortState{Field: SortByDueDate, Ascending: true}
	}
	m.refreshDisplayTasks()
}

func (m *TaskManagerModel) togglePreset(preset ViewPreset) {
	if m.activePreset == preset {
		m.filterState.Reset()
		m.sortState.Reset()
		m.activePreset = PresetNone
		m.refreshDisplayTasks()
	} else {
		m.applyPreset(preset)
	}
}

func (m *TaskManagerModel) cyclePriorityFilter() {
	m.activePreset = PresetNone
	switch m.filterState.PriorityPresence {
	case PriorityPresenceAny:
		m.filterState.PriorityPresence = PriorityPresenceHas
	case PriorityPresenceHas:
		m.filterState.PriorityPresence = PriorityPresenceNone
	case PriorityPresenceNone:
		m.filterState.PriorityPresence = PriorityPresenceAny
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
	case "tag":
		field = SortByTag
	}

	m.activePreset = PresetNone
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
	case "tag":
		field = GroupByTag
	case "board":
		field = GroupByBoard
	}

	m.groupState.Field = field
	m.groupState.Ascending = ascending
	m.refreshDisplayTasks()
	m.inputContext.Reset()
}

func openTaskNoteEditorCmd(cardPath string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	c := exec.Command(editor, cardPath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return taskNoteEditorFinishedMsg{err: err}
	})
}

func (m TaskManagerModel) openTaskEditor() (TaskManagerModel, tea.Cmd) {
	task := m.selectedTask()
	if task == nil {
		return m, nil
	}
	if task.IsTaskNote {
		if task.File == "" {
			return m, messages.StatusCmd("Task note has no file path", messages.LevelError)
		}
		return m, openTaskNoteEditorCmd(task.File)
	}

	m.taskEditor = NewTaskEditor(task, m.allProjectItems, m.allTags)
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
	if task.IsTaskNote {
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
		if m.pickerContext == "convert-to-board" {
			m.convertSelection = nil
			m.convertBoardPath = ""
			m.refreshDisplayTasks()
		}
		m.inputContext.Reset()
		m.pickerContext = ""
		return m, nil
	}

	switch m.pickerContext {
	case "filter-project":
		m.activePreset = PresetNone
		m.filterState.ProjectFilter = msg.Selected
	case "filter-tag":
		m.activePreset = PresetNone
		m.filterState.TagFilter = msg.Selected
	case "filter-file":
		m.activePreset = PresetNone
		m.filterState.FileFilter = msg.Selected
	case "filter-workspace":
		m.activePreset = PresetNone
		m.filterState.WorkspaceFilter = msg.Selected
	case "edit-project":
		task := m.findTaskByID(m.directEditTaskID)
		if task != nil {
			task.Projects = msg.Selected
			m.refreshDisplayTasks()
			m.inputContext.Reset()
			m.pickerContext = ""
			m.directEditTaskID = ""
			return m, func() tea.Msg { return TaskUpdateMsg{Task: *task} }
		}
	case "edit-tag":
		task := m.findTaskByID(m.directEditTaskID)
		if task != nil {
			task.Tags = msg.Selected
			m.refreshDisplayTasks()
			m.inputContext.Reset()
			m.pickerContext = ""
			m.directEditTaskID = ""
			return m, func() tea.Msg { return TaskUpdateMsg{Task: *task} }
		}
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
	case "convert-to-board":
		if len(msg.Selected) > 0 {
			for _, b := range m.boards {
				if b.Name == msg.Selected[0] {
					m.convertBoardPath = b.Path
					count := len(m.convertSelection)
					m.confirmDialog = &shared.Dialog{
						Title: fmt.Sprintf("Convert %d task(s) to complex?", count),
						Body:  fmt.Sprintf("Tasks will be moved to board \"%s\"", b.Name),
						Hints: theme.Ok.Render("[y/enter]") + " Confirm  " + theme.Error.Render("[n/esc]") + " Cancel",
						Width: 50,
					}
					m.pickerContext = ""
					m.inputContext.TransitionTo(ModeConfirmation)
					return m, nil
				}
			}
		}
		// Nothing selected — cancel
		m.convertSelection = nil
		m.convertBoardPath = ""
		m.refreshDisplayTasks()
		m.inputContext.Reset()
		m.pickerContext = ""
		return m, nil
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
	} else if m.inputContext.Mode == ModeEditURL {
		// Direct URL editing
		task := m.findTaskByID(m.directEditTaskID)
		if task != nil {
			task.SetURL(msg.Value)
			m.directEditTaskID = ""
			m.inputContext.Reset()
			return m, func() tea.Msg { return TaskUpdateMsg{Task: *task} }
		}
	} else if m.inputContext.Mode == ModeEditName {
		// Direct name (rename) editing
		task := m.findTaskByID(m.directEditTaskID)
		if task != nil && strings.TrimSpace(msg.Value) != "" {
			task.Name = strings.TrimSpace(msg.Value)
			m.directEditTaskID = ""
			m.inputContext.Reset()
			return m, func() tea.Msg { return TaskUpdateMsg{Task: *task} }
		}
		m.directEditTaskID = ""
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

	// Apply workspace filter (needs roots context, separate from ApplyFilters)
	filtered = ApplyWorkspaceFilter(filtered, m.filterState.WorkspaceFilter, m.workspaceRoots)

	// Exclude archived tasks from the default view.
	// NOTE: must NOT use filtered[:0] here — filtered may share its backing array
	// with m.tasks/s.tasks. Filter-in-place would corrupt s.tasks, causing
	// WriteAllTasks to write duplicate tasks on the next update.
	{
		var nonArchived []data.Task
		for _, t := range filtered {
			if !t.Archived {
				nonArchived = append(nonArchived, t)
			}
		}
		filtered = nonArchived
	}

	// In convert mode, show only simple tasks (not TaskNotes)
	if m.inputContext.Mode == ModeConvertComplex {
		var simpleTasks []data.Task
		for _, t := range filtered {
			if !t.IsTaskNote {
				simpleTasks = append(simpleTasks, t)
			}
		}
		filtered = simpleTasks
	}

	// Always apply workspace meta-grouping
	m.taskSections = ApplyWorkspaceSections(filtered, m.groupState, m.sortState, m.workspaceRoots)

	// Flatten for cursor navigation
	m.displayTasks = nil
	for _, section := range m.taskSections {
		for _, group := range section.Groups {
			m.displayTasks = append(m.displayTasks, group.Tasks...)
		}
	}
	m.taskGroups = nil

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
// The info bar uses 2 lines (1 content + border), mode bar uses 2 lines (border + content).
func (m *TaskManagerModel) visibleTaskRows() int {
	used := 4 // info bar (2) + mode bar (2)
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
// In grouped mode it accounts for group header rows via countVisualRows.
func (m *TaskManagerModel) ensureCursorVisible() {
	visible := m.visibleTaskRows()
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
		return
	}
	for m.scrollOffset < m.cursor {
		if m.countVisualRows(m.scrollOffset, m.cursor+1) <= visible {
			break
		}
		m.scrollOffset++
	}
}

// countVisualRows returns the number of visual rows needed to display tasks [start, end)
// accounting for section headers, group headers, and blank separator lines.
func (m *TaskManagerModel) countVisualRows(start, end int) int {
	if end <= start {
		return 0
	}
	if len(m.taskSections) == 0 {
		return end - start
	}
	rows := 0
	taskIndex := 0
	firstSection := true
	for _, section := range m.taskSections {
		sectionTaskCount := 0
		for _, g := range section.Groups {
			sectionTaskCount += len(g.Tasks)
		}
		sectionEnd := taskIndex + sectionTaskCount

		if sectionEnd <= start {
			taskIndex = sectionEnd
			continue
		}
		if taskIndex >= end {
			break
		}

		sectionVisible := taskIndex >= start || (taskIndex < start && sectionEnd > start)
		if sectionVisible {
			if !firstSection {
				rows++ // blank separator before section header
			}
			rows++ // section header row
			firstSection = false
		}

		firstGroup := true
		for _, group := range section.Groups {
			groupEnd := taskIndex + len(group.Tasks)
			if groupEnd <= start {
				taskIndex = groupEnd
				firstGroup = false
				continue
			}
			if taskIndex >= end {
				break
			}
			if group.Label != "" {
				groupVisible := taskIndex >= start || (taskIndex < start && groupEnd > start)
				if groupVisible {
					if !firstGroup {
						rows++ // blank separator before sub-group header
					}
					rows++ // sub-group header row
				}
			}
			firstGroup = false
			for i := range group.Tasks {
				idx := taskIndex + i
				if idx >= start && idx < end {
					rows++
				}
			}
			taskIndex = groupEnd
		}
	}
	return rows
}

// handleStartArchive initiates the archive flow
func (m TaskManagerModel) handleStartArchive() (TaskManagerModel, tea.Cmd) {
	// Count completed tasks not yet archived
	count := 0
	for _, task := range m.tasks {
		if task.Done && !task.Archived {
			count++
		}
	}

	if count == 0 {
		return m, messages.StatusCmd("No completed tasks to archive", messages.LevelWarning)
	}

	m.confirmDialog = &shared.Dialog{
		Title: fmt.Sprintf("Archive %d completed task(s)?", count),
		Body:  "This will move completed tasks to archive/tasks/todo.txt",
		Hints: theme.Ok.Render("[y/enter]") + " Confirm  " + theme.Error.Render("[n/esc]") + " Cancel",
		Width: 50,
	}
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
	m.confirmDialog = &shared.Dialog{
		Title: "Delete task?",
		Body:  task.Name,
		Hints: theme.Ok.Render("[y/enter]") + " Confirm  " + theme.Error.Render("[n/esc]") + " Cancel",
		Width: 50,
	}
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
		return m, messages.StatusCmd("Cannot move completed tasks to a board", messages.LevelWarning)
	}
	var realBoards []kanbanmodels.Board
	for _, b := range m.boards {
		if b.Path != "" {
			realBoards = append(realBoards, b)
		}
	}
	if len(realBoards) == 0 {
		return m, messages.StatusCmd("No boards available", messages.LevelWarning)
	}

	// Single board — skip picker
	if len(realBoards) == 1 {
		t := *task
		boardPath := realBoards[0].Path
		return m, func() tea.Msg {
			return MoveTaskToBoardMsg{Task: t, BoardPath: boardPath}
		}
	}

	// Multiple boards — open picker
	boardNames := make([]string, len(realBoards))
	for i, b := range realBoards {
		boardNames[i] = b.Name
	}
	m.fuzzyPicker = NewFuzzyPicker(boardNames, "Move to Board", false, false)
	m.pickerContext = "move-to-board"
	m.inputContext.TransitionTo(ModeBoardPicker)
	return m, nil
}

// handleConfirmationResult processes the confirmation result
func (m TaskManagerModel) handleConfirmationResult(confirmed bool) (TaskManagerModel, tea.Cmd) {
	m.confirmDialog = nil
	m.inputContext.Reset()

	if !confirmed {
		m.pendingDeleteTaskID = ""
		m.archiveSelection = nil
		m.deleteSelection = nil
		m.convertSelection = nil
		m.convertBoardPath = ""
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

	// Archive-selection flow
	if len(m.archiveSelection) > 0 {
		ids := make([]string, 0, len(m.archiveSelection))
		for id := range m.archiveSelection {
			ids = append(ids, id)
		}
		m.archiveSelection = nil
		return m, func() tea.Msg {
			return ArchiveSelectionRequestMsg{IDs: ids}
		}
	}

	// Delete-selection flow
	if len(m.deleteSelection) > 0 {
		ids := make([]string, 0, len(m.deleteSelection))
		for id := range m.deleteSelection {
			ids = append(ids, id)
		}
		m.deleteSelection = nil
		return m, func() tea.Msg {
			return DeleteSelectionRequestMsg{IDs: ids}
		}
	}

	// Convert-selection flow (convertBoardPath may be "" for default/unassigned location)
	if len(m.convertSelection) > 0 {
		var tasksToConvert []data.Task
		for _, t := range m.tasks {
			if m.convertSelection[t.ID] {
				tasksToConvert = append(tasksToConvert, t)
			}
		}
		boardPath := m.convertBoardPath
		m.convertSelection = nil
		m.convertBoardPath = ""
		return m, func() tea.Msg {
			return ConvertToComplexSelectionMsg{Tasks: tasksToConvert, BoardPath: boardPath}
		}
	}

	// Legacy archive-all flow (StartArchiveMsg path)
	count := 0
	for _, task := range m.tasks {
		if task.Done && !task.Archived {
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
	if m.taskEditor != nil || m.fuzzyPicker != nil || m.textInput != nil || m.datePicker != nil || m.searchActive || m.confirmDialog != nil {
		return true
	}
	return m.inputContext.Mode != ModeNormal
}

// Direct edit handlers

func (m TaskManagerModel) startDirectDueDateEdit() (TaskManagerModel, tea.Cmd) {
	task := m.selectedTask()
	if task == nil {
		return m, nil
	}
	m.directEditTaskID = task.ID
	m.inputContext.TransitionTo(ModeEditDueDate)
	var currentDate *time.Time
	if dateStr := task.GetDueDate(); dateStr != "" {
		if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
			currentDate = &parsed
		}
	}
	dp := shared.NewDatePickerModel(currentDate, "Due Date")
	dp.SetSize(m.width, m.height)
	m.datePicker = &dp
	return m, dp.Init()
}

func (m TaskManagerModel) startDirectScheduledDateEdit() (TaskManagerModel, tea.Cmd) {
	task := m.selectedTask()
	if task == nil {
		return m, nil
	}
	m.directEditTaskID = task.ID
	m.inputContext.TransitionTo(ModeEditScheduledDate)
	var currentDate *time.Time
	if dateStr := task.GetScheduledDate(); dateStr != "" {
		if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
			currentDate = &parsed
		}
	}
	dp := shared.NewDatePickerModel(currentDate, "Scheduled Date")
	dp.SetSize(m.width, m.height)
	m.datePicker = &dp
	return m, dp.Init()
}

func (m TaskManagerModel) startDirectTagEdit() (TaskManagerModel, tea.Cmd) {
	task := m.selectedTask()
	if task == nil {
		return m, nil
	}
	m.directEditTaskID = task.ID
	m.fuzzyPicker = NewTagFuzzyPicker(m.allTags)
	m.fuzzyPicker.PreSelect(task.Tags)
	m.pickerContext = "edit-tag"
	m.inputContext.TransitionTo(ModeFuzzyPicker)
	return m, nil
}

func (m TaskManagerModel) startDirectProjectEdit() (TaskManagerModel, tea.Cmd) {
	task := m.selectedTask()
	if task == nil {
		return m, nil
	}
	m.directEditTaskID = task.ID
	picker := kanbanview.NewProjectPickerModel(task.Projects, m.allProjectItems)
	m.projectPicker = &picker
	return m, picker.Init()
}

func (m TaskManagerModel) directCyclePriority() (TaskManagerModel, tea.Cmd) {
	task := m.selectedTask()
	if task == nil {
		return m, nil
	}
	switch task.Priority {
	case data.PriorityNone:
		task.Priority = data.PriorityA
	case data.PriorityA:
		task.Priority = data.PriorityB
	case data.PriorityB:
		task.Priority = data.PriorityC
	case data.PriorityC:
		task.Priority = data.PriorityD
	case data.PriorityD:
		task.Priority = data.PriorityE
	case data.PriorityE:
		task.Priority = data.PriorityF
	case data.PriorityF:
		task.Priority = data.PriorityNone
	}
	return m, func() tea.Msg { return TaskUpdateMsg{Task: *task} }
}

func (m TaskManagerModel) startDirectNameEdit() (TaskManagerModel, tea.Cmd) {
	task := m.selectedTask()
	if task == nil {
		return m, nil
	}
	m.directEditTaskID = task.ID
	m.inputContext.TransitionTo(ModeEditName)
	m.textInput = NewTextInput("Rename Task", "Task name...", nil)
	m.textInput.SetWidth(m.width)
	m.textInput.SetValue(task.Name)
	return m, m.textInput.Focus()
}

func (m TaskManagerModel) startDirectURLEdit() (TaskManagerModel, tea.Cmd) {
	task := m.selectedTask()
	if task == nil {
		return m, nil
	}
	m.directEditTaskID = task.ID
	m.inputContext.TransitionTo(ModeEditURL)
	m.textInput = NewTextInput("URL", "https://example.com", nil)
	m.textInput.SetWidth(m.width)
	if currentURL := task.GetURL(); currentURL != "" {
		m.textInput.SetValue(currentURL)
	}
	return m, m.textInput.Focus()
}

func (m TaskManagerModel) handleDatePickerUpdate(msg tea.KeyMsg) (TaskManagerModel, tea.Cmd) {
	var cmd tea.Cmd
	*m.datePicker, cmd = m.datePicker.Update(msg)

	switch msg.String() {
	case "enter", "c":
		// Save date (or clear if 'c' was pressed)
		task := m.findTaskByID(m.directEditTaskID)
		if task != nil {
			date := m.datePicker.GetDate()
			dateStr := ""
			if date != nil {
				dateStr = date.Format("2006-01-02")
			}
			switch m.inputContext.Mode {
			case ModeEditDueDate:
				task.SetDueDate(dateStr)
			case ModeEditScheduledDate:
				task.SetScheduledDate(dateStr)
			}
			m.datePicker = nil
			m.directEditTaskID = ""
			m.inputContext.Reset()
			return m, func() tea.Msg { return TaskUpdateMsg{Task: *task} }
		}
		m.datePicker = nil
		m.directEditTaskID = ""
		m.inputContext.Reset()
		return m, nil

	case "esc":
		m.datePicker = nil
		m.directEditTaskID = ""
		m.inputContext.Reset()
		return m, nil
	}

	return m, cmd
}

func (m *TaskManagerModel) findTaskByID(id string) *data.Task {
	for i := range m.displayTasks {
		if m.displayTasks[i].ID == id {
			return &m.displayTasks[i]
		}
	}
	return nil
}
