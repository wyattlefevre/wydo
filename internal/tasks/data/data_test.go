package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTasksFromDir(t *testing.T) {
	wd, _ := os.Getwd()
	tasksDir := filepath.Join(wd, "..", "..", "..", "testdata", "workspace1", "tasks")

	tasks, err := LoadTasksFromDir(tasksDir, []string{"todo.txt", "done.txt"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) == 0 {
		t.Fatal("expected tasks to be loaded")
	}

	// Check that tasks from both files are loaded
	hasFromTodo := false
	hasFromDone := false
	for _, task := range tasks {
		if filepath.Base(task.File) == "todo.txt" {
			hasFromTodo = true
		}
		if filepath.Base(task.File) == "done.txt" {
			hasFromDone = true
		}
	}

	if !hasFromTodo {
		t.Error("expected tasks from todo.txt")
	}
	if !hasFromDone {
		t.Error("expected tasks from done.txt")
	}
}

func TestWriteTasksToFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "todo.txt")

	tasks := []Task{
		{ID: "1", Name: "Task one", File: filePath},
		{ID: "2", Name: "Task two", File: filePath},
		{ID: "3", Name: "Other file task", File: filepath.Join(tmpDir, "other.txt")},
	}

	err := WriteTasksToFile(tasks, filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read back
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}

	s := string(content)
	if s == "" {
		t.Fatal("expected content")
	}

	// Should only contain tasks for this file (not "Other file task")
	loaded, err := LoadTasksFromDir(tmpDir, []string{"todo.txt"}, true)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(loaded))
	}
}

func TestWriteAllTasks(t *testing.T) {
	tmpDir := t.TempDir()
	todoPath := filepath.Join(tmpDir, "todo.txt")
	donePath := filepath.Join(tmpDir, "done.txt")

	tasks := []Task{
		{ID: "1", Name: "Pending task", File: todoPath},
		{ID: "2", Name: "Done task", Done: true, File: donePath},
	}

	err := WriteAllTasks(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify both files exist
	if _, err := os.Stat(todoPath); err != nil {
		t.Error("todo.txt should exist")
	}
	if _, err := os.Stat(donePath); err != nil {
		t.Error("done.txt should exist")
	}

	// Load and verify
	todoTasks, _ := LoadTasksFromDir(tmpDir, []string{"todo.txt"}, true)
	doneTasks, _ := LoadTasksFromDir(tmpDir, []string{"done.txt"}, true)

	if len(todoTasks) != 1 {
		t.Errorf("expected 1 todo task, got %d", len(todoTasks))
	}
	if len(doneTasks) != 1 {
		t.Errorf("expected 1 done task, got %d", len(doneTasks))
	}
}

func TestTaskFileTracking(t *testing.T) {
	wd, _ := os.Getwd()
	tasksDir := filepath.Join(wd, "..", "..", "..", "testdata", "workspace1", "tasks")

	tasks, err := LoadTasksFromDir(tasksDir, []string{"todo.txt"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, task := range tasks {
		if task.File == "" {
			t.Errorf("task %q has empty File field", task.Name)
		}
		if filepath.Base(task.File) != "todo.txt" {
			t.Errorf("task %q File should end with todo.txt, got %q", task.Name, task.File)
		}
	}
}

func TestLoadTasksFromDir_MultipleProjects(t *testing.T) {
	wd, _ := os.Getwd()

	// Load alpha project tasks
	alphaDir := filepath.Join(wd, "..", "..", "..", "testdata", "workspace1", "projects", "alpha", "tasks")
	tasks, err := LoadTasksFromDir(alphaDir, []string{"todo.txt"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) == 0 {
		t.Fatal("expected tasks from alpha project")
	}

	// All should have +alpha
	for _, task := range tasks {
		if !task.HasProject("alpha") {
			t.Errorf("task %q should have +alpha project", task.Name)
		}
	}
}
