package kanban

import (
	"fmt"
	"strings"
	"wydo/internal/kanban/models"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type columnEditorMode int

const (
	columnEditorModeNormal columnEditorMode = iota
	columnEditorModeRename
	columnEditorModeAdd
	columnEditorModeConfirmDelete
)

// ColumnEditorModel handles column editing
type ColumnEditorModel struct {
	board         *models.Board
	columns       []models.Column // Working copy for editing
	cursorPos     int
	mode          columnEditorMode
	textInput     textinput.Model
	message       string
	err           error
	deleteConfirm bool // Track if in delete confirmation
	insertBefore  bool // Track if inserting before vs after cursor
}

// NewColumnEditorModel creates a new column editor
func NewColumnEditorModel(board *models.Board) ColumnEditorModel {
	ti := textinput.New()
	ti.CharLimit = 50
	ti.Width = 50

	// Create working copy of columns
	columns := make([]models.Column, len(board.Columns))
	copy(columns, board.Columns)

	return ColumnEditorModel{
		board:     board,
		columns:   columns,
		cursorPos: 0,
		mode:      columnEditorModeNormal,
		textInput: ti,
	}
}

// Init initializes the column editor
func (m ColumnEditorModel) Init() tea.Cmd {
	return nil
}

// Update handles column editor events
// Returns (model, cmd, isDone)
func (m ColumnEditorModel) Update(msg tea.KeyMsg) (ColumnEditorModel, tea.Cmd, bool) {
	m.message = ""
	m.err = nil

	switch m.mode {
	case columnEditorModeNormal:
		return m.updateNormal(msg)
	case columnEditorModeRename:
		return m.updateRename(msg)
	case columnEditorModeAdd:
		return m.updateAdd(msg)
	case columnEditorModeConfirmDelete:
		return m.updateConfirmDelete(msg)
	}

	return m, nil, false
}

func (m ColumnEditorModel) updateNormal(msg tea.KeyMsg) (ColumnEditorModel, tea.Cmd, bool) {
	switch msg.String() {
	case "enter":
		// Save changes and exit
		return m, nil, true

	case "esc", "q":
		// Cancel without saving
		return m, nil, true

	case "j", "down":
		if m.cursorPos < len(m.columns)-1 {
			m.cursorPos++
		}

	case "k", "up":
		if m.cursorPos > 0 {
			m.cursorPos--
		}

	case "r":
		// Rename column
		if m.cursorPos < len(m.columns) {
			if !m.board.IsDoneColumn(m.columns[m.cursorPos].Name) {
				m.textInput.SetValue(m.columns[m.cursorPos].Name)
				m.textInput.Focus()
				m.mode = columnEditorModeRename
				return m, textinput.Blink, false
			} else {
				m.err = fmt.Errorf("cannot rename Done column")
			}
		}

	case "o":
		// Add new column after current position (below)
		m.textInput.SetValue("")
		m.textInput.Focus()
		m.insertBefore = false
		m.mode = columnEditorModeAdd
		return m, textinput.Blink, false

	case "O":
		// Add new column before current position (above)
		m.textInput.SetValue("")
		m.textInput.Focus()
		m.insertBefore = true
		m.mode = columnEditorModeAdd
		return m, textinput.Blink, false

	case "d":
		// Delete column
		if m.cursorPos < len(m.columns) {
			if canDelete, msg := m.board.CanDeleteColumn(m.cursorPos); !canDelete {
				m.err = fmt.Errorf("%s", msg)
			} else {
				m.mode = columnEditorModeConfirmDelete
			}
		}

	case "J":
		// Move column down (shift+j)
		if m.cursorPos < len(m.columns)-1 {
			// Check if can move
			if m.board.IsDoneColumn(m.columns[m.cursorPos].Name) {
				m.err = fmt.Errorf("cannot move Done column")
			} else {
				// Check if moving past Done
				lastIndex := len(m.columns) - 1
				if m.cursorPos+1 == lastIndex && m.board.IsDoneColumn(m.columns[lastIndex].Name) {
					m.err = fmt.Errorf("cannot move column past Done")
				} else {
					// Swap with next column
					m.columns[m.cursorPos], m.columns[m.cursorPos+1] = m.columns[m.cursorPos+1], m.columns[m.cursorPos]
					m.cursorPos++
				}
			}
		}

	case "K":
		// Move column up (shift+k)
		if m.cursorPos > 0 {
			// Check if can move
			if m.board.IsDoneColumn(m.columns[m.cursorPos].Name) {
				m.err = fmt.Errorf("cannot move Done column")
			} else {
				// Swap with previous column
				m.columns[m.cursorPos], m.columns[m.cursorPos-1] = m.columns[m.cursorPos-1], m.columns[m.cursorPos]
				m.cursorPos--
			}
		}
	}

	return m, nil, false
}

func (m ColumnEditorModel) updateRename(msg tea.KeyMsg) (ColumnEditorModel, tea.Cmd, bool) {
	switch msg.String() {
	case "enter":
		// Confirm rename
		newName := strings.TrimSpace(m.textInput.Value())
		if newName == "" {
			m.err = fmt.Errorf("column name cannot be empty")
			m.textInput.Blur()
			m.mode = columnEditorModeNormal
			return m, nil, false
		}

		// Check for duplicates
		for i, col := range m.columns {
			if i != m.cursorPos && strings.EqualFold(col.Name, newName) {
				m.err = fmt.Errorf("column name already exists")
				m.textInput.Blur()
				m.mode = columnEditorModeNormal
				return m, nil, false
			}
		}

		// Apply rename
		m.columns[m.cursorPos].Name = newName
		m.message = "Column renamed"
		m.textInput.Blur()
		m.mode = columnEditorModeNormal

	case "esc":
		// Cancel rename
		m.textInput.Blur()
		m.mode = columnEditorModeNormal
		return m, nil, false

	default:
		// Handle text input
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd, false
	}

	return m, nil, false
}

func (m ColumnEditorModel) updateAdd(msg tea.KeyMsg) (ColumnEditorModel, tea.Cmd, bool) {
	switch msg.String() {
	case "enter":
		// Confirm add
		newName := strings.TrimSpace(m.textInput.Value())
		if newName == "" {
			m.err = fmt.Errorf("column name cannot be empty")
			m.textInput.Blur()
			m.mode = columnEditorModeNormal
			return m, nil, false
		}

		// Check for duplicates
		for _, col := range m.columns {
			if strings.EqualFold(col.Name, newName) {
				m.err = fmt.Errorf("column name already exists")
				m.textInput.Blur()
				m.mode = columnEditorModeNormal
				return m, nil, false
			}
		}

		// Determine insertion position based on insertBefore flag
		var insertPos int
		if m.insertBefore {
			// Insert before current position
			insertPos = m.cursorPos
		} else {
			// Insert after current position
			insertPos = m.cursorPos + 1
		}

		// If inserting after Done, insert before it instead
		if insertPos >= len(m.columns) || (insertPos < len(m.columns) && m.board.IsDoneColumn(m.columns[insertPos].Name)) {
			insertPos = len(m.columns) - 1
			if insertPos < 0 {
				insertPos = 0
			}
		}

		// Create new column
		newColumn := models.Column{
			Name:  newName,
			Cards: []models.Card{},
		}

		// Insert at position
		m.columns = append(m.columns[:insertPos], append([]models.Column{newColumn}, m.columns[insertPos:]...)...)
		m.cursorPos = insertPos
		m.message = "Column added"
		m.textInput.Blur()
		m.mode = columnEditorModeNormal

	case "esc":
		// Cancel add
		m.textInput.Blur()
		m.mode = columnEditorModeNormal
		return m, nil, false

	default:
		// Handle text input
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd, false
	}

	return m, nil, false
}

func (m ColumnEditorModel) updateConfirmDelete(msg tea.KeyMsg) (ColumnEditorModel, tea.Cmd, bool) {
	switch msg.String() {
	case "y":
		// Confirm delete
		if m.cursorPos < len(m.columns) {
			column := m.columns[m.cursorPos]

			// Migrate cards to adjacent column if any exist
			if len(column.Cards) > 0 {
				targetIndex := m.cursorPos - 1 // Try left
				if targetIndex < 0 {
					targetIndex = m.cursorPos + 1 // Use right if leftmost
				}

				if targetIndex >= 0 && targetIndex < len(m.columns) {
					// Move all cards to target column
					m.columns[targetIndex].Cards = append(
						m.columns[targetIndex].Cards,
						column.Cards...,
					)
				}
			}

			// Remove column
			m.columns = append(m.columns[:m.cursorPos], m.columns[m.cursorPos+1:]...)

			// Adjust cursor if needed
			if m.cursorPos >= len(m.columns) && m.cursorPos > 0 {
				m.cursorPos--
			}

			m.message = "Column deleted"
		}
		m.mode = columnEditorModeNormal

	case "n", "esc":
		// Cancel delete
		m.mode = columnEditorModeNormal
	}

	return m, nil, false
}

// View renders the column editor
func (m ColumnEditorModel) View() string {
	var s strings.Builder

	// Title
	title := columnEditorTitleStyle.Render("Column Editor")
	s.WriteString(title)
	s.WriteString("\n\n")

	// Show text input if in rename/add mode
	if m.mode == columnEditorModeRename {
		s.WriteString(columnEditorPromptStyle.Render("Rename: "))
		s.WriteString(m.textInput.View())
		s.WriteString("\n\n")
	} else if m.mode == columnEditorModeAdd {
		s.WriteString(columnEditorPromptStyle.Render("New column: "))
		s.WriteString(m.textInput.View())
		s.WriteString("\n\n")
	}

	// Column list
	for i, col := range m.columns {
		cardCount := len(col.Cards)
		isDone := m.board.IsDoneColumn(col.Name)

		line := fmt.Sprintf("%d. %s (%d cards)", i+1, col.Name, cardCount)

		if isDone {
			line += " [immutable]"
		}

		// Apply style
		style := columnEditorItemStyle
		if i == m.cursorPos {
			style = columnEditorItemHighlightStyle
		} else if isDone {
			style = columnEditorItemImmutableStyle
		}

		s.WriteString(style.Render(line))
		s.WriteString("\n")
	}

	s.WriteString("\n")

	// Error or message
	if m.err != nil {
		s.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		s.WriteString("\n")
	} else if m.message != "" {
		s.WriteString(successStyle.Render(m.message))
		s.WriteString("\n")
	}

	// Help text based on mode
	var help string
	switch m.mode {
	case columnEditorModeRename, columnEditorModeAdd:
		help = helpStyle.Render("enter: confirm • esc: cancel")
	case columnEditorModeConfirmDelete:
		if m.cursorPos < len(m.columns) {
			cardCount := len(m.columns[m.cursorPos].Cards)
			if cardCount > 0 {
				help = warningStyle.Render(fmt.Sprintf("Delete column and move %d cards to adjacent column? (y/n)", cardCount))
			} else {
				help = warningStyle.Render("Delete this column? (y/n)")
			}
		}
	default:
		help = helpStyle.Render("jk: navigate • r: rename • o: add below • O: add above • d: delete • JK: reorder • enter: save • esc: cancel")
	}
	s.WriteString(help)

	// Wrap in box
	content := s.String()
	return columnEditorBoxStyle.Render(content)
}
