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

## Background: `Board.Path`

Throughout this spec, `board.Path` refers to the absolute path to the **board directory** (the directory containing `board.md`), not to `board.md` itself. This matches the existing codebase where `Board.Path` is the directory path.

## Design

### 1. `MultiSelectPickerConfig` — `SingleSelect bool`

Add `SingleSelect bool` to `MultiSelectPickerConfig` in `multiselect_picker.go`. When true:

- Pressing `enter` on a highlighted item immediately selects it as the sole selection and returns `isDone=true, cancelled=false`.
- Pressing `space` behaves identically to `enter` in `SingleSelect` mode (unlike normal multi-select mode, where `space` only toggles without closing).
- Pressing `enter` or `space` on an item that is already the sole selection clears it and returns `isDone=true, cancelled=false` — this is the "unlink" path.
- Pressing `esc` returns `isDone=true, cancelled=true` — no change to selection.
- All other picker behavior (fuzzy filter, navigation) is unchanged.

`MultiSelectPickerModel.Update()` currently returns `(MultiSelectPickerModel, tea.Cmd, bool)`. Add a fourth return value: `(MultiSelectPickerModel, tea.Cmd, bool, bool)` where the fourth value is `cancelled`. `ProjectPickerModel.Update` and `TagPickerModel.Update` receive and forward `cancelled` in their own return signatures. Their callers in `board.go` (`updateTagEdit`, `updateProjectEdit`) already use `msg.String() == "enter"` for their save decision — they do not need to change, since `cancelled` is only relevant to the board-project-link flow. The board-project picker uses `cancelled` to decide whether to persist changes.

### 2. `ProjectPickerItem` — `DirPath string`

Add `DirPath string` to `ProjectPickerItem` in `projectpicker.go`. Virtual projects (those that exist only in task/card frontmatter, with no directory on disk) have `DirPath == ""`. Physical projects have `DirPath` set to the absolute path of the project directory.

Add `NewBoardProjectPickerModel(currentProjectName string, allProjects []ProjectPickerItem) ProjectPickerModel` — a wrapper that builds a `ProjectPickerModel` with `SingleSelect: true` and `currentProjectName` (the project name string, e.g. `"home-remodel"`) pre-selected if non-empty.

### 3. `SetBoardProject` operation

Add `SetBoardProject(board *models.Board, relPath string) error` to `board_ops.go`. It sets `board.Project = relPath` and calls `fs.WriteBoard(*board)` (by value, consistent with all other operations in `board_ops.go`). Passing an empty string clears the link.

### 4. Board TUI changes (`board.go`)

**New mode:** `boardModeProjectLink`.

**New field:** `boardProjectPicker *ProjectPickerModel`.

**New accessor:** `SetBoardProjects(projects []string)` sets `m.boardProjects`. Values are project names (same format as the existing `boardProjects` field). This allows `app.go` to push the resolved board project chain into the model after a data refresh.

**Keybinding:** `L` in normal mode opens the board-project picker. `L` is not used by the board's existing normal-mode handler and is not in the global view-switch key list in `app.go` (which uses `N`, `P`, `B`, `A`, `W`, `M`, `T`).

**Opening the picker:** Construct `NewBoardProjectPickerModel` with the current directly-linked project name. Use `m.boardProjects[0]` if non-empty (the immediate project; ancestors follow at index 1+). If `m.boardProjects` is nil — including the case where `board.Project` is set but resolves to no known project (stale path after a rename/move) — pass an empty string; the picker opens with nothing pre-selected.

**Picker completion — order of operations:**
1. Check `cancelled`: if `true`, close picker, no change.
2. Get selected project name from picker. If selected project has `DirPath == ""` (virtual project), show `m.err = "cannot link to virtual project"` and close picker — no persist.
3. If a project was selected: compute `relPath, _ = filepath.Rel(board.Path, filepath.Join(project.DirPath, project.Name+".md"))`. Call `SetBoardProject`. On error, set `m.err` and close picker. On success: set `m.boardProjects = []string{projectName}` immediately; show success message; emit `DataRefreshMsg`.
4. If no project was selected (unlink): call `SetBoardProject` with `""`. On error, set `m.err`. On success: set `m.boardProjects = nil`; show success message; emit `DataRefreshMsg`.

The immediate `m.boardProjects` update (step 3/4) is intentional: it allows subsequent card creations to use the correct project right away, before the `DataRefreshMsg` completes. The transient single-element state (missing ancestors) is acceptable because ancestor resolution only matters for display and cross-filtering, not for card frontmatter writes.

**Card creation (`handleNew`):** After `operations.CreateCard` returns successfully, immediately call `m.ensureCardBoardProjects` with the new card's column and card indices. This writes the board's linked project(s) to the card file before the editor opens. `ensureCardBoardProjects` is already idempotent and a no-op when `boardProjects` is empty.

**`View()`:** Render the project picker overlay when in `boardModeProjectLink`.

**`IsModal()`:** Return `true` when in `boardModeProjectLink`.

**`HintText()`:** Include `L link project` in the normal-mode hint.

### 5. `app.go` changes

**`collectAllProjects`:** Populate `DirPath` on each `ProjectPickerItem` from the corresponding `workspace.Project.DirPath`. Virtual projects get `DirPath == ""`.

**`DataRefreshMsg` handler:** After `m.refreshData()`, unconditionally (not gated on `m.currentView`), update board projects if a board is loaded:
```go
if m.boardLoaded {
    m.boardView.SetBoardProjects(projectsForBoard(m.workspaces, m.boardView.BoardPath()))
}
```
`m.boardLoaded` is a pre-existing field on `AppModel`. This ensures the board model has the fully resolved project chain (including ancestor projects) after the board's project link changes. Board project resolution is not re-triggered on `SwitchViewMsg` (navigating back to the board view) — this is acceptable because board project links only change via explicit `L`-picker actions, which always emit `DataRefreshMsg`.

## File Change Summary

| File | Change |
|------|--------|
| `internal/tui/kanban/multiselect_picker.go` | Add `SingleSelect bool` to config; add `cancelled bool` as 4th return from `Update`; handle single-select auto-close |
| `internal/tui/kanban/projectpicker.go` | Add `DirPath string` to `ProjectPickerItem`; add `NewBoardProjectPickerModel`; thread `cancelled` through `Update` return |
| `internal/tui/kanban/tagpicker.go` | Thread `cancelled` through `Update` return (callers ignore it) |
| `internal/kanban/operations/board_ops.go` | Add `SetBoardProject` |
| `internal/tui/kanban/board.go` | Add mode, field, accessor, keybinding, picker handling, card creation fix |
| `internal/tui/app.go` | Populate `DirPath` in `collectAllProjects`; refresh board projects in `DataRefreshMsg` |

## Key Invariants

- A board with no `project:` frontmatter has `boardProjects == nil`; `ensureCardBoardProjects` is a no-op.
- Setting a board project to a virtual project (no directory) is rejected at the TUI layer.
- Unlinking sets `board.Project = ""` and `boardProjects = nil`; subsequent card creations are unaffected.
- The `project:` field format in `board.md` is unchanged: a relative path from the board directory to the project's `<name>.md` index file.
- `boardProjects` values are project names (strings like `"home-remodel"`), not paths.
