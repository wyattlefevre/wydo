package projects

import (
	"fmt"
	"path/filepath"
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

type detailSection int

const (
	sectionNotes detailSection = iota
	sectionTasks
	sectionCards
	sectionBoards
	sectionSubProjects
	sectionDates
	sectionChildrenData // tasks+cards grouped by child project
	sectionCount        // sentinel for cycling
)

type detailEditMode int

const (
	detailModeNormal        detailEditMode = iota
	detailModeDateLabel                    // editing label text input
	detailModeDatePicker                   // shared date picker
	detailModeCreateSubName                // typing new sub-project name
)

// DetailModel shows project details with notes, tasks, cards, boards, sub-projects, and dates.
type DetailModel struct {
	name      string
	wsDir     string
	notes     []notes.Note
	tasks     []data.Task
	cards     []kanbanmodels.Card
	boards    []kanbanmodels.Board
	cardBoard map[string]kanbanmodels.Board // card filename → parent board
	section   detailSection
	selected  int // cursor within current section
	width     int
	height    int

	// New fields
	project      *workspace.Project
	registry     *workspace.ProjectRegistry
	children     []*workspace.Project
	indexPreview string

	// Child project aggregation
	childTasks map[string][]data.Task
	childCards map[string][]kanbanmodels.Card
	allTasks   []data.Task

	// Date editing state
	editMode       detailEditMode
	editingDateIdx int // -1 = new date
	editingLabel   string
	labelInput     textinput.Model
	datePicker     *shared.DatePickerModel

	// Sub-project creation state
	subNameInput textinput.Model
}

func NewDetailModel(name, wsDir string, n []notes.Note, tasks []data.Task, cards []kanbanmodels.Card, boards []kanbanmodels.Board, allBoards []kanbanmodels.Board, project *workspace.Project, registry *workspace.ProjectRegistry, children []*workspace.Project, indexPreview string, allTasks []data.Task) DetailModel {
	cardBoard := make(map[string]kanbanmodels.Board)
	for _, b := range allBoards {
		for _, col := range b.Columns {
			for _, c := range col.Cards {
				cardBoard[c.Filename] = b
			}
		}
	}

	// Compute child tasks and cards
	childTasks := make(map[string][]data.Task)
	childCards := make(map[string][]kanbanmodels.Card)
	if registry != nil {
		for _, child := range children {
			childTasks[child.Name] = registry.TasksForProject(child.Name, allTasks)
			childCards[child.Name] = registry.CardsForProject(child.Name, allBoards)
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

	return DetailModel{
		name:           name,
		wsDir:          wsDir,
		notes:          n,
		tasks:          tasks,
		cards:          cards,
		boards:         boards,
		cardBoard:      cardBoard,
		project:        project,
		registry:       registry,
		children:       children,
		indexPreview:   indexPreview,
		childTasks:     childTasks,
		childCards:     childCards,
		allTasks:       allTasks,
		editingDateIdx: -1,
		labelInput:     ti,
		subNameInput:   si,
	}
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
	if m.section == sectionDates {
		return "j/k:navigate  tab/1-7:sections  n:new date  e:edit  d:delete  enter:open  esc:back  ?:help"
	}
	if m.section == sectionSubProjects {
		return "j/k:navigate  tab/1-7:sections  n:new sub-project  enter:open  esc:back  ?:help"
	}
	if m.section == sectionChildrenData {
		return "j/k:navigate  tab/1-7:sections  esc:back  ?:help"
	}
	return "j/k:navigate  tab/1-7:sections  enter:open  esc:back  ?:help"
}

func (m DetailModel) sectionLen() int {
	switch m.section {
	case sectionNotes:
		return len(m.notes)
	case sectionTasks:
		return len(m.tasks)
	case sectionCards:
		return len(m.cards)
	case sectionBoards:
		return len(m.boards)
	case sectionSubProjects:
		return len(m.children)
	case sectionDates:
		if m.project != nil {
			return len(m.project.Dates)
		}
	case sectionChildrenData:
		total := 0
		for _, child := range m.children {
			total += len(m.childTasks[child.Name])
			total += len(m.childCards[child.Name])
		}
		return total
	}
	return 0
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
		// Move to date picker
		var cur *time.Time
		if m.editingDateIdx >= 0 && m.project != nil && m.editingDateIdx < len(m.project.Dates) {
			t := m.project.Dates[m.editingDateIdx].Date
			cur = &t
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
		if picked := m.datePicker.GetDate(); picked != nil && m.project != nil {
			newDate := workspace.ProjectDate{
				Label: m.editingLabel,
				Date:  *picked,
			}
			dates := make([]workspace.ProjectDate, len(m.project.Dates))
			copy(dates, m.project.Dates)
			if m.editingDateIdx == -1 {
				dates = append(dates, newDate)
			} else if m.editingDateIdx < len(dates) {
				dates[m.editingDateIdx] = newDate
			}
			if err := workspace.WriteProjectDates(m.project, dates); err == nil {
				// project.Dates already updated by WriteProjectDates
			}
		}
		m.editMode = detailModeNormal
		m.datePicker = nil
		// Refresh with DataRefreshMsg so app reloads project data
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

	case "tab":
		m.section = (m.section + 1) % sectionCount
		// Skip sectionChildrenData if no children
		if m.section == sectionChildrenData && len(m.children) == 0 {
			m.section = sectionNotes
		}
		m.selected = 0

	case "shift+tab":
		m.section = (m.section - 1 + sectionCount) % sectionCount
		// Skip sectionChildrenData if no children
		if m.section == sectionChildrenData && len(m.children) == 0 {
			m.section = sectionDates
		}
		m.selected = 0

	case "1":
		m.section = sectionNotes
		m.selected = 0

	case "2":
		m.section = sectionTasks
		m.selected = 0

	case "3":
		m.section = sectionCards
		m.selected = 0

	case "4":
		m.section = sectionBoards
		m.selected = 0

	case "5":
		m.section = sectionSubProjects
		m.selected = 0

	case "6":
		m.section = sectionDates
		m.selected = 0

	case "7":
		if len(m.children) > 0 {
			m.section = sectionChildrenData
			m.selected = 0
		}

	case "j", "down":
		if m.selected < m.sectionLen()-1 {
			m.selected++
		}

	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}

	case "n":
		if m.section == sectionDates {
			return m.startDateEdit(-1)
		}
		if m.section == sectionSubProjects {
			m.editMode = detailModeCreateSubName
			m.subNameInput.SetValue("")
			m.subNameInput.Focus()
			return m, textinput.Blink
		}

	case "e", "enter":
		if m.section == sectionDates {
			if m.project != nil && m.selected < len(m.project.Dates) {
				return m.startDateEdit(m.selected)
			}
		}
		if msg.String() == "enter" {
			if m.section == sectionTasks && m.selected < len(m.tasks) {
				task := m.tasks[m.selected]
				return m, func() tea.Msg {
					return messages.FocusTaskMsg{TaskID: task.ID}
				}
			}
			if m.section == sectionBoards && m.selected < len(m.boards) {
				board := m.boards[m.selected]
				return m, func() tea.Msg {
					return messages.OpenBoardMsg{BoardPath: board.Path}
				}
			}
			if m.section == sectionSubProjects && m.selected < len(m.children) {
				child := m.children[m.selected]
				wsDir := m.wsDir
				return m, func() tea.Msg {
					return messages.OpenProjectMsg{
						ProjectName:      child.Name,
						WorkspaceRootDir: wsDir,
					}
				}
			}
		}

	case "d":
		if m.section == sectionDates && m.project != nil && m.selected < len(m.project.Dates) {
			dates := make([]workspace.ProjectDate, 0, len(m.project.Dates)-1)
			for i, d := range m.project.Dates {
				if i != m.selected {
					dates = append(dates, d)
				}
			}
			_ = workspace.WriteProjectDates(m.project, dates)
			if m.selected >= len(m.project.Dates) {
				m.selected = max(0, len(m.project.Dates)-1)
			}
			return m, func() tea.Msg { return messages.DataRefreshMsg{} }
		}
	}
	return m, nil
}

func (m DetailModel) startDateEdit(idx int) (DetailModel, tea.Cmd) {
	m.editingDateIdx = idx
	m.editMode = detailModeDateLabel
	if idx >= 0 && m.project != nil && idx < len(m.project.Dates) {
		m.editingLabel = m.project.Dates[idx].Label
		m.labelInput.SetValue(m.project.Dates[idx].Label)
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

	// Index preview (greyed, above tabs)
	if m.indexPreview != "" {
		for _, line := range strings.Split(m.indexPreview, "\n") {
			lines = append(lines, pathStyle.Render("  "+line))
		}
		lines = append(lines, "")
	}

	// Focus bar (tabs)
	lines = append(lines, m.renderTabs())
	lines = append(lines, "")

	// All sections stacked
	lines = append(lines, m.renderAllSections()...)

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

func (m DetailModel) renderTabs() string {
	type tabInfo struct {
		name  string
		count int
	}
	tabs := []tabInfo{
		{"Notes", len(m.notes)},
		{"Tasks", len(m.tasks)},
		{"Cards", len(m.cards)},
		{"Boards", len(m.boards)},
		{"Sub-projects", len(m.children)},
		{"Dates", func() int {
			if m.project != nil {
				return len(m.project.Dates)
			}
			return 0
		}()},
	}
	if len(m.children) > 0 {
		total := 0
		for _, child := range m.children {
			total += len(m.childTasks[child.Name])
			total += len(m.childCards[child.Name])
		}
		tabs = append(tabs, tabInfo{"Children", total})
	}
	var rendered []string
	for i, t := range tabs {
		label := fmt.Sprintf(" %d:%s (%d) ", i+1, t.name, t.count)
		if detailSection(i) == m.section {
			rendered = append(rendered, sectionActiveStyle.Render(label))
		} else {
			rendered = append(rendered, sectionHeaderStyle.Render(label))
		}
	}
	return "  " + strings.Join(rendered, pathStyle.Render(" │ "))
}

func (m DetailModel) countForSection(s detailSection) int {
	switch s {
	case sectionNotes:
		return len(m.notes)
	case sectionTasks:
		return len(m.tasks)
	case sectionCards:
		return len(m.cards)
	case sectionBoards:
		return len(m.boards)
	case sectionSubProjects:
		return len(m.children)
	case sectionDates:
		if m.project != nil {
			return len(m.project.Dates)
		}
	case sectionChildrenData:
		total := 0
		for _, child := range m.children {
			total += len(m.childTasks[child.Name])
			total += len(m.childCards[child.Name])
		}
		return total
	}
	return 0
}

// renderSectionHeader renders a styled section divider with label and count.
func (m DetailModel) renderSectionHeader(label string, count int, focused bool) string {
	content := fmt.Sprintf("── %s (%d) ", label, count)
	if focused {
		return sectionActiveStyle.Render(content)
	}
	return sectionHeaderStyle.Render(content)
}

// renderAllSections renders all sections stacked vertically.
// Non-focused sections show up to 3 items; focused section shows up to 5 with cursor.
func (m DetailModel) renderAllSections() []string {
	const nonFocusedMax = 3
	const focusedMax = 5

	type sectionDef struct {
		s     detailSection
		label string
	}

	sections := []sectionDef{
		{sectionNotes, "Notes"},
		{sectionTasks, "Tasks"},
		{sectionCards, "Cards"},
		{sectionBoards, "Boards"},
		{sectionSubProjects, "Sub-Projects"},
		{sectionDates, "Dates"},
	}

	var lines []string

	for _, sec := range sections {
		count := m.countForSection(sec.s)
		focused := sec.s == m.section
		lines = append(lines, m.renderSectionHeader(sec.label, count, focused))

		if focused {
			switch sec.s {
			case sectionNotes:
				lines = append(lines, m.renderNotes(focusedMax)...)
			case sectionTasks:
				lines = append(lines, m.renderTasks(focusedMax)...)
			case sectionCards:
				lines = append(lines, m.renderCards(focusedMax)...)
			case sectionBoards:
				lines = append(lines, m.renderBoards(focusedMax)...)
			case sectionSubProjects:
				lines = append(lines, m.renderSubProjects(focusedMax)...)
			case sectionDates:
				lines = append(lines, m.renderDates(focusedMax)...)
			}
		} else {
			lines = append(lines, m.renderSectionPreview(sec.s, nonFocusedMax)...)
		}
		lines = append(lines, "")
	}

	// Child project data (parent projects only)
	if len(m.children) > 0 {
		childDataFocused := m.section == sectionChildrenData
		childCount := m.countForSection(sectionChildrenData)
		lines = append(lines, m.renderSectionHeader("Child Project Data", childCount, childDataFocused))
		lines = append(lines, m.renderChildrenData(3)...)
	}

	return lines
}

// renderSectionPreview renders a section without cursor, showing up to maxItems from index 0.
func (m DetailModel) renderSectionPreview(section detailSection, maxItems int) []string {
	var items []string

	switch section {
	case sectionNotes:
		for _, n := range m.notes {
			display := n.Title
			if display == "" {
				display = filepath.Base(n.FilePath)
			}
			items = append(items, detailItemStyle.Render("  "+display))
		}
	case sectionTasks:
		for _, t := range m.tasks {
			items = append(items, detailItemStyle.Render("  ")+shared.StyledTaskLine(t))
		}
	case sectionCards:
		for _, c := range m.cards {
			title := c.Title
			if title == "" {
				title = c.Filename
			}
			if b, ok := m.cardBoard[c.Filename]; ok {
				boardName := b.Name
				if boardName == "" {
					boardName = filepath.Base(b.Path)
				}
				items = append(items, detailItemStyle.Render("  "+title)+"  "+pathStyle.Render(boardName))
			} else {
				items = append(items, detailItemStyle.Render("  "+title))
			}
		}
	case sectionBoards:
		for _, b := range m.boards {
			title := b.Name
			if title == "" {
				title = filepath.Base(b.Path)
			}
			items = append(items, detailItemStyle.Render("  "+title))
		}
	case sectionSubProjects:
		for _, child := range m.children {
			items = append(items, detailItemStyle.Render("  "+child.Name))
		}
	case sectionDates:
		if m.project != nil {
			for _, d := range m.project.Dates {
				label := d.Label
				if label == "" {
					label = "(no label)"
				}
				dateStr := d.Date.Format("Mon Jan 2, 2006")
				items = append(items, detailItemStyle.Render("  "+label+" — "+dateStr))
			}
		}
	}

	if len(items) == 0 {
		return []string{listItemStyle.Render("  (none)")}
	}

	var lines []string
	end := len(items)
	if end > maxItems {
		end = maxItems
	}
	lines = append(lines, items[:end]...)
	if len(items) > maxItems {
		lines = append(lines, pathStyle.Render(fmt.Sprintf("  +%d more", len(items)-maxItems)))
	}
	return lines
}

// renderChildrenData renders child project tasks and cards grouped by child.
func (m DetailModel) renderChildrenData(maxItemsPerChild int) []string {
	var lines []string
	for _, child := range m.children {
		lines = append(lines, sectionHeaderStyle.Render(fmt.Sprintf("  %s", child.Name)))

		tasks := m.childTasks[child.Name]
		if len(tasks) == 0 {
			lines = append(lines, listItemStyle.Render("    Tasks: (none)"))
		} else {
			for i, t := range tasks {
				if i >= maxItemsPerChild {
					lines = append(lines, pathStyle.Render(fmt.Sprintf("    +%d more tasks", len(tasks)-maxItemsPerChild)))
					break
				}
				lines = append(lines, detailItemStyle.Render("    ")+shared.StyledTaskLine(t))
			}
		}

		cards := m.childCards[child.Name]
		if len(cards) == 0 {
			lines = append(lines, listItemStyle.Render("    Cards: (none)"))
		} else {
			for i, c := range cards {
				if i >= maxItemsPerChild {
					lines = append(lines, pathStyle.Render(fmt.Sprintf("    +%d more cards", len(cards)-maxItemsPerChild)))
					break
				}
				title := c.Title
				if title == "" {
					title = c.Filename
				}
				lines = append(lines, detailItemStyle.Render("    "+title))
			}
		}
		lines = append(lines, "")
	}
	return lines
}

func (m DetailModel) renderNotes(maxItems int) []string {
	if len(m.notes) == 0 {
		return []string{listItemStyle.Render("  No notes found")}
	}
	var lines []string
	start, end := m.visibleRange(len(m.notes), maxItems)
	for i := start; i < end; i++ {
		n := m.notes[i]
		style := detailItemStyle
		prefix := "  "
		if i == m.selected {
			style = selectedDetailItemStyle
			prefix = "► "
		}
		display := n.Title
		if display == "" {
			display = filepath.Base(n.FilePath)
		}
		lines = append(lines, style.Render(prefix+display))
	}
	lines = append(lines, m.scrollIndicators(len(m.notes), start, end)...)
	return lines
}

func (m DetailModel) renderTasks(maxItems int) []string {
	if len(m.tasks) == 0 {
		return []string{listItemStyle.Render("  No tasks found")}
	}
	var lines []string
	start, end := m.visibleRange(len(m.tasks), maxItems)
	for i := start; i < end; i++ {
		t := m.tasks[i]
		prefix := "  "
		if i == m.selected {
			prefix = "► "
		}
		taskLine := shared.StyledTaskLine(t)
		if i == m.selected {
			lines = append(lines, selectedDetailItemStyle.Render(prefix)+taskLine)
		} else {
			lines = append(lines, detailItemStyle.Render(prefix)+taskLine)
		}
	}
	lines = append(lines, m.scrollIndicators(len(m.tasks), start, end)...)
	return lines
}

func (m DetailModel) renderCards(maxItems int) []string {
	if len(m.cards) == 0 {
		return []string{listItemStyle.Render("  No cards found")}
	}
	var lines []string
	start, end := m.visibleRange(len(m.cards), maxItems)
	for i := start; i < end; i++ {
		c := m.cards[i]
		style := detailItemStyle
		prefix := "  "
		if i == m.selected {
			style = selectedDetailItemStyle
			prefix = "► "
		}
		title := c.Title
		if title == "" {
			title = c.Filename
		}
		if b, ok := m.cardBoard[c.Filename]; ok {
			boardName := b.Name
			if boardName == "" {
				boardName = filepath.Base(b.Path)
			}
			relPath, err := filepath.Rel(m.wsDir, b.Path)
			if err != nil {
				relPath = b.Path
			}
			lines = append(lines, style.Render(prefix+title)+"  "+pathStyle.Render(boardName+" · "+relPath))
		} else {
			lines = append(lines, style.Render(prefix+title))
		}
	}
	lines = append(lines, m.scrollIndicators(len(m.cards), start, end)...)
	return lines
}

func (m DetailModel) renderBoards(maxItems int) []string {
	if len(m.boards) == 0 {
		return []string{listItemStyle.Render("  No boards found")}
	}
	var lines []string
	start, end := m.visibleRange(len(m.boards), maxItems)
	for i := start; i < end; i++ {
		b := m.boards[i]
		style := detailItemStyle
		prefix := "  "
		if i == m.selected {
			style = selectedDetailItemStyle
			prefix = "► "
		}
		title := b.Name
		if title == "" {
			title = filepath.Base(b.Path)
		}
		lines = append(lines, style.Render(prefix+title))
	}
	lines = append(lines, m.scrollIndicators(len(m.boards), start, end)...)
	return lines
}

func (m DetailModel) renderSubProjects(maxItems int) []string {
	var lines []string
	if len(m.children) == 0 {
		lines = append(lines, listItemStyle.Render("  No sub-projects found"))
	} else {
		start, end := m.visibleRange(len(m.children), maxItems)
		for i := start; i < end; i++ {
			child := m.children[i]
			style := detailItemStyle
			prefix := "  "
			if i == m.selected {
				style = selectedDetailItemStyle
				prefix = "► "
			}
			lines = append(lines, style.Render(prefix+child.Name))
		}
		lines = append(lines, m.scrollIndicators(len(m.children), start, end)...)
	}
	if m.editMode == detailModeCreateSubName {
		lines = append(lines, "")
		lines = append(lines, "  New sub-project: "+m.subNameInput.View())
	}
	return lines
}

func (m DetailModel) renderDates(maxItems int) []string {
	var dates []workspace.ProjectDate
	if m.project != nil {
		dates = m.project.Dates
	}
	if len(dates) == 0 {
		return []string{listItemStyle.Render("  No dates. Press 'n' to add one.")}
	}
	var lines []string
	start, end := m.visibleRange(len(dates), maxItems)
	for i := start; i < end; i++ {
		d := dates[i]
		style := detailItemStyle
		prefix := "  "
		if i == m.selected {
			style = selectedDetailItemStyle
			prefix = "► "
		}
		label := d.Label
		if label == "" {
			label = "(no label)"
		}
		dateStr := d.Date.Format("Mon Jan 2, 2006")
		lines = append(lines, style.Render(prefix+label+" — "+dateStr))
	}
	lines = append(lines, m.scrollIndicators(len(dates), start, end)...)
	return lines
}

func (m DetailModel) visibleRange(total, maxItems int) (int, int) {
	start := 0
	if m.selected >= maxItems {
		start = m.selected - maxItems + 1
	}
	end := start + maxItems
	if end > total {
		end = total
	}
	return start, end
}

func (m DetailModel) scrollIndicators(total, start, end int) []string {
	var lines []string
	if start > 0 {
		lines = append(lines, pathStyle.Render(fmt.Sprintf("  ▲ %d more above", start)))
	}
	if end < total {
		lines = append(lines, pathStyle.Render(fmt.Sprintf("  ▼ %d more below", total-end)))
	}
	return lines
}
