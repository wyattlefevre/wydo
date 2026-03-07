package kanban

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// --- slugify ---

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Auth Service", "auth-service"},
		{"auth-service", "auth-service"},
		{"API: v2 Setup", "api-v2-setup"},
		{"  spaces  ", "spaces"},
		{"under_score", "under-score"},
		{"UPPER CASE", "upper-case"},
		{"hello--world", "hello-world"},
		{"feat/my-thing", "featmy-thing"},
	}
	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- SessionCreateModel choice step ---

func TestSessionCreateInitialState(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	if m.step != sessionCreateStepChoice {
		t.Errorf("initial step = %d, want %d (choice)", m.step, sessionCreateStepChoice)
	}
	if m.choiceCursor != 0 {
		t.Errorf("initial choiceCursor = %d, want 0 (new)", m.choiceCursor)
	}
}

func TestSessionCreateChoiceNavigation(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)

	m2, _, done := m.Handle(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if done {
		t.Fatal("should not be done after j")
	}
	if m2.choiceCursor != 1 {
		t.Errorf("after j, choiceCursor = %d, want 1", m2.choiceCursor)
	}

	m3, _, _ := m2.Handle(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m3.choiceCursor != 0 {
		t.Errorf("after k, choiceCursor = %d, want 0", m3.choiceCursor)
	}
}

func TestSessionCreateChoiceEscCancels(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	_, _, done := m.Handle(tea.KeyMsg{Type: tea.KeyEsc})
	if !done {
		t.Error("esc should return done=true")
	}
}

func TestSessionCreateNewEnterAdvancesToNameStep(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m2, _, done := m.Handle(tea.KeyMsg{Type: tea.KeyEnter})
	if done {
		t.Fatal("should not be done after enter on 'new'")
	}
	if m2.step != sessionCreateStepName {
		t.Errorf("after enter on new, step = %d, want %d (name)", m2.step, sessionCreateStepName)
	}
}

func TestSessionCreateExistingEnterAdvancesToExistingStep(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m, _, _ = m.Handle(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m2, _, done := m.Handle(tea.KeyMsg{Type: tea.KeyEnter})
	if done {
		t.Fatal("should not be done")
	}
	if m2.step != sessionCreateStepExisting {
		t.Errorf("after enter on existing, step = %d, want %d (existing)", m2.step, sessionCreateStepExisting)
	}
}

// --- existing step ---

func TestSessionCreateExistingNavigation(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m.step = sessionCreateStepExisting
	m.existingDirs = []string{"feat-alpha", "feat-beta", "feat-gamma"}
	m.existingCursor = 0

	m2, _, _ := m.Handle(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m2.existingCursor != 1 {
		t.Errorf("after j, existingCursor = %d, want 1", m2.existingCursor)
	}

	m3, _, _ := m2.Handle(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m3.existingCursor != 0 {
		t.Errorf("after k, existingCursor = %d, want 0", m3.existingCursor)
	}

	m4 := m
	m4.existingCursor = 2
	m5, _, _ := m4.Handle(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m5.existingCursor != 2 {
		t.Errorf("j at max, existingCursor = %d, want 2", m5.existingCursor)
	}
}

func TestSessionCreateExistingEscGoesBack(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m.step = sessionCreateStepExisting

	m2, _, done := m.Handle(tea.KeyMsg{Type: tea.KeyEsc})
	if done {
		t.Error("esc should go back to choice step, not close modal")
	}
	if m2.step != sessionCreateStepChoice {
		t.Errorf("after esc, step = %d, want choice (%d)", m2.step, sessionCreateStepChoice)
	}
}

func TestSessionCreateExistingEnterEmptyList(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m.step = sessionCreateStepExisting
	m.existingDirs = []string{}

	m2, _, done := m.Handle(tea.KeyMsg{Type: tea.KeyEnter})
	if done {
		t.Error("should not close on empty list")
	}
	if m2.step != sessionCreateStepExisting {
		t.Errorf("step should stay existing, got %d", m2.step)
	}
}

func TestSessionCreateExistingEnterAdvancesToProgress(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m.step = sessionCreateStepExisting
	m.existingDirs = []string{"feat-alpha", "feat-beta"}
	m.existingCursor = 1

	m2, cmd, done := m.Handle(tea.KeyMsg{Type: tea.KeyEnter})
	if done {
		t.Fatal("should not be done yet")
	}
	if m2.step != sessionCreateStepProgress {
		t.Errorf("after enter, step = %d, want progress (%d)", m2.step, sessionCreateStepProgress)
	}
	if cmd == nil {
		t.Error("expected a tea.Cmd to start creation")
	}
	if len(m2.progressLines) == 0 || m2.progressLines[0] != "Attaching to feat-beta..." {
		t.Errorf("first progress line = %q, want %q", m2.progressLines[0], "Attaching to feat-beta...")
	}
}

// --- name step ---

func TestSessionCreateNameStepDefaultSlug(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m.step = sessionCreateStepName
	if m.nameInput.Value() != "auth-service" {
		t.Errorf("nameInput value = %q, want %q", m.nameInput.Value(), "auth-service")
	}
}

func TestSessionCreateNameStepEscGoesBack(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m.step = sessionCreateStepName

	m2, _, done := m.Handle(tea.KeyMsg{Type: tea.KeyEsc})
	if done {
		t.Error("esc should go back to choice, not close modal")
	}
	if m2.step != sessionCreateStepChoice {
		t.Errorf("after esc, step = %d, want choice", m2.step)
	}
}

func TestSessionCreateNameStepEnterAdvancesToRepos(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m.step = sessionCreateStepName

	m2, _, done := m.Handle(tea.KeyMsg{Type: tea.KeyEnter})
	if done {
		t.Fatal("should not be done")
	}
	if m2.step != sessionCreateStepRepos {
		t.Errorf("after enter, step = %d, want repos (%d)", m2.step, sessionCreateStepRepos)
	}
}

// --- repos step ---

func TestSessionCreateReposNavigation(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m.step = sessionCreateStepRepos
	m.allRepos = []string{"dashboard", "opsmuxer", "infra"}
	m.repoFiltered = m.allRepos
	m.repoCursor = 0

	m2, _, _ := m.Handle(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m2.repoCursor != 1 {
		t.Errorf("after j, repoCursor = %d, want 1", m2.repoCursor)
	}
}

func TestSessionCreateReposToggle(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m.step = sessionCreateStepRepos
	m.allRepos = []string{"dashboard", "opsmuxer"}
	m.repoFiltered = m.allRepos
	m.repoCursor = 0

	m2, _, _ := m.Handle(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if !m2.selectedRepos["dashboard"] {
		t.Error("space should select 'dashboard'")
	}

	m3, _, _ := m2.Handle(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if m3.selectedRepos["dashboard"] {
		t.Error("second space should deselect 'dashboard'")
	}
}

func TestSessionCreateReposEnterWithNoneSelectedDoesNothing(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m.step = sessionCreateStepRepos
	m.allRepos = []string{"dashboard"}
	m.repoFiltered = m.allRepos

	m2, _, done := m.Handle(tea.KeyMsg{Type: tea.KeyEnter})
	if done {
		t.Error("enter with no selection should not close")
	}
	if m2.step != sessionCreateStepRepos {
		t.Errorf("step should stay repos, got %d", m2.step)
	}
}

func TestSessionCreateReposEnterWithSelectionAdvancesToProgress(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m.step = sessionCreateStepRepos
	m.allRepos = []string{"dashboard", "opsmuxer"}
	m.repoFiltered = m.allRepos
	m.selectedRepos["dashboard"] = true
	m.nameInput.SetValue("feat-auth")

	m2, cmd, done := m.Handle(tea.KeyMsg{Type: tea.KeyEnter})
	if done {
		t.Fatal("should not be done yet")
	}
	if m2.step != sessionCreateStepProgress {
		t.Errorf("after enter, step = %d, want progress (%d)", m2.step, sessionCreateStepProgress)
	}
	if cmd == nil {
		t.Error("expected a tea.Cmd to start creation")
	}
}

func TestSessionCreateReposEscGoesBack(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m.step = sessionCreateStepRepos

	m2, _, _ := m.Handle(tea.KeyMsg{Type: tea.KeyEsc})
	if m2.step != sessionCreateStepName {
		t.Errorf("after esc, step = %d, want name (%d)", m2.step, sessionCreateStepName)
	}
}

func TestSessionCreateChoiceNoOvershoot(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m.choiceCursor = 1
	// j at max should not go to 2
	m2, _, _ := m.Handle(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m2.choiceCursor != 1 {
		t.Errorf("j at max, choiceCursor = %d, want 1", m2.choiceCursor)
	}
}

func TestSessionCreateReposProgressLineContent(t *testing.T) {
	m := NewSessionCreateModel("feat-auth", 80, 24)
	m.step = sessionCreateStepRepos
	m.allRepos = []string{"dashboard"}
	m.repoFiltered = m.allRepos
	m.selectedRepos["dashboard"] = true
	m.nameInput.SetValue("feat-auth")

	m2, _, _ := m.Handle(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m2.progressLines) == 0 {
		t.Fatal("expected progress lines")
	}
	want := "Creating worktree feat-auth..."
	if m2.progressLines[0] != want {
		t.Errorf("progressLines[0] = %q, want %q", m2.progressLines[0], want)
	}
}

func TestSessionCreateFilterRepos(t *testing.T) {
	m := NewSessionCreateModel("x", 80, 24)
	m.allRepos = []string{"dashboard", "opsmuxer", "infra"}
	m.repoFiltered = m.allRepos
	m.repoCursor = 2

	// Filter to match only "dashboard"
	m.repoFilterText = "dash"
	m2 := m.filterRepos()
	if len(m2.repoFiltered) != 1 || m2.repoFiltered[0] != "dashboard" {
		t.Errorf("filter 'dash': got %v, want [dashboard]", m2.repoFiltered)
	}
	// Cursor should clamp to 0 (was 2, only 1 result now)
	if m2.repoCursor != 0 {
		t.Errorf("cursor should clamp to 0 after filter, got %d", m2.repoCursor)
	}

	// Clear filter resets to full list
	m2.repoFilterText = ""
	m3 := m2.filterRepos()
	if len(m3.repoFiltered) != 3 {
		t.Errorf("empty filter: got %d items, want 3", len(m3.repoFiltered))
	}
}

func TestSessionCreateReposDeselectRemovesFromMap(t *testing.T) {
	m := NewSessionCreateModel("Auth Service", 80, 24)
	m.step = sessionCreateStepRepos
	m.allRepos = []string{"dashboard", "opsmuxer"}
	m.repoFiltered = m.allRepos
	m.repoCursor = 0

	// select
	m2, _, _ := m.Handle(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if !m2.selectedRepos["dashboard"] {
		t.Fatal("should be selected after first space")
	}

	// deselect
	m3, _, _ := m2.Handle(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if _, exists := m3.selectedRepos["dashboard"]; exists {
		t.Error("deselected repo should be removed from map, not set to false")
	}
	if len(m3.selectedRepos) != 0 {
		t.Errorf("map should be empty after deselect, got len=%d", len(m3.selectedRepos))
	}

	// enter with empty selection should do nothing
	m4, cmd, done := m3.Handle(tea.KeyMsg{Type: tea.KeyEnter})
	if done || cmd != nil || m4.step != sessionCreateStepRepos {
		t.Error("enter with all deselected should be a no-op")
	}
}
