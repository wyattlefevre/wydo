package kanban

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"wydo/internal/tui/theme"
)

// DeleteConfirmModel is a modal that asks the user to confirm deleting a card.
// It follows the same pattern as PriorityInputModel.
type DeleteConfirmModel struct {
	cardTitle string
	width     int
	height    int
}

func NewDeleteConfirmModel(cardTitle string) DeleteConfirmModel {
	return DeleteConfirmModel{cardTitle: cardTitle}
}

func (m DeleteConfirmModel) Init() tea.Cmd {
	return nil
}

// Update handles key input. Returns (model, confirmed, cancelled).
func (m DeleteConfirmModel) Update(msg tea.KeyMsg) (DeleteConfirmModel, bool, bool) {
	switch msg.String() {
	case "y":
		return m, true, false
	case "n", "esc":
		return m, false, true
	}
	return m, false, false
}

func (m DeleteConfirmModel) View() string {
	var s strings.Builder

	title := deleteConfirmTitleStyle.Render("Delete Card?")
	s.WriteString(title)
	s.WriteString("\n\n")

	// Show card title, truncated to fit the modal width
	displayTitle := m.cardTitle
	const maxLen = 44 // modal width 50 minus padding
	runes := []rune(displayTitle)
	if len(runes) > maxLen {
		displayTitle = string(runes[:maxLen-3]) + "..."
	}
	s.WriteString(deleteConfirmCardTitleStyle.Render(`"` + displayTitle + `"`))
	s.WriteString("\n\n")

	s.WriteString(theme.Muted.Render("This action cannot be undone."))
	s.WriteString("\n\n")

	s.WriteString(theme.ModalHelp.Render("y:confirm  n/esc:cancel"))

	box := deleteConfirmBoxStyle.Render(s.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
