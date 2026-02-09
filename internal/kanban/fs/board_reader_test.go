package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func testdataDir() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..", "..", "testdata")
}

func TestReadBoard_Columns(t *testing.T) {
	boardPath := filepath.Join(testdataDir(), "workspace1", "boards", "dev-work")
	board, err := ReadBoard(boardPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if board.Name != "dev-work" {
		t.Errorf("expected board name 'dev-work', got %q", board.Name)
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
}

func TestReadBoard_Cards(t *testing.T) {
	boardPath := filepath.Join(testdataDir(), "workspace1", "boards", "dev-work")
	board, err := ReadBoard(boardPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// To Do column should have db-migration
	todoCol := board.Columns[0]
	if len(todoCol.Cards) != 1 {
		t.Fatalf("expected 1 card in To Do, got %d", len(todoCol.Cards))
	}
	if todoCol.Cards[0].Title != "DB Migration" {
		t.Errorf("expected 'DB Migration', got %q", todoCol.Cards[0].Title)
	}

	// In Progress should have auth-service
	inProgressCol := board.Columns[1]
	if len(inProgressCol.Cards) != 1 {
		t.Fatalf("expected 1 card in In Progress, got %d", len(inProgressCol.Cards))
	}
	if inProgressCol.Cards[0].Title != "Auth Service" {
		t.Errorf("expected 'Auth Service', got %q", inProgressCol.Cards[0].Title)
	}
}

func TestReadBoard_CardFrontmatter(t *testing.T) {
	boardPath := filepath.Join(testdataDir(), "workspace1", "boards", "dev-work")
	board, err := ReadBoard(boardPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Auth Service card should have frontmatter
	card := board.Columns[1].Cards[0] // In Progress -> Auth Service
	if len(card.Tags) == 0 {
		t.Error("expected tags on auth-service card")
	}
	if len(card.Projects) == 0 {
		t.Error("expected projects on auth-service card")
	}
	if card.Projects[0] != "alpha" {
		t.Errorf("expected project 'alpha', got %q", card.Projects[0])
	}
	if card.DueDate == nil {
		t.Error("expected due date on auth-service card")
	}
	if card.Priority != 1 {
		t.Errorf("expected priority 1, got %d", card.Priority)
	}
}

func TestWriteBoard_ReadBoard_RoundTrip(t *testing.T) {
	boardPath := filepath.Join(testdataDir(), "workspace1", "boards", "dev-work")
	original, err := ReadBoard(boardPath)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}

	// Write to temp dir
	tmpDir := t.TempDir()
	tmpBoardPath := filepath.Join(tmpDir, "test-board")
	os.MkdirAll(filepath.Join(tmpBoardPath, "cards"), 0755)

	// Copy card files
	for _, col := range original.Columns {
		for _, card := range col.Cards {
			srcPath := filepath.Join(boardPath, "cards", card.Filename)
			dstPath := filepath.Join(tmpBoardPath, "cards", card.Filename)
			content, _ := os.ReadFile(srcPath)
			os.WriteFile(dstPath, content, 0644)
		}
	}

	// Write board
	original.Path = tmpBoardPath
	if err := WriteBoard(original); err != nil {
		t.Fatalf("write error: %v", err)
	}

	// Read back
	loaded, err := ReadBoard(tmpBoardPath)
	if err != nil {
		t.Fatalf("read-back error: %v", err)
	}

	if loaded.Name != original.Name {
		t.Errorf("expected name %q, got %q", original.Name, loaded.Name)
	}
	if len(loaded.Columns) != len(original.Columns) {
		t.Fatalf("expected %d columns, got %d", len(original.Columns), len(loaded.Columns))
	}
}
