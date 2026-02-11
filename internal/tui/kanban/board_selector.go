package kanban

import (
	"os"
	"path/filepath"
	"strings"
	"wydo/internal/kanban/models"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// BoardSelectorModel is a simple single-select list for picking a board.
type BoardSelectorModel struct {
	boards []models.Board
	cursor int
	width  int
	height int
}

// NewBoardSelectorModel creates a new board selector, excluding the current board.
func NewBoardSelectorModel(allBoards []models.Board, currentBoardPath string) BoardSelectorModel {
	var filtered []models.Board
	for _, b := range allBoards {
		if b.Path != currentBoardPath {
			filtered = append(filtered, b)
		}
	}
	return BoardSelectorModel{boards: filtered}
}

// Empty returns true when there are no boards to choose from.
func (m BoardSelectorModel) Empty() bool {
	return len(m.boards) == 0
}

// Update handles key events. Returns (model, selectedPath, done).
// selectedPath is non-empty only on enter; done is true on enter or esc.
func (m BoardSelectorModel) Update(msg tea.KeyMsg) (BoardSelectorModel, string, bool) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.boards)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		if len(m.boards) > 0 && m.cursor < len(m.boards) {
			return m, m.boards[m.cursor].Path, true
		}
		return m, "", true
	case "esc", "q":
		return m, "", true
	}
	return m, "", false
}

// View renders the board selector as a centered modal.
func (m BoardSelectorModel) View() string {
	var lines []string

	lines = append(lines, tagPickerTitleStyle.Render("Move to Board"))
	lines = append(lines, "")

	for i, b := range m.boards {
		style := listItemStyle
		prefix := "  "
		if i == m.cursor {
			style = selectedListItemStyle
			prefix = "► "
		}
		parentDir := filepath.Dir(b.Path)
		displayPath := abbreviateBoardPath(parentDir)
		line := style.Render(prefix+b.Name) + "  " + pathStyle.Render(displayPath)
		lines = append(lines, line)
	}

	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("j/k: navigate • enter: select • esc: cancel"))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	boxed := tagPickerBoxStyle.Width(60).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, boxed)
}

func abbreviateBoardPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
