package service

import (
	"os"
	"path/filepath"
	"testing"

	"wydo/internal/scanner"
	"wydo/internal/tasks/data"
)

func setupTestDirs(t *testing.T) (string, []scanner.TaskDirInfo) {
	tmpDir := t.TempDir()

	// Create two task directories
	dir1 := filepath.Join(tmpDir, "tasks1")
	dir2 := filepath.Join(tmpDir, "tasks2")
	os.MkdirAll(dir1, 0755)
	os.MkdirAll(dir2, 0755)

	os.WriteFile(filepath.Join(dir1, "todo.txt"), []byte(
		"(A) Task from dir1 +alpha @computer\nBuy groceries @errands\n",
	), 0644)

	os.WriteFile(filepath.Join(dir2, "todo.txt"), []byte(
		"(B) Task from dir2 +beta @work\n",
	), 0644)

	taskDirs := []scanner.TaskDirInfo{
		{DirPath: dir1},
		{DirPath: dir2},
	}

	return tmpDir, taskDirs
}

func TestMultiSourceLoad(t *testing.T) {
	_, taskDirs := setupTestDirs(t)

	svc, err := NewTaskService(taskDirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tasks, _ := svc.List()
	if len(tasks) < 2 {
		t.Fatalf("expected at least 2 tasks from 2 dirs, got %d", len(tasks))
	}

	// Verify tasks from both dirs are present
	hasDir1 := false
	hasDir2 := false
	for _, task := range tasks {
		if task.HasProject("alpha") {
			hasDir1 = true
		}
		if task.HasProject("beta") {
			hasDir2 = true
		}
	}

	if !hasDir1 {
		t.Error("expected tasks from dir1")
	}
	if !hasDir2 {
		t.Error("expected tasks from dir2")
	}
}

func TestAddDefaultTarget(t *testing.T) {
	_, taskDirs := setupTestDirs(t)

	svc, err := NewTaskService(taskDirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task, err := svc.Add("New task +gamma")
	if err != nil {
		t.Fatalf("add error: %v", err)
	}

	if task == nil {
		t.Fatal("expected non-nil task")
	}

	// Task should have been appended to first todo.txt
	if filepath.Base(task.File) != "todo.txt" {
		t.Errorf("expected task in todo.txt, got %q", task.File)
	}
}

func TestCompleteMovesToDone(t *testing.T) {
	_, taskDirs := setupTestDirs(t)

	svc, err := NewTaskService(taskDirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pendingBefore, _ := svc.ListPending()
	doneBefore, _ := svc.ListDone()
	pendingCountBefore := len(pendingBefore)
	doneCountBefore := len(doneBefore)

	if pendingCountBefore == 0 {
		t.Fatal("expected pending tasks")
	}

	taskID := pendingBefore[0].ID
	taskName := pendingBefore[0].Name
	taskDir := filepath.Dir(pendingBefore[0].File)

	err = svc.Complete(taskID)
	if err != nil {
		t.Fatalf("complete error: %v", err)
	}

	// After complete+reload, there should be one fewer pending and one more done
	pendingAfter, _ := svc.ListPending()
	doneAfter, _ := svc.ListDone()

	if len(pendingAfter) != pendingCountBefore-1 {
		t.Errorf("expected %d pending, got %d", pendingCountBefore-1, len(pendingAfter))
	}
	if len(doneAfter) != doneCountBefore+1 {
		t.Errorf("expected %d done, got %d", doneCountBefore+1, len(doneAfter))
	}

	// The completed task should stay in todo.txt (same dir), not moved to done.txt
	found := false
	for _, dt := range doneAfter {
		if dt.Name == taskName {
			found = true
			expectedFile := filepath.Join(taskDir, "todo.txt")
			if dt.File != expectedFile {
				t.Errorf("expected file %q, got %q", expectedFile, dt.File)
			}
			break
		}
	}
	if !found {
		t.Errorf("completed task %q not found in done list", taskName)
	}
}

func TestDeleteRewritesCorrectFile(t *testing.T) {
	_, taskDirs := setupTestDirs(t)

	svc, err := NewTaskService(taskDirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tasks, _ := svc.List()
	initialCount := len(tasks)
	if initialCount == 0 {
		t.Fatal("expected tasks")
	}

	err = svc.Delete(tasks[0].ID)
	if err != nil {
		t.Fatalf("delete error: %v", err)
	}

	tasks, _ = svc.List()
	if len(tasks) != initialCount-1 {
		t.Errorf("expected %d tasks after delete, got %d", initialCount-1, len(tasks))
	}
}

func TestAddWithEditorStyleTask(t *testing.T) {
	_, taskDirs := setupTestDirs(t)

	svc, err := NewTaskService(taskDirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Simulate what the task editor produces: priority, projects, contexts, due date
	tests := []struct {
		name    string
		rawLine string
		wantErr bool
	}{
		{"simple task", "Buy milk", false},
		{"with priority", "(A) Important task", false},
		{"with project and context", "Fix bug +myproject @work", false},
		{"with due date", "Submit report due:2026-03-15", false},
		{"full editor task", "(B) Write docs +myproject @computer due:2026-04-01", false},
		{"empty task", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := svc.Add(tt.rawLine)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if task == nil {
				t.Fatal("expected non-nil task")
			}
			if filepath.Base(task.File) != "todo.txt" {
				t.Errorf("expected task in todo.txt, got %q", task.File)
			}
		})
	}
}

func TestUpdateWithEmptyFileDoesNotPersist(t *testing.T) {
	_, taskDirs := setupTestDirs(t)

	svc, err := NewTaskService(taskDirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	beforeTasks, _ := svc.List()
	beforeCount := len(beforeTasks)

	// Create a task with empty File (simulates what createNewTaskAndOpenEditor does)
	emptyFileTask := data.Task{
		ID:       "test-empty-file",
		Name:     "Ghost task",
		Projects: []string{},
		Tags:     []string{},
		Properties:   map[string]string{},
		File:     "", // This is the bug: empty file
	}

	// Update with empty file — WriteAllTasks skips tasks with File==""
	err = svc.Update(emptyFileTask)
	if err != nil {
		t.Fatalf("update error: %v", err)
	}

	// After Reload (which Update calls internally), the task should be gone
	afterTasks, _ := svc.List()
	if len(afterTasks) != beforeCount {
		t.Errorf("expected %d tasks after update+reload (task with empty File lost), got %d", beforeCount, len(afterTasks))
	}
}

func TestAddPersistsAcrossReload(t *testing.T) {
	_, taskDirs := setupTestDirs(t)

	svc, err := NewTaskService(taskDirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	beforeTasks, _ := svc.List()
	beforeCount := len(beforeTasks)

	// Add a task via Add() — the correct way to persist new tasks
	task, err := svc.Add("(A) Persistent task +testproject due:2026-05-01")
	if err != nil {
		t.Fatalf("add error: %v", err)
	}
	if task == nil {
		t.Fatal("expected non-nil task")
	}

	// Reload to simulate app restart
	err = svc.Reload()
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}

	afterTasks, _ := svc.List()
	if len(afterTasks) != beforeCount+1 {
		t.Errorf("expected %d tasks after add+reload, got %d", beforeCount+1, len(afterTasks))
	}

	// Verify the task content survived
	found := false
	for _, t := range afterTasks {
		if t.Name == "Persistent task" {
			found = true
			break
		}
	}
	if !found {
		t.Error("added task not found after reload")
	}
}

func TestArchivePerDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	dir1 := filepath.Join(tmpDir, "tasks1")
	os.MkdirAll(dir1, 0755)
	os.WriteFile(filepath.Join(dir1, "todo.txt"), []byte(
		"Pending task\nx 2026-02-01 Completed task\n",
	), 0644)

	taskDirs := []scanner.TaskDirInfo{
		{DirPath: dir1},
	}

	svc, err := NewTaskService(taskDirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = svc.Archive()
	if err != nil {
		t.Fatalf("archive error: %v", err)
	}

	// archive/tasks/todo.txt should now exist under the workspace root (parent of dir1)
	workspaceRoot := filepath.Dir(dir1)
	archiveFile := filepath.Join(workspaceRoot, "archive", "tasks", "todo.txt")
	if _, err := os.Stat(archiveFile); err != nil {
		t.Errorf("expected archive/tasks/todo.txt to be created after archive, got: %v", err)
	}

	// Reload and verify: 1 pending in todo.txt, 1 done in archive
	tasks, _ := svc.ListPending()
	done, _ := svc.ListDone()

	if len(tasks) != 1 {
		t.Errorf("expected 1 pending task, got %d", len(tasks))
	}
	if len(done) != 1 {
		t.Errorf("expected 1 done task, got %d", len(done))
	}
}

// TestDeleteByIDs_AllDeletedInOneBatch verifies that all selected IDs are deleted
// even when the list has multiple tasks. The old loop-with-Delete approach failed
// because each Delete triggered Reload, reassigning IDs from new line numbers and
// making subsequent IDs in the batch stale.
func TestDeleteByIDs_AllDeletedInOneBatch(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	os.MkdirAll(tasksDir, 0755)
	os.WriteFile(filepath.Join(tasksDir, "todo.txt"), []byte("task one\ntask two\ntask three\n"), 0644)

	svc, err := NewTaskService([]scanner.TaskDirInfo{{DirPath: tasksDir}})
	if err != nil {
		t.Fatalf("NewTaskService: %v", err)
	}

	tasks, _ := svc.List()
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	ids := []string{tasks[0].ID, tasks[1].ID, tasks[2].ID}
	if err := svc.DeleteByIDs(ids); err != nil {
		t.Fatalf("DeleteByIDs: %v", err)
	}

	remaining, _ := svc.List()
	if len(remaining) != 0 {
		t.Errorf("expected 0 tasks after deleting all, got %d", len(remaining))
	}
}

// TestDeleteByIDs_PartialDelete verifies that only the specified tasks are removed.
func TestDeleteByIDs_PartialDelete(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	os.MkdirAll(tasksDir, 0755)
	os.WriteFile(filepath.Join(tasksDir, "todo.txt"), []byte("task one\ntask two\ntask three\n"), 0644)

	svc, err := NewTaskService([]scanner.TaskDirInfo{{DirPath: tasksDir}})
	if err != nil {
		t.Fatalf("NewTaskService: %v", err)
	}

	tasks, _ := svc.List()
	ids := []string{tasks[0].ID, tasks[2].ID} // delete first and third
	if err := svc.DeleteByIDs(ids); err != nil {
		t.Fatalf("DeleteByIDs: %v", err)
	}

	remaining, _ := svc.List()
	if len(remaining) != 1 {
		t.Fatalf("expected 1 task remaining, got %d", len(remaining))
	}
	if remaining[0].Name != "task two" {
		t.Errorf("wrong task remaining: got %q, want %q", remaining[0].Name, "task two")
	}
}

// TestDeleteByIDs_WithBlankLines confirms batch delete works when the source file
// had blank lines (same ID-stability scenario as the duplication bug).
func TestDeleteByIDs_WithBlankLines(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	os.MkdirAll(tasksDir, 0755)
	os.WriteFile(filepath.Join(tasksDir, "todo.txt"), []byte("task one\n\ntask two\n\ntask three\n"), 0644)

	svc, err := NewTaskService([]scanner.TaskDirInfo{{DirPath: tasksDir}})
	if err != nil {
		t.Fatalf("NewTaskService: %v", err)
	}

	tasks, _ := svc.List()
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	ids := []string{tasks[0].ID, tasks[1].ID, tasks[2].ID}
	if err := svc.DeleteByIDs(ids); err != nil {
		t.Fatalf("DeleteByIDs: %v", err)
	}

	remaining, _ := svc.List()
	if len(remaining) != 0 {
		t.Errorf("expected 0 tasks, got %d — stale-ID bug", len(remaining))
	}
}
