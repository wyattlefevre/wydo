# Priority Badge Styling Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace flat foreground-only priority indicators with full background-color badges across all views.

**Architecture:** Add a shared `taskPriorityStyle(p data.Priority) lipgloss.Style` helper in `shared/task_line.go`; update `kanban/priority_input.go`'s `priorityColor()` to return a full style with background; update all rendering sites to use the new styles.

**Tech Stack:** Go, charmbracelet/lipgloss (ANSI terminal styling)

---

### Task 1: Add shared task priority style helper

**Files:**
- Modify: `internal/tui/shared/task_line.go`

**Step 1: Add helper function** at the bottom of `task_line.go`:

```go
// taskPriorityStyle returns a background-badge style for a todo.txt priority (A–F).
func taskPriorityStyle(p data.Priority) lipgloss.Style {
	var bg, fg lipgloss.Color
	switch p {
	case data.PriorityA:
		bg, fg = lipgloss.Color("5"), lipgloss.Color("0")
	case data.PriorityB:
		bg, fg = lipgloss.Color("1"), lipgloss.Color("0")
	case data.PriorityC:
		bg, fg = lipgloss.Color("208"), lipgloss.Color("0")
	case data.PriorityD:
		bg, fg = lipgloss.Color("3"), lipgloss.Color("0")
	case data.PriorityE:
		bg, fg = lipgloss.Color("2"), lipgloss.Color("0")
	default: // F and beyond
		bg, fg = lipgloss.Color("8"), lipgloss.Color("15")
	}
	return lipgloss.NewStyle().Bold(true).Background(bg).Foreground(fg)
}
```

**Step 2: Update `StyledTaskLine()` priority rendering** (around line 27–33):

Replace:
```go
parts = append(parts, theme.Priority.Render("("+string(t.Priority)+")"))
```
With:
```go
parts = append(parts, taskPriorityStyle(t.Priority).Render("("+string(t.Priority)+")"))
```

**Step 3: Build to verify**

```bash
cd internal/tui/shared && go build ./...
```

**Step 4: Commit**

```bash
git add internal/tui/shared/task_line.go
git commit -m "feat: per-priority background badge style in task list"
```

---

### Task 2: Remove now-unused theme.Priority and update agenda view

**Files:**
- Modify: `internal/tui/theme/theme.go`
- Modify: `internal/tui/agenda/item_line.go`

**Step 1: Remove `Priority` from `theme.go`** (line 46):

Delete: `Priority = lipgloss.NewStyle().Bold(true).Foreground(Danger)`

**Step 2: Verify `theme.Priority` is unused**

```bash
grep -r "theme\.Priority" internal/
```
Expected: no results.

**Step 3: Update `agenda/item_line.go` — split priority badge from title**

In `RenderItemLine()`, replace the title rendering block:

```go
// Title
title := itemTitle(item)
if selected {
    parts = append(parts, selectedStyle.Render(title))
} else if item.Completed {
    parts = append(parts, completedStyle.Render(title))
} else {
    parts = append(parts, normalStyle.Render(title))
}
```

With:

```go
// Priority badge (rendered separately so it gets its own background)
if item.Source == agendapkg.SourceTask && item.Task != nil && item.Task.Priority != 0 && !item.Completed {
    badge := shared.AgendaPriorityBadge(item.Task.Priority)
    parts = append(parts, badge)
}

// Title (without priority prefix)
title := itemTitleNoPrefix(item)
if selected {
    parts = append(parts, selectedStyle.Render(title))
} else if item.Completed {
    parts = append(parts, completedStyle.Render(title))
} else {
    parts = append(parts, normalStyle.Render(title))
}
```

**Step 4: Add `AgendaPriorityBadge` to `shared/task_line.go`**

```go
// AgendaPriorityBadge returns a styled "(A)" badge for use in the agenda view.
func AgendaPriorityBadge(p data.Priority) string {
	return taskPriorityStyle(p).Render("(" + string(p) + ")")
}
```

**Step 5: Add `itemTitleNoPrefix` to `agenda/item_line.go`** (alongside `itemTitle`):

```go
// itemTitleNoPrefix returns the item title without the priority prefix.
// Used when the priority badge is rendered separately.
func itemTitleNoPrefix(item agendapkg.AgendaItem) string {
	switch item.Source {
	case agendapkg.SourceTask:
		if item.Task != nil {
			return item.Task.Name
		}
	case agendapkg.SourceCard:
		if item.Card != nil {
			return item.Card.Title
		}
	case agendapkg.SourceNote:
		if item.Note != nil {
			return item.Note.Title
		}
	}
	return ""
}
```

**Step 6: Add `shared` import to `agenda/item_line.go`** if not present.

**Step 7: Build to verify**

```bash
go build ./...
```

**Step 8: Commit**

```bash
git add internal/tui/theme/theme.go internal/tui/agenda/item_line.go internal/tui/shared/task_line.go
git commit -m "feat: per-priority background badge in agenda view; remove unused theme.Priority"
```

---

### Task 3: Update kanban priority style (input modal + card rendering)

**Files:**
- Modify: `internal/tui/kanban/priority_input.go`
- Modify: `internal/tui/kanban/board.go`

**Step 1: Replace `priorityColor()` with `kanbanPriorityStyle()` in `priority_input.go`**

Remove:
```go
func priorityColor(priority int) lipgloss.Color {
	switch priority {
	case 1:
		return theme.Accent
	case 2:
		return theme.Danger
	case 3:
		return lipgloss.Color("208")
	default:
		return theme.Warning
	}
}
```

Add:
```go
func kanbanPriorityStyle(priority int) lipgloss.Style {
	var bg, fg lipgloss.Color
	switch priority {
	case 1:
		bg, fg = lipgloss.Color("5"), lipgloss.Color("0")   // magenta
	case 2:
		bg, fg = lipgloss.Color("1"), lipgloss.Color("0")   // red
	case 3:
		bg, fg = lipgloss.Color("208"), lipgloss.Color("0") // orange
	case 4:
		bg, fg = lipgloss.Color("3"), lipgloss.Color("0")   // yellow
	case 5:
		bg, fg = lipgloss.Color("2"), lipgloss.Color("0")   // green
	default:
		bg, fg = lipgloss.Color("8"), lipgloss.Color("15")  // gray
	}
	return lipgloss.NewStyle().Bold(true).Background(bg).Foreground(fg)
}
```

**Step 2: Update `View()` in `priority_input.go`** (around line 67):

Replace:
```go
priorityStyle := lipgloss.NewStyle().Bold(true).Foreground(priorityColor(m.priority))
s.WriteString(fmt.Sprintf("Priority: %s", priorityStyle.Render(fmt.Sprintf("%d", m.priority))))
```
With:
```go
s.WriteString(fmt.Sprintf("Priority: %s", kanbanPriorityStyle(m.priority).Render(fmt.Sprintf("%d", m.priority))))
```

**Step 3: Update card rendering in `board.go`** (around line 1503):

Replace:
```go
pStyle := lipgloss.NewStyle().Bold(true).Foreground(priorityColor(card.Priority))
```
With:
```go
pStyle := kanbanPriorityStyle(card.Priority)
```

Also update the selected/move-selected overrides that follow — they currently set `.Background()` on `pStyle` to match the card bg. With the new badge style, keep those background overrides so the badge integrates with card selection highlighting:

```go
if isMoveSelected {
    pStyle = pStyle.Background(lipgloss.Color("54"))
    tStyle = tStyle.Foreground(theme.Warning).Background(lipgloss.Color("54")).Width(maxWidth - priorityPrefixWidth)
} else if isSelected {
    pStyle = pStyle.Background(theme.Surface)
    tStyle = tStyle.Background(theme.Surface)
}
```
(These lines stay the same — they correctly override the badge background to match card selection state.)

**Step 4: Build to verify**

```bash
go build ./...
```

**Step 5: Run tests**

```bash
go test ./...
```

**Step 6: Commit**

```bash
git add internal/tui/kanban/priority_input.go internal/tui/kanban/board.go
git commit -m "feat: per-priority background badge in kanban card and priority modal"
```
