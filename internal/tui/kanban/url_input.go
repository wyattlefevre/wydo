package kanban

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type URLInputModel struct {
	textInput  textinput.Model
	currentURL string
	width      int
	height     int
}

func NewURLInputModel(currentURL string) URLInputModel {
	ti := textinput.New()
	ti.Placeholder = "https://example.com"
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 50
	ti.SetValue(currentURL)

	return URLInputModel{
		textInput:  ti,
		currentURL: currentURL,
	}
}

func (m URLInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m URLInputModel) Update(msg tea.KeyMsg) (URLInputModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		// Cancel - no changes
		return m, nil
	case "enter":
		// Save
		return m, nil
	default:
		m.textInput, cmd = m.textInput.Update(msg)
	}

	return m, cmd
}

func (m URLInputModel) View() string {
	var s strings.Builder

	title := urlInputTitleStyle.Render("Edit Card URL")
	s.WriteString(title)
	s.WriteString("\n\n")

	s.WriteString(m.textInput.View())
	s.WriteString("\n\n")

	help := helpStyle.Render("enter: save • esc: cancel")
	s.WriteString(help)

	content := s.String()
	box := urlInputBoxStyle.Render(content)

	// Center the box
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m URLInputModel) GetURL() string {
	return strings.TrimSpace(m.textInput.Value())
}
