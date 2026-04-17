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
	Path     string // absolute path to board dir (containing board.md)
	Archived bool   // true if this board lives under archive/boards/
}

// TaskDirInfo describes a discovered tasks/ directory
type TaskDirInfo struct {
	DirPath  string // absolute path to the tasks/ directory (for .md task notes)
	TodoPath string // explicit path to todo.txt file (at workspace root or archive root)
}

// ProjectInfo describes a discovered project file
type ProjectInfo struct {
	Name     string
	Path     string // absolute path to the .md file
	Parent   string // parent project name if nested
	Archived bool   // true if discovered under archive/projects/
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
				if dir == rootDir {
					if err := scanBoardsDir(absPath, scan); err != nil {
						return err
					}
				}
			case "tasks":
				if dir == rootDir {
					scan.TaskDirs = append(scan.TaskDirs, TaskDirInfo{
						DirPath:  absPath,
						TodoPath: filepath.Join(rootDir, "todo.txt"),
					})
				}
			case "archive":
				if dir == rootDir {
					if err := scanArchiveDir(absPath, scan); err != nil {
						return err
					}
				}
			case "projects":
				if err := scanProjectsDir(absPath, projectContext, scan); err != nil {
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

// scanBoardsDir scans a boards/ directory for board .txt files
func scanBoardsDir(boardsDir string, scan *WorkspaceScan) error {
	entries, err := os.ReadDir(boardsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}

		boardPath := filepath.Join(boardsDir, entry.Name())
		scan.Boards = append(scan.Boards, BoardInfo{
			Path: boardPath,
		})
	}

	return nil
}

// scanArchiveDir scans archive/ for mirrored subdirectories (e.g. archive/tasks/, archive/boards/).
func scanArchiveDir(archiveDir string, scan *WorkspaceScan) error {
	tasksDir := filepath.Join(archiveDir, "tasks")
	if _, err := os.Stat(tasksDir); err == nil {
		scan.TaskDirs = append(scan.TaskDirs, TaskDirInfo{
			DirPath:  tasksDir,
			TodoPath: filepath.Join(archiveDir, "todo.txt"),
		})
	}

	boardsDir := filepath.Join(archiveDir, "boards")
	if _, err := os.Stat(boardsDir); err == nil {
		if err := scanArchivedBoardsDir(boardsDir, scan); err != nil {
			return err
		}
	}

	projectsDir := filepath.Join(archiveDir, "projects")
	if _, err := os.Stat(projectsDir); err == nil {
		if err := scanArchivedProjectsDir(projectsDir, scan); err != nil {
			return err
		}
	}

	return nil
}

// scanArchivedBoardsDir scans archive/boards/ for archived board .txt files
func scanArchivedBoardsDir(boardsDir string, scan *WorkspaceScan) error {
	entries, err := os.ReadDir(boardsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}

		boardPath := filepath.Join(boardsDir, entry.Name())
		scan.Boards = append(scan.Boards, BoardInfo{
			Path:     boardPath,
			Archived: true,
		})
	}

	return nil
}

// scanProjectsDir scans a projects/ directory for flat .md project files.
// Each .md file (e.g. alpha.md) represents a project named by its stem.
func scanProjectsDir(projectsDir, parentProject string, scan *WorkspaceScan) error {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // flat structure — subdirectories are not projects
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		scan.Projects = append(scan.Projects, ProjectInfo{
			Name:   name,
			Path:   filepath.Join(projectsDir, entry.Name()),
			Parent: parentProject,
		})
	}

	return nil
}

// scanArchivedProjectsDir scans archive/projects/ for flat .md project files.
func scanArchivedProjectsDir(projectsDir string, scan *WorkspaceScan) error {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		scan.Projects = append(scan.Projects, ProjectInfo{
			Name:     name,
			Path:     filepath.Join(projectsDir, entry.Name()),
			Archived: true,
		})
	}

	return nil
}

// isNoteFile returns true if the file is a markdown note (not in cards/)
func isNoteFile(name, dir string) bool {
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		return false
	}
	// Skip files inside cards/ directories (legacy format, during migration)
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
