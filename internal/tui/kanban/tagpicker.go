package kanban

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// TagPickerModel is a fuzzy-searchable multi-select tag picker
type TagPickerModel struct {
	picker MultiSelectPickerModel
}

// NewTagPickerModel creates a new tag picker with current card tags and all available tags
func NewTagPickerModel(currentTags []string, allTags []string) TagPickerModel {
	// Build selected tags set
	selected := make(map[string]bool)
	for _, tag := range currentTags {
		selected[tag] = true
	}

	config := MultiSelectPickerConfig{
		Title:            "Edit Tags",
		ItemTypeSingular: "tag",
		SanitizeFunc:     sanitizeTag,
		AllItems:         allTags,
		SelectedItems:    selected,
	}

	return TagPickerModel{
		picker: NewMultiSelectPickerModel(config),
	}
}

// Init initializes the tag picker
func (m TagPickerModel) Init() tea.Cmd {
	return m.picker.Init()
}

// Update handles tag picker events
// Returns (model, cmd, isDone)
func (m TagPickerModel) Update(msg tea.Msg) (TagPickerModel, tea.Cmd, bool) {
	picker, cmd, isDone := m.picker.Update(msg)
	m.picker = picker
	return m, cmd, isDone
}

// View renders the tag picker
func (m TagPickerModel) View() string {
	return m.picker.View()
}

// sanitizeTag cleans and normalizes a tag string
func sanitizeTag(tag string) string {
	// Trim spaces and convert to lowercase
	cleaned := strings.ToLower(strings.TrimSpace(tag))

	// Remove special characters except hyphens and underscores
	var result strings.Builder
	for _, r := range cleaned {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// GetSelectedTags returns the final list of selected tags
func (m TagPickerModel) GetSelectedTags() []string {
	return m.picker.GetSelectedItems()
}
