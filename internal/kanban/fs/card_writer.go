package fs

import (
	"bytes"
	"os"
	"time"
	"wydo/internal/kanban/models"

	"gopkg.in/yaml.v3"
)

// WriteCard writes a Card to a markdown file with frontmatter
func WriteCard(card models.Card, path string) error {
	var buf bytes.Buffer

	if len(card.Tags) > 0 || len(card.Projects) > 0 || card.URL != "" || card.DueDate != nil || card.ScheduledDate != nil || card.DateCompleted != nil || card.Priority > 0 || card.Archived {
		buf.WriteString("---\n")

		frontmatter := struct {
			Tags          []string `yaml:"tags,omitempty"`
			Projects      []string `yaml:"projects,omitempty"`
			URL           string   `yaml:"url,omitempty"`
			Due           string   `yaml:"due,omitempty"`
			Scheduled     string   `yaml:"scheduled,omitempty"`
			DateCompleted string   `yaml:"date_completed,omitempty"`
			Priority      int      `yaml:"priority,omitempty"`
			Archived      bool     `yaml:"archived,omitempty"`
		}{
			Tags:     card.Tags,
			Projects: card.Projects,
			URL:      card.URL,
			Priority: card.Priority,
			Archived: card.Archived,
		}

		if card.DueDate != nil {
			frontmatter.Due = card.DueDate.Format("2006-01-02")
		}
		if card.ScheduledDate != nil {
			frontmatter.Scheduled = card.ScheduledDate.Format("2006-01-02")
		}
		if card.DateCompleted != nil {
			frontmatter.DateCompleted = card.DateCompleted.Format(time.RFC3339)
		}

		yamlBytes, err := yaml.Marshal(frontmatter)
		if err != nil {
			return err
		}

		buf.Write(yamlBytes)
		buf.WriteString("---\n\n")
	}

	buf.WriteString(card.Content)

	return os.WriteFile(path, buf.Bytes(), 0644)
}
