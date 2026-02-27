package shared

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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
		switch k {
		case "url":
			if t.Done {
				parts = append(parts, theme.Done.Render("↗"))
			} else {
				parts = append(parts, theme.Tag.Render("↗"))
			}
		case "due", "scheduled":
			parts = append(parts, renderDateTag(k, v, t.Done))
		default:
			formatted := k + ":" + data.FormatTagValue(v)
			if t.Done {
				parts = append(parts, theme.Done.Render(formatted))
			} else {
				parts = append(parts, theme.Tag.Render(formatted))
			}
		}
	}

	return strings.Join(parts, " ")
}

func renderDateTag(key, value string, done bool) string {
	prefix := "D"
	if key == "scheduled" {
		prefix = "S"
	}

	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		// Fall back to existing key:value rendering
		formatted := key + ":" + data.FormatTagValue(value)
		if done {
			return theme.Done.Render(formatted)
		}
		return theme.Tag.Render(formatted)
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	targetDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
	daysUntil := int(targetDate.Sub(today).Hours() / 24)

	var relStr string
	switch {
	case daysUntil == 0:
		relStr = "0d"
	case daysUntil > 0:
		relStr = fmt.Sprintf("+%dd", daysUntil)
	default:
		relStr = fmt.Sprintf("%dd", daysUntil)
	}

	label := fmt.Sprintf("%s:%s %s", prefix, date.Format("01-02"), relStr)

	if done {
		return theme.Done.Render(label)
	}

	var style lipgloss.Style
	if daysUntil > 7 {
		style = lipgloss.NewStyle().Foreground(theme.Success)
	} else if daysUntil > 0 {
		style = lipgloss.NewStyle().Foreground(theme.Warning)
	} else {
		style = lipgloss.NewStyle().Foreground(theme.Danger)
	}

	return style.Render(label)
}
