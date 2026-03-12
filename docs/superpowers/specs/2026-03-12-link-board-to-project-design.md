# Link Board to Project — Design Spec

**Date:** 2026-03-12

## Problem

A board can declare a linked project via its `project:` frontmatter field, but there is currently no TUI to set or change that link. Users must edit `board.md` by hand. Additionally, when a card is created on a project-linked board, the project is not applied to the card until after the editor closes — the card file is written with empty projects first.

## Goals

1. Add a TUI keyboard shortcut (`L`) on the board view that opens a single-select project picker, allowing the user to link or unlink the board to a project.
2. Ensure newly created cards inherit the board's linked project immediately at creation time (before the editor opens), not only after the editor closes.

## Non-goals

- Linking a board to more than one project (single-project linking is sufficient).
- Modifying the file format (`project:` frontmatter field stays as-is).

## Design

### 1. `MultiSelectPickerConfig` — `SingleSelect bool`

Add `SingleSelect bool` to `MultiSelectPickerConfig` in `multiselect_picker.go`. When true:

- Pressing `enter` or `space` on a highlighted item immediately selects it as the sole selection and returns `isDone=true`.
- Pressing `enter` or `space` on an item that is already selected (the current linked project) clears the selection and returns `isDone=true` — this is the "unlink" path.
- All other picker behavior (fuzzy filter, navigation, `esc` to cancel) is unchanged.

### 2. `ProjectPickerItem` — `DirPath string`

Add `DirPath string` to `ProjectPickerItem` in `projectpicker.go`. Virtual projects (those that exist only in task/card frontmatter, with no directory on disk) have `DirPath == ""`. Physical projects have `DirPath` set to the absolute path of the project directory.

Add `NewBoardProjectPickerModel(currentProject string, allProjects []ProjectPickerItem) ProjectPickerModel` — a wrapper that builds a `ProjectPickerModel` with `SingleSelect: true` and `currentProject` pre-selected.

### 3. `SetBoardProject` operation

Add `SetBoardProject(board *models.Board, relPath string) error` to `board_ops.go`. It sets `board.Project = relPath` and calls `fs.WriteBoard`. Passing an empty string clears the link.

### 4. Board TUI changes (`board.go`)

**New mode:** `boardModeProjectLink`.

**New field:** `boardProjectPicker *ProjectPickerModel`.

**New accessor:** `SetBoardProjects(projects []string)` — allows `app.go` to push the resolved board project chain into the model after a data refresh.

**Keybinding:** `L` in normal mode opens the board-project picker. `L` is not used by the board's existing normal-mode handler and is not in the global view-switch key list in `app.go`.

**Picker completion:**
1. User selects a project → compute `relPath = filepath.Rel(board.Path, project.DirPath + "/" + project.Name + ".md")`. Call `SetBoardProject`. Set `m.boardProjects = []string{projectName}` immediately (ancestors are resolved after the subsequent `DataRefreshMsg`).
2. User clears selection → call `SetBoardProject` with empty string. Set `m.boardProjects = nil`.
3. User cancels with `esc` → no change.

After any change, emit `DataRefreshMsg` so `app.go` rescans workspaces and calls `SetBoardProjects` with the fully resolved project chain (immediate + ancestors).

**Cannot link with virtual project:** If the selected project has `DirPath == ""`, show an error message and do not persist. (A relative path to a project index file requires the project to have a directory on disk.)

**Card creation (`handleNew`):** After `operations.CreateCard` returns, immediately call `m.ensureCardBoardProjects` for the new card. This writes the board's linked project(s) to the card file before the editor opens, so the user sees the project already populated in the frontmatter.

**`View()`:** Render the project picker overlay when in `boardModeProjectLink`.

**`IsModal()`:** Return `true` when in `boardModeProjectLink`.

**`HintText()`:** Include `L link project` in the normal-mode hint.

### 5. `app.go` changes

**`collectAllProjects`:** Populate `DirPath` on each `ProjectPickerItem` from `workspace.Project.DirPath`.

**`DataRefreshMsg` handler:** After `m.refreshData()`, if a board is loaded, call:
```go
m.boardView.SetBoardProjects(projectsForBoard(m.workspaces, m.boardView.BoardPath()))
```
This ensures the board model has the fully resolved project chain (including ancestor projects) after the board's project link changes.

## File Change Summary

| File | Change |
|------|--------|
| `internal/tui/kanban/multiselect_picker.go` | Add `SingleSelect bool` to config; handle auto-close on selection |
| `internal/tui/kanban/projectpicker.go` | Add `DirPath string` to `ProjectPickerItem`; add `NewBoardProjectPickerModel` |
| `internal/kanban/operations/board_ops.go` | Add `SetBoardProject` |
| `internal/tui/kanban/board.go` | Add mode, field, accessor, keybinding, picker handling, card creation fix |
| `internal/tui/app.go` | Populate `DirPath` in `collectAllProjects`; refresh board projects in `DataRefreshMsg` |

## Key Invariants

- A board with no `project:` frontmatter has `boardProjects == nil`; `ensureCardBoardProjects` is a no-op.
- Setting a board project to a virtual project (no directory) is rejected — a relative path cannot be computed.
- Unlinking sets `board.Project = ""` and `boardProjects = nil`; subsequent card creations are unaffected.
- The `project:` field format in `board.md` is unchanged: a relative path from the board directory to the project's `<name>.md` index file.
