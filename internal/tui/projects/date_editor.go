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
	dateEditorNav           dateEditorMode = iota
	dateEditorEditLabel                    // text input for label (add or edit)
	dateEditorPickDate                     // embedded date picker
	dateEditorSelectProject                // project selector when adding to multi-project editor
)

// dateItem is a single date entry with its owning project name.
type dateItem struct {
	projectName string
	date        workspace.ProjectDate
}

// DateEditorModel is a list editor modal for managing a project's dates.
type DateEditorModel struct {
	items          []dateItem
	cursor         int
	mode           dateEditorMode
	textInput      textinput.Model
	datePicker     *shared.DatePickerModel
	pendingLabel   string   // label entered during the add flow (n)
	pendingProject string   // project chosen in selectProject step
	pickingForNew  bool     // true when picker is for a new item (add flow)
	projectNames   []string // root first, then sub-projects; empty = single-project mode
	projectCursor  int
	width          int
	height         int
}

// NewDateEditorModel creates a new date editor for a single project.
func NewDateEditorModel(dates []workspace.ProjectDate) DateEditorModel {
	items := make([]dateItem, len(dates))
	for i, d := range dates {
		items[i] = dateItem{date: d}
	}
	return DateEditorModel{
		items: items,
		mode:  dateEditorNav,
	}
}

// NewDateEditorModelWithProjects creates a date editor showing dates across root and sub-projects.
// projectNames has the root project first; existingSubDates maps sub-project name → current dates.
func NewDateEditorModelWithProjects(rootDates []workspace.ProjectDate, projectNames []string, existingSubDates map[string][]workspace.ProjectDate) DateEditorModel {
	var items []dateItem
	rootName := ""
	if len(projectNames) > 0 {
		rootName = projectNames[0]
	}
	for _, d := range rootDates {
		items = append(items, dateItem{projectName: rootName, date: d})
	}
	for _, name := range projectNames[1:] {
		for _, d := range existingSubDates[name] {
			items = append(items, dateItem{projectName: name, date: d})
		}
	}
	names := make([]string, len(projectNames))
	copy(names, projectNames)
	return DateEditorModel{
		items:        items,
		mode:         dateEditorNav,
		projectNames: names,
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
	case dateEditorSelectProject:
		return m.updateSelectProject(msg)
	}
	return m, nil, false, false
}

func (m DateEditorModel) updateNav(msg tea.KeyMsg) (DateEditorModel, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.items)-1 {
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
		if len(m.items) > 0 && m.cursor < len(m.items) {
			m.items = append(m.items[:m.cursor], m.items[m.cursor+1:]...)
			if m.cursor >= len(m.items) && m.cursor > 0 {
				m.cursor--
			}
		}
	case "e":
		if len(m.items) > 0 && m.cursor < len(m.items) {
			m.mode = dateEditorEditLabel
			m.pickingForNew = false
			cmd := m.initTextInput("Label", m.items[m.cursor].date.Label)
			return m, cmd, false, false
		}
	case "D":
		if len(m.items) > 0 && m.cursor < len(m.items) {
			m.mode = dateEditorPickDate
			m.pickingForNew = false
			d := m.items[m.cursor].date.Date
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
			if len(m.projectNames) > 1 {
				// Ask which project to add to
				m.projectCursor = 0
				m.mode = dateEditorSelectProject
				return m, nil, false, false
			}
			// Single project: go straight to date picker
			rootName := ""
			if len(m.projectNames) > 0 {
				rootName = m.projectNames[0]
			}
			m.pendingProject = rootName
			m.mode = dateEditorPickDate
			now := time.Now()
			dp := shared.NewDatePickerModel(&now, "Pick Date")
			dp.SetSize(m.width, m.height)
			m.datePicker = &dp
			return m, nil, false, false
		}
		// Editing existing label
		if m.cursor < len(m.items) {
			m.items[m.cursor].date.Label = label
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

func (m DateEditorModel) updateSelectProject(msg tea.KeyMsg) (DateEditorModel, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "j", "down":
		if m.projectCursor < len(m.projectNames)-1 {
			m.projectCursor++
		}
	case "k", "up":
		if m.projectCursor > 0 {
			m.projectCursor--
		}
	case "enter":
		m.pendingProject = m.projectNames[m.projectCursor]
		m.mode = dateEditorPickDate
		now := time.Now()
		dp := shared.NewDatePickerModel(&now, "Pick Date")
		dp.SetSize(m.width, m.height)
		m.datePicker = &dp
		return m, nil, false, false
	case "esc":
		m.pendingLabel = ""
		m.pendingProject = ""
		m.mode = dateEditorNav
	}
	return m, nil, false, false
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
			m.datePicker = nil
			m.mode = dateEditorNav
			return m, nil, false, false
		}
		return m, cmd, false, false
	case "enter":
		if !inTextInput {
			selectedDate := m.datePicker.GetDate()
			m.datePicker = nil
			m.mode = dateEditorNav
			if selectedDate != nil {
				if m.pickingForNew {
					m.insertIntoProject(m.pendingProject, workspace.ProjectDate{
						Label: m.pendingLabel,
						Date:  *selectedDate,
					})
					m.pendingLabel = ""
					m.pendingProject = ""
				} else if m.cursor < len(m.items) {
					m.items[m.cursor].date.Date = *selectedDate
				}
			}
			return m, nil, false, false
		}
		return m, cmd, false, false
	}
	return m, cmd, false, false
}

// insertIntoProject appends a date after the last existing item for the given project
// (or at the end if the project has no items yet), then sets cursor to the new item.
func (m *DateEditorModel) insertIntoProject(projectName string, date workspace.ProjectDate) {
	item := dateItem{projectName: projectName, date: date}
	insertIdx := len(m.items)
	for i := len(m.items) - 1; i >= 0; i-- {
		if m.items[i].projectName == projectName {
			insertIdx = i + 1
			break
		}
	}
	m.items = append(m.items, dateItem{})
	copy(m.items[insertIdx+1:], m.items[insertIdx:])
	m.items[insertIdx] = item
	m.cursor = insertIdx
}

// GetDates returns the dates belonging to the root project (or all dates for single-project use).
func (m DateEditorModel) GetDates() []workspace.ProjectDate {
	rootName := ""
	if len(m.projectNames) > 0 {
		rootName = m.projectNames[0]
	}
	var result []workspace.ProjectDate
	for _, it := range m.items {
		if it.projectName == rootName {
			result = append(result, it.date)
		}
	}
	return result
}

// GetSubProjectDates returns all non-root dates grouped by project name.
// Includes an entry (possibly empty) for every managed sub-project so callers
// can write back even when all dates for a project were deleted.
func (m DateEditorModel) GetSubProjectDates() map[string][]workspace.ProjectDate {
	if len(m.projectNames) == 0 {
		return nil
	}
	rootName := m.projectNames[0]
	result := make(map[string][]workspace.ProjectDate)
	for _, name := range m.projectNames[1:] {
		result[name] = nil
	}
	for _, it := range m.items {
		if it.projectName != rootName {
			result[it.projectName] = append(result[it.projectName], it.date)
		}
	}
	return result
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
	return m.mode != dateEditorNav && m.mode != dateEditorSelectProject
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

	if m.mode == dateEditorSelectProject {
		s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render("Add to project:"))
		s.WriteString("\n\n")
		for i, name := range m.projectNames {
			prefix := "  "
			if i == m.projectCursor {
				prefix = "> "
			}
			var line string
			if i == m.projectCursor {
				line = selectedListItemStyle.Render(prefix + name)
			} else {
				line = listItemStyle.Render(prefix + name)
			}
			s.WriteString(line)
			s.WriteString("\n")
		}
		s.WriteString("\n")
		s.WriteString(dateEditorHelpStyle.Render("j/k: navigate  enter: confirm  esc: cancel"))
	} else if m.mode == dateEditorEditLabel {
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
		s.WriteString(dateEditorHelpStyle.Render("enter: confirm  esc: cancel"))
	} else {
		// Nav mode — show date list with per-project headers when multi-project
		multiProject := len(m.projectNames) > 1
		if len(m.items) == 0 {
			s.WriteString(pathStyle.Render("  No dates"))
			s.WriteString("\n")
		} else {
			lastProject := "\x00" // sentinel so first header always shows
			for i, item := range m.items {
				d := item.date

				// Project header when in multi-project mode and project changes
				if multiProject && item.projectName != lastProject {
					if lastProject != "\x00" {
						s.WriteString("\n")
					}
					header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render("  " + item.projectName)
					s.WriteString(header)
					s.WriteString("\n")
					lastProject = item.projectName
				}

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
