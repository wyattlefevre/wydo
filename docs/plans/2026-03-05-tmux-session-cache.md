# Tmux Session Cache Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Eliminate per-card `tmux list-sessions` subprocess calls from the render path by caching the session list in `BoardModel` and refreshing it periodically in the background.

**Architecture:** Add a `tmuxSessions map[string]bool` field to `BoardModel`. On `Init()`, fire an immediate background fetch. Each time the fetch result arrives, store it and schedule the next fetch 3 seconds later. `renderCard()` reads from the cache instead of spawning a subprocess.

**Tech Stack:** Go, charmbracelet/bubbletea (`tea.Tick`, `tea.Cmd`, `tea.Batch`)

---

## Task 1: Add cache field and message type to `board.go`

**Files:**
- Modify: `wydo/internal/tui/kanban/board.go`

**Step 1: Add `tmuxSessions` field to `BoardModel`**

Find the `BoardModel` struct (line 106). Add the field after `showArchived`:

```go
showArchived           bool
tmuxSessions          map[string]bool // cached set of active tmux session names
```

**Step 2: Add the message type and two commands**

Add immediately after the `jiraStatusMsg` block (around line 241, after the `case jiraStatusMsg:` handler — put the type/func declarations near the other msg types at the top of the file, or just above `Init()`):

```go
// tmuxSessionsMsg is sent when the background tmux session list fetch completes.
type tmuxSessionsMsg struct {
	sessions []string
}

// fetchTmuxSessionsCmd fetches the tmux session list once, immediately.
func fetchTmuxSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		return tmuxSessionsMsg{sessions: listTmuxSessions()}
	}
}

// scheduleTmuxRefresh waits 3 seconds then fetches the session list.
func scheduleTmuxRefresh() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return tmuxSessionsMsg{sessions: listTmuxSessions()}
	})
}
```

Note: `listTmuxSessions()` is already defined in `tmux_picker.go` in the same package — no import needed. `time` is already imported in `board.go`; if not, add it.

**Step 3: Verify `time` is imported**

```bash
cd wydo && grep '"time"' internal/tui/kanban/board.go
```

If missing, add `"time"` to the import block.

**Step 4: Build to check for compile errors**

```bash
cd wydo && go build ./...
```

Expected: no errors.

**Step 5: Commit**

```bash
cd wydo && git add internal/tui/kanban/board.go
git commit -m "feat: add tmuxSessions cache field and refresh commands to BoardModel"
```

---

## Task 2: Wire up the cache into `Init()` and `Update()`

**Files:**
- Modify: `wydo/internal/tui/kanban/board.go`

**Step 1: Update `Init()` to fire an immediate fetch**

Find `Init()` at line 210. Change it from:

```go
func (m BoardModel) Init() tea.Cmd {
	return m.initJiraRefresh()
}
```

To:

```go
func (m BoardModel) Init() tea.Cmd {
	return tea.Batch(
		m.initJiraRefresh(),
		fetchTmuxSessionsCmd(),
	)
}
```

**Step 2: Handle `tmuxSessionsMsg` in `Update()`**

Find the `Update()` switch at line 239. Add a new case before `case tea.KeyMsg:`:

```go
case tmuxSessionsMsg:
	set := make(map[string]bool, len(msg.sessions))
	for _, s := range msg.sessions {
		set[s] = true
	}
	m.tmuxSessions = set
	return m, scheduleTmuxRefresh()
```

**Step 3: Build**

```bash
cd wydo && go build ./...
```

Expected: no errors.

**Step 4: Commit**

```bash
cd wydo && git add internal/tui/kanban/board.go
git commit -m "feat: wire tmux session cache into Init and Update with periodic refresh"
```

---

## Task 3: Use the cache in `renderCard()`

**Files:**
- Modify: `wydo/internal/tui/kanban/board.go`

**Step 1: Add `getChildSessionsFromCache()` helper**

Add this method immediately after `cardLineCount()` (around line 1902):

```go
// getChildSessionsFromCache checks which child sessions exist using the
// cached session set, avoiding a subprocess call on every render.
func (m BoardModel) getChildSessionsFromCache(root string) map[string]bool {
	children := make(map[string]bool, len(childSuffixes))
	for _, suffix := range childSuffixes {
		children[suffix] = m.tmuxSessions[root+suffix]
	}
	return children
}
```

Note: `childSuffixes` is defined in `tmux_picker.go` in the same package — it's accessible here. If `m.tmuxSessions` is nil (not yet loaded on first frame), map lookups return `false`, which is correct — the claude badge just won't show until the first fetch completes (~instant).

**Step 2: Replace `getChildSessions()` call in `renderCard()`**

Find this line inside `renderCard()` (around line 1855):

```go
hasClaudeSession := getChildSessions(card.TmuxSession)["-claude"]
```

Replace with:

```go
hasClaudeSession := m.getChildSessionsFromCache(card.TmuxSession)["-claude"]
```

**Step 3: Build**

```bash
cd wydo && go build ./...
```

Expected: no errors.

**Step 4: Smoke test**

```bash
cd wydo && go run ./cmd/wydo/
```

- Navigate to a board with tmux sessions
- Hold `j` / `k` — movement should be noticeably faster
- The claude badge (` C `) should still appear on cards that have a `-claude` child session
- After ~3 seconds, the badge state should reflect any session changes made externally

**Step 5: Commit**

```bash
cd wydo && git add internal/tui/kanban/board.go
git commit -m "perf: use cached tmux sessions in renderCard to eliminate per-card subprocess calls"
```
