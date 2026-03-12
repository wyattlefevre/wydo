# Implementation Plan: Add Items Directly in Project View

**Spec:** `docs/superpowers/specs/2026-03-12-project-view-add-items-design.md`
**Date:** 2026-03-12

## Steps

### Step 1: Add `title` field to `BoardSelectorModel`

**File:** `internal/tui/kanban/board_selector.go`

- Add `title string` to `BoardSelectorModel` struct
- Update `NewBoardSelectorModel` to accept a `title string` parameter and store it
- Replace hardcoded `"Move to Board"` string in `View()` with `m.title`
- Update the one existing call site in `board.go` to pass `"Move to Board"` explicitly

**Verify:** `go build ./...` passes. The board view move-to-board flow still shows "Move to Board".

---

### Step 2: Add new mode constants and fields to `DetailModel`

**File:** `internal/tui/projects/detail.go`

- Add 5 new `detailMode` constants after the existing ones:
  ```go
  detailModeSubProjectPick
  detailModeNewNoteName
  detailModeNewTaskName
  detailModeNewTaskEditor
  detailModeNewBoardPick
  ```
- Add new fields to `DetailModel`:
  ```go
  createSubProjectPicker *createSubProjectPickerModel
  createNoteInput        *taskview.TextInputModel
  createTaskInput        *taskview.TextInputModel
  createTaskEditor       *taskview.TaskEditorModel
  createBoardPicker      *kanban.BoardSelectorModel
  pendingProject         *workspace.Project
  allProjectItems        []kanbanview.ProjectPickerItem
  allContexts            []string
  ```
- Add import: `taskview "wydo/internal/tui/tasks"`
- Update `NewDetailModel` signature to accept `allProjectItems []kanbanview.ProjectPickerItem` and `allContexts []string`; store them on the model

**Verify:** `go build ./...` — will fail until `app.go` is updated (Step 4), but the types compile.

---

### Step 3: Create `detail_create.go`

**File:** `internal/tui/projects/detail_create.go` (new file)

This file implements all creation logic in the `projects` package.

#### 3a. `cardEditorFinishedMsg` type
```go
type cardEditorFinishedMsg struct{ err error }
```

#### 3b. `createSubProjectPickerModel`
A simple j/k picker listing the root project + descendants:
```go
type createSubProjectPickerModel struct {
    projects []*workspace.Project  // root first, then descendants
    cursor   int
    width    int
    height   int
}
```
- `Update(msg tea.KeyMsg) (*workspace.Project, bool)` — returns selected project + done flag
- `View() string` — centered modal with title "Add to Project", j/k list with `> ` cursor marker, help line

#### 3c. `handleNew`
Called when `n` is pressed in normal mode:
```go
func (m DetailModel) handleNew() (DetailModel, tea.Cmd)
```
- If `len(m.allDescendants) > 0`:
  - Determine pre-selected project: `currentRow()` — if non-nil use `row.projectName` resolved via `m.registry.Get(row.projectName)`; if nil use `m.project`
  - Build project list: `[m.project] + m.allDescendants`
  - Find pre-selected index in list
  - Construct `createSubProjectPickerModel`, set width/height, set mode to `detailModeSubProjectPick`
- Else:
  - Set `m.pendingProject = m.project`
  - Call `m.startItemCreation()`

#### 3d. `updateSubProjectPicker`
Called from the `tea.KeyMsg` branch of `Update()` when mode is `detailModeSubProjectPick`:
```go
func (m DetailModel) updateSubProjectPicker(msg tea.KeyMsg) (DetailModel, tea.Cmd)
```
- Forward key to `m.createSubProjectPicker.Update(msg)`
- If done and project selected: set `m.pendingProject`, clear picker, call `m.startItemCreation()`
- If done and cancelled (esc): clear picker, return to `detailModeNormal`

#### 3e. `startItemCreation`
Branches on `m.selectedCol` to start the column-specific flow:
```go
func (m DetailModel) startItemCreation() (DetailModel, tea.Cmd)
```
- `colNotes`: construct `TextInputModel` with prompt "Note filename", placeholder "my-note", set mode `detailModeNewNoteName`, return focus cmd
- `colTasks`: construct `TextInputModel` with prompt "Task name", placeholder "do something...", set mode `detailModeNewTaskName`, return focus cmd
- `colCards`: construct `BoardSelectorModel` with `kanban.NewBoardSelectorModel(m.allBoards, "")` and title `"Select Board"`, set `picker.width`/`picker.height`, set mode `detailModeNewBoardPick`

#### 3f. `handleTextInputResult`
Called from top-level `Update()` when `taskview.TextInputResultMsg` arrives:
```go
func (m DetailModel) handleTextInputResult(msg taskview.TextInputResultMsg) (DetailModel, tea.Cmd)
```
- If `msg.Cancelled`: clear inputs, return to `detailModeNormal`
- Branch on `m.mode`:
  - `detailModeNewNoteName`: call `m.finishNoteCreation(msg.Value)`
  - `detailModeNewTaskName`: call `m.finishTaskNameEntry(msg.Value)`

#### 3g. `finishNoteCreation`
```go
func (m DetailModel) finishNoteCreation(name string) (DetailModel, tea.Cmd)
```
- If `strings.TrimSpace(name) == ""`: return to normal
- If `m.pendingProject.DirPath == ""`: set `m.message = "Cannot create note: project has no directory"`, return to `detailModeNormal`
- Sanitize filename: strip `.md` suffix if user typed it, then append `.md`
- Create file: `os.WriteFile(filepath.Join(m.pendingProject.DirPath, filename), []byte(""), 0644)`
- Open in editor via `openNoteInEditor(filePath)` (existing function in `detail.go`)
- Clear `createNoteInput`, set mode to `detailModeNormal` (editor takes over)

#### 3h. `finishTaskNameEntry`
```go
func (m DetailModel) finishTaskNameEntry(name string) (DetailModel, tea.Cmd)
```
- If `strings.TrimSpace(name) == ""`: return to normal
- Construct `*data.Task{Name: name, Projects: []string{m.pendingProject.Name}, Contexts: []string{}, Tags: make(map[string]string)}`
- `editor := taskview.NewTaskEditor(task, m.allProjectItems, m.allContexts)`
- Set `editor.Width = m.width`, `editor.Height = m.height`
- Store as `m.createTaskEditor`, clear `createTaskInput`, set mode to `detailModeNewTaskEditor`

#### 3i. `updateNewTaskEditor`
Called from `tea.KeyMsg` branch when mode is `detailModeNewTaskEditor`:
```go
func (m DetailModel) updateNewTaskEditor(msg tea.KeyMsg) (DetailModel, tea.Cmd)
```
- Forward key to `m.createTaskEditor.Update(msg)` with type assertion
- Result message handling is done in `handleTaskEditorResult` (top-level `Update()`)

#### 3j. `handleTaskEditorResult`
Called from top-level `Update()` when `taskview.TaskEditorResultMsg` arrives:
```go
func (m DetailModel) handleTaskEditorResult(msg taskview.TaskEditorResultMsg) (DetailModel, tea.Cmd)
```
- Clear `m.createTaskEditor`, return to `detailModeNormal`
- If `msg.Cancelled`: return, no commands
- If saved: return `tea.Batch` of:
  - `func() tea.Msg { return taskview.TaskUpdateMsg{Task: msg.Task} }`
  - `func() tea.Msg { return messages.DataRefreshMsg{} }`

#### 3k. `updateNewBoardPick`
Called from `tea.KeyMsg` branch when mode is `detailModeNewBoardPick`:
```go
func (m DetailModel) updateNewBoardPick(msg tea.KeyMsg) (DetailModel, tea.Cmd)
```
- Forward key to `m.createBoardPicker.Update(msg)` (returns concrete tuple, no type assertion needed)
- If done and path selected: call `m.finishCardCreation(selectedPath)`
- If done and cancelled: clear picker, return to `detailModeNormal`

#### 3l. `finishCardCreation`
```go
func (m DetailModel) finishCardCreation(boardPath string) (DetailModel, tea.Cmd)
```
- Clear `m.createBoardPicker`, set mode to `detailModeNormal`
- Load board: `board, err := fs.ReadBoard(boardPath)` — on error: log, return
- Create card: `card, err := operations.CreateCardFromTask(&board, "", []string{m.pendingProject.Name}, []string{}, nil, nil, 0)` — on error: log, return
- Open editor: `tea.ExecProcess(exec.Command(editor, cardPath), func(err error) tea.Msg { return cardEditorFinishedMsg{err} })`

#### 3m. View rendering for creation modes

In `DetailModel.View()` (in `detail.go`), add overlay rendering for the new modes before returning — same pattern as URL editor and date editor overlays:
```go
case detailModeSubProjectPick:
    return m.createSubProjectPicker.View()
case detailModeNewNoteName:
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.createNoteInput.View())
case detailModeNewTaskName:
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.createTaskInput.View())
case detailModeNewTaskEditor:
    return m.createTaskEditor.View()
case detailModeNewBoardPick:
    return m.createBoardPicker.View()
```

**Verify:** `go build ./...` — will still fail until Step 4 updates `app.go`.

---

### Step 4: Wire creation modes into `DetailModel.Update()` and `handleKey`

**File:** `internal/tui/projects/detail.go`

#### In `Update()` top-level switch, add alongside `noteEditorFinishedMsg`:
```go
case taskview.TextInputResultMsg:
    return m.handleTextInputResult(msg)
case taskview.TaskEditorResultMsg:
    return m.handleTaskEditorResult(msg)
case cardEditorFinishedMsg:
    return m, func() tea.Msg { return messages.DataRefreshMsg{} }
```

#### In `handleKey`, add new mode cases to the `detailMode` switch at the top:
```go
case detailModeSubProjectPick:
    return m.updateSubProjectPicker(msg)
case detailModeNewNoteName, detailModeNewTaskName:
    // TextInputModel consumes all keys; forward and let TextInputResultMsg come back via Update()
    var cmd tea.Cmd
    if m.mode == detailModeNewNoteName {
        result, c := m.createNoteInput.Update(msg)
        m.createNoteInput = result.(*taskview.TextInputModel)
        cmd = c
    } else {
        result, c := m.createTaskInput.Update(msg)
        m.createTaskInput = result.(*taskview.TextInputModel)
        cmd = c
    }
    return m, cmd
case detailModeNewTaskEditor:
    return m.updateNewTaskEditor(msg)
case detailModeNewBoardPick:
    return m.updateNewBoardPick(msg)
```

#### In `handleKey` normal mode, add:
```go
case "n":
    return m.handleNew()
```

**Verify:** `go build ./...` — still fails until `app.go` Step 5.

---

### Step 5: Update `app.go` to pass new fields

**File:** `internal/tui/app.go`

In the `OpenProjectMsg` handler (around line 211), add:
```go
allProjectItems := collectAllProjects(m.workspaces)
allContexts := taskview.ExtractUniqueContexts(ws.Tasks)
```

Update `NewDetailModel(...)` call to pass `allProjectItems` and `allContexts` as the last two arguments.

**Verify:** `go build ./...` passes cleanly.

---

### Step 6: Manual smoke test

1. Open a project with sub-projects
2. Press `n` in Notes column — sub-project picker should appear pre-selected on current row's project
3. Select a sub-project, enter filename — editor opens, save — note appears in project view
4. Press `n` in Notes column on a virtual project — error message shown, no crash
5. Press `n` in Tasks column — task name input → task editor (project pre-filled) → save — task appears
6. Press `n` in Cards column — board picker appears — select board — editor opens — card saved with project in frontmatter
7. Press `n` on project with no sub-projects — sub-project picker is skipped
8. Press `esc` at any creation step — returns to normal mode, no crash
9. In board view, move-card-to-board picker still shows "Move to Board" title

---

## File change summary

| File | Type | Description |
|------|------|-------------|
| `internal/tui/kanban/board_selector.go` | Modify | Add `title` field, update constructor + view |
| `internal/tui/projects/detail.go` | Modify | New mode constants, new fields, Update routing, handleKey routing, NewDetailModel signature, View overlays |
| `internal/tui/projects/detail_create.go` | New | All creation handlers, subproject picker, cardEditorFinishedMsg |
| `internal/tui/app.go` | Modify | Pass allProjectItems + allContexts to NewDetailModel |
