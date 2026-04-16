package models

import (
	"strings"
	"time"
)

// TaskNoteURL represents a URL with an optional label
type TaskNoteURL struct {
	Label string `yaml:"label,omitempty"`
	URL   string `yaml:"url"`
}

// TaskNote represents a kanban task note with frontmatter metadata
type TaskNote struct {
	Filename      string        // Base filename of the task note file
	FilePath      string        // Absolute path to the task note file
	Title         string        // Extracted from first H1 in markdown
	Tags          []string      // From YAML frontmatter
	Projects      []string      // From YAML frontmatter
	URLs          []TaskNoteURL // From YAML frontmatter
	Preview       string        // First few lines of content
	Content       string        // Full markdown content (without frontmatter)
	DueDate       *time.Time    // From YAML frontmatter (ISO 8601 date)
	ScheduledDate *time.Time    // From YAML frontmatter (ISO 8601 date)
	DateCompleted *time.Time    // From YAML frontmatter (RFC3339 datetime)
	Priority      int           // From YAML frontmatter (0 = unset)
	Archived      bool          // Derived from file path (true if under archive/boards/)
	Status        string        // From YAML frontmatter; current column/status name
	Board         string        // From YAML frontmatter; board this task belongs to
	TmuxSession   string        // From YAML frontmatter
	JiraKey       string        // From YAML frontmatter (e.g. "PROJ-123")
	JiraStatus    string        // From YAML frontmatter (cached Jira status)
}

// TaskNotePriorityLabel converts an int priority (1-6) to its letter label ("A"-"F").
// Returns "" for 0 or any out-of-range value.
func TaskNotePriorityLabel(p int) string {
	if p < 1 || p > 6 {
		return ""
	}
	return string(rune('A' + p - 1))
}

// TaskNotePriorityFromLetter converts a priority letter ("A"-"F", case-insensitive) to
// its int value (1-6). Returns 0 for any unrecognized input.
func TaskNotePriorityFromLetter(s string) int {
	if len(s) != 1 {
		return 0
	}
	r := rune(strings.ToUpper(s)[0])
	if r < 'A' || r > 'F' {
		return 0
	}
	return int(r-'A') + 1
}

// HasURLs returns true if the task note has at least one URL
func (c TaskNote) HasURLs() bool {
	return len(c.URLs) > 0
}

// FirstURL returns the URL string of the first URL, or empty string
func (c TaskNote) FirstURL() string {
	if len(c.URLs) > 0 {
		return c.URLs[0].URL
	}
	return ""
}
