package tasks

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	pickerTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	pickerItemStyle     = lipgloss.NewStyle().PaddingLeft(2)
	pickerSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2")).PaddingLeft(0)
	pickerCheckedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	pickerCreateStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Italic(true).PaddingLeft(2)
	pickerBoxStyle      = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("4")).Padding(0, 1)
)

// FuzzyPickerModel is a fuzzy-searchable list picker
type FuzzyPickerModel struct {
	Items       []string
	Filtered    []string
	Query       string
	Cursor      int
	Selected    map[string]bool
	MultiSelect bool
	AllowCreate bool
	Title       string
	Width       int
	MaxVisible  int
	textInput   textinput.Model
	filterMode  bool // true when actively typing filter
}

// FuzzyPickerResultMsg is sent when selection is confirmed or cancelled
type FuzzyPickerResultMsg struct {
	Selected  []string
	Cancelled bool
}

// NewFuzzyPicker creates a new fuzzy picker
func NewFuzzyPicker(items []string, title string, multiSelect bool, allowCreate bool) *FuzzyPickerModel {
	ti := textinput.New()
	ti.Placeholder = "press / to filter..."
	ti.CharLimit = 256
	ti.Width = 40
	ti.Blur()

	return &FuzzyPickerModel{
		Items:       items,
		Filtered:    items,
		Selected:    make(map[string]bool),
		MultiSelect: multiSelect,
		AllowCreate: allowCreate,
		Title:       title,
		Width:       50,
		MaxVisible:  10,
		textInput:   ti,
		filterMode:  false,
	}
}

// Init implements tea.Model
func (m *FuzzyPickerModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m *FuzzyPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filterMode {
			switch msg.String() {
			case "enter":
				m.filterMode = false
				m.textInput.Blur()
				return m, nil
			case "esc":
				m.textInput.SetValue("")
				m.Query = ""
				m.filterItems()
				m.Cursor = 0
				m.filterMode = false
				m.textInput.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				newQuery := m.textInput.Value()
				if newQuery != m.Query {
					m.Query = newQuery
					m.filterItems()
					m.Cursor = 0
				}
				return m, cmd
			}
		}

		switch msg.String() {
		case "/":
			m.filterMode = true
			m.textInput.Focus()
			return m, textinput.Blink

		case "enter":
			return m, m.confirm()

		case "esc":
			if m.Query != "" {
				m.textInput.SetValue("")
				m.Query = ""
				m.filterItems()
				m.Cursor = 0
				return m, nil
			}
			return m, func() tea.Msg {
				return FuzzyPickerResultMsg{
					Selected:  nil,
					Cancelled: true,
				}
			}

		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
			return m, nil

		case "down", "j":
			maxIdx := len(m.Filtered) - 1
			if m.AllowCreate && m.Query != "" && !m.itemExists(m.Query) {
				maxIdx++
			}
			if m.Cursor < maxIdx {
				m.Cursor++
			}
			return m, nil

		case " ":
			if m.MultiSelect {
				m.toggleCurrent()
			}
			return m, nil
		}
	}

	return m, nil
}

// View implements tea.Model
func (m *FuzzyPickerModel) View() string {
	var content string

	content += pickerTitleStyle.Render(m.Title) + "\n"
	content += m.textInput.View() + "\n\n"

	startIdx := 0
	if m.Cursor >= m.MaxVisible {
		startIdx = m.Cursor - m.MaxVisible + 1
	}

	for i := startIdx; i < len(m.Filtered) && i < startIdx+m.MaxVisible; i++ {
		item := m.Filtered[i]
		line := m.renderItem(item, i == m.Cursor, m.Selected[item])
		content += line + "\n"
	}

	if m.AllowCreate && m.Query != "" && !m.itemExists(m.Query) {
		isSelected := m.Cursor == len(m.Filtered)
		isChecked := m.Selected[m.Query]
		prefix := "  "
		if isSelected {
			prefix = "> "
		}
		createText := "+ Create \"" + m.Query + "\""
		if m.MultiSelect {
			check := "[ ] "
			if isChecked {
				check = "[x] "
			}
			createText = check + createText
		}
		if isSelected {
			content += prefix + pickerSelectedStyle.Render(createText) + "\n"
		} else if isChecked {
			content += prefix + pickerCheckedStyle.Render(createText) + "\n"
		} else {
			content += prefix + pickerCreateStyle.Render(createText) + "\n"
		}
	}

	var help string
	if m.filterMode {
		help = "[enter] done  [esc] clear"
	} else {
		help = "[/] filter  [enter] select  [esc] cancel"
		if m.MultiSelect {
			help = "[/] filter  [space] toggle  [enter] select  [esc] cancel"
		}
		if m.Query != "" {
			help = "[/] filter  [enter] select  [esc] clear"
			if m.MultiSelect {
				help = "[/] filter  [space] toggle  [enter] select  [esc] clear"
			}
		}
	}
	content += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(help)

	return pickerBoxStyle.Width(m.Width).Render(content)
}

func (m *FuzzyPickerModel) renderItem(item string, cursor bool, checked bool) string {
	prefix := "  "
	if cursor {
		prefix = "> "
	}

	if m.MultiSelect {
		check := "[ ] "
		if checked {
			check = "[x] "
		}
		if cursor {
			return prefix + pickerSelectedStyle.Render(check+item)
		}
		if checked {
			return prefix + pickerCheckedStyle.Render(check+item)
		}
		return prefix + pickerItemStyle.Render(check+item)
	}

	if cursor {
		return prefix + pickerSelectedStyle.Render(item)
	}
	return prefix + pickerItemStyle.Render(item)
}

func (m *FuzzyPickerModel) filterItems() {
	if m.Query == "" {
		m.Filtered = m.Items
		return
	}

	query := strings.ToLower(m.Query)
	var filtered []string
	for _, item := range m.Items {
		if strings.Contains(strings.ToLower(item), query) {
			filtered = append(filtered, item)
		}
	}
	m.Filtered = filtered
}

func (m *FuzzyPickerModel) itemExists(name string) bool {
	lower := strings.ToLower(name)
	for _, item := range m.Items {
		if strings.ToLower(item) == lower {
			return true
		}
	}
	return false
}

func (m *FuzzyPickerModel) toggleCurrent() {
	if m.Cursor < len(m.Filtered) {
		item := m.Filtered[m.Cursor]
		m.Selected[item] = !m.Selected[item]
	} else if m.AllowCreate && m.Query != "" && !m.itemExists(m.Query) {
		m.Selected[m.Query] = !m.Selected[m.Query]
	}
}

func (m *FuzzyPickerModel) confirm() tea.Cmd {
	return func() tea.Msg {
		var selected []string

		if m.MultiSelect {
			for item, checked := range m.Selected {
				if checked {
					selected = append(selected, item)
				}
			}
		} else {
			if m.Cursor < len(m.Filtered) {
				selected = []string{m.Filtered[m.Cursor]}
			} else if m.AllowCreate && m.Query != "" {
				selected = []string{m.Query}
			}
		}

		return FuzzyPickerResultMsg{
			Selected:  selected,
			Cancelled: false,
		}
	}
}

// GetSelected returns the current selection
func (m *FuzzyPickerModel) GetSelected() []string {
	var result []string
	for item, checked := range m.Selected {
		if checked {
			result = append(result, item)
		}
	}
	return result
}

// PreSelect marks items as selected
func (m *FuzzyPickerModel) PreSelect(items []string) {
	for _, item := range items {
		m.Selected[item] = true
	}
}
