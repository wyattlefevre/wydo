package kanban

import (
	"wydo/internal/kanban/models"
	"wydo/internal/tui/messages"
	"wydo/internal/tui/shared"
	"wydo/internal/tui/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const sidebarWidth = 16

// CombinedModel wraps PickerModel (sidebar) and BoardModel (content area) side by side.
type CombinedModel struct {
	picker         PickerModel
	board          BoardModel
	boardLoaded    bool
	sidebarFocused bool
	width          int
	height         int
}

func NewCombinedModel(boards []models.Board, defaultDir string, availableDirs []string) CombinedModel {
	return CombinedModel{
		picker:         NewPickerModel(boards, defaultDir, availableDirs),
		sidebarFocused: true,
	}
}

func (m *CombinedModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.picker.SetSize(sidebarWidth, height)
	if m.boardLoaded {
		m.board.SetSize(m.boardContentWidth(), height)
	}
}

func (m CombinedModel) boardContentWidth() int {
	w := m.width - sidebarWidth - 1 // 1 for separator column
	if w < 0 {
		return 0
	}
	return w
}

func (m *CombinedModel) SetBoards(boards []models.Board) {
	m.picker.SetBoards(boards)
}

// LoadBoard loads a board and shifts focus to the board content area.
func (m *CombinedModel) LoadBoard(board models.Board, allProjects []ProjectPickerItem, allBoards []models.Board, boardProjects []string) {
	m.board = NewBoardModel(board, allProjects, allBoards, boardProjects)
	m.board.SetSize(m.boardContentWidth(), m.height)
	m.boardLoaded = true
	m.sidebarFocused = false
}

// NavigateTo positions the board cursor at a specific column and card index.
func (m *CombinedModel) NavigateTo(colIndex, cardIndex int) {
	if m.boardLoaded {
		m.board.NavigateTo(colIndex, cardIndex)
	}
}

func (m *CombinedModel) SetBoard(board models.Board) {
	if m.boardLoaded {
		m.board.SetBoard(board)
	}
}

func (m *CombinedModel) SetAllProjects(projects []ProjectPickerItem) {
	if m.boardLoaded {
		m.board.SetAllProjects(projects)
	}
}

func (m *CombinedModel) SetBoardProjects(projects []string) {
	if m.boardLoaded {
		m.board.SetBoardProjects(projects)
	}
}

// UnloadBoard clears the currently loaded board and returns focus to the sidebar.
func (m *CombinedModel) UnloadBoard() {
	m.boardLoaded = false
	m.sidebarFocused = true
}

// FocusBoard shifts focus to the board content area (e.g. when returning via B key).
func (m *CombinedModel) FocusBoard() {
	if m.boardLoaded {
		m.sidebarFocused = false
	}
}

// BoardPath returns the path of the currently loaded board, or "" if none.
func (m CombinedModel) BoardPath() string {
	if m.boardLoaded {
		return m.board.BoardPath()
	}
	return ""
}

// IsTyping returns true when the sidebar is focused and has an active text input.
func (m CombinedModel) IsTyping() bool {
	if m.sidebarFocused {
		return m.picker.IsTyping()
	}
	return false
}

// IsModal returns true when the board is focused and has an active modal.
func (m CombinedModel) IsModal() bool {
	if !m.sidebarFocused && m.boardLoaded {
		return m.board.IsModal()
	}
	return false
}

func (m CombinedModel) Init() tea.Cmd {
	if m.boardLoaded {
		return m.board.Init()
	}
	return nil
}

func (m CombinedModel) Update(msg tea.Msg) (CombinedModel, tea.Cmd) {
	switch msg.(type) {
	case messages.FocusSidebarMsg:
		m.sidebarFocused = true
		return m, nil
	}

	if m.sidebarFocused {
		// Intercept esc in list mode when board is loaded → focus board instead of going to agenda
		if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
			if m.picker.ShouldFocusBoard() && m.boardLoaded {
				m.sidebarFocused = false
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd
	}

	if m.boardLoaded {
		var cmd tea.Cmd
		m.board, cmd = m.board.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m CombinedModel) View() string {
	// Show creation/confirmation UI as a centered popup instead of in the sidebar
	if m.picker.mode == modeCreate || m.picker.mode == modeSelectDir || m.picker.mode == modeArchiveConfirm || m.picker.mode == modeDeleteConfirm {
		p := m.picker
		p.width = m.width
		p.height = m.height
		return p.View()
	}

	sidebar := m.picker.viewSidebar(m.height, m.BoardPath(), m.sidebarFocused)
	sep := shared.RenderSeparatorColumn(m.height)

	var boardContent string
	if m.boardLoaded {
		boardContent = m.board.View()
	} else {
		boardContent = lipgloss.NewStyle().
			Width(m.boardContentWidth()).
			Height(m.height).
			Foreground(theme.TextMuted).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Select a board from the sidebar")
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, sep, boardContent)
}
