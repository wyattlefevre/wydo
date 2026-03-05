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
	modeScaffoldSelect  // choose which items to scaffold
	modeScaffoldConfirm
	modeSetParent       // selecting new parent for a project
	modeDeleteVirtual   // confirm-delete a virtual project
)

// parentOption is a candidate parent in the reparent selector.
type parentOption struct {
	project *workspace.Project  // nil = root (no parent)
	label   string
	ws      *workspace.Workspace
}

// scaffoldOption is a toggleable item in the scaffold selection screen.
type scaffoldOption struct {
	label   string // display name
	path    string // relative path to create (e.g. "alpha.md", "boards/", "tasks/")
	checked bool
}

// projectEntry pairs a project with its workspace context and tree depth.
type projectEntry struct {
	Project  *workspace.Project
	RootDir  string
	Registry *workspace.ProjectRegistry
	Depth    int // tree depth; 0 = root
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

	// Tree state
	expanded map[string]bool // project name → expanded

	// Create flow state
	selectedWSIdx  int
	selectedDirIdx int
	createDirs     []string // candidate projects/ dirs for create
	createWSDir    string   // chosen workspace root

	// Rename flow state
	renameEntry *projectEntry

	// Scaffold flow state
	scaffoldEntry     *projectEntry    // virtual project pending scaffold confirmation
	scaffoldTargetDir string           // chosen projects/ dir for scaffold
	scaffoldOptions   []scaffoldOption // items user can toggle
	scaffoldOptCursor int              // cursor in scaffold select screen

	// Reparent flow state
	reparentEntry   *projectEntry  // project being reparented
	parentOptions   []parentOption // candidates
	parentOptCursor int

	// Delete virtual project flow state
	deleteEntry     *projectEntry
	deleteTaskCount int
	deleteCardCount int

	width        int
	height       int
	err          error
	showArchived bool
}

func NewProjectsModel(workspaces []*workspace.Workspace) ProjectsModel {
	ti := textinput.New()
	ti.Placeholder = "Search projects..."
	ti.CharLimit = 100
	ti.Width = 40

	m := ProjectsModel{
		textInput: ti,
		expanded:  make(map[string]bool),
	}
	m.SetData(workspaces)
	return m
}

// SetData rebuilds the project entries from fresh workspace data.
func (m *ProjectsModel) SetData(workspaces []*workspace.Workspace) {
	m.workspaces = workspaces
	m.multiWorkspace = len(workspaces) > 1
	if m.expanded == nil {
		m.expanded = make(map[string]bool)
	}
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
	case modeScaffoldSelect:
		return "j/k:navigate  space:toggle  enter:confirm  esc:cancel"
	case modeScaffoldConfirm:
		return "y:create  n/esc:cancel"
	case modeSetParent:
		return "j/k:navigate  enter:confirm  esc:cancel"
	case modeDeleteVirtual:
		return "y:delete  n/esc:cancel"
	default:
		return "j/k:navigate  enter:open  /:search  ?:help  q:quit"
	}
}

func (m *ProjectsModel) buildEntries() {
	m.entries = nil
	for _, ws := range m.workspaces {
		if ws.Projects == nil {
			continue
		}
		// Find root projects (no parent or parent not in this workspace)
		roots := rootProjectsForWS(ws, m.showArchived)
		sort.Slice(roots, func(i, j int) bool {
			return strings.ToLower(roots[i].Name) < strings.ToLower(roots[j].Name)
		})
		for _, root := range roots {
			m.appendProjectTree(root, 0, ws)
		}
	}
}

func rootProjectsForWS(ws *workspace.Workspace, showArchived bool) []*workspace.Project {
	var roots []*workspace.Project
	for _, p := range ws.Projects.List() {
		if !showArchived && p.Archived {
			continue
		}
		if p.Parent == "" || ws.Projects.Get(p.Parent) == nil {
			roots = append(roots, p)
		}
	}
	return roots
}

func (m *ProjectsModel) appendProjectTree(p *workspace.Project, depth int, ws *workspace.Workspace) {
	m.entries = append(m.entries, projectEntry{
		Project:  p,
		RootDir:  ws.RootDir,
		Registry: ws.Projects,
		Depth:    depth,
	})
	if m.isExpanded(p.Name) {
		children := ws.Projects.ChildrenOf(p.Name)
		sort.Slice(children, func(i, j int) bool {
			return strings.ToLower(children[i].Name) < strings.ToLower(children[j].Name)
		})
		for _, child := range children {
			if !m.showArchived && child.Archived {
				continue
			}
			m.appendProjectTree(child, depth+1, ws)
		}
	}
}

func (m *ProjectsModel) isExpanded(name string) bool {
	return m.expanded[name]
}

func (m *ProjectsModel) hasChildren(entry projectEntry) bool {
	return len(entry.Registry.ChildrenOf(entry.Project.Name)) > 0
}

func (m *ProjectsModel) applyFilter() {
	if m.searchQuery == "" {
		// Show all entries (tree is already built with expand/collapse state)
		m.filtered = make([]int, len(m.entries))
		for i := range m.entries {
			m.filtered[i] = i
		}
	} else {
		// Fuzzy match on all entries ignoring tree structure
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
		case modeScaffoldSelect:
			return m.updateScaffoldSelect(msg)
		case modeScaffoldConfirm:
			return m.updateScaffoldConfirm(msg)
		case modeSetParent:
			return m.updateSetParent(msg)
		case modeDeleteVirtual:
			return m.updateDeleteVirtual(msg)
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

	case " ", "right", "tab", "l":
		// Toggle expand/collapse for projects with children
		if len(m.filtered) > 0 && m.selected < len(m.filtered) {
			entry := m.entries[m.filtered[m.selected]]
			if m.hasChildren(entry) {
				m.expanded[entry.Project.Name] = !m.expanded[entry.Project.Name]
				m.buildEntries()
				m.applyFilter()
			}
		}

	case "left":
		// Collapse current project (if expanded), or no-op
		if len(m.filtered) > 0 && m.selected < len(m.filtered) {
			entry := m.entries[m.filtered[m.selected]]
			if m.expanded[entry.Project.Name] {
				m.expanded[entry.Project.Name] = false
				m.buildEntries()
				m.applyFilter()
			}
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
			if entry.Project.DirPath == "" {
				// Virtual project — prompt to scaffold
				return m.startScaffold(entry)
			}
			return m, func() tea.Msg {
				return messages.OpenProjectMsg{
					ProjectName:      entry.Project.Name,
					WorkspaceRootDir: entry.RootDir,
				}
			}
		}

	case "p":
		if len(m.filtered) > 0 && m.selected < len(m.filtered) {
			entry := m.entries[m.filtered[m.selected]]
			return m.startSetParent(entry)
		}

	case "a":
		if len(m.filtered) > 0 && m.selected < len(m.filtered) {
			entry := m.entries[m.filtered[m.selected]]
			newArchived := !entry.Project.Archived
			var err error
			if entry.Project.DirPath == "" {
				err = workspace.SetVirtualProjectArchived(entry.RootDir, entry.Project, newArchived)
			} else {
				err = workspace.SetProjectArchived(entry.Project, newArchived)
			}
			if err != nil {
				m.err = err
			} else {
				m.err = nil
				m.buildEntries()
				m.applyFilter()
			}
		}

	case "D":
		if len(m.filtered) > 0 && m.selected < len(m.filtered) {
			entry := m.entries[m.filtered[m.selected]]
			if entry.Project.DirPath == "" {
				m.deleteEntry = &entry
				m.deleteTaskCount = countTasksForProject(entry, m.workspaces)
				m.deleteCardCount = countCardsForProject(entry, m.workspaces)
				m.mode = modeDeleteVirtual
				m.err = nil
			}
			// D on physical project: no-op
		}

	case "ctrl+a":
		m.showArchived = !m.showArchived
		m.buildEntries()
		m.applyFilter()
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

func (m ProjectsModel) startScaffold(entry projectEntry) (ProjectsModel, tea.Cmd) {
	m.err = nil
	m.scaffoldEntry = &entry
	m.createDirs = entry.Registry.ProjectsDirs(entry.RootDir)
	if len(m.createDirs) > 1 {
		m.mode = modeSelectDir
		m.selectedDirIdx = 0
		// After dir selection, scaffoldEntry is set → updateSelectDir goes to modeScaffoldSelect
		return m, nil
	}
	if len(m.createDirs) > 0 {
		m.scaffoldTargetDir = m.createDirs[0]
	} else {
		m.scaffoldTargetDir = filepath.Join(entry.RootDir, "projects")
	}
	return m.startScaffoldSelect()
}

func (m ProjectsModel) startScaffoldSelect() (ProjectsModel, tea.Cmd) {
	name := ""
	if m.scaffoldEntry != nil {
		name = m.scaffoldEntry.Project.Name
	}
	m.scaffoldOptions = []scaffoldOption{
		{label: name + ".md (index note)", path: name + ".md", checked: true},
		{label: "boards/ directory", path: "boards/", checked: true},
		{label: "tasks/ directory", path: "tasks/", checked: true},
	}
	m.scaffoldOptCursor = 0
	m.mode = modeScaffoldSelect
	return m, nil
}

func (m ProjectsModel) updateScaffoldSelect(msg tea.KeyMsg) (ProjectsModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.scaffoldEntry = nil
		m.mode = modeList
	case "j", "down":
		if m.scaffoldOptCursor < len(m.scaffoldOptions)-1 {
			m.scaffoldOptCursor++
		}
	case "k", "up":
		if m.scaffoldOptCursor > 0 {
			m.scaffoldOptCursor--
		}
	case " ":
		if m.scaffoldOptCursor < len(m.scaffoldOptions) {
			m.scaffoldOptions[m.scaffoldOptCursor].checked = !m.scaffoldOptions[m.scaffoldOptCursor].checked
		}
	case "enter":
		m.mode = modeScaffoldConfirm
	}
	return m, nil
}

func (m ProjectsModel) updateScaffoldConfirm(msg tea.KeyMsg) (ProjectsModel, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.scaffoldEntry == nil {
			m.mode = modeList
			return m, nil
		}
		name := m.scaffoldEntry.Project.Name
		projectDir := filepath.Join(m.scaffoldTargetDir, name)

		// Always create the project directory itself
		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			m.err = fmt.Errorf("failed to create directory: %w", err)
			m.mode = modeList
			return m, nil
		}

		// Create/write only checked options
		for _, opt := range m.scaffoldOptions {
			if !opt.checked {
				continue
			}
			fullPath := filepath.Join(projectDir, opt.path)
			if strings.HasSuffix(opt.path, "/") {
				// Directory
				if err := os.MkdirAll(fullPath, 0o755); err != nil {
					m.err = fmt.Errorf("failed to create directory %s: %w", opt.path, err)
					m.mode = modeList
					return m, nil
				}
			} else {
				// File (index note)
				content := fmt.Sprintf("# %s\n", name)
				if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
					m.err = fmt.Errorf("failed to write %s: %w", opt.path, err)
					m.mode = modeList
					return m, nil
				}
			}
		}

		wsRoot := m.scaffoldEntry.RootDir
		entryName := m.scaffoldEntry.Project.Name
		m.scaffoldEntry = nil
		m.mode = modeList
		_ = workspace.RemoveFromVirtualArchive(wsRoot, entryName)
		return m, func() tea.Msg { return messages.DataRefreshMsg{} }

	case "n", "N", "esc":
		m.scaffoldEntry = nil
		m.mode = modeList
		return m, nil
	}
	return m, nil
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
			if m.scaffoldEntry != nil {
				// We came here from scaffold flow
				m.scaffoldTargetDir = m.createDirs[m.selectedDirIdx]
				return m.startScaffoldSelect()
			}
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

		_ = workspace.RemoveFromVirtualArchive(m.createWSDir, name)
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
	case modeScaffoldSelect:
		return m.viewScaffoldSelect()
	case modeScaffoldConfirm:
		return m.viewScaffoldConfirm()
	case modeSetParent:
		return m.viewSetParent()
	case modeDeleteVirtual:
		return m.viewDeleteVirtual()
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
			cursorPrefix := "  "
			if i == m.selected {
				style = selectedListItemStyle
				cursorPrefix = "► "
			}

			// Tree indentation
			indent := strings.Repeat("  ", entry.Depth)

			// Tree expand/collapse prefix
			var treePrefix string
			if m.hasChildren(entry) {
				if m.isExpanded(entry.Project.Name) {
					treePrefix = "▼ "
				} else {
					treePrefix = "▶ "
				}
			} else {
				treePrefix = "  "
			}

			name := entry.Project.Name
			var suffix string
			if entry.Project.Archived {
				suffix = " " + virtualBadgeStyle.Render("[archived]")
			} else if entry.Project.DirPath == "" {
				suffix = " " + virtualBadgeStyle.Render("(virtual)")
			}
			if m.multiWorkspace {
				suffix += " " + pathStyle.Render(abbreviatePath(entry.RootDir))
			}

			lines = append(lines, style.Render(cursorPrefix+indent+treePrefix+name)+suffix)
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

func (m ProjectsModel) viewScaffoldSelect() string {
	var lines []string
	name := ""
	if m.scaffoldEntry != nil {
		name = m.scaffoldEntry.Project.Name
	}
	lines = append(lines, titleStyle.Render(fmt.Sprintf("Select items to create for %q:", name)))
	lines = append(lines, "")
	for i, opt := range m.scaffoldOptions {
		cursor := "  "
		if i == m.scaffoldOptCursor {
			cursor = "► "
		}
		check := "[ ]"
		if opt.checked {
			check = "[x]"
		}
		lines = append(lines, listItemStyle.Render(cursor+check+" "+opt.label))
	}
	lines = append(lines, "")
	lines = append(lines, pathStyle.Render("  space:toggle  enter:confirm  esc:cancel"))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m ProjectsModel) viewScaffoldConfirm() string {
	var lines []string
	name := ""
	if m.scaffoldEntry != nil {
		name = m.scaffoldEntry.Project.Name
	}
	lines = append(lines, titleStyle.Render(fmt.Sprintf("Scaffold project directory for %q?", name)))
	lines = append(lines, "")
	lines = append(lines, listItemStyle.Render("Will create:"))
	targetDir := m.scaffoldTargetDir
	if targetDir == "" {
		targetDir = "~/projects"
	}
	projectDir := filepath.Join(targetDir, name)
	lines = append(lines, pathStyle.Render("  "+abbreviatePath(projectDir)+"/"))
	for _, opt := range m.scaffoldOptions {
		if opt.checked {
			lines = append(lines, pathStyle.Render("  "+abbreviatePath(filepath.Join(projectDir, opt.path))))
		}
	}
	lines = append(lines, "")
	lines = append(lines, listItemStyle.Render("[y] Create   [n/esc] Cancel"))

	if m.err != nil {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func countTasksForProject(entry projectEntry, workspaces []*workspace.Workspace) int {
	for _, ws := range workspaces {
		if ws.RootDir != entry.RootDir {
			continue
		}
		count := 0
		for _, t := range ws.Tasks {
			if t.HasProject(entry.Project.Name) {
				count++
			}
		}
		return count
	}
	return 0
}

func countCardsForProject(entry projectEntry, workspaces []*workspace.Workspace) int {
	for _, ws := range workspaces {
		if ws.RootDir != entry.RootDir {
			continue
		}
		count := 0
		for _, board := range ws.Boards {
			for _, col := range board.Columns {
				for _, card := range col.Cards {
					for _, p := range card.Projects {
						if strings.EqualFold(p, entry.Project.Name) {
							count++
							break
						}
					}
				}
			}
		}
		return count
	}
	return 0
}

func (m ProjectsModel) updateDeleteVirtual(msg tea.KeyMsg) (ProjectsModel, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		var ws *workspace.Workspace
		for _, w := range m.workspaces {
			if w.RootDir == m.deleteEntry.RootDir {
				ws = w
				break
			}
		}
		if ws == nil {
			m.err = fmt.Errorf("workspace not found")
			m.mode = modeList
			m.deleteEntry = nil
			return m, nil
		}
		name := m.deleteEntry.Project.Name
		if err := workspace.DeleteVirtualProject(ws, name); err != nil {
			m.err = err
			m.mode = modeList
			m.deleteEntry = nil
			return m, nil
		}
		m.deleteEntry = nil
		m.mode = modeList
		return m, func() tea.Msg { return messages.DataRefreshMsg{} }
	case "n", "N", "esc":
		m.deleteEntry = nil
		m.mode = modeList
	}
	return m, nil
}

func (m ProjectsModel) viewDeleteVirtual() string {
	name := ""
	if m.deleteEntry != nil {
		name = m.deleteEntry.Project.Name
	}
	taskWord := "task"
	if m.deleteTaskCount != 1 {
		taskWord = "tasks"
	}
	cardWord := "card"
	if m.deleteCardCount != 1 {
		cardWord = "cards"
	}

	var lines []string
	lines = append(lines, titleStyle.Render("Delete Virtual Project"))
	lines = append(lines, "")
	lines = append(lines, listItemStyle.Render(fmt.Sprintf("Delete %q?", name)))
	lines = append(lines, listItemStyle.Render(fmt.Sprintf(
		"Will remove from %d %s and %d %s.",
		m.deleteTaskCount, taskWord, m.deleteCardCount, cardWord,
	)))
	lines = append(lines, "")
	lines = append(lines, listItemStyle.Render("[y] Delete   [n/esc] Cancel"))
	if m.err != nil {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// collectDescendants returns a set of all transitive descendant project names.
func collectDescendants(name string, reg *workspace.ProjectRegistry) map[string]bool {
	result := make(map[string]bool)
	queue := []string{name}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, child := range reg.ChildrenOf(cur) {
			if !result[child.Name] {
				result[child.Name] = true
				queue = append(queue, child.Name)
			}
		}
	}
	return result
}

// startSetParent begins the reparent flow for the given entry.
func (m ProjectsModel) startSetParent(entry projectEntry) (ProjectsModel, tea.Cmd) {
	if entry.Project.DirPath == "" {
		m.err = fmt.Errorf("cannot reparent virtual project")
		return m, nil
	}
	m.reparentEntry = &entry
	descendants := collectDescendants(entry.Project.Name, entry.Registry)

	// Always offer "root" as the first option
	m.parentOptions = []parentOption{{label: "(root — no parent)", project: nil, ws: nil}}

	for i := range m.entries {
		e := &m.entries[i]
		if e.Project.Name == entry.Project.Name {
			continue
		}
		if descendants[e.Project.Name] {
			continue
		}
		if e.Project.DirPath == "" {
			continue // skip virtual
		}
		var ws *workspace.Workspace
		for _, w := range m.workspaces {
			if w.RootDir == e.RootDir {
				ws = w
				break
			}
		}
		m.parentOptions = append(m.parentOptions, parentOption{
			project: e.Project,
			label:   e.Project.Name,
			ws:      ws,
		})
	}
	m.parentOptCursor = 0
	m.mode = modeSetParent
	return m, nil
}

func (m ProjectsModel) updateSetParent(msg tea.KeyMsg) (ProjectsModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.reparentEntry = nil
		m.mode = modeList
	case "j", "down":
		if m.parentOptCursor < len(m.parentOptions)-1 {
			m.parentOptCursor++
		}
	case "k", "up":
		if m.parentOptCursor > 0 {
			m.parentOptCursor--
		}
	case "enter":
		if m.reparentEntry == nil {
			m.mode = modeList
			return m, nil
		}
		opt := m.parentOptions[m.parentOptCursor]
		// Find workspace for the project being reparented
		var ws *workspace.Workspace
		for _, w := range m.workspaces {
			if w.RootDir == m.reparentEntry.RootDir {
				ws = w
				break
			}
		}
		if ws == nil {
			m.err = fmt.Errorf("workspace not found")
			m.mode = modeList
			return m, nil
		}
		if err := ws.MoveProjectToParent(m.reparentEntry.Project, opt.project); err != nil {
			m.err = err
			m.mode = modeList
			return m, nil
		}
		m.reparentEntry = nil
		m.mode = modeList
		return m, func() tea.Msg { return messages.DataRefreshMsg{} }
	}
	return m, nil
}

func (m ProjectsModel) viewSetParent() string {
	var lines []string
	name := ""
	if m.reparentEntry != nil {
		name = m.reparentEntry.Project.Name
	}
	lines = append(lines, titleStyle.Render(fmt.Sprintf("Move %q under:", name)))
	lines = append(lines, "")

	maxVisible := m.height - 8
	if maxVisible < 3 {
		maxVisible = 3
	}
	startIdx := 0
	if m.parentOptCursor >= maxVisible {
		startIdx = m.parentOptCursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(m.parentOptions) {
		endIdx = len(m.parentOptions)
	}

	for i := startIdx; i < endIdx; i++ {
		opt := m.parentOptions[i]
		style := listItemStyle
		prefix := "  "
		if i == m.parentOptCursor {
			style = selectedListItemStyle
			prefix = "► "
		}
		lines = append(lines, style.Render(prefix+opt.label))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
