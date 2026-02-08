package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"wydo/internal/tasks/data"
	"wydo/internal/tasks/service"
)

func runAdd(args []string, svc service.TaskService) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: task description required")
		fmt.Fprintln(os.Stderr, "Usage: wydo task add \"Task description +project @context\"")
		return 1
	}

	rawLine := strings.Join(args, " ")

	task, err := svc.Add(rawLine)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding task: %v\n", err)
		return 1
	}

	fmt.Printf("Added: %s\n", task.String())
	fmt.Printf("ID: %s\n", task.ID)
	return 0
}

func runList(args []string, svc service.TaskService) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	project := fs.String("p", "", "Filter by project")
	context := fs.String("c", "", "Filter by context")
	showDone := fs.Bool("done", false, "Show only completed tasks")
	showAll := fs.Bool("all", false, "Show all tasks including completed")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	var tasks []data.Task
	var err error

	if *showDone {
		tasks, err = svc.ListDone()
	} else if *showAll {
		tasks, err = svc.List()
	} else {
		tasks, err = svc.ListPending()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading tasks: %v\n", err)
		return 1
	}

	if *project != "" {
		tasks = filterByProject(tasks, *project)
	}
	if *context != "" {
		tasks = filterByContext(tasks, *context)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return 0
	}

	for _, t := range tasks {
		printTask(t)
	}

	fmt.Printf("\n%d task(s)\n", len(tasks))
	return 0
}

func runDone(args []string, svc service.TaskService) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: task ID required")
		fmt.Fprintln(os.Stderr, "Usage: wydo task done <task-id>")
		return 1
	}

	taskID := args[0]

	task, err := findTaskByPartialID(svc, taskID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if task.Done {
		fmt.Printf("Task already completed: %s\n", task.Name)
		return 0
	}

	err = svc.Complete(task.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error completing task: %v\n", err)
		return 1
	}

	fmt.Printf("Completed: %s\n", task.Name)
	return 0
}

func runDelete(args []string, svc service.TaskService) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: task ID required")
		fmt.Fprintln(os.Stderr, "Usage: wydo task delete <task-id>")
		return 1
	}

	taskID := args[0]

	task, err := findTaskByPartialID(svc, taskID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	err = svc.Delete(task.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting task: %v\n", err)
		return 1
	}

	fmt.Printf("Deleted: %s\n", task.Name)
	return 0
}

func filterByProject(tasks []data.Task, project string) []data.Task {
	var filtered []data.Task
	for _, t := range tasks {
		if t.HasProject(project) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func filterByContext(tasks []data.Task, context string) []data.Task {
	var filtered []data.Task
	for _, t := range tasks {
		if t.HasContext(context) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func printTask(t data.Task) {
	status := " "
	if t.Done {
		status = "x"
	}

	priority := ""
	if t.Priority != 0 {
		priority = fmt.Sprintf("(%c) ", t.Priority)
	}

	fmt.Printf("[%s] %s %s%s\n", t.ID[:7], status, priority, t.Name)

	var meta []string
	for _, p := range t.Projects {
		meta = append(meta, "+"+p)
	}
	for _, c := range t.Contexts {
		meta = append(meta, "@"+c)
	}
	if len(meta) > 0 {
		fmt.Printf("        ")
		for _, m := range meta {
			fmt.Printf("%s ", m)
		}
		fmt.Println()
	}
}

func findTaskByPartialID(svc service.TaskService, partialID string) (*data.Task, error) {
	tasks, err := svc.List()
	if err != nil {
		return nil, err
	}

	var matches []data.Task
	for _, t := range tasks {
		if t.ID == partialID || (len(partialID) >= 4 && len(t.ID) >= len(partialID) && t.ID[:len(partialID)] == partialID) {
			matches = append(matches, t)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no task found with ID: %s", partialID)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple tasks match ID '%s', please be more specific", partialID)
	}

	return &matches[0], nil
}
