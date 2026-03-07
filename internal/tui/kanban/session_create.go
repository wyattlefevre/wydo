package kanban

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"wydo/internal/tui/theme"
)

type sessionCreateStep int

const (
	sessionCreateStepChoice   sessionCreateStep = iota
	sessionCreateStepName
	sessionCreateStepRepos
	sessionCreateStepExisting
	sessionCreateStepProgress
)

// sessionCreatedMsg is sent when session creation completes.
type sessionCreatedMsg struct {
	sessionName string
	err         error
}

// SessionCreateModel manages the multi-step "start work" modal.
type SessionCreateModel struct {
	step      sessionCreateStep
	cardTitle string

	choiceCursor int

	nameInput textinput.Model

	allRepos       []string
	selectedRepos  map[string]bool
	repoCursor     int
	repoFilterText string
	repoFiltered   []string
	repoFilterMode bool

	existingDirs   []string
	existingCursor int

	progressLines []string
	// progressDone and progressErr are set externally by BoardModel when it
	// receives sessionCreatedMsg (see board.go). SessionCreateModel cannot
	// process sessionCreatedMsg itself because Handle only dispatches KeyMsg.
	progressDone  bool
	progressErr   error

	width, height int
}

// NewSessionCreateModel creates a new multi-step session creation modal.
func NewSessionCreateModel(cardTitle string, width, height int) SessionCreateModel {
	slug := slugify(cardTitle)
	ti := textinput.New()
	ti.SetValue(slug)
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 40

	return SessionCreateModel{
		step:          sessionCreateStepChoice,
		cardTitle:     cardTitle,
		nameInput:     ti,
		selectedRepos: make(map[string]bool),
		width:         width,
		height:        height,
	}
}

// Handle is the entry point called by BoardModel.
func (m SessionCreateModel) Handle(msg tea.Msg) (SessionCreateModel, tea.Cmd, bool) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		return m.handleKey(keyMsg)
	}
	return m, nil, false
}

func (m SessionCreateModel) handleKey(msg tea.KeyMsg) (SessionCreateModel, tea.Cmd, bool) {
	switch m.step {
	case sessionCreateStepChoice:
		return m.handleChoiceKey(msg)
	case sessionCreateStepName:
		return m.handleNameKey(msg)
	case sessionCreateStepRepos:
		return m.handleReposKey(msg)
	case sessionCreateStepExisting:
		return m.handleExistingKey(msg)
	case sessionCreateStepProgress:
		if m.progressDone && msg.String() == "esc" {
			return m, nil, true
		}
	}
	return m, nil, false
}

func (m SessionCreateModel) handleChoiceKey(msg tea.KeyMsg) (SessionCreateModel, tea.Cmd, bool) {
	switch msg.String() {
	case "j", "down":
		if m.choiceCursor < 1 {
			m.choiceCursor++
		}
	case "k", "up":
		if m.choiceCursor > 0 {
			m.choiceCursor--
		}
	case "enter":
		if m.choiceCursor == 0 {
			m.step = sessionCreateStepName
			return m, textinput.Blink, false
		}
		m.existingDirs = listExistingWorktrees()
		m.existingCursor = 0
		m.step = sessionCreateStepExisting
	case "esc":
		return m, nil, true
	}
	return m, nil, false
}

func (m SessionCreateModel) handleNameKey(msg tea.KeyMsg) (SessionCreateModel, tea.Cmd, bool) {
	switch msg.String() {
	case "enter":
		m.step = sessionCreateStepRepos
		m.allRepos = listMainRepos()
		m.repoFiltered = m.allRepos
		m.repoCursor = 0
		return m, nil, false
	case "esc":
		m.step = sessionCreateStepChoice
		return m, nil, false
	default:
		var cmd tea.Cmd
		m.nameInput, cmd = m.nameInput.Update(msg)
		return m, cmd, false
	}
}

func (m SessionCreateModel) handleReposKey(msg tea.KeyMsg) (SessionCreateModel, tea.Cmd, bool) {
	if m.repoFilterMode {
		switch msg.String() {
		case "esc", "enter":
			m.repoFilterMode = false
		case "backspace":
			if len(m.repoFilterText) > 0 {
				m.repoFilterText = m.repoFilterText[:len(m.repoFilterText)-1]
				m = m.filterRepos()
			}
		default:
			if len(msg.String()) == 1 {
				m.repoFilterText += msg.String()
				m = m.filterRepos()
				m.repoCursor = 0
			}
		}
		return m, nil, false
	}

	switch msg.String() {
	case "j", "down":
		if m.repoCursor < len(m.repoFiltered)-1 {
			m.repoCursor++
		}
	case "k", "up":
		if m.repoCursor > 0 {
			m.repoCursor--
		}
	case " ", "tab":
		if m.repoCursor < len(m.repoFiltered) {
			repo := m.repoFiltered[m.repoCursor]
			if m.selectedRepos[repo] {
				delete(m.selectedRepos, repo)
			} else {
				m.selectedRepos[repo] = true
			}
		}
	case "/":
		m.repoFilterMode = true
	case "enter":
		if len(m.selectedRepos) == 0 {
			return m, nil, false
		}
		name := m.nameInput.Value()
		var repos []string
		for r := range m.selectedRepos {
			repos = append(repos, r)
		}
		m.step = sessionCreateStepProgress
		m.progressLines = []string{"Creating worktree " + name + "..."}
		return m, createNewWorktreeSessionCmd(name, repos), false
	case "esc":
		m.step = sessionCreateStepName
	}
	return m, nil, false
}

func (m SessionCreateModel) filterRepos() SessionCreateModel {
	if m.repoFilterText == "" {
		m.repoFiltered = m.allRepos
		return m
	}
	lower := strings.ToLower(m.repoFilterText)
	var filtered []string
	for _, r := range m.allRepos {
		if strings.Contains(strings.ToLower(r), lower) {
			filtered = append(filtered, r)
		}
	}
	m.repoFiltered = filtered
	if m.repoCursor >= len(m.repoFiltered) {
		if len(m.repoFiltered) > 0 {
			m.repoCursor = len(m.repoFiltered) - 1
		} else {
			m.repoCursor = 0
		}
	}
	return m
}

func (m SessionCreateModel) handleExistingKey(msg tea.KeyMsg) (SessionCreateModel, tea.Cmd, bool) {
	switch msg.String() {
	case "j", "down":
		if m.existingCursor < len(m.existingDirs)-1 {
			m.existingCursor++
		}
	case "k", "up":
		if m.existingCursor > 0 {
			m.existingCursor--
		}
	case "enter":
		if len(m.existingDirs) == 0 {
			return m, nil, false
		}
		name := m.existingDirs[m.existingCursor]
		m.step = sessionCreateStepProgress
		m.progressLines = []string{"Attaching to " + name + "..."}
		return m, attachOrCreateSessionCmd(name), false
	case "esc":
		m.step = sessionCreateStepChoice
	}
	return m, nil, false
}

// View renders the current step as a centered modal.
func (m SessionCreateModel) View() string {
	switch m.step {
	case sessionCreateStepChoice:
		return m.viewChoice()
	case sessionCreateStepName:
		return m.viewName()
	case sessionCreateStepRepos:
		return m.viewRepos()
	case sessionCreateStepExisting:
		return m.viewExisting()
	case sessionCreateStepProgress:
		return m.viewProgress()
	}
	return ""
}

func (m SessionCreateModel) viewChoice() string {
	var lines []string
	lines = append(lines, tagPickerTitleStyle.Render("Start Work"))
	lines = append(lines, "")
	choices := []string{"new worktree", "use existing"}
	for i, c := range choices {
		style := listItemStyle
		prefix := "  "
		if i == m.choiceCursor {
			style = selectedListItemStyle
			prefix = "> "
		}
		lines = append(lines, style.Render(prefix+c))
	}
	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("j/k navigate  enter select  esc cancel"))
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	boxed := tagPickerBoxStyle.Width(40).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, boxed)
}

func (m SessionCreateModel) viewName() string {
	var lines []string
	lines = append(lines, tagPickerTitleStyle.Render("Session Name"))
	lines = append(lines, "")
	lines = append(lines, m.nameInput.View())
	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("enter: next  esc: back"))
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	boxed := tagPickerBoxStyle.Width(48).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, boxed)
}

func (m SessionCreateModel) viewRepos() string {
	var lines []string
	lines = append(lines, tagPickerTitleStyle.Render("Select Repos"))
	lines = append(lines, "")
	if m.repoFilterText != "" || m.repoFilterMode {
		filterLine := "/ " + m.repoFilterText
		if m.repoFilterMode {
			filterLine += "_"
		}
		lines = append(lines, helpStyle.Render(filterLine))
	}
	if len(m.repoFiltered) == 0 {
		lines = append(lines, helpStyle.Render("No repos found in ~/worktrees/main/"))
	} else {
		for i, repo := range m.repoFiltered {
			checked := "[ ]"
			if m.selectedRepos[repo] {
				checked = "[x]"
			}
			style := listItemStyle
			if i == m.repoCursor {
				style = selectedListItemStyle
			}
			lines = append(lines, style.Render("  "+checked+" "+repo))
		}
	}
	lines = append(lines, "")
	if len(m.selectedRepos) > 0 {
		lines = append(lines, helpStyle.Render(fmt.Sprintf("%d selected  enter: create  space: toggle  /: filter  esc: back", len(m.selectedRepos))))
	} else {
		lines = append(lines, helpStyle.Render("space: toggle  /: filter  esc: back  (select at least one to continue)"))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	boxed := tagPickerBoxStyle.Width(52).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, boxed)
}

func (m SessionCreateModel) viewExisting() string {
	var lines []string
	lines = append(lines, tagPickerTitleStyle.Render("Select Worktree"))
	lines = append(lines, "")
	if len(m.existingDirs) == 0 {
		lines = append(lines, helpStyle.Render("No worktrees found in ~/worktrees/"))
	} else {
		for i, dir := range m.existingDirs {
			style := listItemStyle
			prefix := "  "
			if i == m.existingCursor {
				style = selectedListItemStyle
				prefix = "> "
			}
			lines = append(lines, style.Render(prefix+dir))
		}
	}
	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("j/k navigate  enter select  esc back"))
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	boxed := tagPickerBoxStyle.Width(44).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, boxed)
}

func (m SessionCreateModel) viewProgress() string {
	var lines []string
	lines = append(lines, tagPickerTitleStyle.Render("Starting..."))
	lines = append(lines, "")
	for _, line := range m.progressLines {
		lines = append(lines, helpStyle.Render(line))
	}
	if m.progressDone {
		lines = append(lines, "")
		if m.progressErr != nil {
			errStyle := lipgloss.NewStyle().Foreground(theme.Danger)
			lines = append(lines, errStyle.Render("Error: "+m.progressErr.Error()))
			lines = append(lines, "")
			lines = append(lines, helpStyle.Render("esc: close"))
		} else {
			okStyle := lipgloss.NewStyle().Foreground(theme.Success)
			lines = append(lines, okStyle.Render("Done! Switching to claude session..."))
		}
	} else {
		lines = append(lines, "")
		lines = append(lines, helpStyle.Render("working..."))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	boxed := tagPickerBoxStyle.Width(50).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, boxed)
}

// claudeSettingsItems mirrors worktree-manager.sh CLAUDE_SETTINGS_ITEMS.
var claudeSettingsItems = []string{".claude", "CLAUDE.md", ".mcp.json"}

func createNewWorktreeSessionCmd(name string, repos []string) tea.Cmd {
	return func() tea.Msg {
		base := worktreesBaseDir()
		mainDir := filepath.Join(base, "main")
		worktreePath := filepath.Join(base, name)

		if err := os.MkdirAll(worktreePath, 0755); err != nil {
			return sessionCreatedMsg{err: fmt.Errorf("mkdir %s: %w", worktreePath, err)}
		}

		// Track successfully created worktrees for cleanup on failure
		var created []string

		for _, repo := range repos {
			repoPath := filepath.Join(mainDir, repo)
			targetPath := filepath.Join(worktreePath, repo)
			_ = exec.Command("git", "-C", repoPath, "pull", "--ff-only", "origin", "main").Run()

			err := exec.Command("git", "-C", repoPath, "worktree", "add", targetPath, "-b", name).Run()
			if err != nil {
				// Branch may already exist; try without -b
				err = exec.Command("git", "-C", repoPath, "worktree", "add", targetPath, name).Run()
			}
			if err != nil {
				// Clean up already-created worktrees
				for _, doneRepo := range created {
					_ = exec.Command("git", "-C", filepath.Join(mainDir, doneRepo), "worktree", "remove", "--force", filepath.Join(worktreePath, doneRepo)).Run()
				}
				_ = os.RemoveAll(worktreePath)
				return sessionCreatedMsg{err: fmt.Errorf("worktree add %s/%s: %w", name, repo, err)}
			}
			created = append(created, repo)
		}

		for _, item := range claudeSettingsItems {
			src := filepath.Join(mainDir, item)
			dst := filepath.Join(worktreePath, item)
			if _, err := os.Lstat(src); err != nil {
				continue
			}
			if _, err := os.Lstat(dst); err == nil {
				continue
			}
			_ = os.Symlink(filepath.Join("..", "main", item), dst)
		}

		_ = exec.Command("tmux", "new-session", "-ds", name, "-c", worktreePath).Run()
		_ = exec.Command("tmux", "new-session", "-ds", name+"-claude", "-c", worktreePath, "claude").Run()

		return sessionCreatedMsg{sessionName: name}
	}
}

func attachOrCreateSessionCmd(name string) tea.Cmd {
	return func() tea.Msg {
		base := worktreesBaseDir()
		workDir := filepath.Join(base, name)

		sessionSet := make(map[string]bool)
		for _, s := range listTmuxSessions() {
			sessionSet[s] = true
		}

		if !sessionSet[name] {
			if err := exec.Command("tmux", "new-session", "-ds", name, "-c", workDir).Run(); err != nil {
				return sessionCreatedMsg{err: err}
			}
		}

		claudeSession := name + "-claude"
		if !sessionSet[claudeSession] {
			if err := exec.Command("tmux", "new-session", "-ds", claudeSession, "-c", workDir, "claude").Run(); err != nil {
				return sessionCreatedMsg{err: err}
			}
		}

		return sessionCreatedMsg{sessionName: name}
	}
}

// slugify converts a card title to a lowercase hyphenated slug suitable for
// use as a worktree/session name.
func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == ' ' || r == '_':
			b.WriteRune('-')
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			// drop
		}
	}
	result := b.String()
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}

// worktreesBaseDir returns the base directory for worktrees, defaulting to ~/worktrees.
func worktreesBaseDir() string {
	if d := os.Getenv("WORKTREES_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "worktrees")
}

// listExistingWorktrees returns subdirectory names under the worktrees base dir,
// excluding "main".
func listExistingWorktrees() []string {
	base := worktreesBaseDir()
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != "main" {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs
}

// listMainRepos returns subdirectory names under ~/worktrees/main/.
func listMainRepos() []string {
	base := filepath.Join(worktreesBaseDir(), "main")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var repos []string
	for _, e := range entries {
		if e.IsDir() {
			repos = append(repos, e.Name())
		}
	}
	return repos
}
