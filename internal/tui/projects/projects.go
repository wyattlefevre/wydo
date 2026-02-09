package projects

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// ProjectsModel is a placeholder view for the projects feature.
type ProjectsModel struct {
	width  int
	height int
}

func NewProjectsModel() ProjectsModel {
	return ProjectsModel{}
}

func (m *ProjectsModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m ProjectsModel) Update(msg tea.Msg) (ProjectsModel, tea.Cmd) {
	return m, nil
}

func (m ProjectsModel) View() string {
	title := titleStyle.Render("Projects")
	subtitle := subtitleStyle.Render("Coming soon")
	content := lipgloss.JoinVertical(lipgloss.Left, "", title, subtitle, "")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
