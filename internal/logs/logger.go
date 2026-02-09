package logs

import (
	"io"
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

// Logger is off (discards output) by default.
func init() {
	Logger = log.New(io.Discard, "[wydo] ", log.LstdFlags|log.Lshortfile)
}

// Initialize enables logging to /tmp/wydo-debug.log, or to a file inside
// logDir if provided.
func Initialize(logDir string) error {
	mu.Lock()
	defer mu.Unlock()

	logPath := filepath.Join("/tmp", "wydo-debug.log")
	if logDir != "" && logDir != "." {
		logPath = filepath.Join(logDir, "debug.log")
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	if logFile != nil {
		logFile.Close()
	}

	logFile = f
	Logger = log.New(f, "[wydo] ", log.LstdFlags|log.Lshortfile)

	Logger.Printf("Logger initialized: %s", logPath)

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
