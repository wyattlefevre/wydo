package projects

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/notes"
	"wydo/internal/tasks/data"
	"wydo/internal/tui/messages"
	"wydo/internal/tui/shared"
	"wydo/internal/workspace"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const detailColWidth = 38

type colKind int

const (
	colNotes  colKind = iota
	colTasks
	colCards
	colBoards
	colDates
	colCount
)

var colNames = [colCount]string{"Notes", "Tasks", "Cards", "Boards", "Dates"}

type rowKind int

const (
	rowKindGroup rowKind = iota
	rowKindNote
	rowKindTask
	rowKindCard
	rowKindBoard
	rowKindDate
)

type detailRow struct {
	kind        rowKind
	depth       int
	projectName string

	// Only one populated based on kind:
	note    notes.Note
	task    data.Task
	card    kanbanmodels.Card
	board   kanbanmodels.Board
	date    workspace.ProjectDate
	dateIdx int // index in project.Dates
}

type detailEditMode int

const (
	detailModeNormal        detailEditMode = iota
	detailModeDateLabel                    // editing label text input
	detailModeDatePicker                   // shared date picker
	detailModeCreateSubName                // typing new sub-project name
)

// DetailModel shows project details with notes, tasks, cards, boards, and dates
// in a kanban-style column layout with hierarchical grouping by child project.
type DetailModel struct {
	name         string
	wsDir        string
	project      *workspace.Project
	registry     *workspace.ProjectRegistry
	indexPreview string
	width, height int

	// Pre-computed per-project data (keyed by project name)
	projectNotes  map[string][]notes.Note
	projectTasks  map[string][]data.Task
	projectCards  map[string][]kanbanmodels.Card
	projectBoards map[string][]kanbanmodels.Board
	allDescendants []*workspace.Project

	// Raw all-data
	allBoards []kanbanmodels.Board
	allTasks  []data.Task
	allNotes  []notes.Note
	cardBoard map[string]kanbanmodels.Board // card filename → parent board

	// Column state
	columns        [colCount][]detailRow
	selectedCol    int
	colScrollOff   [colCount]int
	colCursorPos   [colCount]int
	colHorizOffset int // first visible column index (horizontal scroll)

	// Collapse state: project name → collapsed
	collapsedGroups map[string]bool

	// Date editing state
	editMode       detailEditMode
	editingDateIdx int
	editingLabel   string
	editingProject string // which project's dates are being edited
	labelInput     textinput.Model
	datePicker     *shared.DatePickerModel

	// Sub-project creation
	subNameInput textinput.Model
}

func NewDetailModel(name, wsDir string, n []notes.Note, tasks []data.Task, cards []kanbanmodels.Card, boards []kanbanmodels.Board, allBoards []kanbanmodels.Board, project *workspace.Project, registry *workspace.ProjectRegistry, children []*workspace.Project, indexPreview string, allTasks []data.Task, allNotes []notes.Note) DetailModel {
	cardBoard := make(map[string]kanbanmodels.Board)
	for _, b := range allBoards {
		for _, col := range b.Columns {
			for _, c := range col.Cards {
				cardBoard[c.Filename] = b
			}
		}
	}

	ti := textinput.New()
	ti.Placeholder = "Date label..."
	ti.CharLimit = 80
	ti.Width = 40

	si := textinput.New()
	si.Placeholder = "sub-project name"
	si.CharLimit = 60
	si.Width = 40

	m := DetailModel{
		name:            name,
		wsDir:           wsDir,
		project:         project,
		registry:        registry,
		indexPreview:    indexPreview,
		allBoards:       allBoards,
		allTasks:        allTasks,
		allNotes:        allNotes,
		cardBoard:       cardBoard,
		collapsedGroups: make(map[string]bool),
		editingDateIdx:  -1,
		labelInput:      ti,
		subNameInput:    si,
	}

	m.projectNotes = make(map[string][]notes.Note)
	m.projectTasks = make(map[string][]data.Task)
	m.projectCards = make(map[string][]kanbanmodels.Card)
	m.projectBoards = make(map[string][]kanbanmodels.Board)

	if registry != nil {
		m.allDescendants = collectAllDescendants(registry, name)
		// Root project
		m.projectNotes[name] = registry.NotesForProject(name, allNotes)
		m.projectTasks[name] = registry.TasksForProject(name, allTasks)
		m.projectCards[name] = registry.CardsForProject(name, allBoards)
		m.projectBoards[name] = registry.BoardsForProject(name, allBoards)
		// All descendants
		for _, desc := range m.allDescendants {
			m.projectNotes[desc.Name] = registry.NotesForProject(desc.Name, allNotes)
			m.projectTasks[desc.Name] = registry.TasksForProject(desc.Name, allTasks)
			m.projectCards[desc.Name] = registry.CardsForProject(desc.Name, allBoards)
			m.projectBoards[desc.Name] = registry.BoardsForProject(desc.Name, allBoards)
		}
	} else {
		// Fallback: use directly passed data
		m.projectNotes[name] = n
		m.projectTasks[name] = tasks
		m.projectCards[name] = cards
		m.projectBoards[name] = boards
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
		if m.collapsedGroups[p.Name] {
			return
		}
	}

	// Append items for this project at this depth
	switch col {
	case colNotes:
		for _, n := range m.projectNotes[p.Name] {
			*rows = append(*rows, detailRow{kind: rowKindNote, depth: depth, projectName: p.Name, note: n})
		}
	case colTasks:
		for _, t := range m.projectTasks[p.Name] {
			*rows = append(*rows, detailRow{kind: rowKindTask, depth: depth, projectName: p.Name, task: t})
		}
	case colCards:
		for _, c := range m.projectCards[p.Name] {
			*rows = append(*rows, detailRow{kind: rowKindCard, depth: depth, projectName: p.Name, card: c})
		}
	case colBoards:
		for _, b := range m.projectBoards[p.Name] {
			*rows = append(*rows, detailRow{kind: rowKindBoard, depth: depth, projectName: p.Name, board: b})
		}
	case colDates:
		var proj *workspace.Project
		if m.registry != nil {
			proj = m.registry.Get(p.Name)
		}
		if proj == nil && p.Name == m.name {
			proj = m.project
		}
		if proj != nil {
			for i, d := range proj.Dates {
				*rows = append(*rows, detailRow{kind: rowKindDate, depth: depth, projectName: p.Name, date: d, dateIdx: i})
			}
		}
	}

	// Recurse into sorted children
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

// IsModal returns true when the detail view has an active modal or text input.
func (m DetailModel) IsModal() bool {
	return m.editMode != detailModeNormal
}

// IsTyping returns true when the detail view has an active text input.
func (m DetailModel) IsTyping() bool {
	return m.editMode == detailModeDateLabel || m.editMode == detailModeCreateSubName
}

// HintText returns the raw hint string for the detail view.
func (m DetailModel) HintText() string {
	switch m.editMode {
	case detailModeDateLabel:
		return "type label  enter:next  esc:cancel"
	case detailModeDatePicker:
		return "h/l/j/k:navigate  t:today  enter:confirm  c:clear  i:text input  esc:cancel"
	case detailModeCreateSubName:
		return "type name  enter:create  esc:cancel"
	}
	base := "h/l:columns  j/k:navigate  space/enter:expand  enter:open  esc:back"
	if m.selectedCol == int(colDates) {
		return base + "  n:new  e:edit  d:delete"
	}
	return base
}

func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.editMode == detailModeDateLabel {
			return m.handleDateLabelKey(msg)
		}
		if m.editMode == detailModeDatePicker {
			return m.handleDatePickerKey(msg)
		}
		if m.editMode == detailModeCreateSubName {
			return m.handleCreateSubNameKey(msg)
		}
		return m.handleKey(msg)
	}
	return m, nil
}

func (m DetailModel) handleCreateSubNameKey(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(m.subNameInput.Value())
		if name != "" {
			m.editMode = detailModeNormal
			m.subNameInput.Blur()
			proj := m.project
			wsDir := m.wsDir
			return m, func() tea.Msg {
				return messages.CreateSubProjectMsg{
					ParentProject: proj,
					Name:          name,
					WsDir:         wsDir,
				}
			}
		}
		return m, nil
	case "esc":
		m.editMode = detailModeNormal
		m.subNameInput.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.subNameInput, cmd = m.subNameInput.Update(msg)
	return m, cmd
}

func (m DetailModel) handleDateLabelKey(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.editingLabel = m.labelInput.Value()
		var cur *time.Time
		if m.editingDateIdx >= 0 {
			var proj *workspace.Project
			if m.editingProject != "" && m.registry != nil {
				proj = m.registry.Get(m.editingProject)
			}
			if proj == nil {
				proj = m.project
			}
			if proj != nil && m.editingDateIdx < len(proj.Dates) {
				t := proj.Dates[m.editingDateIdx].Date
				cur = &t
			}
		}
		dp := shared.NewDatePickerModel(cur, "Milestone Date")
		m.datePicker = &dp
		m.editMode = detailModeDatePicker
		return m, nil
	case "esc":
		m.editMode = detailModeNormal
		m.labelInput.SetValue("")
		return m, nil
	}
	var cmd tea.Cmd
	m.labelInput, cmd = m.labelInput.Update(msg)
	return m, cmd
}

func (m DetailModel) handleDatePickerKey(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	if m.datePicker == nil {
		m.editMode = detailModeNormal
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.editMode = detailModeNormal
		m.datePicker = nil
		return m, nil
	case "enter":
		if picked := m.datePicker.GetDate(); picked != nil {
			var proj *workspace.Project
			if m.editingProject != "" && m.registry != nil {
				proj = m.registry.Get(m.editingProject)
			}
			if proj == nil {
				proj = m.project
			}
			if proj != nil {
				newDate := workspace.ProjectDate{
					Label: m.editingLabel,
					Date:  *picked,
				}
				dates := make([]workspace.ProjectDate, len(proj.Dates))
				copy(dates, proj.Dates)
				if m.editingDateIdx == -1 {
					dates = append(dates, newDate)
				} else if m.editingDateIdx < len(dates) {
					dates[m.editingDateIdx] = newDate
				}
				_ = workspace.WriteProjectDates(proj, dates)
			}
		}
		m.editMode = detailModeNormal
		m.datePicker = nil
		return m, func() tea.Msg { return messages.DataRefreshMsg{} }
	}
	dp, cmd := m.datePicker.Update(msg)
	m.datePicker = &dp
	return m, cmd
}

func (m DetailModel) handleKey(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return m, messages.SwitchView(messages.ViewProjects)

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
			m.collapsedGroups[projName] = !m.collapsedGroups[projName]
			m.rebuildAllColumns()
			m.restoreCursorToGroup(projName)
		}

	case "enter":
		row := m.currentRow()
		if row == nil {
			return m, nil
		}
		if row.kind == rowKindGroup {
			projName := row.projectName
			m.collapsedGroups[projName] = !m.collapsedGroups[projName]
			m.rebuildAllColumns()
			m.restoreCursorToGroup(projName)
			return m, nil
		}
		switch row.kind {
		case rowKindTask:
			task := row.task
			return m, func() tea.Msg {
				return messages.FocusTaskMsg{TaskID: task.ID}
			}
		case rowKindBoard:
			board := row.board
			return m, func() tea.Msg {
				return messages.OpenBoardMsg{BoardPath: board.Path}
			}
		case rowKindCard:
			if b, ok := m.cardBoard[row.card.Filename]; ok {
				return m, func() tea.Msg {
					return messages.OpenBoardMsg{BoardPath: b.Path}
				}
			}
		case rowKindDate:
			return m.startDateEdit(row.dateIdx, row.projectName)
		}

	case "n":
		if m.selectedCol == int(colDates) {
			return m.startDateEdit(-1, m.name)
		}

	case "e":
		if m.selectedCol == int(colDates) {
			row := m.currentRow()
			if row != nil && row.kind == rowKindDate {
				return m.startDateEdit(row.dateIdx, row.projectName)
			}
		}

	case "d":
		if m.selectedCol == int(colDates) {
			row := m.currentRow()
			if row != nil && row.kind == rowKindDate {
				var proj *workspace.Project
				if m.registry != nil {
					proj = m.registry.Get(row.projectName)
				}
				if proj == nil {
					proj = m.project
				}
				if proj != nil {
					dates := make([]workspace.ProjectDate, 0, len(proj.Dates)-1)
					for i, d := range proj.Dates {
						if i != row.dateIdx {
							dates = append(dates, d)
						}
					}
					_ = workspace.WriteProjectDates(proj, dates)
				}
				// Clamp cursor
				col := m.selectedCol
				if m.colCursorPos[col] > 0 && m.colCursorPos[col] >= len(m.columns[col])-1 {
					m.colCursorPos[col]--
				}
				return m, func() tea.Msg { return messages.DataRefreshMsg{} }
			}
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

func (m DetailModel) startDateEdit(idx int, projName string) (DetailModel, tea.Cmd) {
	m.editingDateIdx = idx
	m.editingProject = projName
	m.editMode = detailModeDateLabel
	var proj *workspace.Project
	if m.registry != nil {
		proj = m.registry.Get(projName)
	}
	if proj == nil {
		proj = m.project
	}
	if idx >= 0 && proj != nil && idx < len(proj.Dates) {
		m.editingLabel = proj.Dates[idx].Label
		m.labelInput.SetValue(proj.Dates[idx].Label)
	} else {
		m.editingLabel = ""
		m.labelInput.SetValue("")
	}
	m.labelInput.Focus()
	return m, textinput.Blink
}

func (m DetailModel) View() string {
	if m.editMode == detailModeDateLabel {
		return m.viewDateLabelInput()
	}
	if m.editMode == detailModeDatePicker && m.datePicker != nil {
		return m.viewDatePicker()
	}

	var lines []string

	// Title
	lines = append(lines, titleStyle.Render(fmt.Sprintf("Project: %s", m.name)))
	lines = append(lines, "")

	// Index preview
	if m.indexPreview != "" {
		for _, line := range strings.Split(m.indexPreview, "\n") {
			lines = append(lines, pathStyle.Render("  "+line))
		}
		lines = append(lines, "")
	}

	headerLines := len(lines)
	fixedColHeight := m.height - headerLines - 2
	if fixedColHeight < 5 {
		fixedColHeight = 5
	}

	startCol, endCol := m.calculateVisibleColumns()

	var colViews []string

	// Left scroll indicator
	if startCol > 0 {
		colViews = append(colViews, m.renderHorizIndicator("◀", fixedColHeight))
	} else {
		colViews = append(colViews, m.renderHorizIndicator(" ", fixedColHeight))
	}

	for i := startCol; i < endCol; i++ {
		colViews = append(colViews, m.renderColumn(i, fixedColHeight))
	}

	// Right scroll indicator
	if endCol < int(colCount) {
		colViews = append(colViews, m.renderHorizIndicator("▶", fixedColHeight))
	} else {
		colViews = append(colViews, m.renderHorizIndicator(" ", fixedColHeight))
	}

	colArea := lipgloss.JoinHorizontal(lipgloss.Top, colViews...)
	lines = append(lines, colArea)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, content)
}

func (m DetailModel) viewDateLabelInput() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Date Label"))
	lines = append(lines, "")
	lines = append(lines, "  "+m.labelInput.View())
	lines = append(lines, "")
	lines = append(lines, pathStyle.Render("  Press enter to continue to date picker, esc to cancel"))
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m DetailModel) viewDatePicker() string {
	return m.datePicker.View()
}

func (m DetailModel) renderColumn(colIdx int, fixedHeight int) string {
	col := colKind(colIdx)
	rows := m.columns[colIdx]
	focused := colIdx == m.selectedCol

	var s strings.Builder

	// Title line
	count := m.totalColCount(col)
	title := fmt.Sprintf("%s (%d)", colNames[col], count)
	if focused {
		s.WriteString(sectionActiveStyle.Render(title))
	} else {
		s.WriteString(sectionHeaderStyle.Render(title))
	}
	s.WriteString("\n")

	// Available rows = fixedHeight - title(1) - topIndicator(1) - bottomIndicator(1)
	availableForRows := fixedHeight - 3
	if availableForRows < 1 {
		availableForRows = 1
	}

	scrollOff := m.colScrollOff[colIdx]
	cursor := m.colCursorPos[colIdx]

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
			s.WriteString(m.renderRow(rows[i], i == cursor && focused, col))
			s.WriteString("\n")
		}
	}

	// Bottom indicator (always reserve 1 line)
	end := scrollOff + availableForRows
	if end > len(rows) {
		end = len(rows)
	}
	remaining := len(rows) - end
	if remaining > 0 {
		s.WriteString(pathStyle.Render(fmt.Sprintf("  ▼ %d below", remaining)))
	}
	s.WriteString("\n")

	style := lipgloss.NewStyle().
		Width(detailColWidth + 2).
		Height(fixedHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8"))
	if focused {
		style = style.BorderForeground(lipgloss.Color("3"))
	}
	return style.Render(s.String())
}

func (m DetailModel) renderRow(row detailRow, isSelected bool, col colKind) string {
	indent := strings.Repeat("  ", row.depth)
	prefix := "  "
	if isSelected {
		prefix = "► "
	}

	switch row.kind {
	case rowKindGroup:
		expanded := !m.collapsedGroups[row.projectName]
		marker := "▶"
		if expanded {
			marker = "▼"
		}
		count := m.subtreeCount(row.projectName, col)
		content := fmt.Sprintf("%s%s%s %s (%d)", indent, prefix, marker, row.projectName, count)
		if isSelected {
			return selectedDetailItemStyle.Render(content)
		}
		return sectionHeaderStyle.Render(content)

	case rowKindNote:
		display := row.note.Title
		if display == "" {
			display = filepath.Base(row.note.FilePath)
		}
		content := indent + prefix + display
		if isSelected {
			return selectedDetailItemStyle.Render(content)
		}
		return detailItemStyle.Render(content)

	case rowKindTask:
		taskLine := shared.StyledTaskLine(row.task)
		if isSelected {
			return selectedDetailItemStyle.Render(indent+prefix) + taskLine
		}
		return detailItemStyle.Render(indent+prefix) + taskLine

	case rowKindCard:
		title := row.card.Title
		if title == "" {
			title = row.card.Filename
		}
		content := indent + prefix + title
		if isSelected {
			return selectedDetailItemStyle.Render(content)
		}
		return detailItemStyle.Render(content)

	case rowKindBoard:
		title := row.board.Name
		if title == "" {
			title = filepath.Base(row.board.Path)
		}
		content := indent + prefix + title
		if isSelected {
			return selectedDetailItemStyle.Render(content)
		}
		return detailItemStyle.Render(content)

	case rowKindDate:
		label := row.date.Label
		if label == "" {
			label = "(no label)"
		}
		dateStr := row.date.Date.Format("Jan 2, 2006")
		content := indent + prefix + label + " — " + dateStr
		if isSelected {
			return selectedDetailItemStyle.Render(content)
		}
		return detailItemStyle.Render(content)
	}

	return ""
}

// subtreeCount counts total items in the subtree of projName for the given column kind.
func (m *DetailModel) subtreeCount(projName string, col colKind) int {
	if m.registry == nil {
		switch col {
		case colNotes:
			return len(m.projectNotes[projName])
		case colTasks:
			return len(m.projectTasks[projName])
		case colCards:
			return len(m.projectCards[projName])
		case colBoards:
			return len(m.projectBoards[projName])
		case colDates:
			if m.project != nil {
				return len(m.project.Dates)
			}
			return 0
		}
		return 0
	}
	var count func(name string) int
	count = func(name string) int {
		var n int
		switch col {
		case colNotes:
			n = len(m.projectNotes[name])
		case colTasks:
			n = len(m.projectTasks[name])
		case colCards:
			n = len(m.projectCards[name])
		case colBoards:
			n = len(m.projectBoards[name])
		case colDates:
			proj := m.registry.Get(name)
			if proj != nil {
				n = len(proj.Dates)
			}
		}
		children := m.registry.ChildrenOf(name)
		for _, child := range children {
			n += count(child.Name)
		}
		return n
	}
	return count(projName)
}

// totalColCount returns total items across all projects for a column.
func (m *DetailModel) totalColCount(col colKind) int {
	total := 0
	switch col {
	case colNotes:
		for _, v := range m.projectNotes {
			total += len(v)
		}
	case colTasks:
		for _, v := range m.projectTasks {
			total += len(v)
		}
	case colCards:
		for _, v := range m.projectCards {
			total += len(v)
		}
	case colBoards:
		for _, v := range m.projectBoards {
			total += len(v)
		}
	case colDates:
		if m.project != nil {
			total += len(m.project.Dates)
		}
		for _, desc := range m.allDescendants {
			total += len(desc.Dates)
		}
	}
	return total
}

func (m DetailModel) renderHorizIndicator(symbol string, height int) string {
	style := lipgloss.NewStyle().
		Width(3).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center)
	if symbol != " " {
		style = style.Bold(true)
	}
	return style.Render(symbol)
}

func (m *DetailModel) adjustScrollPosition() {
	col := m.selectedCol
	rows := m.columns[col]
	if len(rows) == 0 {
		return
	}
	cursor := m.colCursorPos[col]
	// Match the availableForRows logic in renderColumn
	visibleRows := m.height - len(strings.Split(m.indexPreview, "\n")) - 7
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

func (m DetailModel) calculateVisibleColumns() (start, end int) {
	// Each column: content width + 2 (border left+right) + lipgloss padding
	const colTotalWidth = detailColWidth + 4 // border (2) + inner padding (2)
	const indicatorWidth = 3
	availableWidth := m.width - indicatorWidth*2
	visibleCount := availableWidth / colTotalWidth
	if visibleCount < 1 {
		visibleCount = 1
	}
	start = m.colHorizOffset
	end = start + visibleCount
	if end > int(colCount) {
		end = int(colCount)
	}
	return start, end
}

func (m *DetailModel) adjustHorizScroll() {
	startCol, endCol := m.calculateVisibleColumns()
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
