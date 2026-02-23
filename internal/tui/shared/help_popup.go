package shared

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"wydo/internal/tui/theme"
)

// HelpBind represents a single keybind entry
type HelpBind struct {
	Key  string
	Desc string
}

// HelpSection represents a group of related keybinds
type HelpSection struct {
	Title string
	Binds []HelpBind
}

var (
	helpKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.Secondary)
	helpDescStyle = lipgloss.NewStyle().Foreground(theme.Text)
)

// RenderHelpPopup renders a centered help popup with the given sections
func RenderHelpPopup(sections []HelpSection, width, height int) string {
	line := func(key, desc string) string {
		return "  " + helpKeyStyle.Width(14).Render(key) + helpDescStyle.Render(desc)
	}

	var content string
	for i, section := range sections {
		if i > 0 {
			content += "\n"
		}
		content += theme.Title.Render(section.Title) + "\n"
		for _, bind := range section.Binds {
			content += line(bind.Key, bind.Desc) + "\n"
		}
	}

	content += "\n" + theme.Muted.Render("Press any key to close")

	// Trim trailing newline before boxing
	content = strings.TrimRight(content, "\n")

	box := theme.ModalBox.Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
