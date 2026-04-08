package shared

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	"wydo/internal/tui/theme"
)

// ListPickerConfig configures a ListPickerModel.
type ListPickerConfig struct {
	// Title is rendered at the top of the modal.
	Title string

	// ItemTypeSingular is used in empty-state messages ("No tags yet…").
	// Defaults to "item" if empty.
	ItemTypeSingular string

	// AllItems is the full unfiltered list of items in display order.
	AllItems []string

	// PreSelectedItems is the initial selection. Copied on construction.
	PreSelectedItems map[string]bool

	// ItemDepths optionally maps item names to indent depth (0 = top-level).
	// Nil means no indentation.
	ItemDepths map[string]int

	// MultiSelect allows toggling multiple items with Tab/Space.
	// When false (single-select), selecting an item closes the picker immediately.
	MultiSelect bool

	// AllowCreate enables the 'n' key and a "+ Create" row when the query has
	// no exact match. Only valid when MultiSelect is true.
	// SanitizeFunc must be non-nil when AllowCreate is true.
	AllowCreate bool

	// SanitizeFunc cleans user-entered text before creating a new item.
	// Required when AllowCreate is true.
	SanitizeFunc func(string) string

	// Width is the inner content width of the modal box. Default 50 if zero.
	Width int

	// MaxVisible is the maximum number of list rows shown before scrolling.
	// Default 10 if zero.
	MaxVisible int
}

// ListPickerModel is a unified fuzzy-searchable list picker supporting single
// and multi-select, item indentation, create-new, and scroll windowing.
//
// Update uses a non-standard (model, cmd, isDone, cancelled) return so that
// kanban wrappers can detect completion synchronously.
// For callers that need a standard tea.Model, wrap it with a pointer-receiver
// adapter that converts isDone/cancelled to a result message.
type ListPickerModel struct {
	config        ListPickerConfig
	allItems      []string        // copy of config.AllItems; may grow via AllowCreate
	selected      map[string]bool // live selection state; map is shared across copies
	filteredItems []string
	showCreate    bool
	cursorPos     int
	scrollOffset  int
	query         string
	textInput     textinput.Model
	filterMode    bool
	createMode    bool
}

// ListPickerResultMsg is the result type for list picker completion.
type ListPickerResultMsg struct {
	Selected  []string
	Cancelled bool
}

// NewListPickerModel creates a new ListPickerModel.
// Panics if AllowCreate is true and SanitizeFunc is nil.
func NewListPickerModel(config ListPickerConfig) ListPickerModel {
	if config.AllowCreate && config.SanitizeFunc == nil {
		panic("shared.NewListPickerModel: SanitizeFunc must be set when AllowCreate is true")
	}

	// Copy AllItems to prevent external mutation affecting internal state.
	allItems := make([]string, len(config.AllItems))
	copy(allItems, config.AllItems)

	// Copy PreSelectedItems so the caller's map is not mutated.
	selected := make(map[string]bool, len(config.PreSelectedItems))
	for k, v := range config.PreSelectedItems {
		selected[k] = v
	}

	ti := textinput.New()
	ti.Placeholder = "Press / to filter..."
	ti.CharLimit = 100
	ti.Width = 40
	ti.Blur()

	m := ListPickerModel{
		config:    config,
		allItems:  allItems,
		selected:  selected,
		textInput: ti,
	}
	m.filteredItems = m.allItems
	return m
}

// Init implements tea.Model.
func (m ListPickerModel) Init() tea.Cmd {
	return nil
}

// Update handles picker events.
// Returns (updatedModel, cmd, isDone, cancelled).
func (m ListPickerModel) Update(msg tea.Msg) (ListPickerModel, tea.Cmd, bool, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filterMode {
			return m.updateFilterMode(msg)
		}
		if m.createMode {
			return m.updateCreateMode(msg)
		}
		return m.updateNavMode(msg)
	}
	return m, nil, false, false
}

func (m ListPickerModel) updateFilterMode(msg tea.KeyMsg) (ListPickerModel, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "esc":
		m.textInput.SetValue("")
		m.query = ""
		m.filterItems()
		m.textInput.Blur()
		m.filterMode = false
		m.cursorPos = 0
		m.scrollOffset = 0
		return m, nil, false, false

	case "enter":
		m.textInput.Blur()
		m.filterMode = false
		return m, nil, false, false

	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		newQuery := m.textInput.Value()
		if newQuery != m.query {
			m.query = newQuery
			m.filterItems()
			m.cursorPos = 0
			m.scrollOffset = 0
		}
		return m, cmd, false, false
	}
}

func (m ListPickerModel) updateCreateMode(msg tea.KeyMsg) (ListPickerModel, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "esc":
		m.textInput.SetValue("")
		m.textInput.Placeholder = "Press / to filter..."
		m.textInput.Blur()
		m.createMode = false
		return m, nil, false, false

	case "enter":
		input := strings.TrimSpace(m.textInput.Value())
		if input != "" {
			sanitized := m.config.SanitizeFunc(input)
			if sanitized != "" {
				m.selected[sanitized] = true
				if !listContains(m.allItems, sanitized) {
					m.allItems = append(m.allItems, sanitized)
					sort.Strings(m.allItems)
				}
			}
		}
		m.textInput.SetValue("")
		m.textInput.Placeholder = "Press / to filter..."
		m.textInput.Blur()
		m.createMode = false
		m.filterItems()
		return m, nil, false, false

	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd, false, false
	}
}

func (m ListPickerModel) updateNavMode(msg tea.KeyMsg) (ListPickerModel, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "n":
		if !m.config.MultiSelect || !m.config.AllowCreate {
			return m, nil, false, false
		}
		m.textInput.SetValue("")
		singular := m.itemTypeSingular()
		m.textInput.Placeholder = "Enter new " + singular + " name..."
		m.textInput.Focus()
		m.createMode = true
		return m, textinput.Blink, false, false

	case "/":
		m.textInput.Focus()
		m.filterMode = true
		return m, textinput.Blink, false, false

	case "enter":
		if !m.config.MultiSelect {
			// Single-select: toggle item at cursor, then close.
			if m.cursorPos < len(m.filteredItems) {
				item := m.filteredItems[m.cursorPos]
				if m.selected[item] {
					m.selected = map[string]bool{}
				} else {
					m.selected = map[string]bool{item: true}
				}
			}
			return m, nil, true, false
		}
		return m, nil, true, false

	case "esc":
		if m.query != "" {
			m.textInput.SetValue("")
			m.query = ""
			m.filterItems()
			m.cursorPos = 0
			m.scrollOffset = 0
			return m, nil, false, false
		}
		return m, nil, true, true

	case "tab", " ":
		if !m.config.MultiSelect {
			if m.cursorPos < len(m.filteredItems) {
				item := m.filteredItems[m.cursorPos]
				if m.selected[item] {
					m.selected = map[string]bool{}
				} else {
					m.selected = map[string]bool{item: true}
				}
			}
			return m, nil, true, false
		}
		m.toggleItem()
		return m, nil, false, false

	case "j", "down":
		maxPos := len(m.filteredItems) - 1
		if m.showCreate {
			maxPos++
		}
		if m.cursorPos < maxPos {
			m.cursorPos++
		}
		visible := m.effectiveMaxVisible()
		if m.cursorPos >= m.scrollOffset+visible {
			m.scrollOffset = m.cursorPos - visible + 1
		}
		return m, nil, false, false

	case "k", "up":
		if m.cursorPos > 0 {
			m.cursorPos--
		}
		if m.cursorPos < m.scrollOffset {
			m.scrollOffset = m.cursorPos
		}
		return m, nil, false, false
	}

	return m, nil, false, false
}

// View renders the picker inside a modal box.
func (m ListPickerModel) View() string {
	var s strings.Builder

	// Title
	s.WriteString(theme.ModalTitle.Render(m.config.Title))
	s.WriteString("\n\n")

	// Text input line
	if m.createMode {
		s.WriteString(theme.ModalTitle.Render("Create new: "))
	} else if m.filterMode {
		s.WriteString(theme.ModalTitle.Render("Filter: "))
	}
	s.WriteString(m.textInput.View())
	s.WriteString("\n\n")

	// Item list
	singular := m.itemTypeSingular()
	if len(m.allItems) == 0 {
		if m.config.AllowCreate {
			s.WriteString(theme.Muted.Render("No " + singular + "s yet. Press 'n' to create one."))
		} else {
			s.WriteString(theme.Muted.Render("No " + singular + "s available."))
		}
		s.WriteString("\n")
	} else if len(m.filteredItems) == 0 && !m.showCreate {
		s.WriteString(theme.Muted.Render("No matching " + singular + "s"))
		s.WriteString("\n")
	} else {
		visible := m.effectiveMaxVisible()
		end := m.scrollOffset + visible
		if end > len(m.filteredItems) {
			end = len(m.filteredItems)
		}

		if m.scrollOffset > 0 {
			s.WriteString(theme.Muted.Render("  ↑ more") + "\n")
		}

		for i := m.scrollOffset; i < end; i++ {
			s.WriteString(m.renderItem(i, m.filteredItems[i]))
		}

		if end < len(m.filteredItems) {
			s.WriteString(theme.Muted.Render("  ↓ more") + "\n")
		}

		if m.showCreate && m.query != "" {
			s.WriteString(m.renderCreateNew(len(m.filteredItems)))
		}
	}

	s.WriteString("\n")
	s.WriteString(m.renderHelp())

	width := m.config.Width
	if width == 0 {
		width = 50
	}
	return theme.ModalBox.Width(width).Render(s.String())
}

func (m ListPickerModel) renderItem(index int, item string) string {
	// Indentation
	indent := ""
	if m.config.ItemDepths != nil {
		if depth, ok := m.config.ItemDepths[item]; ok {
			indent = strings.Repeat("  ", depth)
		}
	}

	// Checkbox (multi-select only)
	text := indent
	if m.config.MultiSelect {
		if m.selected[item] {
			text += "[x] "
		} else {
			text += "[ ] "
		}
	}
	text += item

	// Style: cursor > selected > normal
	if index == m.cursorPos {
		style := lipgloss.NewStyle().
			Background(lipgloss.Color("0")).
			Foreground(theme.Warning).
			Bold(true).
			PaddingLeft(2)
		return style.Render(text) + "\n"
	}
	if m.selected[item] {
		return theme.ListItemSelected.Render(text) + "\n"
	}
	return theme.ListItem.Render(text) + "\n"
}

func (m ListPickerModel) renderCreateNew(index int) string {
	sanitized := ""
	if m.config.SanitizeFunc != nil {
		sanitized = m.config.SanitizeFunc(m.query)
	}
	checkbox := "[ ] "
	if m.selected[sanitized] {
		checkbox = "[x] "
	}
	text := checkbox + `+ Create new: "` + m.query + `"`

	if index == m.cursorPos {
		style := lipgloss.NewStyle().
			Background(lipgloss.Color("0")).
			Foreground(theme.Success).
			Bold(true).
			PaddingLeft(2)
		return style.Render(text) + "\n"
	}
	return lipgloss.NewStyle().
		Foreground(theme.Success).
		Italic(true).
		PaddingLeft(2).
		Render(text) + "\n"
}

func (m ListPickerModel) renderHelp() string {
	var help string
	switch {
	case m.createMode:
		help = "enter: create • esc: cancel"
	case m.filterMode:
		help = "enter: apply • esc: cancel"
	case !m.config.MultiSelect:
		help = "j/k: navigate • enter: select • /: filter • esc: cancel"
	case m.config.AllowCreate && m.query != "":
		help = "j/k: navigate • tab: toggle • n: new • /: filter • esc: clear • enter: save"
	case m.config.AllowCreate:
		help = "j/k: navigate • tab: toggle • n: new • /: filter • enter: save • esc: cancel"
	case m.query != "":
		help = "j/k: navigate • tab: toggle • /: filter • esc: clear • enter: save"
	default:
		help = "j/k: navigate • tab: toggle • /: filter • enter: save • esc: cancel"
	}
	return theme.ModalHelp.Render(help)
}

// GetSelectedItems returns the sorted list of selected item names.
func (m ListPickerModel) GetSelectedItems() []string {
	items := make([]string, 0, len(m.selected))
	for item, ok := range m.selected {
		if ok {
			items = append(items, item)
		}
	}
	sort.Strings(items)
	return items
}

// PreSelect marks the given items as selected.
func (m *ListPickerModel) PreSelect(items []string) {
	for _, item := range items {
		m.selected[item] = true
	}
}

// CreateModeActive returns true when the picker is in create mode.
// Exposed for test inspection.
func (m ListPickerModel) CreateModeActive() bool {
	return m.createMode
}

// filterItems applies fuzzy matching and updates filteredItems and showCreate.
func (m *ListPickerModel) filterItems() {
	if m.query == "" {
		m.filteredItems = m.allItems
		m.showCreate = false
		return
	}

	matches := fuzzy.Find(m.query, m.allItems)
	filtered := make([]string, len(matches))
	for i, match := range matches {
		filtered[i] = match.Str
	}
	m.filteredItems = filtered
	m.showCreate = m.config.AllowCreate && !listExactMatch(m.query, m.allItems)
}

func (m *ListPickerModel) toggleItem() {
	// Check if cursor is on the "create new" row.
	if m.showCreate && m.cursorPos == len(m.filteredItems) {
		if m.config.SanitizeFunc != nil {
			sanitized := m.config.SanitizeFunc(m.query)
			if sanitized != "" {
				if m.selected[sanitized] {
					delete(m.selected, sanitized)
				} else {
					m.selected[sanitized] = true
				}
			}
		}
		return
	}
	if m.cursorPos >= 0 && m.cursorPos < len(m.filteredItems) {
		item := m.filteredItems[m.cursorPos]
		if m.selected[item] {
			delete(m.selected, item)
		} else {
			m.selected[item] = true
		}
	}
}

func (m ListPickerModel) effectiveMaxVisible() int {
	if m.config.MaxVisible > 0 {
		return m.config.MaxVisible
	}
	return 10
}

func (m ListPickerModel) itemTypeSingular() string {
	if m.config.ItemTypeSingular != "" {
		return m.config.ItemTypeSingular
	}
	return "item"
}

// listContains reports whether target exists in items.
func listContains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// listExactMatch reports whether query exactly matches any item (case-insensitive).
func listExactMatch(query string, items []string) bool {
	normalized := strings.ToLower(strings.TrimSpace(query))
	for _, item := range items {
		if strings.ToLower(item) == normalized {
			return true
		}
	}
	return false
}
