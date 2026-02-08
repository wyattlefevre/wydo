package kanban

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ProjectPickerModel is a fuzzy-searchable multi-select project picker
type ProjectPickerModel struct {
	picker MultiSelectPickerModel
}

// NewProjectPickerModel creates a new project picker with current card projects and all available projects
func NewProjectPickerModel(currentProjects []string, allProjects []string) ProjectPickerModel {
	// Build selected projects set
	selected := make(map[string]bool)
	for _, project := range currentProjects {
		selected[project] = true
	}

	config := MultiSelectPickerConfig{
		Title:            "Edit Projects",
		ItemTypeSingular: "project",
		SanitizeFunc:     sanitizeProject,
		AllItems:         allProjects,
		SelectedItems:    selected,
	}

	return ProjectPickerModel{
		picker: NewMultiSelectPickerModel(config),
	}
}

// Init initializes the project picker
func (m ProjectPickerModel) Init() tea.Cmd {
	return m.picker.Init()
}

// Update handles project picker events
// Returns (model, cmd, isDone)
func (m ProjectPickerModel) Update(msg tea.Msg) (ProjectPickerModel, tea.Cmd, bool) {
	picker, cmd, isDone := m.picker.Update(msg)
	m.picker = picker
	return m, cmd, isDone
}

// View renders the project picker
func (m ProjectPickerModel) View() string {
	return m.picker.View()
}

// sanitizeProject cleans and normalizes a project string
func sanitizeProject(project string) string {
	// Trim spaces and convert to lowercase
	cleaned := strings.ToLower(strings.TrimSpace(project))

	// Remove special characters except hyphens and underscores
	var result strings.Builder
	for _, r := range cleaned {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// GetSelectedProjects returns the final list of selected projects
func (m ProjectPickerModel) GetSelectedProjects() []string {
	return m.picker.GetSelectedItems()
}
