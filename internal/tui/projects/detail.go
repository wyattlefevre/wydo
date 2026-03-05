package projects

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/kanban/operations"
	"wydo/internal/notes"
	"wydo/internal/tasks/data"
	"wydo/internal/tui/kanban"
	"wydo/internal/tui/messages"
	"wydo/internal/tui/shared"
	"wydo/internal/workspace"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const detailIndicatorWidth = 3

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

type detailMode int

const (
	detailModeNormal    detailMode = iota
	detailModeURLEditor            // editing project URLs
	detailModeURLPicker            // picking a URL to open
)

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
	cardBoard   map[string]kanbanmodels.Board  // card filename → parent board
	cardColumn  map[string]string              // card filename → column name

	// Column state
	columns        [colCount][]detailRow
	selectedCol    int
	colScrollOff   [colCount]int
	colCursorPos   [colCount]int
	colHorizOffset int // first visible column index (horizontal scroll)

	// Collapse state: project name → collapsed
	collapsedGroups map[string]bool

	// URL modal state
	mode      detailMode
	urlEditor *kanban.URLEditorModel
	urlPicker *projectURLPicker
}

// detailURLEntry is a URL with its owning project name.
type detailURLEntry struct {
	projectName string
	url         kanbanmodels.CardURL
}

// projectURLPicker is a grouped URL picker for the project detail view.
type projectURLPicker struct {
	entries []detailURLEntry
	cursor  int
	width   int
	height  int
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

func NewDetailModel(name, wsDir string, n []notes.Note, tasks []data.Task, cards []kanbanmodels.Card, boards []kanbanmodels.Board, allBoards []kanbanmodels.Board, project *workspace.Project, registry *workspace.ProjectRegistry, children []*workspace.Project, indexPreview string, allTasks []data.Task, allNotes []notes.Note) DetailModel {
	cardBoard := make(map[string]kanbanmodels.Board)
	cardColumn := make(map[string]string)
	for _, b := range allBoards {
		for _, col := range b.Columns {
			for _, c := range col.Cards {
				cardBoard[c.Filename] = b
				cardColumn[c.Filename] = col.Name
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
		cardColumn:      cardColumn,
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

// IsModal returns true when a URL modal is active.
func (m DetailModel) IsModal() bool {
	return m.mode != detailModeNormal
}

// IsTyping returns true when the URL editor has a text input focused.
func (m DetailModel) IsTyping() bool {
	return m.mode == detailModeURLEditor && m.urlEditor != nil && m.urlEditor.IsTyping()
}

// HintText returns the hint string for the detail view.
func (m DetailModel) HintText() string {
	switch m.mode {
	case detailModeURLEditor:
		return "n:add  d:delete  e:edit url  l:edit label  enter:save  esc:cancel"
	case detailModeURLPicker:
		return "j/k:navigate  /: search  enter:open  esc:cancel"
	}
	return "h/l:columns  j/k:navigate  space/enter:expand  enter:open  u:urls  esc:back"
}

func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case detailModeURLEditor:
			return m.updateURLEditor(msg)
		case detailModeURLPicker:
			return m.updateURLPicker(msg)
		}
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
			editor := kanban.NewURLEditorModel(m.project.URLs)
			editor.SetSize(m.width, m.height)
			m.urlEditor = &editor
			m.mode = detailModeURLEditor
		}

	case "U":
		if m.project != nil {
			editor := kanban.NewURLEditorModel(m.project.URLs)
			editor.SetSize(m.width, m.height)
			m.urlEditor = &editor
			m.mode = detailModeURLEditor
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
		}
		m.urlEditor = nil
		m.mode = detailModeNormal
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

func (m DetailModel) View() string {
	if m.mode == detailModeURLEditor && m.urlEditor != nil {
		return m.urlEditor.View()
	}
	if m.mode == detailModeURLPicker && m.urlPicker != nil {
		return m.urlPicker.View()
	}

	var lines []string

	lines = append(lines, titleStyle.Render(fmt.Sprintf("Project: %s", m.name)))
	lines = append(lines, "")

	if m.indexPreview != "" {
		for _, line := range strings.Split(m.indexPreview, "\n") {
			lines = append(lines, pathStyle.Render("  "+line))
		}
		lines = append(lines, "")
	}

	if allURLs := m.collectAllURLs(); len(allURLs) > 0 {
		lastProject := ""
		for _, e := range allURLs {
			if e.projectName != m.name && e.projectName != lastProject {
				lines = append(lines, sectionHeaderStyle.Render("  "+e.projectName))
			}
			lastProject = e.projectName
			u := e.url
			if u.Label == "" {
				lines = append(lines, pathStyle.Render("    "+u.URL))
			} else {
				lines = append(lines, detailItemStyle.Render("    "+u.Label)+pathStyle.Render("  "+u.URL))
			}
		}
		lines = append(lines, "")
	}

	headerLines := len(lines)
	fixedColHeight := m.height - headerLines - 2
	if fixedColHeight < 5 {
		fixedColHeight = 5
	}

	startCol, endCol, colWidth := m.calculateVisibleColumns()

	var colViews []string

	if startCol > 0 {
		colViews = append(colViews, m.renderHorizIndicator("◀", fixedColHeight))
	} else {
		colViews = append(colViews, m.renderHorizIndicator(" ", fixedColHeight))
	}

	for i := startCol; i < endCol; i++ {
		colViews = append(colViews, m.renderColumn(i, fixedColHeight, colWidth))
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

func (m DetailModel) renderColumn(colIdx int, fixedHeight int, colWidth int) string {
	col := colKind(colIdx)
	rows := m.columns[colIdx]
	focused := colIdx == m.selectedCol

	var s strings.Builder

	// Title line — truncate to column width
	count := m.totalColCount(col)
	title := fmt.Sprintf("%s (%d)", colNames[col], count)
	title = xansi.Truncate(title, colWidth, "")
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
			s.WriteString(m.renderRow(rows[i], i == cursor && focused, col, colWidth))
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

func (m DetailModel) renderRow(row detailRow, isSelected bool, col colKind, colWidth int) string {
	prefix := "  "
	if isSelected {
		prefix = "► "
	}
	indent := strings.Repeat("  ", row.depth)

	var rendered string

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
			rendered = selectedDetailItemStyle.Render(content)
		} else {
			rendered = sectionHeaderStyle.Render(content)
		}

	case rowKindNote:
		display := row.note.Title
		if display == "" {
			display = filepath.Base(row.note.FilePath)
		}
		if isSelected {
			rendered = selectedDetailItemStyle.Render(prefix + display)
		} else {
			rendered = detailItemStyle.Render(prefix + display)
		}

	case rowKindTask:
		taskLine := shared.StyledTaskLine(row.task)
		if isSelected {
			rendered = selectedDetailItemStyle.Render(prefix) + taskLine
		} else {
			rendered = detailItemStyle.Render(prefix) + taskLine
		}

	case rowKindCard:
		title := row.card.Title
		if title == "" {
			title = row.card.Filename
		}
		colName := m.cardColumn[row.card.Filename]
		isDone := strings.EqualFold(colName, "done")
		var titlePart string
		if isSelected {
			titlePart = selectedDetailItemStyle.Render(prefix + title)
		} else {
			titlePart = detailItemStyle.Render(prefix + title)
		}
		if colName != "" {
			var statusPart string
			if isDone {
				statusPart = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(" " + colName)
			} else {
				statusPart = pathStyle.Render(" " + colName)
			}
			rendered = titlePart + statusPart
		} else {
			rendered = titlePart
		}

	default:
		return ""
	}

	return xansi.Truncate(rendered, colWidth, "")
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

// calculateVisibleColumns returns the start/end column indices and the width
// each column should occupy so they fill the available screen width evenly.
func (m DetailModel) calculateVisibleColumns() (start, end, colWidth int) {
	const minColWidth = 20
	availableWidth := m.width - detailIndicatorWidth*2
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
