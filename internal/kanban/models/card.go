package models

import "time"

// Card represents a kanban card with frontmatter metadata
type Card struct {
	Filename      string     // Filename in the cards directory
	Title         string     // Extracted from first H1 in markdown
	Tags          []string   // From YAML frontmatter
	Projects      []string   // From YAML frontmatter
	URL           string     // From YAML frontmatter
	Preview       string     // First few lines of content
	Content       string     // Full markdown content (without frontmatter)
	DueDate       *time.Time // From YAML frontmatter (ISO 8601 date)
	ScheduledDate *time.Time // From YAML frontmatter (ISO 8601 date)
	DateCompleted *time.Time // From YAML frontmatter (RFC3339 datetime)
	Priority      int        // From YAML frontmatter (0 = unset)
	Archived      bool       // From YAML frontmatter
}
