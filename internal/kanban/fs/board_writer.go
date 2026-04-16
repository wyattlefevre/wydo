package fs

import (
	"os"
	"strings"
	"wydo/internal/kanban/models"
)

// WriteBoard writes a Board's Statuses to its .txt file as newline-separated lines.
// "Done" is never written — it's always implicit.
func WriteBoard(board models.Board) error {
	var lines []string
	for _, s := range board.Statuses {
		if !strings.EqualFold(s, "done") {
			lines = append(lines, s)
		}
	}
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	return os.WriteFile(board.Path, []byte(content), 0644)
}
