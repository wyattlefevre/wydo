// migrate-archive converts legacy frontmatter-based archiving to path-based archiving.
//
// Before: archived boards/cards had `archived: true` in their YAML frontmatter
//         and stayed in boards/<name>/ or boards/<name>/cards/
// After:  archived boards live in archive/boards/<name>/
//         archived cards live in archive/boards/<name>/cards/
//
// Usage:
//
//	go run ./cmd/migrate-archive <workspace-path>
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: migrate-archive <workspace-path>")
		os.Exit(1)
	}

	wsRoot, err := filepath.Abs(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := migrate(wsRoot); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func migrate(wsRoot string) error {
	boardsDir := filepath.Join(wsRoot, "boards")
	archiveBoardsDir := filepath.Join(wsRoot, "archive", "boards")

	if err := os.MkdirAll(archiveBoardsDir, 0755); err != nil {
		return fmt.Errorf("create archive/boards: %w", err)
	}

	entries, err := os.ReadDir(boardsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No boards/ directory found — nothing to migrate.")
			return nil
		}
		return fmt.Errorf("read boards/: %w", err)
	}

	migratedBoards := 0
	migratedCards := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		boardName := entry.Name()
		boardPath := filepath.Join(boardsDir, boardName)
		boardMDPath := filepath.Join(boardPath, "board.md")

		boardArchived, err := isArchivedInFrontmatter(boardMDPath)
		if err != nil {
			fmt.Printf("  warning: skipping board %s: %v\n", boardName, err)
			continue
		}

		if boardArchived {
			if err := removeFMField(boardMDPath, "archived"); err != nil {
				fmt.Printf("  warning: could not clean board.md for %s: %v\n", boardName, err)
			}

			destPath := filepath.Join(archiveBoardsDir, boardName)
			if err := os.Rename(boardPath, destPath); err != nil {
				return fmt.Errorf("move board %s: %w", boardName, err)
			}
			fmt.Printf("  board  boards/%s → archive/boards/%s\n", boardName, boardName)
			migratedBoards++
			continue
		}

		// Board is active — look for archived cards within it
		cardsDir := filepath.Join(boardPath, "cards")
		cardEntries, err := os.ReadDir(cardsDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read boards/%s/cards/: %w", boardName, err)
		}

		columnMap, err := buildColumnMap(boardMDPath)
		if err != nil {
			fmt.Printf("  warning: could not parse board.md for %s: %v\n", boardName, err)
		}

		for _, cardEntry := range cardEntries {
			if cardEntry.IsDir() || !strings.HasSuffix(cardEntry.Name(), ".md") {
				continue
			}

			filename := cardEntry.Name()
			cardPath := filepath.Join(cardsDir, filename)

			archived, err := isArchivedInFrontmatter(cardPath)
			if err != nil || !archived {
				continue
			}

			column := columnMap[filename]

			if err := migrateCardFM(cardPath, column); err != nil {
				fmt.Printf("  warning: could not update frontmatter for %s/%s: %v\n", boardName, filename, err)
				continue
			}

			archiveCardsDir := filepath.Join(archiveBoardsDir, boardName, "cards")
			if err := os.MkdirAll(archiveCardsDir, 0755); err != nil {
				return fmt.Errorf("create archive cards dir for %s: %w", boardName, err)
			}

			destCardPath := filepath.Join(archiveCardsDir, filename)
			if err := os.Rename(cardPath, destCardPath); err != nil {
				return fmt.Errorf("move card %s/%s: %w", boardName, filename, err)
			}

			if err := removeCardLink(boardMDPath, filename); err != nil {
				fmt.Printf("  warning: could not remove link from board.md for %s/%s: %v\n", boardName, filename, err)
			}

			col := column
			if col == "" {
				col = "(unknown column)"
			}
			fmt.Printf("  card   boards/%s/cards/%s → archive/boards/%s/cards/%s  [was in %q]\n",
				boardName, filename, boardName, filename, col)
			migratedCards++
		}
	}

	fmt.Printf("\nDone. Migrated %d board(s), %d card(s).\n", migratedBoards, migratedCards)
	return nil
}

// isArchivedInFrontmatter returns true if the file has `archived: true` in its YAML frontmatter.
func isArchivedInFrontmatter(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	fm := readRawFM(content)
	if fm == nil {
		return false, nil
	}

	val, ok := fm["archived"]
	if !ok {
		return false, nil
	}

	if b, ok := val.(bool); ok {
		return b, nil
	}
	return false, nil
}

// readRawFM parses YAML frontmatter into a raw map. Returns nil if none found.
func readRawFM(content []byte) map[string]interface{} {
	lines := bytes.Split(content, []byte("\n"))
	if len(lines) == 0 || !bytes.Equal(bytes.TrimSpace(lines[0]), []byte("---")) {
		return nil
	}

	var end int
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(bytes.TrimSpace(lines[i]), []byte("---")) {
			end = i
			break
		}
	}
	if end == 0 {
		return nil
	}

	raw := bytes.Join(lines[1:end], []byte("\n"))
	var m map[string]interface{}
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

// removeFMField removes a single key from the YAML frontmatter of a file, preserving all else.
func removeFMField(path, field string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	fm := readRawFM(content)
	if fm == nil {
		return nil
	}
	if _, ok := fm[field]; !ok {
		return nil
	}

	delete(fm, field)
	return rewriteWithFM(path, content, fm)
}

// migrateCardFM removes `archived: true` and writes `column: <name>` in a card's frontmatter.
func migrateCardFM(path, column string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	fm := readRawFM(content)
	if fm == nil {
		fm = make(map[string]interface{})
	}

	delete(fm, "archived")
	if column != "" {
		fm["column"] = column
	}

	return rewriteWithFM(path, content, fm)
}

// rewriteWithFM writes the file with a new frontmatter map, preserving the body.
func rewriteWithFM(path string, original []byte, fm map[string]interface{}) error {
	lines := bytes.Split(original, []byte("\n"))

	// Find body start
	bodyStart := 0
	if len(lines) > 0 && bytes.Equal(bytes.TrimSpace(lines[0]), []byte("---")) {
		for i := 1; i < len(lines); i++ {
			if bytes.Equal(bytes.TrimSpace(lines[i]), []byte("---")) {
				bodyStart = i + 1
				break
			}
		}
	}

	body := bytes.TrimLeft(bytes.Join(lines[bodyStart:], []byte("\n")), "\n")

	var buf bytes.Buffer
	if len(fm) > 0 {
		buf.WriteString("---\n")
		yamlBytes, err := yaml.Marshal(fm)
		if err != nil {
			return err
		}
		buf.Write(yamlBytes)
		buf.WriteString("---\n\n")
	}
	buf.Write(body)

	return os.WriteFile(path, buf.Bytes(), 0644)
}

// buildColumnMap parses board.md and returns a map of card filename → column name.
func buildColumnMap(boardMDPath string) (map[string]string, error) {
	content, err := os.ReadFile(boardMDPath)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	var currentColumn string

	for _, rawLine := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(rawLine)

		if strings.HasPrefix(line, "## ") {
			currentColumn = strings.TrimPrefix(line, "## ")
			continue
		}

		// Match markdown links like [Title](./cards/filename.md)
		if i := strings.Index(line, "]("); i != -1 {
			link := line[i+2:]
			link = strings.TrimSuffix(link, ")")
			filename := filepath.Base(link)
			if strings.HasSuffix(filename, ".md") && currentColumn != "" {
				result[filename] = currentColumn
			}
		}
	}

	return result, nil
}

// removeCardLink deletes the markdown link line for the given card from board.md,
// collapsing any resulting double-blank lines.
func removeCardLink(boardMDPath, cardFilename string) error {
	content, err := os.ReadFile(boardMDPath)
	if err != nil {
		return err
	}

	var kept []string
	for _, line := range strings.Split(string(content), "\n") {
		if strings.Contains(line, "](./cards/"+cardFilename+")") ||
			strings.Contains(line, "](cards/"+cardFilename+")") {
			continue
		}
		kept = append(kept, line)
	}

	// Collapse consecutive blank lines to at most one
	var cleaned []string
	prevBlank := false
	for _, line := range kept {
		isBlank := strings.TrimSpace(line) == ""
		if isBlank && prevBlank {
			continue
		}
		cleaned = append(cleaned, line)
		prevBlank = isBlank
	}

	return os.WriteFile(boardMDPath, []byte(strings.Join(cleaned, "\n")), 0644)
}
