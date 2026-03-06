package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func testdataDir() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..", "testdata")
}

func TestScanWorkspace_FindsBoards(t *testing.T) {
	scan, err := ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(scan.Boards) < 2 {
		t.Fatalf("expected at least 2 boards (dev-work, home-reno), got %d", len(scan.Boards))
	}

	boardNames := make(map[string]bool)
	for _, b := range scan.Boards {
		boardNames[filepath.Base(b.Path)] = true
	}

	for _, expected := range []string{"dev-work", "home-reno"} {
		if !boardNames[expected] {
			t.Errorf("expected board %q not found", expected)
		}
	}
	if boardNames["sprint"] {
		t.Error("sprint board (inside project) should not be discovered at workspace root")
	}
}

func TestScanWorkspace_FindsTaskDirs(t *testing.T) {
	scan, err := ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find only tasks/ at root (project-level tasks/ are no longer discovered)
	if len(scan.TaskDirs) != 1 {
		t.Fatalf("expected exactly 1 task dir (root only), got %d", len(scan.TaskDirs))
	}

	td := scan.TaskDirs[0]
	if len(td.Files) < 2 {
		t.Errorf("expected root tasks dir to have at least 2 files, got %d", len(td.Files))
	}
}

func TestScanWorkspace_FindsProjects(t *testing.T) {
	scan, err := ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(scan.Projects) < 2 {
		t.Fatalf("expected at least 2 projects, got %d", len(scan.Projects))
	}

	projNames := make(map[string]bool)
	for _, p := range scan.Projects {
		projNames[p.Name] = true
	}

	for _, expected := range []string{"alpha", "home-remodel"} {
		if !projNames[expected] {
			t.Errorf("expected project %q not found", expected)
		}
	}
}

func TestScanWorkspace_FindsNotes(t *testing.T) {
	scan, err := ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(scan.NotePaths) < 2 {
		t.Fatalf("expected at least 2 notes, got %d", len(scan.NotePaths))
	}

	noteNames := make(map[string]bool)
	for _, n := range scan.NotePaths {
		noteNames[filepath.Base(n)] = true
	}

	if !noteNames["2026-02-07-daily-standup.md"] {
		t.Error("expected daily-standup note not found")
	}
	if !noteNames["2026-02-14-design-review.md"] {
		t.Error("expected design-review note not found")
	}
}

func TestScanWorkspace_ProjectContext(t *testing.T) {
	scan, err := ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Sprint board is inside a project directory and should NOT be discovered
	for _, b := range scan.Boards {
		if filepath.Base(b.Path) == "sprint" {
			t.Error("sprint board (inside project) should not be discovered")
			return
		}
	}
	// Projects are still discovered
	if len(scan.Projects) < 2 {
		t.Errorf("expected at least 2 projects, got %d", len(scan.Projects))
	}
}

func TestScanWorkspace_SkipsHiddenDirs(t *testing.T) {
	// Create a temp workspace with a hidden directory
	tmpDir := t.TempDir()
	hiddenDir := filepath.Join(tmpDir, ".hidden", "tasks")
	os.MkdirAll(hiddenDir, 0755)
	os.WriteFile(filepath.Join(hiddenDir, "todo.txt"), []byte("hidden task"), 0644)

	// Also create a visible tasks dir
	visibleDir := filepath.Join(tmpDir, "tasks")
	os.MkdirAll(visibleDir, 0755)
	os.WriteFile(filepath.Join(visibleDir, "todo.txt"), []byte("visible task"), 0644)

	scan, err := ScanWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(scan.TaskDirs) != 1 {
		t.Fatalf("expected 1 task dir (hidden should be skipped), got %d", len(scan.TaskDirs))
	}
}

func TestScanWorkspace_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	scan, err := ScanWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(scan.Boards) != 0 {
		t.Errorf("expected 0 boards, got %d", len(scan.Boards))
	}
	if len(scan.TaskDirs) != 0 {
		t.Errorf("expected 0 task dirs, got %d", len(scan.TaskDirs))
	}
	if len(scan.Projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(scan.Projects))
	}
	if len(scan.NotePaths) != 0 {
		t.Errorf("expected 0 notes, got %d", len(scan.NotePaths))
	}
}

func TestScanWorkspace_Workspace2(t *testing.T) {
	scan, err := ScanWorkspace(filepath.Join(testdataDir(), "workspace2"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// workspace2 has tasks/ at root only (project-level tasks/ are no longer discovered)
	if len(scan.TaskDirs) != 1 {
		t.Fatalf("expected exactly 1 task dir (root only) in workspace2, got %d", len(scan.TaskDirs))
	}

	if len(scan.Projects) < 1 {
		t.Fatalf("expected at least 1 project in workspace2, got %d", len(scan.Projects))
	}

	if len(scan.NotePaths) < 1 {
		t.Fatalf("expected at least 1 note in workspace2, got %d", len(scan.NotePaths))
	}
}

func TestScanWorkspace_TaskDirFiles(t *testing.T) {
	scan, err := ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find root tasks dir and verify it has both todo.txt and done.txt
	if len(scan.TaskDirs) == 0 {
		t.Fatal("root tasks dir not found")
	}
	td := scan.TaskDirs[0]
	sort.Strings(td.Files)
	foundTodo := false
	foundDone := false
	for _, f := range td.Files {
		if f == "todo.txt" {
			foundTodo = true
		}
		if f == "done.txt" {
			foundDone = true
		}
	}
	if !foundTodo {
		t.Error("root tasks dir missing todo.txt")
	}
	if !foundDone {
		t.Error("root tasks dir missing done.txt")
	}
}

func TestScanWorkspace_BoardsOnlyAtRoot(t *testing.T) {
	tmp := t.TempDir()

	// Create a board at the workspace root boards/ (should be found)
	rootBoard := filepath.Join(tmp, "boards", "myboard")
	os.MkdirAll(rootBoard, 0755)
	os.WriteFile(filepath.Join(rootBoard, "board.md"), []byte("# myboard\n"), 0644)

	// Create a board inside a project (should NOT be found)
	projBoard := filepath.Join(tmp, "projects", "alpha", "boards", "sprint")
	os.MkdirAll(projBoard, 0755)
	os.WriteFile(filepath.Join(projBoard, "board.md"), []byte("# sprint\n"), 0644)

	scan, err := ScanWorkspace(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(scan.Boards) != 1 {
		t.Fatalf("expected exactly 1 board (root only), got %d", len(scan.Boards))
	}
	if filepath.Base(scan.Boards[0].Path) != "myboard" {
		t.Errorf("expected board 'myboard', got %q", filepath.Base(scan.Boards[0].Path))
	}
}

func TestScanWorkspace_TasksOnlyAtRoot(t *testing.T) {
	tmp := t.TempDir()

	// Root tasks dir (should be found)
	rootTasks := filepath.Join(tmp, "tasks")
	os.MkdirAll(rootTasks, 0755)
	os.WriteFile(filepath.Join(rootTasks, "todo.txt"), []byte("root task\n"), 0644)

	// Project tasks dir (should NOT be found)
	projTasks := filepath.Join(tmp, "projects", "alpha", "tasks")
	os.MkdirAll(projTasks, 0755)
	os.WriteFile(filepath.Join(projTasks, "todo.txt"), []byte("alpha task\n"), 0644)

	scan, err := ScanWorkspace(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(scan.TaskDirs) != 1 {
		t.Fatalf("expected exactly 1 task dir (root only), got %d", len(scan.TaskDirs))
	}
}
