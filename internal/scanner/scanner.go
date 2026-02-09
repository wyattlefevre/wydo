package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

// WorkspaceScan holds everything discovered from scanning a single workspace
type WorkspaceScan struct {
	RootDir   string
	Boards    []BoardInfo
	TaskDirs  []TaskDirInfo
	Projects  []ProjectInfo
	NotePaths []string // absolute paths to .md files (not board.md, not in cards/)
}

// BoardInfo describes a discovered board directory
type BoardInfo struct {
	Path    string // absolute path to board dir (containing board.md)
	Project string // project name if under projects/ subtree, "" otherwise
}

// TaskDirInfo describes a discovered tasks/ directory
type TaskDirInfo struct {
	DirPath string   // absolute path to the tasks/ directory
	Files   []string // .txt filenames found within
	Project string   // project name if under projects/ subtree
}

// ProjectInfo describes a discovered project directory
type ProjectInfo struct {
	Name   string
	Path   string // absolute path to project dir
	Parent string // parent project name if nested
}

// ScanWorkspace recursively scans a single workspace directory
func ScanWorkspace(rootDir string) (*WorkspaceScan, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}

	scan := &WorkspaceScan{
		RootDir: absRoot,
	}

	err = walkWorkspace(absRoot, absRoot, "", scan)
	if err != nil {
		return nil, err
	}

	return scan, nil
}

// walkWorkspace recursively walks a directory, tracking project context
func walkWorkspace(dir, rootDir, projectContext string, scan *WorkspaceScan) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		absPath := filepath.Join(dir, name)

		// Skip hidden dirs and common junk
		if entry.IsDir() && shouldSkipDir(name) {
			continue
		}

		if entry.IsDir() {
			switch name {
			case "boards":
				if err := scanBoardsDir(absPath, projectContext, scan); err != nil {
					return err
				}
			case "tasks":
				if err := scanTasksDir(absPath, projectContext, scan); err != nil {
					return err
				}
			case "projects":
				if err := scanProjectsDir(absPath, rootDir, projectContext, scan); err != nil {
					return err
				}
			case "cards":
				// Skip cards/ directories - they belong to boards
				continue
			default:
				// Recurse into other directories (e.g. notes/)
				if err := walkWorkspace(absPath, rootDir, projectContext, scan); err != nil {
					return err
				}
			}
		} else if isNoteFile(name, dir) {
			scan.NotePaths = append(scan.NotePaths, absPath)
		}
	}

	return nil
}

// scanBoardsDir scans a boards/ directory for board subdirectories
func scanBoardsDir(boardsDir, projectContext string, scan *WorkspaceScan) error {
	entries, err := os.ReadDir(boardsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		boardPath := filepath.Join(boardsDir, entry.Name())
		boardFile := filepath.Join(boardPath, "board.md")

		if _, err := os.Stat(boardFile); err == nil {
			scan.Boards = append(scan.Boards, BoardInfo{
				Path:    boardPath,
				Project: projectContext,
			})
		}
	}

	return nil
}

// scanTasksDir scans a tasks/ directory for .txt files
func scanTasksDir(tasksDir, projectContext string, scan *WorkspaceScan) error {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".txt") {
			files = append(files, entry.Name())
		}
	}

	if len(files) > 0 {
		scan.TaskDirs = append(scan.TaskDirs, TaskDirInfo{
			DirPath: tasksDir,
			Files:   files,
			Project: projectContext,
		})
	}

	return nil
}

// scanProjectsDir scans a projects/ directory, where each subdirectory is a project
func scanProjectsDir(projectsDir, rootDir, parentProject string, scan *WorkspaceScan) error {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if shouldSkipDir(entry.Name()) {
			continue
		}

		projectName := entry.Name()
		projectPath := filepath.Join(projectsDir, projectName)

		scan.Projects = append(scan.Projects, ProjectInfo{
			Name:   projectName,
			Path:   projectPath,
			Parent: parentProject,
		})

		// Recurse into the project directory with this project as context
		if err := walkWorkspace(projectPath, rootDir, projectName, scan); err != nil {
			return err
		}
	}

	return nil
}

// isNoteFile returns true if the file is a markdown note (not board.md, not in cards/)
func isNoteFile(name, dir string) bool {
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		return false
	}
	if name == "board.md" {
		return false
	}
	// Check if we're inside a cards/ directory
	if filepath.Base(dir) == "cards" {
		return false
	}
	return true
}

// shouldSkipDir returns true for directories that should be skipped during scanning
func shouldSkipDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "node_modules", "vendor", "__pycache__", "target", "build", "dist":
		return true
	}
	return false
}
