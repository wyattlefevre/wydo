package fs

import (
	"os"
	"path/filepath"
	"testing"
	"wydo/internal/kanban/models"
)

func testdataDir() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..", "..", "testdata")
}

func TestReadBoard_Statuses(t *testing.T) {
	boardPath := filepath.Join(testdataDir(), "workspace1", "boards", "dev-work.txt")
	board, err := ReadBoard(boardPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if board.Name != "dev-work" {
		t.Errorf("expected board name 'dev-work', got %q", board.Name)
	}
	expectedStatuses := []string{"To Do", "In Progress"}
	if len(board.Statuses) != len(expectedStatuses) {
		t.Fatalf("expected %d statuses, got %d: %v", len(expectedStatuses), len(board.Statuses), board.Statuses)
	}
	for i, expected := range expectedStatuses {
		if board.Statuses[i] != expected {
			t.Errorf("status %d: expected %q, got %q", i, expected, board.Statuses[i])
		}
	}
}

func TestReadBoard_NoColumns(t *testing.T) {
	boardPath := filepath.Join(testdataDir(), "workspace1", "boards", "dev-work.txt")
	board, err := ReadBoard(boardPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(board.Columns) != 0 {
		t.Errorf("expected 0 columns from ReadBoard alone, got %d", len(board.Columns))
	}
}

func TestWriteBoard_ReadBoard_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	boardPath := filepath.Join(tmpDir, "test-board.txt")

	board := models.Board{
		Name:     "test-board",
		Path:     boardPath,
		Statuses: []string{"To Do", "In Progress"},
		Columns: []models.Column{
			{Name: "To Do", TaskNotes: []models.TaskNote{}},
			{Name: "In Progress", TaskNotes: []models.TaskNote{}},
			{Name: "Done", TaskNotes: []models.TaskNote{}},
		},
	}
	if err := WriteBoard(board); err != nil {
		t.Fatalf("write error: %v", err)
	}

	loaded, err := ReadBoard(boardPath)
	if err != nil {
		t.Fatalf("read-back error: %v", err)
	}
	if loaded.Name != "test-board" {
		t.Errorf("expected name 'test-board', got %q", loaded.Name)
	}
	if len(loaded.Statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d: %v", len(loaded.Statuses), loaded.Statuses)
	}
	if loaded.Statuses[0] != "To Do" || loaded.Statuses[1] != "In Progress" {
		t.Errorf("status mismatch: got %v", loaded.Statuses)
	}
	// "Done" should NOT be written to the file (it's implicit)
}

func TestReadBoardFull_WithCards(t *testing.T) {
	wsRoot := filepath.Join(testdataDir(), "workspace1")
	boardPath := filepath.Join(wsRoot, "boards", "dev-work.txt")
	board, err := ReadBoardFull(boardPath, wsRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedCols := []string{"To Do", "In Progress", "Done"}
	if len(board.Columns) != len(expectedCols) {
		t.Fatalf("expected %d columns, got %d", len(expectedCols), len(board.Columns))
	}
	for i, expected := range expectedCols {
		if board.Columns[i].Name != expected {
			t.Errorf("column %d: expected %q, got %q", i, expected, board.Columns[i].Name)
		}
	}

	todoCol := board.Columns[0]
	if len(todoCol.TaskNotes) != 1 || todoCol.TaskNotes[0].Title != "DB Migration" {
		t.Errorf("expected 1 card 'DB Migration' in To Do, got %d cards", len(todoCol.TaskNotes))
	}
	inProgressCol := board.Columns[1]
	if len(inProgressCol.TaskNotes) != 1 || inProgressCol.TaskNotes[0].Title != "Auth Service" {
		t.Errorf("expected 1 card 'Auth Service' in In Progress, got %d cards", len(inProgressCol.TaskNotes))
	}
	doneCol := board.Columns[2]
	if len(doneCol.TaskNotes) != 1 || doneCol.TaskNotes[0].Title != "Deploy v2" {
		t.Errorf("expected 1 card 'Deploy v2' in Done, got %d cards", len(doneCol.TaskNotes))
	}
}

func TestReadBoardFull_CardFrontmatter(t *testing.T) {
	wsRoot := filepath.Join(testdataDir(), "workspace1")
	boardPath := filepath.Join(wsRoot, "boards", "dev-work.txt")
	board, err := ReadBoardFull(boardPath, wsRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	card := board.Columns[1].TaskNotes[0] // In Progress -> Auth Service
	if len(card.Tags) == 0 {
		t.Error("expected tags on auth-service card")
	}
	if len(card.Projects) == 0 || card.Projects[0] != "alpha" {
		t.Errorf("expected projects [alpha], got %v", card.Projects)
	}
	if card.DueDate == nil {
		t.Error("expected due date")
	}
	if card.Priority != 1 {
		t.Errorf("expected priority 1, got %d", card.Priority)
	}
}
