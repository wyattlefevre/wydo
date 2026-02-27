package agenda

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	agendapkg "wydo/internal/agenda"
	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/notes"
	"wydo/internal/tasks/service"
	"wydo/internal/tui/messages"
	"wydo/internal/tui/shared"
	"wydo/internal/tui/theme"
)

// DayModel is the day agenda view
type DayModel struct {
	date         time.Time
	buckets      []agendapkg.DateBucket
	overdueItems []agendapkg.AgendaItem
	taskSvc      service.TaskService
	boards       []kanbanmodels.Board
	notes        []notes.Note
	allItems     []agendapkg.AgendaItem // all flattened items before filtering
	items        []agendapkg.AgendaItem // flattened items for cursor navigation (after filter)
	cursor       int
	width        int
	height       int

	// Search state
	searchActive     bool
	searchFilterMode bool
	searchInput      textinput.Model
	searchQuery      string
}

// NewDayModel creates a new day agenda view
func NewDayModel(taskSvc service.TaskService, boards []kanbanmodels.Board, allNotes []notes.Note) DayModel {
	si := textinput.New()
	si.Placeholder = "Search..."
	si.CharLimit = 100
	si.Width = 40

	m := DayModel{
		date:        time.Now(),
		taskSvc:     taskSvc,
		boards:      boards,
		notes:       allNotes,
		searchInput: si,
	}
	m.refreshData()
	return m
}

func (m *DayModel) refreshData() {
	dateRange := agendapkg.DayRange(m.date)
	m.buckets = agendapkg.QueryAgenda(m.taskSvc, m.boards, m.notes, dateRange)
	m.overdueItems = agendapkg.QueryOverdueItems(m.taskSvc, m.boards, dateRange.Start)

	// Flatten all items: overdue first, then regular, then completed
	m.allItems = nil
	m.allItems = append(m.allItems, m.overdueItems...)
	for _, bucket := range m.buckets {
		m.allItems = append(m.allItems, bucket.AllItems()...)
	}
	for _, bucket := range m.buckets {
		m.allItems = append(m.allItems, bucket.AllCompletedItems()...)
	}

	// Apply search filter
	m.applySearchFilter()
}

func (m *DayModel) applySearchFilter() {
	if m.searchQuery == "" {
		m.items = m.allItems
	} else {
		names := make([]string, len(m.allItems))
		for i, item := range m.allItems {
			names[i] = AgendaSearchString(item)
		}
		matches := fuzzy.Find(m.searchQuery, names)
		m.items = make([]agendapkg.AgendaItem, len(matches))
		for i, match := range matches {
			m.items[i] = m.allItems[match.Index]
		}
	}

	// Clamp cursor
	if m.cursor >= len(m.items) {
		m.cursor = max(0, len(m.items)-1)
	}
}

// IsSearching returns true when in search mode
func (m DayModel) IsSearching() bool {
	return m.searchActive
}

// HintText returns hint text for the current state
func (m DayModel) HintText() string {
	if m.searchActive {
		if m.searchFilterMode {
			return "type to filter  enter:confirm  esc:exit"
		}
		return "/:edit filter  j/k:navigate  enter:open  esc:exit"
	}
	return ""
}

// SetSize updates the view dimensions
func (m *DayModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetData updates the data sources and refreshes
func (m *DayModel) SetData(taskSvc service.TaskService, boards []kanbanmodels.Board, allNotes []notes.Note) {
	m.date = time.Now()
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
			return m.openSelectedItem()
		}
	}

	return m, nil
}

func (m DayModel) openSelectedItem() (DayModel, tea.Cmd) {
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
	return m, nil
}

func (m DayModel) handleSearchMode(msg tea.KeyMsg) (DayModel, tea.Cmd) {
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
		if m.cursor < len(m.items)-1 {
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

// View renders the day agenda view
func (m DayModel) View() string {
	var sb strings.Builder

	// Title line
	dateStr := m.date.Format("Monday, Jan 2 2006")
	title := titleStyle.Render(fmt.Sprintf(" Agenda: %s", dateStr))
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

	if len(m.items) == 0 {
		if m.searchQuery != "" {
			sb.WriteString(emptyStyle.Render("  No matching items."))
		} else {
			sb.WriteString(emptyStyle.Render("  No items scheduled for this day."))
		}
		sb.WriteString("\n")
	} else if m.searchQuery != "" {
		// Filtered flat list — render items directly from m.items
		sb.WriteString(sectionStyle.Render(fmt.Sprintf(" Results (%d)", len(m.items))))
		sb.WriteString("\n")
		for i, item := range m.items {
			selected := i == m.cursor
			line := RenderItemLine(item, selected, m.width-4)
			sb.WriteString("   ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	} else {
		// Separate tasks, cards, notes, and completed items from buckets
		var allTasks, allCards, allNotes, allCompleted []agendapkg.AgendaItem
		for _, bucket := range m.buckets {
			allTasks = append(allTasks, bucket.Tasks...)
			allCards = append(allCards, bucket.Cards...)
			allNotes = append(allNotes, bucket.Notes...)
			allCompleted = append(allCompleted, bucket.AllCompletedItems()...)
		}

		cursorIdx := 0

		// Overdue section
		if len(m.overdueItems) > 0 {
			sb.WriteString(overdueHeaderStyle.Render(fmt.Sprintf(" Overdue (%d)", len(m.overdueItems))))
			sb.WriteString("\n")
			for _, item := range m.overdueItems {
				selected := cursorIdx == m.cursor
				line := RenderItemLine(item, selected, m.width-4)
				sb.WriteString("   ")
				sb.WriteString(line)
				sb.WriteString("\n")
				cursorIdx++
			}
			sb.WriteString("\n")
		}

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
			sb.WriteString("\n")
		}

		// Completed section
		if len(allCompleted) > 0 {
			header := lipgloss.NewStyle().Foreground(theme.TextMuted).Bold(true).Render(fmt.Sprintf(" Completed (%d)", len(allCompleted)))
			sb.WriteString(header)
			sb.WriteString("\n")
			for _, item := range allCompleted {
				selected := cursorIdx == m.cursor
				line := RenderItemLine(item, selected, m.width-4)
				sb.WriteString("   ")
				sb.WriteString(line)
				sb.WriteString("\n")
				cursorIdx++
			}
		}
	}

	return shared.CenterContent(sb.String(), m.height)
}
