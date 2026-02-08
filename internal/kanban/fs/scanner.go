package fs

import (
	"log"
	"os"
	"path/filepath"
	"wydo/internal/kanban/models"
)

// ScanBoards scans a single directory for board directories
func ScanBoards(rootDir string) ([]models.Board, error) {
	var boards []models.Board

	entries, err := os.ReadDir(rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return boards, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		boardPath := filepath.Join(rootDir, entry.Name())
		boardFilePath := filepath.Join(boardPath, "board.md")

		if _, err := os.Stat(boardFilePath); err == nil {
			board, err := ReadBoard(boardPath)
			if err != nil {
				continue
			}
			boards = append(boards, board)
		}
	}

	return boards, nil
}

// ScanMultipleDirectories scans multiple directories for boards and deduplicates by path
func ScanMultipleDirectories(dirs []string) ([]models.Board, error) {
	seen := make(map[string]bool)
	var boards []models.Board

	for _, dir := range dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			log.Printf("Warning: could not resolve path %s: %v", dir, err)
			continue
		}

		dirBoards, err := ScanBoards(absDir)
		if err != nil {
			log.Printf("Warning: could not scan directory %s: %v", absDir, err)
			continue
		}

		for _, board := range dirBoards {
			absPath, err := filepath.Abs(board.Path)
			if err != nil {
				absPath = board.Path
			}
			if !seen[absPath] {
				seen[absPath] = true
				boards = append(boards, board)
			}
		}
	}

	return boards, nil
}

// FindBoardsDirectories recursively finds all directories named "boards" under the given roots
func FindBoardsDirectories(recursiveDirs []string) []string {
	var boardsDirs []string
	seen := make(map[string]bool)

	for _, root := range recursiveDirs {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			log.Printf("Warning: could not resolve path %s: %v", root, err)
			continue
		}

		err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				if os.IsPermission(err) {
					log.Printf("Warning: permission denied: %s", path)
					return filepath.SkipDir
				}
				return err
			}

			if d.IsDir() && d.Name() == "boards" {
				absPath, _ := filepath.Abs(path)
				if !seen[absPath] {
					seen[absPath] = true
					boardsDirs = append(boardsDirs, absPath)
				}
				return filepath.SkipDir
			}

			return nil
		})

		if err != nil {
			log.Printf("Warning: error walking directory %s: %v", absRoot, err)
		}
	}

	return boardsDirs
}

// FindContentDirectories recursively finds directories containing board.md or todo.txt
func FindContentDirectories(recursiveDirs []string, todoFilename string) []string {
	var contentDirs []string
	seen := make(map[string]bool)

	for _, root := range recursiveDirs {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			log.Printf("Warning: could not resolve path %s: %v", root, err)
			continue
		}

		err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				if os.IsPermission(err) {
					return filepath.SkipDir
				}
				return err
			}

			if !d.IsDir() {
				return nil
			}

			absPath, _ := filepath.Abs(path)
			if seen[absPath] {
				return nil
			}

			// Check for board.md or todo.txt in this directory
			hasBoardMD := fileExistsAt(filepath.Join(path, "board.md"))
			hasTodoTxt := fileExistsAt(filepath.Join(path, todoFilename))

			if hasBoardMD || hasTodoTxt {
				seen[absPath] = true
				contentDirs = append(contentDirs, absPath)
			}

			return nil
		})

		if err != nil {
			log.Printf("Warning: error walking directory %s: %v", absRoot, err)
		}
	}

	return contentDirs
}

// ScanAllBoards scans both regular directories and recursively found boards directories
func ScanAllBoards(dirs, recursiveDirs []string) ([]models.Board, error) {
	allDirs := make([]string, 0, len(dirs))
	allDirs = append(allDirs, dirs...)

	if len(recursiveDirs) > 0 {
		foundDirs := FindBoardsDirectories(recursiveDirs)
		allDirs = append(allDirs, foundDirs...)
	}

	return ScanMultipleDirectories(allDirs)
}

// TodoFileInfo represents a discovered todo.txt file location
type TodoFileInfo struct {
	Dir      string // Directory containing the file
	TodoPath string // Full path to todo.txt
	DonePath string // Full path to done.txt
}

// ScanTodoFiles finds all todo.txt files in configured directories
func ScanTodoFiles(dirs, recursiveDirs []string, todoFilename, doneFilename string) []TodoFileInfo {
	var results []TodoFileInfo
	seen := make(map[string]bool)

	checkDir := func(dir string) {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return
		}
		if seen[absDir] {
			return
		}

		todoPath := filepath.Join(absDir, todoFilename)
		if fileExistsAt(todoPath) {
			seen[absDir] = true
			results = append(results, TodoFileInfo{
				Dir:      absDir,
				TodoPath: todoPath,
				DonePath: filepath.Join(absDir, doneFilename),
			})
		}
	}

	for _, dir := range dirs {
		checkDir(dir)
	}

	if len(recursiveDirs) > 0 {
		contentDirs := FindContentDirectories(recursiveDirs, todoFilename)
		for _, dir := range contentDirs {
			checkDir(dir)
		}
	}

	return results
}

func fileExistsAt(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
