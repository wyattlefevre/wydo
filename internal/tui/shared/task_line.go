package shared

import (
	"sort"
	"strings"

	"wydo/internal/tasks/data"
	"wydo/internal/tui/theme"
)

// StyledTaskLine renders a task in a simple, readable format.
// Format: [x] (A) Name +project @context due:date
func StyledTaskLine(t data.Task) string {
	var parts []string

	// Status checkbox
	if t.Done {
		parts = append(parts, theme.Done.Render("[x]"))
	} else {
		parts = append(parts, "[ ]")
	}

	// Priority
	if t.Priority != 0 {
		if t.Done {
			parts = append(parts, theme.Done.Render("("+string(t.Priority)+")"))
		} else {
			parts = append(parts, theme.Priority.Render("("+string(t.Priority)+")"))
		}
	}

	// Name
	if t.Name != "" {
		if t.Done {
			parts = append(parts, theme.Done.Render(t.Name))
		} else {
			parts = append(parts, t.Name)
		}
	}

	// Projects
	for _, p := range t.Projects {
		if t.Done {
			parts = append(parts, theme.Done.Render("+"+p))
		} else {
			parts = append(parts, theme.Project.Render("+"+p))
		}
	}

	// Contexts
	for _, c := range t.Contexts {
		if t.Done {
			parts = append(parts, theme.Done.Render("@"+c))
		} else {
			parts = append(parts, theme.Context.Render("@"+c))
		}
	}

	// Tags (including due date) — sorted for deterministic rendering
	tagKeys := make([]string, 0, len(t.Tags))
	for k := range t.Tags {
		tagKeys = append(tagKeys, k)
	}
	sort.Strings(tagKeys)
	for _, k := range tagKeys {
		v := t.Tags[k]
		formatted := k + ":" + data.FormatTagValue(v)
		if t.Done {
			parts = append(parts, theme.Done.Render(formatted))
		} else {
			parts = append(parts, theme.Tag.Render(formatted))
		}
	}

	return strings.Join(parts, " ")
}
