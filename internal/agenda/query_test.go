package agenda

import (
	"testing"
	"time"

	kanbanmodels "wydo/internal/kanban/models"
	"wydo/internal/tasks/data"
)

// mockTaskService implements service.TaskService for testing
type mockTaskService struct {
	tasks []data.Task
}

func (m *mockTaskService) List() ([]data.Task, error)                         { return m.tasks, nil }
func (m *mockTaskService) ListByProject(string) ([]data.Task, error)          { return nil, nil }
func (m *mockTaskService) ListByContext(string) ([]data.Task, error)          { return nil, nil }
func (m *mockTaskService) ListPending() ([]data.Task, error) {
	var pending []data.Task
	for _, t := range m.tasks {
		if !t.Done {
			pending = append(pending, t)
		}
	}
	return pending, nil
}
func (m *mockTaskService) ListDone() ([]data.Task, error) {
	var done []data.Task
	for _, t := range m.tasks {
		if t.Done {
			done = append(done, t)
		}
	}
	return done, nil
}
func (m *mockTaskService) Get(string) (*data.Task, error)                     { return nil, nil }
func (m *mockTaskService) Add(string) (*data.Task, error)                     { return nil, nil }
func (m *mockTaskService) Update(data.Task) error                             { return nil }
func (m *mockTaskService) Complete(string) error                              { return nil }
func (m *mockTaskService) Delete(string) error                                { return nil }
func (m *mockTaskService) Archive() error                                     { return nil }
func (m *mockTaskService) GetProjects() map[string]data.Project               { return nil }
func (m *mockTaskService) Reload() error                                      { return nil }

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func datePtr(y int, m time.Month, d int) *time.Time {
	t := date(y, m, d)
	return &t
}

func TestDayRange(t *testing.T) {
	dr := DayRange(date(2026, 2, 6))
	if dr.Start.Day() != 6 || dr.Start.Month() != 2 {
		t.Errorf("expected start Feb 6, got %v", dr.Start)
	}
	if dr.End.Day() != 6 || dr.End.Month() != 2 {
		t.Errorf("expected end Feb 6, got %v", dr.End)
	}
}

func TestWeekRange(t *testing.T) {
	// Feb 6, 2026 is a Friday
	dr := WeekRange(date(2026, 2, 6))
	if dr.Start.Weekday() != time.Monday {
		t.Errorf("expected start on Monday, got %v", dr.Start.Weekday())
	}
	if dr.Start.Day() != 2 {
		t.Errorf("expected start Feb 2, got Feb %d", dr.Start.Day())
	}
	if dr.End.Day() != 8 {
		t.Errorf("expected end Feb 8, got Feb %d", dr.End.Day())
	}
}

func TestMonthRange(t *testing.T) {
	dr := MonthRange(date(2026, 2, 15))
	if dr.Start.Day() != 1 {
		t.Errorf("expected start Feb 1, got %v", dr.Start)
	}
	if dr.End.Month() != 2 || dr.End.Day() != 28 {
		t.Errorf("expected end Feb 28, got %v", dr.End)
	}
}

func TestQueryAgenda_TaskDueDate(t *testing.T) {
	svc := &mockTaskService{
		tasks: []data.Task{
			{
				ID:   "t1",
				Name: "Buy groceries",
				Tags: map[string]string{"due": "2026-02-06"},
			},
			{
				ID:   "t2",
				Name: "No date task",
				Tags: map[string]string{},
			},
		},
	}

	buckets := QueryAgenda(svc, nil, nil, DayRange(date(2026, 2, 6)))

	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	if len(buckets[0].Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(buckets[0].Tasks))
	}
	if buckets[0].Tasks[0].Task.Name != "Buy groceries" {
		t.Errorf("expected 'Buy groceries', got '%s'", buckets[0].Tasks[0].Task.Name)
	}
	if buckets[0].Tasks[0].Reason != ReasonDue {
		t.Errorf("expected ReasonDue, got %v", buckets[0].Tasks[0].Reason)
	}
}

func TestQueryAgenda_TaskScheduledDate(t *testing.T) {
	svc := &mockTaskService{
		tasks: []data.Task{
			{
				ID:   "t1",
				Name: "Review PR",
				Tags: map[string]string{"scheduled": "2026-02-06"},
			},
		},
	}

	buckets := QueryAgenda(svc, nil, nil, DayRange(date(2026, 2, 6)))

	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	if buckets[0].Tasks[0].Reason != ReasonScheduled {
		t.Errorf("expected ReasonScheduled, got %v", buckets[0].Tasks[0].Reason)
	}
}

func TestQueryAgenda_CardDueDate(t *testing.T) {
	boards := []kanbanmodels.Board{
		{
			Name: "Sprint",
			Path: "/boards/sprint",
			Columns: []kanbanmodels.Column{
				{
					Name: "To Do",
					Cards: []kanbanmodels.Card{
						{
							Title:   "Deploy v2",
							DueDate: datePtr(2026, 2, 6),
						},
					},
				},
				{
					Name: "Done",
					Cards: []kanbanmodels.Card{
						{
							Title:   "Already done",
							DueDate: datePtr(2026, 2, 6),
						},
					},
				},
			},
		},
	}

	buckets := QueryAgenda(nil, boards, nil, DayRange(date(2026, 2, 6)))

	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	// "To Do" card should be in Cards (pending)
	if len(buckets[0].Cards) != 1 {
		t.Fatalf("expected 1 pending card, got %d", len(buckets[0].Cards))
	}
	if buckets[0].Cards[0].Card.Title != "Deploy v2" {
		t.Errorf("expected 'Deploy v2', got '%s'", buckets[0].Cards[0].Card.Title)
	}
	if buckets[0].Cards[0].BoardName != "Sprint" {
		t.Errorf("expected board 'Sprint', got '%s'", buckets[0].Cards[0].BoardName)
	}
	// "Done" card should be in CompletedCards
	if len(buckets[0].CompletedCards) != 1 {
		t.Fatalf("expected 1 completed card, got %d", len(buckets[0].CompletedCards))
	}
	if buckets[0].CompletedCards[0].Card.Title != "Already done" {
		t.Errorf("expected 'Already done', got '%s'", buckets[0].CompletedCards[0].Card.Title)
	}
	if !buckets[0].CompletedCards[0].Completed {
		t.Error("expected Completed flag to be true")
	}
}

func TestQueryAgenda_MultiDayRange(t *testing.T) {
	svc := &mockTaskService{
		tasks: []data.Task{
			{ID: "t1", Name: "Task A", Tags: map[string]string{"due": "2026-02-02"}},
			{ID: "t2", Name: "Task B", Tags: map[string]string{"due": "2026-02-05"}},
			{ID: "t3", Name: "Task C", Tags: map[string]string{"due": "2026-02-08"}},
			{ID: "t4", Name: "Task D", Tags: map[string]string{"due": "2026-02-10"}}, // out of range
		},
	}

	buckets := QueryAgenda(svc, nil, nil, WeekRange(date(2026, 2, 6)))

	// Should include tasks on Feb 2, 5, and 8 (Mon Feb 2 to Sun Feb 8)
	if len(buckets) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(buckets))
	}

	// Verify chronological order
	for i := 0; i < len(buckets)-1; i++ {
		if !buckets[i].Date.Before(buckets[i+1].Date) {
			t.Errorf("buckets not in chronological order: %v >= %v", buckets[i].Date, buckets[i+1].Date)
		}
	}
}

func TestQueryAgenda_DoneTasksSeparated(t *testing.T) {
	svc := &mockTaskService{
		tasks: []data.Task{
			{ID: "t1", Name: "Pending", Tags: map[string]string{"due": "2026-02-06"}},
			{ID: "t2", Name: "Completed", Done: true, Tags: map[string]string{"due": "2026-02-06"}},
		},
	}

	buckets := QueryAgenda(svc, nil, nil, DayRange(date(2026, 2, 6)))

	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	if len(buckets[0].Tasks) != 1 {
		t.Fatalf("expected 1 pending task, got %d", len(buckets[0].Tasks))
	}
	if buckets[0].Tasks[0].Task.Name != "Pending" {
		t.Errorf("expected 'Pending', got '%s'", buckets[0].Tasks[0].Task.Name)
	}
	if len(buckets[0].CompletedTasks) != 1 {
		t.Fatalf("expected 1 completed task, got %d", len(buckets[0].CompletedTasks))
	}
	if buckets[0].CompletedTasks[0].Task.Name != "Completed" {
		t.Errorf("expected 'Completed', got '%s'", buckets[0].CompletedTasks[0].Task.Name)
	}
	if !buckets[0].CompletedTasks[0].Completed {
		t.Error("expected Completed flag to be true")
	}
}

func TestQueryAgenda_EmptyResult(t *testing.T) {
	svc := &mockTaskService{
		tasks: []data.Task{
			{ID: "t1", Name: "No date"},
		},
	}

	buckets := QueryAgenda(svc, nil, nil, DayRange(date(2026, 2, 6)))

	if len(buckets) != 0 {
		t.Fatalf("expected 0 buckets, got %d", len(buckets))
	}
}

// --- QueryOverdueItems tests ---

func TestQueryOverdueItems_BasicTask(t *testing.T) {
	svc := &mockTaskService{
		tasks: []data.Task{
			{ID: "t1", Name: "Past due task", Tags: map[string]string{"due": "2026-02-01"}},
			{ID: "t2", Name: "Today task", Tags: map[string]string{"due": "2026-02-06"}},
			{ID: "t3", Name: "Future task", Tags: map[string]string{"due": "2026-02-10"}},
		},
	}

	// Cutoff is Feb 6 — only Feb 1 is strictly before
	items := QueryOverdueItems(svc, nil, date(2026, 2, 6))

	if len(items) != 1 {
		t.Fatalf("expected 1 overdue item, got %d", len(items))
	}
	if items[0].Task.Name != "Past due task" {
		t.Errorf("expected 'Past due task', got '%s'", items[0].Task.Name)
	}
	if items[0].Reason != ReasonDue {
		t.Errorf("expected ReasonDue, got %v", items[0].Reason)
	}
}

func TestQueryOverdueItems_ScheduledDateExcluded(t *testing.T) {
	svc := &mockTaskService{
		tasks: []data.Task{
			{ID: "t1", Name: "Scheduled only", Tags: map[string]string{"scheduled": "2026-02-01"}},
			{ID: "t2", Name: "Due and scheduled", Tags: map[string]string{"due": "2026-02-01", "scheduled": "2026-02-01"}},
		},
	}

	items := QueryOverdueItems(svc, nil, date(2026, 2, 6))

	// Only the task with a due date should appear (not scheduled-only)
	if len(items) != 1 {
		t.Fatalf("expected 1 overdue item, got %d", len(items))
	}
	if items[0].Task.Name != "Due and scheduled" {
		t.Errorf("expected 'Due and scheduled', got '%s'", items[0].Task.Name)
	}
}

func TestQueryOverdueItems_DoneTasksExcluded(t *testing.T) {
	svc := &mockTaskService{
		tasks: []data.Task{
			{ID: "t1", Name: "Pending overdue", Tags: map[string]string{"due": "2026-02-01"}},
			{ID: "t2", Name: "Done overdue", Done: true, Tags: map[string]string{"due": "2026-02-01"}},
		},
	}

	items := QueryOverdueItems(svc, nil, date(2026, 2, 6))

	if len(items) != 1 {
		t.Fatalf("expected 1 overdue item, got %d", len(items))
	}
	if items[0].Task.Name != "Pending overdue" {
		t.Errorf("expected 'Pending overdue', got '%s'", items[0].Task.Name)
	}
}

func TestQueryOverdueItems_Cards(t *testing.T) {
	boards := []kanbanmodels.Board{
		{
			Name: "Sprint",
			Path: "/boards/sprint",
			Columns: []kanbanmodels.Column{
				{
					Name: "In Progress",
					Cards: []kanbanmodels.Card{
						{Title: "Overdue card", DueDate: datePtr(2026, 2, 1)},
						{Title: "Current card", DueDate: datePtr(2026, 2, 6)},
					},
				},
				{
					Name: "Done",
					Cards: []kanbanmodels.Card{
						{Title: "Done card", DueDate: datePtr(2026, 2, 1)},
					},
				},
			},
		},
	}

	items := QueryOverdueItems(nil, boards, date(2026, 2, 6))

	// Only the "In Progress" overdue card, not the "Done" card or current card
	if len(items) != 1 {
		t.Fatalf("expected 1 overdue item, got %d", len(items))
	}
	if items[0].Card.Title != "Overdue card" {
		t.Errorf("expected 'Overdue card', got '%s'", items[0].Card.Title)
	}
	if items[0].BoardName != "Sprint" {
		t.Errorf("expected board 'Sprint', got '%s'", items[0].BoardName)
	}
}

func TestQueryOverdueItems_SortOrder(t *testing.T) {
	svc := &mockTaskService{
		tasks: []data.Task{
			{ID: "t1", Name: "Recent overdue", Tags: map[string]string{"due": "2026-02-04"}},
			{ID: "t2", Name: "Oldest overdue", Tags: map[string]string{"due": "2026-01-20"}},
			{ID: "t3", Name: "Mid overdue", Tags: map[string]string{"due": "2026-01-28"}},
		},
	}

	items := QueryOverdueItems(svc, nil, date(2026, 2, 6))

	if len(items) != 3 {
		t.Fatalf("expected 3 overdue items, got %d", len(items))
	}
	// Should be sorted oldest first
	if items[0].Task.Name != "Oldest overdue" {
		t.Errorf("expected 'Oldest overdue' first, got '%s'", items[0].Task.Name)
	}
	if items[1].Task.Name != "Mid overdue" {
		t.Errorf("expected 'Mid overdue' second, got '%s'", items[1].Task.Name)
	}
	if items[2].Task.Name != "Recent overdue" {
		t.Errorf("expected 'Recent overdue' third, got '%s'", items[2].Task.Name)
	}
}

func TestQueryOverdueItems_BoundaryDate(t *testing.T) {
	svc := &mockTaskService{
		tasks: []data.Task{
			{ID: "t1", Name: "Day before", Tags: map[string]string{"due": "2026-02-05"}},
			{ID: "t2", Name: "Cutoff day", Tags: map[string]string{"due": "2026-02-06"}},
			{ID: "t3", Name: "Day after", Tags: map[string]string{"due": "2026-02-07"}},
		},
	}

	items := QueryOverdueItems(svc, nil, date(2026, 2, 6))

	// Only "Day before" is strictly before cutoff
	if len(items) != 1 {
		t.Fatalf("expected 1 overdue item, got %d", len(items))
	}
	if items[0].Task.Name != "Day before" {
		t.Errorf("expected 'Day before', got '%s'", items[0].Task.Name)
	}
}

func TestQueryOverdueItems_Empty(t *testing.T) {
	svc := &mockTaskService{
		tasks: []data.Task{
			{ID: "t1", Name: "Future task", Tags: map[string]string{"due": "2026-02-10"}},
		},
	}

	items := QueryOverdueItems(svc, nil, date(2026, 2, 6))

	if len(items) != 0 {
		t.Fatalf("expected 0 overdue items, got %d", len(items))
	}
}

func TestQueryAgenda_TaskWithBothDates(t *testing.T) {
	svc := &mockTaskService{
		tasks: []data.Task{
			{
				ID:   "t1",
				Name: "Both dates",
				Tags: map[string]string{
					"due":       "2026-02-06",
					"scheduled": "2026-02-06",
				},
			},
		},
	}

	buckets := QueryAgenda(svc, nil, nil, DayRange(date(2026, 2, 6)))

	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	// Should appear twice in tasks (once for due, once for scheduled)
	if len(buckets[0].Tasks) != 2 {
		t.Fatalf("expected 2 task items (due + scheduled), got %d", len(buckets[0].Tasks))
	}
}
