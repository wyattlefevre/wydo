package workspace

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wydo/internal/kanban/fs"
	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/notes"
	"wydo/internal/scanner"
	"wydo/internal/tasks/data"
	"wydo/internal/tasks/service"

	"gopkg.in/yaml.v3"
)

// Workspace holds all loaded data for one workspace
type Workspace struct {
	RootDir  string
	Boards   []kanbanmodels.Board
	Tasks    []data.Task
	Notes    []notes.Note
	Projects *ProjectRegistry
	TaskDirs []scanner.TaskDirInfo
	TaskSvc  service.TaskService
}

// Load creates a Workspace from a scan result
func Load(scan *scanner.WorkspaceScan) (*Workspace, error) {
	ws := &Workspace{
		RootDir:  scan.RootDir,
		TaskDirs: scan.TaskDirs,
	}

	// Load boards
	for _, bi := range scan.Boards {
		board, err := fs.ReadBoard(bi.Path)
		if err != nil {
			continue
		}
		ws.Boards = append(ws.Boards, board)
	}

	// Load tasks via task service
	taskSvc, err := service.NewTaskService(scan.TaskDirs)
	if err != nil {
		// Non-fatal: workspace may have no tasks
		taskSvc = nil
	}
	ws.TaskSvc = taskSvc

	if taskSvc != nil {
		if tasks, err := taskSvc.List(); err == nil {
			ws.Tasks = tasks
		}
	}

	// Parse notes
	for _, notePath := range scan.NotePaths {
		if note, ok := notes.ParseNoteFile(notePath, scan.RootDir); ok {
			ws.Notes = append(ws.Notes, note)
		}
	}

	// Build project registry
	ws.Projects = BuildProjectRegistry(scan, ws.Tasks, ws.Boards)

	return ws, nil
}

// RenameProject renames a project, updating the directory on disk (if physical),
// all task +tag references, and all card frontmatter project references.
func (ws *Workspace) RenameProject(oldName, newName string) error {
	// Validate old project exists
	project := ws.Projects.Get(oldName)
	if project == nil {
		return fmt.Errorf("project %q not found", oldName)
	}

	// Check if the target project already exists (merge case)
	targetProject := ws.Projects.Get(newName)

	// Handle directory logic
	if project.DirPath != "" {
		if targetProject == nil || targetProject.DirPath == "" {
			// Source has dir, target has no dir: simple rename
			parentDir := filepath.Dir(project.DirPath)
			newPath := filepath.Join(parentDir, newName)
			if err := os.Rename(project.DirPath, newPath); err != nil {
				return fmt.Errorf("rename directory: %w", err)
			}
		} else {
			// Both have dirs: merge source into target, then remove source
			if err := mergeDirs(project.DirPath, targetProject.DirPath); err != nil {
				return fmt.Errorf("merge directories: %w", err)
			}
		}
	}

	// Update task +tag references
	modified := false
	for i := range ws.Tasks {
		if ws.Tasks[i].HasProject(oldName) {
			ws.Tasks[i].RemoveProject(oldName)
			ws.Tasks[i].AddProject(newName)
			modified = true
		}
	}
	if modified {
		if err := data.WriteAllTasks(ws.Tasks); err != nil {
			return fmt.Errorf("write tasks: %w", err)
		}
		if ws.TaskSvc != nil {
			if err := ws.TaskSvc.Reload(); err != nil {
				return fmt.Errorf("reload tasks: %w", err)
			}
		}
	}

	// Update card frontmatter project references (with dedup for merge case)
	for bi := range ws.Boards {
		for ci := range ws.Boards[bi].Columns {
			for cdi := range ws.Boards[bi].Columns[ci].Cards {
				card := &ws.Boards[bi].Columns[ci].Cards[cdi]
				hasOld := false
				hasNew := false
				for _, p := range card.Projects {
					if strings.EqualFold(p, oldName) {
						hasOld = true
					}
					if strings.EqualFold(p, newName) {
						hasNew = true
					}
				}
				if !hasOld {
					continue
				}
				if hasNew {
					// Card already references target — just remove the old name
					filtered := card.Projects[:0]
					for _, p := range card.Projects {
						if !strings.EqualFold(p, oldName) {
							filtered = append(filtered, p)
						}
					}
					card.Projects = filtered
				} else {
					// Replace old with new
					for pi, p := range card.Projects {
						if strings.EqualFold(p, oldName) {
							card.Projects[pi] = newName
							break
						}
					}
				}
				cardPath := filepath.Join(ws.Boards[bi].Path, "cards", card.Filename)
				if err := fs.WriteCard(*card, cardPath); err != nil {
					return fmt.Errorf("write card %s: %w", card.Filename, err)
				}
			}
		}
	}

	return nil
}

// mergeDirs recursively merges the contents of src into dst.
// For directory entries: if dst/<name> exists as a dir, recurse; otherwise rename the subtree.
// For file entries: if both are .txt files, append src contents to dst; otherwise rename (skip if dst exists).
// After all entries are moved, removes the (now-empty) src directory.
func mergeDirs(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if info, err := os.Stat(dstPath); err == nil && info.IsDir() {
				// Both are directories — recurse
				if err := mergeDirs(srcPath, dstPath); err != nil {
					return err
				}
			} else {
				// Target doesn't exist or isn't a dir — move whole subtree
				if err := os.Rename(srcPath, dstPath); err != nil {
					return err
				}
			}
		} else {
			// File entry
			if _, err := os.Stat(dstPath); err == nil {
				// Destination file exists
				if strings.HasSuffix(entry.Name(), ".txt") {
					// Both are .txt — append src contents to dst
					srcData, err := os.ReadFile(srcPath)
					if err != nil {
						return err
					}
					f, err := os.OpenFile(dstPath, os.O_APPEND|os.O_WRONLY, 0644)
					if err != nil {
						return err
					}
					if _, err := f.Write(srcData); err != nil {
						f.Close()
						return err
					}
					f.Close()
					if err := os.Remove(srcPath); err != nil {
						return err
					}
				}
				// Non-txt file already exists at dst — skip to avoid data loss
			} else {
				// Destination doesn't exist — move the file
				if err := os.Rename(srcPath, dstPath); err != nil {
					return err
				}
			}
		}
	}
	// Remove the now-empty source directory
	return os.Remove(src)
}

// Project represents a discovered project
type Project struct {
	Name     string
	DirPath  string // from projects/ directory, "" if virtual
	Parent   string
	Children []string
	Archived bool
}

// ProjectRegistry manages project discovery and cross-entity queries within a workspace
type ProjectRegistry struct {
	projects map[string]*Project
}

// BuildProjectRegistry builds a registry from scan results, tasks, and boards
func BuildProjectRegistry(scan *scanner.WorkspaceScan, tasks []data.Task, boards []kanbanmodels.Board) *ProjectRegistry {
	r := &ProjectRegistry{
		projects: make(map[string]*Project),
	}

	// 1. From directory structure
	for _, pi := range scan.Projects {
		r.ensureProject(pi.Name, pi.Path, pi.Parent)
	}

	// 2. From task +tags (virtual projects)
	for _, t := range tasks {
		for _, p := range t.Projects {
			r.ensureProject(p, "", "")
		}
	}

	// 3. From card frontmatter projects field
	for _, board := range boards {
		for _, col := range board.Columns {
			for _, card := range col.Cards {
				for _, p := range card.Projects {
					r.ensureProject(p, "", "")
				}
			}
		}
	}

	// Build parent-child relationships
	for name, proj := range r.projects {
		if proj.Parent != "" {
			if parent, ok := r.projects[proj.Parent]; ok {
				parent.Children = append(parent.Children, name)
			}
		}
	}

	return r
}

func (r *ProjectRegistry) ensureProject(name, dirPath, parent string) {
	if existing, ok := r.projects[name]; ok {
		// Upgrade virtual project with directory info
		if dirPath != "" && existing.DirPath == "" {
			existing.DirPath = dirPath
			existing.Archived = readProjectArchived(dirPath, name)
		}
		if parent != "" && existing.Parent == "" {
			existing.Parent = parent
		}
		return
	}
	archived := false
	if dirPath != "" {
		archived = readProjectArchived(dirPath, name)
	}
	r.projects[name] = &Project{
		Name:     name,
		DirPath:  dirPath,
		Parent:   parent,
		Archived: archived,
	}
}

// readProjectArchived reads the project index file and checks for archived: true in frontmatter
func readProjectArchived(dirPath, name string) bool {
	indexPath := filepath.Join(dirPath, name+".md")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		return false
	}

	lines := bytes.Split(content, []byte("\n"))
	if len(lines) == 0 || !bytes.Equal(bytes.TrimSpace(lines[0]), []byte("---")) {
		return false
	}

	var frontmatterEnd int
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(bytes.TrimSpace(lines[i]), []byte("---")) {
			frontmatterEnd = i
			break
		}
	}
	if frontmatterEnd == 0 {
		return false
	}

	frontmatterBytes := bytes.Join(lines[1:frontmatterEnd], []byte("\n"))
	var fm struct {
		Archived bool `yaml:"archived"`
	}
	if err := yaml.Unmarshal(frontmatterBytes, &fm); err != nil {
		return false
	}
	return fm.Archived
}

// SetProjectArchived sets the archived state for a project by updating its index file frontmatter.
// Returns an error for virtual projects (no DirPath).
func SetProjectArchived(project *Project, archived bool) error {
	if project.DirPath == "" {
		return fmt.Errorf("cannot archive virtual project %q", project.Name)
	}

	indexPath := filepath.Join(project.DirPath, project.Name+".md")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		// File doesn't exist — create it
		content = []byte(fmt.Sprintf("# %s\n", project.Name))
	}

	// Strip existing frontmatter
	body := content
	lines := bytes.Split(content, []byte("\n"))
	if len(lines) > 0 && bytes.Equal(bytes.TrimSpace(lines[0]), []byte("---")) {
		for i := 1; i < len(lines); i++ {
			if bytes.Equal(bytes.TrimSpace(lines[i]), []byte("---")) {
				body = bytes.TrimLeft(bytes.Join(lines[i+1:], []byte("\n")), "\n")
				break
			}
		}
	}

	var buf bytes.Buffer
	if archived {
		buf.WriteString("---\narchived: true\n---\n\n")
	}
	buf.Write(body)

	project.Archived = archived
	return os.WriteFile(indexPath, buf.Bytes(), 0644)
}

// List returns all projects
func (r *ProjectRegistry) List() []*Project {
	result := make([]*Project, 0, len(r.projects))
	for _, p := range r.projects {
		result = append(result, p)
	}
	return result
}

// Get returns a project by name
func (r *ProjectRegistry) Get(name string) *Project {
	return r.projects[name]
}

// TasksForProject returns tasks linked to a specific project
func (r *ProjectRegistry) TasksForProject(name string, allTasks []data.Task) []data.Task {
	var result []data.Task
	for _, t := range allTasks {
		if t.HasProject(name) {
			result = append(result, t)
		}
	}
	return result
}

// NotesForProject returns notes whose FilePath is under the project's directory.
// Returns nil for virtual projects (no DirPath).
func (r *ProjectRegistry) NotesForProject(name string, allNotes []notes.Note) []notes.Note {
	proj := r.projects[name]
	if proj == nil || proj.DirPath == "" {
		return nil
	}
	prefix := proj.DirPath + "/"
	var result []notes.Note
	for _, n := range allNotes {
		if strings.HasPrefix(n.FilePath, prefix) {
			result = append(result, n)
		}
	}
	return result
}

// ProjectsDirs collects unique parent directories of existing project DirPaths
// where filepath.Base(parent) == "projects". Always includes <workspaceRoot>/projects/
// as a fallback.
func (r *ProjectRegistry) ProjectsDirs(workspaceRoot string) []string {
	seen := make(map[string]bool)
	var dirs []string

	for _, p := range r.projects {
		if p.DirPath == "" {
			continue
		}
		parent := filepath.Dir(p.DirPath)
		if filepath.Base(parent) == "projects" && !seen[parent] {
			seen[parent] = true
			dirs = append(dirs, parent)
		}
	}

	fallback := filepath.Join(workspaceRoot, "projects")
	if !seen[fallback] {
		dirs = append(dirs, fallback)
	}

	return dirs
}

// BoardsForProject returns boards whose Path is under the project's directory.
// Returns nil for virtual projects (no DirPath).
func (r *ProjectRegistry) BoardsForProject(name string, allBoards []kanbanmodels.Board) []kanbanmodels.Board {
	proj := r.projects[name]
	if proj == nil || proj.DirPath == "" {
		return nil
	}
	prefix := proj.DirPath + "/"
	var result []kanbanmodels.Board
	for _, b := range allBoards {
		if strings.HasPrefix(b.Path, prefix) {
			result = append(result, b)
		}
	}
	return result
}

// ProjectsForBoard returns the project names that own the given board path,
// walking up the parent chain to include all ancestor projects.
// Returns nil if the board is not inside any project directory.
func (r *ProjectRegistry) ProjectsForBoard(boardPath string) []string {
	// Find the immediate project whose directory contains the board
	var immediate *Project
	for _, p := range r.projects {
		if p.DirPath == "" {
			continue
		}
		if strings.HasPrefix(boardPath, p.DirPath+"/") {
			if immediate == nil || len(p.DirPath) > len(immediate.DirPath) {
				immediate = p
			}
		}
	}
	if immediate == nil {
		return nil
	}

	// Walk up parent chain collecting all ancestor project names
	var result []string
	for cur := immediate; cur != nil; {
		result = append(result, cur.Name)
		if cur.Parent == "" {
			break
		}
		cur = r.projects[cur.Parent]
	}
	return result
}

// CardsForProject returns cards linked to a specific project across all boards
func (r *ProjectRegistry) CardsForProject(name string, boards []kanbanmodels.Board) []kanbanmodels.Card {
	var result []kanbanmodels.Card
	for _, board := range boards {
		for _, col := range board.Columns {
			for _, card := range col.Cards {
				for _, p := range card.Projects {
					if strings.EqualFold(p, name) {
						result = append(result, card)
						break
					}
				}
			}
		}
	}
	return result
}
