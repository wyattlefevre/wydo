package notes

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	notespkg "wydo/internal/notes"
	"wydo/internal/tui/messages"
	"wydo/internal/workspace"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type notesMode int

const (
	modeList            notesMode = iota
	modeSelectWorkspace           // pick which workspace to pin into
	modePickFile                  // fuzzy-pick a file from the workspace
	modeInputLabel                // type a label for the pinned note
)

// wsEntry holds a workspace and its pinned notes for display.
type wsEntry struct {
	ws    *workspace.Workspace
	notes []notespkg.PinnedNote
}

// NotesModel is the pinned-notes tab view.
type NotesModel struct {
	workspaces []*workspace.Workspace
	entries    []wsEntry // one per workspace, in order

	// flat list of all pinned notes (for cursor navigation)
	flat    []flatEntry
	cursor  int
	mode    notesMode
	width   int
	height  int

	// pin flow: workspace selection
	selectedWSIdx int

	// pin flow: file picker
	pinWsRoot  string
	allFiles   []string
	filtered   []string
	fileQuery  string
	fileCursor int
	fileInput  textinput.Model

	// pin flow: label input
	selectedRelPath string
	labelInput      textinput.Model
}

// flatEntry maps cursor position to a pinned note.
type flatEntry struct {
	wsRoot string
	note   notespkg.PinnedNote
}

// editorFinishedMsg is sent when the editor process exits.
type editorFinishedMsg struct{ err error }

func NewNotesModel(workspaces []*workspace.Workspace) NotesModel {
	fi := textinput.New()
	fi.Placeholder = "type to filter..."
	fi.CharLimit = 256
	fi.Width = 50

	li := textinput.New()
	li.Placeholder = "Label for this note..."
	li.CharLimit = 100
	li.Width = 50

	m := NotesModel{
		fileInput:  fi,
		labelInput: li,
	}
	m.SetData(workspaces)
	return m
}

// SetData rebuilds the entries from fresh workspace data.
func (m *NotesModel) SetData(workspaces []*workspace.Workspace) {
	m.workspaces = workspaces
	m.entries = nil
	m.flat = nil

	for _, ws := range workspaces {
		pinned, _ := notespkg.ReadPinnedNotes(ws.RootDir)
		m.entries = append(m.entries, wsEntry{ws: ws, notes: pinned})
		for _, n := range pinned {
			m.flat = append(m.flat, flatEntry{wsRoot: ws.RootDir, note: n})
		}
	}

	if m.cursor >= len(m.flat) {
		m.cursor = max(0, len(m.flat)-1)
	}
}

func (m *NotesModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// HintText returns the hint bar text for the current mode.
func (m NotesModel) HintText() string {
	switch m.mode {
	case modeSelectWorkspace:
		return "j/k:navigate  enter:select  esc:cancel"
	case modePickFile:
		return "type to filter  j/k:navigate  enter:select  esc:cancel"
	case modeInputLabel:
		return "enter:confirm  esc:cancel"
	default:
		return "j/k:navigate  enter:open  p:pin note  ?:help  q:quit"
	}
}

// IsTyping returns true when a text input is active (suppresses global key handling).
func (m NotesModel) IsTyping() bool {
	return m.mode == modePickFile || m.mode == modeInputLabel
}

func (m NotesModel) Update(msg tea.Msg) (NotesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case editorFinishedMsg:
		// Nothing to reload for pinned notes after editing
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			return m.updateList(msg)
		case modeSelectWorkspace:
			return m.updateSelectWorkspace(msg)
		case modePickFile:
			return m.updatePickFile(msg)
		case modeInputLabel:
			return m.updateInputLabel(msg)
		}
	}
	return m, nil
}

func (m NotesModel) updateList(msg tea.KeyMsg) (NotesModel, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		return m, messages.SwitchView(messages.ViewAgendaDay)
	case "j", "down":
		if m.cursor < len(m.flat)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		if m.cursor < len(m.flat) {
			return m, openFile(m.flat[m.cursor].note.AbsPath)
		}
	case "p":
		return m.startPin()
	}
	return m, nil
}

func (m NotesModel) startPin() (NotesModel, tea.Cmd) {
	if len(m.workspaces) == 0 {
		return m, nil
	}
	if len(m.workspaces) == 1 {
		return m.startPickFile(m.workspaces[0].RootDir)
	}
	m.mode = modeSelectWorkspace
	m.selectedWSIdx = 0
	return m, nil
}

func (m NotesModel) updateSelectWorkspace(msg tea.KeyMsg) (NotesModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
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
			return m.startPickFile(m.workspaces[m.selectedWSIdx].RootDir)
		}
	}
	return m, nil
}

func (m NotesModel) startPickFile(wsRoot string) (NotesModel, tea.Cmd) {
	files, _ := notespkg.ListWorkspaceFiles(wsRoot)
	m.pinWsRoot = wsRoot
	m.allFiles = files
	m.filtered = files
	m.fileQuery = ""
	m.fileCursor = 0
	m.fileInput.SetValue("")
	m.fileInput.Focus()
	m.mode = modePickFile
	return m, textinput.Blink
}

func (m NotesModel) updatePickFile(msg tea.KeyMsg) (NotesModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.fileInput.Blur()
		return m, nil
	case "enter":
		if m.fileCursor < len(m.filtered) {
			m.selectedRelPath = m.filtered[m.fileCursor]
			m.labelInput.SetValue(labelFromPath(m.selectedRelPath))
			m.labelInput.Focus()
			m.fileInput.Blur()
			m.mode = modeInputLabel
			return m, textinput.Blink
		}
		return m, nil
	case "j", "down":
		if m.fileCursor < len(m.filtered)-1 {
			m.fileCursor++
		}
		return m, nil
	case "k", "up":
		if m.fileCursor > 0 {
			m.fileCursor--
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.fileInput, cmd = m.fileInput.Update(msg)
	newQuery := m.fileInput.Value()
	if newQuery != m.fileQuery {
		m.fileQuery = newQuery
		m.applyFileFilter()
		m.fileCursor = 0
	}
	return m, cmd
}

func (m *NotesModel) applyFileFilter() {
	if m.fileQuery == "" {
		m.filtered = m.allFiles
		return
	}
	q := strings.ToLower(m.fileQuery)
	var result []string
	for _, f := range m.allFiles {
		if strings.Contains(strings.ToLower(f), q) {
			result = append(result, f)
		}
	}
	m.filtered = result
}

func (m NotesModel) updateInputLabel(msg tea.KeyMsg) (NotesModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modePickFile
		m.labelInput.Blur()
		m.fileInput.Focus()
		return m, textinput.Blink
	case "enter":
		label := strings.TrimSpace(m.labelInput.Value())
		if label == "" {
			label = labelFromPath(m.selectedRelPath)
		}
		if err := notespkg.AddPinnedNote(m.pinWsRoot, label, m.selectedRelPath); err == nil {
			m.labelInput.Blur()
			m.mode = modeList
			m.SetData(m.workspaces)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.labelInput, cmd = m.labelInput.Update(msg)
	return m, cmd
}

func (m NotesModel) View() string {
	switch m.mode {
	case modeSelectWorkspace:
		return m.viewSelectWorkspace()
	case modePickFile:
		return m.viewPickFile()
	case modeInputLabel:
		return m.viewInputLabel()
	default:
		return m.viewList()
	}
}

func (m NotesModel) viewList() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Notes"))
	lines = append(lines, "")

	if len(m.flat) == 0 {
		lines = append(lines, listItemStyle.Render("No pinned notes. Press 'p' to pin one."))
		lines = append(lines, "")
		content := lipgloss.JoinVertical(lipgloss.Left, lines...)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	multiWs := len(m.workspaces) > 1
	flatIdx := 0

	maxVisible := m.height - 8
	if maxVisible < 3 {
		maxVisible = 3
	}
	startIdx := 0
	if m.cursor >= maxVisible {
		startIdx = m.cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(m.flat) {
		endIdx = len(m.flat)
	}

	// Build section headers: track which wsRoot we've already shown a header for.
	// We iterate the full flat list to compute section start positions.
	sectionStart := make(map[int]string) // flat index -> ws name header
	for _, e := range m.entries {
		if len(e.notes) == 0 {
			continue
		}
		sectionStart[flatIdx] = abbreviatePath(e.ws.RootDir)
		flatIdx += len(e.notes)
	}

	if startIdx > 0 {
		lines = append(lines, pathStyle.Render(fmt.Sprintf("  ▲ %d more above", startIdx)))
	}

	for i := startIdx; i < endIdx; i++ {
		if multiWs {
			if header, ok := sectionStart[i]; ok {
				lines = append(lines, sectionHeaderStyle.Render("  "+header))
			}
		}
		entry := m.flat[i]
		style := listItemStyle
		prefix := "  "
		if i == m.cursor {
			style = selectedListItemStyle
			prefix = "► "
		}
		label := entry.note.Label
		path := pathStyle.Render("  " + entry.note.RelPath)
		lines = append(lines, style.Render(prefix+label)+path)
	}

	if endIdx < len(m.flat) {
		lines = append(lines, pathStyle.Render(fmt.Sprintf("  ▼ %d more below", len(m.flat)-endIdx)))
	}

	lines = append(lines, "")
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m NotesModel) viewSelectWorkspace() string {
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

func (m NotesModel) viewPickFile() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Pin a Note"))
	lines = append(lines, pathStyle.Render("  "+abbreviatePath(m.pinWsRoot)))
	lines = append(lines, "")
	lines = append(lines, "  "+m.fileInput.View())
	lines = append(lines, "")

	maxVisible := 12
	startIdx := 0
	if m.fileCursor >= maxVisible {
		startIdx = m.fileCursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}

	if startIdx > 0 {
		lines = append(lines, pathStyle.Render(fmt.Sprintf("  ▲ %d more above", startIdx)))
	}
	for i := startIdx; i < endIdx; i++ {
		style := listItemStyle
		prefix := "  "
		if i == m.fileCursor {
			style = selectedListItemStyle
			prefix = "► "
		}
		lines = append(lines, style.Render(prefix+m.filtered[i]))
	}
	if endIdx < len(m.filtered) {
		lines = append(lines, pathStyle.Render(fmt.Sprintf("  ▼ %d more below", len(m.filtered)-endIdx)))
	}
	if len(m.filtered) == 0 {
		lines = append(lines, listItemStyle.Render("  No matching files."))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m NotesModel) viewInputLabel() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Label for Pinned Note"))
	lines = append(lines, "")
	lines = append(lines, pathStyle.Render("  "+m.selectedRelPath))
	lines = append(lines, "")
	lines = append(lines, "  "+m.labelInput.View())
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// openFile opens the given path in $EDITOR (fallback: vim).
func openFile(absPath string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	c := exec.Command(editor, absPath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

// labelFromPath derives a display label from a relative file path.
func labelFromPath(relPath string) string {
	base := filepath.Base(relPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	return name
}

// abbreviatePath replaces the home directory with ~.
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
