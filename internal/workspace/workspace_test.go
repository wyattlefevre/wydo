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
		t.Errorf("expected at least 3 boards (dev-work, home-reno, sprint), got %d", len(ws.Boards))
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

	registry := BuildProjectRegistry(scan, nil, nil, scan.RootDir)

	alpha := registry.Get("alpha")
	if alpha == nil {
		t.Fatal("expected alpha project")
	}
	if alpha.FilePath == "" {
		t.Error("expected alpha to have a FilePath from projects/ directory")
	}

	homeRemodel := registry.Get("home-remodel")
	if homeRemodel == nil {
		t.Fatal("expected home-remodel project")
	}
	if homeRemodel.FilePath == "" {
		t.Error("expected home-remodel to have a FilePath from projects/ directory")
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
	if alpha.FilePath == "" {
		t.Error("alpha should have FilePath from directory")
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

	alphaCards := ws.Projects.TaskNotesForProject("alpha", ws.Boards)
	if len(alphaCards) == 0 {
		t.Error("expected alpha cards")
	}

	// auth-service and db-migration are in root-level dev-work board
	// rate-limiting is in root-level sprint board (sprint moved from projects/alpha/boards/ to boards/)
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
	// Create tasks/ dir so scanner registers it; todo.txt lives at workspace root
	if err := os.MkdirAll(filepath.Join(tmp, "tasks"), 0755); err != nil {
		t.Fatal(err)
	}
	todoFile := filepath.Join(tmp, "todo.txt")
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
	if proj.FilePath != "" {
		t.Errorf("expected virtual project (empty FilePath), got %q", proj.FilePath)
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
	if alpha1.FilePath == alpha2.FilePath {
		t.Error("alpha projects in different workspaces should have different FilePaths")
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
	os.MkdirAll(filepath.Join(tmp, "tasks"), 0755)
	todoFile := filepath.Join(tmp, "todo.txt")
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
	// Both source and target have flat .md files — source content appended to target, source removed
	tmp := t.TempDir()

	// Create projects/ dir with alpha.md and beta.md
	projDir := filepath.Join(tmp, "projects")
	os.MkdirAll(projDir, 0755)
	alphaFile := filepath.Join(projDir, "alpha.md")
	betaFile := filepath.Join(projDir, "beta.md")

	os.WriteFile(alphaFile, []byte("# alpha\nalpha content\n"), 0644)
	os.WriteFile(betaFile, []byte("# beta\nbeta content\n"), 0644)

	// Create tasks referencing both
	os.MkdirAll(filepath.Join(tmp, "tasks"), 0755)
	os.WriteFile(filepath.Join(tmp, "todo.txt"), []byte("Task 1 +alpha\nTask 2 +beta\n"), 0644)

	scan, _ := scanner.ScanWorkspace(tmp)
	ws, _ := Load(scan)

	if err := ws.RenameProject("alpha", "beta"); err != nil {
		t.Fatalf("merge error: %v", err)
	}

	// Alpha file should no longer exist
	if _, err := os.Stat(alphaFile); !os.IsNotExist(err) {
		t.Error("alpha.md should have been removed after merge")
	}

	// Beta file should still exist with merged content
	betaContent, err := os.ReadFile(betaFile)
	if err != nil {
		t.Fatal("beta.md should still exist after merge")
	}
	if !strings.Contains(string(betaContent), "beta content") {
		t.Error("beta.md should still contain beta content")
	}
	if !strings.Contains(string(betaContent), "alpha content") {
		t.Error("beta.md should contain appended alpha content")
	}
}

func TestMergeProject_OnlySourceHasDir(t *testing.T) {
	// Source has a flat .md file, target is virtual — file gets renamed
	tmp := t.TempDir()

	projDir := filepath.Join(tmp, "projects")
	os.MkdirAll(projDir, 0755)
	alphaFile := filepath.Join(projDir, "alpha.md")
	os.WriteFile(alphaFile, []byte("# alpha\nhello\n"), 0644)

	// Create tasks: one for alpha, one for beta (virtual)
	os.MkdirAll(filepath.Join(tmp, "tasks"), 0755)
	os.WriteFile(filepath.Join(tmp, "todo.txt"), []byte("Task 1 +alpha\nTask 2 +beta\n"), 0644)

	scan, _ := scanner.ScanWorkspace(tmp)
	ws, _ := Load(scan)

	if err := ws.RenameProject("alpha", "beta"); err != nil {
		t.Fatalf("merge error: %v", err)
	}

	// Alpha file should be gone, beta file should exist
	if _, err := os.Stat(alphaFile); !os.IsNotExist(err) {
		t.Error("alpha.md should be gone after rename")
	}
	betaFile := filepath.Join(projDir, "beta.md")
	if _, err := os.Stat(betaFile); err != nil {
		t.Error("beta.md should exist after rename")
	}
}

func TestRenameProject_FileRenamed(t *testing.T) {
	// When a flat project file is renamed, alpha.md → beta.md
	tmp := t.TempDir()

	projDir := filepath.Join(tmp, "projects")
	os.MkdirAll(projDir, 0755)
	alphaFile := filepath.Join(projDir, "alpha.md")
	os.WriteFile(alphaFile, []byte("# alpha\ncontent\n"), 0644)

	os.MkdirAll(filepath.Join(tmp, "tasks"), 0755)
	os.WriteFile(filepath.Join(tmp, "todo.txt"), []byte("Task 1 +alpha\n"), 0644)

	scan, _ := scanner.ScanWorkspace(tmp)
	ws, _ := Load(scan)

	if err := ws.RenameProject("alpha", "beta"); err != nil {
		t.Fatalf("rename error: %v", err)
	}

	// Old file should not exist
	if _, err := os.Stat(alphaFile); !os.IsNotExist(err) {
		t.Error("alpha.md should have been renamed")
	}

	// New file should exist
	betaFile := filepath.Join(projDir, "beta.md")
	if _, err := os.Stat(betaFile); err != nil {
		t.Error("beta.md should exist after rename")
	}
}

func TestMergeProject_CardDedup(t *testing.T) {
	// A card with projects: [alpha, beta] should become projects: [beta] after merge
	tmp := t.TempDir()

	// Create a board .txt file
	boardsDir := filepath.Join(tmp, "boards")
	os.MkdirAll(boardsDir, 0755)
	os.WriteFile(filepath.Join(boardsDir, "testboard.txt"), []byte("Todo\n"), 0644)

	// Card in tasks/ with both projects
	tasksDir := filepath.Join(tmp, "tasks")
	os.MkdirAll(tasksDir, 0755)
	cardFile := filepath.Join(tasksDir, "test-card.md")
	os.WriteFile(cardFile, []byte("---\nprojects:\n  - alpha\n  - beta\nboard: testboard\nstatus: Todo\n---\n# Test Card\nContent here\n"), 0644)

	// Tasks referencing alpha (todo.txt at workspace root)
	os.WriteFile(filepath.Join(tmp, "todo.txt"), []byte("Task 1 +alpha\n"), 0644)

	scan, _ := scanner.ScanWorkspace(tmp)
	ws, _ := Load(scan)

	if err := ws.RenameProject("alpha", "beta"); err != nil {
		t.Fatalf("merge error: %v", err)
	}

	// Find the card and check projects
	found := false
	for _, board := range ws.Boards {
		for _, col := range board.Columns {
			for _, card := range col.TaskNotes {
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
	cardContent, _ := os.ReadFile(cardFile)
	cardStr := string(cardContent)
	if strings.Contains(cardStr, "alpha") {
		t.Errorf("card file still references alpha:\n%s", cardStr)
	}
}

func TestSetVirtualProjectArchived(t *testing.T) {
	tmp := t.TempDir()

	p := &Project{Name: "myproject"}

	// Archive — file should be created
	if err := SetVirtualProjectArchived(tmp, p, true); err != nil {
		t.Fatalf("archive error: %v", err)
	}
	if !p.Archived {
		t.Error("expected p.Archived to be true")
	}
	archived := readVirtualArchive(tmp)
	if !archived["myproject"] {
		t.Error("expected myproject in virtual archive")
	}

	// Unarchive — file should be removed (no more entries)
	if err := SetVirtualProjectArchived(tmp, p, false); err != nil {
		t.Fatalf("unarchive error: %v", err)
	}
	if p.Archived {
		t.Error("expected p.Archived to be false")
	}
	if _, err := os.Stat(virtualArchivePath(tmp)); !os.IsNotExist(err) {
		t.Error("expected archive file to be removed when empty")
	}
}

func TestBuildProjectRegistry_LoadsVirtualArchive(t *testing.T) {
	tmp := t.TempDir()

	// Create a task referencing "virtualproj"
	os.MkdirAll(filepath.Join(tmp, "tasks"), 0755)
	os.WriteFile(filepath.Join(tmp, "todo.txt"), []byte("Do something +virtualproj\n"), 0644)

	// Create a physical project "physicalproj" as a flat .md file
	projDir := filepath.Join(tmp, "projects")
	os.MkdirAll(projDir, 0755)
	os.WriteFile(filepath.Join(projDir, "physicalproj.md"), []byte("# physicalproj\n"), 0644)

	// Write archive file listing both — physical project entry should be ignored
	archiveContent := "physicalproj\nvirtualproj\n"
	os.WriteFile(virtualArchivePath(tmp), []byte(archiveContent), 0644)

	scan, err := scanner.ScanWorkspace(tmp)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	ws, err := Load(scan)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	vp := ws.Projects.Get("virtualproj")
	if vp == nil {
		t.Fatal("expected virtualproj in registry")
	}
	if !vp.Archived {
		t.Error("expected virtualproj to be marked archived")
	}

	// Physical project should NOT be affected by virtual archive
	pp := ws.Projects.Get("physicalproj")
	if pp == nil {
		t.Fatal("expected physicalproj in registry")
	}
	if pp.Archived {
		t.Error("physical project should not be archived via virtual archive file")
	}
}

func TestDeleteVirtualProject(t *testing.T) {
	tmp := t.TempDir()

	os.MkdirAll(filepath.Join(tmp, "tasks"), 0755)
	todoFile := filepath.Join(tmp, "todo.txt")
	os.WriteFile(todoFile, []byte("Buy milk +foo +bar\nDo stuff +foo\nOther task +bar\n"), 0644)

	scan, err := scanner.ScanWorkspace(tmp)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	ws, err := Load(scan)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Verify foo is virtual
	foo := ws.Projects.Get("foo")
	if foo == nil {
		t.Fatal("expected foo in registry")
	}
	if foo.FilePath != "" {
		t.Errorf("expected foo to be virtual")
	}

	if err := DeleteVirtualProject(ws, "foo"); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	// +foo should be gone from tasks
	content, _ := os.ReadFile(todoFile)
	if strings.Contains(string(content), "+foo") {
		t.Errorf("todo.txt still contains +foo:\n%s", content)
	}
	// +bar should be preserved
	if !strings.Contains(string(content), "+bar") {
		t.Errorf("todo.txt should still contain +bar:\n%s", content)
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
