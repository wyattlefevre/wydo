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
	urlEditorSelectProject
)

// urlItem is a single URL entry with its owning project name.
// When projectNames is empty (card URL editor), projectName is always "".
type urlItem struct {
	projectName string
	url         models.CardURL
}

// URLEditorModel is a list editor modal for managing a card's URLs.
type URLEditorModel struct {
	items     []urlItem
	cursor    int
	mode      urlEditorMode
	textInput textinput.Model
	width     int
	height    int

	// Sub-project support
	pendingURL    string
	pendingLabel  string
	projectNames  []string // root first, then sub-projects; empty = no selection step
	projectCursor int
}

// NewURLEditorModel creates a new URL editor with the given URLs (card/single-project use).
func NewURLEditorModel(urls []models.CardURL) URLEditorModel {
	items := make([]urlItem, len(urls))
	for i, u := range urls {
		items[i] = urlItem{url: u}
	}
	return URLEditorModel{
		items: items,
		mode:  urlEditorNav,
	}
}

// NewURLEditorModelWithProjects creates a URL editor showing URLs across root and sub-projects.
// projectNames has the root project first; existingSubURLs maps sub-project name → current URLs.
func NewURLEditorModelWithProjects(rootURLs []models.CardURL, projectNames []string, existingSubURLs map[string][]models.CardURL) URLEditorModel {
	var items []urlItem
	rootName := ""
	if len(projectNames) > 0 {
		rootName = projectNames[0]
	}
	for _, u := range rootURLs {
		items = append(items, urlItem{projectName: rootName, url: u})
	}
	// Append sub-project URLs in projectNames order so display is deterministic.
	for _, name := range projectNames[1:] {
		for _, u := range existingSubURLs[name] {
			items = append(items, urlItem{projectName: name, url: u})
		}
	}

	names := make([]string, len(projectNames))
	copy(names, projectNames)
	return URLEditorModel{
		items:        items,
		mode:         urlEditorNav,
		projectNames: names,
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
	case urlEditorSelectProject:
		return m.updateSelectProject(msg)
	}
	return m, nil, false, false
}

func (m URLEditorModel) updateNav(msg tea.KeyMsg) (URLEditorModel, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.items)-1 {
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
		if len(m.items) > 0 && m.cursor < len(m.items) {
			m.items = append(m.items[:m.cursor], m.items[m.cursor+1:]...)
			if m.cursor >= len(m.items) && m.cursor > 0 {
				m.cursor--
			}
		}
	case "e":
		if len(m.items) > 0 && m.cursor < len(m.items) {
			m.mode = urlEditorEditURL
			cmd := m.initTextInput("https://example.com", m.items[m.cursor].url.URL)
			return m, cmd, false, false
		}
	case "l":
		if len(m.items) > 0 && m.cursor < len(m.items) {
			m.mode = urlEditorEditLabel
			cmd := m.initTextInput("Label (optional)", m.items[m.cursor].url.Label)
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
			m.pendingURL = url
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
		m.pendingLabel = strings.TrimSpace(m.textInput.Value())
		if len(m.projectNames) > 1 {
			m.projectCursor = 0
			m.mode = urlEditorSelectProject
			return m, nil, false, false
		}
		// No sub-projects: commit directly to root.
		rootName := ""
		if len(m.projectNames) > 0 {
			rootName = m.projectNames[0]
		}
		m.insertIntoProject(rootName, models.CardURL{URL: m.pendingURL, Label: m.pendingLabel})
		m.pendingURL = ""
		m.pendingLabel = ""
		m.mode = urlEditorNav
		return m, nil, false, false
	case "esc":
		m.pendingURL = ""
		m.pendingLabel = ""
		m.mode = urlEditorNav
		return m, nil, false, false
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd, false, false
	}
}

func (m URLEditorModel) updateSelectProject(msg tea.KeyMsg) (URLEditorModel, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "j", "down":
		if m.projectCursor < len(m.projectNames)-1 {
			m.projectCursor++
		}
	case "k", "up":
		if m.projectCursor > 0 {
			m.projectCursor--
		}
	case "enter":
		projName := m.projectNames[m.projectCursor]
		m.insertIntoProject(projName, models.CardURL{URL: m.pendingURL, Label: m.pendingLabel})
		m.pendingURL = ""
		m.pendingLabel = ""
		m.mode = urlEditorNav
	case "esc":
		m.pendingURL = ""
		m.pendingLabel = ""
		m.mode = urlEditorNav
	}
	return m, nil, false, false
}

// insertIntoProject appends a URL after the last existing item for the given project
// (or at the end if the project has no items yet), then sets cursor to the new item.
func (m *URLEditorModel) insertIntoProject(projectName string, url models.CardURL) {
	item := urlItem{projectName: projectName, url: url}
	insertIdx := len(m.items)
	for i := len(m.items) - 1; i >= 0; i-- {
		if m.items[i].projectName == projectName {
			insertIdx = i + 1
			break
		}
	}
	m.items = append(m.items, urlItem{})
	copy(m.items[insertIdx+1:], m.items[insertIdx:])
	m.items[insertIdx] = item
	m.cursor = insertIdx
}

func (m URLEditorModel) updateEditURL(msg tea.KeyMsg) (URLEditorModel, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "enter":
		url := strings.TrimSpace(m.textInput.Value())
		if url != "" && m.cursor < len(m.items) {
			m.items[m.cursor].url.URL = url
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
		if m.cursor < len(m.items) {
			m.items[m.cursor].url.Label = label
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

// GetURLs returns the URLs belonging to the root project (or all URLs for single-project use).
func (m URLEditorModel) GetURLs() []models.CardURL {
	rootName := ""
	if len(m.projectNames) > 0 {
		rootName = m.projectNames[0]
	}
	var result []models.CardURL
	for _, it := range m.items {
		if it.projectName == rootName {
			result = append(result, it.url)
		}
	}
	return result
}

// GetSubProjectURLs returns all non-root URLs grouped by project name.
// Includes an entry (possibly empty) for every managed sub-project so callers can
// write back even when all URLs for a project were deleted.
func (m URLEditorModel) GetSubProjectURLs() map[string][]models.CardURL {
	if len(m.projectNames) == 0 {
		return nil
	}
	rootName := m.projectNames[0]
	result := make(map[string][]models.CardURL)
	for _, name := range m.projectNames[1:] {
		result[name] = nil // ensure key exists even if no URLs remain
	}
	for _, it := range m.items {
		if it.projectName != rootName {
			result[it.projectName] = append(result[it.projectName], it.url)
		}
	}
	return result
}

// SetSize sets the display dimensions for centering the modal.
func (m *URLEditorModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// IsTyping returns true when a text input is active inside the editor.
func (m URLEditorModel) IsTyping() bool {
	return m.mode != urlEditorNav && m.mode != urlEditorSelectProject
}

// View renders the URL editor modal.
func (m URLEditorModel) View() string {
	var s strings.Builder

	title := urlInputTitleStyle.Render("Edit URLs")
	s.WriteString(title)
	s.WriteString("\n\n")

	if m.mode == urlEditorSelectProject {
		s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render("Add to project:"))
		s.WriteString("\n\n")
		for i, name := range m.projectNames {
			prefix := "  "
			if i == m.projectCursor {
				prefix = "> "
			}
			var line string
			if i == m.projectCursor {
				line = selectedListItemStyle.Render(prefix + name)
			} else {
				line = listItemStyle.Render(prefix + name)
			}
			s.WriteString(line)
			s.WriteString("\n")
		}
		s.WriteString("\n")
		s.WriteString(helpStyle.Render("j/k: navigate  enter: confirm  esc: cancel"))
	} else if m.mode != urlEditorNav {
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
		multiProject := len(m.projectNames) > 1
		if len(m.items) == 0 {
			s.WriteString(cardPreviewStyle.Render("  No URLs"))
			s.WriteString("\n")
		} else {
			const maxWidth = 52 // usable width inside the modal box
			lastProject := "\x00"  // sentinel so first header always shows
			for i, item := range m.items {
				u := item.url

				// Project header when in multi-project mode and project changes
				if multiProject && item.projectName != lastProject {
					if lastProject != "\x00" {
						s.WriteString("\n")
					}
					header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render("  " + item.projectName)
					s.WriteString(header)
					s.WriteString("\n")
					lastProject = item.projectName
				}

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
