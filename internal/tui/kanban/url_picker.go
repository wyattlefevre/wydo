package kanban

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	"wydo/internal/kanban/models"
	"wydo/internal/tui/theme"
)

// URLPickerModel is a single-select fuzzy picker for choosing which URL to open.
type URLPickerModel struct {
	urls       []models.CardURL
	display    []string // "label - url" or just "url"
	cursor     int
	filterText string
	filtered   []int // indices into urls
	width      int
	height     int
}

// NewURLPickerModel creates a new URL picker.
func NewURLPickerModel(urls []models.CardURL) URLPickerModel {
	display := make([]string, len(urls))
	for i, u := range urls {
		if u.Label != "" {
			display[i] = fmt.Sprintf("%s - %s", u.Label, u.URL)
		} else {
			display[i] = u.URL
		}
	}

	m := URLPickerModel{
		urls:    urls,
		display: display,
	}
	m.recomputeFilter()
	return m
}

func (m *URLPickerModel) recomputeFilter() {
	if m.filterText == "" {
		m.filtered = make([]int, len(m.urls))
		for i := range m.urls {
			m.filtered[i] = i
		}
		return
	}

	matches := fuzzy.Find(m.filterText, m.display)
	m.filtered = make([]int, len(matches))
	for i, match := range matches {
		m.filtered[i] = match.Index
	}
}

// Update handles key events. Returns (model, selectedURL, done).
func (m URLPickerModel) Update(msg tea.KeyMsg) (URLPickerModel, string, bool) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			return m, m.urls[m.filtered[m.cursor]].URL, true
		}
		return m, "", true
	case "esc":
		return m, "", true
	case "backspace":
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
			m.recomputeFilter()
			m.cursor = 0
		}
	default:
		if len(msg.String()) == 1 {
			m.filterText += msg.String()
			m.recomputeFilter()
			m.cursor = 0
		}
	}
	return m, "", false
}

// View renders the URL picker as a centered modal.
func (m URLPickerModel) View() string {
	var lines []string

	lines = append(lines, tagPickerTitleStyle.Render("Open URL"))
	lines = append(lines, "")

	if m.filterText != "" {
		lines = append(lines, helpStyle.Render("/ "+m.filterText))
	}

	if len(m.urls) == 0 {
		lines = append(lines, cardPreviewStyle.Render("No URLs"))
	} else if len(m.filtered) == 0 {
		lines = append(lines, cardPreviewStyle.Render("No matching URLs"))
	} else {
		const maxWidth = 52
		for i, idx := range m.filtered {
			prefix := "  "
			if i == m.cursor {
				prefix = "> "
			}
			u := m.urls[idx]

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
				urlStr := u.URL
				remaining := maxWidth - len(prefix) - len(u.Label) - 1
				if remaining < 10 {
					remaining = 10
				}
				if len(urlStr) > remaining {
					urlStr = urlStr[:remaining-3] + "..."
				}
				line = bgStyle.Render(prefix) + labelStyle.Render(u.Label) + bgStyle.Render(" ") + urlStyle.Render(urlStr)
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
			lines = append(lines, line)
		}
	}

	lines = append(lines, "")
	help := "j/k: navigate  enter: open  esc: cancel"
	lines = append(lines, helpStyle.Render(help))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	boxed := tagPickerBoxStyle.Width(60).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, boxed)
}
