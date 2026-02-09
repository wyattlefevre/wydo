package tasks

// InputMode represents the current input handling mode
type InputMode int

const (
	ModeNormal InputMode = iota

	// Top-level category selection
	ModeFilterSelect // 'f' pressed - choosing filter type
	ModeSortSelect   // 's' pressed - choosing sort field
	ModeGroupSelect  // 'g' pressed - choosing group field

	// Sub-modes for direction selection
	ModeSortDirection  // after selecting sort field, choose asc/desc
	ModeGroupDirection // after selecting group field, choose asc/desc

	// Active input modes
	ModeSearch      // '/' - fuzzy text search
	ModeDateInput   // entering date for filter
	ModeFuzzyPicker // generic picker for project/context/file
	ModeCreateTask  // 'n' pressed - entering new task name

	// Task Editor modes
	ModeTaskEditor        // viewing task details
	ModeEditDueDate       // 'd' in editor - date picker
	ModeEditScheduledDate // 'S' in editor - scheduled date picker
	ModeEditContext       // 't'/'c' in editor - context picker
	ModeEditProject       // 'p' in editor - project picker

	// Confirmation mode
	ModeConfirmation // confirmation modal (e.g., archive)
)

// InputModeContext holds the current mode and related context
type InputModeContext struct {
	Mode         InputMode
	PreviousMode InputMode
	Category     string // "filter", "sort", "group"
	Field        string // "date", "project", "priority", "context", "status", "file"
	Direction    string // "asc", "desc"
}

// NewInputModeContext creates a new context in normal mode
func NewInputModeContext() InputModeContext {
	return InputModeContext{
		Mode: ModeNormal,
	}
}

// IsNormal returns true if in normal navigation mode
func (c *InputModeContext) IsNormal() bool {
	return c.Mode == ModeNormal
}

// IsFilterMode returns true if in any filter-related mode
func (c *InputModeContext) IsFilterMode() bool {
	return c.Mode == ModeFilterSelect || c.Mode == ModeSearch ||
		c.Mode == ModeDateInput || c.Mode == ModeFuzzyPicker
}

// IsSortMode returns true if in any sort-related mode
func (c *InputModeContext) IsSortMode() bool {
	return c.Mode == ModeSortSelect || c.Mode == ModeSortDirection
}

// IsGroupMode returns true if in any group-related mode
func (c *InputModeContext) IsGroupMode() bool {
	return c.Mode == ModeGroupSelect || c.Mode == ModeGroupDirection
}

// IsEditorMode returns true if in task editor mode
func (c *InputModeContext) IsEditorMode() bool {
	return c.Mode == ModeTaskEditor || c.Mode == ModeEditDueDate ||
		c.Mode == ModeEditScheduledDate || c.Mode == ModeEditContext ||
		c.Mode == ModeEditProject
}

// TransitionTo moves to a new mode, preserving the previous mode
func (c *InputModeContext) TransitionTo(mode InputMode) {
	c.PreviousMode = c.Mode
	c.Mode = mode
}

// Back returns to the previous mode
func (c *InputModeContext) Back() {
	c.Mode = c.PreviousMode
	c.PreviousMode = ModeNormal
}

// Reset returns to normal mode and clears context
func (c *InputModeContext) Reset() {
	c.Mode = ModeNormal
	c.PreviousMode = ModeNormal
	c.Category = ""
	c.Field = ""
	c.Direction = ""
}

// String returns a display name for the current mode
func (c *InputModeContext) String() string {
	switch c.Mode {
	case ModeNormal:
		return "Normal"
	case ModeFilterSelect:
		return "Filter"
	case ModeSortSelect:
		return "Sort"
	case ModeGroupSelect:
		return "Group"
	case ModeSortDirection, ModeGroupDirection:
		return "Direction"
	case ModeSearch:
		return "Search"
	case ModeDateInput:
		return "Date"
	case ModeFuzzyPicker:
		return "Pick"
	case ModeTaskEditor:
		return "Editor"
	case ModeEditDueDate:
		return "Edit Due"
	case ModeEditScheduledDate:
		return "Edit Scheduled"
	case ModeEditContext:
		return "Edit Context"
	case ModeEditProject:
		return "Edit Project"
	case ModeConfirmation:
		return "Confirmation"
	case ModeCreateTask:
		return "Create"
	default:
		return "Unknown"
	}
}
