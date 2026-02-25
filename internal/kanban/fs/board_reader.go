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

	body, archived := stripBoardFrontmatter(content)

	board := models.Board{
		Path:     boardPath,
		Columns:  []models.Column{},
		Archived: archived,
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
					Name:  headingText,
					Cards: []models.Card{},
				}
			}

		case *ast.Link:
			dest := string(node.Destination)
			if strings.HasPrefix(dest, "./cards/") || strings.HasPrefix(dest, "cards/") {
				cardPath := filepath.Join(boardPath, dest)
				card, err := ReadCard(cardPath)
				if err == nil && currentColumn != nil {
					currentColumn.Cards = append(currentColumn.Cards, card)
				}
			}
		}

		return ast.WalkContinue, nil
	})

	if currentColumn != nil {
		board.Columns = append(board.Columns, *currentColumn)
	}

	return board, nil
}

// stripBoardFrontmatter extracts optional YAML frontmatter from board.md content.
// Returns the body (without frontmatter) and whether archived is true.
func stripBoardFrontmatter(content []byte) ([]byte, bool) {
	lines := bytes.Split(content, []byte("\n"))
	if len(lines) == 0 || !bytes.Equal(bytes.TrimSpace(lines[0]), []byte("---")) {
		return content, false
	}

	var frontmatterEnd int
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(bytes.TrimSpace(lines[i]), []byte("---")) {
			frontmatterEnd = i
			break
		}
	}

	if frontmatterEnd == 0 {
		return content, false
	}

	frontmatterBytes := bytes.Join(lines[1:frontmatterEnd], []byte("\n"))
	var fm struct {
		Archived bool `yaml:"archived"`
	}
	if err := yaml.Unmarshal(frontmatterBytes, &fm); err != nil {
		return content, false
	}

	body := bytes.TrimLeft(bytes.Join(lines[frontmatterEnd+1:], []byte("\n")), "\n")
	return body, fm.Archived
}
