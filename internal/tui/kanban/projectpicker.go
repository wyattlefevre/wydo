package kanban

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ProjectPickerItem represents a project with its nesting depth for display
type ProjectPickerItem struct {
	Name    string
	Depth   int
	DirPath string
}

// ProjectPickerModel is a fuzzy-searchable multi-select project picker
type ProjectPickerModel struct {
	picker MultiSelectPickerModel
}

// NewProjectPickerModel creates a new project picker with current card projects and all available projects
func NewProjectPickerModel(currentProjects []string, allProjects []ProjectPickerItem) ProjectPickerModel {
	// Build selected projects set
	selected := make(map[string]bool)
	for _, project := range currentProjects {
		selected[project] = true
	}

	// Build flat name list and depth map
	names := make([]string, len(allProjects))
	depths := make(map[string]int, len(allProjects))
	for i, item := range allProjects {
		names[i] = item.Name
		depths[item.Name] = item.Depth
	}

	config := MultiSelectPickerConfig{
		Title:            "Edit Projects",
		ItemTypeSingular: "project",
		SanitizeFunc:     sanitizeProject,
		AllItems:         names,
		SelectedItems:    selected,
		ItemDepths:       depths,
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
// Returns (model, cmd, isDone, cancelled)
func (m ProjectPickerModel) Update(msg tea.Msg) (ProjectPickerModel, tea.Cmd, bool, bool) {
	picker, cmd, isDone, cancelled := m.picker.Update(msg)
	m.picker = picker
	return m, cmd, isDone, cancelled
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

// NewBoardProjectPickerModel creates a single-select project picker for linking a board
// to a project. currentProjectName is the project name currently linked, or "" if none.
func NewBoardProjectPickerModel(currentProjectName string, allProjects []ProjectPickerItem) ProjectPickerModel {
	selected := make(map[string]bool)
	if currentProjectName != "" {
		selected[currentProjectName] = true
	}

	names := make([]string, len(allProjects))
	depths := make(map[string]int, len(allProjects))
	for i, item := range allProjects {
		names[i] = item.Name
		depths[item.Name] = item.Depth
	}

	config := MultiSelectPickerConfig{
		Title:            "Link Board to Project",
		ItemTypeSingular: "project",
		SanitizeFunc:     sanitizeProject,
		AllItems:         names,
		SelectedItems:    selected,
		ItemDepths:       depths,
		SingleSelect:     true,
	}

	return ProjectPickerModel{
		picker: NewMultiSelectPickerModel(config),
	}
}
