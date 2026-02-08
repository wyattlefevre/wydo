package fs

import (
	"os"
	"path/filepath"
	"strings"
	"wydo/internal/kanban/models"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// ReadBoard reads a board.md file and parses it into a Board struct
func ReadBoard(boardPath string) (models.Board, error) {
	boardFilePath := filepath.Join(boardPath, "board.md")

	content, err := os.ReadFile(boardFilePath)
	if err != nil {
		return models.Board{}, err
	}

	board := models.Board{
		Path:    boardPath,
		Columns: []models.Column{},
	}

	reader := text.NewReader(content)
	parser := goldmark.DefaultParser()
	doc := parser.Parse(reader)

	var currentColumn *models.Column

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := n.(type) {
		case *ast.Heading:
			headingText := string(node.Text(content))

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
