package projects

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"wydo/internal/workspace"
	"wydo/internal/tui/messages"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

type projectMode int

const (
	modeList projectMode = iota
	modeSearch
	modeSelectWorkspace
	modeSelectDir
	modeCreate
	modeRename
)

// projectEntry pairs a project with its workspace context.
type projectEntry struct {
	Project  *workspace.Project
	RootDir  string
	Registry *workspace.ProjectRegistry
}

// ProjectsModel is the main projects list view.
type ProjectsModel struct {
	workspaces     []*workspace.Workspace
	entries        []projectEntry
	filtered       []int // indices into entries
	selected       int
	mode           projectMode
	textInput      textinput.Model
	searchQuery    string
	multiWorkspace bool

	// Create flow state
	selectedWSIdx  int
	selectedDirIdx int
	createDirs     []string  // candidate projects/ dirs for create
	createWSDir    string    // chosen workspace root

	// Rename flow state
	renameEntry *projectEntry

	width  int
	height int
	err    error
}

func NewProjectsModel(workspaces []*workspace.Workspace) ProjectsModel {
	ti := textinput.New()
	ti.Placeholder = "Search projects..."
	ti.CharLimit = 100
	ti.Width = 40

	m := ProjectsModel{
		textInput: ti,
	}
	m.SetData(workspaces)
	return m
}

// SetData rebuilds the project entries from fresh workspace data.
func (m *ProjectsModel) SetData(workspaces []*workspace.Workspace) {
	m.workspaces = workspaces
	m.multiWorkspace = len(workspaces) > 1
	m.buildEntries()
	m.applyFilter()
}

// SetSize updates view dimensions.
func (m *ProjectsModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// IsTyping returns true when the view has an active text input.
func (m ProjectsModel) IsTyping() bool {
	return m.mode == modeSearch || m.mode == modeCreate || m.mode == modeRename
}

// HintText returns the raw hint string for the current projects mode.
func (m ProjectsModel) HintText() string {
	switch m.mode {
	case modeSearch:
		return "type to filter  enter:confirm  esc:cancel"
	case modeSelectWorkspace:
		return "j/k:navigate  enter:select  esc:cancel"
	case modeSelectDir:
		return "j/k:navigate  enter:select  esc:cancel"
	case modeCreate:
		return "enter:create  esc:cancel"
	case modeRename:
		return "enter:rename  esc:cancel"
	default:
		return "j/k:navigate  /:search  enter:open  n:new  r:rename  ?:help  q:quit"
	}
}

func (m *ProjectsModel) buildEntries() {
	m.entries = nil
	for _, ws := range m.workspaces {
		if ws.Projects == nil {
			continue
		}
		for _, p := range ws.Projects.List() {
			m.entries = append(m.entries, projectEntry{
				Project:  p,
				RootDir:  ws.RootDir,
				Registry: ws.Projects,
			})
		}
	}
	sort.Slice(m.entries, func(i, j int) bool {
		return strings.ToLower(m.entries[i].Project.Name) < strings.ToLower(m.entries[j].Project.Name)
	})
}

func (m *ProjectsModel) applyFilter() {
	if m.searchQuery == "" {
		m.filtered = make([]int, len(m.entries))
		for i := range m.entries {
			m.filtered[i] = i
		}
	} else {
		names := make([]string, len(m.entries))
		for i, e := range m.entries {
			names[i] = e.Project.Name
		}
		matches := fuzzy.Find(m.searchQuery, names)
		m.filtered = make([]int, len(matches))
		for i, match := range matches {
			m.filtered[i] = match.Index
		}
	}
	if m.selected >= len(m.filtered) {
		m.selected = max(0, len(m.filtered)-1)
	}
}

// abbreviatePath replaces home directory with ~
func abbreviatePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

func (m ProjectsModel) Update(msg tea.Msg) (ProjectsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			return m.updateList(msg)
		case modeSearch:
			return m.updateSearch(msg)
		case modeSelectWorkspace:
			return m.updateSelectWorkspace(msg)
		case modeSelectDir:
			return m.updateSelectDir(msg)
		case modeCreate:
			return m.updateCreate(msg)
		case modeRename:
			return m.updateRename(msg)
		}
	}
	return m, nil
}

func (m ProjectsModel) updateList(msg tea.KeyMsg) (ProjectsModel, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit

	case "esc":
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.applyFilter()
			return m, nil
		}
		return m, messages.SwitchView(messages.ViewAgendaDay)

	case "j", "down":
		if m.selected < len(m.filtered)-1 {
			m.selected++
		}

	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}

	case "/":
		m.mode = modeSearch
		m.textInput.SetValue(m.searchQuery)
		m.textInput.Focus()
		return m, textinput.Blink

	case "n":
		return m.startCreate()

	case "r":
		if len(m.filtered) > 0 && m.selected < len(m.filtered) {
			entry := m.entries[m.filtered[m.selected]]
			m.renameEntry = &entry
			m.mode = modeRename
			m.err = nil
			m.textInput.SetValue(entry.Project.Name)
			m.textInput.Placeholder = "New project name..."
			m.textInput.Focus()
			return m, textinput.Blink
		}

	case "enter":
		if len(m.filtered) > 0 && m.selected < len(m.filtered) {
			entry := m.entries[m.filtered[m.selected]]
			return m, func() tea.Msg {
				return messages.OpenProjectMsg{
					ProjectName:      entry.Project.Name,
					WorkspaceRootDir: entry.RootDir,
				}
			}
		}
	}
	return m, nil
}

func (m ProjectsModel) updateSearch(msg tea.KeyMsg) (ProjectsModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.searchQuery = ""
		m.textInput.SetValue("")
		m.applyFilter()
		return m, nil

	case "enter":
		m.searchQuery = m.textInput.Value()
		m.mode = modeList
		m.applyFilter()
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	m.searchQuery = m.textInput.Value()
	m.applyFilter()
	return m, cmd
}

func (m ProjectsModel) startCreate() (ProjectsModel, tea.Cmd) {
	m.err = nil

	// Pre-fill name if selected project is virtual
	prefill := ""
	if len(m.filtered) > 0 && m.selected < len(m.filtered) {
		entry := m.entries[m.filtered[m.selected]]
		if entry.Project.DirPath == "" {
			prefill = entry.Project.Name
			// Use this workspace for create
			m.createWSDir = entry.RootDir
			m.createDirs = entry.Registry.ProjectsDirs(entry.RootDir)
			if len(m.createDirs) > 1 {
				m.mode = modeSelectDir
				m.selectedDirIdx = 0
				return m, nil
			}
			m.mode = modeCreate
			m.textInput.SetValue(prefill)
			m.textInput.Placeholder = "Project name..."
			m.textInput.Focus()
			return m, textinput.Blink
		}
	}

	// If multiple workspaces and no specific virtual project, pick workspace first
	if m.multiWorkspace {
		m.mode = modeSelectWorkspace
		m.selectedWSIdx = 0
		return m, nil
	}

	// Single workspace — pick dir or go straight to create
	if len(m.workspaces) > 0 {
		ws := m.workspaces[0]
		m.createWSDir = ws.RootDir
		if ws.Projects != nil {
			m.createDirs = ws.Projects.ProjectsDirs(ws.RootDir)
		} else {
			m.createDirs = []string{filepath.Join(ws.RootDir, "projects")}
		}
		if len(m.createDirs) > 1 {
			m.mode = modeSelectDir
			m.selectedDirIdx = 0
			return m, nil
		}
	}

	m.mode = modeCreate
	m.textInput.SetValue(prefill)
	m.textInput.Placeholder = "Project name..."
	m.textInput.Focus()
	return m, textinput.Blink
}

func (m ProjectsModel) updateSelectWorkspace(msg tea.KeyMsg) (ProjectsModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil

	case "j", "down":
		if m.selectedWSIdx < len(m.workspaces)-1 {
			m.selectedWSIdx++
		}

	case "k", "up":
		if m.selectedWSIdx > 0 {
			m.selectedWSIdx--
		}

	case "enter":
		if m.selectedWSIdx < len(m.workspaces) {
			ws := m.workspaces[m.selectedWSIdx]
			m.createWSDir = ws.RootDir
			if ws.Projects != nil {
				m.createDirs = ws.Projects.ProjectsDirs(ws.RootDir)
			} else {
				m.createDirs = []string{filepath.Join(ws.RootDir, "projects")}
			}
			if len(m.createDirs) > 1 {
				m.mode = modeSelectDir
				m.selectedDirIdx = 0
				return m, nil
			}
			m.mode = modeCreate
			m.textInput.SetValue("")
			m.textInput.Placeholder = "Project name..."
			m.textInput.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m ProjectsModel) updateSelectDir(msg tea.KeyMsg) (ProjectsModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil

	case "j", "down":
		if m.selectedDirIdx < len(m.createDirs)-1 {
			m.selectedDirIdx++
		}

	case "k", "up":
		if m.selectedDirIdx > 0 {
			m.selectedDirIdx--
		}

	case "enter":
		if m.selectedDirIdx < len(m.createDirs) {
			m.mode = modeCreate
			m.textInput.Placeholder = "Project name..."
			m.textInput.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m ProjectsModel) updateCreate(msg tea.KeyMsg) (ProjectsModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.textInput.SetValue("")
		return m, nil

	case "enter":
		name := strings.TrimSpace(m.textInput.Value())
		if name == "" {
			return m, nil
		}

		// Determine target dir
		var targetDir string
		if len(m.createDirs) > 0 && m.selectedDirIdx < len(m.createDirs) {
			targetDir = m.createDirs[m.selectedDirIdx]
		} else if m.createWSDir != "" {
			targetDir = filepath.Join(m.createWSDir, "projects")
		} else if len(m.workspaces) > 0 {
			targetDir = filepath.Join(m.workspaces[0].RootDir, "projects")
		} else {
			m.err = fmt.Errorf("no workspace configured")
			return m, nil
		}

		projectDir := filepath.Join(targetDir, name)
		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			m.err = fmt.Errorf("failed to create directory: %w", err)
			return m, nil
		}

		// Write an index note
		indexPath := filepath.Join(projectDir, name+".md")
		content := fmt.Sprintf("# %s\n", name)
		if err := os.WriteFile(indexPath, []byte(content), 0o644); err != nil {
			m.err = fmt.Errorf("failed to write index note: %w", err)
			return m, nil
		}

		m.mode = modeList
		m.textInput.SetValue("")
		return m, func() tea.Msg { return messages.DataRefreshMsg{} }
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m ProjectsModel) updateRename(msg tea.KeyMsg) (ProjectsModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.renameEntry = nil
		m.textInput.SetValue("")
		return m, nil

	case "enter":
		newName := strings.TrimSpace(m.textInput.Value())
		if newName == "" || newName == m.renameEntry.Project.Name {
			m.mode = modeList
			m.renameEntry = nil
			m.textInput.SetValue("")
			return m, nil
		}

		// Find workspace for this entry
		var ws *workspace.Workspace
		for _, w := range m.workspaces {
			if w.RootDir == m.renameEntry.RootDir {
				ws = w
				break
			}
		}
		if ws == nil {
			m.err = fmt.Errorf("workspace not found")
			return m, nil
		}

		if err := ws.RenameProject(m.renameEntry.Project.Name, newName); err != nil {
			m.err = err
			return m, nil
		}

		m.mode = modeList
		m.renameEntry = nil
		m.textInput.SetValue("")
		return m, func() tea.Msg { return messages.DataRefreshMsg{} }
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m ProjectsModel) viewRename() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Rename Project"))
	lines = append(lines, "")
	if m.renameEntry != nil {
		lines = append(lines, listItemStyle.Render("Current: "+m.renameEntry.Project.Name))
		lines = append(lines, "")
	}
	lines = append(lines, "  "+m.textInput.View())
	lines = append(lines, "")

	if m.err != nil {
		lines = append(lines, errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		lines = append(lines, "")
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m ProjectsModel) View() string {
	switch m.mode {
	case modeSearch:
		return m.viewSearch()
	case modeSelectWorkspace:
		return m.viewSelectWorkspace()
	case modeSelectDir:
		return m.viewSelectDir()
	case modeCreate:
		return m.viewCreate()
	case modeRename:
		return m.viewRename()
	default:
		return m.viewList()
	}
}

func (m ProjectsModel) viewList() string {
	var lines []string

	lines = append(lines, titleStyle.Render("Projects"))
	lines = append(lines, "")

	if m.searchQuery != "" {
		lines = append(lines, searchLabelStyle.Render("  Filter: ")+pathStyle.Render(m.searchQuery))
		lines = append(lines, "")
	}

	if len(m.filtered) == 0 {
		if len(m.entries) == 0 {
			lines = append(lines, listItemStyle.Render("No projects found. Press 'n' to create one."))
		} else {
			lines = append(lines, listItemStyle.Render("No matching projects."))
		}
		lines = append(lines, "")
	} else {
		// Calculate visible range for scrolling
		maxVisible := m.height - 8 // header + help + margins
		if maxVisible < 3 {
			maxVisible = 3
		}
		startIdx := 0
		if m.selected >= maxVisible {
			startIdx = m.selected - maxVisible + 1
		}
		endIdx := startIdx + maxVisible
		if endIdx > len(m.filtered) {
			endIdx = len(m.filtered)
		}

		for i := startIdx; i < endIdx; i++ {
			entry := m.entries[m.filtered[i]]
			style := listItemStyle
			prefix := "  "
			if i == m.selected {
				style = selectedListItemStyle
				prefix = "► "
			}

			name := entry.Project.Name
			var suffix string
			if entry.Project.DirPath == "" {
				suffix = " " + virtualBadgeStyle.Render("(virtual)")
			}
			if m.multiWorkspace {
				suffix += " " + pathStyle.Render(abbreviatePath(entry.RootDir))
			}

			lines = append(lines, style.Render(prefix+name)+suffix)
		}

		if startIdx > 0 {
			lines = append([]string{lines[0], lines[1], pathStyle.Render(fmt.Sprintf("  ▲ %d more above", startIdx))}, lines[2:]...)
		}
		if endIdx < len(m.filtered) {
			lines = append(lines, pathStyle.Render(fmt.Sprintf("  ▼ %d more below", len(m.filtered)-endIdx)))
		}
		lines = append(lines, "")
	}

	if m.err != nil {
		lines = append(lines, errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		lines = append(lines, "")
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m ProjectsModel) viewSearch() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Search Projects"))
	lines = append(lines, "")
	lines = append(lines, "  "+m.textInput.View())
	lines = append(lines, "")

	// Show live results
	if len(m.filtered) > 0 {
		show := min(8, len(m.filtered))
		for i := 0; i < show; i++ {
			entry := m.entries[m.filtered[i]]
			prefix := "  "
			if i == m.selected {
				prefix = "► "
			}
			name := entry.Project.Name
			if entry.Project.DirPath == "" {
				name += " " + virtualBadgeStyle.Render("(virtual)")
			}
			lines = append(lines, listItemStyle.Render(prefix+name))
		}
		if len(m.filtered) > show {
			lines = append(lines, pathStyle.Render(fmt.Sprintf("  ... %d more", len(m.filtered)-show)))
		}
	} else {
		lines = append(lines, listItemStyle.Render("  No matches"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m ProjectsModel) viewSelectWorkspace() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Select Workspace"))
	lines = append(lines, "")

	for i, ws := range m.workspaces {
		style := listItemStyle
		prefix := "  "
		if i == m.selectedWSIdx {
			style = selectedListItemStyle
			prefix = "► "
		}
		lines = append(lines, style.Render(prefix+abbreviatePath(ws.RootDir)))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m ProjectsModel) viewSelectDir() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Select Directory"))
	lines = append(lines, "")

	for i, dir := range m.createDirs {
		style := listItemStyle
		prefix := "  "
		if i == m.selectedDirIdx {
			style = selectedListItemStyle
			prefix = "► "
		}
		lines = append(lines, style.Render(prefix+abbreviatePath(dir)))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m ProjectsModel) viewCreate() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Create New Project"))
	lines = append(lines, "")
	lines = append(lines, "  "+m.textInput.View())
	lines = append(lines, "")

	if m.err != nil {
		lines = append(lines, errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		lines = append(lines, "")
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
