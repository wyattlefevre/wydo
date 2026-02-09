package notes

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var datePattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// ParseNoteFile parses a markdown file as a Note. Returns the note and true if
// the file has a valid date (from frontmatter or filename), otherwise false.
func ParseNoteFile(absPath, rootDir string) (Note, bool) {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return Note{}, false
	}

	filename := filepath.Base(absPath)
	relPath, err := filepath.Rel(rootDir, absPath)
	if err != nil {
		relPath = filename
	}

	var noteDate time.Time
	var title string
	hasDate := false

	// Try frontmatter first
	fmDate, fmTitle := parseFrontmatter(content)
	if !fmDate.IsZero() {
		noteDate = fmDate
		hasDate = true
	}
	if fmTitle != "" {
		title = fmTitle
	}

	// Try filename date (used if frontmatter has no date)
	if !hasDate {
		if match := datePattern.FindString(filename); match != "" {
			if parsed, err := time.Parse("2006-01-02", match); err == nil {
				noteDate = parsed
				hasDate = true
			}
		}
	}

	if !hasDate {
		return Note{}, false
	}

	// Derive title from filename if not found in frontmatter
	if title == "" {
		title = titleFromFilename(filename)
	}

	return Note{
		Title:    title,
		FilePath: absPath,
		RelPath:  relPath,
		Date:     noteDate,
	}, true
}

type noteFrontmatter struct {
	Date  string `yaml:"date"`
	Title string `yaml:"title"`
}

func parseFrontmatter(content []byte) (time.Time, string) {
	lines := bytes.Split(content, []byte("\n"))

	if len(lines) == 0 || !bytes.Equal(bytes.TrimSpace(lines[0]), []byte("---")) {
		return time.Time{}, ""
	}

	var fmEnd int
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(bytes.TrimSpace(lines[i]), []byte("---")) {
			fmEnd = i
			break
		}
	}

	if fmEnd == 0 {
		return time.Time{}, ""
	}

	fmBytes := bytes.Join(lines[1:fmEnd], []byte("\n"))
	var fm noteFrontmatter
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		return time.Time{}, ""
	}

	var date time.Time
	if fm.Date != "" {
		if parsed, err := time.Parse("2006-01-02", fm.Date); err == nil {
			date = parsed
		}
	}

	return date, fm.Title
}

func titleFromFilename(filename string) string {
	// Strip .md extension
	name := strings.TrimSuffix(filename, ".md")

	// Strip leading date pattern (e.g. "2026-02-14-")
	if loc := datePattern.FindStringIndex(name); loc != nil {
		after := name[loc[1]:]
		if strings.HasPrefix(after, "-") {
			after = after[1:]
		}
		if after != "" {
			name = after
		}
	}

	// Replace - and _ with spaces
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	if name == "" {
		return "Note"
	}

	return name
}
