package data

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"wydo/internal/logs"
)

var (
	mu sync.RWMutex
)

func HashTaskLine(line string) string {
	h := sha1.New()
	h.Write([]byte(line))
	return hex.EncodeToString(h.Sum(nil))[:10]
}

type ParseTaskMismatchError struct {
	Msg string
}

func (e *ParseTaskMismatchError) Error() string {
	return e.Msg
}

func UpdateTask(tasks []Task, updatedTask Task) []Task {
	logs.Logger.Printf("Update Task: %s\n", updatedTask)
	for i, t := range tasks {
		if t.ID == updatedTask.ID {
			logs.Logger.Println("task found. updating...")
			tasks[i] = updatedTask
			return tasks
		}
	}
	logs.Logger.Printf("ERROR: UpdateTask: task ID %s not found — skipping to prevent duplicate creation\n", updatedTask.ID)
	return tasks
}

// LoadTasksFromPaths loads todo.txt from an explicit todoPath.
// dirPath is the tasks/ directory (for context); todoPath is the actual file to read.
// Returns empty (no error) if todoPath does not exist yet.
func LoadTasksFromPaths(dirPath, todoPath string, allowMismatch bool) ([]Task, error) {
	tasks, err := loadTaskFile(todoPath, allowMismatch, make(map[string]Project))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		if _, ok := err.(*ParseTaskMismatchError); ok {
			return nil, err
		}
		return nil, fmt.Errorf("error reading %s: %v", todoPath, err)
	}
	return tasks, nil
}

// LoadTasksFromDir loads todo.txt from a tasks/ directory.
// Backward-compatible shim; prefer LoadTasksFromPaths for new code.
// Returns empty (no error) if todo.txt does not exist yet.
func LoadTasksFromDir(dirPath string, allowMismatch bool) ([]Task, error) {
	return LoadTasksFromPaths(dirPath, filepath.Join(dirPath, "todo.txt"), allowMismatch)
}

// WriteTasksToFile writes tasks that belong to a specific file
func WriteTasksToFile(tasks []Task, filePath string) error {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("error creating directory: %v", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error writing %s: %v", filePath, err)
	}
	defer file.Close()

	for _, task := range tasks {
		if task.File != filePath {
			continue
		}
		if _, err := fmt.Fprintln(file, task.String()); err != nil {
			return fmt.Errorf("error writing to %s: %v", filePath, err)
		}
	}
	return nil
}

// WriteAllTasks groups tasks by their File field and writes each group
func WriteAllTasks(tasks []Task) error {
	grouped := make(map[string][]Task)
	for _, t := range tasks {
		if t.File != "" {
			grouped[t.File] = append(grouped[t.File], t)
		}
	}
	for filePath, fileTasks := range grouped {
		if err := writeTaskSliceToFile(fileTasks, filePath); err != nil {
			return err
		}
	}
	return nil
}

func writeTaskSliceToFile(tasks []Task, filePath string) error {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("error creating directory: %v", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error writing %s: %v", filePath, err)
	}
	defer file.Close()

	for _, task := range tasks {
		if _, err := fmt.Fprintln(file, task.String()); err != nil {
			return fmt.Errorf("error writing to %s: %v", filePath, err)
		}
	}
	return nil
}

func TaskCount(tasks []Task, project string) (int, int) {
	todoCount := 0
	doneCount := 0
	for _, task := range tasks {
		if task.HasProject(project) {
			if task.Done {
				doneCount++
			} else {
				todoCount++
			}
		}
	}
	return todoCount, doneCount
}

func loadTaskFile(filePath string, allowMismatch bool, projects map[string]Project) ([]Task, error) {
	mu.Lock()
	defer mu.Unlock()

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	taskList := []Task{}

	scanner := bufio.NewScanner(file)
	nonEmptyCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		nonEmptyCount++
		hashId := HashTaskLine(fmt.Sprintf("%d:%s", nonEmptyCount, filePath))
		task := ParseTask(line, hashId, filePath)
		// Mark as archived if the file lives inside (or directly under) an archive/ directory.
		// Handles both <ws>/archive/tasks/todo.txt (old) and <ws>/archive/todo.txt (new).
		task.Archived = isArchivedFilePath(filePath)
		for _, project := range task.Projects {
			if _, exists := projects[project]; !exists {
				projects[project] = Project{Name: project}
			}
		}
		if task.String() != line && !allowMismatch {
			msg := fmt.Sprintf("malformed task\nparsed: %s\noriginal: %s", task.String(), line)
			logs.Logger.Println(msg)
			return nil, &ParseTaskMismatchError{Msg: msg}
		}
		taskList = append(taskList, task)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return taskList, nil
}

// DeleteTask removes a task by ID from the task slice and returns the updated slice.
func DeleteTask(tasks []Task, id string) []Task {
	for i, t := range tasks {
		if t.ID == id {
			return append(tasks[:i], tasks[i+1:]...)
		}
	}
	return tasks
}

// isArchivedFilePath returns true if the file path lives under an archive/ directory.
// Handles both the old layout (<ws>/archive/tasks/todo.txt) and the new layout (<ws>/archive/todo.txt).
func isArchivedFilePath(p string) bool {
	d1 := filepath.Dir(p)
	if filepath.Base(d1) == "archive" {
		return true
	}
	return filepath.Base(filepath.Dir(d1)) == "archive"
}

// AppendTaskToFile appends a single task line to a file.
func AppendTaskToFile(rawLine, todoFilePath string) (*Task, error) {
	mu.Lock()
	defer mu.Unlock()

	rawLine = strings.TrimSpace(rawLine)
	if rawLine == "" {
		return nil, fmt.Errorf("empty task line")
	}

	if err := os.MkdirAll(filepath.Dir(todoFilePath), 0755); err != nil {
		return nil, fmt.Errorf("error creating directory: %v", err)
	}

	lineCount := 0
	needsNewline := false
	file, err := os.Open(todoFilePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error opening %s: %v", todoFilePath, err)
	}
	if file != nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) != "" {
				lineCount++
			}
		}
		// Check whether the file ends with a newline so we can ensure the
		// appended task starts on its own line.
		if info, statErr := file.Stat(); statErr == nil && info.Size() > 0 {
			if _, seekErr := file.Seek(-1, io.SeekEnd); seekErr == nil {
				last := make([]byte, 1)
				if _, readErr := file.Read(last); readErr == nil {
					needsNewline = last[0] != '\n'
				}
			}
		}
		file.Close()
	}

	hashId := HashTaskLine(fmt.Sprintf("%d:%s", lineCount+1, todoFilePath))
	task := ParseTask(rawLine, hashId, todoFilePath)

	f, err := os.OpenFile(todoFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("error opening %s for append: %v", todoFilePath, err)
	}
	defer f.Close()

	if needsNewline {
		if _, err := fmt.Fprint(f, "\n"); err != nil {
			return nil, fmt.Errorf("error writing newline to %s: %v", todoFilePath, err)
		}
	}

	_, err = fmt.Fprintln(f, task.String())
	if err != nil {
		return nil, fmt.Errorf("error writing to %s: %v", todoFilePath, err)
	}

	return &task, nil
}
