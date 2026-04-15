package operations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"wydo/internal/kanban/fs"
	"wydo/internal/kanban/models"
)

// CreateTaskNote creates a new task note in the specified column
func CreateTaskNote(board *models.Board, columnName string) (models.TaskNote, error) {
	cardsDir := filepath.Join(board.Path, "cards")

	if err := os.MkdirAll(cardsDir, 0755); err != nil {
		return models.TaskNote{}, err
	}

	defaultTitle := ""
	baseFilename := ToSnakeCase(defaultTitle)
	filename := UniqueFilename(baseFilename, cardsDir, "")

	cardPath := filepath.Join(cardsDir, filename)

	tn := models.TaskNote{
		Filename: filename,
		Title:    defaultTitle,
		Tags:     []string{},
		Content:  "# \n",
	}

	if err := fs.WriteNewTaskNote(tn, cardPath); err != nil {
		return models.TaskNote{}, err
	}

	col := board.GetColumn(columnName)
	if col != nil {
		col.TaskNotes = append(col.TaskNotes, tn)
		if err := fs.WriteBoard(*board); err != nil {
			return models.TaskNote{}, err
		}
	}

	return tn, nil
}

// SyncTaskNoteFilename renames a task note file if its title has changed
func SyncTaskNoteFilename(board *models.Board, columnIndex, cardIndex int) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.TaskNotes) {
		return fmt.Errorf("invalid card index")
	}

	tn := &column.TaskNotes[cardIndex]
	cardsDir := filepath.Join(board.Path, "cards")
	cardPath := filepath.Join(cardsDir, tn.Filename)

	updatedTN, err := fs.ReadTaskNote(cardPath)
	if err != nil {
		return err
	}

	expectedBase := ToSnakeCase(updatedTN.Title)
	expectedFilename := UniqueFilename(expectedBase, cardsDir, tn.Filename)

	if expectedFilename == tn.Filename {
		return nil
	}

	oldPath := cardPath
	newPath := filepath.Join(cardsDir, expectedFilename)
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}

	tn.Filename = expectedFilename

	return fs.WriteBoard(*board)
}

// EditTaskNote opens a task note in the user's editor
func EditTaskNote(boardPath, filename string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	cardPath := filepath.Join(boardPath, "cards", filename)

	cmd := exec.Command(editor, cardPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// DeleteTaskNote removes a task note file and its reference from the board
func DeleteTaskNote(board *models.Board, columnIndex, cardIndex int) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.TaskNotes) {
		return fmt.Errorf("invalid card index")
	}

	tn := column.TaskNotes[cardIndex]
	cardPath := filepath.Join(board.Path, "cards", tn.Filename)

	if err := os.Remove(cardPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	column.TaskNotes = append(column.TaskNotes[:cardIndex], column.TaskNotes[cardIndex+1:]...)

	return fs.WriteBoard(*board)
}

// MoveTaskNote moves a task note from one column to another
func MoveTaskNote(board *models.Board, fromColIndex, cardIndex, toColIndex int) error {
	if fromColIndex < 0 || fromColIndex >= len(board.Columns) {
		return fmt.Errorf("invalid source column index")
	}
	if toColIndex < 0 || toColIndex >= len(board.Columns) {
		return fmt.Errorf("invalid destination column index")
	}

	fromCol := &board.Columns[fromColIndex]
	if cardIndex < 0 || cardIndex >= len(fromCol.TaskNotes) {
		return fmt.Errorf("invalid card index")
	}

	tn := fromCol.TaskNotes[cardIndex]
	fromCol.TaskNotes = append(fromCol.TaskNotes[:cardIndex], fromCol.TaskNotes[cardIndex+1:]...)

	toCol := &board.Columns[toColIndex]

	// Stamp date_completed when moving to a done column
	if board.IsDoneColumn(toCol.Name) {
		now := time.Now()
		tn.DateCompleted = &now
		cardPath := filepath.Join(board.Path, "cards", tn.Filename)
		if err := fs.WriteTaskNote(tn, cardPath); err != nil {
			return err
		}
	}

	toCol.TaskNotes = append(toCol.TaskNotes, tn)

	return fs.WriteBoard(*board)
}

// ReorderTaskNote swaps a task note's position within a column
func ReorderTaskNote(board *models.Board, colIndex, fromIndex, toIndex int) error {
	if colIndex < 0 || colIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	col := &board.Columns[colIndex]
	if fromIndex < 0 || fromIndex >= len(col.TaskNotes) {
		return fmt.Errorf("invalid source card index")
	}
	if toIndex < 0 || toIndex >= len(col.TaskNotes) {
		return fmt.Errorf("invalid destination card index")
	}

	col.TaskNotes[fromIndex], col.TaskNotes[toIndex] = col.TaskNotes[toIndex], col.TaskNotes[fromIndex]

	return fs.WriteBoard(*board)
}

// ReloadTaskNote reloads a task note from disk
func ReloadTaskNote(boardPath, filename string) (models.TaskNote, error) {
	cardPath := filepath.Join(boardPath, "cards", filename)
	return fs.ReadTaskNote(cardPath)
}

// CollectAllTags gathers all unique tags across all task notes in a board
func CollectAllTags(board *models.Board) []string {
	tagSet := make(map[string]bool)
	for _, col := range board.Columns {
		for _, tn := range col.TaskNotes {
			for _, tag := range tn.Tags {
				tagSet[tag] = true
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}

	sortStrings(tags)
	return tags
}

func sortStrings(s []string) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// CollectAllProjects gathers all unique projects across all task notes in a board
func CollectAllProjects(board *models.Board) []string {
	projectSet := make(map[string]bool)
	for _, col := range board.Columns {
		for _, tn := range col.TaskNotes {
			for _, project := range tn.Projects {
				projectSet[project] = true
			}
		}
	}

	projects := make([]string, 0, len(projectSet))
	for project := range projectSet {
		projects = append(projects, project)
	}

	sortStrings(projects)
	return projects
}

// UpdateTaskNoteTags updates a task note's tags and persists to disk
func UpdateTaskNoteTags(board *models.Board, columnIndex, cardIndex int, tags []string) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.TaskNotes) {
		return fmt.Errorf("invalid card index")
	}

	tn := &column.TaskNotes[cardIndex]
	tn.Tags = tags

	cardPath := filepath.Join(board.Path, "cards", tn.Filename)
	return fs.WriteTaskNote(*tn, cardPath)
}

// UpdateTaskNoteProjects updates a task note's projects and persists to disk
func UpdateTaskNoteProjects(board *models.Board, columnIndex, cardIndex int, projects []string) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.TaskNotes) {
		return fmt.Errorf("invalid card index")
	}

	tn := &column.TaskNotes[cardIndex]
	tn.Projects = projects

	cardPath := filepath.Join(board.Path, "cards", tn.Filename)
	return fs.WriteTaskNote(*tn, cardPath)
}

// UpdateTaskNoteURLs updates a task note's URLs and persists to disk
func UpdateTaskNoteURLs(board *models.Board, columnIndex, cardIndex int, urls []models.TaskNoteURL) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.TaskNotes) {
		return fmt.Errorf("invalid card index")
	}

	tn := &column.TaskNotes[cardIndex]
	tn.URLs = urls

	cardPath := filepath.Join(board.Path, "cards", tn.Filename)
	return fs.WriteTaskNote(*tn, cardPath)
}

// UpdateTaskNoteDueDate updates a task note's due date and persists to disk
func UpdateTaskNoteDueDate(board *models.Board, columnIndex, cardIndex int, dueDate *time.Time) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.TaskNotes) {
		return fmt.Errorf("invalid card index")
	}

	tn := &column.TaskNotes[cardIndex]
	tn.DueDate = dueDate

	cardPath := filepath.Join(board.Path, "cards", tn.Filename)
	return fs.WriteTaskNote(*tn, cardPath)
}

// UpdateTaskNoteScheduledDate updates a task note's scheduled date and persists to disk
func UpdateTaskNoteScheduledDate(board *models.Board, columnIndex, cardIndex int, scheduledDate *time.Time) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.TaskNotes) {
		return fmt.Errorf("invalid card index")
	}

	tn := &column.TaskNotes[cardIndex]
	tn.ScheduledDate = scheduledDate

	cardPath := filepath.Join(board.Path, "cards", tn.Filename)
	return fs.WriteTaskNote(*tn, cardPath)
}

// UpdateTaskNotePriority updates a task note's priority and persists to disk
func UpdateTaskNotePriority(board *models.Board, columnIndex, cardIndex, priority int) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.TaskNotes) {
		return fmt.Errorf("invalid card index")
	}

	tn := &column.TaskNotes[cardIndex]
	tn.Priority = priority

	cardPath := filepath.Join(board.Path, "cards", tn.Filename)
	return fs.WriteTaskNote(*tn, cardPath)
}

// UpdateTaskNoteTmuxSession updates a task note's tmux session link and persists to disk
func UpdateTaskNoteTmuxSession(board *models.Board, columnIndex, cardIndex int, session string) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.TaskNotes) {
		return fmt.Errorf("invalid card index")
	}

	tn := &column.TaskNotes[cardIndex]
	tn.TmuxSession = session

	cardPath := filepath.Join(board.Path, "cards", tn.Filename)
	return fs.WriteTaskNote(*tn, cardPath)
}

// TaskPriorityToTaskNotePriority maps a todo.txt priority rune (A-F) to a task note priority int (1-6).
// Returns 0 for no priority.
func TaskPriorityToTaskNotePriority(p rune) int {
	if p >= 'A' && p <= 'F' {
		return int(p-'A') + 1
	}
	return 0
}

// CreateTaskNoteFromTask creates a new task note in the first column of a board from task data.
func CreateTaskNoteFromTask(board *models.Board, title string, projects []string, tags []string, dueDate *time.Time, scheduledDate *time.Time, priority int) (models.TaskNote, error) {
	if len(board.Columns) == 0 {
		return models.TaskNote{}, fmt.Errorf("board has no columns")
	}

	cardsDir := filepath.Join(board.Path, "cards")
	if err := os.MkdirAll(cardsDir, 0755); err != nil {
		return models.TaskNote{}, err
	}

	baseFilename := ToSnakeCase(title)
	filename := UniqueFilename(baseFilename, cardsDir, "")

	tn := models.TaskNote{
		Filename:      filename,
		Title:         title,
		Tags:          tags,
		Projects:      projects,
		Content:       "# " + title + "\n",
		DueDate:       dueDate,
		ScheduledDate: scheduledDate,
		Priority:      priority,
	}

	cardPath := filepath.Join(cardsDir, filename)
	if err := fs.WriteNewTaskNote(tn, cardPath); err != nil {
		return models.TaskNote{}, err
	}

	board.Columns[0].TaskNotes = append(board.Columns[0].TaskNotes, tn)
	if err := fs.WriteBoard(*board); err != nil {
		return models.TaskNote{}, err
	}

	return tn, nil
}

// EnsureBoardProjects ensures that a task note has all the given board projects
// in its frontmatter. Returns nil without writing if all are already present.
func EnsureBoardProjects(board *models.Board, colIndex, cardIndex int, boardProjects []string) error {
	if len(boardProjects) == 0 {
		return nil
	}
	if colIndex < 0 || colIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}
	col := &board.Columns[colIndex]
	if cardIndex < 0 || cardIndex >= len(col.TaskNotes) {
		return fmt.Errorf("invalid card index")
	}

	tn := &col.TaskNotes[cardIndex]
	missing := false
	for _, bp := range boardProjects {
		if !hasProject(tn.Projects, bp) {
			missing = true
			break
		}
	}
	if !missing {
		return nil
	}

	for _, bp := range boardProjects {
		if !hasProject(tn.Projects, bp) {
			tn.Projects = append(tn.Projects, bp)
		}
	}

	cardPath := filepath.Join(board.Path, "cards", tn.Filename)
	return fs.WriteTaskNote(*tn, cardPath)
}

// MoveTaskNoteToBoard moves a task note from one board to another.
// Done-column task notes land in the target's done column (or first column if none);
// all other task notes land in the target's first column.
// Any boardProjects not already on the task note are added to its projects frontmatter.
func MoveTaskNoteToBoard(srcBoard *models.Board, colIndex, cardIndex int, dstBoard *models.Board, boardProjects []string) error {
	if colIndex < 0 || colIndex >= len(srcBoard.Columns) {
		return fmt.Errorf("invalid source column index")
	}
	srcCol := &srcBoard.Columns[colIndex]
	if cardIndex < 0 || cardIndex >= len(srcCol.TaskNotes) {
		return fmt.Errorf("invalid source card index")
	}
	if len(dstBoard.Columns) == 0 {
		return fmt.Errorf("target board has no columns")
	}

	tn := srcCol.TaskNotes[cardIndex]

	// Link source board projects if not already present
	for _, bp := range boardProjects {
		if !hasProject(tn.Projects, bp) {
			tn.Projects = append(tn.Projects, bp)
		}
	}

	// Determine target column index
	dstColIdx := 0
	if srcBoard.IsDoneColumn(srcCol.Name) {
		for i, c := range dstBoard.Columns {
			if dstBoard.IsDoneColumn(c.Name) {
				dstColIdx = i
				break
			}
		}
	}

	// Write to destination (destination-first for crash safety)
	dstCardsDir := filepath.Join(dstBoard.Path, "cards")
	if err := os.MkdirAll(dstCardsDir, 0755); err != nil {
		return fmt.Errorf("create target cards dir: %w", err)
	}

	baseFilename := ToSnakeCase(tn.Title)
	newFilename := UniqueFilename(baseFilename, dstCardsDir, "")
	origFilename := tn.Filename
	tn.Filename = newFilename

	cardPath := filepath.Join(dstCardsDir, newFilename)
	if err := fs.WriteTaskNote(tn, cardPath); err != nil {
		return fmt.Errorf("write target card: %w", err)
	}

	dstBoard.Columns[dstColIdx].TaskNotes = append(dstBoard.Columns[dstColIdx].TaskNotes, tn)
	if err := fs.WriteBoard(*dstBoard); err != nil {
		return fmt.Errorf("write target board: %w", err)
	}

	// Remove from source
	srcCardsDir := filepath.Join(srcBoard.Path, "cards")
	srcCol.TaskNotes = append(srcCol.TaskNotes[:cardIndex], srcCol.TaskNotes[cardIndex+1:]...)
	if err := os.Remove(filepath.Join(srcCardsDir, origFilename)); err != nil && !os.IsNotExist(err) {
		// Non-fatal: card is already in target
	}
	if err := fs.WriteBoard(*srcBoard); err != nil {
		return fmt.Errorf("write source board: %w", err)
	}

	return nil
}

func hasProject(projects []string, name string) bool {
	for _, p := range projects {
		if strings.EqualFold(p, name) {
			return true
		}
	}
	return false
}

// ToggleTaskNoteArchive moves a card between the board's cards/ dir and archive/boards/<name>/cards/.
func ToggleTaskNoteArchive(board *models.Board, columnIndex, cardIndex int) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.TaskNotes) {
		return fmt.Errorf("invalid card index")
	}

	tn := column.TaskNotes[cardIndex]

	// board.Path is always the active board path (<ws>/boards/<name>)
	boardName := filepath.Base(board.Path)
	boardsDir := filepath.Dir(board.Path)
	wsRoot := filepath.Dir(boardsDir)

	activeCardPath := filepath.Join(board.Path, "cards", tn.Filename)
	archiveCardsDir := filepath.Join(wsRoot, "archive", "boards", boardName, "cards")
	archiveCardPath := filepath.Join(archiveCardsDir, tn.Filename)

	if tn.Archived {
		// Unarchive: move from archive/boards/<name>/cards/ to boards/<name>/cards/
		targetColIdx := board.GetColumnIndex(tn.Column)
		if targetColIdx == -1 {
			targetColIdx = 0
		}

		if err := os.MkdirAll(filepath.Join(board.Path, "cards"), 0755); err != nil {
			return err
		}
		if err := os.Rename(archiveCardPath, activeCardPath); err != nil {
			return err
		}

		tn.Archived = false
		tn.Column = ""
		if err := fs.WriteTaskNote(tn, activeCardPath); err != nil {
			return err
		}

		// Remove from current column and add to target column
		column.TaskNotes = append(column.TaskNotes[:cardIndex], column.TaskNotes[cardIndex+1:]...)
		board.Columns[targetColIdx].TaskNotes = append(board.Columns[targetColIdx].TaskNotes, tn)

		return fs.WriteBoard(*board)
	}

	// Archive: move from boards/<name>/cards/ to archive/boards/<name>/cards/
	tn.Column = board.Columns[columnIndex].Name
	tn.Archived = true

	if err := os.MkdirAll(archiveCardsDir, 0755); err != nil {
		return err
	}
	if err := fs.WriteTaskNote(tn, archiveCardPath); err != nil {
		return err
	}

	column.TaskNotes = append(column.TaskNotes[:cardIndex], column.TaskNotes[cardIndex+1:]...)
	if err := fs.WriteBoard(*board); err != nil {
		return err
	}

	return os.Remove(activeCardPath)
}

// OpenURL opens a URL in the default browser
func OpenURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}
