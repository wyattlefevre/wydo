package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wydo/internal/logs"
	"wydo/internal/scanner"
	"wydo/internal/tasks/data"
)

// TaskService defines the interface for task operations.
type TaskService interface {
	List() ([]data.Task, error)
	ListByProject(project string) ([]data.Task, error)
	ListByContext(context string) ([]data.Task, error)
	ListPending() ([]data.Task, error)
	ListDone() ([]data.Task, error)
	Get(id string) (*data.Task, error)
	Add(rawLine string) (*data.Task, error)
	Update(task data.Task) error
	Complete(id string) error
	Delete(id string) error
	Archive() error
	GetProjects() map[string]data.Project
	Reload() error
}

type taskServiceImpl struct {
	tasks    []data.Task
	projects map[string]data.Project
	taskDirs []scanner.TaskDirInfo
}

// NewTaskService creates a new TaskService from discovered task directories
func NewTaskService(taskDirs []scanner.TaskDirInfo) (TaskService, error) {
	svc := &taskServiceImpl{
		taskDirs: taskDirs,
	}
	if err := svc.Reload(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *taskServiceImpl) Reload() error {
	var allTasks []data.Task
	projects := make(map[string]data.Project)

	for i, td := range s.taskDirs {
		// Re-discover .txt files in the directory (handles newly created done.txt)
		files := discoverTxtFiles(td.DirPath)
		if len(files) > 0 {
			s.taskDirs[i].Files = files
		}

		tasks, err := data.LoadTasksFromDir(td.DirPath, s.taskDirs[i].Files, true)
		if err != nil {
			logs.Logger.Printf("Warning: error loading tasks from %s: %v", td.DirPath, err)
			continue
		}
		allTasks = append(allTasks, tasks...)
	}

	// Build project map from task tags
	for _, t := range allTasks {
		for _, p := range t.Projects {
			if _, exists := projects[p]; !exists {
				projects[p] = data.Project{Name: p}
			}
		}
	}

	s.tasks = allTasks
	s.projects = projects
	return nil
}

func discoverTxtFiles(dirPath string) []string {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".txt") {
			files = append(files, e.Name())
		}
	}
	return files
}

func (s *taskServiceImpl) List() ([]data.Task, error) {
	return s.tasks, nil
}

func (s *taskServiceImpl) ListByProject(project string) ([]data.Task, error) {
	var filtered []data.Task
	for _, t := range s.tasks {
		if t.HasProject(project) {
			filtered = append(filtered, t)
		}
	}
	return filtered, nil
}

func (s *taskServiceImpl) ListByContext(context string) ([]data.Task, error) {
	var filtered []data.Task
	for _, t := range s.tasks {
		if t.HasContext(context) {
			filtered = append(filtered, t)
		}
	}
	return filtered, nil
}

func (s *taskServiceImpl) ListPending() ([]data.Task, error) {
	var pending []data.Task
	for _, t := range s.tasks {
		if !t.Done {
			pending = append(pending, t)
		}
	}
	return pending, nil
}

func (s *taskServiceImpl) ListDone() ([]data.Task, error) {
	var done []data.Task
	for _, t := range s.tasks {
		if t.Done {
			done = append(done, t)
		}
	}
	return done, nil
}

func (s *taskServiceImpl) Get(id string) (*data.Task, error) {
	for _, t := range s.tasks {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("task not found: %s", id)
}

// Add appends a task to the first todo.txt found across all task dirs
func (s *taskServiceImpl) Add(rawLine string) (*data.Task, error) {
	targetFile := s.firstTodoFile()
	if targetFile == "" {
		return nil, fmt.Errorf("no todo.txt file found in any task directory")
	}

	task, err := data.AppendTaskToFile(rawLine, targetFile)
	if err != nil {
		return nil, err
	}
	if err := s.Reload(); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *taskServiceImpl) Update(task data.Task) error {
	logs.Logger.Printf("Service: Update Task: %s\n", task.ID)
	s.tasks = data.UpdateTask(s.tasks, task)
	if err := data.WriteAllTasks(s.tasks); err != nil {
		return err
	}
	return s.Reload()
}

// Complete marks a task as done and moves it to done.txt in the same tasks/ directory
func (s *taskServiceImpl) Complete(id string) error {
	task, err := s.Get(id)
	if err != nil {
		return err
	}

	task.Done = true
	task.CompletionDate = time.Now().Format("2006-01-02")

	// Find done.txt in the same tasks/ directory
	taskDir := filepath.Dir(task.File)
	doneFile := filepath.Join(taskDir, "done.txt")
	task.File = doneFile

	s.tasks = data.UpdateTask(s.tasks, *task)
	if err := data.WriteAllTasks(s.tasks); err != nil {
		return err
	}
	return s.Reload()
}

func (s *taskServiceImpl) Delete(id string) error {
	// Remember which file the task was in so we can rewrite it even if empty
	var affectedFile string
	for _, t := range s.tasks {
		if t.ID == id {
			affectedFile = t.File
			break
		}
	}

	s.tasks = data.DeleteTask(s.tasks, id)
	if err := data.WriteAllTasks(s.tasks); err != nil {
		return err
	}

	// If the affected file has no remaining tasks, rewrite it as empty
	if affectedFile != "" {
		hasTasksInFile := false
		for _, t := range s.tasks {
			if t.File == affectedFile {
				hasTasksInFile = true
				break
			}
		}
		if !hasTasksInFile {
			// Rewrite as empty file
			if err := os.WriteFile(affectedFile, []byte{}, 0644); err != nil {
				return err
			}
		}
	}

	return s.Reload()
}

// Archive moves done tasks to done.txt within each tasks/ directory
func (s *taskServiceImpl) Archive() error {
	for i := range s.tasks {
		if s.tasks[i].Done {
			taskDir := filepath.Dir(s.tasks[i].File)
			doneFile := filepath.Join(taskDir, "done.txt")
			s.tasks[i].File = doneFile
		}
	}
	if err := data.WriteAllTasks(s.tasks); err != nil {
		return err
	}
	return s.Reload()
}

func (s *taskServiceImpl) GetProjects() map[string]data.Project {
	return s.projects
}

// firstTodoFile returns the path to the first todo.txt found
func (s *taskServiceImpl) firstTodoFile() string {
	for _, td := range s.taskDirs {
		for _, f := range td.Files {
			if f == "todo.txt" {
				return filepath.Join(td.DirPath, f)
			}
		}
	}
	// Fallback: first file in first dir
	if len(s.taskDirs) > 0 && len(s.taskDirs[0].Files) > 0 {
		return filepath.Join(s.taskDirs[0].DirPath, s.taskDirs[0].Files[0])
	}
	return ""
}
