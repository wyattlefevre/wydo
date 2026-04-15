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

func TestArchivedTasksExcludedByDefault(t *testing.T) {
	m := &TaskManagerModel{tasks: makeTasksWithArchived()}
	m.refreshDisplayTasks()
	for _, task := range m.displayTasks {
		if task.Archived {
			t.Errorf("expected archived tasks excluded, got %q", task.Name)
		}
	}
	if len(m.displayTasks) != 2 {
		t.Errorf("expected 2 non-archived tasks, got %d", len(m.displayTasks))
	}
}

// TestRefreshDisplayTasksDoesNotCorruptSourceSlice is a regression test for the
// duplication bug. refreshDisplayTasks used filtered[:0] (filter-in-place), which
// shares the backing array with m.tasks/s.tasks. When archived tasks were present,
// the loop overwrote the archived-task slots with later non-archived tasks, silently
// corrupting s.tasks. WriteAllTasks would then write the duplicate entries to disk.
func TestRefreshDisplayTasksDoesNotCorruptSourceSlice(t *testing.T) {
	original := makeTasksWithArchived()
	// Keep an independent copy to compare against
	snapshot := make([]data.Task, len(original))
	copy(snapshot, original)

	m := &TaskManagerModel{tasks: original}
	m.refreshDisplayTasks()

	// m.tasks must not have been mutated by refreshDisplayTasks
	if len(m.tasks) != len(snapshot) {
		t.Fatalf("m.tasks length changed: got %d, want %d", len(m.tasks), len(snapshot))
	}
	for i := range snapshot {
		if m.tasks[i].ID != snapshot[i].ID || m.tasks[i].Name != snapshot[i].Name {
			t.Errorf("m.tasks[%d] corrupted: got {ID:%s Name:%q}, want {ID:%s Name:%q}",
				i, m.tasks[i].ID, m.tasks[i].Name, snapshot[i].ID, snapshot[i].Name)
		}
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
