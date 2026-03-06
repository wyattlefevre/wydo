package projects

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"wydo/internal/tui/shared"
	"wydo/internal/workspace"
)

type dateEditorMode int

const (
	dateEditorNav       dateEditorMode = iota
	dateEditorEditLabel                // text input for label (add or edit)
	dateEditorPickDate                 // embedded date picker
)

// DateEditorModel is a list editor modal for managing a project's dates.
type DateEditorModel struct {
	dates         []workspace.ProjectDate
	cursor        int
	mode          dateEditorMode
	textInput     textinput.Model
	datePicker    *shared.DatePickerModel
	pendingLabel  string // label entered during the add flow (n)
	pickingForNew bool   // true when picker is for a new item (add flow)
	width         int
	height        int
}

// NewDateEditorModel creates a new date editor with the given dates.
func NewDateEditorModel(dates []workspace.ProjectDate) DateEditorModel {
	copied := make([]workspace.ProjectDate, len(dates))
	copy(copied, dates)
	return DateEditorModel{
		dates: copied,
		mode:  dateEditorNav,
	}
}

func (m *DateEditorModel) initTextInput(placeholder, value string) tea.Cmd {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 100
	ti.Width = 50
	ti.SetValue(value)
	ti.Focus()
	m.textInput = ti
	return textinput.Blink
}

// Update handles key events. Returns (model, cmd, saved, done).
func (m DateEditorModel) Update(msg tea.KeyMsg) (DateEditorModel, tea.Cmd, bool, bool) {
	switch m.mode {
	case dateEditorNav:
		return m.updateNav(msg)
	case dateEditorEditLabel:
		return m.updateEditLabel(msg)
	case dateEditorPickDate:
		return m.updatePickDate(msg)
	}
	return m, nil, false, false
}

func (m DateEditorModel) updateNav(msg tea.KeyMsg) (DateEditorModel, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.dates)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "n":
		m.mode = dateEditorEditLabel
		m.pendingLabel = ""
		m.pickingForNew = true
		cmd := m.initTextInput("Label (e.g. Code Complete)", "")
		return m, cmd, false, false
	case "d":
		if len(m.dates) > 0 && m.cursor < len(m.dates) {
			m.dates = append(m.dates[:m.cursor], m.dates[m.cursor+1:]...)
			if m.cursor >= len(m.dates) && m.cursor > 0 {
				m.cursor--
			}
		}
	case "e":
		if len(m.dates) > 0 && m.cursor < len(m.dates) {
			m.mode = dateEditorEditLabel
			m.pickingForNew = false
			cmd := m.initTextInput("Label", m.dates[m.cursor].Label)
			return m, cmd, false, false
		}
	case "D":
		if len(m.dates) > 0 && m.cursor < len(m.dates) {
			m.mode = dateEditorPickDate
			m.pickingForNew = false
			d := m.dates[m.cursor].Date
			dp := shared.NewDatePickerModel(&d, "Edit Date")
			dp.SetSize(m.width, m.height)
			m.datePicker = &dp
			return m, nil, false, false
		}
	case "enter":
		return m, nil, true, true
	case "esc":
		return m, nil, false, true
	}
	return m, nil, false, false
}

func (m DateEditorModel) updateEditLabel(msg tea.KeyMsg) (DateEditorModel, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "enter":
		label := strings.TrimSpace(m.textInput.Value())
		if m.pickingForNew {
			if label == "" {
				m.mode = dateEditorNav
				return m, nil, false, false
			}
			m.pendingLabel = label
			m.mode = dateEditorPickDate
			now := time.Now()
			dp := shared.NewDatePickerModel(&now, "Pick Date")
			dp.SetSize(m.width, m.height)
			m.datePicker = &dp
			return m, nil, false, false
		}
		// Editing existing label
		if m.cursor < len(m.dates) {
			m.dates[m.cursor].Label = label
		}
		m.mode = dateEditorNav
		return m, nil, false, false
	case "esc":
		m.mode = dateEditorNav
		return m, nil, false, false
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd, false, false
	}
}

func (m DateEditorModel) updatePickDate(msg tea.KeyMsg) (DateEditorModel, tea.Cmd, bool, bool) {
	if m.datePicker == nil {
		m.mode = dateEditorNav
		return m, nil, false, false
	}
	inTextInput := m.datePicker.IsTextInputActive()
	dp, cmd := m.datePicker.Update(msg)
	m.datePicker = &dp

	switch msg.String() {
	case "esc":
		if !inTextInput {
			// Cancel date picker — go back to nav without saving
			m.datePicker = nil
			m.mode = dateEditorNav
			return m, nil, false, false
		}
		// Picker was in text input mode — it handled esc (back to calendar)
		return m, cmd, false, false
	case "enter":
		if !inTextInput {
			// Confirm date selection
			selectedDate := m.datePicker.GetDate()
			m.datePicker = nil
			m.mode = dateEditorNav
			if selectedDate != nil {
				if m.pickingForNew {
					m.dates = append(m.dates, workspace.ProjectDate{
						Label: m.pendingLabel,
						Date:  *selectedDate,
					})
					m.cursor = len(m.dates) - 1
				} else if m.cursor < len(m.dates) {
					m.dates[m.cursor].Date = *selectedDate
				}
			}
			return m, nil, false, false
		}
		// Picker was in text input mode — it handled enter (parsed date, back to calendar)
		return m, cmd, false, false
	}
	return m, cmd, false, false
}

// GetDates returns the current list of dates.
func (m DateEditorModel) GetDates() []workspace.ProjectDate {
	return m.dates
}

// SetSize sets the display dimensions for centering the modal.
func (m *DateEditorModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	if m.datePicker != nil {
		m.datePicker.SetSize(w, h)
	}
}

// IsTyping returns true when a text input is active inside the editor.
func (m DateEditorModel) IsTyping() bool {
	return m.mode != dateEditorNav
}

// View renders the date editor modal.
func (m DateEditorModel) View() string {
	if m.mode == dateEditorPickDate && m.datePicker != nil {
		return m.datePicker.View()
	}

	var s strings.Builder

	title := dateEditorTitleStyle.Render("Edit Dates")
	s.WriteString(title)
	s.WriteString("\n\n")

	if m.mode == dateEditorEditLabel {
		var prompt string
		if m.pickingForNew {
			prompt = "Label:"
		} else {
			prompt = "Edit Label:"
		}
		s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render(prompt))
		s.WriteString("\n")
		s.WriteString(m.textInput.View())
		s.WriteString("\n\n")
		s.WriteString(dateEditorHelpStyle.Render("enter: confirm • esc: cancel"))
	} else {
		// Nav mode — show date list
		if len(m.dates) == 0 {
			s.WriteString(pathStyle.Render("  No dates"))
			s.WriteString("\n")
		} else {
			for i, d := range m.dates {
				prefix := "  "
				if i == m.cursor {
					prefix = "> "
				}
				label := d.Label
				if label == "" {
					label = "date"
				}
				dateStr := d.Date.Format("Jan 2 2006")
				var line string
				if i == m.cursor {
					line = selectedListItemStyle.Render(prefix+label) + "    " + pathStyle.Render(dateStr)
				} else {
					line = listItemStyle.Render(prefix+label) + "    " + pathStyle.Render(dateStr)
				}
				s.WriteString(line)
				s.WriteString("\n")
			}
		}
		s.WriteString("\n")
		s.WriteString(dateEditorHelpStyle.Render("n:add  d:delete  e:edit label  D:edit date  enter:save  esc:cancel"))
	}

	content := s.String()
	box := dateEditorBoxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
