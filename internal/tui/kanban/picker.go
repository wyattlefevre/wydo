package kanban

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"wydo/internal/kanban/models"
	"wydo/internal/kanban/operations"
	"wydo/internal/tui/messages"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

type pickerMode int

const (
	modeList pickerMode = iota
	modeSearch
	modeSelectDir
	modeCreate
)

type PickerModel struct {
	boards         []models.Board
	filtered       []int // indices into boards
	selected       int
	mode           pickerMode
	textInput      textinput.Model
	searchQuery    string
	defaultDir     string
	availableDirs  []string
	selectedDirIdx int
	width          int
	height         int
	err            error
}

func NewPickerModel(boards []models.Board, defaultDir string, availableDirs []string) PickerModel {
	ti := textinput.New()
	ti.Placeholder = "Enter board name..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 40

	filtered := make([]int, len(boards))
	for i := range boards {
		filtered[i] = i
	}

	return PickerModel{
		boards:        boards,
		filtered:      filtered,
		selected:      0,
		mode:          modeList,
		textInput:     ti,
		defaultDir:    defaultDir,
		availableDirs: availableDirs,
	}
}

// SetSize updates the view dimensions
func (m *PickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// IsTyping returns true when the picker is in create or search mode with active text input
func (m PickerModel) IsTyping() bool {
	return m.mode == modeCreate || m.mode == modeSearch
}

// HintText returns the raw hint string for the current picker mode.
func (m PickerModel) HintText() string {
	switch m.mode {
	case modeSearch:
		return "type to filter  enter:confirm  esc:cancel"
	case modeSelectDir:
		return "j/k:navigate  enter:select  esc:cancel"
	case modeCreate:
		return "enter:create  esc:cancel"
	default:
		return "j/k:navigate  /:search  enter:select  n:new board  ?:help  q:quit"
	}
}

// SetBoards updates the boards list
func (m *PickerModel) SetBoards(boards []models.Board) {
	m.boards = boards
	m.applyFilter()
}

func (m *PickerModel) applyFilter() {
	if m.searchQuery == "" {
		m.filtered = make([]int, len(m.boards))
		for i := range m.boards {
			m.filtered[i] = i
		}
	} else {
		names := make([]string, len(m.boards))
		for i, b := range m.boards {
			names[i] = b.Name
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

func (m PickerModel) Init() tea.Cmd {
	return nil
}

// Update handles picker events, returns (PickerModel, tea.Cmd) as a child view
func (m PickerModel) Update(msg tea.Msg) (PickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			return m.updateList(msg)
		case modeSearch:
			return m.updateSearch(msg)
		case modeSelectDir:
			return m.updateSelectDir(msg)
		case modeCreate:
			return m.updateCreate(msg)
		}
	}

	return m, nil
}

func (m PickerModel) updateList(msg tea.KeyMsg) (PickerModel, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit

	case "esc":
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.applyFilter()
			return m, nil
		}
		return m, func() tea.Msg {
			return messages.SwitchViewMsg{View: messages.ViewAgendaDay}
		}

	case "/":
		m.mode = modeSearch
		m.textInput.Placeholder = "Search boards..."
		m.textInput.SetValue(m.searchQuery)
		m.textInput.Focus()
		return m, textinput.Blink

	case "j", "down":
		if len(m.filtered) > 0 && m.selected < len(m.filtered)-1 {
			m.selected++
		}

	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}

	case "n":
		// If multiple directories, let user choose first
		if len(m.availableDirs) > 1 {
			m.mode = modeSelectDir
			m.selectedDirIdx = 0
			return m, nil
		}
		// Otherwise go straight to create mode
		m.mode = modeCreate
		m.textInput.Placeholder = "Enter board name..."
		m.textInput.Focus()
		return m, textinput.Blink

	case "enter":
		if len(m.filtered) == 0 {
			// If no boards, enter means create new
			m.mode = modeCreate
			m.textInput.Placeholder = "Enter board name..."
			m.textInput.Focus()
			return m, textinput.Blink
		} else if m.selected < len(m.filtered) {
			// Select board - send OpenBoardMsg
			board := m.boards[m.filtered[m.selected]]
			return m, func() tea.Msg {
				return messages.OpenBoardMsg{
					BoardPath: board.Path,
				}
			}
		}
	}

	return m, nil
}

func (m PickerModel) updateSearch(msg tea.KeyMsg) (PickerModel, tea.Cmd) {
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

func (m PickerModel) updateCreate(msg tea.KeyMsg) (PickerModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.textInput.SetValue("")
		return m, nil

	case "enter":
		boardName := strings.TrimSpace(m.textInput.Value())
		if boardName != "" {
			if m.defaultDir == "" {
				m.err = fmt.Errorf("no directory configured for creating boards")
				return m, nil
			}
			board, err := operations.CreateBoard(m.defaultDir, boardName)
			if err != nil {
				m.err = err
				return m, nil
			}

			// Open the newly created board
			return m, func() tea.Msg {
				return messages.OpenBoardMsg{
					BoardPath: board.Path,
				}
			}
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m PickerModel) updateSelectDir(msg tea.KeyMsg) (PickerModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil

	case "j", "down":
		if m.selectedDirIdx < len(m.availableDirs)-1 {
			m.selectedDirIdx++
		}

	case "k", "up":
		if m.selectedDirIdx > 0 {
			m.selectedDirIdx--
		}

	case "enter":
		if len(m.availableDirs) > 0 && m.selectedDirIdx < len(m.availableDirs) {
			m.defaultDir = m.availableDirs[m.selectedDirIdx]
			m.mode = modeCreate
			m.textInput.Focus()
			return m, textinput.Blink
		}
	}

	return m, nil
}

func (m PickerModel) View() string {
	switch m.mode {
	case modeSearch:
		return m.viewSearch()
	case modeSelectDir:
		return m.viewSelectDir()
	case modeCreate:
		return m.viewCreate()
	default:
		return m.viewList()
	}
}

func (m PickerModel) viewSearch() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Search Boards"))
	lines = append(lines, "")
	lines = append(lines, "  "+m.textInput.View())
	lines = append(lines, "")

	// Show live results
	if len(m.filtered) > 0 {
		show := min(8, len(m.filtered))
		for i := 0; i < show; i++ {
			board := m.boards[m.filtered[i]]
			prefix := "  "
			if i == m.selected {
				prefix = "► "
			}
			lines = append(lines, listItemStyle.Render(prefix+board.Name))
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

func (m PickerModel) viewList() string {
	var lines []string

	// Title
	lines = append(lines, titleStyle.Render("Board Picker"))
	lines = append(lines, "")

	if m.searchQuery != "" {
		lines = append(lines, filterIndicatorStyle.Render("  Filter: ")+pathStyle.Render(m.searchQuery))
		lines = append(lines, "")
	}

	if len(m.filtered) == 0 {
		if len(m.boards) == 0 {
			lines = append(lines, listItemStyle.Render("No boards found. Press 'n' to create one."))
		} else {
			lines = append(lines, listItemStyle.Render("No matching boards."))
		}
		lines = append(lines, "")
	} else {
		// Calculate max board name width for alignment
		maxNameWidth := 0
		for _, idx := range m.filtered {
			board := m.boards[idx]
			width := lipgloss.Width(listItemStyle.Render("► " + board.Name))
			if width > maxNameWidth {
				maxNameWidth = width
			}
		}

		// List filtered boards
		for i, idx := range m.filtered {
			board := m.boards[idx]
			style := listItemStyle
			prefix := "  "
			if i == m.selected {
				style = selectedListItemStyle
				prefix = "► "
			}
			nameCol := style.Width(maxNameWidth).Render(prefix + board.Name)
			parentDir := filepath.Dir(board.Path)
			displayPath := abbreviatePath(parentDir)
			line := nameCol + "  " + pathStyle.Render(displayPath)
			lines = append(lines, line)
		}
		lines = append(lines, "")
	}

	// Error message
	if m.err != nil {
		lines = append(lines, errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		lines = append(lines, "")
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m PickerModel) viewSelectDir() string {
	var lines []string

	lines = append(lines, titleStyle.Render("Select Directory"))
	lines = append(lines, "")

	for i, dir := range m.availableDirs {
		style := listItemStyle
		prefix := "  "
		if i == m.selectedDirIdx {
			style = selectedListItemStyle
			prefix = "► "
		}
		displayPath := abbreviatePath(dir)
		lines = append(lines, style.Render(prefix+displayPath))
	}
	lines = append(lines, "")

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m PickerModel) viewCreate() string {
	var lines []string

	lines = append(lines, titleStyle.Render("Create New Board"))
	lines = append(lines, "")
	lines = append(lines, m.textInput.View())
	lines = append(lines, "")

	if m.err != nil {
		lines = append(lines, errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		lines = append(lines, "")
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
