package shared

import (
	tea "github.com/charmbracelet/bubbletea"
	"wydo/internal/tui/theme"
)

// ConfirmationModal displays a simple yes/no confirmation dialog.
type ConfirmationModal struct {
	Message string // Primary question
	Details string // Additional context (optional)
	Width   int    // Modal width
}

// ConfirmationResultMsg is sent when the user confirms or cancels.
type ConfirmationResultMsg struct {
	Confirmed bool
	Cancelled bool
}

// NewConfirmationModal creates a new confirmation modal.
func NewConfirmationModal(message, details string, width int) *ConfirmationModal {
	return &ConfirmationModal{
		Message: message,
		Details: details,
		Width:   width,
	}
}

// Update handles key events for the confirmation modal.
func (m *ConfirmationModal) Update(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "enter":
		return func() tea.Msg {
			return ConfirmationResultMsg{Confirmed: true}
		}
	case "n", "esc":
		return func() tea.Msg {
			return ConfirmationResultMsg{Cancelled: true}
		}
	}
	return nil
}

// View renders the confirmation modal.
func (m *ConfirmationModal) View() string {
	var content string

	content += theme.Title.Render(m.Message) + "\n"

	if m.Details != "" {
		content += "\n" + m.Details + "\n"
	}

	content += "\n"
	content += theme.Ok.Render("[y]") + " Yes  "
	content += theme.Error.Render("[n/esc]") + " No"

	return theme.ModalBox.Width(m.Width).Render(content)
}
