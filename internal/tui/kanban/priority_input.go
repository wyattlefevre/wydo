package kanban

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	kanbanmodels "wydo/internal/kanban/models"
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
	case "a", "A":
		m.priority = 1
	case "b", "B":
		m.priority = 2
	case "c", "C":
		m.priority = 3
	case "d", "D":
		m.priority = 4
	case "e", "E":
		m.priority = 5
	case "f", "F":
		m.priority = 6
	}

	return m, false
}

func (m PriorityInputModel) View() string {
	var s strings.Builder

	title := priorityInputTitleStyle.Render("Set Priority")
	s.WriteString(title)
	s.WriteString("\n\n")

	if m.priority > 0 {
		label := kanbanmodels.TaskNotePriorityLabel(m.priority)
		s.WriteString(fmt.Sprintf("Priority: %s", kanbanPriorityStyle(m.priority).Render(" "+label+" ")))
	} else {
		s.WriteString(helpStyle.Render("Priority: (none)"))
	}
	s.WriteString("\n\n")

	help := helpStyle.Render("a-f: set priority • 0/backspace: clear • enter: save • esc: cancel")
	s.WriteString(help)

	content := s.String()
	box := priorityInputBoxStyle.Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m PriorityInputModel) GetPriority() int {
	return m.priority
}

