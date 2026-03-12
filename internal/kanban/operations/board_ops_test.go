package operations

import (
	"os"
	"path/filepath"
	"testing"

	"wydo/internal/kanban/fs"
	"wydo/internal/kanban/models"
)

func TestSetBoardProject_SetAndClear(t *testing.T) {
	// Create a temp board directory
	dir := t.TempDir()
	cardsDir := filepath.Join(dir, "cards")
	if err := os.MkdirAll(cardsDir, 0755); err != nil {
		t.Fatal(err)
	}

	board := models.Board{
		Name:    "test-board",
		Path:    dir,
		Columns: []models.Column{{Name: "To Do", Cards: []models.Card{}}},
	}
	if err := fs.WriteBoard(board); err != nil {
		t.Fatalf("WriteBoard: %v", err)
	}

	// Set a project
	relPath := "../../projects/my-project/my-project.md"
	if err := SetBoardProject(&board, relPath); err != nil {
		t.Fatalf("SetBoardProject: %v", err)
	}
	if board.Project != relPath {
		t.Errorf("board.Project: got %q, want %q", board.Project, relPath)
	}

	// Read back from disk and confirm
	loaded, err := fs.ReadBoard(dir)
	if err != nil {
		t.Fatalf("ReadBoard: %v", err)
	}
	if loaded.Project != relPath {
		t.Errorf("loaded.Project: got %q, want %q", loaded.Project, relPath)
	}

	// Clear the project
	if err := SetBoardProject(&board, ""); err != nil {
		t.Fatalf("SetBoardProject clear: %v", err)
	}
	if board.Project != "" {
		t.Errorf("expected empty project after clear, got %q", board.Project)
	}

	loaded2, err := fs.ReadBoard(dir)
	if err != nil {
		t.Fatalf("ReadBoard after clear: %v", err)
	}
	if loaded2.Project != "" {
		t.Errorf("loaded.Project after clear: got %q, want empty", loaded2.Project)
	}
}
