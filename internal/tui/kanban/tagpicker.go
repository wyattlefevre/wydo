package kanban

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"wydo/internal/tui/shared"
)

// TagPickerModel is a fuzzy-searchable multi-select tag picker.
type TagPickerModel struct {
	picker shared.ListPickerModel
}

// NewTagPickerModel creates a new tag picker with current card tags and all available tags.
func NewTagPickerModel(currentTags []string, allTags []string) TagPickerModel {
	selected := make(map[string]bool, len(currentTags))
	for _, tag := range currentTags {
		selected[tag] = true
	}

	return TagPickerModel{
		picker: shared.NewListPickerModel(shared.ListPickerConfig{
			Title:            "Edit Tags",
			ItemTypeSingular: "tag",
			AllItems:         allTags,
			PreSelectedItems: selected,
			MultiSelect:      true,
			AllowCreate:      true,
			SanitizeFunc:     sanitizeTag,
		}),
	}
}

// Init initializes the tag picker.
func (m TagPickerModel) Init() tea.Cmd {
	return m.picker.Init()
}

// Update handles tag picker events.
// Returns (model, cmd, isDone, cancelled).
func (m TagPickerModel) Update(msg tea.Msg) (TagPickerModel, tea.Cmd, bool, bool) {
	picker, cmd, isDone, cancelled := m.picker.Update(msg)
	m.picker = picker
	return m, cmd, isDone, cancelled
}

// View renders the tag picker.
func (m TagPickerModel) View() string {
	return m.picker.View()
}

// GetSelectedTags returns the final list of selected tags.
func (m TagPickerModel) GetSelectedTags() []string {
	return m.picker.GetSelectedItems()
}

// sanitizeTag cleans and normalizes a tag string.
func sanitizeTag(tag string) string {
	cleaned := strings.ToLower(strings.TrimSpace(tag))
	var result strings.Builder
	for _, r := range cleaned {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}
	return result.String()
}
