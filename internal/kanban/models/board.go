package models

import "strings"

// Board represents a kanban board with its columns and cards
type Board struct {
	Name    string   // Board name from H1 in board.md
	Path    string   // Absolute path to board directory
	Columns []Column // List of columns
}

// GetColumn returns a pointer to the column with the given name
func (b *Board) GetColumn(name string) *Column {
	for i := range b.Columns {
		if b.Columns[i].Name == name {
			return &b.Columns[i]
		}
	}
	return nil
}

// GetColumnIndex returns the index of the column with the given name
func (b *Board) GetColumnIndex(name string) int {
	for i := range b.Columns {
		if b.Columns[i].Name == name {
			return i
		}
	}
	return -1
}

// IsLastColumn checks if index is the last column
func (b *Board) IsLastColumn(index int) bool {
	return index == len(b.Columns)-1
}

// IsDoneColumn checks if column name is "Done" (case-insensitive)
func (b *Board) IsDoneColumn(name string) bool {
	return strings.EqualFold(name, "done")
}

// CanDeleteColumn returns (bool, errorMessage)
func (b *Board) CanDeleteColumn(index int) (bool, string) {
	if index < 0 || index >= len(b.Columns) {
		return false, "invalid column index"
	}

	if b.IsDoneColumn(b.Columns[index].Name) {
		return false, "cannot delete Done column"
	}

	if len(b.Columns) <= 1 {
		return false, "cannot delete the last column"
	}

	return true, ""
}

// CanRenameColumn returns (bool, errorMessage)
func (b *Board) CanRenameColumn(index int) (bool, string) {
	if index < 0 || index >= len(b.Columns) {
		return false, "invalid column index"
	}

	if b.IsDoneColumn(b.Columns[index].Name) {
		return false, "cannot rename Done column"
	}

	return true, ""
}
