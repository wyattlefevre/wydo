package kanban

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"wydo/internal/tui/shared"
)

// ProjectPickerItem represents a project with its nesting depth for display.
type ProjectPickerItem struct {
	Name    string
	Depth   int
	FilePath string
}

// ProjectPickerModel is a fuzzy-searchable multi-select project picker.
type ProjectPickerModel struct {
	picker shared.ListPickerModel
}

// NewProjectPickerModel creates a new project picker with current card projects and all available projects.
func NewProjectPickerModel(currentProjects []string, allProjects []ProjectPickerItem) ProjectPickerModel {
	selected := make(map[string]bool, len(currentProjects))
	for _, project := range currentProjects {
		selected[project] = true
	}

	names := make([]string, len(allProjects))
	depths := make(map[string]int, len(allProjects))
	for i, item := range allProjects {
		names[i] = item.Name
		depths[item.Name] = item.Depth
	}

	return ProjectPickerModel{
		picker: shared.NewListPickerModel(shared.ListPickerConfig{
			Title:            "Edit Projects",
			ItemTypeSingular: "project",
			AllItems:         names,
			PreSelectedItems: selected,
			ItemDepths:       depths,
			MultiSelect:      true,
			AllowCreate:      true,
			SanitizeFunc:     sanitizeProject,
		}),
	}
}

// Init initializes the project picker.
func (m ProjectPickerModel) Init() tea.Cmd {
	return m.picker.Init()
}

// Update handles project picker events.
// Returns (model, cmd, isDone, cancelled).
func (m ProjectPickerModel) Update(msg tea.Msg) (ProjectPickerModel, tea.Cmd, bool, bool) {
	picker, cmd, isDone, cancelled := m.picker.Update(msg)
	m.picker = picker
	return m, cmd, isDone, cancelled
}

// View renders the project picker.
func (m ProjectPickerModel) View() string {
	return m.picker.View()
}

// GetSelectedProjects returns the final list of selected projects.
func (m ProjectPickerModel) GetSelectedProjects() []string {
	return m.picker.GetSelectedItems()
}

// NewBoardProjectPickerModel creates a single-select project picker for linking a board to a project.
// currentProjectName is the project currently linked, or "" if none.
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

	return ProjectPickerModel{
		picker: shared.NewListPickerModel(shared.ListPickerConfig{
			Title:            "Link Board to Project",
			ItemTypeSingular: "project",
			AllItems:         names,
			PreSelectedItems: selected,
			ItemDepths:       depths,
			MultiSelect:      false,
			AllowCreate:      false,
		}),
	}
}

// sanitizeProject cleans and normalizes a project string.
func sanitizeProject(project string) string {
	cleaned := strings.ToLower(strings.TrimSpace(project))
	var result strings.Builder
	for _, r := range cleaned {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}
	return result.String()
}
