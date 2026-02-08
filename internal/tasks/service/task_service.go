package service

import (
	"fmt"
	"time"

	"wydo/internal/logs"
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
	tasks        []data.Task
	projects     map[string]data.Project
	todoFilePath string
	doneFilePath string
	projDir      string
}

// NewTaskService creates a new TaskService for specific file paths
func NewTaskService(todoFilePath, doneFilePath, projDir string) (TaskService, error) {
	svc := &taskServiceImpl{
		todoFilePath: todoFilePath,
		doneFilePath: doneFilePath,
		projDir:      projDir,
	}
	if err := svc.Reload(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *taskServiceImpl) Reload() error {
	tasks, projects, err := data.LoadDataFromFiles(s.todoFilePath, s.doneFilePath, s.projDir, true)
	if err != nil {
		return err
	}
	s.tasks = tasks
	s.projects = projects
	return nil
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

func (s *taskServiceImpl) Add(rawLine string) (*data.Task, error) {
	task, err := data.AppendTaskToFile(rawLine, s.todoFilePath)
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
	data.UpdateTask(s.tasks, task)
	if err := data.WriteDataToFiles(s.tasks, s.todoFilePath, s.doneFilePath); err != nil {
		return err
	}
	return s.Reload()
}

func (s *taskServiceImpl) Complete(id string) error {
	task, err := s.Get(id)
	if err != nil {
		return err
	}

	task.Done = true
	task.CompletionDate = time.Now().Format("2006-01-02")
	task.File = s.doneFilePath

	data.UpdateTask(s.tasks, *task)
	if err := data.WriteDataToFiles(s.tasks, s.todoFilePath, s.doneFilePath); err != nil {
		return err
	}
	return s.Reload()
}

func (s *taskServiceImpl) Delete(id string) error {
	s.tasks = data.DeleteTask(s.tasks, id)
	if err := data.WriteDataToFiles(s.tasks, s.todoFilePath, s.doneFilePath); err != nil {
		return err
	}
	return s.Reload()
}

func (s *taskServiceImpl) Archive() error {
	if err := data.ArchiveDoneToFiles(s.tasks, s.doneFilePath); err != nil {
		return err
	}
	return s.Reload()
}

func (s *taskServiceImpl) GetProjects() map[string]data.Project {
	return s.projects
}
