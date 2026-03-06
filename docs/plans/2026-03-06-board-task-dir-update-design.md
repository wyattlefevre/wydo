# Board and Task Directory Update Design

Date: 2026-03-06

## Overview

Three related changes:
1. Remove support for multiple board directories (boards only at workspace root)
2. Remove support for multiple task directories (tasks only at workspace root)
3. Add `project` frontmatter to `board.md` for linking boards to projects

## Part 1: Single boards/ directory

**Problem:** The scanner currently discovers `boards/` directories anywhere in the workspace tree, including inside project subdirectories. This complexity is unnecessary.

**Change:** In `scanner.go`, `walkWorkspace` only calls `scanBoardsDir` when `dir == rootDir`. Boards inside project directories are ignored.

**Files changed:**
- `internal/scanner/scanner.go`: Add `dir == rootDir` guard before processing `boards` dir; remove `Project` field from `BoardInfo`
- `internal/tui/app.go`: Simplify `availableDirs` computation — always use `<ws.RootDir>/boards/` per workspace
- `testdata/workspace1/`: Move `projects/alpha/boards/sprint/` to `boards/sprint/`
- `internal/scanner/scanner_test.go`: Update `TestScanWorkspace_ProjectContext` (sprint board no longer has project context from directory)

## Part 2: Single tasks/ directory

**Problem:** Same as boards — tasks can currently be discovered inside project subdirectories.

**Change:** In `scanner.go`, `walkWorkspace` only calls `scanTasksDir` when `dir == rootDir`. Remove `Project` field from `TaskDirInfo`.

**Files changed:**
- `internal/scanner/scanner.go`: Add `dir == rootDir` guard before processing `tasks` dir; remove `Project` field from `TaskDirInfo`
- `internal/scanner/scanner_test.go`: Update `TestScanWorkspace_FindsTaskDirs` (expect 1 dir, not 3); update `TestScanWorkspace_Workspace2` (expect 1 task dir)
- `testdata/workspace1/`: Remove `projects/alpha/tasks/` and `projects/home-remodel/tasks/` (or consolidate into root tasks/)

## Part 3: Board-to-project linking via frontmatter

**Problem:** Boards were previously implicitly linked to projects via directory location. With boards now only at workspace root, an explicit link is needed.

**Solution:** Add a `project` frontmatter field to `board.md` containing the relative file path from `board.md` to the project's index file. Example:

```yaml
---
project: ../../projects/alpha/alpha.md
---
```

This relative path is resolved from the `board.md` file's location, making it valid when the file is opened in any Markdown editor.

**Files changed:**

`internal/kanban/models/board.go`:
- Add `Project string` field

`internal/kanban/fs/board_reader.go`:
- Parse `project` from frontmatter YAML into `board.Project`

`internal/kanban/fs/board_writer.go`:
- Emit `project: <path>` in frontmatter when `board.Project != ""`

`internal/workspace/workspace.go`:
- `BoardsForProject(name, allBoards)`: Replace directory-prefix matching with frontmatter resolution. For each board with a non-empty `Project` field, resolve the relative path from `<board.Path>/board.md` and compare to the project's index file path (`<proj.DirPath>/<proj.Name>.md`).
- `ProjectsForBoard(boardPath, allBoards)`: Gain `allBoards` parameter. Look up board by path, resolve its `Project` frontmatter path to an absolute path, find the matching project, then walk up the parent chain (same logic as before).

`internal/tui/app.go`:
- `projectsForBoard` helper: pass `ws.Boards` alongside board path

## Part 4: UI — scaffold screen cleanup

**Problem:** The scaffold flow for converting virtual projects to physical projects offers `boards/` and `tasks/` as optional directories to create. These are no longer valid since projects don't own their own board/task directories.

**Change:** Remove `boards/` and `tasks/` options from `startScaffoldSelect()`. Since only the index note remains, skip `modeScaffoldSelect` entirely — go straight to `modeScaffoldConfirm`. Remove `modeScaffoldSelect`, `updateScaffoldSelect`, `viewScaffoldSelect`, and `scaffoldOptCursor` field from `ProjectsModel`.

**Files changed:**
- `internal/tui/projects/projects.go`: Remove scaffold select mode and associated code; simplify `startScaffoldSelect` → `startScaffoldConfirm` that populates options with only the index note then enters `modeScaffoldConfirm`

## What does NOT change

- Project registry building from task `+tags` and card frontmatter
- Board picker UI, board creation flow, board selector
- The `project` frontmatter in `board.md` is opt-in — boards without it continue to work
- All existing `projects/` directory scanning and project hierarchy
