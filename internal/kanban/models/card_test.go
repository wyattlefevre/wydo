package models

import "testing"

func TestCardPriorityLabel(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{1, "A"},
		{2, "B"},
		{3, "C"},
		{4, "D"},
		{5, "E"},
		{6, "F"},
		{0, ""},
		{7, ""},
		{-1, ""},
	}
	for _, tt := range tests {
		got := CardPriorityLabel(tt.input)
		if got != tt.want {
			t.Errorf("CardPriorityLabel(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCardPriorityFromLetter(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"A", 1},
		{"B", 2},
		{"C", 3},
		{"D", 4},
		{"E", 5},
		{"F", 6},
		// case-insensitive
		{"a", 1},
		{"f", 6},
		// out of range
		{"G", 0},
		{"Z", 0},
		// invalid
		{"", 0},
		{"AB", 0},
		{"1", 0},
	}
	for _, tt := range tests {
		got := CardPriorityFromLetter(tt.input)
		if got != tt.want {
			t.Errorf("CardPriorityFromLetter(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestCardPriorityRoundTrip(t *testing.T) {
	// CardPriorityLabel -> CardPriorityFromLetter must be identity for 1-6
	for p := 1; p <= 6; p++ {
		label := CardPriorityLabel(p)
		got := CardPriorityFromLetter(label)
		if got != p {
			t.Errorf("round-trip failed for %d: label=%q, back=%d", p, label, got)
		}
	}
}
