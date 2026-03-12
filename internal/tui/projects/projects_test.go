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
