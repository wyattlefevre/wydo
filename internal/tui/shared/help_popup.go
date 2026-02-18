package shared

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	helpSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	helpKeyStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	helpDescStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	helpBoxStyle     = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("4")).
				Padding(1, 2)
	helpDismissStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
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
		content += helpSectionStyle.Render(section.Title) + "\n"
		for _, bind := range section.Binds {
			content += line(bind.Key, bind.Desc) + "\n"
		}
	}

	content += "\n" + helpDismissStyle.Render("Press any key to close")

	// Trim trailing newline before boxing
	content = strings.TrimRight(content, "\n")

	box := helpBoxStyle.Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
