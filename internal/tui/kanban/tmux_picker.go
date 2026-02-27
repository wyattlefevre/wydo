package kanban

import (
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	"wydo/internal/tui/theme"
)

// Child session suffixes that indicate a child tmux session
var childSuffixes = []string{"-claude", "-lazygit", "-term"}

// TmuxPickerModel is a single-select fuzzy picker for linking tmux sessions to cards.
type TmuxPickerModel struct {
	sessions       []string // root sessions only
	cursor         int
	filterText     string
	filtered       []int // indices into sessions
	currentSession string
	width          int
	height         int
	filterMode     bool
}

// NewTmuxPickerModel creates a new tmux session picker.
func NewTmuxPickerModel(currentSession string) TmuxPickerModel {
	allSessions := listTmuxSessions()
	var rootSessions []string
	for _, s := range allSessions {
		if !isChildSession(s) {
			rootSessions = append(rootSessions, s)
		}
	}

	m := TmuxPickerModel{
		sessions:       rootSessions,
		currentSession: currentSession,
	}
	m.recomputeFilter()
	return m
}

func (m *TmuxPickerModel) recomputeFilter() {
	if m.filterText == "" {
		m.filtered = make([]int, len(m.sessions))
		for i := range m.sessions {
			m.filtered[i] = i
		}
		return
	}

	matches := fuzzy.Find(m.filterText, m.sessions)
	m.filtered = make([]int, len(matches))
	for i, match := range matches {
		m.filtered[i] = match.Index
	}
}

// Update handles key events. Returns (model, selectedSession, done).
// selectedSession is the chosen session on enter, empty string on unlink (d), or empty on cancel.
// done is true on enter, d, or esc.
func (m TmuxPickerModel) Update(msg tea.KeyMsg) (TmuxPickerModel, string, bool) {
	if m.filterMode {
		switch msg.String() {
		case "enter":
			m.filterMode = false
		case "esc":
			m.filterText = ""
			m.filterMode = false
			m.recomputeFilter()
			m.cursor = 0
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

	switch msg.String() {
	case "/":
		m.filterMode = true
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
			return m, m.sessions[m.filtered[m.cursor]], true
		}
		return m, "", true
	case "d":
		// Unlink
		return m, "", true
	case "esc":
		return m, m.currentSession, true
	case "backspace":
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
			m.recomputeFilter()
			m.cursor = 0
		}
	}
	return m, "", false
}

// View renders the tmux session picker as a centered modal.
func (m TmuxPickerModel) View() string {
	var lines []string

	lines = append(lines, tagPickerTitleStyle.Render("Link Tmux Session"))
	lines = append(lines, "")

	if m.filterText != "" || m.filterMode {
		filterDisplay := "/ " + m.filterText
		if m.filterMode {
			filterDisplay += "_"
		}
		lines = append(lines, helpStyle.Render(filterDisplay))
	}

	if len(m.sessions) == 0 {
		lines = append(lines, cardPreviewStyle.Render("No tmux sessions found"))
	} else if len(m.filtered) == 0 {
		lines = append(lines, cardPreviewStyle.Render("No matching sessions"))
	} else {
		for i, idx := range m.filtered {
			session := m.sessions[idx]
			style := listItemStyle
			prefix := "  "
			if i == m.cursor {
				style = selectedListItemStyle
				prefix = "> "
			}

			indicator := ""
			if session == m.currentSession {
				indicator = " *"
			}

			lines = append(lines, style.Render(prefix+session+indicator))
		}
	}

	lines = append(lines, "")
	var help string
	if m.filterMode {
		help = "enter: done  esc: clear filter"
	} else {
		help = "/ filter  j/k navigate  enter select  d unlink  esc cancel"
	}
	lines = append(lines, helpStyle.Render(help))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	boxed := tagPickerBoxStyle.Width(50).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, boxed)
}

// TmuxLaunchModel is a small popup for choosing root vs child tmux session.
type TmuxLaunchModel struct {
	rootSession string
	children    map[string]bool // suffix -> exists
	width       int
	height      int
}

// NewTmuxLaunchModel creates a launch popup for a root session, checking which children exist.
func NewTmuxLaunchModel(rootSession string) TmuxLaunchModel {
	return TmuxLaunchModel{
		rootSession: rootSession,
		children:    getChildSessions(rootSession),
	}
}

// Update handles key events. Returns (model, targetSession, done).
func (m TmuxLaunchModel) Update(msg tea.KeyMsg) (TmuxLaunchModel, string, bool) {
	switch msg.String() {
	case "r":
		return m, m.rootSession, true
	case "c":
		name := m.rootSession + "-claude"
		if m.children["-claude"] {
			return m, name, true
		}
	case "l":
		name := m.rootSession + "-lazygit"
		if m.children["-lazygit"] {
			return m, name, true
		}
	case "t":
		name := m.rootSession + "-term"
		if m.children["-term"] {
			return m, name, true
		}
	case "esc":
		return m, "", true
	}
	return m, "", false
}

// View renders the launch popup as a centered modal.
func (m TmuxLaunchModel) View() string {
	var lines []string

	lines = append(lines, tagPickerTitleStyle.Render("Switch to Session"))
	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(theme.Primary).Render(m.rootSession))
	lines = append(lines, "")

	type entry struct {
		key    string
		label  string
		suffix string
	}
	entries := []entry{
		{"r", "root", ""},
		{"c", "claude", "-claude"},
		{"l", "lazygit", "-lazygit"},
		{"t", "term", "-term"},
	}

	for _, e := range entries {
		available := e.suffix == "" || m.children[e.suffix]
		style := listItemStyle
		if !available {
			style = lipgloss.NewStyle().Foreground(theme.TextMuted).Padding(0, 2)
		}
		keyStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.Warning)
		if !available {
			keyStyle = lipgloss.NewStyle().Foreground(theme.TextMuted)
		}
		lines = append(lines, style.Render(keyStyle.Render(e.key)+" "+e.label))
	}

	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("esc: cancel"))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	boxed := tagPickerBoxStyle.Width(40).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, boxed)
}

// listTmuxSessions returns all tmux session names.
func listTmuxSessions() []string {
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var sessions []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			sessions = append(sessions, line)
		}
	}
	return sessions
}

// isChildSession checks if a session name ends with a known child suffix.
func isChildSession(name string) bool {
	for _, suffix := range childSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// getChildSessions checks which child sessions exist for a root session.
func getChildSessions(root string) map[string]bool {
	allSessions := listTmuxSessions()
	sessionSet := make(map[string]bool)
	for _, s := range allSessions {
		sessionSet[s] = true
	}

	children := make(map[string]bool)
	for _, suffix := range childSuffixes {
		children[suffix] = sessionSet[root+suffix]
	}
	return children
}

// switchTmuxSession switches the tmux client to the given session.
func switchTmuxSession(name string) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("tmux", "switch-client", "-t", name).Run()
		return nil
	}
}
