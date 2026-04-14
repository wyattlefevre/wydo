package shared

import "wydo/internal/tui/theme"

// Dialog is a simple modal box that renders a title, optional body, and key hints.
// It produces only the box string — placement is the caller's responsibility via PlaceOverlay.
type Dialog struct {
	Title string
	Body  string // optional multi-line content between title and hints
	Hints string // pre-styled hint string, e.g. theme.Ok.Render("[y]") + " Yes"
	Width int
}

func (d Dialog) View() string {
	content := theme.ModalTitle.Render(d.Title)
	if d.Body != "" {
		content += "\n\n" + d.Body
	}
	content += "\n\n" + d.Hints
	return theme.ModalBox.Width(d.Width).Render(content)
}
