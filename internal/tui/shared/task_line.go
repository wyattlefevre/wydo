package shared

import (
	"fmt"
	"sort"
	"strings"
	"time"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/lipgloss"
	"wydo/internal/tasks/data"
	"wydo/internal/tui/theme"
)

var boardNameStyle = lipgloss.NewStyle().Foreground(theme.Secondary)
var columnNameStyle = lipgloss.NewStyle().Foreground(theme.Primary)
var simpleBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))

const statusWidth = 7

// StyledTaskLine renders a task in a simple, readable format.
// Format: [x] (A) Name +project @context due:date
//
// width is the available visual width for the entire line. When > 0:
//   - The task name is truncated to keep the line within width.
//   - For TaskNotes the board name and column status are right-aligned at the far right.
//
// boardNameWidth sets the minimum width for the board name field (for column alignment).
// The column/status field is always fixed at statusWidth (7) chars.
// Pass width=0 to skip truncation/alignment (e.g. in non-fixed-width views).
func StyledTaskLine(t data.Task, width int, boardNameWidth int) string {
	var parts []string

	// Priority
	if t.Priority != 0 {
		if t.Done {
			parts = append(parts, priorityBadgeDone(t.Priority))
		} else {
			parts = append(parts, priorityBadge(t.Priority))
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

	// Tags (@ prefix)
	for _, c := range t.Tags {
		if t.Done {
			parts = append(parts, theme.Done.Render("@"+c))
		} else {
			parts = append(parts, theme.AtTag.Render("@"+c))
		}
	}

	// Properties (including due date) — sorted for deterministic rendering
	tagKeys := make([]string, 0, len(t.Properties))
	for k := range t.Properties {
		tagKeys = append(tagKeys, k)
	}
	sort.Strings(tagKeys)
	for _, k := range tagKeys {
		v := t.Properties[k]
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

	left := strings.Join(parts, " ")

	// Build right-aligned suffix
	var suffix string
	if t.IsTaskNote {
		colStatus := t.ColumnName
		if len(colStatus) > statusWidth {
			colStatus = colStatus[:statusWidth]
		} else {
			colStatus = fmt.Sprintf("%-*s", statusWidth, colStatus)
		}
		if t.Done {
			suffix = "  " + theme.Done.Render(fmt.Sprintf("%-*s", boardNameWidth, t.BoardName)) + " " + theme.Done.Render(colStatus)
		} else {
			suffix = "  " + boardNameStyle.Render(fmt.Sprintf("%-*s", boardNameWidth, t.BoardName)) + " " + columnNameStyle.Render(colStatus)
		}
	} else {
		simplePart := simpleBadgeStyle.Render(fmt.Sprintf("%-*s", boardNameWidth, "simple"))
		if t.Done {
			suffix = "  " + simplePart + " " + theme.Done.Render(fmt.Sprintf("%-*s", statusWidth, "Done"))
		} else {
			suffix = "  " + simplePart + " " + strings.Repeat(" ", statusWidth)
		}
	}

	if suffix == "" {
		if width > 0 && lipgloss.Width(left) > width {
			left = xansi.Cut(left, 0, width)
		}
		return left
	}

	if width <= 0 {
		return left + suffix
	}

	suffixW := lipgloss.Width(suffix)
	leftW := lipgloss.Width(left)

	// Minimum 1 space between left content and suffix
	maxLeft := width - suffixW - 1
	if maxLeft < 1 {
		maxLeft = 1
	}
	if leftW > maxLeft {
		left = xansi.Cut(left, 0, maxLeft)
		leftW = maxLeft
	}

	gap := width - leftW - suffixW
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + suffix
}

// AgendaPriorityBadge returns a styled badge for use in the agenda view.
// Callers must ensure p != PriorityNone before calling.
func AgendaPriorityBadge(p data.Priority) string {
	return priorityBadge(p)
}

func priorityColors(p data.Priority) (bg, fg lipgloss.Color) {
	switch p {
	case data.PriorityA:
		return lipgloss.Color("1"), lipgloss.Color("16")   // red
	case data.PriorityB:
		return lipgloss.Color("208"), lipgloss.Color("16") // orange
	case data.PriorityC:
		return lipgloss.Color("3"), lipgloss.Color("16")   // yellow
	case data.PriorityD:
		return lipgloss.Color("2"), lipgloss.Color("16")   // green
	case data.PriorityE:
		return lipgloss.Color("4"), lipgloss.Color("15")   // blue
	default: // F and beyond
		return lipgloss.Color("54"), lipgloss.Color("15")  // dark purple
	}
}

// priorityBadge renders a lualine-style powerline badge for a priority letter.
func priorityBadge(p data.Priority) string {
	bg, fg := priorityColors(p)
	leftCap := lipgloss.NewStyle().Foreground(bg).Render("\ue0b6")
	body := lipgloss.NewStyle().Bold(true).Background(bg).Foreground(fg).Render(string(p))
	rightCap := lipgloss.NewStyle().Foreground(bg).Render("\ue0b4")
	return leftCap + body + rightCap
}

// priorityBadgeDone renders the same pill shape as priorityBadge but muted for completed tasks.
func priorityBadgeDone(p data.Priority) string {
	activeBg, _ := priorityColors(p)
	bg := theme.Surface
	leftCap := lipgloss.NewStyle().Foreground(bg).Render("\ue0b6")
	body := lipgloss.NewStyle().Background(bg).Foreground(activeBg).Render(string(p))
	rightCap := lipgloss.NewStyle().Foreground(bg).Render("\ue0b4")
	return leftCap + body + rightCap
}

// taskPriorityStyle returns a background-badge style for a todo.txt priority (A–F).
func taskPriorityStyle(p data.Priority) lipgloss.Style {
	bg, fg := priorityColors(p)
	return lipgloss.NewStyle().Bold(true).Background(bg).Foreground(fg)
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
	} else if daysUntil == 0 {
		style = lipgloss.NewStyle().Foreground(theme.Primary)
	} else {
		style = lipgloss.NewStyle().Foreground(theme.Danger)
	}

	return style.Render(label)
}
