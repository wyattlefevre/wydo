package operations

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ToSnakeCase converts a title to lowercase snake_case
// "My Card Title!" -> "my_card_title"
func ToSnakeCase(title string) string {
	// Lowercase the string
	s := strings.ToLower(title)

	// Replace spaces and hyphens with underscores
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")

	// Remove non-alphanumeric chars (except underscore)
	var result strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			result.WriteRune(r)
		}
	}
	s = result.String()

	// Collapse multiple underscores
	multiUnderscore := regexp.MustCompile(`_+`)
	s = multiUnderscore.ReplaceAllString(s, "_")

	// Trim leading/trailing underscores
	s = strings.Trim(s, "_")

	// Handle empty result
	if s == "" {
		s = "card"
	}

	return s
}

// UniqueFilename finds a unique filename in the given directory
// If base.md exists (and isn't currentFile), tries base_2.md, base_3.md, etc.
func UniqueFilename(base, dir, currentFile string) string {
	candidate := base + ".md"

	// If the candidate doesn't exist or is the current file, use it
	candidatePath := filepath.Join(dir, candidate)
	if !fileExists(candidatePath) || candidate == currentFile {
		return candidate
	}

	// Otherwise, try with counter suffix
	for i := 2; ; i++ {
		candidate = base + "_" + strconv.Itoa(i) + ".md"
		candidatePath = filepath.Join(dir, candidate)
		if !fileExists(candidatePath) || candidate == currentFile {
			return candidate
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
