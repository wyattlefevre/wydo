package fs

import (
	"bytes"
	"os"
	"path/filepath"
	"wydo/internal/kanban/models"
)

// WriteBoard writes a Board struct to board.md
func WriteBoard(board models.Board) error {
	boardFilePath := filepath.Join(board.Path, "board.md")

	var buf bytes.Buffer

	if board.Archived {
		buf.WriteString("---\narchived: true\n---\n\n")
	}

	buf.WriteString("# ")
	buf.WriteString(board.Name)
	buf.WriteString("\n\n")

	for _, column := range board.Columns {
		buf.WriteString("## ")
		buf.WriteString(column.Name)
		buf.WriteString("\n\n")

		for _, card := range column.Cards {
			buf.WriteString("[")
			buf.WriteString(card.Title)
			buf.WriteString("](./cards/")
			buf.WriteString(card.Filename)
			buf.WriteString(")\n\n")
		}
	}

	return os.WriteFile(boardFilePath, buf.Bytes(), 0644)
}
