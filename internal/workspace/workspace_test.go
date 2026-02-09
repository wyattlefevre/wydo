package workspace

import (
	"os"
	"path/filepath"
	"strings"
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

func TestRenameProject_Virtual(t *testing.T) {
	// Create a temp workspace with tasks referencing a virtual project (no directory)
	tmp := t.TempDir()
	tasksDir := filepath.Join(tmp, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}
	todoFile := filepath.Join(tasksDir, "todo.txt")
	if err := os.WriteFile(todoFile, []byte("Buy milk +virtualproj\nDo stuff +virtualproj @home\n"), 0644); err != nil {
		t.Fatal(err)
	}

	scan, err := scanner.ScanWorkspace(tmp)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	ws, err := Load(scan)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	// Verify virtual project exists
	proj := ws.Projects.Get("virtualproj")
	if proj == nil {
		t.Fatal("expected virtualproj in registry")
	}
	if proj.DirPath != "" {
		t.Errorf("expected virtual project (empty DirPath), got %q", proj.DirPath)
	}

	// Verify tasks have the project
	matchCount := 0
	for _, task := range ws.Tasks {
		if task.HasProject("virtualproj") {
			matchCount++
		}
	}
	if matchCount != 2 {
		t.Fatalf("expected 2 tasks with +virtualproj, got %d", matchCount)
	}

	// Rename virtual project
	if err := ws.RenameProject("virtualproj", "renamedproj"); err != nil {
		t.Fatalf("RenameProject error: %v", err)
	}

	// Check tasks in memory
	for _, task := range ws.Tasks {
		if task.HasProject("virtualproj") {
			t.Errorf("task %q still has old project virtualproj", task.Name)
		}
	}

	// Check file on disk
	content, err := os.ReadFile(todoFile)
	if err != nil {
		t.Fatalf("read todo.txt: %v", err)
	}
	if strings.Contains(string(content), "+virtualproj") {
		t.Errorf("todo.txt still contains +virtualproj:\n%s", content)
	}
	if !strings.Contains(string(content), "+renamedproj") {
		t.Errorf("todo.txt does not contain +renamedproj:\n%s", content)
	}
	t.Logf("todo.txt after rename:\n%s", content)
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

func TestMergeProject_VirtualProjects(t *testing.T) {
	// Two virtual projects (no directories) — merging consolidates task +tags
	tmp := t.TempDir()
	tasksDir := filepath.Join(tmp, "tasks")
	os.MkdirAll(tasksDir, 0755)
	todoFile := filepath.Join(tasksDir, "todo.txt")
	os.WriteFile(todoFile, []byte("Task A +alpha\nTask B +beta\nTask C +alpha +beta\n"), 0644)

	scan, _ := scanner.ScanWorkspace(tmp)
	ws, _ := Load(scan)

	if err := ws.RenameProject("alpha", "beta"); err != nil {
		t.Fatalf("merge error: %v", err)
	}

	// All tasks should now reference beta, none should reference alpha
	for _, task := range ws.Tasks {
		if task.HasProject("alpha") {
			t.Errorf("task %q still has +alpha after merge", task.Name)
		}
	}

	// Check file on disk
	content, _ := os.ReadFile(todoFile)
	if strings.Contains(string(content), "+alpha") {
		t.Errorf("todo.txt still contains +alpha:\n%s", content)
	}
	if !strings.Contains(string(content), "+beta") {
		t.Errorf("todo.txt missing +beta:\n%s", content)
	}

	// Task C should have only one +beta, not +beta +beta
	for _, task := range ws.Tasks {
		if task.Name == "Task C" {
			count := 0
			for _, p := range task.Projects {
				if p == "beta" {
					count++
				}
			}
			if count != 1 {
				t.Errorf("Task C should have exactly 1 +beta reference, got %d", count)
			}
		}
	}
}

func TestMergeProject_BothPhysical(t *testing.T) {
	// Both source and target have directories — contents should be merged
	tmp := t.TempDir()

	// Create projects/ dir with alpha and beta
	projDir := filepath.Join(tmp, "projects")
	alphaDir := filepath.Join(projDir, "alpha")
	betaDir := filepath.Join(projDir, "beta")
	os.MkdirAll(alphaDir, 0755)
	os.MkdirAll(betaDir, 0755)

	// Alpha has file1.txt and notes.txt
	os.WriteFile(filepath.Join(alphaDir, "file1.txt"), []byte("alpha file1\n"), 0644)
	os.WriteFile(filepath.Join(alphaDir, "notes.txt"), []byte("alpha notes\n"), 0644)

	// Beta has file2.txt and notes.txt
	os.WriteFile(filepath.Join(betaDir, "file2.txt"), []byte("beta file2\n"), 0644)
	os.WriteFile(filepath.Join(betaDir, "notes.txt"), []byte("beta notes\n"), 0644)

	// Create tasks referencing both
	tasksDir := filepath.Join(tmp, "tasks")
	os.MkdirAll(tasksDir, 0755)
	os.WriteFile(filepath.Join(tasksDir, "todo.txt"), []byte("Task 1 +alpha\nTask 2 +beta\n"), 0644)

	scan, _ := scanner.ScanWorkspace(tmp)
	ws, _ := Load(scan)

	if err := ws.RenameProject("alpha", "beta"); err != nil {
		t.Fatalf("merge error: %v", err)
	}

	// Alpha dir should no longer exist
	if _, err := os.Stat(alphaDir); !os.IsNotExist(err) {
		t.Error("alpha directory should have been removed after merge")
	}

	// Beta dir should contain file1.txt, file2.txt, and notes.txt
	if _, err := os.Stat(filepath.Join(betaDir, "file1.txt")); err != nil {
		t.Error("beta should contain file1.txt from alpha")
	}
	if _, err := os.Stat(filepath.Join(betaDir, "file2.txt")); err != nil {
		t.Error("beta should still contain file2.txt")
	}

	// notes.txt should have appended content (both are .txt)
	notesContent, _ := os.ReadFile(filepath.Join(betaDir, "notes.txt"))
	if !strings.Contains(string(notesContent), "beta notes") {
		t.Error("notes.txt should still contain beta notes")
	}
	if !strings.Contains(string(notesContent), "alpha notes") {
		t.Error("notes.txt should contain appended alpha notes")
	}
}

func TestMergeProject_OnlySourceHasDir(t *testing.T) {
	// Source has a directory, target is virtual — directory gets renamed
	tmp := t.TempDir()

	projDir := filepath.Join(tmp, "projects")
	alphaDir := filepath.Join(projDir, "alpha")
	os.MkdirAll(alphaDir, 0755)
	os.WriteFile(filepath.Join(alphaDir, "readme.txt"), []byte("hello\n"), 0644)

	// Create tasks: one for alpha, one for beta (virtual)
	tasksDir := filepath.Join(tmp, "tasks")
	os.MkdirAll(tasksDir, 0755)
	os.WriteFile(filepath.Join(tasksDir, "todo.txt"), []byte("Task 1 +alpha\nTask 2 +beta\n"), 0644)

	scan, _ := scanner.ScanWorkspace(tmp)
	ws, _ := Load(scan)

	if err := ws.RenameProject("alpha", "beta"); err != nil {
		t.Fatalf("merge error: %v", err)
	}

	// Alpha dir should be gone, beta dir should exist
	if _, err := os.Stat(alphaDir); !os.IsNotExist(err) {
		t.Error("alpha directory should be gone")
	}
	betaDir := filepath.Join(projDir, "beta")
	if _, err := os.Stat(filepath.Join(betaDir, "readme.txt")); err != nil {
		t.Error("beta directory should contain readme.txt from alpha")
	}
}

func TestMergeProject_CardDedup(t *testing.T) {
	// A card with projects: [alpha, beta] should become projects: [beta] after merge
	tmp := t.TempDir()

	// Create a board in boards/<name>/ with board.md and cards/
	boardDir := filepath.Join(tmp, "boards", "testboard")
	cardsDir := filepath.Join(boardDir, "cards")
	os.MkdirAll(cardsDir, 0755)

	// board.md — cards are referenced via markdown links
	os.WriteFile(filepath.Join(boardDir, "board.md"), []byte("# Test Board\n\n## Todo\n- [Test Card](cards/test-card.md)\n"), 0644)

	// Card with both projects
	os.WriteFile(filepath.Join(cardsDir, "test-card.md"), []byte("---\nprojects:\n  - alpha\n  - beta\n---\n# Test Card\nContent here\n"), 0644)

	// Tasks referencing alpha
	tasksDir := filepath.Join(tmp, "tasks")
	os.MkdirAll(tasksDir, 0755)
	os.WriteFile(filepath.Join(tasksDir, "todo.txt"), []byte("Task 1 +alpha\n"), 0644)

	scan, _ := scanner.ScanWorkspace(tmp)
	ws, _ := Load(scan)

	if err := ws.RenameProject("alpha", "beta"); err != nil {
		t.Fatalf("merge error: %v", err)
	}

	// Find the card and check projects
	found := false
	for _, board := range ws.Boards {
		for _, col := range board.Columns {
			for _, card := range col.Cards {
				if card.Title == "Test Card" {
					found = true
					for _, p := range card.Projects {
						if strings.EqualFold(p, "alpha") {
							t.Errorf("card still has alpha in projects: %v", card.Projects)
						}
					}
					betaCount := 0
					for _, p := range card.Projects {
						if strings.EqualFold(p, "beta") {
							betaCount++
						}
					}
					if betaCount != 1 {
						t.Errorf("expected exactly 1 beta in card projects, got %d: %v", betaCount, card.Projects)
					}
				}
			}
		}
	}
	if !found {
		t.Error("could not find Test Card in boards")
	}

	// Verify on disk too
	cardContent, _ := os.ReadFile(filepath.Join(cardsDir, "test-card.md"))
	cardStr := string(cardContent)
	if strings.Contains(cardStr, "alpha") {
		t.Errorf("card file still references alpha:\n%s", cardStr)
	}
}

func TestMergeDirs_RecursiveSubdirs(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")

	// Create overlapping subdirectory structures
	os.MkdirAll(filepath.Join(src, "sub", "deep"), 0755)
	os.MkdirAll(filepath.Join(dst, "sub"), 0755)

	os.WriteFile(filepath.Join(src, "sub", "deep", "file.txt"), []byte("from src\n"), 0644)
	os.WriteFile(filepath.Join(dst, "sub", "existing.txt"), []byte("from dst\n"), 0644)

	if err := mergeDirs(src, dst); err != nil {
		t.Fatalf("mergeDirs error: %v", err)
	}

	// src should be removed
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("src directory should be removed")
	}

	// dst should have both files
	if _, err := os.Stat(filepath.Join(dst, "sub", "deep", "file.txt")); err != nil {
		t.Error("expected deep/file.txt in dst")
	}
	if _, err := os.Stat(filepath.Join(dst, "sub", "existing.txt")); err != nil {
		t.Error("existing.txt should still be in dst")
	}
}
