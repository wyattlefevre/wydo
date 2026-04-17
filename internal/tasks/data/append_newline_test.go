package data

import (
	"os"
	"strings"
	"testing"
)

// TestAppendTaskToFile_NoTrailingNewline verifies that AppendTaskToFile starts
// the new task on its own line even when the existing file has no trailing newline.
func TestAppendTaskToFile_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/todo.txt"

	// Write file WITHOUT a trailing newline.
	if err := os.WriteFile(path, []byte("task one\ntask two"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := AppendTaskToFile("task three +myproject", path); err != nil {
		t.Fatalf("AppendTaskToFile: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// "task two" and "task three" must be on separate lines.
	if strings.Contains(string(content), "task twotask three") {
		t.Errorf("tasks were merged onto the same line: %q", string(content))
	}

	tasks, err := loadTaskFile(path, true, make(map[string]Project))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d; file content: %q", len(tasks), string(content))
	}
}
