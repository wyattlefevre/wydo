package agenda

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
	agendapkg "wydo/internal/agenda"
	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/notes"
	"wydo/internal/tasks/service"
	"wydo/internal/tui/messages"
	"wydo/internal/tui/shared"
)

// WeekModel is the week agenda view
type WeekModel struct {
	date         time.Time // any date in the week being viewed
	buckets      []agendapkg.DateBucket
	overdueItems []agendapkg.AgendaItem
	unfilteredItems []agendapkg.AgendaItem // all flattened items before filtering
	allItems     []agendapkg.AgendaItem    // flattened items across all days (after filter)
	taskSvc      service.TaskService
	boards       []kanbanmodels.Board
	notes        []notes.Note
	cursor       int
	width        int
	height       int

	// Search state
	searchActive     bool
	searchFilterMode bool
	searchInput      textinput.Model
	searchQuery      string
}

// NewWeekModel creates a new week agenda view
func NewWeekModel(taskSvc service.TaskService, boards []kanbanmodels.Board, allNotes []notes.Note) WeekModel {
	si := textinput.New()
	si.Placeholder = "Search..."
	si.CharLimit = 100
	si.Width = 40

	m := WeekModel{
		date:        time.Now(),
		taskSvc:     taskSvc,
		boards:      boards,
		notes:       allNotes,
		searchInput: si,
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
	m.unfilteredItems = nil
	m.unfilteredItems = append(m.unfilteredItems, m.overdueItems...)

	start := agendapkg.WeekRange(m.date).Start
	for d := 0; d < 7; d++ {
		day := start.AddDate(0, 0, d)
		key := day.Format("2006-01-02")
		if bucket, ok := bucketMap[key]; ok {
			m.unfilteredItems = append(m.unfilteredItems, bucket.AllItems()...)
			m.unfilteredItems = append(m.unfilteredItems, bucket.AllCompletedItems()...)
		}
	}

	m.applySearchFilter()
}

func (m *WeekModel) applySearchFilter() {
	if m.searchQuery == "" {
		m.allItems = m.unfilteredItems
	} else {
		names := make([]string, len(m.unfilteredItems))
		for i, item := range m.unfilteredItems {
			names[i] = AgendaSearchString(item)
		}
		matches := fuzzy.Find(m.searchQuery, names)
		m.allItems = make([]agendapkg.AgendaItem, len(matches))
		for i, match := range matches {
			m.allItems[i] = m.unfilteredItems[match.Index]
		}
	}

	// Clamp cursor
	if m.cursor >= len(m.allItems) {
		m.cursor = max(0, len(m.allItems)-1)
	}
}

// IsSearching returns true when in search mode
func (m WeekModel) IsSearching() bool {
	return m.searchActive
}

// HintText returns hint text for the current state
func (m WeekModel) HintText() string {
	if m.searchActive {
		if m.searchFilterMode {
			return "type to filter  enter:confirm  esc:exit"
		}
		return "/:edit filter  j/k:navigate  enter:open  esc:exit"
	}
	return ""
}

// SetSize updates the view dimensions
func (m *WeekModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetData updates the data sources and refreshes
func (m *WeekModel) SetData(taskSvc service.TaskService, boards []kanbanmodels.Board, allNotes []notes.Note) {
	m.date = time.Now()
	m.taskSvc = taskSvc
	m.boards = boards
	m.notes = allNotes
	m.refreshData()
}

// Update handles key events for the week view
func (m WeekModel) Update(msg tea.Msg) (WeekModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.searchActive {
			return m.handleSearchMode(msg)
		}

		switch msg.String() {
		case "/":
			m.searchActive = true
			m.searchFilterMode = true
			m.searchInput.SetValue(m.searchQuery)
			return m, m.searchInput.Focus()
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
			return m.openSelectedItem()
		}
	}

	return m, nil
}

func (m WeekModel) openSelectedItem() (WeekModel, tea.Cmd) {
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
	return m, nil
}

func (m WeekModel) handleSearchMode(msg tea.KeyMsg) (WeekModel, tea.Cmd) {
	if m.searchFilterMode {
		switch msg.String() {
		case "enter":
			m.searchFilterMode = false
			m.searchInput.Blur()
			return m, nil
		case "esc":
			m.searchInput.SetValue("")
			m.searchQuery = ""
			m.searchActive = false
			m.searchFilterMode = false
			m.searchInput.Blur()
			m.applySearchFilter()
			return m, nil
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.searchQuery = m.searchInput.Value()
			m.applySearchFilter()
			return m, cmd
		}
	}

	// Navigation mode
	switch msg.String() {
	case "/":
		m.searchFilterMode = true
		return m, m.searchInput.Focus()
	case "enter":
		return m.openSelectedItem()
	case "esc":
		m.searchInput.SetValue("")
		m.searchQuery = ""
		m.searchActive = false
		m.searchFilterMode = false
		m.applySearchFilter()
		return m, nil
	case "j", "down":
		if m.cursor < len(m.allItems)-1 {
			m.cursor++
		}
		return m, nil
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
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
	sb.WriteString(title)
	sb.WriteString("\n")

	if m.searchActive {
		if m.searchFilterMode {
			sb.WriteString("  " + m.searchInput.View())
		} else if m.searchQuery != "" {
			sb.WriteString("  " + searchLabelStyle.Render("Filter: ") + m.searchQuery)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	if len(m.allItems) == 0 {
		if m.searchQuery != "" {
			sb.WriteString(emptyStyle.Render("  No matching items."))
		} else {
			sb.WriteString(emptyStyle.Render("  No items scheduled for this week."))
		}
		sb.WriteString("\n")
	} else if m.searchQuery != "" {
		// Filtered flat list — render items directly from m.allItems
		sb.WriteString(sectionStyle.Render(fmt.Sprintf(" Results (%d)", len(m.allItems))))
		sb.WriteString("\n")
		for i, item := range m.allItems {
			selected := i == m.cursor
			line := RenderItemLine(item, selected, m.width-6)
			sb.WriteString("     ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	} else {
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
	}

	return shared.CenterContent(sb.String(), m.height)
}
