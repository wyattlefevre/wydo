package kanban

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type datePickerMode int

const (
	calendarMode datePickerMode = iota
	textInputMode
)

type DatePickerModel struct {
	mode        datePickerMode
	currentDate *time.Time // The date being edited (nil if none)
	cursorDate  time.Time  // The date under cursor in calendar
	viewMonth   time.Time  // The month being viewed
	textInput   textinput.Model
	width       int
	height      int
	title       string // "Due Date" or "Scheduled Date"
}

func NewDatePickerModel(currentDate *time.Time, title string) DatePickerModel {
	now := time.Now()

	// Initialize cursor date
	cursorDate := now
	if currentDate != nil {
		cursorDate = *currentDate
	}

	// Initialize view month to cursor date's month
	viewMonth := time.Date(cursorDate.Year(), cursorDate.Month(), 1, 0, 0, 0, 0, time.Local)

	// Initialize text input
	ti := textinput.New()
	ti.Placeholder = "2026-03-15, +5, tomorrow"
	ti.Focus()
	ti.CharLimit = 20
	ti.Width = 30
	if currentDate != nil {
		ti.SetValue(currentDate.Format("2006-01-02"))
	}

	return DatePickerModel{
		mode:        calendarMode,
		currentDate: currentDate,
		cursorDate:  cursorDate,
		viewMonth:   viewMonth,
		textInput:   ti,
		title:       title,
	}
}

func (m DatePickerModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m DatePickerModel) Update(msg tea.KeyMsg) (DatePickerModel, tea.Cmd) {
	if m.mode == textInputMode {
		return m.updateTextInput(msg)
	}
	return m.updateCalendar(msg)
}

func (m DatePickerModel) updateCalendar(msg tea.KeyMsg) (DatePickerModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel
		return m, nil
	case "enter":
		// Confirm selection
		m.currentDate = &m.cursorDate
		return m, nil
	case "i":
		// Switch to text input mode
		m.mode = textInputMode
		return m, textinput.Blink
	case "c":
		// Clear date
		m.currentDate = nil
		return m, nil
	case "t":
		// Jump to today
		today := time.Now()
		m.cursorDate = today
		m.viewMonth = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.Local)
		return m, nil
	case "h", "left":
		// Move cursor left (previous day)
		m.cursorDate = m.cursorDate.AddDate(0, 0, -1)
		m.ensureCursorInView()
		return m, nil
	case "l", "right":
		// Move cursor right (next day)
		m.cursorDate = m.cursorDate.AddDate(0, 0, 1)
		m.ensureCursorInView()
		return m, nil
	case "k", "up":
		// Move cursor up (previous week)
		m.cursorDate = m.cursorDate.AddDate(0, 0, -7)
		m.ensureCursorInView()
		return m, nil
	case "j", "down":
		// Move cursor down (next week)
		m.cursorDate = m.cursorDate.AddDate(0, 0, 7)
		m.ensureCursorInView()
		return m, nil
	case "-", "H":
		// Previous month
		m.viewMonth = m.viewMonth.AddDate(0, -1, 0)
		return m, nil
	case "+", "=", "L":
		// Next month
		m.viewMonth = m.viewMonth.AddDate(0, 1, 0)
		return m, nil
	}

	return m, nil
}

func (m DatePickerModel) updateTextInput(msg tea.KeyMsg) (DatePickerModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		// Go back to calendar mode or cancel
		if m.mode == textInputMode {
			m.mode = calendarMode
			return m, nil
		}
		return m, nil
	case "enter":
		// Parse text input
		parsed, err := m.parseTextInput(m.textInput.Value())
		if err == nil {
			m.currentDate = &parsed
			m.cursorDate = parsed
			m.ensureCursorInView()
		}
		return m, nil
	default:
		m.textInput, cmd = m.textInput.Update(msg)
	}

	return m, cmd
}

func (m *DatePickerModel) ensureCursorInView() {
	// If cursor is in a different month than view, update view
	if m.cursorDate.Year() != m.viewMonth.Year() || m.cursorDate.Month() != m.viewMonth.Month() {
		m.viewMonth = time.Date(m.cursorDate.Year(), m.cursorDate.Month(), 1, 0, 0, 0, 0, time.Local)
	}
}

func (m DatePickerModel) parseTextInput(input string) (time.Time, error) {
	input = strings.TrimSpace(input)

	// Handle relative dates
	if strings.HasPrefix(input, "+") {
		days, err := strconv.Atoi(input[1:])
		if err != nil {
			return time.Time{}, err
		}
		return time.Now().AddDate(0, 0, days), nil
	}

	if strings.HasPrefix(input, "-") {
		days, err := strconv.Atoi(input[1:])
		if err != nil {
			return time.Time{}, err
		}
		return time.Now().AddDate(0, 0, -days), nil
	}

	// Handle "tomorrow"
	if strings.ToLower(input) == "tomorrow" {
		return time.Now().AddDate(0, 0, 1), nil
	}

	// Handle "today"
	if strings.ToLower(input) == "today" {
		return time.Now(), nil
	}

	// Try full date format: 2026-03-15
	if parsed, err := time.Parse("2006-01-02", input); err == nil {
		return parsed, nil
	}

	// Try short format: 03-15 (assumes current year)
	now := time.Now()
	if parsed, err := time.Parse("01-02", input); err == nil {
		return time.Date(now.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.Local), nil
	}

	return time.Time{}, fmt.Errorf("invalid date format")
}

func (m DatePickerModel) View() string {
	if m.mode == textInputMode {
		return m.viewTextInput()
	}
	return m.viewCalendar()
}

func (m DatePickerModel) viewCalendar() string {
	var s strings.Builder

	// Title
	title := datePickerTitleStyle.Render(m.title)
	s.WriteString(title)
	s.WriteString("\n\n")

	// Month/Year header
	monthYear := datePickerMonthStyle.Render(m.viewMonth.Format("January 2006"))
	s.WriteString(monthYear)
	s.WriteString("\n\n")

	// Day headers
	dayHeaders := []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}
	for _, day := range dayHeaders {
		s.WriteString(datePickerDayHeaderStyle.Render(day))
		s.WriteString(" ")
	}
	s.WriteString("\n")

	// Calendar grid
	firstDay := m.viewMonth
	startWeekday := int(firstDay.Weekday())

	// Get days in month
	lastDay := firstDay.AddDate(0, 1, -1)
	daysInMonth := lastDay.Day()

	// Start with offset for first day of month
	currentDay := 1 - startWeekday
	today := time.Now()

	for week := 0; week < 6; week++ {
		for weekday := 0; weekday < 7; weekday++ {
			if currentDay < 1 || currentDay > daysInMonth {
				// Empty cell (2 spaces to match day width)
				s.WriteString("  ")
			} else {
				date := time.Date(m.viewMonth.Year(), m.viewMonth.Month(), currentDay, 0, 0, 0, 0, time.Local)
				dayStr := fmt.Sprintf("%2d", currentDay)

				// Apply styles based on date
				isCursor := m.isSameDay(date, m.cursorDate)
				isToday := m.isSameDay(date, today)

				if isCursor {
					s.WriteString(datePickerCursorStyle.Render(dayStr))
				} else if isToday {
					s.WriteString(datePickerTodayStyle.Render(dayStr))
				} else {
					s.WriteString(datePickerDayStyle.Render(dayStr))
				}
			}
			s.WriteString(" ")
			currentDay++
		}
		s.WriteString("\n")

		// Stop if we've rendered all days
		if currentDay > daysInMonth {
			break
		}
	}

	s.WriteString("\n")

	// Help text
	help := helpStyle.Render("hjkl/arrows: move • t: today • +/-: month • c: clear • i: text input • enter: save • esc: cancel")
	s.WriteString(help)

	content := s.String()
	box := datePickerBoxStyle.Render(content)

	// Center the box
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m DatePickerModel) viewTextInput() string {
	var s strings.Builder

	// Title
	title := datePickerTitleStyle.Render(m.title + " (Text Input)")
	s.WriteString(title)
	s.WriteString("\n\n")

	s.WriteString(m.textInput.View())
	s.WriteString("\n\n")

	// Help text
	help := helpStyle.Render("enter: save • esc: back to calendar")
	s.WriteString(help)
	s.WriteString("\n\n")

	// Format examples
	examples := datePickerExamplesStyle.Render("Examples: 2026-03-15, 03-15, +5, tomorrow, today")
	s.WriteString(examples)

	content := s.String()
	box := datePickerBoxStyle.Render(content)

	// Center the box
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m DatePickerModel) isSameDay(d1, d2 time.Time) bool {
	return d1.Year() == d2.Year() && d1.Month() == d2.Month() && d1.Day() == d2.Day()
}

func (m DatePickerModel) GetDate() *time.Time {
	return m.currentDate
}

func (m *DatePickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}
