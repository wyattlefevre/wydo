package fs

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"time"
	"wydo/internal/kanban/models"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

// ReadCard reads a card file and parses its frontmatter and content
func ReadCard(cardPath string) (models.Card, error) {
	content, err := os.ReadFile(cardPath)
	if err != nil {
		return models.Card{}, err
	}

	filename := filepath.Base(cardPath)
	result, err := ParseFrontmatter(content)
	if err != nil {
		return models.Card{}, err
	}

	title := extractTitle(result.Body)
	preview := extractPreview(result.Body)

	return models.Card{
		Filename:      filename,
		Title:         title,
		Tags:          result.Tags,
		Projects:      result.Projects,
		URL:           result.URL,
		Preview:       preview,
		Content:       result.Body,
		DueDate:       result.DueDate,
		ScheduledDate: result.ScheduledDate,
		DateCompleted: result.DateCompleted,
		Priority:      result.Priority,
		Archived:      result.Archived,
	}, nil
}

// FrontmatterResult holds parsed YAML frontmatter fields from a card
type FrontmatterResult struct {
	Tags          []string
	Projects      []string
	URL           string
	DueDate       *time.Time
	ScheduledDate *time.Time
	DateCompleted *time.Time
	Priority      int
	Archived      bool
	Body          string
}

// ParseFrontmatter extracts YAML frontmatter from markdown content
func ParseFrontmatter(content []byte) (FrontmatterResult, error) {
	empty := FrontmatterResult{Tags: []string{}, Projects: []string{}, Body: string(content)}

	lines := bytes.Split(content, []byte("\n"))

	// Check if content starts with ---
	if len(lines) == 0 || !bytes.Equal(bytes.TrimSpace(lines[0]), []byte("---")) {
		return empty, nil
	}

	// Find the closing ---
	var frontmatterEnd int
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(bytes.TrimSpace(lines[i]), []byte("---")) {
			frontmatterEnd = i
			break
		}
	}

	if frontmatterEnd == 0 {
		return empty, nil
	}

	// Parse frontmatter
	frontmatterBytes := bytes.Join(lines[1:frontmatterEnd], []byte("\n"))
	var frontmatter struct {
		Tags          []string `yaml:"tags"`
		Projects      []string `yaml:"projects"`
		URL           string   `yaml:"url"`
		Due           string   `yaml:"due"`
		Scheduled     string   `yaml:"scheduled"`
		DateCompleted string   `yaml:"date_completed"`
		Priority      int      `yaml:"priority"`
		Archived      bool     `yaml:"archived"`
	}

	if err := yaml.Unmarshal(frontmatterBytes, &frontmatter); err != nil {
		return empty, nil
	}

	body := strings.TrimLeft(string(bytes.Join(lines[frontmatterEnd+1:], []byte("\n"))), "\n")

	tags := frontmatter.Tags
	if tags == nil {
		tags = []string{}
	}

	projects := frontmatter.Projects
	if projects == nil {
		projects = []string{}
	}

	var dueDate *time.Time
	if frontmatter.Due != "" {
		if parsed, err := time.Parse("2006-01-02", frontmatter.Due); err == nil {
			dueDate = &parsed
		}
	}

	var scheduledDate *time.Time
	if frontmatter.Scheduled != "" {
		if parsed, err := time.Parse("2006-01-02", frontmatter.Scheduled); err == nil {
			scheduledDate = &parsed
		}
	}

	var dateCompleted *time.Time
	if frontmatter.DateCompleted != "" {
		if parsed, err := time.Parse(time.RFC3339, frontmatter.DateCompleted); err == nil {
			dateCompleted = &parsed
		}
	}

	return FrontmatterResult{
		Tags:          tags,
		Projects:      projects,
		URL:           frontmatter.URL,
		DueDate:       dueDate,
		ScheduledDate: scheduledDate,
		DateCompleted: dateCompleted,
		Priority:      frontmatter.Priority,
		Archived:      frontmatter.Archived,
		Body:          body,
	}, nil
}

func extractTitle(markdown string) string {
	reader := text.NewReader([]byte(markdown))
	parser := goldmark.DefaultParser()
	doc := parser.Parse(reader)

	var title string
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindHeading {
			heading := n.(*ast.Heading)
			if heading.Level == 1 {
				title = string(n.Text([]byte(markdown)))
				return ast.WalkStop, nil
			}
		}
		return ast.WalkContinue, nil
	})

	if title == "" {
		title = "Untitled"
	}

	return title
}

func extractPreview(markdown string) string {
	reader := text.NewReader([]byte(markdown))
	parser := goldmark.DefaultParser()
	doc := parser.Parse(reader)

	var preview strings.Builder
	lineCount := 0
	maxLines := 2

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if n.Kind() == ast.KindHeading {
			return ast.WalkSkipChildren, nil
		}

		if n.Kind() == ast.KindParagraph {
			if lineCount >= maxLines {
				return ast.WalkStop, nil
			}

			text := string(n.Text([]byte(markdown)))
			if text != "" {
				if preview.Len() > 0 {
					preview.WriteString(" ")
				}
				preview.WriteString(text)
				lineCount++
			}

			return ast.WalkSkipChildren, nil
		}

		return ast.WalkContinue, nil
	})

	previewText := preview.String()
	if len(previewText) > 60 {
		previewText = previewText[:57] + "..."
	}

	return previewText
}
