# Board and Task Directory Update Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove multi-directory support for boards and tasks (single root-level `boards/` and `tasks/` per workspace), and add explicit board-to-project linking via `board.md` frontmatter.

**Architecture:** The scanner stops recursing into project directories for boards/tasks. Board model gains a `Project` string field (relative path from `board.md` to the project index file). `BoardsForProject` and `ProjectsForBoard` switch from directory-prefix matching to frontmatter path resolution. The scaffold UI loses its now-invalid `boards/` and `tasks/` directory options.

**Tech Stack:** Go, bubbletea TUI, goldmark markdown parser, gopkg.in/yaml.v3

---

### Task 1: Update scanner to only discover boards/ and tasks/ at workspace root

**Files:**
- Modify: `internal/scanner/scanner.go`

The scanner's `walkWorkspace` function currently calls `scanBoardsDir` and `scanTasksDir` at any directory depth. After this change it only does so at the workspace root.

Also remove the `Project` field from both `BoardInfo` and `TaskDirInfo` — it was only meaningful when these dirs could exist inside project subdirectories.

Remove the now-unused `projectContext` parameter from `scanBoardsDir` and `scanTasksDir`.

**Step 1: Write a failing test that verifies boards inside project dirs are NOT discovered**

In `internal/scanner/scanner_test.go`, add at the end:

```go
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
```

**Step 2: Run the new tests to verify they fail**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && go test ./internal/scanner/ -run "TestScanWorkspace_BoardsOnlyAtRoot|TestScanWorkspace_TasksOnlyAtRoot" -v
```
Expected: FAIL — both boards and project boards/tasks are currently discovered.

**Step 3: Update scanner.go**

Replace `BoardInfo` and `TaskDirInfo` struct definitions and update `walkWorkspace`:

In `BoardInfo`, remove the `Project string` field:
```go
type BoardInfo struct {
	Path string // absolute path to board dir (containing board.md)
}
```

In `TaskDirInfo`, remove the `Project string` field:
```go
type TaskDirInfo struct {
	DirPath string   // absolute path to the tasks/ directory
	Files   []string // .txt filenames found within
}
```

In `walkWorkspace`, add a `dir == rootDir` guard for both `boards` and `tasks` cases:
```go
case "boards":
	if dir == rootDir {
		if err := scanBoardsDir(absPath, scan); err != nil {
			return err
		}
	}
case "tasks":
	if dir == rootDir {
		if err := scanTasksDir(absPath, scan); err != nil {
			return err
		}
	}
```

Remove `projectContext` from `scanBoardsDir` and `scanTasksDir` signatures, and remove it from the `BoardInfo` and `TaskDirInfo` construction inside those functions:

```go
func scanBoardsDir(boardsDir string, scan *WorkspaceScan) error {
	entries, err := os.ReadDir(boardsDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		boardPath := filepath.Join(boardsDir, entry.Name())
		boardFile := filepath.Join(boardPath, "board.md")
		if _, err := os.Stat(boardFile); err == nil {
			scan.Boards = append(scan.Boards, BoardInfo{
				Path: boardPath,
			})
		}
	}
	return nil
}

func scanTasksDir(tasksDir string, scan *WorkspaceScan) error {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".txt") {
			files = append(files, entry.Name())
		}
	}
	if len(files) > 0 {
		scan.TaskDirs = append(scan.TaskDirs, TaskDirInfo{
			DirPath: tasksDir,
			Files:   files,
		})
	}
	return nil
}
```

**Step 4: Run the new tests to verify they pass**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && go test ./internal/scanner/ -run "TestScanWorkspace_BoardsOnlyAtRoot|TestScanWorkspace_TasksOnlyAtRoot" -v
```
Expected: PASS

**Step 5: Fix compilation errors from removed fields**

The removed `Project` field will cause compile errors in `workspace.go` (`BuildProjectRegistry` uses `bi.Project`) and `task_service.go` (uses `TaskDirInfo`). Check:

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && go build ./...
```

In `internal/workspace/workspace.go`, `BuildProjectRegistry` iterates `scan.Boards` and passes `bi.Project` to `ensureProject`. Remove that call — boards no longer contribute project context through directory structure:

```go
// 1. From directory structure
for _, pi := range scan.Projects {
	r.ensureProject(pi.Name, pi.Path, pi.Parent)
}
```
(Delete the boards loop that called `r.ensureProject(bi.Project, ...)`)

Check for any other references to `BoardInfo.Project` or `TaskDirInfo.Project`:
```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && grep -rn "\.Project\b" internal/scanner/ internal/workspace/ internal/tui/ main.go
```
Fix any remaining references.

**Step 6: Verify compilation succeeds**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && go build ./...
```
Expected: no errors.

**Step 7: Commit**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && git add -p && git commit -m "$(cat <<'EOF'
Remove multi-directory support for boards and tasks in scanner

Only boards/ and tasks/ at the workspace root are now discovered.
Remove Project field from BoardInfo and TaskDirInfo.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Update testdata and fix broken scanner tests

**Files:**
- Move: `testdata/workspace1/projects/alpha/boards/sprint/` → `testdata/workspace1/boards/sprint/`
- Delete: `testdata/workspace1/projects/alpha/tasks/todo.txt`
- Delete: `testdata/workspace1/projects/home-remodel/tasks/todo.txt`
- Delete: `testdata/workspace2/projects/alpha/tasks/todo.txt`
- Modify: `internal/scanner/scanner_test.go`

After Task 1, the following existing scanner tests will fail because the testdata no longer matches their expectations:
- `TestScanWorkspace_FindsBoards` — still passes (sprint moves to root, still 3 boards)
- `TestScanWorkspace_FindsTaskDirs` — fails (expects 3 dirs, now only 1)
- `TestScanWorkspace_ProjectContext` — fails (references `b.Project` field that no longer exists)
- `TestScanWorkspace_Workspace2` — fails (expects 2 task dirs, now 1)

**Step 1: Move the sprint board in testdata**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && mkdir -p testdata/workspace1/boards/sprint/cards && cp testdata/workspace1/projects/alpha/boards/sprint/board.md testdata/workspace1/boards/sprint/board.md && cp testdata/workspace1/projects/alpha/boards/sprint/cards/rate-limiting.md testdata/workspace1/boards/sprint/cards/rate-limiting.md && rm -rf testdata/workspace1/projects/alpha/boards
```

**Step 2: Remove project-specific task directories from testdata**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && rm testdata/workspace1/projects/alpha/tasks/todo.txt && rmdir testdata/workspace1/projects/alpha/tasks && rm testdata/workspace1/projects/home-remodel/tasks/todo.txt && rmdir testdata/workspace1/projects/home-remodel/tasks && rm testdata/workspace2/projects/alpha/tasks/todo.txt && rmdir testdata/workspace2/projects/alpha/tasks
```

**Step 3: Update scanner_test.go**

Delete `TestScanWorkspace_ProjectContext` entirely (the concept is gone — boards no longer carry project context from directory location).

Update `TestScanWorkspace_FindsTaskDirs`:
```go
func TestScanWorkspace_FindsTaskDirs(t *testing.T) {
	scan, err := ScanWorkspace(filepath.Join(testdataDir(), "workspace1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find only the root tasks/ directory
	if len(scan.TaskDirs) != 1 {
		t.Fatalf("expected exactly 1 task dir (root only), got %d", len(scan.TaskDirs))
	}

	td := scan.TaskDirs[0]
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
```

(Also delete `TestScanWorkspace_TaskDirFiles` — its content is now covered by the updated `TestScanWorkspace_FindsTaskDirs` above.)

Update `TestScanWorkspace_Workspace2`:
```go
func TestScanWorkspace_Workspace2(t *testing.T) {
	scan, err := ScanWorkspace(filepath.Join(testdataDir(), "workspace2"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// workspace2 has tasks/ at root and a note
	if len(scan.TaskDirs) != 1 {
		t.Fatalf("expected exactly 1 task dir in workspace2, got %d", len(scan.TaskDirs))
	}

	if len(scan.Projects) < 1 {
		t.Fatalf("expected at least 1 project in workspace2, got %d", len(scan.Projects))
	}

	if len(scan.NotePaths) < 1 {
		t.Fatalf("expected at least 1 note in workspace2, got %d", len(scan.NotePaths))
	}
}
```

**Step 4: Run the full scanner test suite**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && go test ./internal/scanner/ -v
```
Expected: all tests PASS.

**Step 5: Run the full test suite to check for regressions**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && go test ./...
```
Expected: all tests PASS (workspace tests expect ≥3 boards — sprint is now at root, so this still holds).

**Step 6: Commit**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && git add testdata/ internal/scanner/scanner_test.go && git commit -m "$(cat <<'EOF'
Update testdata and scanner tests for root-only boards/tasks

Move sprint board to workspace root. Remove project-level task dirs.
Remove ProjectContext test; update FindsTaskDirs and Workspace2 tests.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Add Project field to Board model, reader, and writer

**Files:**
- Modify: `internal/kanban/models/board.go`
- Modify: `internal/kanban/fs/board_reader.go`
- Modify: `internal/kanban/fs/board_writer.go`
- Modify: `internal/kanban/fs/board_reader_test.go`

**Step 1: Write failing tests**

Add to `internal/kanban/fs/board_reader_test.go`:

```go
func TestReadBoard_ProjectFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	boardPath := filepath.Join(tmp, "sprint")
	os.MkdirAll(boardPath, 0755)
	os.WriteFile(filepath.Join(boardPath, "board.md"), []byte(
		"---\nproject: ../../projects/alpha/alpha.md\n---\n\n# sprint\n\n## Backlog\n\n## Done\n",
	), 0644)

	board, err := ReadBoard(boardPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if board.Project != "../../projects/alpha/alpha.md" {
		t.Errorf("expected project path '../../projects/alpha/alpha.md', got %q", board.Project)
	}
}

func TestWriteBoard_ProjectFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	boardPath := filepath.Join(tmp, "sprint")
	os.MkdirAll(boardPath, 0755)

	board := models.Board{
		Path:    boardPath,
		Name:    "sprint",
		Project: "../../projects/alpha/alpha.md",
		Columns: []models.Column{{Name: "Backlog", Cards: []models.Card{}}, {Name: "Done", Cards: []models.Card{}}},
	}
	if err := WriteBoard(board); err != nil {
		t.Fatalf("write error: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(boardPath, "board.md"))
	if !strings.Contains(string(content), "project: ../../projects/alpha/alpha.md") {
		t.Errorf("expected project frontmatter in board.md, got:\n%s", content)
	}

	// Round-trip: read back and verify
	loaded, err := ReadBoard(boardPath)
	if err != nil {
		t.Fatalf("read-back error: %v", err)
	}
	if loaded.Project != board.Project {
		t.Errorf("expected project %q after round-trip, got %q", board.Project, loaded.Project)
	}
}
```

Add `"strings"` to the imports in `board_reader_test.go` if not already present.

**Step 2: Run new tests to verify they fail**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && go test ./internal/kanban/fs/ -run "TestReadBoard_ProjectFrontmatter|TestWriteBoard_ProjectFrontmatter" -v
```
Expected: FAIL — `Board` has no `Project` field yet.

**Step 3: Add Project field to Board model**

In `internal/kanban/models/board.go`, add the field after `JiraBoardID`:
```go
type Board struct {
	Name        string
	Path        string
	Columns     []Column
	Archived    bool
	JiraBoardID int
	Project     string // relative path from board.md to the linked project index file, or ""
}
```

**Step 4: Update board_reader.go to parse the project field**

Update `stripBoardFrontmatter` to return a fourth value and parse `project`:

```go
func stripBoardFrontmatter(content []byte) ([]byte, bool, int, string) {
	lines := bytes.Split(content, []byte("\n"))
	if len(lines) == 0 || !bytes.Equal(bytes.TrimSpace(lines[0]), []byte("---")) {
		return content, false, 0, ""
	}

	var frontmatterEnd int
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(bytes.TrimSpace(lines[i]), []byte("---")) {
			frontmatterEnd = i
			break
		}
	}

	if frontmatterEnd == 0 {
		return content, false, 0, ""
	}

	frontmatterBytes := bytes.Join(lines[1:frontmatterEnd], []byte("\n"))
	var fm struct {
		Archived    bool   `yaml:"archived"`
		JiraBoardID int    `yaml:"jira_board_id"`
		Project     string `yaml:"project"`
	}
	if err := yaml.Unmarshal(frontmatterBytes, &fm); err != nil {
		return content, false, 0, ""
	}

	body := bytes.TrimLeft(bytes.Join(lines[frontmatterEnd+1:], []byte("\n")), "\n")
	return body, fm.Archived, fm.JiraBoardID, fm.Project
}
```

Update `ReadBoard` to use the new return value:
```go
body, archived, jiraBoardID, project := stripBoardFrontmatter(content)

board := models.Board{
	Path:        boardPath,
	Columns:     []models.Column{},
	Archived:    archived,
	JiraBoardID: jiraBoardID,
	Project:     project,
}
```

**Step 5: Update board_writer.go to emit the project field**

Update the frontmatter condition and emission in `WriteBoard`:
```go
if board.Archived || board.JiraBoardID != 0 || board.Project != "" {
	buf.WriteString("---\n")
	if board.Archived {
		buf.WriteString("archived: true\n")
	}
	if board.JiraBoardID != 0 {
		buf.WriteString(fmt.Sprintf("jira_board_id: %d\n", board.JiraBoardID))
	}
	if board.Project != "" {
		buf.WriteString(fmt.Sprintf("project: %s\n", board.Project))
	}
	buf.WriteString("---\n\n")
}
```

**Step 6: Run the new tests**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && go test ./internal/kanban/fs/ -v
```
Expected: all tests PASS.

**Step 7: Run the full test suite**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && go test ./...
```
Expected: all tests PASS.

**Step 8: Commit**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && git add internal/kanban/ && git commit -m "$(cat <<'EOF'
Add project frontmatter field to board model, reader, and writer

Board.Project stores the relative path from board.md to the linked
project index file. Parsed from and written to board.md frontmatter.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Update BoardsForProject and ProjectsForBoard to use frontmatter

**Files:**
- Modify: `internal/workspace/workspace.go`
- Modify: `internal/workspace/workspace_test.go`
- Modify: `internal/tui/app.go`

**Step 1: Write failing tests**

Add to `internal/workspace/workspace_test.go`:

```go
func TestBoardsForProject_UsesProjectFrontmatter(t *testing.T) {
	tmp := t.TempDir()

	// Create project dir with index file
	projDir := filepath.Join(tmp, "projects", "alpha")
	os.MkdirAll(projDir, 0755)
	os.WriteFile(filepath.Join(projDir, "alpha.md"), []byte("# alpha\n"), 0644)

	// Create board with project frontmatter pointing at alpha
	boardDir := filepath.Join(tmp, "boards", "sprint")
	os.MkdirAll(filepath.Join(boardDir, "cards"), 0755)
	// path from board.md: ../../projects/alpha/alpha.md
	os.WriteFile(filepath.Join(boardDir, "board.md"), []byte(
		"---\nproject: ../../projects/alpha/alpha.md\n---\n\n# sprint\n\n## Backlog\n\n## Done\n",
	), 0644)

	// Create board WITHOUT project frontmatter
	boardDir2 := filepath.Join(tmp, "boards", "personal")
	os.MkdirAll(filepath.Join(boardDir2, "cards"), 0755)
	os.WriteFile(filepath.Join(boardDir2, "board.md"), []byte("# personal\n\n## Backlog\n\n## Done\n"), 0644)

	scan, err := scanner.ScanWorkspace(tmp)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	ws, err := Load(scan)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	alphaBoards := ws.Projects.BoardsForProject("alpha", ws.Boards)
	if len(alphaBoards) != 1 {
		t.Fatalf("expected 1 board for alpha, got %d", len(alphaBoards))
	}
	if filepath.Base(alphaBoards[0].Path) != "sprint" {
		t.Errorf("expected sprint board, got %q", filepath.Base(alphaBoards[0].Path))
	}
}

func TestProjectsForBoard_UsesProjectFrontmatter(t *testing.T) {
	tmp := t.TempDir()

	// Create project dir
	projDir := filepath.Join(tmp, "projects", "alpha")
	os.MkdirAll(projDir, 0755)
	os.WriteFile(filepath.Join(projDir, "alpha.md"), []byte("# alpha\n"), 0644)

	// Board with project frontmatter
	boardDir := filepath.Join(tmp, "boards", "sprint")
	os.MkdirAll(filepath.Join(boardDir, "cards"), 0755)
	os.WriteFile(filepath.Join(boardDir, "board.md"), []byte(
		"---\nproject: ../../projects/alpha/alpha.md\n---\n\n# sprint\n\n## Backlog\n\n## Done\n",
	), 0644)

	scan, err := scanner.ScanWorkspace(tmp)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	ws, err := Load(scan)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	names := ws.Projects.ProjectsForBoard(boardDir, ws.Boards)
	if len(names) == 0 {
		t.Fatal("expected project names for sprint board, got none")
	}
	if names[0] != "alpha" {
		t.Errorf("expected 'alpha', got %q", names[0])
	}
}

func TestProjectsForBoard_NoFrontmatter(t *testing.T) {
	tmp := t.TempDir()

	boardDir := filepath.Join(tmp, "boards", "personal")
	os.MkdirAll(filepath.Join(boardDir, "cards"), 0755)
	os.WriteFile(filepath.Join(boardDir, "board.md"), []byte("# personal\n\n## Backlog\n\n## Done\n"), 0644)

	scan, err := scanner.ScanWorkspace(tmp)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	ws, err := Load(scan)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	names := ws.Projects.ProjectsForBoard(boardDir, ws.Boards)
	if names != nil {
		t.Errorf("expected nil for board with no project frontmatter, got %v", names)
	}
}
```

**Step 2: Run new tests to verify they fail**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && go test ./internal/workspace/ -run "TestBoardsForProject_UsesProjectFrontmatter|TestProjectsForBoard_UsesProjectFrontmatter|TestProjectsForBoard_NoFrontmatter" -v
```
Expected: FAIL — `ProjectsForBoard` has wrong signature and both functions use the old dir-prefix logic.

**Step 3: Update BoardsForProject in workspace.go**

Replace the existing `BoardsForProject` body with frontmatter-based matching. The `path/filepath` import is already present.

```go
// BoardsForProject returns boards whose project frontmatter links to the given project.
// Returns nil for virtual projects (no DirPath).
func (r *ProjectRegistry) BoardsForProject(name string, allBoards []kanbanmodels.Board) []kanbanmodels.Board {
	proj := r.projects[name]
	if proj == nil || proj.DirPath == "" {
		return nil
	}
	indexPath := filepath.Join(proj.DirPath, proj.Name+".md")
	var result []kanbanmodels.Board
	for _, b := range allBoards {
		if b.Project == "" {
			continue
		}
		boardMdPath := filepath.Join(b.Path, "board.md")
		resolved := filepath.Clean(filepath.Join(filepath.Dir(boardMdPath), b.Project))
		if resolved == indexPath {
			result = append(result, b)
		}
	}
	return result
}
```

**Step 4: Update ProjectsForBoard in workspace.go**

Replace the existing `ProjectsForBoard` with the new signature and frontmatter-based logic:

```go
// ProjectsForBoard returns the project names (immediate + ancestors) linked to the
// board at boardPath via the board's project frontmatter field.
// Returns nil if the board has no project frontmatter or no matching project is found.
func (r *ProjectRegistry) ProjectsForBoard(boardPath string, allBoards []kanbanmodels.Board) []string {
	var board *kanbanmodels.Board
	for i := range allBoards {
		if allBoards[i].Path == boardPath {
			board = &allBoards[i]
			break
		}
	}
	if board == nil || board.Project == "" {
		return nil
	}

	boardMdPath := filepath.Join(boardPath, "board.md")
	resolved := filepath.Clean(filepath.Join(filepath.Dir(boardMdPath), board.Project))

	var immediate *Project
	for _, p := range r.projects {
		if p.DirPath == "" {
			continue
		}
		indexPath := filepath.Join(p.DirPath, p.Name+".md")
		if resolved == indexPath {
			immediate = p
			break
		}
	}
	if immediate == nil {
		return nil
	}

	var result []string
	for cur := immediate; cur != nil; {
		result = append(result, cur.Name)
		if cur.Parent == "" {
			break
		}
		cur = r.projects[cur.Parent]
	}
	return result
}
```

**Step 5: Fix the ProjectsForBoard call in app.go**

In `internal/tui/app.go`, update the `projectsForBoard` helper function to pass `ws.Boards`:

```go
func projectsForBoard(workspaces []*workspace.Workspace, boardPath string) []string {
	for _, ws := range workspaces {
		if ws.Projects == nil {
			continue
		}
		if names := ws.Projects.ProjectsForBoard(boardPath, ws.Boards); len(names) > 0 {
			return names
		}
	}
	return nil
}
```

**Step 6: Run new tests**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && go test ./internal/workspace/ -run "TestBoardsForProject_UsesProjectFrontmatter|TestProjectsForBoard_UsesProjectFrontmatter|TestProjectsForBoard_NoFrontmatter" -v
```
Expected: PASS.

**Step 7: Run the full test suite**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && go test ./...
```
Expected: all tests PASS.

**Step 8: Commit**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && git add internal/workspace/ internal/tui/app.go && git commit -m "$(cat <<'EOF'
Switch board-project linking from directory structure to frontmatter

BoardsForProject and ProjectsForBoard now resolve the board's project
frontmatter field (relative path) instead of checking directory prefixes.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Simplify availableDirs computation in app.go

**Files:**
- Modify: `internal/tui/app.go`

`availableDirs` is used by the board picker to know which `boards/` directory to create new boards in. Previously it gathered parent dirs of all discovered boards (some of which could be inside project subdirs). Now boards only live at `<workspace>/boards/`.

**Step 1: Replace the availableDirs computation block**

Find the block starting with `// Compute available boards/ directories for the picker.` in `NewAppModel` and replace it:

```go
// Compute available boards/ directories for the picker.
// Always use <workspace>/boards/ for each workspace.
seen := make(map[string]bool)
var availableDirs []string

for _, ws := range workspaces {
	dir := filepath.Join(ws.RootDir, "boards")
	if !seen[dir] {
		seen[dir] = true
		availableDirs = append(availableDirs, dir)
	}
}
sort.Strings(availableDirs)
```

**Step 2: Build and run full test suite**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && go build ./... && go test ./...
```
Expected: builds cleanly, all tests PASS.

**Step 3: Commit**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && git add internal/tui/app.go && git commit -m "$(cat <<'EOF'
Simplify availableDirs in board picker to always use workspace root

boards/ is now always at <workspace>/boards/; no need to discover
dynamically from existing board paths.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Remove boards/ and tasks/ options from project scaffold UI

**Files:**
- Modify: `internal/tui/projects/projects.go`

The scaffold flow for converting virtual projects to physical ones currently offers three toggleable options: index note, `boards/` directory, `tasks/` directory. Since projects no longer own their own boards/tasks, remove those two options. With only one option remaining, skip the selection screen (`modeScaffoldSelect`) entirely.

**Step 1: Remove modeScaffoldSelect from the projectMode iota**

In the `const` block at the top of `projects.go`, delete `modeScaffoldSelect`:

```go
const (
	modeList projectMode = iota
	modeSearch
	modeSelectWorkspace
	modeSelectDir
	modeCreate
	modeRename
	modeScaffoldConfirm
	modeSetParent
	modeDeleteVirtual
)
```

**Step 2: Remove scaffoldOptCursor from ProjectsModel**

Delete the `scaffoldOptCursor int` field from the `ProjectsModel` struct.

**Step 3: Remove modeScaffoldSelect from HintText**

Delete the `case modeScaffoldSelect:` case from `HintText()`.

**Step 4: Remove modeScaffoldSelect from Update and View**

In `Update`, delete `case modeScaffoldSelect: return m.updateScaffoldSelect(msg)`.
In `View`, delete `case modeScaffoldSelect: return m.viewScaffoldSelect()`.

**Step 5: Delete updateScaffoldSelect and viewScaffoldSelect methods**

Delete the entire `updateScaffoldSelect` and `viewScaffoldSelect` method bodies.

**Step 6: Rewrite startScaffoldSelect as startScaffoldConfirm**

Replace `startScaffoldSelect` with a simplified version that populates only the index note option and goes straight to `modeScaffoldConfirm`:

```go
func (m ProjectsModel) startScaffoldConfirm() (ProjectsModel, tea.Cmd) {
	name := ""
	if m.scaffoldEntry != nil {
		name = m.scaffoldEntry.Project.Name
	}
	m.scaffoldOptions = []scaffoldOption{
		{label: name + ".md (index note)", path: name + ".md", checked: true},
	}
	m.mode = modeScaffoldConfirm
	return m, nil
}
```

**Step 7: Update all callers of startScaffoldSelect**

There are two callers:
1. `startScaffold()` — replace `return m.startScaffoldSelect()` with `return m.startScaffoldConfirm()`
2. `updateSelectDir()` — replace `return m.startScaffoldSelect()` with `return m.startScaffoldConfirm()`

**Step 8: Build to verify no compile errors**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && go build ./...
```
Expected: no errors.

**Step 9: Run full test suite**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && go test ./...
```
Expected: all tests PASS.

**Step 10: Commit**

```bash
cd /Users/wyatt/worktrees/board-task-dir-update/wydo && git add internal/tui/projects/projects.go && git commit -m "$(cat <<'EOF'
Remove boards/ and tasks/ options from project scaffold UI

Projects no longer own board/task directories. Scaffold now only
creates the index note; skip the selection screen entirely.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```
