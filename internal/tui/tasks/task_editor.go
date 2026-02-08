package tasks

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"wydo/internal/tasks/data"
)

var (
	editorTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	editorLabelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Width(12)
	editorValueStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	editorHelpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	editorBoxStyle      = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("4")).Padding(1, 2)
	editorModifiedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

// TaskEditorModel allows viewing and editing a task
type TaskEditorModel struct {
	task         *data.Task
	originalTask data.Task
	inputContext InputModeContext
	fuzzyPicker  *FuzzyPickerModel
	textInput    *TextInputModel
	allProjects  []string
	allContexts  []string
	Width        int
}

// TaskEditorResultMsg is sent when the editor closes
type TaskEditorResultMsg struct {
	Task      data.Task
	Saved     bool
	Cancelled bool
}

// NewTaskEditor creates a new task editor for the given task
func NewTaskEditor(task *data.Task, allProjects []string, allContexts []string) *TaskEditorModel {
	// Make a copy of the original task for comparison/cancel
	original := *task
	// Deep copy slices
	original.Projects = make([]string, len(task.Projects))
	copy(original.Projects, task.Projects)
	original.Contexts = make([]string, len(task.Contexts))
	copy(original.Contexts, task.Contexts)
	original.Tags = make(map[string]string)
	for k, v := range task.Tags {
		original.Tags[k] = v
	}

	return &TaskEditorModel{
		task:         task,
		originalTask: original,
		inputContext: InputModeContext{Mode: ModeTaskEditor},
		allProjects:  allProjects,
		allContexts:  allContexts,
		Width:        60,
	}
}

// Init implements tea.Model
func (m *TaskEditorModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m *TaskEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle sub-component updates first
	if m.fuzzyPicker != nil {
		return m.updateFuzzyPicker(msg)
	}
	if m.textInput != nil {
		return m.updateTextInput(msg)
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
		// Edit due date
		m.inputContext.Mode = ModeEditDueDate
		m.textInput = NewDateInput("Due Date")
		m.textInput.SetValue(m.task.GetDueDate())
		return m, m.textInput.Focus()

	case "p":
		// Edit projects
		m.inputContext.Mode = ModeEditProject
		m.fuzzyPicker = NewFuzzyPicker(m.allProjects, "Select Projects", true, true)
		m.fuzzyPicker.PreSelect(m.task.Projects)
		return m, nil

	case "t", "c":
		// Edit contexts
		m.inputContext.Mode = ModeEditContext
		m.fuzzyPicker = NewFuzzyPicker(m.allContexts, "Select Contexts", true, false)
		m.fuzzyPicker.PreSelect(m.task.Contexts)
		return m, nil

	case "P":
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

func (m *TaskEditorModel) updateFuzzyPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Check for result message
	if result, ok := msg.(FuzzyPickerResultMsg); ok {
		if !result.Cancelled {
			switch m.inputContext.Mode {
			case ModeEditProject:
				m.task.Projects = result.Selected
			case ModeEditContext:
				m.task.Contexts = result.Selected
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

func (m *TaskEditorModel) updateTextInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Check for result message
	if result, ok := msg.(TextInputResultMsg); ok {
		if !result.Cancelled {
			switch m.inputContext.Mode {
			case ModeEditDueDate:
				m.task.SetDueDate(result.Value)
			}
		}
		m.textInput = nil
		m.inputContext.Mode = ModeTaskEditor
		return m, nil
	}

	// Forward to text input
	updated, cmd := m.textInput.Update(msg)
	m.textInput = updated.(*TextInputModel)
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

// View implements tea.Model
func (m *TaskEditorModel) View() string {
	// If sub-component is active, show it
	if m.fuzzyPicker != nil {
		return m.fuzzyPicker.View()
	}
	if m.textInput != nil {
		return m.textInput.View()
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
	priStr := "(none)"
	if m.task.Priority != 0 {
		priStr = "(" + string(m.task.Priority) + ")"
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

	// Contexts
	content.WriteString(editorLabelStyle.Render("Contexts:"))
	ctxStr := "(none)"
	if len(m.task.Contexts) > 0 {
		ctxStr = "@" + strings.Join(m.task.Contexts, ", @")
	}
	if !slicesEqual(m.task.Contexts, m.originalTask.Contexts) {
		content.WriteString(editorModifiedStyle.Render(ctxStr + " *"))
	} else {
		content.WriteString(editorValueStyle.Render(ctxStr))
	}
	content.WriteString("\n\n")

	// Help
	content.WriteString(editorHelpStyle.Render("[d] due  [p] projects  [t] contexts  [P] priority"))
	content.WriteString("\n")
	content.WriteString(editorHelpStyle.Render("[enter] save  [esc] cancel"))

	return editorBoxStyle.Width(m.Width).Render(content.String())
}

// IsModified returns true if the task has been modified
func (m *TaskEditorModel) IsModified() bool {
	if m.task.Priority != m.originalTask.Priority {
		return true
	}
	if m.task.GetDueDate() != m.originalTask.GetDueDate() {
		return true
	}
	if !slicesEqual(m.task.Projects, m.originalTask.Projects) {
		return true
	}
	if !slicesEqual(m.task.Contexts, m.originalTask.Contexts) {
		return true
	}
	return false
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
