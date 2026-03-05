package workspace

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	ws.Projects = BuildProjectRegistry(scan, ws.Tasks, ws.Boards, scan.RootDir)

	return ws, nil
}

// MoveProjectToParent moves a project's directory under a new parent project (or to root).
// If newParent is nil, moves to the first ProjectsDir of the workspace (root-level).
// Updates project.DirPath and project.Parent in memory; caller must emit DataRefreshMsg.
func (ws *Workspace) MoveProjectToParent(project *Project, newParent *Project) error {
	if project.DirPath == "" {
		return fmt.Errorf("cannot move virtual project %q", project.Name)
	}
	var targetBase string
	if newParent == nil {
		// Move to root — use the first ProjectsDir
		dirs := ws.Projects.ProjectsDirs(ws.RootDir)
		if len(dirs) > 0 {
			targetBase = dirs[0]
		} else {
			targetBase = filepath.Join(ws.RootDir, "projects")
		}
	} else {
		if newParent.DirPath == "" {
			return fmt.Errorf("cannot move under virtual project %q", newParent.Name)
		}
		targetBase = filepath.Join(newParent.DirPath, "projects")
	}
	targetDir := filepath.Join(targetBase, project.Name)
	if targetDir == project.DirPath {
		return nil // already in the right place
	}
	if err := os.MkdirAll(targetBase, 0o755); err != nil {
		return err
	}
	if err := os.Rename(project.DirPath, targetDir); err != nil {
		return err
	}
	project.DirPath = targetDir
	if newParent == nil {
		project.Parent = ""
	} else {
		project.Parent = newParent.Name
	}
	return nil
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

// DeleteVirtualProject removes all references to a virtual project from tasks and cards,
// and removes it from the virtual archive file.
func DeleteVirtualProject(ws *Workspace, projectName string) error {
	project := ws.Projects.Get(projectName)
	if project == nil {
		return fmt.Errorf("project %q not found", projectName)
	}
	if project.DirPath != "" {
		return fmt.Errorf("DeleteVirtualProject called on physical project %q", projectName)
	}

	// Remove from task +tags
	modified := false
	for i := range ws.Tasks {
		if ws.Tasks[i].HasProject(projectName) {
			ws.Tasks[i].RemoveProject(projectName)
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

	// Remove from card frontmatter
	for bi := range ws.Boards {
		for ci := range ws.Boards[bi].Columns {
			for cdi := range ws.Boards[bi].Columns[ci].Cards {
				card := &ws.Boards[bi].Columns[ci].Cards[cdi]
				var hasProject bool
				for _, p := range card.Projects {
					if strings.EqualFold(p, projectName) {
						hasProject = true
						break
					}
				}
				if !hasProject {
					continue
				}
				filtered := card.Projects[:0]
				for _, p := range card.Projects {
					if !strings.EqualFold(p, projectName) {
						filtered = append(filtered, p)
					}
				}
				card.Projects = filtered
				cardPath := filepath.Join(ws.Boards[bi].Path, "cards", card.Filename)
				if err := fs.WriteCard(*card, cardPath); err != nil {
					return fmt.Errorf("write card %s: %w", card.Filename, err)
				}
			}
		}
	}

	return RemoveFromVirtualArchive(ws.RootDir, projectName)
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

// ProjectDate is a labeled date stored in a project's index frontmatter
type ProjectDate struct {
	Label string
	Date  time.Time
}

// Project represents a discovered project
type Project struct {
	Name     string
	DirPath  string // from projects/ directory, "" if virtual
	Parent   string
	Archived bool
	Dates    []ProjectDate // from index frontmatter
}

// ProjectRegistry manages project discovery and cross-entity queries within a workspace
type ProjectRegistry struct {
	projects map[string]*Project
}

// BuildProjectRegistry builds a registry from scan results, tasks, and boards
func BuildProjectRegistry(scan *scanner.WorkspaceScan, tasks []data.Task, boards []kanbanmodels.Board, wsRoot string) *ProjectRegistry {
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

	// Apply virtual archive: mark archived virtual projects
	if wsRoot != "" {
		virtualArchived := readVirtualArchive(wsRoot)
		for _, p := range r.projects {
			if p.DirPath == "" && virtualArchived[p.Name] {
				p.Archived = true
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
			archived, dates := readProjectFrontmatter(dirPath, name)
			existing.Archived = archived
			existing.Dates = dates
		}
		if parent != "" && existing.Parent == "" {
			existing.Parent = parent
		}
		return
	}
	var archived bool
	var dates []ProjectDate
	if dirPath != "" {
		archived, dates = readProjectFrontmatter(dirPath, name)
	}
	r.projects[name] = &Project{
		Name:     name,
		DirPath:  dirPath,
		Parent:   parent,
		Archived: archived,
		Dates:    dates,
	}
}

// projectIndexFM is the shared YAML frontmatter structure for project index files
type projectIndexFM struct {
	Archived bool `yaml:"archived"`
	Dates    []struct {
		Label string `yaml:"label"`
		Date  string `yaml:"date"`
	} `yaml:"dates,omitempty"`
}

// readProjectFrontmatter reads the project index file and returns archived status and dates
func readProjectFrontmatter(dirPath, name string) (archived bool, dates []ProjectDate) {
	indexPath := filepath.Join(dirPath, name+".md")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		return false, nil
	}

	lines := bytes.Split(content, []byte("\n"))
	if len(lines) == 0 || !bytes.Equal(bytes.TrimSpace(lines[0]), []byte("---")) {
		return false, nil
	}

	var frontmatterEnd int
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(bytes.TrimSpace(lines[i]), []byte("---")) {
			frontmatterEnd = i
			break
		}
	}
	if frontmatterEnd == 0 {
		return false, nil
	}

	frontmatterBytes := bytes.Join(lines[1:frontmatterEnd], []byte("\n"))
	var fm projectIndexFM
	if err := yaml.Unmarshal(frontmatterBytes, &fm); err != nil {
		return false, nil
	}

	for _, d := range fm.Dates {
		t, err := time.Parse("2006-01-02", d.Date)
		if err != nil {
			continue
		}
		dates = append(dates, ProjectDate{Label: d.Label, Date: t})
	}

	return fm.Archived, dates
}

// writeProjectFrontmatter serializes the project's archived flag and dates back to the index file,
// preserving the body content.
func writeProjectFrontmatter(project *Project) error {
	indexPath := filepath.Join(project.DirPath, project.Name+".md")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		// File doesn't exist — create it
		content = []byte(fmt.Sprintf("# %s\n", project.Name))
	}

	// Strip existing frontmatter, keep body
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

	// Build new frontmatter — only emit if something is non-zero
	needsFM := project.Archived || len(project.Dates) > 0
	var buf bytes.Buffer
	if needsFM {
		buf.WriteString("---\n")
		if project.Archived {
			buf.WriteString("archived: true\n")
		}
		if len(project.Dates) > 0 {
			buf.WriteString("dates:\n")
			for _, d := range project.Dates {
				buf.WriteString(fmt.Sprintf("  - label: %s\n    date: %s\n", d.Label, d.Date.Format("2006-01-02")))
			}
		}
		buf.WriteString("---\n\n")
	}
	buf.Write(body)

	return os.WriteFile(indexPath, buf.Bytes(), 0644)
}

// SetProjectArchived sets the archived state for a project by updating its index file frontmatter.
// Returns an error for virtual projects (no DirPath).
func SetProjectArchived(project *Project, archived bool) error {
	if project.DirPath == "" {
		return fmt.Errorf("cannot archive virtual project %q", project.Name)
	}
	project.Archived = archived
	return writeProjectFrontmatter(project)
}

const virtualArchiveFilename = ".wydo-virtual-archive.txt"

func virtualArchivePath(wsRoot string) string {
	return filepath.Join(wsRoot, virtualArchiveFilename)
}

// readVirtualArchive returns the set of archived virtual project names.
// Returns empty map if file does not exist.
func readVirtualArchive(wsRoot string) map[string]bool {
	data, err := os.ReadFile(virtualArchivePath(wsRoot))
	if err != nil {
		return make(map[string]bool)
	}
	result := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			result[name] = true
		}
	}
	return result
}

// writeVirtualArchive writes the archived set. Removes the file when empty.
func writeVirtualArchive(wsRoot string, archived map[string]bool) error {
	path := virtualArchivePath(wsRoot)
	if len(archived) == 0 {
		_ = os.Remove(path)
		return nil
	}
	var names []string
	for name := range archived {
		names = append(names, name)
	}
	sort.Strings(names)
	return os.WriteFile(path, []byte(strings.Join(names, "\n")+"\n"), 0644)
}

// SetVirtualProjectArchived archives/unarchives a virtual project in the workspace archive file.
func SetVirtualProjectArchived(wsRoot string, project *Project, archived bool) error {
	existing := readVirtualArchive(wsRoot)
	if archived {
		existing[project.Name] = true
	} else {
		delete(existing, project.Name)
	}
	if err := writeVirtualArchive(wsRoot, existing); err != nil {
		return err
	}
	project.Archived = archived
	return nil
}

// RemoveFromVirtualArchive removes a name from the virtual archive file (no-op if absent).
func RemoveFromVirtualArchive(wsRoot, name string) error {
	existing := readVirtualArchive(wsRoot)
	if !existing[name] {
		return nil
	}
	delete(existing, name)
	return writeVirtualArchive(wsRoot, existing)
}

// WriteProjectDates sets the project's dates and persists them to the index file frontmatter.
// Returns an error for virtual projects (no DirPath).
func WriteProjectDates(project *Project, dates []ProjectDate) error {
	if project.DirPath == "" {
		return fmt.Errorf("cannot write dates for virtual project %q", project.Name)
	}
	project.Dates = dates
	return writeProjectFrontmatter(project)
}

// ChildrenOf returns all projects whose Parent equals name (case-insensitive).
func (r *ProjectRegistry) ChildrenOf(name string) []*Project {
	var result []*Project
	for _, p := range r.projects {
		if strings.EqualFold(p.Parent, name) {
			result = append(result, p)
		}
	}
	return result
}

// ReadIndexPreview returns the first 4 non-empty body lines of the project index file.
// Strips YAML frontmatter. Returns "" if no file or empty body.
func ReadIndexPreview(dirPath, name string) string {
	indexPath := filepath.Join(dirPath, name+".md")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		return ""
	}

	// Strip frontmatter
	body := content
	lines := bytes.Split(content, []byte("\n"))
	if len(lines) > 0 && bytes.Equal(bytes.TrimSpace(lines[0]), []byte("---")) {
		for i := 1; i < len(lines); i++ {
			if bytes.Equal(bytes.TrimSpace(lines[i]), []byte("---")) {
				body = bytes.Join(lines[i+1:], []byte("\n"))
				break
			}
		}
	}

	// Collect up to 4 non-empty lines
	bodyLines := bytes.Split(body, []byte("\n"))
	var preview []string
	for _, l := range bodyLines {
		trimmed := bytes.TrimSpace(l)
		if len(trimmed) > 0 {
			preview = append(preview, string(trimmed))
		}
		if len(preview) >= 4 {
			break
		}
	}
	return strings.Join(preview, "\n")
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
