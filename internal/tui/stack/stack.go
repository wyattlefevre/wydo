package stack

import (
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/tasks/data"
	"wydo/internal/tasks/service"
	"wydo/internal/tui/messages"
	"wydo/internal/tui/theme"
	"wydo/internal/workspace"
)

type itemKind int

const (
	kindTask itemKind = iota
	kindCard
)

type stackItem struct {
	kind     itemKind
	priority int // 1=A (highest) … 6=F
	title    string
	subtitle string // board › column for cards
	task     *data.Task
	card     *kanbanmodels.Card
	// card navigation
	boardPath string
	colIndex  int
	cardIndex int
}

type stackGroup struct {
	name  string
	items []stackItem
}

// StackModel is the stack view — all prioritized cards and tasks, grouped by workspace.
type StackModel struct {
	workspaces []*workspace.Workspace
	taskSvc    service.TaskService
	boards     []kanbanmodels.Board

	groups    []stackGroup
	flatItems []stackItem // for cursor indexing

	cursor int
	width  int
	height int
}

func NewStackModel(workspaces []*workspace.Workspace, taskSvc service.TaskService, boards []kanbanmodels.Board) StackModel {
	m := StackModel{
		workspaces: workspaces,
		taskSvc:    taskSvc,
		boards:     boards,
	}
	m.refreshData()
	return m
}

func (m *StackModel) SetData(workspaces []*workspace.Workspace, taskSvc service.TaskService, boards []kanbanmodels.Board) {
	m.workspaces = workspaces
	m.taskSvc = taskSvc
	m.boards = boards
	m.refreshData()
}

func (m *StackModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *StackModel) refreshData() {
	// One bucket per workspace, preserving order
	names := make([]string, len(m.workspaces))
	buckets := make([][]stackItem, len(m.workspaces))
	for i, ws := range m.workspaces {
		names[i] = filepath.Base(ws.RootDir)
	}

	wsIndexFor := func(path string) int {
		for i, ws := range m.workspaces {
			if strings.HasPrefix(path, ws.RootDir) {
				return i
			}
		}
		return -1
	}

	// Prioritized tasks (pending only)
	if m.taskSvc != nil {
		tasks, _ := m.taskSvc.ListPending()
		for _, t := range tasks {
			if t.Priority == data.PriorityNone || t.Done {
				continue
			}
			idx := wsIndexFor(t.File)
			if idx < 0 {
				continue
			}
			p := int(t.Priority-'A') + 1
			tc := t
			buckets[idx] = append(buckets[idx], stackItem{
				kind:     kindTask,
				priority: p,
				title:    t.Name,
				task:     &tc,
			})
		}
	}

	// Prioritized cards (non-done columns, non-archived)
	for _, board := range m.boards {
		idx := wsIndexFor(board.Path)
		if idx < 0 {
			continue
		}
		for ci, col := range board.Columns {
			if strings.EqualFold(col.Name, "done") {
				continue
			}
			for ki, card := range col.Cards {
				if card.Priority == 0 || card.Archived {
					continue
				}
				cc := card
				buckets[idx] = append(buckets[idx], stackItem{
					kind:      kindCard,
					priority:  card.Priority,
					title:     card.Title,
					subtitle:  board.Name + " › " + col.Name,
					card:      &cc,
					boardPath: board.Path,
					colIndex:  ci,
					cardIndex: ki,
				})
			}
		}
	}

	// Sort each bucket by priority ascending (1=A first) and build groups
	m.groups = nil
	m.flatItems = nil
	for i, items := range buckets {
		if len(items) == 0 {
			continue
		}
		sort.SliceStable(items, func(a, b int) bool {
			return items[a].priority < items[b].priority
		})
		m.groups = append(m.groups, stackGroup{name: names[i], items: items})
		m.flatItems = append(m.flatItems, items...)
	}

	if m.cursor >= len(m.flatItems) {
		m.cursor = 0
	}
}

func (m StackModel) Init() tea.Cmd {
	return nil
}

func (m StackModel) Update(msg tea.Msg) (StackModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.flatItems)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "g":
			m.cursor = 0
		case "G":
			if len(m.flatItems) > 0 {
				m.cursor = len(m.flatItems) - 1
			}
		case "enter":
			return m.openSelected()
		}
	}
	return m, nil
}

func (m StackModel) openSelected() (StackModel, tea.Cmd) {
	if m.cursor < 0 || m.cursor >= len(m.flatItems) {
		return m, nil
	}
	item := m.flatItems[m.cursor]
	switch item.kind {
	case kindTask:
		if item.task != nil {
			id := item.task.ID
			return m, func() tea.Msg { return messages.FocusTaskMsg{TaskID: id} }
		}
	case kindCard:
		path, ci, ki := item.boardPath, item.colIndex, item.cardIndex
		return m, func() tea.Msg {
			return messages.OpenBoardMsg{BoardPath: path, ColIndex: ci, CardIndex: ki}
		}
	}
	return m, nil
}

func (m StackModel) View() string {
	if len(m.groups) == 0 {
		return theme.Muted.Render("  No prioritized items.")
	}

	type renderedLine struct {
		text      string
		itemIndex int // -1 for headers/spacers
	}

	var lines []renderedLine
	groupHeaderStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Secondary)
	flatIdx := 0

	for gi, g := range m.groups {
		lines = append(lines, renderedLine{text: groupHeaderStyle.Render("  " + g.name), itemIndex: -1})
		for _, item := range g.items {
			lines = append(lines, renderedLine{text: renderItem(item, flatIdx == m.cursor), itemIndex: flatIdx})
			flatIdx++
		}
		if gi < len(m.groups)-1 {
			lines = append(lines, renderedLine{text: "", itemIndex: -1})
		}
	}

	// Find cursor line
	cursorLine := 0
	for i, l := range lines {
		if l.itemIndex == m.cursor {
			cursorLine = i
			break
		}
	}

	// Scroll so cursor is visible
	visibleHeight := m.height
	if visibleHeight <= 0 {
		visibleHeight = 24
	}
	scrollStart := 0
	if cursorLine >= visibleHeight {
		scrollStart = cursorLine - visibleHeight + 1
	}
	end := scrollStart + visibleHeight
	if end > len(lines) {
		end = len(lines)
	}

	out := make([]string, 0, end-scrollStart)
	for _, l := range lines[scrollStart:end] {
		out = append(out, l.text)
	}
	return strings.Join(out, "\n")
}

func priorityStyle(p int) lipgloss.Style {
	var bg, fg lipgloss.Color
	switch p {
	case 1:
		bg, fg = lipgloss.Color("5"), lipgloss.Color("16")   // magenta
	case 2:
		bg, fg = lipgloss.Color("1"), lipgloss.Color("16")   // red
	case 3:
		bg, fg = lipgloss.Color("208"), lipgloss.Color("16") // orange
	case 4:
		bg, fg = lipgloss.Color("3"), lipgloss.Color("16")   // yellow
	case 5:
		bg, fg = lipgloss.Color("2"), lipgloss.Color("16")   // green
	default:
		bg, fg = lipgloss.Color("8"), lipgloss.Color("15")   // gray
	}
	return lipgloss.NewStyle().Bold(true).Background(bg).Foreground(fg)
}

var (
	taskKindStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))  // bright blue
	cardKindStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")) // amber
)

func renderItem(item stackItem, selected bool) string {
	pBadge := priorityStyle(item.priority).Render(" " + priorityLabel(item.priority) + " ")

	var kindBadge string
	switch item.kind {
	case kindTask:
		kindBadge = taskKindStyle.Render("[Task]")
	case kindCard:
		kindBadge = cardKindStyle.Render("[Card]")
	}

	var titleStr string
	if selected {
		titleStr = lipgloss.NewStyle().Bold(true).Foreground(theme.TextBright).Render(item.title)
	} else {
		titleStr = lipgloss.NewStyle().Foreground(theme.Text).Render(item.title)
	}

	var suffix string
	if item.subtitle != "" {
		suffix = "  " + theme.Muted.Render(item.subtitle)
	}

	if selected {
		cursor := theme.Cursor.Render(">")
		return "  " + cursor + " " + pBadge + " " + kindBadge + " " + titleStr + suffix
	}
	return "    " + pBadge + " " + kindBadge + " " + titleStr + suffix
}

func priorityLabel(p int) string {
	if p < 1 || p > 6 {
		return "?"
	}
	return string(rune('A' + p - 1))
}
