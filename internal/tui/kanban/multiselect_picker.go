package kanban

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
)

// MultiSelectPickerConfig configures a generic multi-select picker
type MultiSelectPickerConfig struct {
	Title            string              // "Edit Tags" or "Edit Projects"
	ItemTypeSingular string              // "tag" or "project"
	SanitizeFunc     func(string) string // sanitizeTag or sanitizeProject
	AllItems         []string
	SelectedItems    map[string]bool
}

// MultiSelectPickerModel is a generic fuzzy-searchable multi-select picker
type MultiSelectPickerModel struct {
	config        MultiSelectPickerConfig
	textInput     textinput.Model
	query         string
	filteredItems []string
	cursorPos     int
	width         int
	height        int
	showCreate    bool // Show "create new" option
	filterMode    bool // true when in filter mode, false in navigation mode
	createMode    bool // true when in create mode
}

// NewMultiSelectPickerModel creates a new multi-select picker
func NewMultiSelectPickerModel(config MultiSelectPickerConfig) MultiSelectPickerModel {
	ti := textinput.New()
	ti.Placeholder = "Press / to filter..."
	ti.CharLimit = 50
	ti.Width = 40

	// Always start in navigation mode
	ti.Blur()

	m := MultiSelectPickerModel{
		config:        config,
		textInput:     ti,
		query:         "",
		filteredItems: config.AllItems,
		cursorPos:     0,
		width:         50,
		height:        20,
		showCreate:    false,
		filterMode:    false,
		createMode:    false,
	}

	return m
}

// Init initializes the picker
func (m MultiSelectPickerModel) Init() tea.Cmd {
	return nil
}

// Update handles picker events
// Returns (model, cmd, isDone)
func (m MultiSelectPickerModel) Update(msg tea.Msg) (MultiSelectPickerModel, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle keys based on current mode
		if m.filterMode {
			// FILTER MODE: handle text input and mode exits
			switch msg.String() {
			case "esc":
				// Cancel filter, return to navigation mode
				m.textInput.SetValue("")
				m.query = ""
				m.filterItems()
				m.textInput.Blur()
				m.filterMode = false
				m.cursorPos = 0
				return m, nil, false

			case "enter":
				// Just exit filter mode and return to navigation
				m.textInput.Blur()
				m.filterMode = false
				return m, nil, false

			default:
				// Pass all other keys to text input
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				m.query = m.textInput.Value()
				m.filterItems()
				m.cursorPos = 0
				return m, cmd, false
			}
		} else if m.createMode {
			// CREATE MODE: handle new item input
			switch msg.String() {
			case "esc":
				// Cancel create, return to navigation mode
				m.textInput.SetValue("")
				m.textInput.Placeholder = "Press / to filter..."
				m.textInput.Blur()
				m.createMode = false
				return m, nil, false

			case "enter":
				// Create new item and return to navigation mode
				input := strings.TrimSpace(m.textInput.Value())
				if input != "" {
					sanitized := m.config.SanitizeFunc(input)
					if sanitized != "" {
						// Add to selection
						m.config.SelectedItems[sanitized] = true
						// Add to all items list if not already there
						if !contains(m.config.AllItems, sanitized) {
							m.config.AllItems = append(m.config.AllItems, sanitized)
							sortItems(m.config.AllItems)
						}
					}
				}
				// Reset and return to navigation
				m.textInput.SetValue("")
				m.textInput.Placeholder = "Press / to filter..."
				m.textInput.Blur()
				m.createMode = false
				m.filterItems() // Refresh filtered list
				return m, nil, false

			default:
				// Pass to text input
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd, false
			}
		} else {
			// NAVIGATION MODE: handle list navigation and mode switches
			switch msg.String() {
			case "n":
				// Enter create mode
				m.textInput.SetValue("")
				m.textInput.Placeholder = "Enter new " + m.config.ItemTypeSingular + " name..."
				m.textInput.Focus()
				m.createMode = true
				return m, textinput.Blink, false

			case "/":
				// Enter filter mode
				m.textInput.Focus()
				m.filterMode = true
				return m, textinput.Blink, false

			case "enter":
				// Save and exit picker
				return m, nil, true

			case "esc":
				// If filter is active, clear it; otherwise exit picker
				if m.query != "" {
					m.textInput.SetValue("")
					m.query = ""
					m.filterItems()
					m.cursorPos = 0
					return m, nil, false
				}
				// No filter active, exit picker (cancel)
				return m, nil, true

			case "tab", " ":
				// Toggle item selection at cursor
				m.toggleItem()
				return m, nil, false

			case "j", "down":
				// Move cursor down
				maxPos := len(m.filteredItems) - 1
				if m.showCreate {
					maxPos++
				}
				if m.cursorPos < maxPos {
					m.cursorPos++
				}
				return m, nil, false

			case "k", "up":
				// Move cursor up
				if m.cursorPos > 0 {
					m.cursorPos--
				}
				return m, nil, false
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil, false
	}

	return m, nil, false
}

// View renders the picker
func (m MultiSelectPickerModel) View() string {
	var s strings.Builder

	// Title
	title := tagPickerTitleStyle.Render(m.config.Title)
	s.WriteString(title)
	s.WriteString("\n\n")

	// Text input - show with indicator based on mode
	if m.createMode {
		s.WriteString(tagPickerTitleStyle.Render("Create new: "))
	} else if m.filterMode {
		s.WriteString(tagPickerTitleStyle.Render("Filtering: "))
	}
	s.WriteString(m.textInput.View())
	s.WriteString("\n\n")

	// Item list
	if len(m.config.AllItems) == 0 {
		s.WriteString(tagItemStyle.Render("No " + m.config.ItemTypeSingular + "s yet. Press 'n' to create one."))
		s.WriteString("\n")
	} else if len(m.filteredItems) == 0 && !m.showCreate {
		s.WriteString(tagItemStyle.Render("No matching " + m.config.ItemTypeSingular + "s"))
		s.WriteString("\n")
	} else {
		// Render filtered items
		for i, item := range m.filteredItems {
			s.WriteString(m.renderItem(i, item))
		}

		// Render "create new" option if query doesn't match existing item
		if m.showCreate && m.query != "" {
			s.WriteString(m.renderCreateNew(len(m.filteredItems)))
		}
	}

	s.WriteString("\n")

	// Help text - show different help based on mode
	var help string
	if m.createMode {
		help = helpStyle.Render("enter: create • esc: cancel")
	} else if m.filterMode {
		help = helpStyle.Render("enter: apply filter • esc: cancel")
	} else if m.query != "" {
		help = helpStyle.Render("jk: navigate • tab: toggle • n: new • /: filter • esc: clear • enter: save")
	} else {
		help = helpStyle.Render("jk: navigate • tab: toggle • n: new • /: filter • enter: save • esc: cancel")
	}
	s.WriteString(help)

	// Wrap in box
	content := s.String()
	return tagPickerBoxStyle.Render(content)
}

// renderItem renders a single item
func (m MultiSelectPickerModel) renderItem(index int, item string) string {
	// Checkbox
	checkbox := "[ ]"
	if m.config.SelectedItems[item] {
		checkbox = "[x]"
	}

	// Item with checkbox
	text := checkbox + " " + item

	// Highlight if selected item
	if m.config.SelectedItems[item] {
		text += " (on card)"
	}

	// Apply cursor highlight
	style := tagItemStyle
	if index == m.cursorPos {
		style = tagItemHighlightStyle
	} else if m.config.SelectedItems[item] {
		style = tagItemSelectedStyle
	}

	return style.Render(text) + "\n"
}

// renderCreateNew renders the "create new" option
func (m MultiSelectPickerModel) renderCreateNew(index int) string {
	checkbox := "[ ]"
	if m.config.SelectedItems[m.query] {
		checkbox = "[x]"
	}

	text := checkbox + " + Create new: \"" + m.query + "\""

	style := tagCreateNewStyle
	if index == m.cursorPos {
		style = tagItemHighlightStyle.Copy().Foreground(colorSuccess)
	}

	return style.Render(text) + "\n"
}

// toggleItem toggles the item at the current cursor position
func (m *MultiSelectPickerModel) toggleItem() {
	// Check if cursor is on "create new" option
	if m.showCreate && m.cursorPos == len(m.filteredItems) {
		// Toggle new item
		sanitized := m.config.SanitizeFunc(m.query)
		if sanitized != "" {
			if m.config.SelectedItems[sanitized] {
				delete(m.config.SelectedItems, sanitized)
			} else {
				m.config.SelectedItems[sanitized] = true
			}
		}
		return
	}

	// Toggle existing item
	if m.cursorPos >= 0 && m.cursorPos < len(m.filteredItems) {
		item := m.filteredItems[m.cursorPos]
		if m.config.SelectedItems[item] {
			delete(m.config.SelectedItems, item)
		} else {
			m.config.SelectedItems[item] = true
		}
	}
}

// filterItems applies fuzzy matching to filter items
func (m *MultiSelectPickerModel) filterItems() {
	if m.query == "" {
		// No filter, show all items
		m.filteredItems = m.config.AllItems
		m.showCreate = false
		return
	}

	// Fuzzy match
	matches := fuzzy.Find(m.query, m.config.AllItems)

	// Extract matched items
	filtered := make([]string, len(matches))
	for i, match := range matches {
		filtered[i] = match.Str
	}
	m.filteredItems = filtered

	// Check if query exactly matches an existing item
	m.showCreate = !exactMatch(m.query, m.config.AllItems)
}

// exactMatch checks if query exactly matches any item (case-insensitive)
func exactMatch(query string, items []string) bool {
	normalized := strings.ToLower(strings.TrimSpace(query))
	for _, item := range items {
		if strings.ToLower(item) == normalized {
			return true
		}
	}
	return false
}

// GetSelectedItems returns the final list of selected items
func (m MultiSelectPickerModel) GetSelectedItems() []string {
	items := make([]string, 0, len(m.config.SelectedItems))
	for item := range m.config.SelectedItems {
		items = append(items, item)
	}

	// Sort for consistent ordering
	sortItems(items)
	return items
}

// sortItems sorts items alphabetically
func sortItems(items []string) {
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i] > items[j] {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// contains checks if a string exists in a slice
func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
