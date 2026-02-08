package logs

import (
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	Logger  *log.Logger
	logFile *os.File
	mu      sync.Mutex
)

// This runs automatically when the package is imported.
// Creates a logger in the current directory as a fallback.
func init() {
	f, err := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("failed to open debug file: %v", err)
	}
	logFile = f
	Logger = log.New(f, "[wydo] ", log.LstdFlags|log.Lshortfile)
}

// Initialize reinitializes the logger to write to a new directory.
func Initialize(logDir string) error {
	mu.Lock()
	defer mu.Unlock()

	if logDir == "" || logDir == "." {
		return nil
	}

	logPath := filepath.Join(logDir, "debug.log")

	Logger.Printf("Reinitializing logger to: %s", logPath)

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		Logger.Printf("Failed to open new log file at %s: %v", logPath, err)
		return err
	}

	if logFile != nil {
		logFile.Close()
	}

	logFile = f
	Logger = log.New(f, "[wydo] ", log.LstdFlags|log.Lshortfile)

	Logger.Printf("Logger successfully reinitialized to: %s", logPath)

	return nil
}

// Close closes the log file.
func Close() error {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		return logFile.Close()
	}
	return nil
}
