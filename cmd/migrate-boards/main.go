package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

func main() {
	wsFlag := flag.String("workspace", ".", "path to workspace root")
	flag.Parse()

	wsRoot, err := filepath.Abs(*wsFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := migrateWorkspace(wsRoot); err != nil {
		fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Migration complete.")
}

func migrateWorkspace(wsRoot string) error {
	// Ensure tasks/ dir exists
	tasksDir := filepath.Join(wsRoot, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		return err
	}

	boardsDir := filepath.Join(wsRoot, "boards")
	entries, err := os.ReadDir(boardsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		boardName := entry.Name()
		boardDir := filepath.Join(boardsDir, boardName)
		boardMD := filepath.Join(boardDir, "board.md")

		if _, err := os.Stat(boardMD); err != nil {
			continue
		}

		fmt.Printf("Migrating board: %s\n", boardName)
		if err := migrateBoard(wsRoot, boardDir, boardName); err != nil {
			fmt.Fprintf(os.Stderr, "  ERROR: %v\n", err)
		}
	}

	// Migrate archived boards
	archiveBoardsDir := filepath.Join(wsRoot, "archive", "boards")
	archiveEntries, err := os.ReadDir(archiveBoardsDir)
	if err == nil {
		for _, entry := range archiveEntries {
			if !entry.IsDir() {
				continue
			}
			boardName := entry.Name()
			boardDir := filepath.Join(archiveBoardsDir, boardName)
			boardMD := filepath.Join(boardDir, "board.md")
			if _, err := os.Stat(boardMD); err != nil {
				continue
			}
			fmt.Printf("Migrating archived board: %s\n", boardName)
			if err := migrateArchivedBoard(wsRoot, boardDir, boardName); err != nil {
				fmt.Fprintf(os.Stderr, "  ERROR: %v\n", err)
			}
		}
	}

	return nil
}

type boardFM struct {
	JiraBoardID int    `yaml:"jira_board_id"`
	Project     string `yaml:"project"`
}

func migrateBoard(wsRoot, boardDir, boardName string) error {
	boardMD := filepath.Join(boardDir, "board.md")
	content, err := os.ReadFile(boardMD)
	if err != nil {
		return err
	}

	body, fm := parseBoardFrontmatter(content)
	if fm.JiraBoardID != 0 {
		fmt.Printf("  WARN: dropping jira_board_id=%d\n", fm.JiraBoardID)
	}
	if fm.Project != "" {
		fmt.Printf("  WARN: dropping project=%s\n", fm.Project)
	}

	columns, cardLinks := parseBoardMarkdown(body, boardDir)

	// Write .txt file (exclude Done)
	newBoardPath := filepath.Join(wsRoot, "boards", boardName+".txt")
	var statuses []string
	for _, col := range columns {
		if !strings.EqualFold(col, "done") {
			statuses = append(statuses, col)
		}
	}
	if err := os.WriteFile(newBoardPath, []byte(strings.Join(statuses, "\n")+"\n"), 0644); err != nil {
		return err
	}

	// Migrate cards
	tasksDir := filepath.Join(wsRoot, "tasks")
	for col, cards := range cardLinks {
		for _, cardFile := range cards {
			if err := migrateCard(boardDir, tasksDir, boardName, col, cardFile); err != nil {
				fmt.Fprintf(os.Stderr, "  ERROR migrating card %s: %v\n", cardFile, err)
			}
		}
	}

	// Migrate archived cards from archive/boards/<name>/cards/
	archiveCardsDir := filepath.Join(wsRoot, "archive", "boards", boardName, "cards")
	archiveEntries, err := os.ReadDir(archiveCardsDir)
	if err == nil {
		for _, entry := range archiveEntries {
			if !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			archiveBoardDir := filepath.Join(wsRoot, "archive", "boards", boardName)
			if err := migrateCard(archiveBoardDir, tasksDir, boardName, "Done", entry.Name()); err != nil {
				fmt.Fprintf(os.Stderr, "  ERROR migrating archived card %s: %v\n", entry.Name(), err)
			}
		}
		os.RemoveAll(filepath.Join(wsRoot, "archive", "boards", boardName))
	}

	// Delete old board directory
	os.RemoveAll(boardDir)
	return nil
}

func migrateArchivedBoard(wsRoot, boardDir, boardName string) error {
	boardMD := filepath.Join(boardDir, "board.md")
	content, err := os.ReadFile(boardMD)
	if err != nil {
		return err
	}

	body, fm := parseBoardFrontmatter(content)
	if fm.JiraBoardID != 0 {
		fmt.Printf("  WARN: dropping jira_board_id=%d\n", fm.JiraBoardID)
	}
	if fm.Project != "" {
		fmt.Printf("  WARN: dropping project=%s\n", fm.Project)
	}

	columns, cardLinks := parseBoardMarkdown(body, boardDir)

	// Write .txt file to archive/boards/
	newBoardPath := filepath.Join(wsRoot, "archive", "boards", boardName+".txt")
	var statuses []string
	for _, col := range columns {
		if !strings.EqualFold(col, "done") {
			statuses = append(statuses, col)
		}
	}
	if err := os.WriteFile(newBoardPath, []byte(strings.Join(statuses, "\n")+"\n"), 0644); err != nil {
		return err
	}

	tasksDir := filepath.Join(wsRoot, "tasks")
	for col, cards := range cardLinks {
		for _, cardFile := range cards {
			if err := migrateCard(boardDir, tasksDir, boardName, col, cardFile); err != nil {
				fmt.Fprintf(os.Stderr, "  ERROR migrating card %s: %v\n", cardFile, err)
			}
		}
	}

	os.RemoveAll(boardDir)
	return nil
}

func migrateCard(boardDir, tasksDir, boardName, columnName, cardFile string) error {
	srcPath := filepath.Join(boardDir, "cards", cardFile)

	content, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	// Add board: and status: frontmatter, rename column: to status: if present
	content = addBoardStatusFrontmatter(content, boardName, columnName)

	// Handle filename conflict
	dstPath := filepath.Join(tasksDir, cardFile)
	if _, err := os.Stat(dstPath); err == nil {
		// Conflict: add suffix
		ext := filepath.Ext(cardFile)
		base := strings.TrimSuffix(cardFile, ext)
		for i := 1; ; i++ {
			candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
			if _, err := os.Stat(filepath.Join(tasksDir, candidate)); os.IsNotExist(err) {
				dstPath = filepath.Join(tasksDir, candidate)
				break
			}
		}
	}

	if err := os.WriteFile(dstPath, content, 0644); err != nil {
		return err
	}

	fmt.Printf("  Migrated: %s → tasks/%s (status: %s)\n", cardFile, filepath.Base(dstPath), columnName)
	return nil
}

func addBoardStatusFrontmatter(content []byte, boardName, status string) []byte {
	lines := bytes.Split(content, []byte("\n"))
	if len(lines) == 0 || !bytes.Equal(bytes.TrimSpace(lines[0]), []byte("---")) {
		// No frontmatter, add one
		fm := fmt.Sprintf("---\nboard: %s\nstatus: %s\n---\n\n", boardName, status)
		return append([]byte(fm), content...)
	}

	var fmEnd int
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(bytes.TrimSpace(lines[i]), []byte("---")) {
			fmEnd = i
			break
		}
	}
	if fmEnd == 0 {
		fm := fmt.Sprintf("---\nboard: %s\nstatus: %s\n---\n\n", boardName, status)
		return append([]byte(fm), content...)
	}

	// Parse existing frontmatter
	fmBytes := bytes.Join(lines[1:fmEnd], []byte("\n"))
	var fmMap map[string]interface{}
	yaml.Unmarshal(fmBytes, &fmMap)
	if fmMap == nil {
		fmMap = make(map[string]interface{})
	}

	// Remove column field, add board and status
	delete(fmMap, "column")
	delete(fmMap, "archived")
	fmMap["board"] = boardName
	fmMap["status"] = status

	newFMBytes, _ := yaml.Marshal(fmMap)
	body := bytes.TrimLeft(bytes.Join(lines[fmEnd+1:], []byte("\n")), "\n")

	var result bytes.Buffer
	result.WriteString("---\n")
	result.Write(newFMBytes)
	result.WriteString("---\n\n")
	result.Write(body)
	return result.Bytes()
}

func parseBoardFrontmatter(content []byte) ([]byte, boardFM) {
	lines := bytes.Split(content, []byte("\n"))
	if len(lines) == 0 || !bytes.Equal(bytes.TrimSpace(lines[0]), []byte("---")) {
		return content, boardFM{}
	}
	var fmEnd int
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(bytes.TrimSpace(lines[i]), []byte("---")) {
			fmEnd = i
			break
		}
	}
	if fmEnd == 0 {
		return content, boardFM{}
	}
	fmBytes := bytes.Join(lines[1:fmEnd], []byte("\n"))
	var fm boardFM
	yaml.Unmarshal(fmBytes, &fm)
	body := bytes.TrimLeft(bytes.Join(lines[fmEnd+1:], []byte("\n")), "\n")
	return body, fm
}

// parseBoardMarkdown parses board.md to extract column names and card filename lists.
// Returns columns in order and a map of column → []filename.
func parseBoardMarkdown(body []byte, boardDir string) ([]string, map[string][]string) {
	reader := text.NewReader(body)
	md := goldmark.New()
	doc := md.Parser().Parse(reader)

	var columns []string
	cardLinks := make(map[string][]string)
	var currentCol string

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Heading:
			if node.Level == 2 {
				currentCol = string(node.Text(body))
				columns = append(columns, currentCol)
				cardLinks[currentCol] = []string{}
			}
		case *ast.Link:
			dest := string(node.Destination)
			if strings.HasPrefix(dest, "./cards/") || strings.HasPrefix(dest, "cards/") {
				filename := filepath.Base(dest)
				if currentCol != "" {
					cardLinks[currentCol] = append(cardLinks[currentCol], filename)
				}
			}
		}
		return ast.WalkContinue, nil
	})

	return columns, cardLinks
}
