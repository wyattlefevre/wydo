package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// JiraConfig holds Jira API credentials and connection info
type JiraConfig struct {
	BaseURL  string `json:"base_url"`
	Email    string `json:"email"`
	APIToken string `json:"api_token"`
}

// Config holds the unified application configuration
type Config struct {
	Workspaces   []string    `json:"workspaces"`
	DefaultView  string      `json:"default_view"`
	DefaultBoard string      `json:"-"` // runtime-only: open a specific board by name
	Jira         *JiraConfig `json:"jira,omitempty"`
}

// Settings represents the config file structure
type Settings struct {
	Workspaces  []string    `json:"workspaces"`
	DefaultView string      `json:"default_view,omitempty"`
	Jira        *JiraConfig `json:"jira,omitempty"`
}

// CLIFlags holds parsed CLI flags
type CLIFlags struct {
	Workspaces []string
}

var globalConfig *Config

// Load loads configuration with priority: CLI flags > env vars > config file > default
func Load(flags CLIFlags) (*Config, error) {
	cfg := &Config{
		DefaultView: "day",
	}

	// Try loading config file first for base values
	configPath, err := getConfigPath()
	if err == nil {
		if fileConfig, err := loadConfigFile(configPath); err == nil {
			if fileConfig.DefaultView != "" {
				cfg.DefaultView = fileConfig.DefaultView
			}
			if len(fileConfig.Workspaces) > 0 {
				cfg.Workspaces = expandPaths(fileConfig.Workspaces)
			}
			if fileConfig.Jira != nil {
				cfg.Jira = fileConfig.Jira
			}
		}
	}

	// Priority 2: Environment variables override config file
	envWorkspaces := os.Getenv("WYDO_WORKSPACES")
	if envWorkspaces != "" {
		cfg.Workspaces = expandPaths(parseColonSeparated(envWorkspaces))
	}

	// Priority 1: CLI flags override everything
	if len(flags.Workspaces) > 0 {
		cfg.Workspaces = expandPaths(flags.Workspaces)
	}

	// Default directory if nothing configured
	if len(cfg.Workspaces) == 0 {
		defaultDir, err := GetDefaultDir()
		if err != nil {
			return nil, err
		}
		cfg.Workspaces = []string{defaultDir}
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

// SaveJiraConfig writes the Jira config block into the config file, preserving other settings.
func SaveJiraConfig(jira *JiraConfig) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	var settings Settings
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &settings)
	}

	settings.Jira = jira

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return err
	}

	// Update global config in memory
	if globalConfig != nil {
		globalConfig.Jira = jira
	}

	return nil
}

// EnsureWorkspaces ensures all workspace directories exist (creates them if missing)
func (c *Config) EnsureWorkspaces() error {
	for _, dir := range c.Workspaces {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// GetFirstWorkspace returns the first workspace directory
func (c *Config) GetFirstWorkspace() string {
	if len(c.Workspaces) > 0 {
		return c.Workspaces[0]
	}
	return ""
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
		Workspaces:  []string{defaultDir},
		DefaultView: "day",
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
