package operations

import (
	"fmt"
	"strings"
	"wydo/internal/kanban/fs"
	"wydo/internal/kanban/models"
)

// RenameColumn renames a column (blocks Done column)
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

	board.Columns[columnIndex].Name = validatedName

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
		Name:  validatedName,
		Cards: []models.Card{},
	}

	board.Columns = append(board.Columns[:position], append([]models.Column{newColumn}, board.Columns[position:]...)...)

	return fs.WriteBoard(*board)
}

// DeleteColumn removes a column and auto-migrates cards to adjacent column
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

	if len(column.Cards) > 0 {
		targetIndex := columnIndex - 1
		if targetIndex < 0 {
			targetIndex = columnIndex + 1
		}

		if targetIndex >= 0 && targetIndex < len(board.Columns) {
			board.Columns[targetIndex].Cards = append(
				board.Columns[targetIndex].Cards,
				column.Cards...,
			)
		}
	}

	board.Columns = append(board.Columns[:columnIndex], board.Columns[columnIndex+1:]...)

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

	return fs.WriteBoard(*board)
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
