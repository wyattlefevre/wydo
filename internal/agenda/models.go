package agenda

import (
	"time"

	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/notes"
	"wydo/internal/tasks/data"
)

// ItemSource identifies whether an agenda item comes from a task or a card
type ItemSource int

const (
	SourceTask ItemSource = iota
	SourceCard
	SourceNote
)

// DateReason identifies why an item appears on a given date
type DateReason int

const (
	ReasonDue DateReason = iota
	ReasonScheduled
	ReasonNote
)

func (r DateReason) String() string {
	switch r {
	case ReasonDue:
		return "due"
	case ReasonScheduled:
		return "sched"
	case ReasonNote:
		return "note"
	default:
		return ""
	}
}

// AgendaItem wraps either a task, card, or note with its date context
type AgendaItem struct {
	Source     ItemSource
	Reason     DateReason
	Date       time.Time
	Task       *data.Task
	Card       *kanbanmodels.Card
	Note       *notes.Note
	BoardName  string
	BoardPath  string
	ColumnName string
	ColIndex   int
	CardIndex  int
	Completed  bool
}

// DateBucket groups agenda items by date
type DateBucket struct {
	Date           time.Time
	Tasks          []AgendaItem
	Cards          []AgendaItem
	Notes          []AgendaItem
	CompletedTasks []AgendaItem
	CompletedCards []AgendaItem
}

// AllItems returns all items in the bucket (tasks first, then cards, then notes)
func (b DateBucket) AllItems() []AgendaItem {
	items := make([]AgendaItem, 0, len(b.Tasks)+len(b.Cards)+len(b.Notes))
	items = append(items, b.Tasks...)
	items = append(items, b.Cards...)
	items = append(items, b.Notes...)
	return items
}

// AllCompletedItems returns all completed items in the bucket (tasks first, then cards)
func (b DateBucket) AllCompletedItems() []AgendaItem {
	items := make([]AgendaItem, 0, len(b.CompletedTasks)+len(b.CompletedCards))
	items = append(items, b.CompletedTasks...)
	items = append(items, b.CompletedCards...)
	return items
}

// TotalCount returns the total number of items in the bucket (including completed)
func (b DateBucket) TotalCount() int {
	return len(b.Tasks) + len(b.Cards) + len(b.Notes) + len(b.CompletedTasks) + len(b.CompletedCards)
}

// DateRange represents a range of dates for querying
type DateRange struct {
	Start time.Time
	End   time.Time
}
