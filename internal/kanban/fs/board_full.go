package fs

import (
	"os"
	"path/filepath"
	"strings"
	"wydo/internal/kanban/models"
)

// ReadBoardFull reads a board .txt file and populates runtime Columns by
// scanning the workspace tasks directories. This is the function to use when
// you need a fully rendered board with cards, not just its status list.
// boardPath = path to the .txt file; wsRoot = workspace root directory.
func ReadBoardFull(boardPath, wsRoot string) (models.Board, error) {
	board, err := ReadBoard(boardPath)
	if err != nil {
		return models.Board{}, err
	}
	board.WSRoot = wsRoot

	var allTaskNotes []models.TaskNote
	scanDir(filepath.Join(wsRoot, "tasks"), false, &allTaskNotes)
	scanDir(filepath.Join(wsRoot, "archive", "tasks"), true, &allTaskNotes)

	BuildBoardColumns(&board, allTaskNotes)
	return board, nil
}

// BuildBoardColumns populates board.Columns from a flat list of task notes.
// Columns are built from board.Statuses plus an implicit "Done" column.
// Only task notes belonging to this board (by name) are included.
func BuildBoardColumns(board *models.Board, allTaskNotes []models.TaskNote) {
	// Build ordered column list: statuses + Done
	statusList := append(append([]string{}, board.Statuses...), "Done")

	board.Columns = make([]models.Column, len(statusList))
	for i, s := range statusList {
		board.Columns[i] = models.Column{Name: s, TaskNotes: []models.TaskNote{}}
	}

	// Bucket each task note into its column
	colIndex := make(map[string]int, len(statusList))
	for i, s := range statusList {
		colIndex[strings.ToLower(s)] = i
	}

	for _, tn := range allTaskNotes {
		if !strings.EqualFold(tn.Board, board.Name) {
			continue
		}
		idx, ok := colIndex[strings.ToLower(tn.Status)]
		if !ok {
			// Unknown status — skip (will be reported as validation issue)
			continue
		}
		board.Columns[idx].TaskNotes = append(board.Columns[idx].TaskNotes, tn)
	}
}

// scanDir scans a directory for .md task note files and appends them to dest.
func scanDir(dir string, archived bool, dest *[]models.TaskNote) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		tn, err := ReadTaskNote(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		tn.Archived = archived
		*dest = append(*dest, tn)
	}
}
