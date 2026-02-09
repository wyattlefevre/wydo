package tasks

import (
	"testing"

	"wydo/internal/tasks/data"
)

func makeTasks() []data.Task {
	return []data.Task{
		{ID: "1", Name: "pending task", Done: false},
		{ID: "2", Name: "done task", Done: true},
		{ID: "3", Name: "another pending", Done: false},
		{ID: "4", Name: "another done", Done: true},
	}
}

func TestFileViewAll_ShowsBothPendingAndDone(t *testing.T) {
	m := &TaskManagerModel{fileViewMode: FileViewAll}
	tasks := makeTasks()
	result := m.applyFileViewFilter(tasks)
	if len(result) != 4 {
		t.Errorf("FileViewAll: expected 4 tasks, got %d", len(result))
	}
}

func TestFileViewTodoOnly_ExcludesDone(t *testing.T) {
	m := &TaskManagerModel{fileViewMode: FileViewTodoOnly}
	tasks := makeTasks()
	result := m.applyFileViewFilter(tasks)
	if len(result) != 2 {
		t.Errorf("FileViewTodoOnly: expected 2 tasks, got %d", len(result))
	}
	for _, task := range result {
		if task.Done {
			t.Errorf("FileViewTodoOnly: got done task %q", task.Name)
		}
	}
}

func TestFileViewDoneOnly_ExcludesPending(t *testing.T) {
	m := &TaskManagerModel{fileViewMode: FileViewDoneOnly}
	tasks := makeTasks()
	result := m.applyFileViewFilter(tasks)
	if len(result) != 2 {
		t.Errorf("FileViewDoneOnly: expected 2 tasks, got %d", len(result))
	}
	for _, task := range result {
		if !task.Done {
			t.Errorf("FileViewDoneOnly: got pending task %q", task.Name)
		}
	}
}

func TestStatusFilterDone_WithFileViewAll_ShowsDoneTasks(t *testing.T) {
	tasks := makeTasks()
	state := FilterState{StatusFilter: StatusDone}
	filtered := ApplyFilters(tasks, state)

	m := &TaskManagerModel{fileViewMode: FileViewAll}
	result := m.applyFileViewFilter(filtered)

	if len(result) != 2 {
		t.Errorf("StatusDone + FileViewAll: expected 2 tasks, got %d", len(result))
	}
	for _, task := range result {
		if !task.Done {
			t.Errorf("StatusDone + FileViewAll: got pending task %q", task.Name)
		}
	}
}

func TestStatusFilterDone_WithFileViewTodoOnly_ShowsNothing(t *testing.T) {
	tasks := makeTasks()
	state := FilterState{StatusFilter: StatusDone}
	filtered := ApplyFilters(tasks, state)

	m := &TaskManagerModel{fileViewMode: FileViewTodoOnly}
	result := m.applyFileViewFilter(filtered)

	if len(result) != 0 {
		t.Errorf("StatusDone + FileViewTodoOnly: expected 0 tasks, got %d", len(result))
	}
}
