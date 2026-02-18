package tasks

import (
	"strings"
	"time"

	"wydo/internal/tasks/data"
)

// StatusFilter represents filtering by task completion status
type StatusFilter int

const (
	StatusAll StatusFilter = iota
	StatusPending
	StatusDone
)

// DateFilterMode represents how to compare dates
type DateFilterMode int

const (
	DateFilterNone DateFilterMode = iota
	DateBefore
	DateOn
	DateAfter
	DateMissing
)

// DateFilter holds date filtering configuration
type DateFilter struct {
	Mode DateFilterMode
	Date time.Time
}

// FilterState holds all active filters
type FilterState struct {
	SearchQuery     string
	StatusFilter    StatusFilter
	DateFilter      *DateFilter
	ProjectFilter   []string
	ContextFilter   []string
	PriorityFilter  []data.Priority
	FileFilter      []string
	WorkspaceFilter []string // workspace basenames
}

// NewFilterState creates a new empty filter state
func NewFilterState() FilterState {
	return FilterState{
		StatusFilter: StatusPending,
	}
}

// IsEmpty returns true if no filters are active
func (f *FilterState) IsEmpty() bool {
	return f.SearchQuery == "" &&
		f.StatusFilter == StatusAll &&
		f.DateFilter == nil &&
		len(f.ProjectFilter) == 0 &&
		len(f.ContextFilter) == 0 &&
		len(f.PriorityFilter) == 0 &&
		len(f.FileFilter) == 0 &&
		len(f.WorkspaceFilter) == 0
}

// Reset clears all filters
func (f *FilterState) Reset() {
	f.SearchQuery = ""
	f.StatusFilter = StatusAll
	f.DateFilter = nil
	f.ProjectFilter = nil
	f.ContextFilter = nil
	f.PriorityFilter = nil
	f.FileFilter = nil
	f.WorkspaceFilter = nil
}

// CycleStatusFilter cycles through status filter options
func (f *FilterState) CycleStatusFilter() {
	switch f.StatusFilter {
	case StatusAll:
		f.StatusFilter = StatusPending
	case StatusPending:
		f.StatusFilter = StatusDone
	case StatusDone:
		f.StatusFilter = StatusAll
	}
}

// ApplyFilters applies all active filters to a task list
func ApplyFilters(tasks []data.Task, state FilterState) []data.Task {
	if state.IsEmpty() {
		return tasks
	}

	var result []data.Task
	for _, task := range tasks {
		if matchesFilters(task, state) {
			result = append(result, task)
		}
	}
	return result
}

func matchesFilters(task data.Task, state FilterState) bool {
	// Search filter (fuzzy match on name)
	if state.SearchQuery != "" {
		if !fuzzyMatch(task.Name, state.SearchQuery) {
			return false
		}
	}

	// Status filter
	switch state.StatusFilter {
	case StatusPending:
		if task.Done {
			return false
		}
	case StatusDone:
		if !task.Done {
			return false
		}
	}

	// Date filter
	if state.DateFilter != nil {
		if !matchesDateFilter(task, state.DateFilter) {
			return false
		}
	}

	// Project filter (task must have at least one matching project)
	if len(state.ProjectFilter) > 0 {
		if !matchesAnyProject(task, state.ProjectFilter) {
			return false
		}
	}

	// Context filter (task must have at least one matching context)
	if len(state.ContextFilter) > 0 {
		if !matchesAnyContext(task, state.ContextFilter) {
			return false
		}
	}

	// Priority filter
	if len(state.PriorityFilter) > 0 {
		if !matchesPriority(task, state.PriorityFilter) {
			return false
		}
	}

	// File filter
	if len(state.FileFilter) > 0 {
		if !matchesFile(task, state.FileFilter) {
			return false
		}
	}

	return true
}

func fuzzyMatch(s, pattern string) bool {
	s = strings.ToLower(s)
	pattern = strings.ToLower(pattern)
	if pattern == "" {
		return true
	}
	// fzf-style sequential character matching:
	// characters must appear in order but not necessarily adjacent
	// e.g., "bgr" matches "buy groceries"
	pIdx := 0
	for i := 0; i < len(s) && pIdx < len(pattern); i++ {
		if s[i] == pattern[pIdx] {
			pIdx++
		}
	}
	return pIdx == len(pattern)
}

func matchesDateFilter(task data.Task, filter *DateFilter) bool {
	dueDate := task.GetDueDate()

	if filter.Mode == DateMissing {
		return dueDate == ""
	}

	if dueDate == "" {
		return false
	}

	taskDate, err := time.Parse("2006-01-02", dueDate)
	if err != nil {
		return false
	}

	switch filter.Mode {
	case DateBefore:
		return taskDate.Before(filter.Date)
	case DateOn:
		return taskDate.Year() == filter.Date.Year() &&
			taskDate.Month() == filter.Date.Month() &&
			taskDate.Day() == filter.Date.Day()
	case DateAfter:
		return taskDate.After(filter.Date)
	}

	return true
}

func matchesAnyProject(task data.Task, projects []string) bool {
	for _, p := range projects {
		if task.HasProject(p) {
			return true
		}
	}
	return false
}

func matchesAnyContext(task data.Task, contexts []string) bool {
	for _, c := range contexts {
		if task.HasContext(c) {
			return true
		}
	}
	return false
}

func matchesPriority(task data.Task, priorities []data.Priority) bool {
	for _, p := range priorities {
		if task.Priority == p {
			return true
		}
	}
	return false
}

func matchesFile(task data.Task, files []string) bool {
	for _, f := range files {
		// Strip workspace prefix ("name: ") if present so suffix matching
		// works with both prefixed display names and raw paths.
		clean := f
		if idx := strings.Index(f, ": "); idx >= 0 {
			clean = f[idx+2:]
		}
		if strings.HasSuffix(task.File, clean) {
			return true
		}
	}
	return false
}

// StatusFilterString returns a display string for the status filter
func (f *FilterState) StatusFilterString() string {
	switch f.StatusFilter {
	case StatusPending:
		return "pending"
	case StatusDone:
		return "done"
	default:
		return ""
	}
}

// Summary returns a human-readable summary of active filters
func (f *FilterState) Summary() string {
	var parts []string

	if f.StatusFilter != StatusAll {
		parts = append(parts, "status="+f.StatusFilterString())
	}

	if len(f.ProjectFilter) > 0 {
		parts = append(parts, "project="+strings.Join(f.ProjectFilter, ","))
	}

	if len(f.ContextFilter) > 0 {
		parts = append(parts, "context="+strings.Join(f.ContextFilter, ","))
	}

	if len(f.PriorityFilter) > 0 {
		var ps []string
		for _, p := range f.PriorityFilter {
			ps = append(ps, string(p))
		}
		parts = append(parts, "priority="+strings.Join(ps, ","))
	}

	if f.DateFilter != nil {
		var mode string
		switch f.DateFilter.Mode {
		case DateBefore:
			mode = "before"
		case DateOn:
			mode = "on"
		case DateAfter:
			mode = "after"
		case DateMissing:
			mode = "missing"
		}
		if f.DateFilter.Mode == DateMissing {
			parts = append(parts, "due:"+mode)
		} else {
			parts = append(parts, "due:"+mode+" "+f.DateFilter.Date.Format("2006-01-02"))
		}
	}

	if len(f.FileFilter) > 0 {
		parts = append(parts, "file="+strings.Join(f.FileFilter, ","))
	}

	if len(f.WorkspaceFilter) > 0 {
		parts = append(parts, "workspace="+strings.Join(f.WorkspaceFilter, ","))
	}

	return strings.Join(parts, " | ")
}

// ApplyWorkspaceFilter filters tasks to only those belonging to the selected
// workspaces. Tasks with an empty File field are always passed through.
func ApplyWorkspaceFilter(tasks []data.Task, workspaces []string, roots []string) []data.Task {
	if len(workspaces) == 0 {
		return tasks
	}
	allowed := make(map[string]bool, len(workspaces))
	for _, w := range workspaces {
		allowed[w] = true
	}
	var result []data.Task
	for _, task := range tasks {
		if task.File == "" {
			result = append(result, task)
			continue
		}
		if allowed[WorkspaceForTask(task.File, roots)] {
			result = append(result, task)
		}
	}
	return result
}
