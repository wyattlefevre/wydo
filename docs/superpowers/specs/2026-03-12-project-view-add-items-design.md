# Design: Add Items Directly in Project View

**Date:** 2026-03-12
**Status:** Approved

## Overview

Add the ability to create notes, tasks, and cards directly from the project detail view (`ViewProjectDetail`). All three item types must be linked to the selected project. The creation flows reuse existing components and patterns already present in the codebase.

## Scope

- Add `n` keybinding in `DetailModel` to trigger item creation (`n` is currently unbound in `handleKey`)
- For each item type: reuse existing creation components and persist via existing message patterns
- Support selecting a sub-project at creation time; pre-populate based on cursor context
- Skip the sub-project step if the project has no descendants

Out of scope: editing or deleting items from the project view (already handled by navigating to the respective view).

## Architecture

### New file: `internal/tui/projects/detail_create.go`

All creation state, step handlers, and view rendering for the new creation flows live here, in the `projects` package. This keeps `detail.go` focused on layout/navigation and avoids growing an already large file.

### New import in `detail.go`

```go
taskview "wydo/internal/tui/tasks"
```

`detail.go` is in package `projects`; `tasks` does not import `projects`, so no circular dependency.

### New `detailMode` values (in `detail.go`)

```go
detailModeSubProjectPick  // j/k list: root project + all descendants
detailModeNewNoteName     // text input: note filename
detailModeNewTaskName     // text input: task name
detailModeNewTaskEditor   // TaskEditorModel modal
detailModeNewBoardPick    // BoardSelectorModel modal
```

### New fields on `DetailModel`

```go
// creation flow state (all nil when not in use)
createSubProjectPicker *createSubProjectPickerModel  // defined in detail_create.go
createNoteInput        *taskview.TextInputModel
createTaskInput        *taskview.TextInputModel
createTaskEditor       *taskview.TaskEditorModel
createBoardPicker      *kanban.BoardSelectorModel

// selected project during multi-step creation flow (nil when not creating)
pendingProject *workspace.Project

// data for task editor
allProjectItems []kanbanview.ProjectPickerItem
allContexts     []string
```

`allProjectItems` and `allContexts` are passed into `NewDetailModel` and sourced from `app.go` (see below).

## Creation Flow

### Trigger

`n` pressed in normal mode → `handleNew()` in `detail_create.go`.

### Step 1: Sub-project picker (conditional)

If `len(m.allDescendants) > 0`, show `createSubProjectPickerModel`:
- Lists root project first, then all descendants in depth-first order
- Pre-selects based on cursor context: read `m.currentRow().projectName` from the currently focused column; if `currentRow()` returns nil (empty column), default to root project name
  - `rowKindGroup` rows have `projectName` set to the sub-project name — use it directly
  - Item rows (`rowKindNote`, `rowKindTask`, `rowKindCard`) also have `projectName` — use it directly
- Simple j/k list (no fuzzy needed; sub-project lists are typically small)
- `enter` confirms and advances to Step 2; `esc` cancels the whole flow and returns to `detailModeNormal`

If `len(m.allDescendants) == 0`, skip directly to Step 2 with `m.pendingProject = m.project`.

### Step 2: Item-specific creation (branches on `m.selectedCol`)

**Notes column (`colNotes`)**

Mode: `detailModeNewNoteName`

- Show `taskview.TextInputModel` with prompt "Note filename" and placeholder "my-note"
- On confirm: verify `m.pendingProject.DirPath != ""` — if empty (virtual project), show an inline error message on `DetailModel` and return to `detailModeNormal` without creating anything
- If valid: create `filepath.Join(m.pendingProject.DirPath, name+".md")` with empty content (`""`), open in `$EDITOR` via `tea.ExecProcess`, storing the result as `noteEditorFinishedMsg` (existing type in `detail.go`)
- On `noteEditorFinishedMsg`: emit `DataRefreshMsg`

**Tasks column (`colTasks`)**

Mode: `detailModeNewTaskName` → `detailModeNewTaskEditor`

- Show `taskview.TextInputModel` with prompt "Task name" and placeholder "do something..."
- On confirm: construct `*data.Task` with `Projects: []string{m.pendingProject.Name}` and all other fields at zero/empty values; open `taskview.TaskEditorModel` pre-populated via `taskview.NewTaskEditor(task, m.allProjectItems, m.allContexts)`, set `Width`/`Height`, transition mode to `detailModeNewTaskEditor`
- Handle `taskview.TaskEditorResultMsg` as a top-level `case` in `DetailModel.Update()` (same pattern as `noteEditorFinishedMsg`):
  - If saved: return `tea.Batch` of `taskview.TaskUpdateMsg` command (app.go persists to task service) and `DataRefreshMsg` command (app.go reloads project detail via `OpenProjectMsg`)
  - If cancelled: return to `detailModeNormal`, clear `createTaskEditor`

**Cards column (`colCards`)**

Mode: `detailModeNewBoardPick`

- Construct `kanban.BoardSelectorModel` via `kanban.NewBoardSelectorModel(m.allBoards, "")` (empty `currentBoardPath` = no exclusions); then set `picker.width = m.width` and `picker.height = m.height` directly (no `SetSize` method exists)
- Note: `BoardSelectorModel.View()` currently renders the title as "Move to Board". Add a `title string` field to `BoardSelectorModel` and pass `"Select Board"` when constructing for creation. The existing call site in `board.go` continues to pass `"Move to Board"`.
- `BoardSelectorModel.Update()` returns `(model, selectedPath, done)` — handle inline in `detailModeNewBoardPick` update handler
- On board selected: load board via `fs.ReadBoard(selectedPath)`; call `operations.CreateCardFromTask(&board, "", []string{m.pendingProject.Name}, []string{}, nil, nil, 0)` (blank title, project in frontmatter, empty tags); open card in `$EDITOR` via `tea.ExecProcess`, storing result as `cardEditorFinishedMsg` (new type defined in `detail_create.go`, same shape as `noteEditorFinishedMsg`)
- On `cardEditorFinishedMsg`: emit `DataRefreshMsg`
- On `esc`: return to `detailModeNormal`
- If board load or card creation fails: log error, return to `detailModeNormal`

## `DetailModel.Update()` routing

Add these cases to the top-level `Update()` switch alongside the existing `noteEditorFinishedMsg` case:

```go
case taskview.TextInputResultMsg:
    return m.handleTextInputResult(msg)  // routes to updateNewNoteInput or updateNewTaskInput based on m.mode
case taskview.TaskEditorResultMsg:
    return m.handleTaskEditorResult(msg)
case cardEditorFinishedMsg:
    return m, func() tea.Msg { return messages.DataRefreshMsg{} }
```

`TextInputModel.Update` dispatches `TextInputResultMsg` via a returned `tea.Cmd` — it arrives in `DetailModel.Update` as a `tea.Msg`, not a `tea.KeyMsg`, so it must be a top-level case. `handleTextInputResult` (in `detail_create.go`) branches on `m.mode` to call the appropriate handler.

**Type assertion note:** Both `taskview.TextInputModel.Update` and `taskview.TaskEditorModel.Update` have the signature `func (...) Update(msg tea.Msg) (tea.Model, tea.Cmd)` — they return the `tea.Model` interface, not the concrete type. All update call sites must type-assert the result:

```go
result, cmd := m.createNoteInput.Update(msg)
m.createNoteInput = result.(*taskview.TextInputModel)

result, cmd := m.createTaskEditor.Update(msg)
m.createTaskEditor = result.(*taskview.TaskEditorModel)
```

`BoardSelectorModel.Update` is unaffected — it returns a concrete `(BoardSelectorModel, string, bool)` tuple.

Add new mode cases to the `detailMode` switch inside `handleKey` delegation:

```go
case detailModeSubProjectPick:
    return m.updateSubProjectPicker(msg)
case detailModeNewNoteName:
    return m.updateNewNoteInput(msg)
case detailModeNewTaskName:
    return m.updateNewTaskInput(msg)
case detailModeNewTaskEditor:
    return m.updateNewTaskEditor(msg)
case detailModeNewBoardPick:
    return m.updateNewBoardPick(msg)
```

## Data & Persistence

| Item | Storage location | Project link |
|------|-----------------|--------------|
| Note | `pendingProject.DirPath/{name}.md` | Location in project directory |
| Task | First task dir in workspace (via `taskSvc.Add` in `app.go`) | `+projectName` tag, pre-set on `task.Projects` before editor |
| Card | `selectedBoard/cards/{filename}.md` | `projects: [projectName]` in YAML frontmatter via `CreateCardFromTask` |

## `app.go` changes

### Pass new fields into `NewDetailModel`

Compute in the `OpenProjectMsg` handler (same place the detail model is constructed):

```go
allProjectItems := collectAllProjects(m.workspaces)  // already exists
allContexts := taskview.ExtractUniqueContexts(ws.Tasks)  // exported from sort_group_state.go
```

Pass both to `NewDetailModel` (update signature).

### No new message types at app level

`taskview.TaskUpdateMsg` is already handled at app.go:258 — when `task.File == ""` it calls `taskSvc.Add`.
`DataRefreshMsg` is already handled at app.go:360 — when `currentView == ViewProjectDetail` it triggers `OpenProjectMsg` to reload.
Both are returned as a `tea.Batch` from `DetailModel` after task creation, so app.go handles each independently.

## `BoardSelectorModel` change

Add `title string` field; use it in `View()` instead of the hardcoded `"Move to Board"` string. The existing construction site in `board.go` passes `"Move to Board"` explicitly.

## Key bindings

| Key | Context | Action |
|-----|---------|--------|
| `n` | Normal mode, any column | Start item creation flow |
| `j`/`k` | Sub-project picker | Navigate |
| `enter` | Sub-project picker | Confirm sub-project, advance |
| `esc` | Any creation mode | Cancel entire flow, return to normal |
| `j`/`k` | Board picker | Navigate |
| `enter` | Board picker | Confirm board selection |

## Error handling

- **Virtual project for note**: if `pendingProject.DirPath == ""`, show inline error and abort — notes require a physical project directory
- **Board load failure**: log error, return to `detailModeNormal`
- **Card creation failure**: log error, return to `detailModeNormal`

## Files changed

| File | Change |
|------|--------|
| `internal/tui/projects/detail.go` | New mode constants, new fields on `DetailModel`, route new modes in `Update()`, update `NewDetailModel` signature |
| `internal/tui/projects/detail_create.go` | New file: `createSubProjectPickerModel`, all creation step handlers, `cardEditorFinishedMsg` |
| `internal/tui/kanban/board_selector.go` | Add `title string` field; replace hardcoded `"Move to Board"` with `m.title` in `View()`; update existing construction call |
| `internal/tui/app.go` | Compute and pass `allProjectItems` + `allContexts` to `NewDetailModel`; update `OpenProjectMsg` handler |
