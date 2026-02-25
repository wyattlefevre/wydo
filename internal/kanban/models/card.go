package models

import "time"

// CardURL represents a URL with an optional label
type CardURL struct {
	Label string `yaml:"label,omitempty"`
	URL   string `yaml:"url"`
}

// Card represents a kanban card with frontmatter metadata
type Card struct {
	Filename      string     // Filename in the cards directory
	Title         string     // Extracted from first H1 in markdown
	Tags          []string   // From YAML frontmatter
	Projects      []string   // From YAML frontmatter
	URLs          []CardURL  // From YAML frontmatter
	Preview       string     // First few lines of content
	Content       string     // Full markdown content (without frontmatter)
	DueDate       *time.Time // From YAML frontmatter (ISO 8601 date)
	ScheduledDate *time.Time // From YAML frontmatter (ISO 8601 date)
	DateCompleted *time.Time // From YAML frontmatter (RFC3339 datetime)
	Priority      int        // From YAML frontmatter (0 = unset)
	Archived      bool       // From YAML frontmatter
	TmuxSession   string     // From YAML frontmatter
}

// HasURLs returns true if the card has at least one URL
func (c Card) HasURLs() bool {
	return len(c.URLs) > 0
}

// FirstURL returns the URL string of the first URL, or empty string
func (c Card) FirstURL() string {
	if len(c.URLs) > 0 {
		return c.URLs[0].URL
	}
	return ""
}
