package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"wydo/internal/cli"
	"wydo/internal/config"
	"wydo/internal/kanban/fs"
	"wydo/internal/logs"
	"wydo/internal/tasks/service"
	"wydo/internal/tui"
)

func main() {
	// Parse CLI flags
	dirsFlag := flag.String("dirs", "", "Directories to scan (comma-separated)")
	flag.StringVar(dirsFlag, "d", "", "Directories to scan (shorthand, comma-separated)")
	recursiveDirsFlag := flag.String("recursive-dirs", "", "Root directories to recursively search (comma-separated)")
	flag.StringVar(recursiveDirsFlag, "r", "", "Root directories to recursively search (shorthand, comma-separated)")
	viewFlag := flag.String("view", "", "Initial view: day, week, month, tasks, boards")
	flag.Parse()

	// Build CLIFlags
	cliFlags := config.CLIFlags{
		Dirs:          config.ParseCommaSeparated(*dirsFlag),
		RecursiveDirs: config.ParseCommaSeparated(*recursiveDirsFlag),
	}

	// Load configuration
	cfg, err := config.Load(cliFlags)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Ensure config file exists
	if err := config.EnsureConfigFile(); err != nil {
		log.Printf("Warning: could not create config file: %v", err)
	}

	// Ensure directories exist
	if err := cfg.EnsureDirs(); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	// Reinitialize logger
	if err := logs.Initialize(cfg.GetFirstDir()); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not initialize logger: %v\n", err)
	}

	// Initialize task service from first found todo.txt
	var taskSvc service.TaskService
	var todoFilePath, doneFilePath string
	todoFiles := fs.ScanTodoFiles(cfg.Dirs, cfg.RecursiveDirs, cfg.TodoFile, cfg.DoneFile)
	if len(todoFiles) > 0 {
		first := todoFiles[0]
		todoFilePath = first.TodoPath
		doneFilePath = first.DonePath
		taskSvc, err = service.NewTaskService(todoFilePath, doneFilePath, "")
		if err != nil {
			logs.Logger.Printf("Warning: could not initialize task service: %v", err)
		}
	} else {
		// Create a task service pointing to the default directory
		todoFilePath = cfg.GetTodoFilePath(cfg.GetFirstDir())
		doneFilePath = cfg.GetDoneFilePath(cfg.GetFirstDir())
		taskSvc, err = service.NewTaskService(todoFilePath, doneFilePath, "")
		if err != nil {
			logs.Logger.Printf("Warning: could not initialize task service: %v", err)
		}
	}

	// Check for CLI subcommands
	args := flag.Args()
	if len(args) > 0 {
		if taskSvc == nil {
			fmt.Fprintln(os.Stderr, "Error: could not initialize task service")
			os.Exit(1)
		}
		exitCode := cli.Run(args, taskSvc)
		os.Exit(exitCode)
	}

	// Apply --view flag override
	if *viewFlag != "" {
		cfg.DefaultView = *viewFlag
	}

	// TUI mode
	logs.Logger.Println("Starting app in TUI mode")
	appModel := tui.NewAppModel(cfg, taskSvc, todoFilePath, doneFilePath)
	p := tea.NewProgram(appModel, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
