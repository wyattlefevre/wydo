package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTasksFromDir(t *testing.T) {
	wd, _ := os.Getwd()
	tasksDir := filepath.Join(wd, "..", "..", "..", "testdata", "workspace1", "tasks")

	tasks, err := LoadTasksFromDir(tasksDir, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) == 0 {
		t.Fatal("expected tasks to be loaded")
	}

	// All tasks should come from todo.txt
	for _, task := range tasks {
		if filepath.Base(task.File) != "todo.txt" {
			t.Errorf("expected file todo.txt, got %q", filepath.Base(task.File))
		}
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
	loaded, err := LoadTasksFromDir(tmpDir, true)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(loaded))
	}
}

func TestWriteAllTasks(t *testing.T) {
	tmpDir := t.TempDir()
	todoPath := filepath.Join(tmpDir, "tasks", "todo.txt")
	archivePath := filepath.Join(tmpDir, "archive", "tasks", "todo.txt")

	tasks := []Task{
		{ID: "1", Name: "Pending task", File: todoPath},
		{ID: "2", Name: "Done task", Done: true, File: archivePath},
	}

	err := WriteAllTasks(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify both files exist
	if _, err := os.Stat(todoPath); err != nil {
		t.Error("tasks/todo.txt should exist")
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Error("archive/tasks/todo.txt should exist")
	}

	// Load and verify
	activeTasks, _ := LoadTasksFromDir(filepath.Join(tmpDir, "tasks"), true)
	archivedTasks, _ := LoadTasksFromDir(filepath.Join(tmpDir, "archive", "tasks"), true)

	if len(activeTasks) != 1 {
		t.Errorf("expected 1 active task, got %d", len(activeTasks))
	}
	if len(archivedTasks) != 1 {
		t.Errorf("expected 1 archived task, got %d", len(archivedTasks))
	}
}

func TestTaskFileTracking(t *testing.T) {
	wd, _ := os.Getwd()
	tasksDir := filepath.Join(wd, "..", "..", "..", "testdata", "workspace1", "tasks")

	tasks, err := LoadTasksFromDir(tasksDir, true)
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
	// Verify that tasks with project tags are loaded correctly from any directory.
	tmpDir := t.TempDir()
	todoFile := filepath.Join(tmpDir, "todo.txt")
	if err := os.WriteFile(todoFile, []byte("Fix bug +alpha\nWrite tests +alpha\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tasks, err := LoadTasksFromDir(tmpDir, true)
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

// TestIDsStableAcrossWriteReload is the regression test for the duplication bug.
// A file with blank lines is loaded, written (blank lines removed), then reloaded.
// IDs must be identical before and after the write.
func TestIDsStableAcrossWriteReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	if err := os.WriteFile(path, []byte("task one\n\ntask two\n\ntask three\n"), 0644); err != nil {
		t.Fatal(err)
	}

	first, err := loadTaskFile(path, true, make(map[string]Project))
	if err != nil {
		t.Fatalf("initial load: %v", err)
	}
	if len(first) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(first))
	}

	idsBefore := make([]string, len(first))
	for i, task := range first {
		idsBefore[i] = task.ID
	}

	// Write strips blank lines
	if err := writeTaskSliceToFile(first, path); err != nil {
		t.Fatalf("write: %v", err)
	}

	second, err := loadTaskFile(path, true, make(map[string]Project))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(second) != 3 {
		t.Fatalf("after reload: expected 3 tasks, got %d", len(second))
	}

	for i, task := range second {
		if task.ID != idsBefore[i] {
			t.Errorf("task %q: ID changed from %s to %s after write+reload", task.Name, idsBefore[i], task.ID)
		}
	}
}

// TestUpdateTaskNoDuplicateOnMiss ensures UpdateTask does not append when the ID
// is not found — the silent-append was the mechanism that turned ID misses into
// duplicate tasks.
func TestUpdateTaskNoDuplicateOnMiss(t *testing.T) {
	tasks := []Task{
		{ID: "aaa", Name: "task one"},
		{ID: "bbb", Name: "task two"},
	}
	ghost := Task{ID: "nonexistent", Name: "ghost task"}
	result := UpdateTask(tasks, ghost)
	if len(result) != 2 {
		t.Errorf("UpdateTask with unknown ID should not append: got %d tasks, want 2", len(result))
	}
}

// TestUpdateTaskModifiesCorrectEntry ensures a found task is updated in place.
func TestUpdateTaskModifiesCorrectEntry(t *testing.T) {
	tasks := []Task{
		{ID: "aaa", Name: "original"},
		{ID: "bbb", Name: "other"},
	}
	updated := Task{ID: "aaa", Name: "modified"}
	result := UpdateTask(tasks, updated)
	if len(result) != 2 {
		t.Errorf("UpdateTask should preserve length: got %d", len(result))
	}
	if result[0].Name != "modified" {
		t.Errorf("task not updated: got %q", result[0].Name)
	}
	if result[1].Name != "other" {
		t.Errorf("wrong task modified: got %q", result[1].Name)
	}
}

// TestAppendTaskConsistentID verifies AppendTaskToFile and loadTaskFile assign the
// same ID to a task (same non-empty-line-count hash scheme).
func TestAppendTaskConsistentID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	// File with a blank line before the target position
	if err := os.WriteFile(path, []byte("task one\n\ntask two\n"), 0644); err != nil {
		t.Fatal(err)
	}

	appended, err := AppendTaskToFile("task three", path)
	if err != nil {
		t.Fatalf("AppendTaskToFile: %v", err)
	}

	tasks, err := loadTaskFile(path, true, make(map[string]Project))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var found *Task
	for i := range tasks {
		if tasks[i].Name == "task three" {
			found = &tasks[i]
			break
		}
	}
	if found == nil {
		t.Fatal("appended task not found after reload")
	}
	if found.ID != appended.ID {
		t.Errorf("ID mismatch: AppendTaskToFile gave %s, loadTaskFile gave %s", appended.ID, found.ID)
	}
}

// TestUpdateAfterWriteReloadNoDuplication simulates the full bug scenario:
// load → mark done → write → reload → update again → no duplicates.
func TestUpdateAfterWriteReloadNoDuplication(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	if err := os.WriteFile(path, []byte("task one\n\ntask two\n\ntask three\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tasks, err := loadTaskFile(path, true, make(map[string]Project))
	if err != nil {
		t.Fatalf("initial load: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3, got %d", len(tasks))
	}

	// Mark second task done
	tasks[1].Done = true
	if err := writeTaskSliceToFile(tasks, path); err != nil {
		t.Fatalf("write: %v", err)
	}
	tasks, err = loadTaskFile(path, true, make(map[string]Project))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("after first reload: expected 3, got %d", len(tasks))
	}

	// Mark first task done — exercises ID lookup after blank lines were stripped
	tasks[0].Done = true
	tasks = UpdateTask(tasks, tasks[0])
	if err := writeTaskSliceToFile(tasks, path); err != nil {
		t.Fatalf("second write: %v", err)
	}
	tasks, err = loadTaskFile(path, true, make(map[string]Project))
	if err != nil {
		t.Fatalf("second reload: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("after second reload: expected 3, got %d — duplication bug!", len(tasks))
	}
}
