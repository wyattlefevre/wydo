package operations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"wydo/internal/kanban/fs"
	"wydo/internal/kanban/models"
)

// syncStatuses updates board.Statuses from board.Columns (excluding Done).
func syncStatuses(board *models.Board) {
	board.Statuses = make([]string, 0, len(board.Columns))
	for _, col := range board.Columns {
		if !board.IsDoneColumn(col.Name) {
			board.Statuses = append(board.Statuses, col.Name)
		}
	}
}

// RenameColumn renames a column and updates all affected task note status fields.
func RenameColumn(board *models.Board, columnIndex int, newName string) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	validatedName, err := ValidateColumnName(newName)
	if err != nil {
		return err
	}

	if board.IsDoneColumn(board.Columns[columnIndex].Name) {
		return fmt.Errorf("cannot rename the Done column")
	}

	for i, col := range board.Columns {
		if i != columnIndex && strings.EqualFold(col.Name, validatedName) {
			return fmt.Errorf("column name already exists")
		}
	}

	oldName := board.Columns[columnIndex].Name
	board.Columns[columnIndex].Name = validatedName

	// Update task note status fields for this board
	if board.WSRoot != "" {
		updateStatusInTaskNotes(board.WSRoot, board.Name, oldName, validatedName)
	}

	syncStatuses(board)
	return fs.WriteBoard(*board)
}

// AddColumn inserts a new column at position (-1 = before Done)
func AddColumn(board *models.Board, name string, position int) error {
	validatedName, err := ValidateColumnName(name)
	if err != nil {
		return err
	}

	for _, col := range board.Columns {
		if strings.EqualFold(col.Name, validatedName) {
			return fmt.Errorf("column name already exists")
		}
	}

	if position == -1 {
		position = len(board.Columns) - 1
		if position < 0 {
			position = 0
		}
	}

	if position < 0 || position > len(board.Columns) {
		return fmt.Errorf("invalid position")
	}

	newColumn := models.Column{
		Name:      validatedName,
		TaskNotes: []models.TaskNote{},
	}

	board.Columns = append(board.Columns[:position], append([]models.Column{newColumn}, board.Columns[position:]...)...)

	syncStatuses(board)
	return fs.WriteBoard(*board)
}

// DeleteColumn removes a column and auto-migrates task notes to adjacent column.
func DeleteColumn(board *models.Board, columnIndex int) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := board.Columns[columnIndex]

	if board.IsDoneColumn(column.Name) {
		return fmt.Errorf("cannot delete the Done column")
	}

	if len(board.Columns) <= 1 {
		return fmt.Errorf("cannot delete the last column")
	}

	// Determine migration target
	targetIndex := columnIndex - 1
	if targetIndex < 0 {
		targetIndex = columnIndex + 1
	}
	targetName := board.Columns[targetIndex].Name

	if len(column.TaskNotes) > 0 {
		if targetIndex >= 0 && targetIndex < len(board.Columns) {
			board.Columns[targetIndex].TaskNotes = append(
				board.Columns[targetIndex].TaskNotes,
				column.TaskNotes...,
			)
		}
		// Update task note status fields
		if board.WSRoot != "" {
			updateStatusInTaskNotes(board.WSRoot, board.Name, column.Name, targetName)
		}
	}

	board.Columns = append(board.Columns[:columnIndex], board.Columns[columnIndex+1:]...)

	syncStatuses(board)
	return fs.WriteBoard(*board)
}

// ReorderColumn moves a column from one position to another (blocks Done movement)
func ReorderColumn(board *models.Board, fromIndex, toIndex int) error {
	if fromIndex < 0 || fromIndex >= len(board.Columns) {
		return fmt.Errorf("invalid source index")
	}
	if toIndex < 0 || toIndex >= len(board.Columns) {
		return fmt.Errorf("invalid destination index")
	}

	if fromIndex == toIndex {
		return nil
	}

	if board.IsDoneColumn(board.Columns[fromIndex].Name) {
		return fmt.Errorf("cannot move the Done column")
	}

	lastIndex := len(board.Columns) - 1
	if board.IsLastColumn(lastIndex) && board.IsDoneColumn(board.Columns[lastIndex].Name) {
		if toIndex >= lastIndex {
			return fmt.Errorf("cannot move column past Done")
		}
	}

	column := board.Columns[fromIndex]
	board.Columns = append(board.Columns[:fromIndex], board.Columns[fromIndex+1:]...)

	if fromIndex < toIndex {
		toIndex--
	}

	board.Columns = append(board.Columns[:toIndex], append([]models.Column{column}, board.Columns[toIndex:]...)...)

	syncStatuses(board)
	return fs.WriteBoard(*board)
}

// updateStatusInTaskNotes scans task notes for a board and renames a status value.
func updateStatusInTaskNotes(wsRoot, boardName, oldStatus, newStatus string) {
	for _, dir := range []string{
		filepath.Join(wsRoot, "tasks"),
		filepath.Join(wsRoot, "archive", "tasks"),
	} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			tnPath := filepath.Join(dir, entry.Name())
			tn, err := fs.ReadTaskNote(tnPath)
			if err != nil {
				continue
			}
			if strings.EqualFold(tn.Board, boardName) && strings.EqualFold(tn.Status, oldStatus) {
				tn.Status = newStatus
				_ = fs.WriteTaskNote(tn, tnPath)
			}
		}
	}
}

// ValidateColumnName checks if column name is valid (trim, length check)
func ValidateColumnName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)

	if trimmed == "" {
		return "", fmt.Errorf("column name cannot be empty")
	}

	if len(trimmed) > 50 {
		return "", fmt.Errorf("column name too long (max 50 characters)")
	}

	return trimmed, nil
}
