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

// taskNotePath returns the absolute path to a task note file.
func taskNotePath(board *models.Board, tn models.TaskNote) string {
	if tn.Archived {
		return filepath.Join(board.WSRoot, "archive", "tasks", tn.Filename)
	}
	return filepath.Join(board.WSRoot, "tasks", tn.Filename)
}

// CreateTaskNote creates a new task note in the specified column
func CreateTaskNote(board *models.Board, columnName string) (models.TaskNote, error) {
	tasksDir := filepath.Join(board.WSRoot, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		return models.TaskNote{}, err
	}

	filename := UniqueFilename("", tasksDir, "")
	cardPath := filepath.Join(tasksDir, filename)

	tn := models.TaskNote{
		Filename: filename,
		Title:    "",
		Tags:     []string{},
		Board:    board.Name,
		Status:   columnName,
		Content:  "# \n",
	}

	if err := fs.WriteNewTaskNote(tn, cardPath); err != nil {
		return models.TaskNote{}, err
	}

	col := board.GetColumn(columnName)
	if col != nil {
		col.TaskNotes = append(col.TaskNotes, tn)
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
	tasksDir := filepath.Join(board.WSRoot, "tasks")
	cardPath := filepath.Join(tasksDir, tn.Filename)

	updatedTN, err := fs.ReadTaskNote(cardPath)
	if err != nil {
		return err
	}

	expectedBase := ToSnakeCase(updatedTN.Title)
	expectedFilename := UniqueFilename(expectedBase, tasksDir, tn.Filename)

	if expectedFilename == tn.Filename {
		return nil
	}

	newPath := filepath.Join(tasksDir, expectedFilename)
	if err := os.Rename(cardPath, newPath); err != nil {
		return err
	}

	tn.Filename = expectedFilename
	return nil
}

// EditTaskNote opens a task note in the user's editor
func EditTaskNote(cardPath string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	cmd := exec.Command(editor, cardPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// DeleteTaskNote removes a task note file and its reference from the board's in-memory columns
func DeleteTaskNote(board *models.Board, columnIndex, cardIndex int) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.TaskNotes) {
		return fmt.Errorf("invalid card index")
	}

	tn := column.TaskNotes[cardIndex]
	cardPath := taskNotePath(board, tn)

	if err := os.Remove(cardPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	column.TaskNotes = append(column.TaskNotes[:cardIndex], column.TaskNotes[cardIndex+1:]...)
	return nil
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

	// Update status to match destination column
	tn.Status = toCol.Name

	// Stamp date_completed when moving to Done
	if board.IsDoneColumn(toCol.Name) {
		now := time.Now()
		tn.DateCompleted = &now
	} else if board.IsDoneColumn(fromCol.Name) {
		tn.DateCompleted = nil
	}

	cardPath := taskNotePath(board, tn)
	if err := fs.WriteTaskNote(tn, cardPath); err != nil {
		return err
	}

	toCol.TaskNotes = append(toCol.TaskNotes, tn)
	return nil
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
	return nil
}

// ReloadTaskNote reloads a task note from disk given its full path
func ReloadTaskNote(cardPath string) (models.TaskNote, error) {
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

// CollectAllProjects gathers all unique projects across all task notes in a board.
// Wikilinks are stripped so callers receive plain project names.
func CollectAllProjects(board *models.Board) []string {
	projectSet := make(map[string]bool)
	for _, col := range board.Columns {
		for _, tn := range col.TaskNotes {
			for _, project := range tn.Projects {
				projectSet[models.StripWikilink(project)] = true
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
	return fs.WriteTaskNote(*tn, taskNotePath(board, *tn))
}

// UpdateTaskNoteProjects updates a task note's projects and persists to disk.
// Each project name is stored as a wikilink (e.g. [[project-name]]).
func UpdateTaskNoteProjects(board *models.Board, columnIndex, cardIndex int, projects []string) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}
	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.TaskNotes) {
		return fmt.Errorf("invalid card index")
	}
	tn := &column.TaskNotes[cardIndex]
	wrapped := make([]string, len(projects))
	for i, p := range projects {
		wrapped[i] = models.WrapWikilink(p)
	}
	tn.Projects = wrapped
	return fs.WriteTaskNote(*tn, taskNotePath(board, *tn))
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
	return fs.WriteTaskNote(*tn, taskNotePath(board, *tn))
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
	return fs.WriteTaskNote(*tn, taskNotePath(board, *tn))
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
	return fs.WriteTaskNote(*tn, taskNotePath(board, *tn))
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
	return fs.WriteTaskNote(*tn, taskNotePath(board, *tn))
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
	return fs.WriteTaskNote(*tn, taskNotePath(board, *tn))
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

	tasksDir := filepath.Join(board.WSRoot, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		return models.TaskNote{}, err
	}

	baseFilename := ToSnakeCase(title)
	filename := UniqueFilename(baseFilename, tasksDir, "")

	// Wrap project names as wikilinks
	wrappedProjects := make([]string, len(projects))
	for i, p := range projects {
		wrappedProjects[i] = models.WrapWikilink(p)
	}

	tn := models.TaskNote{
		Filename:      filename,
		Title:         title,
		Tags:          tags,
		Projects:      wrappedProjects,
		Content:       "# " + title + "\n",
		DueDate:       dueDate,
		ScheduledDate: scheduledDate,
		Priority:      priority,
		Board:         board.Name,
		Status:        board.Columns[0].Name,
	}

	cardPath := filepath.Join(tasksDir, filename)
	if err := fs.WriteNewTaskNote(tn, cardPath); err != nil {
		return models.TaskNote{}, err
	}

	board.Columns[0].TaskNotes = append(board.Columns[0].TaskNotes, tn)
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
			tn.Projects = append(tn.Projects, models.WrapWikilink(bp))
		}
	}

	return fs.WriteTaskNote(*tn, taskNotePath(board, *tn))
}

// MoveTaskNoteToBoard moves a task note from one board to another.
// The file stays in place (tasks/); only the board: and status: frontmatter fields change.
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

	// Add source board projects if missing
	for _, bp := range boardProjects {
		if !hasProject(tn.Projects, bp) {
			tn.Projects = append(tn.Projects, models.WrapWikilink(bp))
		}
	}

	// Determine target status
	dstStatus := dstBoard.Columns[0].Name
	if srcBoard.IsDoneColumn(srcCol.Name) {
		for _, c := range dstBoard.Columns {
			if dstBoard.IsDoneColumn(c.Name) {
				dstStatus = c.Name
				break
			}
		}
	}

	tn.Board = dstBoard.Name
	tn.Status = dstStatus

	// Write updated task note (same file location — file stays in tasks/)
	cardPath := taskNotePath(srcBoard, tn)
	if err := fs.WriteTaskNote(tn, cardPath); err != nil {
		return fmt.Errorf("write task note: %w", err)
	}

	// Update in-memory boards
	srcCol.TaskNotes = append(srcCol.TaskNotes[:cardIndex], srcCol.TaskNotes[cardIndex+1:]...)

	dstColIdx := 0
	for i, c := range dstBoard.Columns {
		if c.Name == dstStatus {
			dstColIdx = i
			break
		}
	}
	dstBoard.Columns[dstColIdx].TaskNotes = append(dstBoard.Columns[dstColIdx].TaskNotes, tn)

	return nil
}

func hasProject(projects []string, name string) bool {
	for _, p := range projects {
		if strings.EqualFold(models.StripWikilink(p), name) {
			return true
		}
	}
	return false
}

// ToggleTaskNoteArchive moves a task note between tasks/ and archive/tasks/
func ToggleTaskNoteArchive(board *models.Board, columnIndex, cardIndex int) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.TaskNotes) {
		return fmt.Errorf("invalid card index")
	}

	tn := column.TaskNotes[cardIndex]
	wsRoot := board.WSRoot

	activeCardPath := filepath.Join(wsRoot, "tasks", tn.Filename)
	archiveDir := filepath.Join(wsRoot, "archive", "tasks")
	archiveCardPath := filepath.Join(archiveDir, tn.Filename)

	if tn.Archived {
		// Unarchive: move from archive/tasks/ to tasks/
		if err := os.MkdirAll(filepath.Join(wsRoot, "tasks"), 0755); err != nil {
			return err
		}
		if err := os.Rename(archiveCardPath, activeCardPath); err != nil {
			return err
		}
		tn.Archived = false
		if err := fs.WriteTaskNote(tn, activeCardPath); err != nil {
			return err
		}
		// Remove from current (archived) column position, find the correct column by status
		column.TaskNotes = append(column.TaskNotes[:cardIndex], column.TaskNotes[cardIndex+1:]...)
		targetColIdx := board.GetColumnIndex(tn.Status)
		if targetColIdx == -1 {
			targetColIdx = 0
		}
		if len(board.Columns) > 0 {
			board.Columns[targetColIdx].TaskNotes = append(board.Columns[targetColIdx].TaskNotes, tn)
		}
	} else {
		// Archive: move from tasks/ to archive/tasks/
		if err := os.MkdirAll(archiveDir, 0755); err != nil {
			return err
		}
		tn.Archived = true
		if err := os.Rename(activeCardPath, archiveCardPath); err != nil {
			return err
		}
		if err := fs.WriteTaskNote(tn, archiveCardPath); err != nil {
			return err
		}
		column.TaskNotes = append(column.TaskNotes[:cardIndex], column.TaskNotes[cardIndex+1:]...)
	}

	return nil
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
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}
