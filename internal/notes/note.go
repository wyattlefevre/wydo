package notes

import "time"

// Note represents a markdown note with a date
type Note struct {
	Title    string    // From frontmatter `title`, or derived from filename
	FilePath string    // Absolute path to file
	RelPath  string    // Path relative to scanned dir root (for display)
	Date     time.Time // From frontmatter `date`, or parsed from filename
}
