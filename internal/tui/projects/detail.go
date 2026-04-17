package projects

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	xansi "github.com/charmbracelet/x/ansi"
	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/kanban/operations"
	"wydo/internal/tasks/data"
	"wydo/internal/tui/kanban"
	"wydo/internal/tui/messages"
	"wydo/internal/tui/shared"
	taskview "wydo/internal/tui/tasks"
	"wydo/internal/tui/theme"
	"wydo/internal/workspace"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type colKind int

const (
	colTasks colKind = iota
	colCount
)

type rowKind int

const (
	rowKindGroup rowKind = iota
	rowKindTask
)

type detailRow struct {
	kind        rowKind
	depth       int
	projectName string

	// Only populated when kind == rowKindTask:
	task data.Task
}

type detailMode int

const (
	detailModeNormal         detailMode = iota
	detailModeURLEditor                 // editing project URLs
	detailModeURLPicker                 // picking a URL to open
	detailModeDateEditor                // editing project dates
	detailModeSubProjectPick            // picking which sub-project to add item to
	detailModeNewTaskName               // text input for task name
	detailModeNewTaskEditor             // task editor modal
	detailModeNewBoardPick              // board selector for new card
	detailModeChildPicker               // picking a child project to open
)

// DetailModel shows project details with notes, tasks, and cards in a
// kanban-style column layout with hierarchical grouping by child project.
type DetailModel struct {
	name         string
	wsDir        string
	project      *workspace.Project
	registry     *workspace.ProjectRegistry
	width, height int

	// Pre-computed per-project data (keyed by project name)
	projectTasks map[string][]data.Task
	allDescendants []*workspace.Project

	// Raw all-data
	allBoards []kanbanmodels.Board
	allTasks  []data.Task

	// Column state
	columns        [colCount][]detailRow
	selectedCol    int
	colScrollOff   [colCount]int
	colCursorPos   [colCount]int
	colHorizOffset int // first visible column index (horizontal scroll)

	// Collapse state: per-column, project name → collapsed
	collapsedGroups [colCount]map[string]bool

	// Modal state
	mode        detailMode
	urlEditor   *kanban.URLEditorModel
	urlPicker   *projectURLPicker
	dateEditor  *DateEditorModel
	childPicker *detailChildPicker

	// Creation flow state (all nil when not in use)
	createSubProjectPicker *createSubProjectPickerModel
	createTaskInput        *taskview.TextInputModel
	createTaskEditor       *taskview.TaskEditorModel
	createBoardPicker      *kanban.BoardSelectorModel
	pendingProject         *workspace.Project

	// Data for task editor
	allProjectItems []kanban.ProjectPickerItem
	allTags         []string
}

// detailURLEntry is a URL with its owning project name.
type detailURLEntry struct {
	projectName string
	url         kanbanmodels.TaskNoteURL
}

type noteEditorFinishedMsg struct{ err error }

func openNoteInEditor(filePath string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	c := exec.Command(editor, filePath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return noteEditorFinishedMsg{err: err}
	})
}

// projectURLPicker is a grouped URL picker for the project detail view.
type projectURLPicker struct {
	entries []detailURLEntry
	cursor  int
	width   int
	height  int
}

// detailChildPicker is an inline picker for selecting a child project.
type detailChildPicker struct {
	entries []*workspace.Project
	cursor  int
	width   int
	height  int
}

func (p detailChildPicker) Update(msg tea.KeyMsg) (detailChildPicker, *workspace.Project, bool) {
	switch msg.String() {
	case "j", "down":
		if p.cursor < len(p.entries)-1 {
			p.cursor++
		}
	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
		}
	case "enter":
		if len(p.entries) > 0 && p.cursor < len(p.entries) {
			return p, p.entries[p.cursor], true
		}
		return p, nil, true
	case "esc":
		return p, nil, true
	}
	return p, nil, false
}

func (p detailChildPicker) View() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Switch to Child Project"))
	lines = append(lines, "")

	if len(p.entries) == 0 {
		lines = append(lines, pathStyle.Render("No children"))
	} else {
		for i, proj := range p.entries {
			style := theme.ListItem
			prefix := "  "
			if i == p.cursor {
				style = selectedDetailItemStyle
				prefix = "> "
			}
			lines = append(lines, style.Render(prefix+proj.Name))
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2).
		Render(content)
	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, box)
}

func (p projectURLPicker) Update(msg tea.KeyMsg) (projectURLPicker, string, bool) {
	switch msg.String() {
	case "j", "down":
		if p.cursor < len(p.entries)-1 {
			p.cursor++
		}
	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
		}
	case "enter":
		if len(p.entries) > 0 && p.cursor < len(p.entries) {
			return p, p.entries[p.cursor].url.URL, true
		}
		return p, "", true
	case "esc":
		return p, "", true
	}
	return p, "", false
}

func (p projectURLPicker) View() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("Open URL"))
	s.WriteString("\n\n")

	if len(p.entries) == 0 {
		s.WriteString(pathStyle.Render("No URLs"))
		s.WriteString("\n")
	} else {
		lastProject := ""
		for i, e := range p.entries {
			if e.projectName != lastProject {
				if lastProject != "" {
					s.WriteString("\n")
				}
				s.WriteString(sectionHeaderStyle.Render(e.projectName))
				s.WriteString("\n")
				lastProject = e.projectName
			}
			prefix := "  "
			if i == p.cursor {
				prefix = "> "
			}
			u := e.url
			var line string
			if u.Label != "" {
				if i == p.cursor {
					line = selectedDetailItemStyle.Render(prefix+u.Label) + pathStyle.Render("  "+u.URL)
				} else {
					line = detailItemStyle.Render(prefix+u.Label) + pathStyle.Render("  "+u.URL)
				}
			} else {
				if i == p.cursor {
					line = selectedDetailItemStyle.Render(prefix + u.URL)
				} else {
					line = pathStyle.Render(prefix + u.URL)
				}
			}
			s.WriteString(line)
			s.WriteString("\n")
		}
	}

	s.WriteString("\n")
	s.WriteString(pathStyle.Render("j/k: navigate  enter: open  esc: cancel"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2).
		Render(s.String())

	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, box)
}

func NewDetailModel(name, wsDir string, tasks []data.Task, taskNotes []kanbanmodels.TaskNote, boards []kanbanmodels.Board, allBoards []kanbanmodels.Board, project *workspace.Project, registry *workspace.ProjectRegistry, children []*workspace.Project, allTasks []data.Task, allProjectItems []kanban.ProjectPickerItem, allTags []string) DetailModel {
	// Build board+column lookup for TaskNote → Task conversion.
	tnBoard := make(map[string]kanbanmodels.Board)
	tnCol := make(map[string]kanbanmodels.Column)
	for _, b := range allBoards {
		for _, col := range b.Columns {
			for _, tn := range col.TaskNotes {
				tnBoard[tn.Filename] = b
				tnCol[tn.Filename] = col
			}
		}
	}

	m := DetailModel{
		name:            name,
		wsDir:           wsDir,
		project:         project,
		registry:        registry,
		allBoards:       allBoards,
		allTasks:        allTasks,
		allProjectItems: allProjectItems,
		allTags:         allTags,
	}
	for i := range m.collapsedGroups {
		m.collapsedGroups[i] = make(map[string]bool)
	}

	m.projectTasks = make(map[string][]data.Task)

	appendTaskNotes := func(p string, tns []kanbanmodels.TaskNote) {
		for _, tn := range tns {
			if b, ok := tnBoard[tn.Filename]; ok {
				if c, ok2 := tnCol[tn.Filename]; ok2 {
					m.projectTasks[p] = append(m.projectTasks[p], taskview.TaskNoteToTask(tn, b, c))
				}
			}
		}
	}

	if registry != nil {
		m.allDescendants = collectAllDescendants(registry, name)
		m.projectTasks[name] = registry.TasksForProject(name, allTasks)
		appendTaskNotes(name, registry.TaskNotesForProject(name, allBoards))
		for _, desc := range m.allDescendants {
			m.projectTasks[desc.Name] = registry.TasksForProject(desc.Name, allTasks)
			appendTaskNotes(desc.Name, registry.TaskNotesForProject(desc.Name, allBoards))
		}
	} else {
		m.projectTasks[name] = tasks
		appendTaskNotes(name, taskNotes)
	}

	m.rebuildAllColumns()
	return m
}

// collectAllDescendants returns all descendants in depth-first order.
func collectAllDescendants(registry *workspace.ProjectRegistry, rootName string) []*workspace.Project {
	var result []*workspace.Project
	var collect func(name string)
	collect = func(name string) {
		children := registry.ChildrenOf(name)
		sort.Slice(children, func(i, j int) bool {
			return children[i].Name < children[j].Name
		})
		for _, child := range children {
			result = append(result, child)
			collect(child.Name)
		}
	}
	collect(rootName)
	return result
}

func (m *DetailModel) rebuildAllColumns() {
	for col := colKind(0); col < colCount; col++ {
		m.columns[col] = m.buildColumnRows(col)
	}
}

func (m *DetailModel) buildColumnRows(col colKind) []detailRow {
	var rows []detailRow
	if m.project != nil {
		m.appendProjectRows(&rows, m.project, 0, col)
	}
	return rows
}

func (m *DetailModel) appendProjectRows(rows *[]detailRow, p *workspace.Project, depth int, col colKind) {
	if depth > 0 {
		*rows = append(*rows, detailRow{
			kind:        rowKindGroup,
			depth:       depth - 1,
			projectName: p.Name,
		})
		if m.collapsedGroups[col][p.Name] {
			return
		}
	}

	for _, t := range m.projectTasks[p.Name] {
		*rows = append(*rows, detailRow{kind: rowKindTask, depth: depth, projectName: p.Name, task: t})
	}

	if m.registry == nil {
		return
	}
	children := m.registry.ChildrenOf(p.Name)
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name < children[j].Name
	})
	for _, child := range children {
		m.appendProjectRows(rows, child, depth+1, col)
	}
}

// detailProjectNames returns the root project name followed by all physical (non-virtual)
// descendant names in depth-first order. Only physical projects can receive URLs via WriteProjectURLs.
func detailProjectNames(m *DetailModel) []string {
	if m.project == nil {
		return nil
	}
	names := []string{m.name}
	for _, desc := range m.allDescendants {
		if desc.FilePath != "" {
			names = append(names, desc.Name)
		}
	}
	return names
}

// detailExistingSubURLs returns current URLs for all physical sub-projects.
func detailExistingSubURLs(m *DetailModel) map[string][]kanbanmodels.TaskNoteURL {
	result := make(map[string][]kanbanmodels.TaskNoteURL)
	for _, desc := range m.allDescendants {
		if desc.FilePath == "" {
			continue
		}
		if proj := m.registry.Get(desc.Name); proj != nil {
			result[desc.Name] = proj.URLs
		}
	}
	return result
}

// collectAllURLs returns URLs from the root project and all descendants, in depth-first order.
func (m *DetailModel) collectAllURLs() []detailURLEntry {
	var entries []detailURLEntry
	if m.project != nil {
		for _, u := range m.project.URLs {
			entries = append(entries, detailURLEntry{projectName: m.name, url: u})
		}
	}
	for _, desc := range m.allDescendants {
		var proj *workspace.Project
		if m.registry != nil {
			proj = m.registry.Get(desc.Name)
		}
		if proj != nil {
			for _, u := range proj.URLs {
				entries = append(entries, detailURLEntry{projectName: desc.Name, url: u})
			}
		}
	}
	return entries
}

// detailDateEntry is a ProjectDate with its owning project name.
type detailDateEntry struct {
	projectName string
	date        workspace.ProjectDate
}

// collectAllDates returns dates from the root project and all descendants.
func (m *DetailModel) collectAllDates() []detailDateEntry {
	var entries []detailDateEntry
	if m.project != nil {
		for _, d := range m.project.Dates {
			entries = append(entries, detailDateEntry{projectName: m.name, date: d})
		}
	}
	for _, desc := range m.allDescendants {
		var proj *workspace.Project
		if m.registry != nil {
			proj = m.registry.Get(desc.Name)
		}
		if proj != nil {
			for _, d := range proj.Dates {
				entries = append(entries, detailDateEntry{projectName: desc.Name, date: d})
			}
		}
	}
	return entries
}

func (m *DetailModel) currentRow() *detailRow {
	if m.selectedCol < 0 || m.selectedCol >= int(colCount) {
		return nil
	}
	rows := m.columns[m.selectedCol]
	pos := m.colCursorPos[m.selectedCol]
	if pos < 0 || pos >= len(rows) {
		return nil
	}
	return &rows[pos]
}

// SetSize updates the view dimensions.
func (m *DetailModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// OpenInfo returns the project name and workspace dir for re-opening the detail view.
func (m DetailModel) OpenInfo() (name, wsDir string) {
	return m.name, m.wsDir
}

// IsModal returns true when a modal is active.
func (m DetailModel) IsModal() bool {
	return m.mode != detailModeNormal
}

// IsTyping returns true when a text input is focused in an active modal.
func (m DetailModel) IsTyping() bool {
	if m.mode == detailModeURLEditor && m.urlEditor != nil && m.urlEditor.IsTyping() {
		return true
	}
	if m.mode == detailModeDateEditor && m.dateEditor != nil && m.dateEditor.IsTyping() {
		return true
	}
	if m.mode == detailModeNewTaskName {
		return true
	}
	return false
}

func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case detailModeURLEditor:
			return m.updateURLEditor(msg)
		case detailModeURLPicker:
			return m.updateURLPicker(msg)
		case detailModeDateEditor:
			return m.updateDateEditor(msg)
		case detailModeSubProjectPick:
			return m.updateSubProjectPicker(msg)
		case detailModeNewTaskName:
			return m.updateNewTaskInput(msg)
		case detailModeNewTaskEditor:
			return m.updateNewTaskEditor(msg)
		case detailModeNewBoardPick:
			return m.updateNewBoardPick(msg)
		case detailModeChildPicker:
			return m.updateChildPicker(msg)
		}
		return m.handleKey(msg)
	case noteEditorFinishedMsg:
		return m, func() tea.Msg { return messages.DataRefreshMsg{} }
	case cardEditorFinishedMsg:
		return m, func() tea.Msg { return messages.DataRefreshMsg{} }
	case taskview.TextInputResultMsg:
		return m.handleTextInputResult(msg)
	case taskview.TaskEditorResultMsg:
		return m.handleTaskEditorResult(msg)
	}
	return m, nil
}

func (m DetailModel) handleKey(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	switch msg.String() {
	case "n":
		return m.handleNew()

	case "o":
		if m.project != nil && m.project.FilePath != "" {
			return m, openNoteInEditor(m.project.FilePath)
		}

	case "esc", "q":
		return m, func() tea.Msg { return messages.FocusSidebarMsg{} }

	case "h", "left":
		if m.selectedCol > 0 {
			m.selectedCol--
			m.adjustHorizScroll()
		}

	case "l", "right":
		if m.selectedCol < int(colCount)-1 {
			m.selectedCol++
			m.adjustHorizScroll()
		}

	case "j", "down":
		rows := m.columns[m.selectedCol]
		if m.colCursorPos[m.selectedCol] < len(rows)-1 {
			m.colCursorPos[m.selectedCol]++
			m.adjustScrollPosition()
		}

	case "k", "up":
		if m.colCursorPos[m.selectedCol] > 0 {
			m.colCursorPos[m.selectedCol]--
			m.adjustScrollPosition()
		}

	case "tab", " ":
		row := m.currentRow()
		if row != nil && row.kind == rowKindGroup {
			projName := row.projectName
			col := m.selectedCol
			m.collapsedGroups[col][projName] = !m.collapsedGroups[col][projName]
			m.columns[col] = m.buildColumnRows(colKind(col))
			m.restoreCursorToGroup(projName)
		}

	case "u":
		entries := m.collectAllURLs()
		if len(entries) == 1 {
			url := entries[0].url.URL
			return m, func() tea.Msg {
				_ = operations.OpenURL(url)
				return nil
			}
		} else if len(entries) > 1 {
			p := projectURLPicker{entries: entries, width: m.width, height: m.height}
			m.urlPicker = &p
			m.mode = detailModeURLPicker
		} else if m.project != nil {
			// No URLs anywhere — open editor for root project
			editor := kanban.NewURLEditorModelWithProjects(m.project.URLs, detailProjectNames(&m), detailExistingSubURLs(&m))
			editor.SetSize(m.width, m.height)
			m.urlEditor = &editor
			m.mode = detailModeURLEditor
		}

	case "U":
		if m.project != nil {
			editor := kanban.NewURLEditorModelWithProjects(m.project.URLs, detailProjectNames(&m), detailExistingSubURLs(&m))
			editor.SetSize(m.width, m.height)
			m.urlEditor = &editor
			m.mode = detailModeURLEditor
		}

	case "d":
		if m.project != nil {
			projectNames := []string{m.name}
			existingSubDates := make(map[string][]workspace.ProjectDate)
			for _, desc := range m.allDescendants {
				if desc.FilePath == "" {
					continue
				}
				projectNames = append(projectNames, desc.Name)
				if m.registry != nil {
					if proj := m.registry.Get(desc.Name); proj != nil {
						existingSubDates[desc.Name] = proj.Dates
					}
				}
			}
			editor := NewDateEditorModelWithProjects(m.project.Dates, projectNames, existingSubDates)
			editor.SetSize(m.width, m.height)
			m.dateEditor = &editor
			m.mode = detailModeDateEditor
		}

	case "[":
		if m.project == nil || m.project.Parent == "" {
			return m, nil
		}
		if m.registry == nil {
			return m, nil
		}
		parent := m.registry.Get(m.project.Parent)
		if parent == nil {
			return m, nil
		}
		wsDir := m.wsDir
		parentName := parent.Name
		return m, func() tea.Msg {
			return messages.OpenProjectMsg{
				ProjectName:      parentName,
				WorkspaceRootDir: wsDir,
			}
		}

	case "]":
		if m.registry == nil {
			return m, nil
		}
		children := m.registry.ChildrenOf(m.name)
		if len(children) == 0 {
			return m, nil
		}
		sort.Slice(children, func(i, j int) bool {
			return strings.ToLower(children[i].Name) < strings.ToLower(children[j].Name)
		})
		picker := detailChildPicker{
			entries: children,
			cursor:  0,
			width:   m.width,
			height:  m.height,
		}
		m.childPicker = &picker
		m.mode = detailModeChildPicker
		return m, nil

	case "enter":
		row := m.currentRow()
		if row == nil {
			return m, nil
		}
		if row.kind == rowKindGroup {
			projName := row.projectName
			col := m.selectedCol
			m.collapsedGroups[col][projName] = !m.collapsedGroups[col][projName]
			m.columns[col] = m.buildColumnRows(colKind(col))
			m.restoreCursorToGroup(projName)
			return m, nil
		}
		switch row.kind {
		case rowKindTask:
			task := row.task
			if task.IsTaskNote {
				return m, func() tea.Msg { return messages.OpenBoardMsg{BoardPath: task.File} }
			}
			return m, func() tea.Msg { return messages.FocusTaskMsg{TaskID: task.ID} }
		}
	}
	return m, nil
}

// restoreCursorToGroup finds the group header for projName in the focused column
// and sets the cursor there.
func (m *DetailModel) restoreCursorToGroup(projName string) {
	col := m.selectedCol
	for i, row := range m.columns[col] {
		if row.kind == rowKindGroup && row.projectName == projName {
			m.colCursorPos[col] = i
			m.adjustScrollPosition()
			return
		}
	}
}

func (m DetailModel) updateURLEditor(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	if m.urlEditor == nil {
		m.mode = detailModeNormal
		return m, nil
	}
	editor, cmd, saved, done := m.urlEditor.Update(msg)
	m.urlEditor = &editor
	if done {
		if saved && m.project != nil {
			_ = workspace.WriteProjectURLs(m.project, m.urlEditor.GetURLs())
			for projName, urls := range m.urlEditor.GetSubProjectURLs() {
				if proj := m.registry.Get(projName); proj != nil {
					_ = workspace.WriteProjectURLs(proj, urls)
				}
			}
		}
		m.urlEditor = nil
		m.mode = detailModeNormal
		if saved {
			return m, func() tea.Msg { return messages.DataRefreshMsg{} }
		}
	}
	return m, cmd
}

func (m DetailModel) updateURLPicker(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	if m.urlPicker == nil {
		m.mode = detailModeNormal
		return m, nil
	}
	picker, selectedURL, done := m.urlPicker.Update(msg)
	m.urlPicker = &picker
	if done {
		m.urlPicker = nil
		m.mode = detailModeNormal
		if selectedURL != "" {
			url := selectedURL
			return m, func() tea.Msg {
				_ = operations.OpenURL(url)
				return nil
			}
		}
	}
	return m, nil // no cmd needed, projectURLPicker is synchronous
}

func (m DetailModel) updateDateEditor(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	if m.dateEditor == nil {
		m.mode = detailModeNormal
		return m, nil
	}
	editor, cmd, saved, done := m.dateEditor.Update(msg)
	m.dateEditor = &editor
	if done {
		if saved && m.project != nil {
			_ = workspace.WriteProjectDates(m.project, m.dateEditor.GetDates())
			for projName, dates := range m.dateEditor.GetSubProjectDates() {
				if m.registry != nil {
					if proj := m.registry.Get(projName); proj != nil {
						_ = workspace.WriteProjectDates(proj, dates)
					}
				}
			}
		}
		m.dateEditor = nil
		m.mode = detailModeNormal
		if saved {
			return m, func() tea.Msg { return messages.DataRefreshMsg{} }
		}
	}
	return m, cmd
}

func (m DetailModel) updateChildPicker(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	if m.childPicker == nil {
		m.mode = detailModeNormal
		return m, nil
	}
	picker, selected, done := m.childPicker.Update(msg)
	m.childPicker = &picker
	if done {
		m.mode = detailModeNormal
		m.childPicker = nil
		if selected != nil {
			wsDir := m.wsDir
			return m, func() tea.Msg {
				return messages.OpenProjectMsg{
					ProjectName:      selected.Name,
					WorkspaceRootDir: wsDir,
				}
			}
		}
	}
	return m, nil
}

func (m DetailModel) View() string {
	if m.mode == detailModeURLEditor && m.urlEditor != nil {
		return m.urlEditor.View()
	}
	if m.mode == detailModeURLPicker && m.urlPicker != nil {
		return m.urlPicker.View()
	}
	if m.mode == detailModeDateEditor && m.dateEditor != nil {
		return m.dateEditor.View()
	}
	if m.mode == detailModeSubProjectPick && m.createSubProjectPicker != nil {
		return m.createSubProjectPicker.View()
	}
	if m.mode == detailModeNewTaskName && m.createTaskInput != nil {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.createTaskInput.View())
	}
	if m.mode == detailModeNewTaskEditor && m.createTaskEditor != nil {
		return m.createTaskEditor.View()
	}
	if m.mode == detailModeNewBoardPick && m.createBoardPicker != nil {
		return m.createBoardPicker.View()
	}
	if m.mode == detailModeChildPicker && m.childPicker != nil {
		return m.childPicker.View()
	}

	var lines []string

	lines = append(lines, titleStyle.Render(fmt.Sprintf("Project: %s", m.name)))
	lines = append(lines, "")

	// Build URL lines (left half of header row)
	var urlLines []string
	if allURLs := m.collectAllURLs(); len(allURLs) > 0 {
		lastProject := ""
		for _, e := range allURLs {
			if e.projectName != m.name && e.projectName != lastProject {
				urlLines = append(urlLines, sectionHeaderStyle.Render("  "+e.projectName))
			}
			lastProject = e.projectName
			u := e.url
			urlStr := u.URL
			const maxURLLen = 60
			if len(urlStr) > maxURLLen {
				urlStr = urlStr[:maxURLLen-3] + "..."
			}
			if u.Label == "" {
				urlLines = append(urlLines, pathStyle.Render("    "+urlStr))
			} else {
				urlLines = append(urlLines, urlLabelStyle.Render("    "+u.Label)+pathStyle.Render("  "+urlStr))
			}
		}
	}

	// Build date lines (right half of header row)
	var dateLines []string
	if allDates := m.collectAllDates(); len(allDates) > 0 {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		lastProject := ""
		for _, e := range allDates {
			if e.projectName != m.name && e.projectName != lastProject {
				dateLines = append(dateLines, sectionHeaderStyle.Render("  "+e.projectName))
			}
			lastProject = e.projectName
			d := e.date
			dateDay := time.Date(d.Date.Year(), d.Date.Month(), d.Date.Day(), 0, 0, 0, 0, time.Local)
			dateStr := d.Date.Format("Jan 2 2006")
			label := d.Label
			if label == "" {
				label = "date"
			}
			if dateDay.Before(today) {
				dateLines = append(dateLines, pathStyle.Render("    "+label+"  "+dateStr))
			} else {
				dateLines = append(dateLines, upcomingDateStyle.Render("    "+label)+"  "+upcomingDateValueStyle.Render(dateStr))
			}
		}
	}

	// Render URLs (left) and dates (right) as a side-by-side row
	sideBlockHeight := 0
	if len(urlLines) > 0 || len(dateLines) > 0 {
		halfWidth := m.width / 2
		leftBlock := lipgloss.NewStyle().Width(halfWidth).Render(
			lipgloss.JoinVertical(lipgloss.Left, urlLines...))
		rightBlock := lipgloss.NewStyle().Width(m.width - halfWidth).Render(
			lipgloss.JoinVertical(lipgloss.Left, dateLines...))
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, rightBlock))
		lines = append(lines, "")
		sideBlockHeight = len(urlLines)
		if len(dateLines) > sideBlockHeight {
			sideBlockHeight = len(dateLines)
		}
	}

	// len(lines) undercounts by (sideBlockHeight - 1) because the combined block is
	// one string in the slice but sideBlockHeight terminal rows tall.
	headerLines := len(lines)
	if sideBlockHeight > 0 {
		headerLines += sideBlockHeight - 1
	}
	fixedColHeight := m.height - headerLines - 2
	if fixedColHeight < 5 {
		fixedColHeight = 5
	}

	startCol, endCol, colWidth := m.calculateVisibleColumns()

	var colViews []string
	for i := startCol; i < endCol; i++ {
		colViews = append(colViews, m.renderColumn(i, fixedColHeight, colWidth))
	}

	colArea := lipgloss.JoinHorizontal(lipgloss.Top, colViews...)
	lines = append(lines, colArea)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, content)
}

func (m DetailModel) renderColumn(colIdx int, fixedHeight int, colWidth int) string {
	col := colKind(colIdx)
	rows := m.columns[colIdx]
	focused := colIdx == m.selectedCol

	var s strings.Builder

	// availableForRows = fixedHeight - topIndicator(1) - bottomIndicator(1)
	availableForRows := fixedHeight - 2
	if availableForRows < 1 {
		availableForRows = 1
	}

	scrollOff := m.colScrollOff[colIdx]
	cursor := m.colCursorPos[colIdx]

	// Compute boardNameWidth for task-note alignment in the Tasks column.
	boardNameWidth := 0
	if col == colTasks {
		for _, r := range rows {
			if r.kind == rowKindTask && r.task.IsTaskNote && len(r.task.BoardName) > boardNameWidth {
				boardNameWidth = len(r.task.BoardName)
			}
		}
	}

	// Top indicator (always reserve 1 line)
	if scrollOff > 0 {
		s.WriteString(pathStyle.Render(fmt.Sprintf("  ▲ %d above", scrollOff)))
	}
	s.WriteString("\n")

	if len(rows) == 0 {
		s.WriteString(pathStyle.Render("  (none)"))
		s.WriteString("\n")
	} else {
		end := scrollOff + availableForRows
		if end > len(rows) {
			end = len(rows)
		}
		for i := scrollOff; i < end; i++ {
			s.WriteString(m.renderRow(rows[i], i == cursor && focused, col, colWidth, boardNameWidth))
			s.WriteString("\n")
		}
	}

	// Bottom indicator (always reserve 1 line)
	end := scrollOff + availableForRows
	if end > len(rows) {
		end = len(rows)
	}
	if remaining := len(rows) - end; remaining > 0 {
		s.WriteString(pathStyle.Render(fmt.Sprintf("  ▼ %d below", remaining)))
	}
	s.WriteString("\n")

	return lipgloss.NewStyle().Width(colWidth).Height(fixedHeight).Render(s.String())
}

func (m DetailModel) renderRow(row detailRow, isSelected bool, col colKind, colWidth int, boardNameWidth int) string {
	indent := strings.Repeat("  ", row.depth)

	switch row.kind {
	case rowKindGroup:
		expanded := !m.collapsedGroups[col][row.projectName]
		marker := "▶"
		if expanded {
			marker = "▼"
		}
		done := m.subtreeDoneCount(row.projectName, col)
		total := m.subtreeCount(row.projectName, col)
		countStr := fmt.Sprintf("%d/%d", done, total)
		content := fmt.Sprintf("%s%s %s (%s)", indent, marker, row.projectName, countStr)
		if isSelected {
			rendered := shared.ApplyRowBackground(colItemSelectedStyle.Render(content), colWidth)
			return rendered
		}
		return xansi.Truncate(childProjectStyle.Render(content), colWidth, "")

	case rowKindTask:
		prefix := "  "
		if isSelected {
			prefix = theme.Cursor.Render("> ")
		}
		prefixWidth := lipgloss.Width(prefix)
		line := prefix + shared.StyledTaskLine(row.task, colWidth-prefixWidth, boardNameWidth)
		if isSelected {
			line = shared.ApplyRowBackground(line, colWidth)
		}
		return line

	default:
		return ""
	}
}

// subtreeCount counts total items in the subtree of projName for the given column kind.
func (m *DetailModel) subtreeCount(projName string, col colKind) int {
	if m.registry == nil {
		return len(m.projectTasks[projName])
	}
	var count func(name string) int
	count = func(name string) int {
		n := len(m.projectTasks[name])
		for _, child := range m.registry.ChildrenOf(name) {
			n += count(child.Name)
		}
		return n
	}
	return count(projName)
}


// subtreeDoneCount counts done items in the subtree of projName for the given column kind.
func (m *DetailModel) subtreeDoneCount(projName string, col colKind) int {
	var count func(name string) int
	count = func(name string) int {
		var n int
		if col == colTasks {
			for _, t := range m.projectTasks[name] {
				if t.Done {
					n++
				}
			}
		}
		if m.registry != nil {
			for _, child := range m.registry.ChildrenOf(name) {
				n += count(child.Name)
			}
		}
		return n
	}
	return count(projName)
}


func (m *DetailModel) adjustScrollPosition() {
	col := m.selectedCol
	rows := m.columns[col]
	if len(rows) == 0 {
		return
	}
	cursor := m.colCursorPos[col]
	visibleRows := m.height - 7
	if visibleRows < 3 {
		visibleRows = 3
	}
	if cursor < m.colScrollOff[col] {
		m.colScrollOff[col] = cursor
	} else if cursor >= m.colScrollOff[col]+visibleRows {
		m.colScrollOff[col] = cursor - visibleRows + 1
		if m.colScrollOff[col] < 0 {
			m.colScrollOff[col] = 0
		}
	}
}

// calculateVisibleColumns returns the start/end column indices and the width
// each column should occupy so they fill the available screen width evenly.
func (m DetailModel) calculateVisibleColumns() (start, end, colWidth int) {
	const minColWidth = 20
	availableWidth := m.width
	if availableWidth < minColWidth {
		availableWidth = minColWidth
	}
	visibleCount := int(colCount) // try to show all columns
	// Shrink visible count until each column is at least minColWidth wide
	for visibleCount > 1 && availableWidth/visibleCount < minColWidth {
		visibleCount--
	}
	colWidth = availableWidth / visibleCount

	start = m.colHorizOffset
	end = start + visibleCount
	if end > int(colCount) {
		end = int(colCount)
	}
	// Recalculate: if fewer columns remain than visibleCount, spread them wider
	actual := end - start
	if actual > 0 {
		colWidth = availableWidth / actual
	}
	return start, end, colWidth
}

func (m *DetailModel) adjustHorizScroll() {
	startCol, endCol, _ := m.calculateVisibleColumns()
	if m.selectedCol < startCol {
		m.colHorizOffset = m.selectedCol
		return
	}
	if m.selectedCol >= endCol {
		visibleCount := endCol - startCol
		m.colHorizOffset = m.selectedCol - visibleCount + 1
		if m.colHorizOffset < 0 {
			m.colHorizOffset = 0
		}
	}
}
