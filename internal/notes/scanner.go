package notes

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var datePattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// ScanNotes finds all dated markdown notes in configured directories
func ScanNotes(dirs, recursiveDirs []string) []Note {
	var notes []Note
	seen := make(map[string]bool)

	// Scan explicit directories (non-recursive, top-level .md files only)
	for _, dir := range dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		entries, err := os.ReadDir(absDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
				continue
			}
			if entry.Name() == "board.md" {
				continue
			}
			fullPath := filepath.Join(absDir, entry.Name())
			absPath, _ := filepath.Abs(fullPath)
			if seen[absPath] {
				continue
			}
			if note, ok := parseNoteFile(absPath, absDir); ok {
				seen[absPath] = true
				notes = append(notes, note)
			}
		}
	}

	// Scan recursive directories
	for _, root := range recursiveDirs {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				if os.IsPermission(err) {
					return filepath.SkipDir
				}
				return err
			}

			// Skip cards/ subdirectories
			if d.IsDir() && d.Name() == "cards" {
				return filepath.SkipDir
			}

			if d.IsDir() {
				return nil
			}

			if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
				return nil
			}

			// Skip board.md files
			if d.Name() == "board.md" {
				return nil
			}

			absPath, _ := filepath.Abs(path)
			if seen[absPath] {
				return nil
			}

			if note, ok := parseNoteFile(absPath, absRoot); ok {
				seen[absPath] = true
				notes = append(notes, note)
			}

			return nil
		})
		if err != nil {
			log.Printf("Warning: error scanning for notes in %s: %v", absRoot, err)
		}
	}

	return notes
}

func parseNoteFile(absPath, rootDir string) (Note, bool) {
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
