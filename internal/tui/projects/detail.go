package projects

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/notes"
	"wydo/internal/tasks/data"
	"wydo/internal/tui/messages"
	"wydo/internal/tui/shared"
	"wydo/internal/workspace"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const detailColWidth = 38

type colKind int

const (
	colNotes colKind = iota
	colTasks
	colCards
	colCount
)

var colNames = [colCount]string{"Notes", "Tasks", "Cards"}

type rowKind int

const (
	rowKindGroup rowKind = iota
	rowKindNote
	rowKindTask
	rowKindCard
)

type detailRow struct {
	kind        rowKind
	depth       int
	projectName string

	// Only one populated based on kind:
	note notes.Note
	task data.Task
	card kanbanmodels.Card
}

// DetailModel shows project details with notes, tasks, and cards in a
// kanban-style column layout with hierarchical grouping by child project.
type DetailModel struct {
	name         string
	wsDir        string
	project      *workspace.Project
	registry     *workspace.ProjectRegistry
	indexPreview string
	width, height int

	// Pre-computed per-project data (keyed by project name)
	projectNotes map[string][]notes.Note
	projectTasks map[string][]data.Task
	projectCards map[string][]kanbanmodels.Card
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
	}

	m.projectNotes = make(map[string][]notes.Note)
	m.projectTasks = make(map[string][]data.Task)
	m.projectCards = make(map[string][]kanbanmodels.Card)

	if registry != nil {
		m.allDescendants = collectAllDescendants(registry, name)
		m.projectNotes[name] = registry.NotesForProject(name, allNotes)
		m.projectTasks[name] = registry.TasksForProject(name, allTasks)
		m.projectCards[name] = registry.CardsForProject(name, allBoards)
		for _, desc := range m.allDescendants {
			m.projectNotes[desc.Name] = registry.NotesForProject(desc.Name, allNotes)
			m.projectTasks[desc.Name] = registry.TasksForProject(desc.Name, allTasks)
			m.projectCards[desc.Name] = registry.CardsForProject(desc.Name, allBoards)
		}
	} else {
		m.projectNotes[name] = n
		m.projectTasks[name] = tasks
		m.projectCards[name] = cards
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

// IsModal always returns false (no modals in this view currently).
func (m DetailModel) IsModal() bool {
	return false
}

// IsTyping always returns false (no text inputs in this view currently).
func (m DetailModel) IsTyping() bool {
	return false
}

// HintText returns the hint string for the detail view.
func (m DetailModel) HintText() string {
	return "h/l:columns  j/k:navigate  space/enter:expand  enter:open  esc:back"
}

func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
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
		case rowKindCard:
			if b, ok := m.cardBoard[row.card.Filename]; ok {
				return m, func() tea.Msg {
					return messages.OpenBoardMsg{BoardPath: b.Path}
				}
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

func (m DetailModel) View() string {
	var lines []string

	lines = append(lines, titleStyle.Render(fmt.Sprintf("Project: %s", m.name)))
	lines = append(lines, "")

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

	if startCol > 0 {
		colViews = append(colViews, m.renderHorizIndicator("◀", fixedColHeight))
	} else {
		colViews = append(colViews, m.renderHorizIndicator(" ", fixedColHeight))
	}

	for i := startCol; i < endCol; i++ {
		colViews = append(colViews, m.renderColumn(i, fixedColHeight))
	}

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

	// availableForRows = fixedHeight - title(1) - topIndicator(1) - bottomIndicator(1)
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
	if remaining := len(rows) - end; remaining > 0 {
		s.WriteString(pathStyle.Render(fmt.Sprintf("  ▼ %d below", remaining)))
	}
	s.WriteString("\n")

	return lipgloss.NewStyle().Width(detailColWidth + 2).Height(fixedHeight).Render(s.String())
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
		}
		for _, child := range m.registry.ChildrenOf(name) {
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
	const colTotalWidth = detailColWidth + 4
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
