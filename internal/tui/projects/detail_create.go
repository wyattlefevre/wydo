package projects

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"wydo/internal/kanban/fs"
	"wydo/internal/kanban/operations"
	"wydo/internal/tasks/data"
	"wydo/internal/tui/kanban"
	"wydo/internal/tui/messages"
	taskview "wydo/internal/tui/tasks"
	"wydo/internal/workspace"

	tea "github.com/charmbracelet/bubbletea"
	"wydo/internal/logs"
	"github.com/charmbracelet/lipgloss"
)

// cardEditorFinishedMsg is sent when the editor process for a new card exits.
type cardEditorFinishedMsg struct{ err error }

// createSubProjectPickerModel is a simple j/k list for selecting which project
// to add a new item to.
type createSubProjectPickerModel struct {
	projects []*workspace.Project
	cursor   int
	width    int
	height   int
}

// update processes a key and returns the selected project (non-nil when done/confirmed)
// and a done flag.
func (p *createSubProjectPickerModel) update(msg tea.KeyMsg) (*workspace.Project, bool) {
	switch msg.String() {
	case "j", "down":
		if p.cursor < len(p.projects)-1 {
			p.cursor++
		}
	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
		}
	case "enter":
		if len(p.projects) > 0 && p.cursor < len(p.projects) {
			return p.projects[p.cursor], true
		}
		return nil, true
	case "esc":
		return nil, true
	}
	return nil, false
}

func (p *createSubProjectPickerModel) View() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Add to Project"))
	lines = append(lines, "")
	for i, proj := range p.projects {
		prefix := "  "
		if i == p.cursor {
			prefix = "> "
			lines = append(lines, selectedDetailItemStyle.Render(prefix+proj.Name))
		} else {
			lines = append(lines, detailItemStyle.Render(prefix+proj.Name))
		}
	}
	lines = append(lines, "")
	lines = append(lines, pathStyle.Render("j/k: navigate  enter: select  esc: cancel"))

	content := strings.Join(lines, "\n")
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2).
		Render(content)
	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, box)
}

// handleNew is called when 'n' is pressed in normal mode.
func (m DetailModel) handleNew() (DetailModel, tea.Cmd) {
	if len(m.allDescendants) > 0 {
		// Build project list: root first, then all descendants.
		projects := make([]*workspace.Project, 0, 1+len(m.allDescendants))
		projects = append(projects, m.project)
		projects = append(projects, m.allDescendants...)

		// Pre-select based on cursor context.
		preselect := 0
		if row := m.currentRow(); row != nil {
			for i, p := range projects {
				if p.Name == row.projectName {
					preselect = i
					break
				}
			}
		}

		picker := &createSubProjectPickerModel{
			projects: projects,
			cursor:   preselect,
			width:    m.width,
			height:   m.height,
		}
		m.createSubProjectPicker = picker
		m.mode = detailModeSubProjectPick
		return m, nil
	}

	// No sub-projects — skip straight to item creation.
	m.pendingProject = m.project
	return m.startItemCreation()
}

// updateSubProjectPicker handles keys when in detailModeSubProjectPick.
func (m DetailModel) updateSubProjectPicker(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	if m.createSubProjectPicker == nil {
		m.mode = detailModeNormal
		return m, nil
	}
	selected, done := m.createSubProjectPicker.update(msg)
	if !done {
		return m, nil
	}
	m.createSubProjectPicker = nil
	if selected == nil {
		// Cancelled.
		m.mode = detailModeNormal
		return m, nil
	}
	m.pendingProject = selected
	return m.startItemCreation()
}

// startItemCreation branches on the selected column to start the right flow.
func (m DetailModel) startItemCreation() (DetailModel, tea.Cmd) {
	switch colKind(m.selectedCol) {
	case colNotes:
		input := taskview.NewTextInput("Note filename", "my-note", nil)
		input.SetWidth(m.width)
		m.createNoteInput = input
		m.mode = detailModeNewNoteName
		return m, input.Focus()

	case colTasks:
		input := taskview.NewTextInput("Task name", "do something...", nil)
		input.SetWidth(m.width)
		m.createTaskInput = input
		m.mode = detailModeNewTaskName
		return m, input.Focus()

	case colCards:
		selector := kanban.NewBoardSelectorModel(m.allBoards, "", "Select Board")
		selector.SetSize(m.width, m.height)
		m.createBoardPicker = &selector
		m.mode = detailModeNewBoardPick
		return m, nil
	}
	m.mode = detailModeNormal
	return m, nil
}

// updateNewNoteInput forwards keys to the note name text input.
// TextInputResultMsg arrives in Update() and is dispatched to handleTextInputResult.
func (m DetailModel) updateNewNoteInput(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	if m.createNoteInput == nil {
		m.mode = detailModeNormal
		return m, nil
	}
	result, cmd := m.createNoteInput.Update(msg)
	m.createNoteInput = result.(*taskview.TextInputModel)
	return m, cmd
}

// updateNewTaskInput forwards keys to the task name text input.
func (m DetailModel) updateNewTaskInput(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	if m.createTaskInput == nil {
		m.mode = detailModeNormal
		return m, nil
	}
	result, cmd := m.createTaskInput.Update(msg)
	m.createTaskInput = result.(*taskview.TextInputModel)
	return m, cmd
}

// handleTextInputResult is called when TextInputResultMsg arrives in Update().
func (m DetailModel) handleTextInputResult(msg taskview.TextInputResultMsg) (DetailModel, tea.Cmd) {
	if msg.Cancelled {
		m.createNoteInput = nil
		m.createTaskInput = nil
		m.mode = detailModeNormal
		return m, nil
	}
	switch m.mode {
	case detailModeNewNoteName:
		return m.finishNoteCreation(msg.Value)
	case detailModeNewTaskName:
		return m.finishTaskNameEntry(msg.Value)
	}
	m.mode = detailModeNormal
	return m, nil
}

// finishNoteCreation creates the note file and opens it in $EDITOR.
func (m DetailModel) finishNoteCreation(name string) (DetailModel, tea.Cmd) {
	m.createNoteInput = nil
	m.mode = detailModeNormal

	if strings.TrimSpace(name) == "" {
		return m, nil
	}
	if m.pendingProject == nil || m.pendingProject.DirPath == "" {
		name := ""
		if m.pendingProject != nil {
			name = m.pendingProject.Name
		}
		logs.Logger.Printf("Cannot create note: project %q has no directory", name)
		return m, nil
	}

	// Normalize filename.
	filename := strings.TrimSpace(name)
	filename = strings.TrimSuffix(filename, ".md")
	filename = filename + ".md"

	filePath := filepath.Join(m.pendingProject.DirPath, filename)

	if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
		logs.Logger.Printf("Error creating note file: %v", err)
		return m, nil
	}

	return m, openNoteInEditor(filePath)
}

// finishTaskNameEntry opens the task editor with the project pre-populated.
func (m DetailModel) finishTaskNameEntry(name string) (DetailModel, tea.Cmd) {
	m.createTaskInput = nil

	if strings.TrimSpace(name) == "" {
		m.mode = detailModeNormal
		return m, nil
	}

	projectName := ""
	if m.pendingProject != nil {
		projectName = m.pendingProject.Name
	}

	task := &data.Task{
		Name:     strings.TrimSpace(name),
		Projects: []string{projectName},
		Contexts: []string{},
		Tags:     make(map[string]string),
	}

	editor := taskview.NewTaskEditor(task, m.allProjectItems, m.allContexts)
	editor.Width = m.width
	editor.Height = m.height
	m.createTaskEditor = editor
	m.mode = detailModeNewTaskEditor
	return m, nil
}

// updateNewTaskEditor forwards keys to the task editor.
// TaskEditorResultMsg arrives in Update() and is dispatched to handleTaskEditorResult.
func (m DetailModel) updateNewTaskEditor(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	if m.createTaskEditor == nil {
		m.mode = detailModeNormal
		return m, nil
	}
	result, cmd := m.createTaskEditor.Update(msg)
	m.createTaskEditor = result.(*taskview.TaskEditorModel)
	return m, cmd
}

// handleTaskEditorResult is called when TaskEditorResultMsg arrives in Update().
func (m DetailModel) handleTaskEditorResult(msg taskview.TaskEditorResultMsg) (DetailModel, tea.Cmd) {
	m.createTaskEditor = nil
	m.mode = detailModeNormal

	if msg.Cancelled {
		return m, nil
	}

	task := msg.Task
	updateCmd := func() tea.Msg { return taskview.TaskUpdateMsg{Task: task} }
	refreshCmd := func() tea.Msg { return messages.DataRefreshMsg{} }
	return m, tea.Batch(updateCmd, refreshCmd)
}

// updateNewBoardPick handles keys when in detailModeNewBoardPick.
func (m DetailModel) updateNewBoardPick(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	if m.createBoardPicker == nil {
		m.mode = detailModeNormal
		return m, nil
	}
	picker, selectedPath, done := m.createBoardPicker.Update(msg)
	m.createBoardPicker = &picker
	if !done {
		return m, nil
	}
	m.createBoardPicker = nil
	m.mode = detailModeNormal
	if selectedPath == "" {
		return m, nil
	}
	return m.finishCardCreation(selectedPath)
}

// finishCardCreation loads the board, creates the card, and opens it in $EDITOR.
func (m DetailModel) finishCardCreation(boardPath string) (DetailModel, tea.Cmd) {
	projectName := ""
	if m.pendingProject != nil {
		projectName = m.pendingProject.Name
	}

	board, err := fs.ReadBoard(boardPath)
	if err != nil {
		logs.Logger.Printf("Error loading board for card creation: %v", err)
		return m, nil
	}

	card, err := operations.CreateCardFromTask(&board, "", []string{projectName}, []string{}, nil, nil, 0)
	if err != nil {
		logs.Logger.Printf("Error creating card: %v", err)
		return m, nil
	}

	cardPath := filepath.Join(board.Path, "cards", card.Filename)
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	c := exec.Command(editor, cardPath)
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return cardEditorFinishedMsg{err: err}
	})
}
