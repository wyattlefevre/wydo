# Parent/Child Project Switching Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `[` and `]` key bindings to the project detail view that navigate directly to the parent project or open a child project picker.

**Architecture:** Add a `detailChildPicker` widget to `detail.go` (mirroring the existing `projectURLPicker` pattern), wire it into `DetailModel`'s mode dispatch, then register the `ViewProjectDetail` help section in `app.go`.

**Tech Stack:** Go, bubbletea (charmbracelet/bubbletea), lipgloss

---

## Chunk 1: Child picker widget and detail view integration

### Task 1: Add `detailModeChildPicker` and `detailChildPicker` to `detail.go`

**Files:**
- Modify: `internal/tui/projects/detail.go`

**Context:** `detail.go` uses a `detailMode int` enum (currently has `detailModeNormal`, `detailModeURLEditor`, `detailModeURLPicker`, `detailModeDateEditor`). The `projectURLPicker` struct in the same file is the exact pattern to mirror.

- [ ] **Step 1: Add the new mode constant**

In `detail.go`, find the `detailMode` const block:

```go
const (
	detailModeNormal    detailMode = iota
	detailModeURLEditor            // editing project URLs
	detailModeURLPicker            // picking a URL to open
	detailModeDateEditor           // editing project dates
)
```

Add `detailModeChildPicker` at the end:

```go
const (
	detailModeNormal    detailMode = iota
	detailModeURLEditor            // editing project URLs
	detailModeURLPicker            // picking a URL to open
	detailModeDateEditor           // editing project dates
	detailModeChildPicker          // picking a child project to open
)
```

- [ ] **Step 2: Add the `detailChildPicker` struct**

After the closing `}` of the `projectURLPicker` type (around line 134), add:

```go
// detailChildPicker is an inline picker for selecting a child project.
type detailChildPicker struct {
	entries []*workspace.Project
	cursor  int
	width   int
	height  int
}

func (p detailChildPicker) Update(msg tea.KeyMsg) (detailChildPicker, *workspace.Project, bool) {
	switch msg.String() {
	case "j", "down":
		if p.cursor < len(p.entries)-1 {
			p.cursor++
		}
	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
		}
	case "enter":
		if len(p.entries) > 0 && p.cursor < len(p.entries) {
			return p, p.entries[p.cursor], true
		}
		return p, nil, true
	case "esc":
		return p, nil, true
	}
	return p, nil, false
}

func (p detailChildPicker) View() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Switch to Child Project"))
	lines = append(lines, "")

	for i, proj := range p.entries {
		style := listItemStyle
		prefix := "  "
		if i == p.cursor {
			style = selectedDetailItemStyle
			prefix = "> "
		}
		lines = append(lines, style.Render(prefix+proj.Name))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, content)
}
```

- [ ] **Step 3: Verify it compiles**

```bash
cd /Users/wyatt/worktrees/parentchild-project-switching/wydo
go build ./internal/tui/projects/...
```

Expected: no output, exit 0.

---

### Task 2: Add `childPicker` field to `DetailModel` and wire it into `Update`

**Files:**
- Modify: `internal/tui/projects/detail.go`

- [ ] **Step 1: Add the field to `DetailModel`**

Find the `DetailModel` struct. Add `childPicker *detailChildPicker` alongside the existing picker fields (`urlEditor`, `urlPicker`, `dateEditor`):

```go
// Modal state
mode       detailMode
urlEditor  *kanban.URLEditorModel
urlPicker  *projectURLPicker
dateEditor *DateEditorModel
childPicker *detailChildPicker
```

- [ ] **Step 2: Add dispatch in `DetailModel.Update`**

In `DetailModel.Update`, the existing switch dispatches on mode before calling `handleKey`. Add the child picker case:

```go
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case detailModeURLEditor:
			return m.updateURLEditor(msg)
		case detailModeURLPicker:
			return m.updateURLPicker(msg)
		case detailModeDateEditor:
			return m.updateDateEditor(msg)
		case detailModeChildPicker:
			return m.updateChildPicker(msg)
		}
		return m.handleKey(msg)
	case noteEditorFinishedMsg:
		return m, func() tea.Msg { return messages.DataRefreshMsg{} }
	}
	return m, nil
}
```

- [ ] **Step 3: Add the `updateChildPicker` method**

Add this method to `DetailModel` (near the other `update*` methods):

```go
func (m DetailModel) updateChildPicker(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	if m.childPicker == nil {
		m.mode = detailModeNormal
		return m, nil
	}
	picker, selected, done := m.childPicker.Update(msg)
	m.childPicker = &picker
	if done {
		m.mode = detailModeNormal
		m.childPicker = nil
		if selected != nil {
			wsDir := m.wsDir
			return m, func() tea.Msg {
				return messages.OpenProjectMsg{
					ProjectName:      selected.Name,
					WorkspaceRootDir: wsDir,
				}
			}
		}
	}
	return m, nil
}
```

- [ ] **Step 4: Add child picker rendering in `View`**

In `DetailModel.View`, insert one new guard after the existing three at the top of the function. The existing code starts:

```go
func (m DetailModel) View() string {
	if m.mode == detailModeURLEditor && m.urlEditor != nil {
		return m.urlEditor.View()
	}
	if m.mode == detailModeURLPicker && m.urlPicker != nil {
		return m.urlPicker.View()
	}
	if m.mode == detailModeDateEditor && m.dateEditor != nil {
		return m.dateEditor.View()
	}

	var lines []string
```

Insert the new guard between the `dateEditor` check and `var lines []string`, so it reads:

```go
	if m.mode == detailModeDateEditor && m.dateEditor != nil {
		return m.dateEditor.View()
	}
	if m.mode == detailModeChildPicker && m.childPicker != nil {
		return m.childPicker.View()
	}

	var lines []string
```

Everything else in `View` is unchanged.

- [ ] **Step 5: Verify it compiles**

```bash
cd /Users/wyatt/worktrees/parentchild-project-switching/wydo
go build ./internal/tui/projects/...
```

Expected: no output, exit 0.

---

### Task 3: Add `[` and `]` key handlers in `handleKey`

**Files:**
- Modify: `internal/tui/projects/detail.go`

- [ ] **Step 1: Add the `[` handler**

In `handleKey`, add a case for `[` (after the existing `"d"` case is a natural spot):

```go
case "[":
	if m.project == nil || m.project.Parent == "" {
		return m, nil
	}
	if m.registry == nil {
		return m, nil
	}
	parent := m.registry.Get(m.project.Parent)
	if parent == nil {
		return m, nil
	}
	wsDir := m.wsDir
	parentName := parent.Name
	return m, func() tea.Msg {
		return messages.OpenProjectMsg{
			ProjectName:      parentName,
			WorkspaceRootDir: wsDir,
		}
	}
```

- [ ] **Step 2: Add the `]` handler**

In `handleKey`, add a case for `]` immediately after `[`:

```go
case "]":
	if m.registry == nil {
		return m, nil
	}
	children := m.registry.ChildrenOf(m.name)
	if len(children) == 0 {
		return m, nil
	}
	sort.Slice(children, func(i, j int) bool {
		return strings.ToLower(children[i].Name) < strings.ToLower(children[j].Name)
	})
	picker := detailChildPicker{
		entries: children,
		cursor:  0,
		width:   m.width,
		height:  m.height,
	}
	m.childPicker = &picker
	m.mode = detailModeChildPicker
	return m, nil
```

Note: `sort` and `strings` are already imported in `detail.go`.

- [ ] **Step 3: Update `HintText`**

Find `HintText` in `detail.go`:

```go
func (m DetailModel) HintText() string {
	switch m.mode {
	case detailModeURLEditor:
		return "n:add  d:delete  e:edit url  l:edit label  enter:save  esc:cancel"
	case detailModeURLPicker:
		return "j/k:navigate  /: search  enter:open  esc:cancel"
	case detailModeDateEditor:
		return "n:add  d:delete  e:edit label  D:edit date  enter:save  esc:cancel"
	}
	return "h/l:columns  j/k:navigate  space/enter:expand  enter:open  u:urls  d:dates  esc:back"
}
```

Update to:

```go
func (m DetailModel) HintText() string {
	switch m.mode {
	case detailModeURLEditor:
		return "n:add  d:delete  e:edit url  l:edit label  enter:save  esc:cancel"
	case detailModeURLPicker:
		return "j/k:navigate  /: search  enter:open  esc:cancel"
	case detailModeDateEditor:
		return "n:add  d:delete  e:edit label  D:edit date  enter:save  esc:cancel"
	case detailModeChildPicker:
		return "j/k:navigate  enter:open  esc:cancel"
	}
	return "h/l:columns  j/k:navigate  space/enter:expand  enter:open  u:urls  d:dates  [:parent  ]:children  esc:back"
}
```

- [ ] **Step 4: Build and test**

```bash
cd /Users/wyatt/worktrees/parentchild-project-switching/wydo
go build ./...
go test ./...
```

Expected: build succeeds, all tests pass (same output as baseline).

- [ ] **Step 5: Commit**

```bash
cd /Users/wyatt/worktrees/parentchild-project-switching/wydo
git add internal/tui/projects/detail.go
git commit -m "feat: add parent/child project navigation in detail view ([/] keys)"
```

---

## Chunk 2: Help overlay

### Task 4: Add `ViewProjectDetail` section to the `?` help overlay in `app.go`

**Files:**
- Modify: `internal/tui/app.go`

**Context:** `renderHelpOverlay` in `app.go` has a `switch m.currentView` block with per-view `shared.HelpSection` entries. `ViewProjectDetail` is currently missing from this switch. The `shared.HelpSection` and `shared.HelpBind` types are used by all existing sections.

- [ ] **Step 1: Add the `ViewProjectDetail` case**

In `renderHelpOverlay`, find the closing brace of the `case ViewProjects:` block. Add the new case immediately after it (before `}`):

```go
case ViewProjectDetail:
	sections = append(sections, shared.HelpSection{
		Title: "Project Detail",
		Binds: []shared.HelpBind{
			{"[", "Go to parent project"},
			{"]", "Pick child project"},
			{"h / l", "Navigate columns"},
			{"j / k", "Navigate items"},
			{"space / enter", "Expand / open item"},
			{"u", "Open URL(s)"},
			{"U", "Edit URLs"},
			{"d", "Edit dates"},
			{"esc / q", "Back to projects"},
		},
	})
```

- [ ] **Step 2: Build and test**

```bash
cd /Users/wyatt/worktrees/parentchild-project-switching/wydo
go build ./...
go test ./...
```

Expected: build succeeds, all tests pass.

- [ ] **Step 3: Commit**

```bash
cd /Users/wyatt/worktrees/parentchild-project-switching/wydo
git add internal/tui/app.go
git commit -m "feat: add project detail section to help overlay"
```
