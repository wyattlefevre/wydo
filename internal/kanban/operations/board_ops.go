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
			{Name: "To Do", TaskNotes: []models.TaskNote{}},
			{Name: "In Progress", TaskNotes: []models.TaskNote{}},
			{Name: "Done", TaskNotes: []models.TaskNote{}},
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

// ToggleBoardArchive moves a board between boards/ and archive/boards/ to toggle its archived state.
func ToggleBoardArchive(board *models.Board) error {
	boardName := filepath.Base(board.Path)

	if board.Archived {
		// board.Path = <ws>/archive/boards/<name>
		// Go up 3 levels: archive/boards → archive → ws
		wsRoot := filepath.Dir(filepath.Dir(filepath.Dir(board.Path)))
		activePath := filepath.Join(wsRoot, "boards", boardName)

		if err := os.Rename(board.Path, activePath); err != nil {
			return err
		}
		board.Path = activePath
		board.Archived = false
	} else {
		// board.Path = <ws>/boards/<name>
		boardsDir := filepath.Dir(board.Path)
		wsRoot := filepath.Dir(boardsDir)
		archivePath := filepath.Join(wsRoot, "archive", "boards", boardName)

		if err := os.MkdirAll(filepath.Join(wsRoot, "archive", "boards"), 0755); err != nil {
			return err
		}

		if _, err := os.Stat(archivePath); err == nil {
			// archive/boards/<name> already exists (has individually archived cards) — merge
			if err := mergeBoardIntoArchive(board.Path, archivePath); err != nil {
				return err
			}
		} else {
			if err := os.Rename(board.Path, archivePath); err != nil {
				return err
			}
		}

		board.Path = archivePath
		board.Archived = true
	}
	return nil
}

// mergeBoardIntoArchive moves an active board's contents into an existing archive directory.
// Used when archive/boards/<name>/ already exists (e.g. from individually archived cards).
func mergeBoardIntoArchive(boardPath, archivePath string) error {
	if err := os.Rename(filepath.Join(boardPath, "board.md"), filepath.Join(archivePath, "board.md")); err != nil {
		return err
	}

	activeCardsDir := filepath.Join(boardPath, "cards")
	archiveCardsDir := filepath.Join(archivePath, "cards")
	if err := os.MkdirAll(archiveCardsDir, 0755); err != nil {
		return err
	}

	if entries, err := os.ReadDir(activeCardsDir); err == nil {
		for _, entry := range entries {
			src := filepath.Join(activeCardsDir, entry.Name())
			dst := filepath.Join(archiveCardsDir, entry.Name())
			if _, err := os.Stat(dst); err != nil {
				_ = os.Rename(src, dst)
			}
		}
	}

	_ = os.Remove(activeCardsDir)
	return os.Remove(boardPath)
}

// RenameBoard renames a board's display name and directory on disk
func RenameBoard(board *models.Board, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("board name cannot be empty")
	}
	if newName == board.Name {
		return nil
	}

	oldPath := board.Path
	newDirName := sanitizeName(newName)
	newPath := filepath.Join(filepath.Dir(oldPath), newDirName)

	if oldPath != newPath {
		if _, err := os.Stat(newPath); err == nil {
			return fmt.Errorf("a board already exists at %s", newDirName)
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return err
		}
		board.Path = newPath
	}

	board.Name = newName
	return fs.WriteBoard(*board)
}

// SetBoardProject sets the board's linked project to the given relative path
// (relative from the board directory to the project's index .md file) and
// persists the change to disk. Pass an empty string to clear the link.
func SetBoardProject(board *models.Board, relPath string) error {
	board.Project = relPath
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
