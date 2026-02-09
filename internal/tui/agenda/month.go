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
	calDayHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8")).Width(5).Align(lipgloss.Center)
	calDayStyle        = lipgloss.NewStyle().Width(5).Align(lipgloss.Center)
	calTodayStyle      = lipgloss.NewStyle().Width(5).Align(lipgloss.Center).Bold(true).Foreground(lipgloss.Color("2"))
	calCursorStyle     = lipgloss.NewStyle().Width(5).Align(lipgloss.Center).Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("4"))
	calHasItemsStyle   = lipgloss.NewStyle().Width(5).Align(lipgloss.Center).Foreground(lipgloss.Color("3"))
	calEmptyStyle      = lipgloss.NewStyle().Width(5).Align(lipgloss.Center).Foreground(lipgloss.Color("8"))
	calMonthTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	detailHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
)

// MonthModel is the month agenda view with calendar grid
type MonthModel struct {
	viewMonth  time.Time // first of the month being viewed
	cursorDate time.Time // the day under cursor in the calendar
	buckets    []agendapkg.DateBucket
	bucketMap  map[string]*agendapkg.DateBucket
	taskSvc    service.TaskService
	boards     []kanbanmodels.Board
	notes      []notes.Note
	// Detail panel: items for the cursor day
	detailItems []agendapkg.AgendaItem
	detailIdx   int // cursor within detail panel
	inDetail    bool // true when navigating in the detail panel
	width       int
	height      int
}

// NewMonthModel creates a new month agenda view
func NewMonthModel(taskSvc service.TaskService, boards []kanbanmodels.Board, allNotes []notes.Note) MonthModel {
	now := time.Now()
	m := MonthModel{
		viewMonth:  time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local),
		cursorDate: now,
		taskSvc:    taskSvc,
		boards:     boards,
		notes:      allNotes,
	}
	m.refreshData()
	return m
}

func (m *MonthModel) refreshData() {
	dateRange := agendapkg.MonthRange(m.cursorDate)
	m.buckets = agendapkg.QueryAgenda(m.taskSvc, m.boards, m.notes, dateRange)

	m.bucketMap = make(map[string]*agendapkg.DateBucket)
	for i := range m.buckets {
		key := m.buckets[i].Date.Format("2006-01-02")
		b := m.buckets[i]
		m.bucketMap[key] = &b
	}

	m.refreshDetail()
}

func (m *MonthModel) refreshDetail() {
	key := m.cursorDate.Format("2006-01-02")
	if bucket, ok := m.bucketMap[key]; ok {
		m.detailItems = bucket.AllItems()
		m.detailItems = append(m.detailItems, bucket.AllCompletedItems()...)
	} else {
		m.detailItems = nil
	}
	if m.detailIdx >= len(m.detailItems) {
		m.detailIdx = max(0, len(m.detailItems)-1)
	}
}

// SetSize updates the view dimensions
func (m *MonthModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetData updates the data sources and refreshes
func (m *MonthModel) SetData(taskSvc service.TaskService, boards []kanbanmodels.Board, allNotes []notes.Note) {
	m.taskSvc = taskSvc
	m.boards = boards
	m.notes = allNotes
	m.refreshData()
}

// Update handles key events for the month view
func (m MonthModel) Update(msg tea.Msg) (MonthModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.inDetail {
			return m.updateDetail(msg)
		}
		return m.updateCalendar(msg)
	}
	return m, nil
}

func (m MonthModel) updateCalendar(msg tea.KeyMsg) (MonthModel, tea.Cmd) {
	switch msg.String() {
	case "h", "left":
		m.cursorDate = m.cursorDate.AddDate(0, 0, -1)
		m.ensureCursorInView()
		m.refreshDetail()
	case "l", "right":
		m.cursorDate = m.cursorDate.AddDate(0, 0, 1)
		m.ensureCursorInView()
		m.refreshDetail()
	case "k", "up":
		m.cursorDate = m.cursorDate.AddDate(0, 0, -7)
		m.ensureCursorInView()
		m.refreshDetail()
	case "j", "down":
		m.cursorDate = m.cursorDate.AddDate(0, 0, 7)
		m.ensureCursorInView()
		m.refreshDetail()
	case "H":
		// Previous month
		m.viewMonth = m.viewMonth.AddDate(0, -1, 0)
		m.cursorDate = m.viewMonth
		m.refreshData()
	case "L":
		// Next month
		m.viewMonth = m.viewMonth.AddDate(0, 1, 0)
		m.cursorDate = m.viewMonth
		m.refreshData()
	case "t":
		now := time.Now()
		m.cursorDate = now
		m.viewMonth = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		m.refreshData()
	case "enter":
		// Enter detail panel if there are items
		if len(m.detailItems) > 0 {
			m.inDetail = true
			m.detailIdx = 0
		}
	}
	return m, nil
}

func (m MonthModel) updateDetail(msg tea.KeyMsg) (MonthModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.detailIdx < len(m.detailItems)-1 {
			m.detailIdx++
		}
	case "k", "up":
		if m.detailIdx > 0 {
			m.detailIdx--
		}
	case "esc":
		m.inDetail = false
	case "enter":
		if m.detailIdx < len(m.detailItems) {
			item := m.detailItems[m.detailIdx]
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
	return m, nil
}

func (m *MonthModel) ensureCursorInView() {
	if m.cursorDate.Year() != m.viewMonth.Year() || m.cursorDate.Month() != m.viewMonth.Month() {
		m.viewMonth = time.Date(m.cursorDate.Year(), m.cursorDate.Month(), 1, 0, 0, 0, 0, time.Local)
		m.refreshData()
	}
}

// View renders the month agenda view
func (m MonthModel) View() string {
	var sb strings.Builder

	// Title line
	monthStr := m.viewMonth.Format("January 2006")
	title := calMonthTitleStyle.Render(fmt.Sprintf(" %s", monthStr))
	nav := navHintStyle.Render("[h/l: day] [k/j: week] [H/L: month] [t: today] [enter: detail]")

	titleLine := title
	padding := m.width - lipgloss.Width(title) - lipgloss.Width(nav) - 1
	if padding > 0 {
		titleLine += strings.Repeat(" ", padding) + nav
	}
	sb.WriteString(titleLine)
	sb.WriteString("\n\n")

	// Calendar grid
	sb.WriteString(m.renderCalendar())
	sb.WriteString("\n")

	// Detail panel for selected day
	sb.WriteString(m.renderDetailPanel())

	return sb.String()
}

func (m MonthModel) renderCalendar() string {
	var sb strings.Builder

	// Day headers (Su Mo Tu We Th Fr Sa)
	dayHeaders := []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}
	for _, d := range dayHeaders {
		sb.WriteString(calDayHeaderStyle.Render(d))
	}
	sb.WriteString("\n")

	firstDay := m.viewMonth
	startWeekday := int(firstDay.Weekday())
	lastDay := firstDay.AddDate(0, 1, -1)
	daysInMonth := lastDay.Day()
	today := time.Now()

	currentDay := 1 - startWeekday

	for week := 0; week < 6; week++ {
		for weekday := 0; weekday < 7; weekday++ {
			if currentDay < 1 || currentDay > daysInMonth {
				sb.WriteString(calEmptyStyle.Render(""))
			} else {
				date := time.Date(m.viewMonth.Year(), m.viewMonth.Month(), currentDay, 0, 0, 0, 0, time.Local)
				isCursor := isSameDay(date, m.cursorDate)
				isToday := isSameDay(date, today)

				key := date.Format("2006-01-02")
				count := 0
				if bucket, ok := m.bucketMap[key]; ok {
					count = bucket.TotalCount()
				}

				dayStr := fmt.Sprintf("%2d", currentDay)
				if count > 0 {
					dayStr = fmt.Sprintf("%2d*", currentDay)
				}

				switch {
				case isCursor:
					sb.WriteString(calCursorStyle.Render(dayStr))
				case isToday:
					sb.WriteString(calTodayStyle.Render(dayStr))
				case count > 0:
					sb.WriteString(calHasItemsStyle.Render(dayStr))
				default:
					sb.WriteString(calDayStyle.Render(dayStr))
				}
			}
			currentDay++
		}
		sb.WriteString("\n")

		if currentDay > daysInMonth {
			break
		}
	}

	return sb.String()
}

func (m MonthModel) renderDetailPanel() string {
	var sb strings.Builder

	dateStr := m.cursorDate.Format("Mon, Jan 2")
	header := detailHeaderStyle.Render(fmt.Sprintf(" %s", dateStr))

	if len(m.detailItems) == 0 {
		sb.WriteString(header)
		sb.WriteString("  ")
		sb.WriteString(emptyStyle.Render("No items"))
		sb.WriteString("\n")
		return sb.String()
	}

	countStr := weekCountStyle.Render(fmt.Sprintf("(%d items)", len(m.detailItems)))
	sb.WriteString(header + " " + countStr)
	if m.inDetail {
		sb.WriteString("  " + navHintStyle.Render("[j/k: navigate] [enter: open] [esc: back]"))
	}
	sb.WriteString("\n")

	for i, item := range m.detailItems {
		selected := m.inDetail && i == m.detailIdx
		line := RenderItemLine(item, selected, m.width-6)
		sb.WriteString("     ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

func isSameDay(d1, d2 time.Time) bool {
	return d1.Year() == d2.Year() && d1.Month() == d2.Month() && d1.Day() == d2.Day()
}
