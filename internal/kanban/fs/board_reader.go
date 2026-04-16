package fs

import (
	"os"
	"path/filepath"
	"strings"
	"wydo/internal/kanban/models"
)

// ReadBoard reads a board .txt file and returns a Board with Statuses populated.
// boardPath is the path to the .txt file (e.g. boards/dev-work.txt).
// The board's runtime Columns field is NOT populated here — use ReadBoardFull
// or workspace.Load() to get a fully populated board.
func ReadBoard(boardPath string) (models.Board, error) {
	content, err := os.ReadFile(boardPath)
	if err != nil {
		return models.Board{}, err
	}

	name := strings.TrimSuffix(filepath.Base(boardPath), ".txt")

	var statuses []string
	for _, line := range strings.Split(strings.TrimRight(string(content), "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			statuses = append(statuses, line)
		}
	}
	if statuses == nil {
		statuses = []string{}
	}

	board := models.Board{
		Name:     name,
		Path:     boardPath,
		Statuses: statuses,
		Columns:  []models.Column{},
	}

	return board, nil
}
