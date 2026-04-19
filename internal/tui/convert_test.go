package tui

import (
	"testing"
)

func TestMergeProjects(t *testing.T) {
	t.Run("empty base and extra", func(t *testing.T) {
		result := mergeProjects(nil, nil)
		if len(result) != 0 {
			t.Errorf("expected empty, got %v", result)
		}
	})

	t.Run("no overlap combines both sets", func(t *testing.T) {
		result := mergeProjects([]string{"alpha", "beta"}, []string{"gamma", "delta"})
		if len(result) != 4 {
			t.Errorf("expected 4 items, got %v", result)
		}
	})

	t.Run("full overlap case-insensitive adds no duplicates", func(t *testing.T) {
		result := mergeProjects([]string{"Alpha", "Beta"}, []string{"alpha", "BETA"})
		if len(result) != 2 {
			t.Errorf("expected 2 items, got %v", result)
		}
	})

	t.Run("partial overlap adds only new items", func(t *testing.T) {
		result := mergeProjects([]string{"alpha", "beta"}, []string{"Beta", "gamma"})
		if len(result) != 3 {
			t.Errorf("expected 3 items, got %v", result)
		}
		found := false
		for _, r := range result {
			if r == "gamma" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected gamma in result, got %v", result)
		}
	})

	t.Run("original base slice is not modified", func(t *testing.T) {
		base := []string{"alpha", "beta"}
		baseCopy := []string{"alpha", "beta"}
		mergeProjects(base, []string{"gamma", "delta"})
		for i, v := range base {
			if v != baseCopy[i] {
				t.Errorf("base slice was modified at index %d: got %q, want %q", i, v, baseCopy[i])
			}
		}
		if len(base) != len(baseCopy) {
			t.Errorf("base slice length changed: got %d, want %d", len(base), len(baseCopy))
		}
	})
}
