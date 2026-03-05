package fs

import (
	"bytes"
	"os"
	"time"
	"wydo/internal/kanban/models"

	"gopkg.in/yaml.v3"
)

// WriteCard writes a Card to a markdown file with frontmatter.
// It is non-destructive: any existing frontmatter fields not known to this
// version of wydo are preserved unchanged.
func WriteCard(card models.Card, path string) error {
	// Load existing frontmatter as a raw map so unknown fields are preserved.
	fm := loadRawFrontmatter(path)

	// Helper: set key if condition is true, otherwise delete it.
	set := func(key string, val interface{}, keep bool) {
		if keep {
			fm[key] = val
		} else {
			delete(fm, key)
		}
	}

	set("tags", card.Tags, len(card.Tags) > 0)
	set("projects", card.Projects, len(card.Projects) > 0)
	set("urls", card.URLs, len(card.URLs) > 0)
	delete(fm, "url") // remove legacy single-url field when urls list is written

	if card.DueDate != nil {
		fm["due"] = card.DueDate.Format("2006-01-02")
	} else {
		delete(fm, "due")
	}
	if card.ScheduledDate != nil {
		fm["scheduled"] = card.ScheduledDate.Format("2006-01-02")
	} else {
		delete(fm, "scheduled")
	}
	if card.DateCompleted != nil {
		fm["date_completed"] = card.DateCompleted.Format(time.RFC3339)
	} else {
		delete(fm, "date_completed")
	}

	set("priority", card.Priority, card.Priority > 0)
	set("archived", card.Archived, card.Archived)
	set("tmux_session", card.TmuxSession, card.TmuxSession != "")
	set("jira_key", card.JiraKey, card.JiraKey != "")
	set("jira_status", card.JiraStatus, card.JiraStatus != "")

	var buf bytes.Buffer
	if len(fm) > 0 {
		buf.WriteString("---\n")
		yamlBytes, err := yaml.Marshal(fm)
		if err != nil {
			return err
		}
		buf.Write(yamlBytes)
		buf.WriteString("---\n\n")
	}

	buf.WriteString(card.Content)

	return os.WriteFile(path, buf.Bytes(), 0644)
}

// loadRawFrontmatter reads the file at path and returns its YAML frontmatter
// as a raw map. Returns an empty map if the file doesn't exist, has no
// frontmatter, or cannot be parsed.
func loadRawFrontmatter(path string) map[string]interface{} {
	fm := make(map[string]interface{})

	data, err := os.ReadFile(path)
	if err != nil {
		return fm
	}

	lines := bytes.Split(data, []byte("\n"))
	if len(lines) == 0 || !bytes.Equal(bytes.TrimSpace(lines[0]), []byte("---")) {
		return fm
	}

	var end int
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(bytes.TrimSpace(lines[i]), []byte("---")) {
			end = i
			break
		}
	}
	if end == 0 {
		return fm
	}

	raw := bytes.Join(lines[1:end], []byte("\n"))
	_ = yaml.Unmarshal(raw, &fm)
	return fm
}
