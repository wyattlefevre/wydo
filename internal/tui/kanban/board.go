package kanban

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"wydo/internal/kanban/fs"
	"wydo/internal/kanban/models"
	"wydo/internal/kanban/operations"
	"wydo/internal/tui/messages"
	"wydo/internal/tui/shared"
	"wydo/internal/tui/theme"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

type boardMode int

const (
	boardModeNormal boardMode = iota
	boardModeMove
	boardModeConfirmDelete
	boardModeTagEdit
	boardModeProjectEdit
	boardModeColumnEdit
	boardModeURLPicker
	boardModeURLEditor
	boardModeDueDateEdit
	boardModeScheduledDateEdit
	boardModePriorityInput
	boardModeFilter
	boardModeBoardMove
	boardModeTmuxPicker
	boardModeTmuxLaunch
)

func (m boardMode) String() string {
	switch m {
	case boardModeNormal:
		return "NORMAL"
	case boardModeMove:
		return "MOVE"
	case boardModeConfirmDelete:
		return "DELETE"
	case boardModeTagEdit:
		return "TAG"
	case boardModeProjectEdit:
		return "PROJECT"
	case boardModeColumnEdit:
		return "COLUMN"
	case boardModeURLPicker:
		return "URL"
	case boardModeURLEditor:
		return "URL"
	case boardModeDueDateEdit:
		return "DUE DATE"
	case boardModeScheduledDateEdit:
		return "SCHEDULED"
	case boardModePriorityInput:
		return "PRIORITY"
	case boardModeFilter:
		return "FILTER"
	case boardModeBoardMove:
		return "BOARD"
	case boardModeTmuxPicker:
		return "TMUX"
	case boardModeTmuxLaunch:
		return "TMUX"
	default:
		return "NORMAL"
	}
}

func (m boardMode) modeColor() lipgloss.Color {
	switch m {
	case boardModeNormal:
		return theme.Primary
	case boardModeMove:
		return theme.Warning
	case boardModeFilter:
		return theme.Secondary
	case boardModeConfirmDelete:
		return theme.Danger
	case boardModeTmuxPicker, boardModeTmuxLaunch:
		return theme.Success
	default:
		return theme.Accent
	}
}

type BoardModel struct {
	board                  models.Board
	allProjects            []string
	selectedCol            int
	selectedCard           int
	mode                   boardMode
	width                  int
	height                 int
	err                    error
	message                string
	tagPicker              *TagPickerModel
	projectPicker          *ProjectPickerModel
	columnEditor           *ColumnEditorModel
	urlPicker              *URLPickerModel
	urlEditor              *URLEditorModel
	dueDatePicker          *shared.DatePickerModel
	scheduledDatePicker    *shared.DatePickerModel
	priorityInput          *PriorityInputModel
	columnScrollOffsets    []int // scroll position (card index) for each column
	columnCursorPos        []int // cursor position (card index) for each column
	columnHorizontalOffset int   // horizontal scroll offset (first visible column index)
	filterInput            textinput.Model
	filterQuery            string
	filterActive           bool
	filteredIndices        [][]int // per-column: original card indices that match
	allBoards              []models.Board
	boardSelector          *BoardSelectorModel
	tmuxPicker             *TmuxPickerModel
	tmuxLaunch             *TmuxLaunchModel
	boardProjects          []string
	showArchived           bool
}

func NewBoardModel(board models.Board, allProjects []string, allBoards []models.Board, boardProjects []string) BoardModel {
	return BoardModel{
		board:                  board,
		allProjects:            allProjects,
		allBoards:              allBoards,
		boardProjects:          boardProjects,
		selectedCol:            0,
		selectedCard:           0,
		mode:                   boardModeNormal,
		columnScrollOffsets:    make([]int, len(board.Columns)),
		columnCursorPos:        make([]int, len(board.Columns)),
		columnHorizontalOffset: 0,
	}
}

// SetSize updates the view dimensions
func (m *BoardModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.adjustScrollPosition()
	m.adjustHorizontalScrollPosition()
}

// SetBoard updates the board data
func (m *BoardModel) SetBoard(board models.Board) {
	m.board = board
	m.reloadBoardState()
}

// NavigateTo positions the cursor at a specific column and card
func (m *BoardModel) NavigateTo(colIndex, cardIndex int) {
	if colIndex >= 0 && colIndex < len(m.board.Columns) {
		m.selectedCol = colIndex
		if cardIndex >= 0 && cardIndex < len(m.board.Columns[colIndex].Cards) {
			m.selectedCard = cardIndex
			m.columnCursorPos[colIndex] = cardIndex
		}
		m.adjustScrollPosition()
		m.adjustHorizontalScrollPosition()
	}
}

// IsModal returns true if the board is in a modal mode (picking tags, editing, etc.)
func (m BoardModel) IsModal() bool {
	return m.mode != boardModeNormal
}

// HintText returns the raw hint string for the current board mode.
func (m BoardModel) HintText() string {
	switch m.mode {
	case boardModeMove:
		return "h/l:move card  j/k:reorder  enter:open  esc:cancel"
	case boardModeConfirmDelete:
		return "Delete this card? (y/n)"
	case boardModeFilter:
		return "type to filter  enter:lock filter  esc:cancel"
	default:
		if m.filterActive {
			return "?:help  /:edit filter  esc:clear filter"
		}
		return "?:help  /:filter  space/m:move  esc:back"
	}
}

// ModeText returns a styled mode badge like [MOVE] in the mode's color.
func (m BoardModel) ModeText() string {
	return modeIndicatorStyle(m.mode.modeColor()).Render("[" + m.mode.String() + "]")
}

func (m BoardModel) Init() tea.Cmd {
	return nil
}

// Update handles board events as a child view
func (m BoardModel) Update(msg tea.Msg) (BoardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case editorFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			// Sync filename before reloading (in case title changed)
			realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
			if err := operations.SyncCardFilename(&m.board, m.selectedCol, realIdx); err != nil {
				m.err = err
				return m, nil
			}

			// Reload board to refresh card content
			board, err := fs.ReadBoard(m.board.Path)
			if err != nil {
				m.err = err
			} else {
				m.board = board
				m.message = "Card updated"
				m.reloadBoardState()
				m.ensureCardBoardProjects(m.selectedCol, realIdx)
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case boardModeNormal:
			return m.updateNormal(msg)
		case boardModeMove:
			return m.updateMove(msg)
		case boardModeConfirmDelete:
			return m.updateConfirmDelete(msg)
		case boardModeTagEdit:
			return m.updateTagEdit(msg)
		case boardModeProjectEdit:
			return m.updateProjectEdit(msg)
		case boardModeColumnEdit:
			return m.updateColumnEdit(msg)
		case boardModeURLPicker:
			return m.updateURLPicker(msg)
		case boardModeURLEditor:
			return m.updateURLEditor(msg)
		case boardModeDueDateEdit:
			return m.updateDueDateEdit(msg)
		case boardModeScheduledDateEdit:
			return m.updateScheduledDateEdit(msg)
		case boardModePriorityInput:
			return m.updatePriorityInput(msg)
		case boardModeFilter:
			return m.updateFilter(msg)
		case boardModeBoardMove:
			return m.updateBoardMove(msg)
		case boardModeTmuxPicker:
			return m.updateTmuxPicker(msg)
		case boardModeTmuxLaunch:
			return m.updateTmuxLaunch(msg)
		}
	}

	return m, nil
}

func (m BoardModel) updateNormal(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	m.message = ""
	m.err = nil

	switch msg.String() {
	case "q", "b":
		// Return to picker via message
		return m, func() tea.Msg {
			return messages.SwitchViewMsg{View: messages.ViewKanbanPicker}
		}

	case "esc":
		if m.filterActive {
			m.filterQuery = ""
			m.filterActive = false
			m.filteredIndices = nil
			m.selectedCard = 0
			m.columnCursorPos[m.selectedCol] = 0
			m.adjustScrollPosition()
		} else {
			// Return to picker
			return m, func() tea.Msg {
				return messages.SwitchViewMsg{View: messages.ViewKanbanPicker}
			}
		}

	case "/":
		ti := textinput.New()
		ti.Placeholder = "filter..."
		ti.CharLimit = 100
		ti.Width = 40
		ti.SetValue(m.filterQuery)
		ti.Focus()
		m.filterInput = ti
		m.mode = boardModeFilter
		m.selectedCard = 0
		m.columnCursorPos[m.selectedCol] = 0
		return m, textinput.Blink

	case "h", "left":
		if m.selectedCol > 0 {
			m.selectedCol--
			// Restore saved cursor position
			m.selectedCard = m.columnCursorPos[m.selectedCol]
			// Validate bounds against visible cards
			visibleCount := len(m.getVisibleCards(m.selectedCol))
			if m.selectedCard >= visibleCount {
				m.selectedCard = max(0, visibleCount-1)
				m.columnCursorPos[m.selectedCol] = m.selectedCard
			}
			m.adjustScrollPosition()
			m.adjustHorizontalScrollPosition()
		}

	case "l", "right":
		if m.selectedCol < len(m.board.Columns)-1 {
			m.selectedCol++
			// Restore saved cursor position
			m.selectedCard = m.columnCursorPos[m.selectedCol]
			// Validate bounds against visible cards
			visibleCount := len(m.getVisibleCards(m.selectedCol))
			if m.selectedCard >= visibleCount {
				m.selectedCard = max(0, visibleCount-1)
				m.columnCursorPos[m.selectedCol] = m.selectedCard
			}
			m.adjustScrollPosition()
			m.adjustHorizontalScrollPosition()
		}

	case "j", "down":
		if m.selectedCol < len(m.board.Columns) {
			maxCard := len(m.getVisibleCards(m.selectedCol)) - 1
			if m.selectedCard < maxCard {
				m.selectedCard++
				m.columnCursorPos[m.selectedCol] = m.selectedCard
				m.adjustScrollPosition()
			}
		}

	case "k", "up":
		if m.selectedCard > 0 {
			m.selectedCard--
			m.columnCursorPos[m.selectedCol] = m.selectedCard
			m.adjustScrollPosition()
		}

	case "m", " ":
		if m.selectedCol < len(m.board.Columns) && len(m.getVisibleCards(m.selectedCol)) > 0 {
			m.mode = boardModeMove
		}

	case "enter":
		return m.handleEdit()

	case "n":
		return m.handleNew()

	case "d":
		if m.selectedCol < len(m.board.Columns) && m.selectedCard < len(m.getVisibleCards(m.selectedCol)) {
			return m.handleDueDateEdit()
		}

	case "t":
		if m.selectedCol < len(m.board.Columns) && m.selectedCard < len(m.getVisibleCards(m.selectedCol)) {
			return m.handleTagEdit()
		}

	case "p":
		if m.selectedCol < len(m.board.Columns) && m.selectedCard < len(m.getVisibleCards(m.selectedCol)) {
			return m.handleProjectEdit()
		}

	case "u":
		if m.selectedCol < len(m.board.Columns) && m.selectedCard < len(m.getVisibleCards(m.selectedCol)) {
			return m.handleOpenURL()
		}

	case "U":
		if m.selectedCol < len(m.board.Columns) && m.selectedCard < len(m.getVisibleCards(m.selectedCol)) {
			return m.handleURLEdit()
		}

	case "D":
		if m.selectedCol < len(m.board.Columns) && len(m.getVisibleCards(m.selectedCol)) > 0 {
			m.mode = boardModeConfirmDelete
		}

	case "s":
		if m.selectedCol < len(m.board.Columns) && m.selectedCard < len(m.getVisibleCards(m.selectedCol)) {
			return m.handleScheduledDateEdit()
		}

	case "i":
		if m.selectedCol < len(m.board.Columns) && m.selectedCard < len(m.getVisibleCards(m.selectedCol)) {
			return m.handlePriorityEdit()
		}

	case "c":
		return m.handleColumnEdit()

	case "M":
		if m.selectedCol < len(m.board.Columns) && len(m.getVisibleCards(m.selectedCol)) > 0 {
			return m.handleBoardMove()
		}

	case "a":
		if m.selectedCol < len(m.board.Columns) && len(m.getVisibleCards(m.selectedCol)) > 0 {
			realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
			if err := operations.ToggleCardArchive(&m.board, m.selectedCol, realIdx); err != nil {
				m.err = err
			} else {
				if m.board.Columns[m.selectedCol].Cards[realIdx].Archived {
					m.message = "Card archived"
				} else {
					m.message = "Card unarchived"
				}
				// Clamp cursor for potentially reduced visible set
				visibleCount := len(m.getVisibleCards(m.selectedCol))
				if m.selectedCard >= visibleCount && m.selectedCard > 0 {
					m.selectedCard--
				}
				m.columnCursorPos[m.selectedCol] = m.selectedCard
				m.adjustScrollPosition()
			}
		}

	case "x":
		if m.selectedCol < len(m.board.Columns) && len(m.getVisibleCards(m.selectedCol)) > 0 {
			return m.handleTmuxLaunch()
		}

	case "X":
		if m.selectedCol < len(m.board.Columns) && m.selectedCard < len(m.getVisibleCards(m.selectedCol)) {
			return m.handleTmuxEdit()
		}

	case "ctrl+a":
		m.showArchived = !m.showArchived
		m.clampFilteredCursors()
		m.adjustScrollPosition()
	}

	return m, nil
}

func (m BoardModel) updateMove(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", " ":
		m.mode = boardModeNormal
		return m, nil

	case "enter":
		m.mode = boardModeNormal
		return m.handleEdit()

	case "h", "left":
		if m.selectedCol > 0 {
			realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
			if err := operations.MoveCard(&m.board, m.selectedCol, realIdx, m.selectedCol-1); err != nil {
				m.err = err
			} else {
				m.selectedCol--
				// Card is appended to destination column, so it's at the end
				m.selectedCard = len(m.board.Columns[m.selectedCol].Cards) - 1
				if m.filterActive {
					m.recomputeFilter()
					m.selectedCard = max(0, len(m.getVisibleCards(m.selectedCol))-1)
				}
				m.columnCursorPos[m.selectedCol] = m.selectedCard
				m.adjustHorizontalScrollPosition()
				m.ensureCardBoardProjects(m.selectedCol, m.selectedCard)
			}
		}

	case "l", "right":
		if m.selectedCol < len(m.board.Columns)-1 {
			realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
			if err := operations.MoveCard(&m.board, m.selectedCol, realIdx, m.selectedCol+1); err != nil {
				m.err = err
			} else {
				m.selectedCol++
				// Card is appended to destination column, so it's at the end
				m.selectedCard = len(m.board.Columns[m.selectedCol].Cards) - 1
				if m.filterActive {
					m.recomputeFilter()
					m.selectedCard = max(0, len(m.getVisibleCards(m.selectedCol))-1)
				}
				m.columnCursorPos[m.selectedCol] = m.selectedCard
				m.adjustHorizontalScrollPosition()
				m.ensureCardBoardProjects(m.selectedCol, m.selectedCard)
			}
		}

	case "j", "down":
		realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
		col := m.board.Columns[m.selectedCol]
		if realIdx < len(col.Cards)-1 {
			if err := operations.ReorderCard(&m.board, m.selectedCol, realIdx, realIdx+1); err != nil {
				m.err = err
			} else {
				m.selectedCard++
				m.columnCursorPos[m.selectedCol] = m.selectedCard
				m.adjustScrollPosition()
			}
		}

	case "k", "up":
		realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
		if realIdx > 0 {
			if err := operations.ReorderCard(&m.board, m.selectedCol, realIdx, realIdx-1); err != nil {
				m.err = err
			} else {
				m.selectedCard--
				m.columnCursorPos[m.selectedCol] = m.selectedCard
				m.adjustScrollPosition()
			}
		}
	}

	return m, nil
}

func (m BoardModel) updateConfirmDelete(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	switch msg.String() {
	case "y":
		realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
		if err := operations.DeleteCard(&m.board, m.selectedCol, realIdx); err != nil {
			m.err = err
		} else {
			m.message = "Card deleted"
			if m.filterActive {
				m.recomputeFilter()
			}
			// Adjust selection against visible cards
			visibleCount := len(m.getVisibleCards(m.selectedCol))
			if m.selectedCard >= visibleCount && m.selectedCard > 0 {
				m.selectedCard--
			}

			// Sync saved cursor position
			m.columnCursorPos[m.selectedCol] = m.selectedCard

			// Adjust scroll position if needed
			if m.columnScrollOffsets[m.selectedCol] >= visibleCount {
				if m.columnScrollOffsets[m.selectedCol] > 0 {
					m.columnScrollOffsets[m.selectedCol]--
				}
			}
		}
		m.mode = boardModeNormal

	case "n", "esc":
		m.mode = boardModeNormal
	}

	return m, nil
}

func (m BoardModel) updateFilter(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Lock filter and return to normal mode
		m.filterQuery = m.filterInput.Value()
		if m.filterQuery != "" {
			m.filterActive = true
			m.recomputeFilter()
			// Reset cursors for filtered view
			m.selectedCard = 0
			m.columnCursorPos[m.selectedCol] = 0
			m.adjustScrollPosition()
		} else {
			m.filterActive = false
			m.filteredIndices = nil
		}
		m.mode = boardModeNormal
		return m, nil

	case "esc":
		// Clear filter entirely
		m.filterQuery = ""
		m.filterActive = false
		m.filteredIndices = nil
		m.mode = boardModeNormal
		m.selectedCard = 0
		m.columnCursorPos[m.selectedCol] = 0
		m.adjustScrollPosition()
		return m, nil

	default:
		// Forward to textinput
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		// Live recompute
		m.filterQuery = m.filterInput.Value()
		if m.filterQuery != "" {
			m.filterActive = true
			m.recomputeFilter()
		} else {
			m.filterActive = false
			m.filteredIndices = nil
		}
		m.clampFilteredCursors()
		m.adjustScrollPosition()
		return m, cmd
	}
}

type editorFinishedMsg struct {
	err error
}

func openEditor(boardPath, filename string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	cardPath := filepath.Join(boardPath, "cards", filename)
	c := exec.Command(editor, cardPath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func (m BoardModel) handleEdit() (BoardModel, tea.Cmd) {
	if m.selectedCol >= len(m.board.Columns) {
		return m, nil
	}

	visibleCards := m.getVisibleCards(m.selectedCol)
	if m.selectedCard >= len(visibleCards) {
		return m, nil
	}

	realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
	card := m.board.Columns[m.selectedCol].Cards[realIdx]

	// Open editor
	return m, openEditor(m.board.Path, card.Filename)
}

func (m BoardModel) handleNew() (BoardModel, tea.Cmd) {
	if m.selectedCol >= len(m.board.Columns) {
		return m, nil
	}

	columnName := m.board.Columns[m.selectedCol].Name
	card, err := operations.CreateCard(&m.board, columnName)
	if err != nil {
		m.err = err
		return m, nil
	}

	// Open editor for the new card
	return m, openEditor(m.board.Path, card.Filename)
}

func (m BoardModel) handleTagEdit() (BoardModel, tea.Cmd) {
	allTags := operations.CollectAllTags(&m.board)
	realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
	currentCard := m.board.Columns[m.selectedCol].Cards[realIdx]

	picker := NewTagPickerModel(currentCard.Tags, allTags)
	m.tagPicker = &picker
	m.mode = boardModeTagEdit

	return m, picker.Init()
}

func (m BoardModel) updateTagEdit(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	var cmd tea.Cmd
	var isDone bool

	*m.tagPicker, cmd, isDone = m.tagPicker.Update(msg)

	if isDone {
		// Save tags if confirmed with enter
		if msg.String() == "enter" {
			realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
			newTags := m.tagPicker.GetSelectedTags()
			err := operations.UpdateCardTags(&m.board, m.selectedCol, realIdx, newTags)
			if err != nil {
				m.err = err
			} else {
				// Reload board to refresh display
				board, err := fs.ReadBoard(m.board.Path)
				if err != nil {
					m.err = err
				} else {
					m.board = board
					m.message = "Tags updated"
					m.reloadBoardState()
					m.ensureCardBoardProjects(m.selectedCol, realIdx)
				}
			}
		}

		// Exit tag edit mode
		m.mode = boardModeNormal
		m.tagPicker = nil
	}

	return m, cmd
}

func (m BoardModel) handleProjectEdit() (BoardModel, tea.Cmd) {
	allProjects := m.allProjects
	realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
	currentCard := m.board.Columns[m.selectedCol].Cards[realIdx]

	picker := NewProjectPickerModel(currentCard.Projects, allProjects)
	m.projectPicker = &picker
	m.mode = boardModeProjectEdit

	return m, picker.Init()
}

func (m BoardModel) updateProjectEdit(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	var cmd tea.Cmd
	var isDone bool

	*m.projectPicker, cmd, isDone = m.projectPicker.Update(msg)

	if isDone {
		// Save projects if confirmed with enter
		if msg.String() == "enter" {
			realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
			newProjects := m.projectPicker.GetSelectedProjects()
			err := operations.UpdateCardProjects(&m.board, m.selectedCol, realIdx, newProjects)
			if err != nil {
				m.err = err
			} else {
				// Reload board to refresh display
				board, err := fs.ReadBoard(m.board.Path)
				if err != nil {
					m.err = err
				} else {
					m.board = board
					m.message = "Projects updated"
					m.reloadBoardState()
					m.ensureCardBoardProjects(m.selectedCol, realIdx)
				}
			}
		}

		// Exit project edit mode
		m.mode = boardModeNormal
		m.projectPicker = nil
	}

	return m, cmd
}

func (m BoardModel) handleColumnEdit() (BoardModel, tea.Cmd) {
	editor := NewColumnEditorModel(&m.board)
	editor.width = m.width
	editor.height = m.height
	m.columnEditor = &editor
	m.mode = boardModeColumnEdit
	return m, editor.Init()
}

func (m BoardModel) updateColumnEdit(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	var cmd tea.Cmd
	var isDone bool

	*m.columnEditor, cmd, isDone = m.columnEditor.Update(msg)

	if isDone {
		// Save changes if confirmed with enter
		if msg.String() == "enter" {
			// Apply changes from editor
			m.board.Columns = m.columnEditor.columns

			// Persist to disk
			err := fs.WriteBoard(m.board)
			if err != nil {
				m.err = err
			} else {
				// Reload board to refresh
				board, err := fs.ReadBoard(m.board.Path)
				if err != nil {
					m.err = err
				} else {
					m.board = board
					m.message = "Columns updated"

					// Resize arrays for new column count
					m.columnScrollOffsets = make([]int, len(m.board.Columns))
					m.columnCursorPos = make([]int, len(m.board.Columns))

					// Reset to first column
					m.selectedCol = 0
					m.selectedCard = 0

					if m.filterActive {
						m.recomputeFilter()
					}
				}
			}
		}

		// Exit column edit mode
		m.mode = boardModeNormal
		m.columnEditor = nil
	}

	return m, cmd
}

func (m BoardModel) handleURLEdit() (BoardModel, tea.Cmd) {
	realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
	currentCard := m.board.Columns[m.selectedCol].Cards[realIdx]
	editor := NewURLEditorModel(currentCard.URLs)
	editor.width = m.width
	editor.height = m.height
	m.urlEditor = &editor
	m.mode = boardModeURLEditor
	return m, editor.Init()
}

func (m BoardModel) updateURLEditor(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	var cmd tea.Cmd
	var saved, done bool

	*m.urlEditor, cmd, saved, done = m.urlEditor.Update(msg)

	if done {
		if saved {
			realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
			newURLs := m.urlEditor.GetURLs()
			err := operations.UpdateCardURLs(&m.board, m.selectedCol, realIdx, newURLs)
			if err != nil {
				m.err = err
			} else {
				board, err := fs.ReadBoard(m.board.Path)
				if err != nil {
					m.err = err
				} else {
					m.board = board
					m.message = "URLs updated"
					m.reloadBoardState()
					m.ensureCardBoardProjects(m.selectedCol, realIdx)
				}
			}
		}
		m.mode = boardModeNormal
		m.urlEditor = nil
	}

	return m, cmd
}

func (m BoardModel) updateURLPicker(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	var selectedURL string
	var done bool

	*m.urlPicker, selectedURL, done = m.urlPicker.Update(msg)

	if done {
		m.mode = boardModeNormal
		m.urlPicker = nil
		if selectedURL != "" {
			err := operations.OpenURL(selectedURL)
			if err != nil {
				m.err = fmt.Errorf("failed to open URL: %v", err)
			} else {
				m.message = "Opening URL in browser..."
			}
		}
	}

	return m, nil
}

func (m BoardModel) handleDueDateEdit() (BoardModel, tea.Cmd) {
	realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
	currentCard := m.board.Columns[m.selectedCol].Cards[realIdx]
	datePickerModel := shared.NewDatePickerModel(currentCard.DueDate, "Due Date")
	datePickerModel.SetSize(m.width, m.height)
	m.dueDatePicker = &datePickerModel
	m.mode = boardModeDueDateEdit
	return m, datePickerModel.Init()
}

func (m BoardModel) updateDueDateEdit(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	var cmd tea.Cmd

	*m.dueDatePicker, cmd = m.dueDatePicker.Update(msg)

	switch msg.String() {
	case "enter", "c":
		// Save date (or clear if 'c' was pressed)
		realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
		newDate := m.dueDatePicker.GetDate()
		err := operations.UpdateCardDueDate(&m.board, m.selectedCol, realIdx, newDate)
		if err != nil {
			m.err = err
		} else {
			// Reload board to refresh display
			board, err := fs.ReadBoard(m.board.Path)
			if err != nil {
				m.err = err
			} else {
				m.board = board
				if newDate != nil {
					m.message = "Due date updated"
				} else {
					m.message = "Due date cleared"
				}
				m.reloadBoardState()
				m.ensureCardBoardProjects(m.selectedCol, realIdx)
			}
		}
		m.mode = boardModeNormal
		m.dueDatePicker = nil

	case "esc":
		// Cancel
		m.mode = boardModeNormal
		m.dueDatePicker = nil
	}

	return m, cmd
}

func (m BoardModel) handleScheduledDateEdit() (BoardModel, tea.Cmd) {
	realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
	currentCard := m.board.Columns[m.selectedCol].Cards[realIdx]
	datePickerModel := shared.NewDatePickerModel(currentCard.ScheduledDate, "Scheduled Date")
	datePickerModel.SetSize(m.width, m.height)
	m.scheduledDatePicker = &datePickerModel
	m.mode = boardModeScheduledDateEdit
	return m, datePickerModel.Init()
}

func (m BoardModel) updateScheduledDateEdit(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	var cmd tea.Cmd

	*m.scheduledDatePicker, cmd = m.scheduledDatePicker.Update(msg)

	switch msg.String() {
	case "enter", "c":
		// Save date (or clear if 'c' was pressed)
		realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
		newDate := m.scheduledDatePicker.GetDate()
		err := operations.UpdateCardScheduledDate(&m.board, m.selectedCol, realIdx, newDate)
		if err != nil {
			m.err = err
		} else {
			// Reload board to refresh display
			board, err := fs.ReadBoard(m.board.Path)
			if err != nil {
				m.err = err
			} else {
				m.board = board
				if newDate != nil {
					m.message = "Scheduled date updated"
				} else {
					m.message = "Scheduled date cleared"
				}
				m.reloadBoardState()
				m.ensureCardBoardProjects(m.selectedCol, realIdx)
			}
		}
		m.mode = boardModeNormal
		m.scheduledDatePicker = nil

	case "esc":
		// Cancel
		m.mode = boardModeNormal
		m.scheduledDatePicker = nil
	}

	return m, cmd
}

func (m BoardModel) handlePriorityEdit() (BoardModel, tea.Cmd) {
	realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
	currentCard := m.board.Columns[m.selectedCol].Cards[realIdx]
	priorityInputModel := NewPriorityInputModel(currentCard.Priority)
	priorityInputModel.width = m.width
	priorityInputModel.height = m.height
	m.priorityInput = &priorityInputModel
	m.mode = boardModePriorityInput
	return m, priorityInputModel.Init()
}

func (m BoardModel) updatePriorityInput(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	var isDone bool

	*m.priorityInput, isDone = m.priorityInput.Update(msg)

	if isDone {
		if msg.String() == "enter" {
			realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
			newPriority := m.priorityInput.GetPriority()
			err := operations.UpdateCardPriority(&m.board, m.selectedCol, realIdx, newPriority)
			if err != nil {
				m.err = err
			} else {
				board, err := fs.ReadBoard(m.board.Path)
				if err != nil {
					m.err = err
				} else {
					m.board = board
					if newPriority > 0 {
						m.message = fmt.Sprintf("Priority set to %d", newPriority)
					} else {
						m.message = "Priority cleared"
					}
					m.reloadBoardState()
					m.ensureCardBoardProjects(m.selectedCol, realIdx)
				}
			}
		}
		m.mode = boardModeNormal
		m.priorityInput = nil
	}

	return m, nil
}

func (m BoardModel) handleOpenURL() (BoardModel, tea.Cmd) {
	realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
	currentCard := m.board.Columns[m.selectedCol].Cards[realIdx]

	if !currentCard.HasURLs() {
		m.message = "No URLs set for this card"
		return m, nil
	}

	// Single URL — open directly
	if len(currentCard.URLs) == 1 {
		err := operations.OpenURL(currentCard.URLs[0].URL)
		if err != nil {
			m.err = fmt.Errorf("failed to open URL: %v", err)
		} else {
			m.message = "Opening URL in browser..."
		}
		return m, nil
	}

	// Multiple URLs — open picker
	picker := NewURLPickerModel(currentCard.URLs)
	picker.width = m.width
	picker.height = m.height
	m.urlPicker = &picker
	m.mode = boardModeURLPicker
	return m, nil
}

func (m BoardModel) handleBoardMove() (BoardModel, tea.Cmd) {
	selector := NewBoardSelectorModel(m.allBoards, m.board.Path)
	if selector.Empty() {
		m.message = "No other boards available"
		return m, nil
	}
	selector.width = m.width
	selector.height = m.height
	m.boardSelector = &selector
	m.mode = boardModeBoardMove
	return m, nil
}

func (m BoardModel) updateBoardMove(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	var selectedPath string
	var done bool

	*m.boardSelector, selectedPath, done = m.boardSelector.Update(msg)

	if done {
		if selectedPath != "" {
			// Load target board
			dstBoard, err := fs.ReadBoard(selectedPath)
			if err != nil {
				m.err = fmt.Errorf("load target board: %w", err)
			} else {
				realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
				err := operations.MoveCardToBoard(&m.board, m.selectedCol, realIdx, &dstBoard, m.boardProjects)
				if err != nil {
					m.err = err
				} else {
					m.message = fmt.Sprintf("Card moved to %s", dstBoard.Name)
					if m.filterActive {
						m.recomputeFilter()
					}
					// Adjust selection (same pattern as delete)
					visibleCount := len(m.getVisibleCards(m.selectedCol))
					if m.selectedCard >= visibleCount && m.selectedCard > 0 {
						m.selectedCard--
					}
					m.columnCursorPos[m.selectedCol] = m.selectedCard
					if m.columnScrollOffsets[m.selectedCol] >= visibleCount {
						if m.columnScrollOffsets[m.selectedCol] > 0 {
							m.columnScrollOffsets[m.selectedCol]--
						}
					}
				}
			}
		}
		m.mode = boardModeNormal
		m.boardSelector = nil
	}

	return m, nil
}

func (m BoardModel) handleTmuxEdit() (BoardModel, tea.Cmd) {
	realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
	currentCard := m.board.Columns[m.selectedCol].Cards[realIdx]
	picker := NewTmuxPickerModel(currentCard.TmuxSession)
	picker.width = m.width
	picker.height = m.height
	m.tmuxPicker = &picker
	m.mode = boardModeTmuxPicker
	return m, nil
}

func (m BoardModel) updateTmuxPicker(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	var selectedSession string
	var done bool

	*m.tmuxPicker, selectedSession, done = m.tmuxPicker.Update(msg)

	if done {
		realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
		currentCard := m.board.Columns[m.selectedCol].Cards[realIdx]

		if selectedSession != currentCard.TmuxSession {
			err := operations.UpdateCardTmuxSession(&m.board, m.selectedCol, realIdx, selectedSession)
			if err != nil {
				m.err = err
			} else {
				board, err := fs.ReadBoard(m.board.Path)
				if err != nil {
					m.err = err
				} else {
					m.board = board
					if selectedSession != "" {
						m.message = "Tmux session linked: " + selectedSession
					} else {
						m.message = "Tmux session unlinked"
					}
					m.reloadBoardState()
				}
			}
		}
		m.mode = boardModeNormal
		m.tmuxPicker = nil
	}

	return m, nil
}

func (m BoardModel) handleTmuxLaunch() (BoardModel, tea.Cmd) {
	realIdx := m.resolveCardIndex(m.selectedCol, m.selectedCard)
	currentCard := m.board.Columns[m.selectedCol].Cards[realIdx]

	if currentCard.TmuxSession == "" {
		m.message = "No tmux session linked"
		return m, nil
	}

	// Check if any children exist
	children := getChildSessions(currentCard.TmuxSession)
	hasChildren := false
	for _, exists := range children {
		if exists {
			hasChildren = true
			break
		}
	}

	if !hasChildren {
		// Switch directly to root session
		m.message = "Switching to " + currentCard.TmuxSession
		return m, switchTmuxSession(currentCard.TmuxSession)
	}

	// Show launch popup
	launch := NewTmuxLaunchModel(currentCard.TmuxSession)
	launch.width = m.width
	launch.height = m.height
	m.tmuxLaunch = &launch
	m.mode = boardModeTmuxLaunch
	return m, nil
}

func (m BoardModel) updateTmuxLaunch(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	var targetSession string
	var done bool

	*m.tmuxLaunch, targetSession, done = m.tmuxLaunch.Update(msg)

	if done {
		m.mode = boardModeNormal
		m.tmuxLaunch = nil
		if targetSession != "" {
			m.message = "Switching to " + targetSession
			return m, switchTmuxSession(targetSession)
		}
	}

	return m, nil
}

func (m BoardModel) View() string {
	// Show tag picker if in tag edit mode
	if m.mode == boardModeTagEdit && m.tagPicker != nil {
		return m.tagPicker.View()
	}

	// Show project picker if in project edit mode
	if m.mode == boardModeProjectEdit && m.projectPicker != nil {
		return m.projectPicker.View()
	}

	// Show column editor if in column edit mode
	if m.mode == boardModeColumnEdit && m.columnEditor != nil {
		return m.columnEditor.View()
	}

	// Show URL picker if in URL picker mode
	if m.mode == boardModeURLPicker && m.urlPicker != nil {
		return m.urlPicker.View()
	}

	// Show URL editor if in URL editor mode
	if m.mode == boardModeURLEditor && m.urlEditor != nil {
		return m.urlEditor.View()
	}

	// Show due date picker if in due date edit mode
	if m.mode == boardModeDueDateEdit && m.dueDatePicker != nil {
		return m.dueDatePicker.View()
	}

	// Show scheduled date picker if in scheduled date edit mode
	if m.mode == boardModeScheduledDateEdit && m.scheduledDatePicker != nil {
		return m.scheduledDatePicker.View()
	}

	// Show priority input if in priority input mode
	if m.mode == boardModePriorityInput && m.priorityInput != nil {
		return m.priorityInput.View()
	}

	// Show board selector if in board move mode
	if m.mode == boardModeBoardMove && m.boardSelector != nil {
		return m.boardSelector.View()
	}

	// Show tmux picker if in tmux picker mode
	if m.mode == boardModeTmuxPicker && m.tmuxPicker != nil {
		return m.tmuxPicker.View()
	}

	// Show tmux launch popup if in tmux launch mode
	if m.mode == boardModeTmuxLaunch && m.tmuxLaunch != nil {
		return m.tmuxLaunch.View()
	}

	var s strings.Builder

	// Title
	s.WriteString(titleStyle.Render(fmt.Sprintf("Board: %s", m.board.Name)))
	s.WriteString("\n")

	// Filter bar
	if m.mode == boardModeFilter {
		s.WriteString("  / " + m.filterInput.View())
	} else if m.filterActive {
		s.WriteString("  " + filterIndicatorStyle.Render("Filter: "+m.filterQuery))
	}
	s.WriteString("\n")

	// Calculate fixed column height
	boardHeaderLines := 3
	statusLines := 3
	marginLines := 2

	totalFixedColumnHeight := m.height - boardHeaderLines - statusLines - marginLines

	if totalFixedColumnHeight < 10 {
		totalFixedColumnHeight = 10
	}

	// Render columns with fixed height and horizontal scrolling
	startCol, endCol := m.calculateVisibleColumns()
	visibleColumnViews := []string{}

	// Left scroll indicator space (always allocated)
	if startCol > 0 {
		leftIndicator := m.renderScrollIndicator("◀", totalFixedColumnHeight)
		visibleColumnViews = append(visibleColumnViews, leftIndicator)
	} else {
		emptySpace := m.renderScrollIndicator(" ", totalFixedColumnHeight)
		visibleColumnViews = append(visibleColumnViews, emptySpace)
	}

	// Render visible columns only
	for i := startCol; i < endCol; i++ {
		col := m.board.Columns[i]
		cards := m.getVisibleCards(i)
		colView := m.renderColumn(i, col, cards, totalFixedColumnHeight)
		visibleColumnViews = append(visibleColumnViews, colView)
	}

	// Right scroll indicator space (always allocated)
	if endCol < len(m.board.Columns) {
		rightIndicator := m.renderScrollIndicator("▶", totalFixedColumnHeight)
		visibleColumnViews = append(visibleColumnViews, rightIndicator)
	} else {
		emptySpace := m.renderScrollIndicator(" ", totalFixedColumnHeight)
		visibleColumnViews = append(visibleColumnViews, emptySpace)
	}

	columns := lipgloss.JoinHorizontal(lipgloss.Top, visibleColumnViews...)
	centeredColumns := lipgloss.Place(m.width, 0, lipgloss.Center, lipgloss.Top, columns)
	s.WriteString(centeredColumns)
	s.WriteString("\n")

	// Status message or error
	if m.err != nil {
		s.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		s.WriteString("\n")
	} else if m.message != "" {
		s.WriteString(successStyle.Render(m.message))
		s.WriteString("\n")
	}

	return s.String()
}

func (m BoardModel) renderColumn(index int, col models.Column, cards []models.Card, fixedHeight int) string {
	var s strings.Builder

	// Column title
	colTitleStyle := columnTitleStyle
	if index == m.selectedCol {
		colTitleStyle = selectedColumnTitleStyle
	}
	s.WriteString(colTitleStyle.Render(col.Name))
	s.WriteString("\n\n")

	// Handle empty column
	if len(cards) == 0 {
		s.WriteString(cardPreviewStyle.Render("(empty)"))
		s.WriteString("\n")

		style := columnStyle
		if index == m.selectedCol {
			style = selectedColumnStyle
		}
		return style.Height(fixedHeight).Render(s.String())
	}

	// Get scroll offset
	scrollOffset := 0
	if index < len(m.columnScrollOffsets) {
		scrollOffset = m.columnScrollOffsets[index]
	}

	cardsAbove := scrollOffset

	// Top scroll indicator (always reserve space)
	if cardsAbove > 0 {
		indicator := scrollIndicatorStyle.Render(fmt.Sprintf("▲ +%d cards above", cardsAbove))
		s.WriteString(indicator)
		s.WriteString("\n")
		s.WriteString("\n")
	} else {
		s.WriteString("\n")
		s.WriteString("\n")
	}

	// Calculate available space for cards
	overhead := 8
	availableCardSpace := fixedHeight - overhead

	// Render cards that fit in available space
	var cardBuilder strings.Builder
	cardsRendered := 0
	currentCardHeight := 0

	for i := scrollOffset; i < len(cards); i++ {
		card := cards[i]
		cardView := m.renderCard(index, i, card)
		cardHeight := lipgloss.Height(cardView)

		if cardsRendered > 0 && currentCardHeight+cardHeight > availableCardSpace {
			break
		}

		cardBuilder.WriteString(cardView)
		cardBuilder.WriteString("\n")
		cardsRendered++
		currentCardHeight += cardHeight
	}

	s.WriteString(cardBuilder.String())

	cardsBelow := len(cards) - scrollOffset - cardsRendered

	if cardsBelow > 0 {
		indicator := scrollIndicatorStyle.Render(fmt.Sprintf("▼ +%d cards below", cardsBelow))
		s.WriteString(indicator)
	}

	style := columnStyle
	if index == m.selectedCol {
		style = selectedColumnStyle
	}

	return style.Height(fixedHeight).Render(s.String())
}

func (m BoardModel) renderCard(colIndex, cardIndex int, card models.Card) string {
	maxWidth := columnWidth - (2 * columnPaddingHorizontal) - cardBorderWidth - (2 * cardPaddingHorizontal)

	var lines []string

	// Line 1: Card title with priority prefix and URL indicator
	title := card.Title
	urlIndicator := ""
	if card.HasURLs() {
		urlIndicator = "↗"
	}

	priorityPrefix := ""
	priorityPrefixWidth := 0
	if card.Priority > 0 {
		priorityPrefix = fmt.Sprintf("%d ", card.Priority)
		priorityPrefixWidth = len(priorityPrefix)
	}

	effectiveMaxWidth := maxWidth - priorityPrefixWidth
	if urlIndicator != "" {
		effectiveMaxWidth -= 2
	}

	if len(title) > effectiveMaxWidth {
		title = title[:effectiveMaxWidth-3] + "..."
	}

	if urlIndicator != "" {
		title = title + " " + urlIndicator
	}

	isSelected := colIndex == m.selectedCol && cardIndex == m.selectedCard
	if card.Priority > 0 {
		pStyle := lipgloss.NewStyle().Bold(true).Foreground(priorityColor(card.Priority))
		tStyle := cardTitleStyle
		if isSelected {
			pStyle = pStyle.Background(theme.Surface)
			tStyle = tStyle.Background(theme.Surface)
		}
		lines = append(lines, pStyle.Render(priorityPrefix)+tStyle.Render(title))
	} else {
		lines = append(lines, cardTitleStyle.Render(title))
	}

	// Line 2: Preview/Description (only if not empty)
	if card.Preview != "" {
		preview := strings.ReplaceAll(card.Preview, "\n", " ")
		preview = strings.ReplaceAll(preview, "\r", " ")
		preview = strings.Join(strings.Fields(preview), " ")
		if len(preview) > maxWidth {
			preview = preview[:maxWidth-3] + "..."
		}
		lines = append(lines, cardPreviewStyle.Render(preview))
	}

	// Line 3-4: Dates
	isDone := m.board.IsDoneColumn(m.board.Columns[colIndex].Name)
	if isDone {
		// In done column: show completed date in green, hide due/scheduled
		if card.DateCompleted != nil {
			dateStr := fmt.Sprintf("done:%02d-%02d %s",
				card.DateCompleted.Month(), card.DateCompleted.Day(),
				strings.ToLower(card.DateCompleted.Weekday().String()[:3]))
			lines = append(lines, theme.Ok.Render(dateStr))
		}
	} else {
		// Not in done column: show due/scheduled dates
		isSelected := colIndex == m.selectedCol && cardIndex == m.selectedCard
		if card.DueDate != nil {
			lines = append(lines, formatDateWithDaysUntil(card.DueDate, "D", isSelected))
		}
		if card.ScheduledDate != nil {
			lines = append(lines, formatDateWithDaysUntil(card.ScheduledDate, "S", isSelected))
		}
	}

	// Line 5: Projects (only if not empty)
	if len(card.Projects) > 0 {
		projectsLine := "+" + strings.Join(card.Projects, " +")
		if len(projectsLine) > maxWidth {
			projectsLine = projectsLine[:maxWidth-3] + "..."
		}
		lines = append(lines, cardProjectStyle.Render(projectsLine))
	}

	// Line 6: Tags (only if not empty)
	if len(card.Tags) > 0 {
		tagsLine := "#" + strings.Join(card.Tags, " #")
		if len(tagsLine) > maxWidth {
			tagsLine = tagsLine[:maxWidth-3] + "..."
		}
		lines = append(lines, cardTagStyle.Render(tagsLine))
	}

	// Tmux session indicator
	if card.TmuxSession != "" {
		tmuxLine := " " + card.TmuxSession + " "
		if len(tmuxLine) > maxWidth {
			tmuxLine = tmuxLine[:maxWidth-3] + "..."
		}
		lines = append(lines, cardTmuxStyle.Render(tmuxLine))
	}

	// Archived indicator (only visible when showArchived is on)
	if card.Archived {
		archivedStyle := lipgloss.NewStyle().Background(theme.Primary).Foreground(lipgloss.Color("16"))
		lines = append(lines, archivedStyle.Render(" archived "))
	}

	content := strings.Join(lines, "\n")

	style := cardStyle
	if colIndex == m.selectedCol && cardIndex == m.selectedCard {
		style = selectedCardStyle
	}

	return style.Render(content)
}

// formatDateWithDaysUntil formats a date with days until/overdue, coloring only the offset
func formatDateWithDaysUntil(date *time.Time, prefix string, selected bool) string {
	if date == nil {
		return ""
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	targetDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)

	daysUntil := int(targetDate.Sub(today).Hours() / 24)

	dayOfWeek := strings.ToLower(date.Weekday().String()[:3])
	datePart := fmt.Sprintf("%s:%02d-%02d %s", prefix, date.Month(), date.Day(), dayOfWeek)
	offsetPart := fmt.Sprintf(" %+d", -daysUntil)

	var offsetColor lipgloss.Color
	if daysUntil > 7 {
		offsetColor = theme.Success
	} else if daysUntil > 0 {
		offsetColor = theme.Warning
	} else {
		offsetColor = theme.Danger
	}

	dateStyle := lipgloss.NewStyle().Bold(true)
	offsetStyle := lipgloss.NewStyle().Foreground(offsetColor).Bold(true)

	if selected {
		dateStyle = dateStyle.Background(theme.Surface)
		offsetStyle = offsetStyle.Background(theme.Surface)
	}

	return dateStyle.Render(datePart) + offsetStyle.Render(offsetPart)
}

// reloadBoardState syncs arrays and validates cursors after a board reload
func (m *BoardModel) reloadBoardState() {
	if len(m.columnScrollOffsets) != len(m.board.Columns) {
		newOffsets := make([]int, len(m.board.Columns))
		copy(newOffsets, m.columnScrollOffsets)
		m.columnScrollOffsets = newOffsets
	}

	if len(m.columnCursorPos) != len(m.board.Columns) {
		newCursorPos := make([]int, len(m.board.Columns))
		copy(newCursorPos, m.columnCursorPos)
		m.columnCursorPos = newCursorPos
	}

	if m.filterActive {
		m.recomputeFilter()
		m.clampFilteredCursors()
	} else {
		if m.selectedCol < len(m.board.Columns) {
			if m.selectedCard >= len(m.board.Columns[m.selectedCol].Cards) {
				m.selectedCard = max(0, len(m.board.Columns[m.selectedCol].Cards)-1)
				m.columnCursorPos[m.selectedCol] = m.selectedCard
			}
		}
	}

	if m.columnHorizontalOffset >= len(m.board.Columns) {
		m.columnHorizontalOffset = max(0, len(m.board.Columns)-1)
	}
	m.adjustHorizontalScrollPosition()
}

// ensureCardBoardProjects adds any missing board projects to the card's frontmatter.
// It's a no-op when boardProjects is empty or the card already has them all.
func (m *BoardModel) ensureCardBoardProjects(colIndex, cardIndex int) {
	if len(m.boardProjects) == 0 {
		return
	}
	_ = operations.EnsureBoardProjects(&m.board, colIndex, cardIndex, m.boardProjects)
}

// cardSearchString builds a single string from all card fields for fuzzy matching
func cardSearchString(card models.Card) string {
	var parts []string
	parts = append(parts, card.Title)
	if card.Preview != "" {
		parts = append(parts, card.Preview)
	}
	for _, tag := range card.Tags {
		parts = append(parts, "#"+tag)
	}
	for _, proj := range card.Projects {
		parts = append(parts, "+"+proj)
	}
	for _, u := range card.URLs {
		if u.Label != "" {
			parts = append(parts, u.Label)
		}
		parts = append(parts, u.URL)
	}
	if card.DueDate != nil {
		parts = append(parts, "due:"+card.DueDate.Format("2006-01-02"))
	}
	if card.ScheduledDate != nil {
		parts = append(parts, "scheduled:"+card.ScheduledDate.Format("2006-01-02"))
	}
	if card.Priority > 0 {
		parts = append(parts, fmt.Sprintf("priority:%d", card.Priority))
	}
	if card.TmuxSession != "" {
		parts = append(parts, "tmux:"+card.TmuxSession)
	}
	return strings.Join(parts, " ")
}

// recomputeFilter rebuilds filteredIndices for each column based on the current filterQuery
func (m *BoardModel) recomputeFilter() {
	if m.filterQuery == "" {
		m.filterActive = false
		m.filteredIndices = nil
		return
	}

	m.filteredIndices = make([][]int, len(m.board.Columns))
	for colIdx, col := range m.board.Columns {
		searchStrings := make([]string, len(col.Cards))
		for i, card := range col.Cards {
			searchStrings[i] = cardSearchString(card)
		}
		matches := fuzzy.Find(m.filterQuery, searchStrings)
		indices := make([]int, len(matches))
		for i, match := range matches {
			indices[i] = match.Index
		}
		m.filteredIndices[colIdx] = indices
	}
}

// getVisibleCardIndices returns the real card indices that should be visible,
// respecting both archive filtering and fuzzy filter.
func (m *BoardModel) getVisibleCardIndices(colIndex int) []int {
	allCards := m.board.Columns[colIndex].Cards

	// Start with all indices
	var baseIndices []int
	if m.filterActive && m.filteredIndices != nil && colIndex < len(m.filteredIndices) {
		baseIndices = m.filteredIndices[colIndex]
	} else {
		baseIndices = make([]int, len(allCards))
		for i := range allCards {
			baseIndices[i] = i
		}
	}

	// Filter out archived cards unless showArchived
	if m.showArchived {
		return baseIndices
	}

	visible := make([]int, 0, len(baseIndices))
	for _, idx := range baseIndices {
		if !allCards[idx].Archived {
			visible = append(visible, idx)
		}
	}
	return visible
}

// getVisibleCards returns the cards to display for a column, respecting the active filter and archive state
func (m *BoardModel) getVisibleCards(colIndex int) []models.Card {
	indices := m.getVisibleCardIndices(colIndex)
	allCards := m.board.Columns[colIndex].Cards
	cards := make([]models.Card, len(indices))
	for i, idx := range indices {
		cards[i] = allCards[idx]
	}
	return cards
}

// resolveCardIndex translates a visible position back to the real card index
func (m *BoardModel) resolveCardIndex(colIndex, filteredIndex int) int {
	indices := m.getVisibleCardIndices(colIndex)
	if filteredIndex < len(indices) {
		return indices[filteredIndex]
	}
	return filteredIndex
}

// clampFilteredCursors ensures cursor positions are valid for the filtered card sets
func (m *BoardModel) clampFilteredCursors() {
	if m.selectedCol >= len(m.board.Columns) {
		return
	}
	visibleCount := len(m.getVisibleCards(m.selectedCol))
	if m.selectedCard >= visibleCount {
		m.selectedCard = max(0, visibleCount-1)
	}
	m.columnCursorPos[m.selectedCol] = m.selectedCard
}

// adjustScrollPosition ensures the selected card is visible by adjusting scroll offset
func (m *BoardModel) adjustScrollPosition() {
	if m.selectedCol >= len(m.board.Columns) {
		return
	}

	cards := m.getVisibleCards(m.selectedCol)
	if len(cards) == 0 {
		return
	}

	boardHeaderLines := 3
	statusLines := 3
	marginLines := 2
	fixedColumnHeight := m.height - boardHeaderLines - statusLines - marginLines

	if fixedColumnHeight < 10 {
		fixedColumnHeight = 10
	}

	availableCardHeight := fixedColumnHeight - 8

	scrollOffset := m.columnScrollOffsets[m.selectedCol]

	if m.selectedCard < scrollOffset {
		m.columnScrollOffsets[m.selectedCol] = m.selectedCard
	} else {
		visibleCards := 0
		accumulatedHeight := 0

		for i := scrollOffset; i < len(cards); i++ {
			card := cards[i]
			cardView := m.renderCard(m.selectedCol, i, card)
			cardHeight := lipgloss.Height(cardView)

			if visibleCards > 0 && accumulatedHeight+cardHeight > availableCardHeight {
				break
			}

			accumulatedHeight += cardHeight
			visibleCards++
		}

		if visibleCards < 1 {
			visibleCards = 1
		}

		if m.selectedCard >= scrollOffset+visibleCards {
			m.columnScrollOffsets[m.selectedCol] = m.selectedCard - visibleCards + 1
		}
	}

	if m.columnScrollOffsets[m.selectedCol] < 0 {
		m.columnScrollOffsets[m.selectedCol] = 0
	}
	maxOffset := len(cards) - 1
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.columnScrollOffsets[m.selectedCol] > maxOffset {
		m.columnScrollOffsets[m.selectedCol] = maxOffset
	}
}

// calculateVisibleColumns determines which columns fit in terminal width
func (m *BoardModel) calculateVisibleColumns() (startCol, endCol int) {
	columnTotalWidth := 46
	availableWidth := m.width

	startCol = m.columnHorizontalOffset

	leftIndicatorWidth := 5
	rightIndicatorWidth := 5

	widthForColumns := availableWidth - leftIndicatorWidth - rightIndicatorWidth
	visibleCount := widthForColumns / columnTotalWidth

	if visibleCount < 1 {
		visibleCount = 1
	}

	endCol = min(startCol+visibleCount, len(m.board.Columns))

	if endCol <= startCol && len(m.board.Columns) > 0 {
		endCol = startCol + 1
	}

	return startCol, endCol
}

// renderScrollIndicator renders ◀ and ▶ indicators for horizontal scrolling
func (m *BoardModel) renderScrollIndicator(symbol string, height int) string {
	indicatorStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	indicator := indicatorStyle.Render(symbol)
	return lipgloss.NewStyle().
		Width(3).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(indicator)
}

// adjustHorizontalScrollPosition ensures the selected column is visible
func (m *BoardModel) adjustHorizontalScrollPosition() {
	if len(m.board.Columns) == 0 {
		return
	}

	startCol, endCol := m.calculateVisibleColumns()

	if m.selectedCol < startCol {
		m.columnHorizontalOffset = m.selectedCol
		return
	}

	if m.selectedCol >= endCol {
		visibleCount := endCol - startCol
		m.columnHorizontalOffset = m.selectedCol - visibleCount + 1
		if m.columnHorizontalOffset < 0 {
			m.columnHorizontalOffset = 0
		}
		return
	}
}
