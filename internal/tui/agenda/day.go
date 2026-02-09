package agenda

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	agendapkg "wydo/internal/agenda"
	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/notes"
	"wydo/internal/tasks/service"
	"wydo/internal/tui/messages"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	sectionStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	emptyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	navHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// DayModel is the day agenda view
type DayModel struct {
	date        time.Time
	buckets     []agendapkg.DateBucket
	taskSvc     service.TaskService
	boards      []kanbanmodels.Board
	notes       []notes.Note
	items       []agendapkg.AgendaItem // flattened items for cursor navigation
	cursor      int
	width       int
	height      int
}

// NewDayModel creates a new day agenda view
func NewDayModel(taskSvc service.TaskService, boards []kanbanmodels.Board, allNotes []notes.Note) DayModel {
	m := DayModel{
		date:    time.Now(),
		taskSvc: taskSvc,
		boards:  boards,
		notes:   allNotes,
	}
	m.refreshData()
	return m
}

func (m *DayModel) refreshData() {
	dateRange := agendapkg.DayRange(m.date)
	m.buckets = agendapkg.QueryAgenda(m.taskSvc, m.boards, m.notes, dateRange)

	// Flatten items for cursor navigation
	m.items = nil
	for _, bucket := range m.buckets {
		m.items = append(m.items, bucket.AllItems()...)
	}

	// Clamp cursor
	if m.cursor >= len(m.items) {
		m.cursor = max(0, len(m.items)-1)
	}
}

// SetSize updates the view dimensions
func (m *DayModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetData updates the data sources and refreshes
func (m *DayModel) SetData(taskSvc service.TaskService, boards []kanbanmodels.Board, allNotes []notes.Note) {
	m.taskSvc = taskSvc
	m.boards = boards
	m.notes = allNotes
	m.refreshData()
}

// Init implements tea.Model
func (m DayModel) Init() tea.Cmd {
	return nil
}

// Update handles key events for the day view
func (m DayModel) Update(msg tea.Msg) (DayModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "h", "left":
			m.date = m.date.AddDate(0, 0, -1)
			m.refreshData()
		case "l", "right":
			m.date = m.date.AddDate(0, 0, 1)
			m.refreshData()
		case "t":
			m.date = time.Now()
			m.refreshData()
		case "j", "down":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if m.cursor < len(m.items) {
				item := m.items[m.cursor]
				switch item.Source {
				case agendapkg.SourceTask:
					if item.Task != nil {
						return m, func() tea.Msg {
							return messages.FocusTaskMsg{TaskID: item.Task.ID}
						}
					}
				case agendapkg.SourceCard:
					return m, func() tea.Msg {
						return messages.OpenBoardMsg{
							BoardPath: item.BoardPath,
							ColIndex:  item.ColIndex,
							CardIndex: item.CardIndex,
						}
					}
				}
			}
		}
	}

	return m, nil
}

// View renders the day agenda view
func (m DayModel) View() string {
	var sb strings.Builder

	// Title line
	dateStr := m.date.Format("Monday, Jan 2 2006")
	title := titleStyle.Render(fmt.Sprintf(" Agenda: %s", dateStr))
	nav := navHintStyle.Render("[h: prev] [t: today] [l: next]")

	titleLine := title
	padding := m.width - lipgloss.Width(title) - lipgloss.Width(nav) - 1
	if padding > 0 {
		titleLine += strings.Repeat(" ", padding) + nav
	}
	sb.WriteString(titleLine)
	sb.WriteString("\n\n")

	if len(m.items) == 0 {
		sb.WriteString(emptyStyle.Render("  No items scheduled for this day."))
		sb.WriteString("\n")
		return sb.String()
	}

	// Separate tasks, cards, and notes from buckets
	var allTasks, allCards, allNotes []agendapkg.AgendaItem
	for _, bucket := range m.buckets {
		allTasks = append(allTasks, bucket.Tasks...)
		allCards = append(allCards, bucket.Cards...)
		allNotes = append(allNotes, bucket.Notes...)
	}

	cursorIdx := 0

	// Tasks section
	if len(allTasks) > 0 {
		sb.WriteString(sectionStyle.Render(fmt.Sprintf(" Tasks (%d)", len(allTasks))))
		sb.WriteString("\n")
		for _, item := range allTasks {
			selected := cursorIdx == m.cursor
			line := RenderItemLine(item, selected, m.width-4)
			sb.WriteString("   ")
			sb.WriteString(line)
			sb.WriteString("\n")
			cursorIdx++
		}
		sb.WriteString("\n")
	}

	// Cards section
	if len(allCards) > 0 {
		sb.WriteString(sectionStyle.Render(fmt.Sprintf(" Cards (%d)", len(allCards))))
		sb.WriteString("\n")
		for _, item := range allCards {
			selected := cursorIdx == m.cursor
			line := RenderItemLine(item, selected, m.width-4)
			sb.WriteString("   ")
			sb.WriteString(line)
			sb.WriteString("\n")
			cursorIdx++
		}
		sb.WriteString("\n")
	}

	// Notes section
	if len(allNotes) > 0 {
		sb.WriteString(sectionStyle.Render(fmt.Sprintf(" Notes (%d)", len(allNotes))))
		sb.WriteString("\n")
		for _, item := range allNotes {
			selected := cursorIdx == m.cursor
			line := RenderItemLine(item, selected, m.width-4)
			sb.WriteString("   ")
			sb.WriteString(line)
			sb.WriteString("\n")
			cursorIdx++
		}
	}

	return sb.String()
}
