package fs

import (
	"bytes"
	"os"
	"time"
	"wydo/internal/kanban/models"

	"gopkg.in/yaml.v3"
)

// WriteTaskNote writes a TaskNote to a markdown file with frontmatter.
// It is non-destructive: any existing frontmatter fields not known to this
// version of wydo are preserved unchanged.
func WriteTaskNote(tn models.TaskNote, path string) error {
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

	set("tags", tn.Tags, len(tn.Tags) > 0)
	set("projects", tn.Projects, len(tn.Projects) > 0)
	set("urls", tn.URLs, len(tn.URLs) > 0)
	delete(fm, "url") // remove legacy single-url field when urls list is written

	if tn.DueDate != nil {
		fm["due"] = tn.DueDate.Format("2006-01-02")
	} else {
		delete(fm, "due")
	}
	if tn.ScheduledDate != nil {
		fm["scheduled"] = tn.ScheduledDate.Format("2006-01-02")
	} else {
		delete(fm, "scheduled")
	}
	if tn.DateCompleted != nil {
		fm["date_completed"] = tn.DateCompleted.Format(time.RFC3339)
	} else {
		delete(fm, "date_completed")
	}

	set("priority", models.TaskNotePriorityLabel(tn.Priority), tn.Priority > 0)
	delete(fm, "archived") // archival is now path-based, not frontmatter-based
	set("column", tn.Column, tn.Column != "")
	set("tmux_session", tn.TmuxSession, tn.TmuxSession != "")
	set("jira_key", tn.JiraKey, tn.JiraKey != "")
	set("jira_status", tn.JiraStatus, tn.JiraStatus != "")

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

	buf.WriteString(tn.Content)

	return os.WriteFile(path, buf.Bytes(), 0644)
}

// WriteNewTaskNote writes a brand-new TaskNote file with all user-facing
// frontmatter fields initialized to empty values, so external editors see
// the full schema.
func WriteNewTaskNote(tn models.TaskNote, path string) error {
	tags := tn.Tags
	if tags == nil {
		tags = []string{}
	}
	projects := tn.Projects
	if projects == nil {
		projects = []string{}
	}
	urls := tn.URLs
	if urls == nil {
		urls = []models.TaskNoteURL{}
	}

	fm := map[string]interface{}{
		"tags":           tags,
		"projects":       projects,
		"urls":           urls,
		"due":            "",
		"scheduled":      "",
		"date_completed": "",
		"priority":       "",
	}

	if tn.DueDate != nil {
		fm["due"] = tn.DueDate.Format("2006-01-02")
	}
	if tn.ScheduledDate != nil {
		fm["scheduled"] = tn.ScheduledDate.Format("2006-01-02")
	}
	if tn.DateCompleted != nil {
		fm["date_completed"] = tn.DateCompleted.Format(time.RFC3339)
	}
	if tn.Priority > 0 {
		fm["priority"] = models.TaskNotePriorityLabel(tn.Priority)
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	yamlBytes, err := yaml.Marshal(fm)
	if err != nil {
		return err
	}
	buf.Write(yamlBytes)
	buf.WriteString("---\n\n")
	buf.WriteString(tn.Content)

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
