package tasks

import (
	tea "github.com/charmbracelet/bubbletea"
	"wydo/internal/tui/shared"
)

// FuzzyPickerResultMsg is sent when selection is confirmed or cancelled.
type FuzzyPickerResultMsg struct {
	Selected  []string
	Cancelled bool
}

// FuzzyPickerModel is a thin pointer-receiver wrapper around shared.ListPickerModel
// that bridges the (isDone, cancelled) return to a FuzzyPickerResultMsg tea.Cmd.
//
// Pointer semantics are required because task_manager.go discards the returned
// tea.Model but still holds the pointer and relies on in-place mutation:
//
//	_, cmd = m.fuzzyPicker.Update(msg)
type FuzzyPickerModel struct {
	inner shared.ListPickerModel
}

// NewFuzzyPicker creates a new fuzzy picker.
func NewFuzzyPicker(items []string, title string, multiSelect bool, allowCreate bool) *FuzzyPickerModel {
	return &FuzzyPickerModel{
		inner: shared.NewListPickerModel(shared.ListPickerConfig{
			Title:       title,
			AllItems:    items,
			MultiSelect: multiSelect,
			AllowCreate: allowCreate,
			Width:       50,
			MaxVisible:  10,
			// SanitizeFunc intentionally nil — tasks callers never use AllowCreate=true.
			// ItemDepths intentionally nil — tasks do not use indentation.
		}),
	}
}

// Init implements tea.Model.
func (m *FuzzyPickerModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
// The pointer receiver ensures that state mutations are visible to the caller
// even when the returned tea.Model is discarded.
func (m *FuzzyPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd, isDone, cancelled := m.inner.Update(msg)
	m.inner = updated
	if isDone {
		var selected []string
		if !cancelled {
			selected = m.inner.GetSelectedItems()
		}
		return m, func() tea.Msg {
			return FuzzyPickerResultMsg{Selected: selected, Cancelled: cancelled}
		}
	}
	return m, cmd
}

// View implements tea.Model.
func (m *FuzzyPickerModel) View() string {
	return m.inner.View()
}

// GetSelected returns the currently selected items.
func (m *FuzzyPickerModel) GetSelected() []string {
	return m.inner.GetSelectedItems()
}

// PreSelect marks items as selected before the picker is shown.
func (m *FuzzyPickerModel) PreSelect(items []string) {
	m.inner.PreSelect(items)
}
