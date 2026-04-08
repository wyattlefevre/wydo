package kanban

import (
	tea "github.com/charmbracelet/bubbletea"
	"wydo/internal/tui/shared"
)

// MultiSelectPickerConfig is kept for backward compatibility with existing callers.
// New code should use shared.ListPickerConfig directly.
type MultiSelectPickerConfig struct {
	Title            string
	ItemTypeSingular string
	SanitizeFunc     func(string) string
	AllItems         []string
	SelectedItems    map[string]bool
	ItemDepths       map[string]int
	SingleSelect     bool
}

// MultiSelectPickerModel is a forwarding shim around shared.ListPickerModel.
// The createMode field is exposed for test compatibility.
type MultiSelectPickerModel struct {
	inner      shared.ListPickerModel
	createMode bool // synced from inner.CreateModeActive() after each Update
}

// NewMultiSelectPickerModel creates a new picker from a legacy config.
func NewMultiSelectPickerModel(config MultiSelectPickerConfig) MultiSelectPickerModel {
	lc := shared.ListPickerConfig{
		Title:            config.Title,
		ItemTypeSingular: config.ItemTypeSingular,
		AllItems:         config.AllItems,
		PreSelectedItems: config.SelectedItems,
		ItemDepths:       config.ItemDepths,
		MultiSelect:      !config.SingleSelect,
		AllowCreate:      !config.SingleSelect && config.SanitizeFunc != nil,
		SanitizeFunc:     config.SanitizeFunc,
	}
	return MultiSelectPickerModel{
		inner: shared.NewListPickerModel(lc),
	}
}

// Init initializes the picker.
func (m MultiSelectPickerModel) Init() tea.Cmd {
	return m.inner.Init()
}

// Update handles picker events.
// Returns (model, cmd, isDone, cancelled).
func (m MultiSelectPickerModel) Update(msg tea.Msg) (MultiSelectPickerModel, tea.Cmd, bool, bool) {
	inner, cmd, isDone, cancelled := m.inner.Update(msg)
	m.inner = inner
	m.createMode = inner.CreateModeActive()
	return m, cmd, isDone, cancelled
}

// View renders the picker.
func (m MultiSelectPickerModel) View() string {
	return m.inner.View()
}

// GetSelectedItems returns the sorted list of selected item names.
func (m MultiSelectPickerModel) GetSelectedItems() []string {
	return m.inner.GetSelectedItems()
}
