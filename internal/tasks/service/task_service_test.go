package service

import (
	"os"
	"path/filepath"
	"testing"

	"wydo/internal/scanner"
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
	os.WriteFile(filepath.Join(dir1, "done.txt"), []byte(
		"x 2026-02-01 Done task from dir1\n",
	), 0644)

	os.WriteFile(filepath.Join(dir2, "todo.txt"), []byte(
		"(B) Task from dir2 +beta @work\n",
	), 0644)

	taskDirs := []scanner.TaskDirInfo{
		{DirPath: dir1, Files: []string{"todo.txt", "done.txt"}},
		{DirPath: dir2, Files: []string{"todo.txt"}},
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
	if len(tasks) < 3 {
		t.Fatalf("expected at least 3 tasks from 2 dirs, got %d", len(tasks))
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

	// The completed task should be in done.txt in the same directory
	found := false
	for _, dt := range doneAfter {
		if dt.Name == taskName {
			found = true
			expectedDone := filepath.Join(taskDir, "done.txt")
			if dt.File != expectedDone {
				t.Errorf("expected file %q, got %q", expectedDone, dt.File)
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

func TestArchivePerDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	dir1 := filepath.Join(tmpDir, "tasks1")
	os.MkdirAll(dir1, 0755)
	os.WriteFile(filepath.Join(dir1, "todo.txt"), []byte(
		"Pending task\nx 2026-02-01 Completed task\n",
	), 0644)

	taskDirs := []scanner.TaskDirInfo{
		{DirPath: dir1, Files: []string{"todo.txt"}},
	}

	svc, err := NewTaskService(taskDirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = svc.Archive()
	if err != nil {
		t.Fatalf("archive error: %v", err)
	}

	// done.txt should now exist in dir1
	doneFile := filepath.Join(dir1, "done.txt")
	if _, err := os.Stat(doneFile); err != nil {
		t.Error("expected done.txt to be created after archive")
	}

	// Reload and verify
	tasks, _ := svc.ListPending()
	done, _ := svc.ListDone()

	if len(tasks) != 1 {
		t.Errorf("expected 1 pending task, got %d", len(tasks))
	}
	if len(done) != 1 {
		t.Errorf("expected 1 done task, got %d", len(done))
	}
}
