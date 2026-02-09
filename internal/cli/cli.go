package cli

import (
	"fmt"
	"os"

	"wydo/internal/tasks/service"
)

// Run executes the CLI with the given arguments.
// The first argument should be the namespace ("task" or "board").
func Run(args []string, svc service.TaskService) int {
	if len(args) == 0 {
		printUsage()
		return 1
	}

	namespace := args[0]
	subArgs := args[1:]

	switch namespace {
	case "task":
		return runTaskCommand(subArgs, svc)
	case "board":
		fmt.Fprintln(os.Stderr, "Board CLI commands are not yet implemented.")
		return 1
	case "help", "-h", "--help":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", namespace)
		printUsage()
		return 1
	}
}

func runTaskCommand(args []string, svc service.TaskService) int {
	if len(args) == 0 {
		printTaskUsage()
		return 1
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "add", "a":
		return runAdd(cmdArgs, svc)
	case "list", "ls", "l":
		return runList(cmdArgs, svc)
	case "done", "do", "d":
		return runDone(cmdArgs, svc)
	case "delete", "rm", "del":
		return runDelete(cmdArgs, svc)
	case "help", "-h", "--help":
		printTaskUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Unknown task command: %s\n", command)
		printTaskUsage()
		return 1
	}
}

func printUsage() {
	fmt.Println(`wydo - Unified agenda, kanban, and task manager

Usage: wydo [flags] [command] [arguments]

Views (launch TUI into a specific view):
  agenda      Day agenda view
  boards [name]  Board picker, or open a specific board by name
  tasks       Task manager
  projects    Projects (coming soon)

Commands:
  task        Task management commands
  board       Board management commands (coming soon)

Flags:
  -w, --workspaces       Workspace directories (comma-separated)
      --view <name>      Initial view: day, week, month, tasks, boards, projects

Running wydo without arguments launches the interactive TUI.
Use "wydo task help" for task subcommands.`)
}

func printTaskUsage() {
	fmt.Println(`wydo task - Task management commands

Usage: wydo task <command> [arguments]

Commands:
  add, a      Add a new task
              wydo task add "Task description +project @context"

  list, ls, l List tasks
              wydo task list              # List all pending tasks
              wydo task list --all        # List all tasks including done
              wydo task list -p project   # Filter by project
              wydo task list -c context   # Filter by context
              wydo task list --done       # List only completed tasks

  done, do, d Mark a task as complete
              wydo task done <task-id>

  delete, rm  Delete a task
              wydo task delete <task-id>

  help        Show this help message`)
}
