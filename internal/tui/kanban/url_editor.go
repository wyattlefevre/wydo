package kanban

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"wydo/internal/kanban/models"
	"wydo/internal/tui/theme"
)

type urlEditorMode int

const (
	urlEditorNav urlEditorMode = iota
	urlEditorAddURL
	urlEditorAddLabel
	urlEditorEditURL
	urlEditorEditLabel
)

// URLEditorModel is a list editor modal for managing a card's URLs.
type URLEditorModel struct {
	urls      []models.CardURL
	cursor    int
	mode      urlEditorMode
	textInput textinput.Model
	width     int
	height    int
}

// NewURLEditorModel creates a new URL editor with the given URLs.
func NewURLEditorModel(urls []models.CardURL) URLEditorModel {
	// Copy URLs to avoid mutating the original
	copied := make([]models.CardURL, len(urls))
	copy(copied, urls)

	return URLEditorModel{
		urls: copied,
		mode: urlEditorNav,
	}
}

func (m URLEditorModel) Init() tea.Cmd {
	return nil
}

func (m *URLEditorModel) initTextInput(placeholder, value string) tea.Cmd {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 500
	ti.Width = 50
	ti.SetValue(value)
	ti.Focus()
	m.textInput = ti
	return textinput.Blink
}

// Update handles key events. Returns (model, cmd, saved, done).
func (m URLEditorModel) Update(msg tea.KeyMsg) (URLEditorModel, tea.Cmd, bool, bool) {
	switch m.mode {
	case urlEditorNav:
		return m.updateNav(msg)
	case urlEditorAddURL:
		return m.updateAddURL(msg)
	case urlEditorAddLabel:
		return m.updateAddLabel(msg)
	case urlEditorEditURL:
		return m.updateEditURL(msg)
	case urlEditorEditLabel:
		return m.updateEditLabel(msg)
	}
	return m, nil, false, false
}

func (m URLEditorModel) updateNav(msg tea.KeyMsg) (URLEditorModel, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.urls)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "n":
		m.mode = urlEditorAddURL
		cmd := m.initTextInput("https://example.com", "")
		return m, cmd, false, false
	case "d":
		if len(m.urls) > 0 && m.cursor < len(m.urls) {
			m.urls = append(m.urls[:m.cursor], m.urls[m.cursor+1:]...)
			if m.cursor >= len(m.urls) && m.cursor > 0 {
				m.cursor--
			}
		}
	case "e":
		if len(m.urls) > 0 && m.cursor < len(m.urls) {
			m.mode = urlEditorEditURL
			cmd := m.initTextInput("https://example.com", m.urls[m.cursor].URL)
			return m, cmd, false, false
		}
	case "l":
		if len(m.urls) > 0 && m.cursor < len(m.urls) {
			m.mode = urlEditorEditLabel
			cmd := m.initTextInput("Label (optional)", m.urls[m.cursor].Label)
			return m, cmd, false, false
		}
	case "enter":
		return m, nil, true, true
	case "esc":
		return m, nil, false, true
	}
	return m, nil, false, false
}

func (m URLEditorModel) updateAddURL(msg tea.KeyMsg) (URLEditorModel, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "enter":
		url := strings.TrimSpace(m.textInput.Value())
		if url != "" {
			m.urls = append(m.urls, models.CardURL{URL: url})
			m.cursor = len(m.urls) - 1
			// Now ask for label
			m.mode = urlEditorAddLabel
			cmd := m.initTextInput("Label (optional, enter to skip)", "")
			return m, cmd, false, false
		}
		m.mode = urlEditorNav
		return m, nil, false, false
	case "esc":
		m.mode = urlEditorNav
		return m, nil, false, false
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd, false, false
	}
}

func (m URLEditorModel) updateAddLabel(msg tea.KeyMsg) (URLEditorModel, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "enter":
		label := strings.TrimSpace(m.textInput.Value())
		if label != "" && m.cursor < len(m.urls) {
			m.urls[m.cursor].Label = label
		}
		m.mode = urlEditorNav
		return m, nil, false, false
	case "esc":
		m.mode = urlEditorNav
		return m, nil, false, false
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd, false, false
	}
}

func (m URLEditorModel) updateEditURL(msg tea.KeyMsg) (URLEditorModel, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "enter":
		url := strings.TrimSpace(m.textInput.Value())
		if url != "" && m.cursor < len(m.urls) {
			m.urls[m.cursor].URL = url
		}
		m.mode = urlEditorNav
		return m, nil, false, false
	case "esc":
		m.mode = urlEditorNav
		return m, nil, false, false
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd, false, false
	}
}

func (m URLEditorModel) updateEditLabel(msg tea.KeyMsg) (URLEditorModel, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "enter":
		label := strings.TrimSpace(m.textInput.Value())
		if m.cursor < len(m.urls) {
			m.urls[m.cursor].Label = label
		}
		m.mode = urlEditorNav
		return m, nil, false, false
	case "esc":
		m.mode = urlEditorNav
		return m, nil, false, false
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd, false, false
	}
}

// GetURLs returns the current list of URLs.
func (m URLEditorModel) GetURLs() []models.CardURL {
	return m.urls
}

// View renders the URL editor modal.
func (m URLEditorModel) View() string {
	var s strings.Builder

	title := urlInputTitleStyle.Render("Edit URLs")
	s.WriteString(title)
	s.WriteString("\n\n")

	if m.mode != urlEditorNav {
		// Show text input
		var prompt string
		switch m.mode {
		case urlEditorAddURL:
			prompt = "New URL:"
		case urlEditorAddLabel:
			prompt = "Label:"
		case urlEditorEditURL:
			prompt = "Edit URL:"
		case urlEditorEditLabel:
			prompt = "Edit Label:"
		}
		s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render(prompt))
		s.WriteString("\n")
		s.WriteString(m.textInput.View())
		s.WriteString("\n\n")
		s.WriteString(helpStyle.Render("enter: confirm • esc: cancel"))
	} else {
		// Show URL list
		if len(m.urls) == 0 {
			s.WriteString(cardPreviewStyle.Render("  No URLs"))
			s.WriteString("\n")
		} else {
			const maxWidth = 52 // usable width inside the modal box
			for i, u := range m.urls {
				prefix := "  "
				if i == m.cursor {
					prefix = "> "
				}

				var line string
				if u.Label != "" {
					labelStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Primary)
					urlStyle := lipgloss.NewStyle().Foreground(theme.TextMuted)
					bgStyle := lipgloss.NewStyle()
					if i == m.cursor {
						labelStyle = labelStyle.Foreground(theme.Accent).Background(theme.Surface)
						urlStyle = urlStyle.Foreground(theme.Warning).Background(theme.Surface)
						bgStyle = bgStyle.Background(theme.Surface)
					}
					label := u.Label
					// Reserve space: prefix(2) + label + " " + url (at least 10 chars)
					urlStr := u.URL
					remaining := maxWidth - len(prefix) - len(label) - 1
					if remaining < 10 {
						remaining = 10
					}
					if len(urlStr) > remaining {
						urlStr = urlStr[:remaining-3] + "..."
					}
					line = bgStyle.Render(prefix) + labelStyle.Render(label) + bgStyle.Render(" ") + urlStyle.Render(urlStr)
				} else {
					style := listItemStyle
					if i == m.cursor {
						style = selectedListItemStyle
					}
					urlStr := u.URL
					remaining := maxWidth - len(prefix)
					if len(urlStr) > remaining {
						urlStr = urlStr[:remaining-3] + "..."
					}
					line = style.Render(prefix + urlStr)
				}
				s.WriteString(line)
				s.WriteString("\n")
			}
		}
		s.WriteString("\n")
		s.WriteString(helpStyle.Render("n:add  d:delete  e:edit url  l:edit label  enter:save  esc:cancel"))
	}

	content := s.String()
	box := urlInputBoxStyle.Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
