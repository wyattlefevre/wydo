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
	boardPath := filepath.Join(rootDir, "boards", dirName+".txt")

	if _, err := os.Stat(boardPath); err == nil {
		return models.Board{}, fmt.Errorf("board already exists")
	}

	if err := os.MkdirAll(filepath.Join(rootDir, "boards"), 0755); err != nil {
		return models.Board{}, err
	}

	// Ensure tasks/ dir exists
	if err := os.MkdirAll(filepath.Join(rootDir, "tasks"), 0755); err != nil {
		return models.Board{}, err
	}

	board := models.Board{
		Name:     dirName,
		Path:     boardPath,
		WSRoot:   rootDir,
		Statuses: []string{"To Do", "In Progress", "Done"},
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

// DeleteBoard removes a board .txt file
func DeleteBoard(board models.Board) error {
	return os.Remove(board.Path)
}

// ToggleBoardArchive moves a board .txt file between boards/ and archive/boards/
func ToggleBoardArchive(board *models.Board) error {
	filename := filepath.Base(board.Path) // e.g. "dev-work.txt"

	if board.Archived {
		// board.Path = <ws>/archive/boards/<name>.txt
		// Go up 3 levels: .txt → boards → archive → ws
		wsRoot := filepath.Dir(filepath.Dir(filepath.Dir(board.Path)))
		activePath := filepath.Join(wsRoot, "boards", filename)

		if err := os.Rename(board.Path, activePath); err != nil {
			return err
		}
		board.Path = activePath
		board.WSRoot = wsRoot
		board.Archived = false
	} else {
		// board.Path = <ws>/boards/<name>.txt
		boardsDir := filepath.Dir(board.Path)
		wsRoot := filepath.Dir(boardsDir)
		archivePath := filepath.Join(wsRoot, "archive", "boards", filename)

		if err := os.MkdirAll(filepath.Join(wsRoot, "archive", "boards"), 0755); err != nil {
			return err
		}

		if err := os.Rename(board.Path, archivePath); err != nil {
			return err
		}

		board.Path = archivePath
		board.Archived = true
	}
	return nil
}

// RenameBoard renames a board's .txt file on disk and updates all task notes
// that reference this board by name.
func RenameBoard(board *models.Board, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("board name cannot be empty")
	}

	newDirName := sanitizeName(newName)
	if newDirName == board.Name {
		return nil
	}

	// Rename .txt file
	oldPath := board.Path
	newPath := filepath.Join(filepath.Dir(oldPath), newDirName+".txt")

	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("a board already exists at %s", newDirName)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}

	oldName := board.Name
	board.Path = newPath
	board.Name = newDirName

	// Update task notes with board: <oldName> → board: <newName>
	if board.WSRoot != "" {
		for _, dir := range []string{
			filepath.Join(board.WSRoot, "tasks"),
			filepath.Join(board.WSRoot, "archive", "tasks"),
		} {
			updateBoardNameInTaskNotes(dir, oldName, newDirName)
		}
	}

	return nil
}

// updateBoardNameInTaskNotes scans a directory and rewrites the board: field
// for any task note that references oldName.
func updateBoardNameInTaskNotes(dir, oldName, newName string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		tnPath := filepath.Join(dir, entry.Name())
		tn, err := fs.ReadTaskNote(tnPath)
		if err != nil {
			continue
		}
		if strings.EqualFold(tn.Board, oldName) {
			tn.Board = newName
			_ = fs.WriteTaskNote(tn, tnPath)
		}
	}
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
