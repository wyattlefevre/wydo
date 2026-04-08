package shared

import "wydo/internal/tui/theme"

// RenderModal renders a standard modal box with an optional title and footer help text.
// The title is styled with theme.ModalTitle, the footer with theme.ModalHelp,
// and the whole thing is wrapped in theme.ModalBox at the given width.
func RenderModal(title, body, footer string, width int) string {
	var content string
	if title != "" {
		content = theme.ModalTitle.Render(title) + "\n\n"
	}
	content += body
	if footer != "" {
		content += "\n\n" + theme.ModalHelp.Render(footer)
	}
	return theme.ModalBox.Width(width).Render(content)
}
