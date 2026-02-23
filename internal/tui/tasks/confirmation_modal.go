package tasks

import (
	tea "github.com/charmbracelet/bubbletea"
	"wydo/internal/tui/theme"
)

var (
	confirmModalBoxStyle = theme.ModalBox
	confirmTitleStyle     = theme.Title
	confirmYesStyle       = theme.Ok
	confirmNoStyle        = theme.Error
)

// ConfirmationModal displays a simple yes/no confirmation dialog
type ConfirmationModal struct {
	Message string // Primary question
	Details string // Additional context (optional)
	Width   int    // Modal width
}

// ConfirmationResultMsg is sent when the user confirms or cancels
type ConfirmationResultMsg struct {
	Confirmed bool
	Cancelled bool
}

// NewConfirmationModal creates a new confirmation modal
func NewConfirmationModal(message, details string, width int) *ConfirmationModal {
	return &ConfirmationModal{
		Message: message,
		Details: details,
		Width:   width,
	}
}

// Update handles key events for the confirmation modal
func (m *ConfirmationModal) Update(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "enter":
		return func() tea.Msg {
			return ConfirmationResultMsg{
				Confirmed: true,
				Cancelled: false,
			}
		}
	case "n", "esc":
		return func() tea.Msg {
			return ConfirmationResultMsg{
				Confirmed: false,
				Cancelled: true,
			}
		}
	}
	return nil
}

// View renders the confirmation modal
func (m *ConfirmationModal) View() string {
	var content string

	content += confirmTitleStyle.Render(m.Message) + "\n"

	if m.Details != "" {
		content += "\n" + m.Details + "\n"
	}

	content += "\n"
	content += confirmYesStyle.Render("[y]") + " Yes  "
	content += confirmNoStyle.Render("[n/esc]") + " No"

	return confirmModalBoxStyle.Width(m.Width).Render(content)
}
