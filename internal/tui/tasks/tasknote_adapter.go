package tasks

import (
	"fmt"
	"wydo/internal/kanban/models"
	"wydo/internal/tasks/data"
)

// TaskNoteToTask converts a TaskNote from a board column into a data.Task
// suitable for display in the Tasks view.
func TaskNoteToTask(tn models.TaskNote, board models.Board, col models.Column) data.Task {
	t := data.Task{
		ID:         fmt.Sprintf("tn:%s:%s", board.Name, tn.Filename),
		Name:       tn.Title,
		Projects:   tn.Projects,
		IsTaskNote: true,
		BoardName:  board.Name,
		ColumnName: col.Name,
		File:       board.Path,
		Properties:     make(map[string]string),
	}

	// Priority: task note stores 0=none, 1=A ... 6=F
	if tn.Priority > 0 && tn.Priority <= 6 {
		t.Priority = data.Priority('A' + rune(tn.Priority-1))
	}

	// Done: task note is done when DateCompleted is set
	if tn.DateCompleted != nil {
		t.Done = true
		t.CompletionDate = tn.DateCompleted.Format("2006-01-02")
	}

	// Due / Scheduled
	if tn.DueDate != nil {
		t.Properties["due"] = tn.DueDate.Format("2006-01-02")
	}
	if tn.ScheduledDate != nil {
		t.Properties["scheduled"] = tn.ScheduledDate.Format("2006-01-02")
	}

	// URL (first URL if present)
	if len(tn.URLs) > 0 {
		t.Properties["url"] = tn.URLs[0].URL
	}

	return t
}
