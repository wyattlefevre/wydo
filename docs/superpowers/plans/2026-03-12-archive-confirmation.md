# Archive Confirmation Modal Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show a confirmation modal before archiving a project, and fix `IsTyping()` to also block global key interception during all modal modes.

**Architecture:** Add `modeArchiveConfirm` to the existing `projectMode` enum in `projects.go`, following the identical pattern used by `modeDeleteVirtual`. A single state field `archiveEntry` holds the pending project. The `"a"` key handler branches: archiving enters confirmation mode, unarchiving executes immediately (unchanged). `IsTyping()` is extended to cover both modal modes.

**Tech Stack:** Go, bubbletea TUI framework (`github.com/charmbracelet/bubbletea`), lipgloss

---

## Chunk 1: Archive confirmation modal

**Files:**
- Modify: `internal/tui/projects/projects.go`
- Create: `internal/tui/projects/projects_test.go`

---

### Task 1: Write failing tests

- [ ] **Step 1: Create test file**

Create `internal/tui/projects/projects_test.go`:

```go
package projects

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"wydo/internal/scanner"
	"wydo/internal/workspace"
)

// makeModel returns a ProjectsModel with one unarchived project (alpha) and one archived (beta).
// Uses BuildProjectRegistry with a fake scan; readProjectFrontmatter won't find the paths
// on disk and will return zero values, which is fine for state-transition tests.
func makeModel() ProjectsModel {
	scan := &scanner.WorkspaceScan{
		RootDir: "/fake",
		Projects: []scanner.ProjectInfo{
			{Name: "alpha", Path: "/fake/projects/alpha"},
			{Name: "beta", Path: "/fake/projects/beta"},
		},
	}
	reg := workspace.BuildProjectRegistry(scan, nil, nil, "")
	// Mark beta as archived manually (readProjectFrontmatter returns false for missing paths)
	if p := reg.Get("beta"); p != nil {
		p.Archived = true
	}
	ws := &workspace.Workspace{
		RootDir:  "/fake",
		Projects: reg,
	}
	return NewProjectsModel([]*workspace.Workspace{ws})
}

func pressKey(m ProjectsModel, key string) ProjectsModel {
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return m
}

func pressSpecialKey(m ProjectsModel, keyType tea.KeyType) ProjectsModel {
	m, _ = m.Update(tea.KeyMsg{Type: keyType})
	return m
}

// TestArchiveKeyEntersConfirmMode verifies pressing "a" on an unarchived project
// enters modeArchiveConfirm and stores archiveEntry without executing the archive.
func TestArchiveKeyEntersConfirmMode(t *testing.T) {
	m := makeModel()
	if len(m.filtered) == 0 {
		t.Fatal("expected at least one filtered entry")
	}
	// Find alpha (unarchived)
	idx := -1
	for i, fi := range m.filtered {
		if m.entries[fi].Project.Name == "alpha" {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatal("could not find unarchived project 'alpha' in filtered list")
	}
	m.selected = idx

	m = pressKey(m, "a")

	if m.mode != modeArchiveConfirm {
		t.Errorf("mode = %v, want modeArchiveConfirm", m.mode)
	}
	if m.archiveEntry == nil {
		t.Fatal("archiveEntry is nil, want non-nil")
	}
	if m.archiveEntry.Project.Name != "alpha" {
		t.Errorf("archiveEntry.Project.Name = %q, want %q", m.archiveEntry.Project.Name, "alpha")
	}
	if m.archiveEntry.Project.Archived {
		t.Error("project was archived before confirmation — should not archive until 'y' pressed")
	}
}

// TestUnarchiveKeyIsImmediate verifies pressing "a" on an archived project
// does NOT enter confirm mode (unarchive executes immediately).
func TestUnarchiveKeyIsImmediate(t *testing.T) {
	m := makeModel()
	m.showArchived = true
	m.buildEntries()
	m.applyFilter()

	idx := -1
	for i, fi := range m.filtered {
		if m.entries[fi].Project.Name == "beta" {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatal("archived project 'beta' not found in filtered list")
	}
	m.selected = idx

	m = pressKey(m, "a")

	if m.mode == modeArchiveConfirm {
		t.Error("mode is modeArchiveConfirm for unarchive — should execute immediately, not confirm")
	}
}

// TestArchiveConfirmCancelN verifies pressing "n" in confirm mode clears state and returns to modeList.
func TestArchiveConfirmCancelN(t *testing.T) {
	m := makeModel()
	m.selected = 0
	m = pressKey(m, "a") // enter confirm mode

	m = pressKey(m, "n")

	if m.mode != modeList {
		t.Errorf("mode = %v, want modeList after cancel", m.mode)
	}
	if m.archiveEntry != nil {
		t.Error("archiveEntry is not nil after cancel")
	}
}

// TestArchiveConfirmCancelEsc verifies pressing Esc in confirm mode clears state.
func TestArchiveConfirmCancelEsc(t *testing.T) {
	m := makeModel()
	m.selected = 0
	m = pressKey(m, "a")

	m = pressSpecialKey(m, tea.KeyEsc)

	if m.mode != modeList {
		t.Errorf("mode = %v, want modeList after esc", m.mode)
	}
	if m.archiveEntry != nil {
		t.Error("archiveEntry is not nil after esc")
	}
}

// TestIsTypingArchiveConfirm verifies IsTyping() returns true for modeArchiveConfirm
// so app-level global key interception is blocked while the modal is visible.
func TestIsTypingArchiveConfirm(t *testing.T) {
	m := makeModel()
	m.mode = modeArchiveConfirm
	if !m.IsTyping() {
		t.Error("IsTyping() = false for modeArchiveConfirm, want true")
	}
}

// TestIsTypingDeleteVirtual verifies IsTyping() also returns true for modeDeleteVirtual
// (pre-existing gap — this fixes a bug where global keys could switch views during delete confirmation).
func TestIsTypingDeleteVirtual(t *testing.T) {
	m := makeModel()
	m.mode = modeDeleteVirtual
	if !m.IsTyping() {
		t.Error("IsTyping() = false for modeDeleteVirtual, want true")
	}
}

// TestHintTextArchiveConfirm verifies the hint text for modeArchiveConfirm.
func TestHintTextArchiveConfirm(t *testing.T) {
	m := makeModel()
	m.mode = modeArchiveConfirm
	want := "y:archive  n/esc:cancel"
	if got := m.HintText(); got != want {
		t.Errorf("HintText() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /Users/wyatt/worktrees/show-confirmation-modal-on-archiving-project/wydo && go test ./internal/tui/projects/... -v 2>&1 | head -60
```

Expected: compile errors referencing `modeArchiveConfirm` and `archiveEntry` — these don't exist yet.

---

### Task 2: Add `modeArchiveConfirm` to the enum and struct

- [ ] **Step 1: Edit the `projectMode` enum**

In `internal/tui/projects/projects.go`, find:
```go
	modeDeleteVirtual   // confirm-delete a virtual project
)
```

Replace with:
```go
	modeDeleteVirtual   // confirm-delete a virtual project
	modeArchiveConfirm  // confirm-archive a project
)
```

- [ ] **Step 2: Add `archiveEntry` field to `ProjectsModel`**

Find:
```go
	// Delete virtual project flow state
	deleteEntry     *projectEntry
	deleteTaskCount int
	deleteCardCount int
```

Replace with:
```go
	// Delete virtual project flow state
	deleteEntry     *projectEntry
	deleteTaskCount int
	deleteCardCount int

	// Archive confirm flow state
	archiveEntry *projectEntry
```

- [ ] **Step 3: Run tests — expect changed failures**

```bash
cd /Users/wyatt/worktrees/show-confirmation-modal-on-archiving-project/wydo && go test ./internal/tui/projects/... -v 2>&1 | head -60
```

Expected: compiles now, but tests fail on behavior (mode stays `modeList`, `IsTyping()` returns false, etc.).

---

### Task 3: Update `IsTyping()` and `HintText()`

- [ ] **Step 1: Update `IsTyping()`**

Find:
```go
func (m ProjectsModel) IsTyping() bool {
	return m.mode == modeSearch || m.mode == modeCreate || m.mode == modeRename
}
```

Replace with:
```go
func (m ProjectsModel) IsTyping() bool {
	return m.mode == modeSearch || m.mode == modeCreate || m.mode == modeRename ||
		m.mode == modeArchiveConfirm || m.mode == modeDeleteVirtual
}
```

- [ ] **Step 2: Add `modeArchiveConfirm` case to `HintText()`**

Find:
```go
	case modeDeleteVirtual:
		return "y:delete  n/esc:cancel"
	default:
```

Replace with:
```go
	case modeDeleteVirtual:
		return "y:delete  n/esc:cancel"
	case modeArchiveConfirm:
		return "y:archive  n/esc:cancel"
	default:
```

- [ ] **Step 3: Run targeted tests**

```bash
cd /Users/wyatt/worktrees/show-confirmation-modal-on-archiving-project/wydo && go test ./internal/tui/projects/... -v -run "TestIsTyping|TestHintText" 2>&1
```

Expected: `TestIsTypingArchiveConfirm`, `TestIsTypingDeleteVirtual`, `TestHintTextArchiveConfirm` all PASS.

---

### Task 4: Update the `"a"` key handler

- [ ] **Step 1: Branch the `"a"` case in `updateList`**

Find:
```go
	case "a":
		if len(m.filtered) > 0 && m.selected < len(m.filtered) {
			entry := m.entries[m.filtered[m.selected]]
			newArchived := !entry.Project.Archived
			var err error
			if entry.Project.DirPath == "" {
				err = workspace.SetVirtualProjectArchived(entry.RootDir, entry.Project, newArchived)
			} else {
				err = workspace.SetProjectArchived(entry.Project, newArchived)
			}
			if err != nil {
				m.err = err
			} else {
				m.err = nil
				m.buildEntries()
				m.applyFilter()
			}
		}
```

Replace with:
```go
	case "a":
		if len(m.filtered) > 0 && m.selected < len(m.filtered) {
			entry := m.entries[m.filtered[m.selected]]
			if entry.Project.Archived {
				// Unarchiving: execute immediately (no confirmation needed)
				var err error
				if entry.Project.DirPath == "" {
					err = workspace.SetVirtualProjectArchived(entry.RootDir, entry.Project, false)
				} else {
					err = workspace.SetProjectArchived(entry.Project, false)
				}
				if err != nil {
					m.err = err
				} else {
					m.err = nil
					m.buildEntries()
					m.applyFilter()
				}
			} else {
				// Archiving: require confirmation
				m.archiveEntry = &entry
				m.mode = modeArchiveConfirm
				m.err = nil
			}
		}
```

- [ ] **Step 2: Run targeted tests**

```bash
cd /Users/wyatt/worktrees/show-confirmation-modal-on-archiving-project/wydo && go test ./internal/tui/projects/... -v -run "TestArchiveKey|TestUnarchiveKey" 2>&1
```

Expected: `TestArchiveKeyEntersConfirmMode` and `TestUnarchiveKeyIsImmediate` PASS.

---

### Task 5: Add `updateArchiveConfirm`, `viewArchiveConfirm`, wire into switches

- [ ] **Step 1: Add `updateArchiveConfirm` after `updateDeleteVirtual`**

Locate the end of the `updateDeleteVirtual` function (it ends with `return m, nil` then `}`). Immediately after it, add:

```go
func (m ProjectsModel) updateArchiveConfirm(msg tea.KeyMsg) (ProjectsModel, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.archiveEntry == nil {
			m.mode = modeList
			return m, nil
		}
		var err error
		if m.archiveEntry.Project.DirPath == "" {
			err = workspace.SetVirtualProjectArchived(m.archiveEntry.RootDir, m.archiveEntry.Project, true)
		} else {
			err = workspace.SetProjectArchived(m.archiveEntry.Project, true)
		}
		m.archiveEntry = nil
		m.mode = modeList
		if err != nil {
			m.err = err
		} else {
			m.err = nil
			m.buildEntries()
			m.applyFilter()
		}
	case "n", "N", "esc":
		m.archiveEntry = nil
		m.mode = modeList
	}
	return m, nil
}
```

- [ ] **Step 2: Add `viewArchiveConfirm` after `viewDeleteVirtual`**

Locate the end of `viewDeleteVirtual` (it ends with `lipgloss.Place(...)` then `}`). Immediately after it, add:

```go
func (m ProjectsModel) viewArchiveConfirm() string {
	name := ""
	if m.archiveEntry != nil {
		name = m.archiveEntry.Project.Name
	}

	var lines []string
	lines = append(lines, titleStyle.Render("Archive Project"))
	lines = append(lines, "")
	lines = append(lines, listItemStyle.Render(fmt.Sprintf("Archive %q?", name)))
	lines = append(lines, "")
	lines = append(lines, listItemStyle.Render("[y] Archive   [n/esc] Cancel"))
	if m.err != nil {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
```

- [ ] **Step 3: Wire `modeArchiveConfirm` into `Update()`**

Find:
```go
		case modeDeleteVirtual:
			return m.updateDeleteVirtual(msg)
		}
```

Replace with:
```go
		case modeDeleteVirtual:
			return m.updateDeleteVirtual(msg)
		case modeArchiveConfirm:
			return m.updateArchiveConfirm(msg)
		}
```

- [ ] **Step 4: Wire `modeArchiveConfirm` into `View()`**

Find:
```go
	case modeDeleteVirtual:
		return m.viewDeleteVirtual()
	default:
```

Replace with:
```go
	case modeDeleteVirtual:
		return m.viewDeleteVirtual()
	case modeArchiveConfirm:
		return m.viewArchiveConfirm()
	default:
```

- [ ] **Step 5: Run all projects tests**

```bash
cd /Users/wyatt/worktrees/show-confirmation-modal-on-archiving-project/wydo && go test ./internal/tui/projects/... -v 2>&1
```

Expected: all tests PASS.

- [ ] **Step 6: Build to confirm no compile errors**

```bash
cd /Users/wyatt/worktrees/show-confirmation-modal-on-archiving-project/wydo && go build ./... 2>&1
```

Expected: no output (clean build).

- [ ] **Step 7: Run full test suite**

```bash
cd /Users/wyatt/worktrees/show-confirmation-modal-on-archiving-project/wydo && go test ./... 2>&1
```

Expected: all tests PASS.

- [ ] **Step 8: Commit**

```bash
cd /Users/wyatt/worktrees/show-confirmation-modal-on-archiving-project/wydo && git add internal/tui/projects/projects.go internal/tui/projects/projects_test.go && git commit -m "$(cat <<'EOF'
feat: show confirmation modal before archiving a project

- Add modeArchiveConfirm following the modeDeleteVirtual pattern
- Archiving requires y/n confirmation; unarchiving remains immediate
- Fix IsTyping() to block global key interception during modal modes
  (covers both modeArchiveConfirm and pre-existing modeDeleteVirtual gap)
EOF
)"
```
