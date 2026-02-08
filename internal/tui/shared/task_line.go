package shared

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"wydo/internal/tasks/data"
)

var (
	doneStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	priorityStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	projectStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	contextStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	tagStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	nameStyle     = lipgloss.NewStyle()
)

// StyledTaskLine renders a task in a simple, readable format.
// Format: [x] (A) Name +project @context due:date
func StyledTaskLine(t data.Task) string {
	var parts []string

	// Status checkbox
	if t.Done {
		parts = append(parts, doneStyle.Render("[x]"))
	} else {
		parts = append(parts, "[ ]")
	}

	// Priority
	if t.Priority != 0 {
		if t.Done {
			parts = append(parts, doneStyle.Render("("+string(t.Priority)+")"))
		} else {
			parts = append(parts, priorityStyle.Render("("+string(t.Priority)+")"))
		}
	}

	// Name
	if t.Name != "" {
		if t.Done {
			parts = append(parts, doneStyle.Render(t.Name))
		} else {
			parts = append(parts, nameStyle.Render(t.Name))
		}
	}

	// Projects
	for _, p := range t.Projects {
		if t.Done {
			parts = append(parts, doneStyle.Render("+"+p))
		} else {
			parts = append(parts, projectStyle.Render("+"+p))
		}
	}

	// Contexts
	for _, c := range t.Contexts {
		if t.Done {
			parts = append(parts, doneStyle.Render("@"+c))
		} else {
			parts = append(parts, contextStyle.Render("@"+c))
		}
	}

	// Tags (including due date)
	for k, v := range t.Tags {
		if t.Done {
			parts = append(parts, doneStyle.Render(k+":"+v))
		} else {
			parts = append(parts, tagStyle.Render(k+":"+v))
		}
	}

	return strings.Join(parts, " ")
}
