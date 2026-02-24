package agenda

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	agendapkg "wydo/internal/agenda"
	"wydo/internal/tui/theme"
)

// RenderItemLine renders a single AgendaItem as a styled line
func RenderItemLine(item agendapkg.AgendaItem, selected bool, width int) string {
	var parts []string

	// Cursor indicator
	if selected {
		parts = append(parts, cursorStyle.Render(">"))
	} else {
		parts = append(parts, " ")
	}

	// Title
	title := itemTitle(item)
	if selected {
		parts = append(parts, selectedStyle.Render(title))
	} else if item.Completed {
		parts = append(parts, completedStyle.Render(title))
	} else {
		parts = append(parts, normalStyle.Render(title))
	}

	// Context info (projects for tasks, board/column for cards)
	if item.Completed {
		context := itemContextText(item)
		if context != "" {
			parts = append(parts, completedTagStyle.Render(context))
		}
	} else {
		context := itemContext(item)
		if context != "" {
			parts = append(parts, context)
		}
	}

	// Right-aligned: reason + relative date
	line := strings.Join(parts, " ")

	reasonDate := formatReasonDate(item)
	if item.Completed {
		reasonDate = completedTagStyle.Render("done")
	}
	padding := width - lipgloss.Width(line) - lipgloss.Width(reasonDate) - 1
	if padding < 1 {
		padding = 1
	}

	return line + strings.Repeat(" ", padding) + reasonDate
}

// AgendaSearchString returns the search string for an agenda item (used by search/filter).
func AgendaSearchString(item agendapkg.AgendaItem) string {
	return itemTitle(item)
}

func itemTitle(item agendapkg.AgendaItem) string {
	switch item.Source {
	case agendapkg.SourceTask:
		if item.Task != nil {
			name := item.Task.Name
			if item.Task.Priority != 0 {
				name = "(" + string(item.Task.Priority) + ") " + name
			}
			return name
		}
	case agendapkg.SourceCard:
		if item.Card != nil {
			return item.Card.Title
		}
	case agendapkg.SourceNote:
		if item.Note != nil {
			return item.Note.Title
		}
	}
	return ""
}

func itemContextText(item agendapkg.AgendaItem) string {
	switch item.Source {
	case agendapkg.SourceTask:
		if item.Task != nil && len(item.Task.Projects) > 0 {
			projs := make([]string, len(item.Task.Projects))
			for i, p := range item.Task.Projects {
				projs[i] = "+" + p
			}
			return strings.Join(projs, " ")
		}
	case agendapkg.SourceCard:
		if item.Card != nil {
			return "[" + item.BoardName + " > " + item.ColumnName + "]"
		}
	case agendapkg.SourceNote:
		if item.Note != nil {
			return item.Note.RelPath
		}
	}
	return ""
}

func itemContext(item agendapkg.AgendaItem) string {
	switch item.Source {
	case agendapkg.SourceTask:
		if item.Task != nil && len(item.Task.Projects) > 0 {
			projs := make([]string, len(item.Task.Projects))
			for i, p := range item.Task.Projects {
				projs[i] = "+" + p
			}
			return projectStyle.Render(strings.Join(projs, " "))
		}
	case agendapkg.SourceCard:
		if item.Card != nil {
			return boardInfoStyle.Render("[" + item.BoardName + " > " + item.ColumnName + "]")
		}
	case agendapkg.SourceNote:
		if item.Note != nil {
			return notePathStyle.Render(item.Note.RelPath)
		}
	}
	return ""
}

func formatReasonDate(item agendapkg.AgendaItem) string {
	if item.Reason == agendapkg.ReasonNote {
		return reasonNoteStyle.Render("note")
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	targetDate := time.Date(item.Date.Year(), item.Date.Month(), item.Date.Day(), 0, 0, 0, 0, time.Local)

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

	reason := item.Reason.String()

	var style lipgloss.Style
	if daysUntil > 7 {
		style = lipgloss.NewStyle().Foreground(theme.Success)
	} else if daysUntil > 0 {
		style = lipgloss.NewStyle().Foreground(theme.Warning)
	} else {
		style = lipgloss.NewStyle().Foreground(theme.Danger)
	}

	label := fmt.Sprintf("%s %s", reason, relStr)
	// For items 7+ days overdue, append the absolute date
	if daysUntil <= -7 {
		label += " " + targetDate.Format("Jan 2")
	}

	return style.Render(label)
}
