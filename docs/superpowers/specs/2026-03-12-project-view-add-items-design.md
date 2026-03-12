# Design: Add Items Directly in Project View

**Date:** 2026-03-12
**Status:** Approved

## Overview

Add the ability to create notes, tasks, and cards directly from the project detail view (`ViewProjectDetail`). All three item types must be linked to the selected project. The creation flows reuse existing components and patterns already present in the codebase.

## Scope

- Add `n` keybinding in `DetailModel` to trigger item creation
- For each item type: reuse existing creation components and persist via existing message patterns
- Support selecting a sub-project at creation time; pre-populate based on cursor context
- Skip the sub-project step if the project has no descendants

Out of scope: editing or deleting items from the project view (already handled by navigating to the respective view).

## Architecture

### New file: `internal/tui/projects/detail_create.go`

All creation state, step handlers, and view rendering for the new creation flows live here, in the `projects` package. This keeps `detail.go` focused on layout/navigation and avoids growing an already large file.

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
createSubProjectPicker *createSubProjectPickerModel
createNoteInput        *taskview.TextInputModel
createTaskInput        *taskview.TextInputModel
createTaskEditor       *taskview.TaskEditorModel
createBoardPicker      *kanban.BoardSelectorModel

// pending project selection during multi-step flow
pendingProjectName string

// data for task editor
allProjectItems []kanbanview.ProjectPickerItem
allContexts     []string
```

`allProjectItems` and `allContexts` are passed into `NewDetailModel` and sourced from the workspace in `app.go`.

## Creation Flow

### Trigger

`n` pressed in normal mode → `handleNew()`.

### Step 1: Sub-project picker (conditional)

If `m.allDescendants` is non-empty, show `createSubProjectPickerModel`:
- Lists root project first, then all descendants in depth-first order
- Pre-selects the project of the row under the cursor (`m.currentRow().projectName`), defaulting to root if cursor is on nothing
- Simple j/k list (no fuzzy needed; sub-project lists are typically small)
- `enter` confirms, `esc` cancels the whole flow

If no descendants, skip directly to Step 2 with `pendingProjectName = m.name`.

### Step 2: Item-specific creation (branches on `m.selectedCol`)

**Notes column (`colNotes`)**

Mode: `detailModeNewNoteName`

- Show `TextInputModel` with placeholder "Note filename..."
- On confirm: create `{pendingProject.DirPath}/{name}.md` with empty content, open in `$EDITOR` via `tea.ExecProcess`
- On `noteEditorFinishedMsg`: emit `DataRefreshMsg`

**Tasks column (`colTasks`)**

Mode: `detailModeNewTaskName` → `detailModeNewTaskEditor`

- Show `TextInputModel` with placeholder "Task name..."
- On confirm: construct `*data.Task` with `Projects: []string{pendingProjectName}`, open `TaskEditorModel` pre-populated
- On `TaskEditorResultMsg` (saved): emit `taskview.TaskUpdateMsg` (persisted by `app.go`) + `DataRefreshMsg`
- On `TaskEditorResultMsg` (cancelled): return to normal mode

**Cards column (`colCards`)**

Mode: `detailModeNewBoardPick`

- Show `BoardSelectorModel` initialised with `m.allBoards` and empty `currentBoardPath` (no exclusions)
- On board selected: load board fresh via `fs.ReadBoard`, call `operations.CreateCardFromTask(&board, "", []string{pendingProjectName}, nil, nil, nil, 0)`, open card in `$EDITOR`
- On `cardEditorFinishedMsg`: emit `DataRefreshMsg`
- On `esc`: return to normal mode

## Data & Persistence

| Item | Storage location | Project link |
|------|-----------------|--------------|
| Note | `selectedProject.DirPath/{name}.md` | Location in project directory |
| Task | First task dir in workspace (via `taskSvc.Add`) | `+projectName` tag pre-set before editor |
| Card | `selectedBoard/cards/{filename}.md` | `projects: [projectName]` in YAML frontmatter |

## `app.go` changes

1. Pass `allProjectItems` and `allContexts` into `NewDetailModel` and when rebuilding `projectDetailView` on `OpenProjectMsg`
2. No new message types needed — `taskview.TaskUpdateMsg` and `DataRefreshMsg` are already handled

## Key bindings

| Key | Context | Action |
|-----|---------|--------|
| `n` | Normal mode, any column | Start item creation flow |
| `j`/`k` | Sub-project picker | Navigate |
| `enter` | Sub-project picker | Confirm sub-project |
| `esc` | Any creation mode | Cancel, return to normal |
| `j`/`k` | Board picker | Navigate |
| `enter` | Board picker | Confirm board selection |

## Error handling

- If `pendingProject.DirPath == ""` (virtual project with no directory) when creating a note: show an inline error message and abort — notes require a physical project directory
- If board load fails after selection: log error, return to normal mode
- If card creation fails: log error, return to normal mode

## Files changed

- `internal/tui/projects/detail.go` — new mode constants, new fields, route new modes in `Update`, pass new args in `NewDetailModel`
- `internal/tui/projects/detail_create.go` — new file: all creation step handlers and `createSubProjectPickerModel`
- `internal/tui/app.go` — pass `allProjectItems` and `allContexts` to `NewDetailModel` / `OpenProjectMsg` handler
