package tasks

import (
	"path/filepath"
	"sort"
	"strings"

	"wydo/internal/tasks/data"
)

// SortField represents what field to sort by
type SortField int

const (
	SortByNone SortField = iota
	SortByDueDate
	SortByProject
	SortByPriority
	SortByContext
)

// SortState holds sorting configuration
type SortState struct {
	Field     SortField
	Ascending bool
}

// NewSortState creates a new default sort state
func NewSortState() SortState {
	return SortState{
		Field:     SortByNone,
		Ascending: true,
	}
}

// IsActive returns true if sorting is enabled
func (s *SortState) IsActive() bool {
	return s.Field != SortByNone
}

// Reset clears the sort state
func (s *SortState) Reset() {
	s.Field = SortByNone
	s.Ascending = true
}

// String returns a display string for the current sort
func (s *SortState) String() string {
	if s.Field == SortByNone {
		return ""
	}

	var field string
	switch s.Field {
	case SortByDueDate:
		field = "due"
	case SortByProject:
		field = "project"
	case SortByPriority:
		field = "priority"
	case SortByContext:
		field = "context"
	}

	dir := "asc"
	if !s.Ascending {
		dir = "desc"
	}

	return field + " " + dir
}

// GroupField represents what field to group by
type GroupField int

const (
	GroupByNone GroupField = iota
	GroupByDueDate
	GroupByProject
	GroupByPriority
	GroupByContext
	GroupByFile
)

// GroupState holds grouping configuration
type GroupState struct {
	Field     GroupField
	Ascending bool
}

// NewGroupState creates a new default group state
func NewGroupState() GroupState {
	return GroupState{
		Field:     GroupByNone,
		Ascending: true,
	}
}

// IsActive returns true if grouping is enabled
func (g *GroupState) IsActive() bool {
	return g.Field != GroupByNone
}

// Reset clears the group state
func (g *GroupState) Reset() {
	g.Field = GroupByNone
	g.Ascending = true
}

// String returns a display string for the current grouping
func (g *GroupState) String() string {
	if g.Field == GroupByNone {
		return ""
	}

	var field string
	switch g.Field {
	case GroupByDueDate:
		field = "due"
	case GroupByProject:
		field = "project"
	case GroupByPriority:
		field = "priority"
	case GroupByContext:
		field = "context"
	case GroupByFile:
		field = "file"
	}

	dir := "asc"
	if !g.Ascending {
		dir = "desc"
	}

	return field + " " + dir
}

// TaskGroup represents a group of tasks with a label
type TaskGroup struct {
	Label string
	Tasks []data.Task
}

// ApplySort applies sorting to a task list (stable sort)
func ApplySort(tasks []data.Task, state SortState) []data.Task {
	if state.Field == SortByNone {
		return tasks
	}

	// Create a copy to avoid modifying the original
	result := make([]data.Task, len(tasks))
	copy(result, tasks)

	sort.SliceStable(result, func(i, j int) bool {
		cmp := compareTasksBy(result[i], result[j], state.Field)
		if state.Ascending {
			return cmp < 0
		}
		return cmp > 0
	})

	return result
}

func compareTasksBy(a, b data.Task, field SortField) int {
	switch field {
	case SortByDueDate:
		dateA := a.GetDueDate()
		dateB := b.GetDueDate()
		// Empty dates sort to the end
		if dateA == "" && dateB == "" {
			return 0
		}
		if dateA == "" {
			return 1
		}
		if dateB == "" {
			return -1
		}
		return strings.Compare(dateA, dateB)

	case SortByProject:
		// Use first project alphabetically
		projA := getFirstProject(a)
		projB := getFirstProject(b)
		if projA == "" && projB == "" {
			return 0
		}
		if projA == "" {
			return 1
		}
		if projB == "" {
			return -1
		}
		return strings.Compare(strings.ToLower(projA), strings.ToLower(projB))

	case SortByPriority:
		// A < B < C < ... < none
		priA := a.Priority
		priB := b.Priority
		if priA == 0 && priB == 0 {
			return 0
		}
		if priA == 0 {
			return 1
		}
		if priB == 0 {
			return -1
		}
		return int(priA) - int(priB)

	case SortByContext:
		// Use first context alphabetically
		ctxA := getFirstContext(a)
		ctxB := getFirstContext(b)
		if ctxA == "" && ctxB == "" {
			return 0
		}
		if ctxA == "" {
			return 1
		}
		if ctxB == "" {
			return -1
		}
		return strings.Compare(strings.ToLower(ctxA), strings.ToLower(ctxB))
	}

	return 0
}

func getFirstProject(t data.Task) string {
	if len(t.Projects) == 0 {
		return ""
	}
	// Projects should already be sorted
	return t.Projects[0]
}

func getFirstContext(t data.Task) string {
	if len(t.Contexts) == 0 {
		return ""
	}
	// Contexts should already be sorted
	return t.Contexts[0]
}

// ApplyGroups groups tasks by the specified field
// Tasks with multiple values (projects/contexts) appear in multiple groups
func ApplyGroups(tasks []data.Task, state GroupState, roots []string) []TaskGroup {
	if state.Field == GroupByNone {
		return []TaskGroup{{Label: "", Tasks: tasks}}
	}

	// Build groups
	groupMap := make(map[string][]data.Task)
	var groupOrder []string

	for _, task := range tasks {
		keys := getGroupKeys(task, state.Field, roots)
		for _, key := range keys {
			if _, exists := groupMap[key]; !exists {
				groupOrder = append(groupOrder, key)
			}
			groupMap[key] = append(groupMap[key], task)
		}
	}

	// Sort group keys
	sort.Slice(groupOrder, func(i, j int) bool {
		cmp := compareGroupKeys(groupOrder[i], groupOrder[j], state.Field)
		if state.Ascending {
			return cmp < 0
		}
		return cmp > 0
	})

	// Build result
	var result []TaskGroup
	for _, key := range groupOrder {
		label := key
		if label == "" {
			label = "(none)"
		}
		result = append(result, TaskGroup{
			Label: label,
			Tasks: groupMap[key],
		})
	}

	return result
}

func getGroupKeys(task data.Task, field GroupField, roots []string) []string {
	switch field {
	case GroupByDueDate:
		due := task.GetDueDate()
		if due == "" {
			return []string{""}
		}
		return []string{due}

	case GroupByProject:
		if len(task.Projects) == 0 {
			return []string{""}
		}
		return task.Projects

	case GroupByPriority:
		if task.Priority == 0 {
			return []string{""}
		}
		return []string{string(task.Priority)}

	case GroupByContext:
		if len(task.Contexts) == 0 {
			return []string{""}
		}
		return task.Contexts

	case GroupByFile:
		return []string{RelativeFilePath(task.File, roots)}
	}

	return []string{""}
}

func compareGroupKeys(a, b string, field GroupField) int {
	// Empty keys sort to the end
	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}

	// For priority, compare as runes (A < B < C)
	if field == GroupByPriority {
		return int(a[0]) - int(b[0])
	}

	// For dates, string comparison works (ISO format)
	// For text fields, case-insensitive comparison
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// ExtractUniqueProjects returns all unique project names from tasks
func ExtractUniqueProjects(tasks []data.Task) []string {
	seen := make(map[string]bool)
	var result []string
	for _, task := range tasks {
		for _, p := range task.Projects {
			if !seen[p] {
				seen[p] = true
				result = append(result, p)
			}
		}
	}
	sort.Strings(result)
	return result
}

// ExtractUniqueContexts returns all unique context names from tasks
func ExtractUniqueContexts(tasks []data.Task) []string {
	seen := make(map[string]bool)
	var result []string
	for _, task := range tasks {
		for _, c := range task.Contexts {
			if !seen[c] {
				seen[c] = true
				result = append(result, c)
			}
		}
	}
	sort.Strings(result)
	return result
}

// ExtractUniqueFiles returns all unique relative file paths from tasks
func ExtractUniqueFiles(tasks []data.Task, roots []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, task := range tasks {
		relPath := RelativeFilePath(task.File, roots)
		if !seen[relPath] {
			seen[relPath] = true
			result = append(result, relPath)
		}
	}
	sort.Strings(result)
	return result
}

// RelativeFilePath returns a path relative to the first matching workspace root.
// When multiple roots are configured, the path is prefixed with the workspace name
// (e.g. "work: tasks/todo.txt") so that identical relative paths from different
// workspaces are disambiguated in group labels and file filter items.
// Falls back to the bare filename if no root matches.
func RelativeFilePath(absPath string, roots []string) string {
	for _, root := range roots {
		rel, err := filepath.Rel(root, absPath)
		if err == nil && !strings.HasPrefix(rel, "..") {
			if len(roots) > 1 {
				return WorkspaceName(root, roots) + ": " + rel
			}
			return rel
		}
	}
	// Fallback: bare filename
	return filepath.Base(absPath)
}

// WorkspaceName returns a short display name for a workspace root.
// It uses filepath.Base(root) unless two roots share the same basename,
// in which case it returns "parent/base" for disambiguation.
func WorkspaceName(root string, allRoots []string) string {
	base := filepath.Base(root)
	for _, other := range allRoots {
		if other != root && filepath.Base(other) == base {
			// Collision — use parent/base
			return filepath.Base(filepath.Dir(root)) + "/" + base
		}
	}
	return base
}

// WorkspaceForTask returns the workspace display name that owns the given
// absolute file path. Returns "" if no root matches.
func WorkspaceForTask(absPath string, roots []string) string {
	for _, root := range roots {
		rel, err := filepath.Rel(root, absPath)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return WorkspaceName(root, roots)
		}
	}
	return ""
}

// ExtractUniqueWorkspaces returns sorted workspace display names that have
// at least one task.
func ExtractUniqueWorkspaces(tasks []data.Task, roots []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, task := range tasks {
		name := WorkspaceForTask(task.File, roots)
		if name != "" && !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}
