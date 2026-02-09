package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCard_Frontmatter(t *testing.T) {
	cardPath := filepath.Join(testdataDir(), "workspace1", "boards", "dev-work", "cards", "auth-service.md")
	card, err := ReadCard(cardPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if card.Title != "Auth Service" {
		t.Errorf("expected title 'Auth Service', got %q", card.Title)
	}

	if len(card.Projects) != 1 || card.Projects[0] != "alpha" {
		t.Errorf("expected projects [alpha], got %v", card.Projects)
	}

	if len(card.Tags) < 1 {
		t.Error("expected at least 1 tag")
	}

	if card.DueDate == nil {
		t.Error("expected due date")
	} else if card.DueDate.Format("2006-01-02") != "2026-02-10" {
		t.Errorf("expected due 2026-02-10, got %s", card.DueDate.Format("2006-01-02"))
	}

	if card.Priority != 1 {
		t.Errorf("expected priority 1, got %d", card.Priority)
	}
}

func TestReadCard_NoFrontmatter(t *testing.T) {
	// Create a card without frontmatter
	tmpDir := t.TempDir()
	cardPath := filepath.Join(tmpDir, "simple.md")
	os.WriteFile(cardPath, []byte("# Simple Card\n\nJust some content.\n"), 0644)

	card, err := ReadCard(cardPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if card.Title != "Simple Card" {
		t.Errorf("expected 'Simple Card', got %q", card.Title)
	}
	if len(card.Tags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(card.Tags))
	}
	if len(card.Projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(card.Projects))
	}
}

func TestWriteCard_ReadCard_RoundTrip(t *testing.T) {
	cardPath := filepath.Join(testdataDir(), "workspace1", "boards", "dev-work", "cards", "auth-service.md")
	original, err := ReadCard(cardPath)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}

	// Write to temp file
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "roundtrip.md")

	if err := WriteCard(original, tmpPath); err != nil {
		t.Fatalf("write error: %v", err)
	}

	// Read back
	loaded, err := ReadCard(tmpPath)
	if err != nil {
		t.Fatalf("read-back error: %v", err)
	}

	if loaded.Title != original.Title {
		t.Errorf("title mismatch: expected %q, got %q", original.Title, loaded.Title)
	}
	if len(loaded.Projects) != len(original.Projects) {
		t.Errorf("projects count mismatch: expected %d, got %d", len(original.Projects), len(loaded.Projects))
	}
	if loaded.Priority != original.Priority {
		t.Errorf("priority mismatch: expected %d, got %d", original.Priority, loaded.Priority)
	}
	if (loaded.DueDate == nil) != (original.DueDate == nil) {
		t.Error("due date presence mismatch")
	}
	if loaded.DueDate != nil && original.DueDate != nil {
		if !loaded.DueDate.Equal(*original.DueDate) {
			t.Errorf("due date mismatch: expected %v, got %v", original.DueDate, loaded.DueDate)
		}
	}
}

func TestReadCard_ContractorQuotes(t *testing.T) {
	cardPath := filepath.Join(testdataDir(), "workspace1", "boards", "home-reno", "cards", "contractor-quotes.md")
	card, err := ReadCard(cardPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if card.Title != "Contractor Quotes" {
		t.Errorf("expected 'Contractor Quotes', got %q", card.Title)
	}

	if len(card.Projects) != 1 || card.Projects[0] != "home-remodel" {
		t.Errorf("expected projects [home-remodel], got %v", card.Projects)
	}

	if card.DueDate == nil {
		t.Error("expected due date")
	}

	if card.Priority != 2 {
		t.Errorf("expected priority 2, got %d", card.Priority)
	}
}
