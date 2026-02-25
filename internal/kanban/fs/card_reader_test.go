package fs

import (
	"os"
	"path/filepath"
	"testing"
	"wydo/internal/kanban/models"
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

func TestParseFrontmatter_LegacyURL(t *testing.T) {
	content := []byte(`---
url: "https://example.com"
---

# Legacy URL Card
`)

	result, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.URLs) != 1 {
		t.Fatalf("expected 1 URL, got %d", len(result.URLs))
	}
	if result.URLs[0].URL != "https://example.com" {
		t.Errorf("expected URL 'https://example.com', got %q", result.URLs[0].URL)
	}
	if result.URLs[0].Label != "" {
		t.Errorf("expected empty label, got %q", result.URLs[0].Label)
	}
}

func TestParseFrontmatter_MultiURL(t *testing.T) {
	content := []byte(`---
urls:
  - label: "GitHub Repo"
    url: "https://github.com/example/repo"
  - url: "https://docs.example.com"
---

# Multi URL Card
`)

	result, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.URLs) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(result.URLs))
	}

	if result.URLs[0].Label != "GitHub Repo" {
		t.Errorf("expected label 'GitHub Repo', got %q", result.URLs[0].Label)
	}
	if result.URLs[0].URL != "https://github.com/example/repo" {
		t.Errorf("expected URL 'https://github.com/example/repo', got %q", result.URLs[0].URL)
	}

	if result.URLs[1].Label != "" {
		t.Errorf("expected empty label for second URL, got %q", result.URLs[1].Label)
	}
	if result.URLs[1].URL != "https://docs.example.com" {
		t.Errorf("expected URL 'https://docs.example.com', got %q", result.URLs[1].URL)
	}
}

func TestParseFrontmatter_NoURLs(t *testing.T) {
	content := []byte(`---
tags:
  - test
---

# No URL Card
`)

	result, err := ParseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.URLs) != 0 {
		t.Errorf("expected 0 URLs, got %d", len(result.URLs))
	}
}

func TestWriteCard_ReadCard_URLRoundTrip(t *testing.T) {
	original := models.Card{
		Filename: "test.md",
		Title:    "URL Round Trip",
		Tags:     []string{},
		URLs: []models.CardURL{
			{Label: "GitHub", URL: "https://github.com/example"},
			{URL: "https://docs.example.com"},
		},
		Content: "# URL Round Trip\n",
	}

	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "url-roundtrip.md")

	if err := WriteCard(original, tmpPath); err != nil {
		t.Fatalf("write error: %v", err)
	}

	loaded, err := ReadCard(tmpPath)
	if err != nil {
		t.Fatalf("read-back error: %v", err)
	}

	if len(loaded.URLs) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(loaded.URLs))
	}
	if loaded.URLs[0].Label != "GitHub" {
		t.Errorf("expected label 'GitHub', got %q", loaded.URLs[0].Label)
	}
	if loaded.URLs[0].URL != "https://github.com/example" {
		t.Errorf("expected URL 'https://github.com/example', got %q", loaded.URLs[0].URL)
	}
	if loaded.URLs[1].Label != "" {
		t.Errorf("expected empty label, got %q", loaded.URLs[1].Label)
	}
	if loaded.URLs[1].URL != "https://docs.example.com" {
		t.Errorf("expected URL 'https://docs.example.com', got %q", loaded.URLs[1].URL)
	}
}
