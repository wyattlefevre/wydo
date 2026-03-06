package fs

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"wydo/internal/kanban/models"

	"gopkg.in/yaml.v3"
)

// WriteBoard writes a Board struct to board.md
func WriteBoard(board models.Board) error {
	boardFilePath := filepath.Join(board.Path, "board.md")

	var buf bytes.Buffer

	if board.Archived || board.JiraBoardID != 0 || board.Project != "" {
		buf.WriteString("---\n")
		if board.Archived {
			buf.WriteString("archived: true\n")
		}
		if board.JiraBoardID != 0 {
			buf.WriteString(fmt.Sprintf("jira_board_id: %d\n", board.JiraBoardID))
		}
		if board.Project != "" {
			if projectYAML, err := yaml.Marshal(board.Project); err == nil {
				buf.WriteString("project: " + strings.TrimRight(string(projectYAML), "\n") + "\n")
			}
		}
		buf.WriteString("---\n\n")
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
