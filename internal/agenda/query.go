package agenda

import (
	"sort"
	"strings"
	"time"

	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/notes"
	"wydo/internal/tasks/data"
	"wydo/internal/tasks/service"
)

// DayRange returns a DateRange for a single day
func DayRange(date time.Time) DateRange {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 0, 1).Add(-time.Nanosecond)
	return DateRange{Start: start, End: end}
}

// WeekRange returns a DateRange for the week containing the given date (Mon-Sun)
func WeekRange(date time.Time) DateRange {
	// Find Monday of this week
	weekday := date.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	monday := date.AddDate(0, 0, -int(weekday-time.Monday))
	start := time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 0, 7).Add(-time.Nanosecond)
	return DateRange{Start: start, End: end}
}

// MonthRange returns a DateRange for the entire month containing the given date
func MonthRange(date time.Time) DateRange {
	start := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0).Add(-time.Nanosecond)
	return DateRange{Start: start, End: end}
}

// QueryAgenda scans task, card, and note data sources for items within the date range
func QueryAgenda(taskSvc service.TaskService, boards []kanbanmodels.Board, allNotes []notes.Note, dateRange DateRange) []DateBucket {
	bucketMap := make(map[string]*DateBucket)

	// Scan pending tasks
	if taskSvc != nil {
		if tasks, err := taskSvc.ListPending(); err == nil {
			for i := range tasks {
				task := &tasks[i]
				addTaskItems(task, false, dateRange, bucketMap)
			}
		}
		// Scan completed tasks
		if tasks, err := taskSvc.ListDone(); err == nil {
			for i := range tasks {
				task := &tasks[i]
				addTaskItems(task, true, dateRange, bucketMap)
			}
		}
	}

	// Scan cards from boards
	for _, board := range boards {
		if board.Archived {
			continue
		}
		for colIdx, col := range board.Columns {
			isDone := strings.EqualFold(col.Name, "done")
			for cardIdx := range col.Cards {
				card := &col.Cards[cardIdx]
				if card.Archived {
					continue
				}
				addCardItems(card, board.Name, board.Path, col.Name, colIdx, cardIdx, isDone, dateRange, bucketMap)
			}
		}
	}

	// Scan notes
	for i := range allNotes {
		note := &allNotes[i]
		addNoteItems(note, dateRange, bucketMap)
	}

	// Convert map to sorted slice
	buckets := make([]DateBucket, 0, len(bucketMap))
	for _, bucket := range bucketMap {
		buckets = append(buckets, *bucket)
	}

	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Date.Before(buckets[j].Date)
	})

	return buckets
}

// QueryOverdueItems returns tasks and cards with due or scheduled dates strictly before the cutoff date.
// Notes are excluded. If a task/card has both an overdue due date and an overdue scheduled date,
// it appears once using the due date. Results are sorted by date ascending (oldest first).
func QueryOverdueItems(taskSvc service.TaskService, boards []kanbanmodels.Board, cutoff time.Time) []AgendaItem {
	cutoffDay := time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, time.Local)
	var items []AgendaItem

	// Scan tasks
	if taskSvc != nil {
		if tasks, err := taskSvc.ListPending(); err == nil {
			for i := range tasks {
				task := &tasks[i]
				added := false
				if dueStr := task.GetDueDate(); dueStr != "" {
					if dueDate, err := time.Parse("2006-01-02", dueStr); err == nil {
						d := time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, time.Local)
						if d.Before(cutoffDay) {
							items = append(items, AgendaItem{
								Source: SourceTask,
								Reason: ReasonDue,
								Date:   dueDate,
								Task:   task,
							})
							added = true
						}
					}
				}
				if !added {
					if schedStr := task.GetScheduledDate(); schedStr != "" {
						if schedDate, err := time.Parse("2006-01-02", schedStr); err == nil {
							d := time.Date(schedDate.Year(), schedDate.Month(), schedDate.Day(), 0, 0, 0, 0, time.Local)
							if d.Before(cutoffDay) {
								items = append(items, AgendaItem{
									Source: SourceTask,
									Reason: ReasonScheduled,
									Date:   schedDate,
									Task:   task,
								})
							}
						}
					}
				}
			}
		}
	}

	// Scan cards from boards
	for _, board := range boards {
		if board.Archived {
			continue
		}
		for colIdx, col := range board.Columns {
			if strings.EqualFold(col.Name, "done") {
				continue
			}
			for cardIdx := range col.Cards {
				card := &col.Cards[cardIdx]
				if card.Archived {
					continue
				}
				added := false
				if card.DueDate != nil {
					dueDate := *card.DueDate
					d := time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, time.Local)
					if d.Before(cutoffDay) {
						items = append(items, AgendaItem{
							Source:     SourceCard,
							Reason:     ReasonDue,
							Date:       dueDate,
							Card:       card,
							BoardName:  board.Name,
							BoardPath:  board.Path,
							ColumnName: col.Name,
							ColIndex:   colIdx,
							CardIndex:  cardIdx,
						})
						added = true
					}
				}
				if !added && card.ScheduledDate != nil {
					schedDate := *card.ScheduledDate
					d := time.Date(schedDate.Year(), schedDate.Month(), schedDate.Day(), 0, 0, 0, 0, time.Local)
					if d.Before(cutoffDay) {
						items = append(items, AgendaItem{
							Source:     SourceCard,
							Reason:     ReasonScheduled,
							Date:       schedDate,
							Card:       card,
							BoardName:  board.Name,
							BoardPath:  board.Path,
							ColumnName: col.Name,
							ColIndex:   colIdx,
							CardIndex:  cardIdx,
						})
					}
				}
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Date.Before(items[j].Date)
	})

	return items
}

func addTaskItems(task *data.Task, completed bool, dateRange DateRange, bucketMap map[string]*DateBucket) {
	// Check due date
	if dueStr := task.GetDueDate(); dueStr != "" {
		if dueDate, err := time.Parse("2006-01-02", dueStr); err == nil {
			if inRange(dueDate, dateRange) {
				bucket := getOrCreateBucket(bucketMap, dueDate)
				item := AgendaItem{
					Source:    SourceTask,
					Reason:    ReasonDue,
					Date:      dueDate,
					Task:      task,
					Completed: completed,
				}
				if completed {
					bucket.CompletedTasks = append(bucket.CompletedTasks, item)
				} else {
					bucket.Tasks = append(bucket.Tasks, item)
				}
			}
		}
	}

	// Check scheduled date (skip if due date is the same day to avoid duplicates)
	if schedStr := task.GetScheduledDate(); schedStr != "" {
		if dueStr := task.GetDueDate(); dueStr == schedStr {
			return
		}
		if schedDate, err := time.Parse("2006-01-02", schedStr); err == nil {
			if inRange(schedDate, dateRange) {
				bucket := getOrCreateBucket(bucketMap, schedDate)
				item := AgendaItem{
					Source:    SourceTask,
					Reason:    ReasonScheduled,
					Date:      schedDate,
					Task:      task,
					Completed: completed,
				}
				if completed {
					bucket.CompletedTasks = append(bucket.CompletedTasks, item)
				} else {
					bucket.Tasks = append(bucket.Tasks, item)
				}
			}
		}
	}
}

func addCardItems(card *kanbanmodels.Card, boardName, boardPath, columnName string, colIdx, cardIdx int, completed bool, dateRange DateRange, bucketMap map[string]*DateBucket) {
	// Check due date
	if card.DueDate != nil {
		dueDate := *card.DueDate
		if inRange(dueDate, dateRange) {
			bucket := getOrCreateBucket(bucketMap, dueDate)
			item := AgendaItem{
				Source:     SourceCard,
				Reason:     ReasonDue,
				Date:       dueDate,
				Card:       card,
				BoardName:  boardName,
				BoardPath:  boardPath,
				ColumnName: columnName,
				ColIndex:   colIdx,
				CardIndex:  cardIdx,
				Completed:  completed,
			}
			if completed {
				bucket.CompletedCards = append(bucket.CompletedCards, item)
			} else {
				bucket.Cards = append(bucket.Cards, item)
			}
		}
	}

	// Check scheduled date (skip if due date is the same day to avoid duplicates)
	if card.ScheduledDate != nil {
		schedDate := *card.ScheduledDate
		if card.DueDate != nil && card.DueDate.Format("2006-01-02") == schedDate.Format("2006-01-02") {
			return
		}
		if inRange(schedDate, dateRange) {
			bucket := getOrCreateBucket(bucketMap, schedDate)
			item := AgendaItem{
				Source:     SourceCard,
				Reason:     ReasonScheduled,
				Date:       schedDate,
				Card:       card,
				BoardName:  boardName,
				BoardPath:  boardPath,
				ColumnName: columnName,
				ColIndex:   colIdx,
				CardIndex:  cardIdx,
				Completed:  completed,
			}
			if completed {
				bucket.CompletedCards = append(bucket.CompletedCards, item)
			} else {
				bucket.Cards = append(bucket.Cards, item)
			}
		}
	}
}

func addNoteItems(note *notes.Note, dateRange DateRange, bucketMap map[string]*DateBucket) {
	noteDate := time.Date(note.Date.Year(), note.Date.Month(), note.Date.Day(), 0, 0, 0, 0, time.Local)
	if inRange(noteDate, dateRange) {
		bucket := getOrCreateBucket(bucketMap, noteDate)
		bucket.Notes = append(bucket.Notes, AgendaItem{
			Source: SourceNote,
			Reason: ReasonNote,
			Date:   noteDate,
			Note:   note,
		})
	}
}

func inRange(date time.Time, dateRange DateRange) bool {
	d := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
	s := time.Date(dateRange.Start.Year(), dateRange.Start.Month(), dateRange.Start.Day(), 0, 0, 0, 0, time.Local)
	e := time.Date(dateRange.End.Year(), dateRange.End.Month(), dateRange.End.Day(), 0, 0, 0, 0, time.Local)
	return !d.Before(s) && !d.After(e)
}

func getOrCreateBucket(bucketMap map[string]*DateBucket, date time.Time) *DateBucket {
	key := date.Format("2006-01-02")
	if bucket, ok := bucketMap[key]; ok {
		return bucket
	}
	bucket := &DateBucket{
		Date: time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local),
	}
	bucketMap[key] = bucket
	return bucket
}
