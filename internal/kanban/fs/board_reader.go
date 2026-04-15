package fs

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"wydo/internal/kanban/models"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

// ReadBoard reads a board.md file and parses it into a Board struct
func ReadBoard(boardPath string) (models.Board, error) {
	boardFilePath := filepath.Join(boardPath, "board.md")

	content, err := os.ReadFile(boardFilePath)
	if err != nil {
		return models.Board{}, err
	}

	body, jiraBoardID, project := stripBoardFrontmatter(content)

	board := models.Board{
		Path:        boardPath,
		Columns:     []models.Column{},
		JiraBoardID: jiraBoardID,
		Project:     project,
	}

	reader := text.NewReader(body)
	parser := goldmark.DefaultParser()
	doc := parser.Parse(reader)

	var currentColumn *models.Column

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := n.(type) {
		case *ast.Heading:
			headingText := string(node.Text(body))

			if node.Level == 1 {
				board.Name = headingText
			} else if node.Level == 2 {
				if currentColumn != nil {
					board.Columns = append(board.Columns, *currentColumn)
				}
				currentColumn = &models.Column{
					Name:      headingText,
					TaskNotes: []models.TaskNote{},
				}
			}

		case *ast.Link:
			dest := string(node.Destination)
			if strings.HasPrefix(dest, "./cards/") || strings.HasPrefix(dest, "cards/") {
				cardPath := filepath.Join(boardPath, dest)
				tn, err := ReadTaskNote(cardPath)
				if err == nil && currentColumn != nil {
					currentColumn.TaskNotes = append(currentColumn.TaskNotes, tn)
				}
			}
		}

		return ast.WalkContinue, nil
	})

	if currentColumn != nil {
		board.Columns = append(board.Columns, *currentColumn)
	}

	// Load archived cards for active boards (boards/ not under archive/)
	boardsDir := filepath.Dir(boardPath)
	if filepath.Base(boardsDir) == "boards" && filepath.Base(filepath.Dir(boardsDir)) != "archive" {
		wsRoot := filepath.Dir(boardsDir)
		archiveCardsDir := filepath.Join(wsRoot, "archive", "boards", filepath.Base(boardPath), "cards")
		loadArchivedCards(&board, archiveCardsDir)
	}

	return board, nil
}

// loadArchivedCards scans an archive cards directory and appends each card to its
// stored column (from the "column" frontmatter field), falling back to column 0.
func loadArchivedCards(board *models.Board, archiveCardsDir string) {
	entries, err := os.ReadDir(archiveCardsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		cardPath := filepath.Join(archiveCardsDir, entry.Name())
		tn, err := ReadTaskNote(cardPath)
		if err != nil {
			continue
		}
		tn.Archived = true

		targetColIdx := board.GetColumnIndex(tn.Column)
		if targetColIdx == -1 {
			targetColIdx = 0
		}
		if len(board.Columns) > 0 {
			board.Columns[targetColIdx].TaskNotes = append(board.Columns[targetColIdx].TaskNotes, tn)
		}
	}
}

// stripBoardFrontmatter extracts optional YAML frontmatter from board.md content.
// Returns the body (without frontmatter), the jira_board_id, and the project path.
func stripBoardFrontmatter(content []byte) ([]byte, int, string) {
	lines := bytes.Split(content, []byte("\n"))
	if len(lines) == 0 || !bytes.Equal(bytes.TrimSpace(lines[0]), []byte("---")) {
		return content, 0, ""
	}

	var frontmatterEnd int
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(bytes.TrimSpace(lines[i]), []byte("---")) {
			frontmatterEnd = i
			break
		}
	}

	if frontmatterEnd == 0 {
		return content, 0, ""
	}

	frontmatterBytes := bytes.Join(lines[1:frontmatterEnd], []byte("\n"))
	var fm struct {
		JiraBoardID int    `yaml:"jira_board_id"`
		Project     string `yaml:"project"`
	}
	if err := yaml.Unmarshal(frontmatterBytes, &fm); err != nil {
		return content, 0, ""
	}

	body := bytes.TrimLeft(bytes.Join(lines[frontmatterEnd+1:], []byte("\n")), "\n")
	return body, fm.JiraBoardID, fm.Project
}
