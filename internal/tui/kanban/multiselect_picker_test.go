package kanban

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestSingleSelectPicker(selected string, items []string) MultiSelectPickerModel {
	sel := make(map[string]bool)
	if selected != "" {
		sel[selected] = true
	}
	return NewMultiSelectPickerModel(MultiSelectPickerConfig{
		Title:            "Test",
		ItemTypeSingular: "item",
		SanitizeFunc:     func(s string) string { return s },
		AllItems:         items,
		SelectedItems:    sel,
		SingleSelect:     true,
	})
}

func pressKey(m MultiSelectPickerModel, key string) (MultiSelectPickerModel, tea.Cmd, bool, bool) {
	return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

func pressSpecialKey(m MultiSelectPickerModel, keyType tea.KeyType) (MultiSelectPickerModel, tea.Cmd, bool, bool) {
	return m.Update(tea.KeyMsg{Type: keyType})
}

func TestSingleSelect_EnterSelectsAndCloses(t *testing.T) {
	m := newTestSingleSelectPicker("", []string{"alpha", "beta", "gamma"})
	// cursor starts at 0 ("alpha"), press enter
	got, _, isDone, cancelled := pressSpecialKey(m, tea.KeyEnter)
	if !isDone {
		t.Fatal("expected isDone=true after enter in single-select mode")
	}
	if cancelled {
		t.Fatal("expected cancelled=false after selecting item")
	}
	selected := got.GetSelectedItems()
	if len(selected) != 1 || selected[0] != "alpha" {
		t.Fatalf("expected [alpha], got %v", selected)
	}
}

func TestSingleSelect_EnterOnSelectedClears(t *testing.T) {
	m := newTestSingleSelectPicker("alpha", []string{"alpha", "beta"})
	// cursor at 0 = "alpha" which is already selected; enter should clear and close
	_, _, isDone, cancelled := pressSpecialKey(m, tea.KeyEnter)
	if !isDone {
		t.Fatal("expected isDone=true")
	}
	if cancelled {
		t.Fatal("expected cancelled=false (unlink path)")
	}
}

func TestSingleSelect_SpaceBehavesLikeEnter(t *testing.T) {
	m := newTestSingleSelectPicker("", []string{"alpha", "beta"})
	_, _, isDone, cancelled := pressKey(m, " ")
	if !isDone {
		t.Fatal("expected isDone=true after space in single-select mode")
	}
	if cancelled {
		t.Fatal("expected cancelled=false")
	}
}

func TestSingleSelect_EscReturnsCancelled(t *testing.T) {
	m := newTestSingleSelectPicker("", []string{"alpha", "beta"})
	_, _, isDone, cancelled := pressSpecialKey(m, tea.KeyEsc)
	if !isDone {
		t.Fatal("expected isDone=true after esc")
	}
	if !cancelled {
		t.Fatal("expected cancelled=true after esc")
	}
}

func TestMultiSelect_CancelledFalseOnNormalEnter(t *testing.T) {
	// Existing multi-select mode: enter returns cancelled=false
	m := NewMultiSelectPickerModel(MultiSelectPickerConfig{
		Title:            "Test",
		ItemTypeSingular: "item",
		SanitizeFunc:     func(s string) string { return s },
		AllItems:         []string{"alpha"},
		SelectedItems:    map[string]bool{},
	})
	_, _, isDone, cancelled := pressSpecialKey(m, tea.KeyEnter)
	if !isDone {
		t.Fatal("expected isDone=true on enter")
	}
	if cancelled {
		t.Fatal("expected cancelled=false on normal enter")
	}
}

func TestMultiSelect_CancelledTrueOnEscNoFilter(t *testing.T) {
	m := NewMultiSelectPickerModel(MultiSelectPickerConfig{
		Title:            "Test",
		ItemTypeSingular: "item",
		SanitizeFunc:     func(s string) string { return s },
		AllItems:         []string{"alpha"},
		SelectedItems:    map[string]bool{},
	})
	_, _, isDone, cancelled := pressSpecialKey(m, tea.KeyEsc)
	if !isDone {
		t.Fatal("expected isDone=true on esc with no filter")
	}
	if !cancelled {
		t.Fatal("expected cancelled=true on esc with no filter")
	}
}
