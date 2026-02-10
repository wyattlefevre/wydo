package projects

import (
	"fmt"
	"path/filepath"
	"strings"

	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/notes"
	"wydo/internal/tasks/data"
	"wydo/internal/tui/messages"
	"wydo/internal/tui/shared"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type detailSection int

const (
	sectionNotes detailSection = iota
	sectionTasks
	sectionCards
	sectionBoards
	sectionCount // sentinel for cycling
)

// DetailModel shows project details with notes, tasks, and cards.
type DetailModel struct {
	name     string
	wsDir    string
	notes    []notes.Note
	tasks    []data.Task
	cards    []kanbanmodels.Card
	boards   []kanbanmodels.Board
	section  detailSection
	selected int // cursor within current section
	width    int
	height   int
}

func NewDetailModel(name, wsDir string, n []notes.Note, tasks []data.Task, cards []kanbanmodels.Card, boards []kanbanmodels.Board) DetailModel {
	return DetailModel{
		name:   name,
		wsDir:  wsDir,
		notes:  n,
		tasks:  tasks,
		cards:  cards,
		boards: boards,
	}
}

// SetSize updates the view dimensions.
func (m *DetailModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// IsModal returns false — detail view has no modals.
func (m DetailModel) IsModal() bool {
	return false
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
	}
	return 0
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

	case "tab":
		m.section = (m.section + 1) % sectionCount
		m.selected = 0

	case "shift+tab":
		m.section = (m.section - 1 + sectionCount) % sectionCount
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

	case "j", "down":
		if m.selected < m.sectionLen()-1 {
			m.selected++
		}

	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}

	case "enter":
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
	}
	return m, nil
}

func (m DetailModel) View() string {
	var lines []string

	// Title
	lines = append(lines, titleStyle.Render(fmt.Sprintf("Project: %s", m.name)))
	lines = append(lines, "")

	// Section tabs
	tabs := m.renderTabs()
	lines = append(lines, tabs)
	lines = append(lines, "")

	// Section content
	maxItems := m.height - 10
	if maxItems < 3 {
		maxItems = 3
	}

	switch m.section {
	case sectionNotes:
		lines = append(lines, m.renderNotes(maxItems)...)
	case sectionTasks:
		lines = append(lines, m.renderTasks(maxItems)...)
	case sectionCards:
		lines = append(lines, m.renderCards(maxItems)...)
	case sectionBoards:
		lines = append(lines, m.renderBoards(maxItems)...)
	}

	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("j/k: navigate • tab/1/2/3/4: switch section • enter: open • esc/q: back"))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m DetailModel) renderTabs() string {
	names := []string{"Notes", "Tasks", "Cards", "Boards"}
	var tabs []string
	for i, name := range names {
		label := fmt.Sprintf(" %d:%s (%d) ", i+1, name, m.countForSection(detailSection(i)))
		if detailSection(i) == m.section {
			tabs = append(tabs, sectionActiveStyle.Render(label))
		} else {
			tabs = append(tabs, sectionHeaderStyle.Render(label))
		}
	}
	return "  " + strings.Join(tabs, pathStyle.Render(" │ "))
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
	}
	return 0
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
		lines = append(lines, style.Render(prefix+title))
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
