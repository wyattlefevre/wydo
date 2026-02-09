package workspace

import (
	"strings"

	"wydo/internal/kanban/fs"
	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/notes"
	"wydo/internal/scanner"
	"wydo/internal/tasks/data"
	"wydo/internal/tasks/service"
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

// Project represents a discovered project
type Project struct {
	Name     string
	DirPath  string // from projects/ directory, "" if virtual
	Parent   string
	Children []string
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
		}
		if parent != "" && existing.Parent == "" {
			existing.Parent = parent
		}
		return
	}
	r.projects[name] = &Project{
		Name:    name,
		DirPath: dirPath,
		Parent:  parent,
	}
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
