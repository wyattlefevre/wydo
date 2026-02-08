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

	// Scan tasks
	if taskSvc != nil {
		if tasks, err := taskSvc.ListPending(); err == nil {
			for i := range tasks {
				task := &tasks[i]
				addTaskItems(task, dateRange, bucketMap)
			}
		}
	}

	// Scan cards from boards
	for _, board := range boards {
		for colIdx, col := range board.Columns {
			// Skip Done columns
			if strings.EqualFold(col.Name, "done") {
				continue
			}
			for cardIdx := range col.Cards {
				card := &col.Cards[cardIdx]
				addCardItems(card, board.Name, board.Path, col.Name, colIdx, cardIdx, dateRange, bucketMap)
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

func addTaskItems(task *data.Task, dateRange DateRange, bucketMap map[string]*DateBucket) {
	// Check due date
	if dueStr := task.GetDueDate(); dueStr != "" {
		if dueDate, err := time.Parse("2006-01-02", dueStr); err == nil {
			if inRange(dueDate, dateRange) {
				bucket := getOrCreateBucket(bucketMap, dueDate)
				bucket.Tasks = append(bucket.Tasks, AgendaItem{
					Source: SourceTask,
					Reason: ReasonDue,
					Date:   dueDate,
					Task:   task,
				})
			}
		}
	}

	// Check scheduled date
	if schedStr := task.GetScheduledDate(); schedStr != "" {
		if schedDate, err := time.Parse("2006-01-02", schedStr); err == nil {
			if inRange(schedDate, dateRange) {
				bucket := getOrCreateBucket(bucketMap, schedDate)
				bucket.Tasks = append(bucket.Tasks, AgendaItem{
					Source: SourceTask,
					Reason: ReasonScheduled,
					Date:   schedDate,
					Task:   task,
				})
			}
		}
	}
}

func addCardItems(card *kanbanmodels.Card, boardName, boardPath, columnName string, colIdx, cardIdx int, dateRange DateRange, bucketMap map[string]*DateBucket) {
	// Check due date
	if card.DueDate != nil {
		dueDate := *card.DueDate
		if inRange(dueDate, dateRange) {
			bucket := getOrCreateBucket(bucketMap, dueDate)
			bucket.Cards = append(bucket.Cards, AgendaItem{
				Source:     SourceCard,
				Reason:     ReasonDue,
				Date:       dueDate,
				Card:       card,
				BoardName:  boardName,
				BoardPath:  boardPath,
				ColumnName: columnName,
				ColIndex:   colIdx,
				CardIndex:  cardIdx,
			})
		}
	}

	// Check scheduled date
	if card.ScheduledDate != nil {
		schedDate := *card.ScheduledDate
		if inRange(schedDate, dateRange) {
			bucket := getOrCreateBucket(bucketMap, schedDate)
			bucket.Cards = append(bucket.Cards, AgendaItem{
				Source:     SourceCard,
				Reason:     ReasonScheduled,
				Date:       schedDate,
				Card:       card,
				BoardName:  boardName,
				BoardPath:  boardPath,
				ColumnName: columnName,
				ColIndex:   colIdx,
				CardIndex:  cardIdx,
			})
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
