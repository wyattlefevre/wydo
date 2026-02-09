package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"wydo/internal/cli"
	"wydo/internal/config"
	"wydo/internal/logs"
	"wydo/internal/scanner"
	"wydo/internal/tasks/service"
	"wydo/internal/tui"
	"wydo/internal/workspace"
)

func main() {
	// Parse CLI flags
	workspacesFlag := flag.String("workspaces", "", "Workspace directories (comma-separated)")
	flag.StringVar(workspacesFlag, "w", "", "Workspace directories (shorthand, comma-separated)")
	viewFlag := flag.String("view", "", "Initial view: day, week, month, tasks, boards")
	flag.Parse()

	// Build CLIFlags
	cliFlags := config.CLIFlags{
		Workspaces: config.ParseCommaSeparated(*workspacesFlag),
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

	// Ensure workspace directories exist
	if err := cfg.EnsureWorkspaces(); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	// Reinitialize logger
	if err := logs.Initialize(cfg.GetFirstWorkspace()); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not initialize logger: %v\n", err)
	}

	// Scan and load all workspaces
	var workspaces []*workspace.Workspace
	var allTaskDirs []scanner.TaskDirInfo

	for _, wsDir := range cfg.Workspaces {
		scan, err := scanner.ScanWorkspace(wsDir)
		if err != nil {
			logs.Logger.Printf("Warning: could not scan workspace %s: %v", wsDir, err)
			continue
		}
		ws, err := workspace.Load(scan)
		if err != nil {
			logs.Logger.Printf("Warning: could not load workspace %s: %v", wsDir, err)
			continue
		}
		workspaces = append(workspaces, ws)
		allTaskDirs = append(allTaskDirs, scan.TaskDirs...)
	}

	// Build a combined task service for CLI use (aggregates all workspaces)
	var taskSvc service.TaskService
	if len(allTaskDirs) > 0 {
		taskSvc, err = service.NewTaskService(allTaskDirs)
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
	appModel := tui.NewAppModel(cfg, workspaces)
	p := tea.NewProgram(appModel, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
