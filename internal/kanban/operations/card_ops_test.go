package operations

import "testing"

func TestTaskPriorityToTaskNotePriority(t *testing.T) {
	tests := []struct {
		input rune
		want  int
	}{
		{'A', 1},
		{'B', 2},
		{'C', 3},
		{'D', 4},
		{'E', 5},
		{'F', 6},
		// no priority
		{0, 0},
		// out of range
		{'G', 0},
		{'Z', 0},
		{'a', 0}, // lowercase not supported in todo.txt format
	}
	for _, tt := range tests {
		got := TaskPriorityToTaskNotePriority(tt.input)
		if got != tt.want {
			t.Errorf("TaskPriorityToTaskNotePriority(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
