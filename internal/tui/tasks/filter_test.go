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

func makeTasksWithArchived() []data.Task {
	return []data.Task{
		{ID: "1", Name: "pending task", Done: false, Archived: false},
		{ID: "2", Name: "done task", Done: true, Archived: false},
		{ID: "3", Name: "archived task", Done: true, Archived: true},
		{ID: "4", Name: "archived pending", Done: false, Archived: true},
	}
}

func TestShowArchived_False_ExcludesArchivedTasks(t *testing.T) {
	m := &TaskManagerModel{showArchived: false, tasks: makeTasksWithArchived()}
	m.refreshDisplayTasks()
	for _, task := range m.displayTasks {
		if task.Archived {
			t.Errorf("showArchived=false: got archived task %q", task.Name)
		}
	}
	if len(m.displayTasks) != 2 {
		t.Errorf("showArchived=false: expected 2 tasks, got %d", len(m.displayTasks))
	}
}

func TestShowArchived_True_IncludesArchivedTasks(t *testing.T) {
	m := &TaskManagerModel{showArchived: true, tasks: makeTasksWithArchived()}
	m.refreshDisplayTasks()
	if len(m.displayTasks) != 4 {
		t.Errorf("showArchived=true: expected 4 tasks, got %d", len(m.displayTasks))
	}
}

func TestStatusFilterDone_ShowsDoneTasks(t *testing.T) {
	tasks := makeTasks()
	state := FilterState{StatusFilter: StatusDone}
	filtered := ApplyFilters(tasks, state)

	if len(filtered) != 2 {
		t.Errorf("StatusDone: expected 2 tasks, got %d", len(filtered))
	}
	for _, task := range filtered {
		if !task.Done {
			t.Errorf("StatusDone: got pending task %q", task.Name)
		}
	}
}
