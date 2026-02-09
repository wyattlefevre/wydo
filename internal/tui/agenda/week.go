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
	weekDayHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	weekTodayStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	weekCountStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// WeekModel is the week agenda view
type WeekModel struct {
	date         time.Time // any date in the week being viewed
	buckets      []agendapkg.DateBucket
	overdueItems []agendapkg.AgendaItem
	allItems     []agendapkg.AgendaItem // flattened items across all days
	dayOffsets   []int                   // index into allItems where each day starts
	taskSvc      service.TaskService
	boards       []kanbanmodels.Board
	notes        []notes.Note
	cursor       int
	width        int
	height       int
}

// NewWeekModel creates a new week agenda view
func NewWeekModel(taskSvc service.TaskService, boards []kanbanmodels.Board, allNotes []notes.Note) WeekModel {
	m := WeekModel{
		date:    time.Now(),
		taskSvc: taskSvc,
		boards:  boards,
		notes:   allNotes,
	}
	m.refreshData()
	return m
}

func (m *WeekModel) refreshData() {
	dateRange := agendapkg.WeekRange(m.date)
	m.buckets = agendapkg.QueryAgenda(m.taskSvc, m.boards, m.notes, dateRange)
	m.overdueItems = agendapkg.QueryOverdueItems(m.taskSvc, m.boards, dateRange.Start)

	// Build a map of date -> bucket for fast lookup
	bucketMap := make(map[string]*agendapkg.DateBucket)
	for i := range m.buckets {
		key := m.buckets[i].Date.Format("2006-01-02")
		m.buckets[i] = m.buckets[i] // ensure copy
		bucketMap[key] = &m.buckets[i]
	}

	// Build flattened item list: overdue first, then day offsets for the 7 days
	m.allItems = nil
	m.allItems = append(m.allItems, m.overdueItems...)

	m.dayOffsets = make([]int, 7)
	start := agendapkg.WeekRange(m.date).Start

	for d := 0; d < 7; d++ {
		day := start.AddDate(0, 0, d)
		key := day.Format("2006-01-02")
		m.dayOffsets[d] = len(m.allItems)
		if bucket, ok := bucketMap[key]; ok {
			m.allItems = append(m.allItems, bucket.AllItems()...)
			m.allItems = append(m.allItems, bucket.AllCompletedItems()...)
		}
	}

	// Clamp cursor
	if m.cursor >= len(m.allItems) {
		m.cursor = max(0, len(m.allItems)-1)
	}
}

// SetSize updates the view dimensions
func (m *WeekModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetData updates the data sources and refreshes
func (m *WeekModel) SetData(taskSvc service.TaskService, boards []kanbanmodels.Board, allNotes []notes.Note) {
	m.taskSvc = taskSvc
	m.boards = boards
	m.notes = allNotes
	m.refreshData()
}

// Update handles key events for the week view
func (m WeekModel) Update(msg tea.Msg) (WeekModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "h", "left":
			m.date = m.date.AddDate(0, 0, -7)
			m.refreshData()
		case "l", "right":
			m.date = m.date.AddDate(0, 0, 7)
			m.refreshData()
		case "t":
			m.date = time.Now()
			m.refreshData()
		case "j", "down":
			if m.cursor < len(m.allItems)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if m.cursor < len(m.allItems) {
				item := m.allItems[m.cursor]
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

// View renders the week agenda view
func (m WeekModel) View() string {
	var sb strings.Builder

	weekRange := agendapkg.WeekRange(m.date)
	start := weekRange.Start
	end := start.AddDate(0, 0, 6)

	// Title line
	titleStr := fmt.Sprintf(" Week: %s - %s", start.Format("Jan 2"), end.Format("Jan 2 2006"))
	title := titleStyle.Render(titleStr)
	nav := navHintStyle.Render("[h: prev] [t: today] [l: next]")

	titleLine := title
	padding := m.width - lipgloss.Width(title) - lipgloss.Width(nav) - 1
	if padding > 0 {
		titleLine += strings.Repeat(" ", padding) + nav
	}
	sb.WriteString(titleLine)
	sb.WriteString("\n\n")

	if len(m.allItems) == 0 {
		sb.WriteString(emptyStyle.Render("  No items scheduled for this week."))
		sb.WriteString("\n")
		return sb.String()
	}

	// Build a map of date -> bucket for rendering
	bucketMap := make(map[string]*agendapkg.DateBucket)
	for i := range m.buckets {
		key := m.buckets[i].Date.Format("2006-01-02")
		b := m.buckets[i]
		bucketMap[key] = &b
	}

	today := time.Now()
	globalIdx := 0

	// Overdue section
	if len(m.overdueItems) > 0 {
		sb.WriteString(overdueHeaderStyle.Render(fmt.Sprintf(" Overdue (%d)", len(m.overdueItems))))
		sb.WriteString("\n")
		for _, item := range m.overdueItems {
			selected := globalIdx == m.cursor
			line := RenderItemLine(item, selected, m.width-6)
			sb.WriteString("     ")
			sb.WriteString(line)
			sb.WriteString("\n")
			globalIdx++
		}
		sb.WriteString("\n")
	}

	for d := 0; d < 7; d++ {
		day := start.AddDate(0, 0, d)
		key := day.Format("2006-01-02")
		isToday := day.Year() == today.Year() && day.Month() == today.Month() && day.Day() == today.Day()

		bucket := bucketMap[key]
		count := 0
		if bucket != nil {
			count = bucket.TotalCount()
		}

		// Day header
		dayName := day.Format("Mon Jan 2")
		countStr := weekCountStyle.Render(fmt.Sprintf("(%d)", count))

		if isToday {
			sb.WriteString(weekTodayStyle.Render(" " + dayName + " (today)"))
		} else {
			sb.WriteString(weekDayHeaderStyle.Render(" " + dayName))
		}
		if count > 0 {
			sb.WriteString(" " + countStr)
		}
		sb.WriteString("\n")

		// Render items for this day
		if bucket != nil {
			items := bucket.AllItems()
			for _, item := range items {
				selected := globalIdx == m.cursor
				line := RenderItemLine(item, selected, m.width-6)
				sb.WriteString("     ")
				sb.WriteString(line)
				sb.WriteString("\n")
				globalIdx++
			}
		}

		// Add spacing between days unless it's the last
		if d < 6 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
