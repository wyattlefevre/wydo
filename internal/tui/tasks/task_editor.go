package tasks

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"wydo/internal/kanban/operations"
	"wydo/internal/tasks/data"
	"wydo/internal/tui/shared"
	"wydo/internal/tui/theme"
	kanbanview "wydo/internal/tui/kanban"
)

var (
	editorTitleStyle    = theme.Title
	editorLabelStyle    = lipgloss.NewStyle().Foreground(theme.Secondary).Width(12)
	editorValueStyle    = lipgloss.NewStyle().Foreground(theme.Text)
	editorHelpStyle     = theme.ModalHelp
	editorBoxStyle      = theme.ModalBox
	editorModifiedStyle = lipgloss.NewStyle().Foreground(theme.Warning)
)

// TaskEditorModel allows viewing and editing a task
type TaskEditorModel struct {
	task            *data.Task
	originalTask    data.Task
	inputContext    InputModeContext
	fuzzyPicker     *FuzzyPickerModel
	datePicker      *shared.DatePickerModel
	urlInput        *TextInputModel
	projectPicker   *kanbanview.ProjectPickerModel
	allProjectItems []kanbanview.ProjectPickerItem
	allTags         []string
	Width           int
	Height          int
}

// TaskEditorResultMsg is sent when the editor closes
type TaskEditorResultMsg struct {
	Task      data.Task
	Saved     bool
	Cancelled bool
}

// NewTaskEditor creates a new task editor for the given task
func NewTaskEditor(task *data.Task, allProjectItems []kanbanview.ProjectPickerItem, allTags []string) *TaskEditorModel {
	// Make a copy of the original task for comparison/cancel
	original := *task
	// Deep copy slices
	original.Projects = make([]string, len(task.Projects))
	copy(original.Projects, task.Projects)
	original.Tags = make([]string, len(task.Tags))
	copy(original.Tags, task.Tags)
	original.Properties = make(map[string]string)
	for k, v := range task.Properties {
		original.Properties[k] = v
	}

	return &TaskEditorModel{
		task:            task,
		originalTask:    original,
		inputContext:    InputModeContext{Mode: ModeTaskEditor},
		allProjectItems: allProjectItems,
		allTags:         allTags,
		Width:           60,
	}
}

// Init implements tea.Model
func (m *TaskEditorModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m *TaskEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle URL input first
	if m.urlInput != nil {
		return m.updateURLInput(msg)
	}
	// Handle project picker
	if m.projectPicker != nil {
		return m.updateProjectPicker(msg)
	}
	// Handle date picker
	if m.datePicker != nil {
		return m.updateDatePicker(msg)
	}
	// Handle sub-component updates
	if m.fuzzyPicker != nil {
		return m.updateFuzzyPicker(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.inputContext.Mode {
		case ModeTaskEditor:
			return m.handleTaskEditorKeys(msg)
		}
	}

	return m, nil
}

func (m *TaskEditorModel) handleTaskEditorKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "d":
		// Edit due date with calendar picker
		m.inputContext.Mode = ModeEditDueDate
		var currentDate *time.Time
		if dateStr := m.task.GetDueDate(); dateStr != "" {
			if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
				currentDate = &parsed
			}
		}
		dp := shared.NewDatePickerModel(currentDate, "Due Date")
		dp.SetSize(m.Width, m.Height)
		m.datePicker = &dp
		return m, dp.Init()

	case "s":
		// Edit scheduled date with calendar picker
		m.inputContext.Mode = ModeEditScheduledDate
		var currentDate *time.Time
		if dateStr := m.task.GetScheduledDate(); dateStr != "" {
			if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
				currentDate = &parsed
			}
		}
		dp := shared.NewDatePickerModel(currentDate, "Scheduled Date")
		dp.SetSize(m.Width, m.Height)
		m.datePicker = &dp
		return m, dp.Init()

	case "p":
		// Edit projects
		m.inputContext.Mode = ModeEditProject
		picker := kanbanview.NewProjectPickerModel(m.task.Projects, m.allProjectItems)
		m.projectPicker = &picker
		return m, picker.Init()

	case "t", "c":
		// Edit tags
		m.inputContext.Mode = ModeEditTag
		m.fuzzyPicker = NewFuzzyPicker(m.allTags, "Select Tags", true, false)
		m.fuzzyPicker.PreSelect(m.task.Tags)
		return m, nil

	case "U":
		// Edit URL
		m.inputContext.Mode = ModeEditURL
		m.urlInput = NewTextInput("URL", "https://example.com", nil)
		m.urlInput.SetWidth(m.Width)
		if currentURL := m.task.GetURL(); currentURL != "" {
			m.urlInput.SetValue(currentURL)
		}
		return m, m.urlInput.Focus()

	case "u":
		// Open URL in browser
		if url := m.task.GetURL(); url != "" {
			operations.OpenURL(url)
		}
		return m, nil

	case "i":
		// Cycle priority: A -> B -> C -> D -> E -> F -> none -> A
		m.cyclePriority()
		return m, nil

	case "enter":
		// Save and close
		return m, func() tea.Msg {
			return TaskEditorResultMsg{
				Task:      *m.task,
				Saved:     true,
				Cancelled: false,
			}
		}

	case "esc":
		// Cancel - restore original task
		*m.task = m.originalTask
		return m, func() tea.Msg {
			return TaskEditorResultMsg{
				Task:      m.originalTask,
				Saved:     false,
				Cancelled: true,
			}
		}
	}

	return m, nil
}

func (m *TaskEditorModel) updateProjectPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var isDone bool
	var cancelled bool
	*m.projectPicker, cmd, isDone, cancelled = m.projectPicker.Update(msg)
	_ = cancelled // projectpicker uses key-based save detection
	if isDone {
		m.task.Projects = m.projectPicker.GetSelectedProjects()
		m.projectPicker = nil
		m.inputContext.Mode = ModeTaskEditor
		return m, nil
	}
	return m, cmd
}

func (m *TaskEditorModel) updateFuzzyPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Check for result message
	if result, ok := msg.(FuzzyPickerResultMsg); ok {
		if !result.Cancelled {
			switch m.inputContext.Mode {
			case ModeEditProject:
				m.task.Projects = result.Selected
			case ModeEditTag:
				m.task.Tags = result.Selected
			}
		}
		m.fuzzyPicker = nil
		m.inputContext.Mode = ModeTaskEditor
		return m, nil
	}

	// Forward to picker
	updated, cmd := m.fuzzyPicker.Update(msg)
	m.fuzzyPicker = updated.(*FuzzyPickerModel)
	return m, cmd
}

func (m *TaskEditorModel) updateDatePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	var cmd tea.Cmd
	*m.datePicker, cmd = m.datePicker.Update(keyMsg)

	switch keyMsg.String() {
	case "enter", "c":
		// Save date (or clear if 'c' was pressed)
		date := m.datePicker.GetDate()
		dateStr := ""
		if date != nil {
			dateStr = date.Format("2006-01-02")
		}
		switch m.inputContext.Mode {
		case ModeEditDueDate:
			m.task.SetDueDate(dateStr)
		case ModeEditScheduledDate:
			m.task.SetScheduledDate(dateStr)
		}
		m.datePicker = nil
		m.inputContext.Mode = ModeTaskEditor
		return m, nil

	case "esc":
		// Cancel date picker
		m.datePicker = nil
		m.inputContext.Mode = ModeTaskEditor
		return m, nil
	}

	return m, cmd
}

func (m *TaskEditorModel) updateURLInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Check for result message
	if result, ok := msg.(TextInputResultMsg); ok {
		if !result.Cancelled {
			m.task.SetURL(result.Value)
		}
		m.urlInput = nil
		m.inputContext.Mode = ModeTaskEditor
		return m, nil
	}

	// Forward to text input
	_, cmd := m.urlInput.Update(msg)
	return m, cmd
}

func (m *TaskEditorModel) cyclePriority() {
	switch m.task.Priority {
	case data.PriorityNone:
		m.task.Priority = data.PriorityA
	case data.PriorityA:
		m.task.Priority = data.PriorityB
	case data.PriorityB:
		m.task.Priority = data.PriorityC
	case data.PriorityC:
		m.task.Priority = data.PriorityD
	case data.PriorityD:
		m.task.Priority = data.PriorityE
	case data.PriorityE:
		m.task.Priority = data.PriorityF
	case data.PriorityF:
		m.task.Priority = data.PriorityNone
	}
}

// HasActiveSubComponent returns true when a sub-picker (date, project, context, URL)
// is currently open inside the editor. Callers use this to decide whether the editor
// should be shown as a full-screen component or composited as an overlay box.
func (m *TaskEditorModel) HasActiveSubComponent() bool {
	return m.urlInput != nil || m.datePicker != nil || m.projectPicker != nil || m.fuzzyPicker != nil
}

// View implements tea.Model
func (m *TaskEditorModel) View() string {
	// If URL input is active, show it
	if m.urlInput != nil {
		return m.urlInput.View()
	}
	// If date picker is active, show it
	if m.datePicker != nil {
		return m.datePicker.View()
	}
	// If project picker is active, show it
	if m.projectPicker != nil {
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, m.projectPicker.View())
	}
	// If sub-component is active, show it
	if m.fuzzyPicker != nil {
		return m.fuzzyPicker.View()
	}

	var content strings.Builder

	// Title
	content.WriteString(editorTitleStyle.Render("Edit Task"))
	content.WriteString("\n\n")

	// Task name
	content.WriteString(editorLabelStyle.Render("Name:"))
	content.WriteString(editorValueStyle.Render(m.task.Name))
	content.WriteString("\n")

	// Priority
	content.WriteString(editorLabelStyle.Render("Priority:"))
	priStr := "none"
	if m.task.Priority != 0 {
		priStr = string(m.task.Priority)
	}
	if m.task.Priority != m.originalTask.Priority {
		content.WriteString(editorModifiedStyle.Render(priStr + " *"))
	} else {
		content.WriteString(editorValueStyle.Render(priStr))
	}
	content.WriteString("\n")

	// Due date
	content.WriteString(editorLabelStyle.Render("Due:"))
	dueStr := m.task.GetDueDate()
	if dueStr == "" {
		dueStr = "(none)"
	}
	if m.task.GetDueDate() != m.originalTask.GetDueDate() {
		content.WriteString(editorModifiedStyle.Render(dueStr + " *"))
	} else {
		content.WriteString(editorValueStyle.Render(dueStr))
	}
	content.WriteString("\n")

	// Scheduled date
	content.WriteString(editorLabelStyle.Render("Scheduled:"))
	schedStr := m.task.GetScheduledDate()
	if schedStr == "" {
		schedStr = "(none)"
	}
	if m.task.GetScheduledDate() != m.originalTask.GetScheduledDate() {
		content.WriteString(editorModifiedStyle.Render(schedStr + " *"))
	} else {
		content.WriteString(editorValueStyle.Render(schedStr))
	}
	content.WriteString("\n")

	// Projects
	content.WriteString(editorLabelStyle.Render("Projects:"))
	projStr := "(none)"
	if len(m.task.Projects) > 0 {
		projStr = "+" + strings.Join(m.task.Projects, ", +")
	}
	if !slicesEqual(m.task.Projects, m.originalTask.Projects) {
		content.WriteString(editorModifiedStyle.Render(projStr + " *"))
	} else {
		content.WriteString(editorValueStyle.Render(projStr))
	}
	content.WriteString("\n")

	// Tags
	content.WriteString(editorLabelStyle.Render("Tags:"))
	ctxStr := "(none)"
	if len(m.task.Tags) > 0 {
		ctxStr = "@" + strings.Join(m.task.Tags, ", @")
	}
	if !slicesEqual(m.task.Tags, m.originalTask.Tags) {
		content.WriteString(editorModifiedStyle.Render(ctxStr + " *"))
	} else {
		content.WriteString(editorValueStyle.Render(ctxStr))
	}
	content.WriteString("\n")

	// URL
	content.WriteString(editorLabelStyle.Render("URL:"))
	urlStr := m.task.GetURL()
	if urlStr == "" {
		urlStr = "(none)"
	}
	if m.task.GetURL() != m.originalTask.GetURL() {
		content.WriteString(editorModifiedStyle.Render(urlStr + " *"))
	} else {
		content.WriteString(editorValueStyle.Render(urlStr))
	}
	content.WriteString("\n\n")

	// Help
	content.WriteString(editorHelpStyle.Render("[d] due  [s] sched  [p] project  [t] tag  [i] priority  [U] url  [u] open url"))
	content.WriteString("\n")
	content.WriteString(editorHelpStyle.Render("[enter] save  [esc] cancel"))

	return editorBoxStyle.Width(m.Width).Render(content.String())
}

// slicesEqual compares two string slices for equality
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
