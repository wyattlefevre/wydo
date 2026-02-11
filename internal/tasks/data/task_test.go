package data

import (
	"testing"
)

func TestParseTask_Basic(t *testing.T) {
	task := ParseTask("Buy groceries", "id1", "todo.txt")
	if task.Name != "Buy groceries" {
		t.Errorf("expected 'Buy groceries', got %q", task.Name)
	}
	if task.Done {
		t.Error("expected not done")
	}
	if task.Priority != PriorityNone {
		t.Error("expected no priority")
	}
}

func TestParseTask_WithProjects(t *testing.T) {
	task := ParseTask("Fix login bug +alpha +webapp", "id1", "todo.txt")
	if task.Name != "Fix login bug" {
		t.Errorf("expected 'Fix login bug', got %q", task.Name)
	}
	if len(task.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(task.Projects))
	}
	if !task.HasProject("alpha") {
		t.Error("expected project alpha")
	}
	if !task.HasProject("webapp") {
		t.Error("expected project webapp")
	}
}

func TestParseTask_WithContexts(t *testing.T) {
	task := ParseTask("Call plumber @phone @home", "id1", "todo.txt")
	if !task.HasContext("phone") {
		t.Error("expected context phone")
	}
	if !task.HasContext("home") {
		t.Error("expected context home")
	}
}

func TestParseTask_WithTags(t *testing.T) {
	task := ParseTask("Buy lumber due:2026-02-15 scheduled:2026-02-12", "id1", "todo.txt")
	if task.GetDueDate() != "2026-02-15" {
		t.Errorf("expected due 2026-02-15, got %q", task.GetDueDate())
	}
	if task.GetScheduledDate() != "2026-02-12" {
		t.Errorf("expected scheduled 2026-02-12, got %q", task.GetScheduledDate())
	}
}

func TestParseTask_Completed(t *testing.T) {
	task := ParseTask("x 2026-02-01 Set up environment", "id1", "done.txt")
	if !task.Done {
		t.Error("expected done")
	}
	if task.CompletionDate != "2026-02-01" {
		t.Errorf("expected completion date 2026-02-01, got %q", task.CompletionDate)
	}
}

func TestParseTask_Priority(t *testing.T) {
	task := ParseTask("(A) Review pull requests +alpha", "id1", "todo.txt")
	if task.Priority != PriorityA {
		t.Errorf("expected priority A, got %c", task.Priority)
	}
}

func TestParseTask_Dates(t *testing.T) {
	task := ParseTask("2026-01-15 Created task", "id1", "todo.txt")
	if task.CreatedDate != "2026-01-15" {
		t.Errorf("expected created date 2026-01-15, got %q", task.CreatedDate)
	}
}

func TestParseTask_CompletedWithDates(t *testing.T) {
	task := ParseTask("x 2026-02-01 2026-01-15 Task completed", "id1", "done.txt")
	if !task.Done {
		t.Error("expected done")
	}
	if task.CompletionDate != "2026-02-01" {
		t.Errorf("expected completion 2026-02-01, got %q", task.CompletionDate)
	}
	if task.CreatedDate != "2026-01-15" {
		t.Errorf("expected created 2026-01-15, got %q", task.CreatedDate)
	}
}

func TestParseTask_StringRoundTrip(t *testing.T) {
	tests := []string{
		"(A) Review pull requests +alpha @computer due:2026-02-10",
		"x 2026-02-01 Set up dev environment +alpha @computer",
		"Buy groceries @errands",
		"(B) Fix login bug +alpha @computer",
	}

	for _, original := range tests {
		task := ParseTask(original, "id1", "todo.txt")
		result := task.String()
		if result != original {
			t.Errorf("round-trip mismatch:\n  original: %q\n  result:   %q", original, result)
		}
	}
}

func TestTask_AddRemoveProject(t *testing.T) {
	task := Task{Name: "test"}
	task.AddProject("alpha")
	if !task.HasProject("alpha") {
		t.Error("expected project alpha after add")
	}
	task.RemoveProject("alpha")
	if task.HasProject("alpha") {
		t.Error("expected project alpha removed")
	}
}

func TestTask_AddRemoveContext(t *testing.T) {
	task := Task{Name: "test"}
	task.AddContext("work")
	if !task.HasContext("work") {
		t.Error("expected context work after add")
	}
	task.RemoveContext("work")
	if task.HasContext("work") {
		t.Error("expected context work removed")
	}
}

func TestTask_SetDueDate(t *testing.T) {
	task := Task{Name: "test"}
	task.SetDueDate("2026-03-01")
	if task.GetDueDate() != "2026-03-01" {
		t.Errorf("expected due 2026-03-01, got %q", task.GetDueDate())
	}
}

func TestParseTask_QuotedTagURL(t *testing.T) {
	task := ParseTask(`Buy domain url:"https://example.com/path?q=1"`, "id1", "todo.txt")
	if task.Name != "Buy domain" {
		t.Errorf("expected name 'Buy domain', got %q", task.Name)
	}
	got := task.Tags["url"]
	if got != "https://example.com/path?q=1" {
		t.Errorf("expected URL tag value, got %q", got)
	}
}

func TestParseTask_MixedQuotedAndUnquotedTags(t *testing.T) {
	task := ParseTask(`Review site due:2026-02-15 url:"https://example.com"`, "id1", "todo.txt")
	if task.Tags["due"] != "2026-02-15" {
		t.Errorf("expected due 2026-02-15, got %q", task.Tags["due"])
	}
	if task.Tags["url"] != "https://example.com" {
		t.Errorf("expected url https://example.com, got %q", task.Tags["url"])
	}
}

func TestParseTask_QuotedTagRoundTrip(t *testing.T) {
	original := `Review site due:2026-02-15 url:"https://example.com/path?q=1"`
	task := ParseTask(original, "id1", "todo.txt")
	result := task.String()
	if result != original {
		t.Errorf("round-trip mismatch:\n  original: %q\n  result:   %q", original, result)
	}
}

func TestFormatTagValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2026-02-15", "2026-02-15"},
		{"simple", "simple"},
		{"https://example.com", `"https://example.com"`},
		{"has spaces", `"has spaces"`},
		{"with:colon", `"with:colon"`},
	}
	for _, tt := range tests {
		got := FormatTagValue(tt.input)
		if got != tt.want {
			t.Errorf("FormatTagValue(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatTagValue_AutoQuotesInSerialization(t *testing.T) {
	task := Task{
		Name: "Check site",
		Tags: map[string]string{"url": "https://example.com/page"},
	}
	result := task.String()
	expected := `Check site url:"https://example.com/page"`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFirstTagIndex_QuotedTag(t *testing.T) {
	s := `Buy domain url:"https://example.com"`
	idx := FirstTagIndex(" " + s) // prepend space since FirstTagIndex needs leading whitespace
	if idx != 12 {
		t.Errorf("expected FirstTagIndex 12, got %d", idx)
	}
}

func TestParseTask_QuotedTagNormalization(t *testing.T) {
	// key:"simple" should round-trip to key:simple (quotes stripped for simple values)
	task := ParseTask(`Do thing tag:"simple"`, "id1", "todo.txt")
	if task.Tags["tag"] != "simple" {
		t.Errorf("expected tag value 'simple', got %q", task.Tags["tag"])
	}
	result := task.String()
	expected := "Do thing tag:simple"
	if result != expected {
		t.Errorf("normalization mismatch: got %q, want %q", result, expected)
	}
}
