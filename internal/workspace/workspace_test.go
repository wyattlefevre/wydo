package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"wydo/internal/scanner"
)

func testdataDir() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..", "testdata")
}

func TestLoad_IntegrationWithFixtures(t *testing.T) {
	scan, err := scanner.ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	ws, err := Load(scan)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if ws.RootDir == "" {
		t.Error("expected non-empty RootDir")
	}

	if len(ws.Boards) < 3 {
		t.Errorf("expected at least 3 boards, got %d", len(ws.Boards))
	}

	if len(ws.Tasks) == 0 {
		t.Error("expected tasks to be loaded")
	}

	if len(ws.Notes) < 2 {
		t.Errorf("expected at least 2 notes, got %d", len(ws.Notes))
	}

	if ws.Projects == nil {
		t.Fatal("expected non-nil project registry")
	}

	if ws.TaskSvc == nil {
		t.Fatal("expected non-nil task service")
	}
}

func TestProjectRegistry_FromDirectories(t *testing.T) {
	scan, err := scanner.ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	registry := BuildProjectRegistry(scan, nil, nil)

	alpha := registry.Get("alpha")
	if alpha == nil {
		t.Fatal("expected alpha project")
	}
	if alpha.DirPath == "" {
		t.Error("expected alpha to have a DirPath from projects/ directory")
	}

	homeRemodel := registry.Get("home-remodel")
	if homeRemodel == nil {
		t.Fatal("expected home-remodel project")
	}
	if homeRemodel.DirPath == "" {
		t.Error("expected home-remodel to have a DirPath from projects/ directory")
	}
}

func TestProjectRegistry_FromTaskTags(t *testing.T) {
	scan, _ := scanner.ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	ws, _ := Load(scan)

	// Tasks have +alpha and +home-remodel tags
	projs := ws.Projects.List()
	if len(projs) == 0 {
		t.Error("expected projects to be discovered from task tags")
	}
}

func TestProjectRegistry_FromCardFrontmatter(t *testing.T) {
	scan, _ := scanner.ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	ws, _ := Load(scan)

	// Cards have projects: [alpha] and projects: [home-remodel]
	alpha := ws.Projects.Get("alpha")
	if alpha == nil {
		t.Error("expected alpha project from card frontmatter")
	}

	homeRemodel := ws.Projects.Get("home-remodel")
	if homeRemodel == nil {
		t.Error("expected home-remodel project from card frontmatter")
	}
}

func TestProjectRegistry_MergesAllSources(t *testing.T) {
	scan, _ := scanner.ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	ws, _ := Load(scan)

	// Alpha appears in directories, tasks, AND cards - should be one project
	alpha := ws.Projects.Get("alpha")
	if alpha == nil {
		t.Fatal("expected alpha project")
	}
	if alpha.DirPath == "" {
		t.Error("alpha should have DirPath from directory")
	}
}

func TestTasksForProject(t *testing.T) {
	scan, _ := scanner.ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	ws, _ := Load(scan)

	alphaTasks := ws.Projects.TasksForProject("alpha", ws.Tasks)
	if len(alphaTasks) == 0 {
		t.Error("expected alpha tasks")
	}

	for _, task := range alphaTasks {
		if !task.HasProject("alpha") {
			t.Errorf("task %q does not have project alpha", task.Name)
		}
	}
}

func TestCardsForProject(t *testing.T) {
	scan, _ := scanner.ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	ws, _ := Load(scan)

	alphaCards := ws.Projects.CardsForProject("alpha", ws.Boards)
	if len(alphaCards) == 0 {
		t.Error("expected alpha cards")
	}

	// auth-service and db-migration and rate-limiting should be linked to alpha
	cardNames := make(map[string]bool)
	for _, c := range alphaCards {
		cardNames[c.Title] = true
	}

	if !cardNames["Auth Service"] {
		t.Error("expected Auth Service card for alpha")
	}
	if !cardNames["DB Migration"] {
		t.Error("expected DB Migration card for alpha")
	}
	if !cardNames["Rate Limiting"] {
		t.Error("expected Rate Limiting card for alpha")
	}
}

func TestWorkspaceIsolation(t *testing.T) {
	scan1, _ := scanner.ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	ws1, _ := Load(scan1)

	scan2, _ := scanner.ScanWorkspace(filepath.Join(testdataDir(), "workspace2"))
	ws2, _ := Load(scan2)

	// Both have "alpha" project but they should be separate registries
	alpha1 := ws1.Projects.Get("alpha")
	alpha2 := ws2.Projects.Get("alpha")

	if alpha1 == nil {
		t.Fatal("expected alpha in workspace1")
	}
	if alpha2 == nil {
		t.Fatal("expected alpha in workspace2")
	}

	// Verify they point to different directories
	if alpha1.DirPath == alpha2.DirPath {
		t.Error("alpha projects in different workspaces should have different DirPaths")
	}

	// Verify tasks don't cross
	alpha1Tasks := ws1.Projects.TasksForProject("alpha", ws1.Tasks)
	alpha2Tasks := ws2.Projects.TasksForProject("alpha", ws2.Tasks)

	for _, t1 := range alpha1Tasks {
		for _, t2 := range alpha2Tasks {
			if t1.ID == t2.ID {
				t.Error("tasks should not be shared between workspaces")
			}
		}
	}
}
