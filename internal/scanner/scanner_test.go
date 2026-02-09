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

	if len(scan.Boards) < 3 {
		t.Fatalf("expected at least 3 boards (dev-work, home-reno, sprint), got %d", len(scan.Boards))
	}

	boardNames := make(map[string]bool)
	for _, b := range scan.Boards {
		boardNames[filepath.Base(b.Path)] = true
	}

	for _, expected := range []string{"dev-work", "home-reno", "sprint"} {
		if !boardNames[expected] {
			t.Errorf("expected board %q not found", expected)
		}
	}
}

func TestScanWorkspace_FindsTaskDirs(t *testing.T) {
	scan, err := ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find tasks/ at root, projects/alpha/tasks/, projects/home-remodel/tasks/
	if len(scan.TaskDirs) < 3 {
		t.Fatalf("expected at least 3 task dirs, got %d", len(scan.TaskDirs))
	}

	foundRoot := false
	foundAlpha := false
	foundHomeRemodel := false
	for _, td := range scan.TaskDirs {
		if td.Project == "" && len(td.Files) >= 2 {
			foundRoot = true
		}
		if td.Project == "alpha" {
			foundAlpha = true
		}
		if td.Project == "home-remodel" {
			foundHomeRemodel = true
		}
	}

	if !foundRoot {
		t.Error("expected root tasks/ dir not found")
	}
	if !foundAlpha {
		t.Error("expected alpha project tasks/ dir not found")
	}
	if !foundHomeRemodel {
		t.Error("expected home-remodel project tasks/ dir not found")
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

	// Sprint board should have project context "alpha"
	for _, b := range scan.Boards {
		if filepath.Base(b.Path) == "sprint" {
			if b.Project != "alpha" {
				t.Errorf("expected sprint board project=alpha, got %q", b.Project)
			}
			return
		}
	}
	t.Error("sprint board not found")
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

	// workspace2 has tasks/, projects/alpha/tasks/, and a note
	if len(scan.TaskDirs) < 2 {
		t.Fatalf("expected at least 2 task dirs in workspace2, got %d", len(scan.TaskDirs))
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
	for _, td := range scan.TaskDirs {
		if td.Project == "" {
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
			return
		}
	}
	t.Error("root tasks dir not found")
}
