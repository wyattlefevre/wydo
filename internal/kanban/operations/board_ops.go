package operations

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"wydo/internal/kanban/fs"
	"wydo/internal/kanban/models"
)

// CreateBoard creates a new board with the given name
func CreateBoard(rootDir, name string) (models.Board, error) {
	dirName := sanitizeName(name)
	boardPath := filepath.Join(rootDir, dirName)

	if _, err := os.Stat(boardPath); err == nil {
		return models.Board{}, fmt.Errorf("board already exists")
	}

	if err := os.MkdirAll(boardPath, 0755); err != nil {
		return models.Board{}, err
	}

	cardsDir := filepath.Join(boardPath, "cards")
	if err := os.MkdirAll(cardsDir, 0755); err != nil {
		return models.Board{}, err
	}

	board := models.Board{
		Name: name,
		Path: boardPath,
		Columns: []models.Column{
			{Name: "To Do", Cards: []models.Card{}},
			{Name: "In Progress", Cards: []models.Card{}},
			{Name: "Done", Cards: []models.Card{}},
		},
	}

	if err := fs.WriteBoard(board); err != nil {
		return models.Board{}, err
	}

	return board, nil
}

// DeleteBoard removes a board directory
func DeleteBoard(board models.Board) error {
	return os.RemoveAll(board.Path)
}

// ToggleBoardArchive flips the archived state of a board and persists to disk
func ToggleBoardArchive(board *models.Board) error {
	board.Archived = !board.Archived
	return fs.WriteBoard(*board)
}

func sanitizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")

	reg := regexp.MustCompile("[^a-z0-9-]+")
	name = reg.ReplaceAllString(name, "")

	reg = regexp.MustCompile("-+")
	name = reg.ReplaceAllString(name, "-")

	name = strings.Trim(name, "-")

	return name
}
