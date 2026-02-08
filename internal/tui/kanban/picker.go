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
)

type pickerMode int

const (
	modeList pickerMode = iota
	modeSelectDir
	modeCreate
)

type PickerModel struct {
	boards         []models.Board
	selected       int
	mode           pickerMode
	textInput      textinput.Model
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

	return PickerModel{
		boards:        boards,
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

// SetBoards updates the boards list
func (m *PickerModel) SetBoards(boards []models.Board) {
	m.boards = boards
	if m.selected >= len(m.boards) {
		m.selected = max(0, len(m.boards)-1)
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
	case "j", "down":
		if len(m.boards) > 0 && m.selected < len(m.boards)-1 {
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
		m.textInput.Focus()
		return m, textinput.Blink

	case "enter":
		if len(m.boards) == 0 {
			// If no boards, enter means create new
			m.mode = modeCreate
			m.textInput.Focus()
			return m, textinput.Blink
		} else if m.selected < len(m.boards) {
			// Select board - send OpenBoardMsg
			board := m.boards[m.selected]
			return m, func() tea.Msg {
				return messages.OpenBoardMsg{
					BoardPath: board.Path,
				}
			}
		}
	}

	return m, nil
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
	case modeSelectDir:
		return m.viewSelectDir()
	case modeCreate:
		return m.viewCreate()
	default:
		return m.viewList()
	}
}

func (m PickerModel) viewList() string {
	var lines []string

	// Title
	lines = append(lines, titleStyle.Render("Board Picker"))
	lines = append(lines, "")

	if len(m.boards) == 0 {
		lines = append(lines, listItemStyle.Render("No boards found. Press 'n' to create one."))
		lines = append(lines, "")
	} else {
		// Calculate max board name width for alignment
		maxNameWidth := 0
		for _, board := range m.boards {
			// Account for prefix
			width := lipgloss.Width(listItemStyle.Render("► " + board.Name))
			if width > maxNameWidth {
				maxNameWidth = width
			}
		}

		// List boards
		for i, board := range m.boards {
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

	// Help
	lines = append(lines, helpStyle.Render("j/k: navigate • enter: select • n: new board"))

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

	lines = append(lines, helpStyle.Render("j/k: navigate • enter: select • esc: cancel"))

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

	lines = append(lines, helpStyle.Render("enter: create • esc: cancel"))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
