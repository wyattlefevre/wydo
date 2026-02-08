package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the unified application configuration
type Config struct {
	Dirs          []string `json:"dirs"`
	RecursiveDirs []string `json:"recursive_dirs"`
	TodoFile      string   `json:"todo_file"`
	DoneFile      string   `json:"done_file"`
	DefaultView   string   `json:"default_view"`
}

// Settings represents the config file structure
type Settings struct {
	Dirs          []string `json:"dirs"`
	RecursiveDirs []string `json:"recursive_dirs"`
	TodoFile      string   `json:"todo_file,omitempty"`
	DoneFile      string   `json:"done_file,omitempty"`
	DefaultView   string   `json:"default_view,omitempty"`
}

// CLIFlags holds parsed CLI flags
type CLIFlags struct {
	Dirs          []string
	RecursiveDirs []string
}

var globalConfig *Config

// Load loads configuration with priority: CLI flags > env vars > config file > default
func Load(flags CLIFlags) (*Config, error) {
	cfg := &Config{
		TodoFile:    "todo.txt",
		DoneFile:    "done.txt",
		DefaultView: "day",
	}

	// Try loading config file first for base values
	configPath, err := getConfigPath()
	if err == nil {
		if fileConfig, err := loadConfigFile(configPath); err == nil {
			if fileConfig.TodoFile != "" {
				cfg.TodoFile = fileConfig.TodoFile
			}
			if fileConfig.DoneFile != "" {
				cfg.DoneFile = fileConfig.DoneFile
			}
			if fileConfig.DefaultView != "" {
				cfg.DefaultView = fileConfig.DefaultView
			}
			if len(fileConfig.Dirs) > 0 || len(fileConfig.RecursiveDirs) > 0 {
				cfg.Dirs = expandPaths(fileConfig.Dirs)
				cfg.RecursiveDirs = expandPaths(fileConfig.RecursiveDirs)
			}
		}
	}

	// Priority 2: Environment variables override config file
	envDirs := os.Getenv("WYDO_DIRS")
	envRecursiveDirs := os.Getenv("WYDO_RECURSIVE_DIRS")
	if envDirs != "" || envRecursiveDirs != "" {
		if envDirs != "" {
			cfg.Dirs = expandPaths(parseColonSeparated(envDirs))
		}
		if envRecursiveDirs != "" {
			cfg.RecursiveDirs = expandPaths(parseColonSeparated(envRecursiveDirs))
		}
	}

	// Priority 1: CLI flags override everything
	if len(flags.Dirs) > 0 || len(flags.RecursiveDirs) > 0 {
		if len(flags.Dirs) > 0 {
			cfg.Dirs = expandPaths(flags.Dirs)
		}
		if len(flags.RecursiveDirs) > 0 {
			cfg.RecursiveDirs = expandPaths(flags.RecursiveDirs)
		}
	}

	// Default directory if nothing configured
	if len(cfg.Dirs) == 0 && len(cfg.RecursiveDirs) == 0 {
		defaultDir, err := GetDefaultDir()
		if err != nil {
			return nil, err
		}
		cfg.Dirs = []string{defaultDir}
	}

	globalConfig = cfg
	return cfg, nil
}

// Get returns the loaded config
func Get() *Config {
	return globalConfig
}

// GetDefaultDir returns the default directory path
func GetDefaultDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "wydo"), nil
}

// getConfigPath returns the path to the configuration file
func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "wydo", "config.json"), nil
}

// loadConfigFile loads configuration from the settings file
func loadConfigFile(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// EnsureDirs ensures all regular directories exist (creates them if missing)
func (c *Config) EnsureDirs() error {
	for _, dir := range c.Dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// GetFirstDir returns the first regular directory
func (c *Config) GetFirstDir() string {
	if len(c.Dirs) > 0 {
		return c.Dirs[0]
	}
	return ""
}

// GetTodoFilePath returns the path to the todo.txt file in a given directory
func (c *Config) GetTodoFilePath(dir string) string {
	return filepath.Join(dir, c.TodoFile)
}

// GetDoneFilePath returns the path to the done.txt file in a given directory
func (c *Config) GetDoneFilePath(dir string) string {
	return filepath.Join(dir, c.DoneFile)
}

// EnsureConfigFile creates the config file with defaults if it doesn't exist
func EnsureConfigFile() error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(configPath); err == nil {
		return nil
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	defaultDir, err := GetDefaultDir()
	if err != nil {
		return err
	}

	settings := Settings{
		Dirs:          []string{defaultDir},
		RecursiveDirs: []string{},
		TodoFile:      "todo.txt",
		DoneFile:      "done.txt",
		DefaultView:   "day",
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// ParseCommaSeparated splits a comma-separated string into a slice
func ParseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func parseColonSeparated(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ":")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

func expandPaths(paths []string) []string {
	result := make([]string, len(paths))
	for i, p := range paths {
		result[i] = expandPath(p)
	}
	return result
}
