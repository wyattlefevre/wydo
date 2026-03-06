package notes

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const PinnedFilename = "pinned.md"

// PinnedNote is a labeled link to a file, stored in a workspace's pinned.md.
type PinnedNote struct {
	Label   string
	RelPath string // relative to workspace root
	AbsPath string // absolute path
	WsRoot  string
}

var mdLinkRe = regexp.MustCompile(`^\[([^\]]+)\]\(([^)]+)\)\s*$`)

// ReadPinnedNotes reads pinned.md from wsRoot and returns pinned notes.
// Returns nil slice (no error) if the file does not exist.
func ReadPinnedNotes(wsRoot string) ([]PinnedNote, error) {
	path := filepath.Join(wsRoot, PinnedFilename)
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var result []PinnedNote
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		m := mdLinkRe.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		result = append(result, PinnedNote{
			Label:   m[1],
			RelPath: m[2],
			AbsPath: filepath.Join(wsRoot, m[2]),
			WsRoot:  wsRoot,
		})
	}
	return result, sc.Err()
}

// RemovePinnedNote removes the entry with the given relPath from wsRoot/pinned.md.
func RemovePinnedNote(wsRoot, relPath string) error {
	existing, err := ReadPinnedNotes(wsRoot)
	if err != nil {
		return err
	}
	path := filepath.Join(wsRoot, PinnedFilename)
	var lines []string
	for _, n := range existing {
		if n.RelPath != relPath {
			lines = append(lines, fmt.Sprintf("[%s](%s)\n", n.Label, n.RelPath))
		}
	}
	if len(lines) == 0 {
		return os.Remove(path)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "")), 0644)
}

// AddPinnedNote appends a new pinned note to wsRoot/pinned.md.
func AddPinnedNote(wsRoot, label, relPath string) error {
	path := filepath.Join(wsRoot, PinnedFilename)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open pinned.md: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "[%s](%s)\n", label, relPath)
	return err
}

// ListWorkspaceFiles returns all files under wsRoot (relative paths), skipping
// hidden directories and common non-document files.
func ListWorkspaceFiles(wsRoot string) ([]string, error) {
	var result []string
	err := filepath.Walk(wsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		name := info.Name()
		// Skip hidden directories and the .git directory
		if info.IsDir() {
			if strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		// Only include files with common text extensions
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".md", ".txt", ".org", ".rst", ".adoc":
			// skip board.md files inside cards/ directories
			rel, err := filepath.Rel(wsRoot, path)
			if err != nil {
				return nil
			}
			result = append(result, rel)
		}
		return nil
	})
	return result, err
}
