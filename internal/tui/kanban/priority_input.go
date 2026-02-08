package kanban

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PriorityInputModel struct {
	priority int
	width    int
	height   int
}

func NewPriorityInputModel(currentPriority int) PriorityInputModel {
	return PriorityInputModel{
		priority: currentPriority,
	}
}

func (m PriorityInputModel) Init() tea.Cmd {
	return nil
}

func (m PriorityInputModel) Update(msg tea.KeyMsg) (PriorityInputModel, bool) {
	switch msg.String() {
	case "esc":
		return m, true
	case "enter":
		return m, true
	case "0", "backspace":
		m.priority = 0
	case "1":
		m.priority = 1
	case "2":
		m.priority = 2
	case "3":
		m.priority = 3
	case "4":
		m.priority = 4
	case "5":
		m.priority = 5
	case "6":
		m.priority = 6
	case "7":
		m.priority = 7
	case "8":
		m.priority = 8
	case "9":
		m.priority = 9
	}

	return m, false
}

func (m PriorityInputModel) View() string {
	var s strings.Builder

	title := priorityInputTitleStyle.Render("Set Priority")
	s.WriteString(title)
	s.WriteString("\n\n")

	if m.priority > 0 {
		priorityStyle := lipgloss.NewStyle().Bold(true).Foreground(priorityColor(m.priority))
		s.WriteString(fmt.Sprintf("Priority: %s", priorityStyle.Render(fmt.Sprintf("%d", m.priority))))
	} else {
		s.WriteString(helpStyle.Render("Priority: (none)"))
	}
	s.WriteString("\n\n")

	help := helpStyle.Render("1-9: set priority • 0/backspace: clear • enter: save • esc: cancel")
	s.WriteString(help)

	content := s.String()
	box := priorityInputBoxStyle.Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m PriorityInputModel) GetPriority() int {
	return m.priority
}

func priorityColor(priority int) lipgloss.Color {
	switch priority {
	case 1:
		return lipgloss.Color("5") // magenta/purple
	case 2:
		return lipgloss.Color("1") // red
	case 3:
		return lipgloss.Color("208") // orange (256-color)
	default:
		return lipgloss.Color("3") // yellow
	}
}
