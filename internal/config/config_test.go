package config

import (
	"os"
	"testing"
)

func TestLoad_Default(t *testing.T) {
	// Clear env vars to test defaults
	os.Unsetenv("WYDO_WORKSPACES")

	cfg, err := Load(CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Workspaces) == 0 {
		t.Error("expected default workspace")
	}

	if cfg.DefaultView != "day" {
		t.Errorf("expected default view 'day', got %q", cfg.DefaultView)
	}
}

func TestLoad_EnvVar(t *testing.T) {
	t.Setenv("WYDO_WORKSPACES", "/tmp/ws1:/tmp/ws2")

	cfg, err := Load(CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(cfg.Workspaces))
	}
	if cfg.Workspaces[0] != "/tmp/ws1" {
		t.Errorf("expected /tmp/ws1, got %q", cfg.Workspaces[0])
	}
	if cfg.Workspaces[1] != "/tmp/ws2" {
		t.Errorf("expected /tmp/ws2, got %q", cfg.Workspaces[1])
	}
}

func TestLoad_CLIFlags(t *testing.T) {
	t.Setenv("WYDO_WORKSPACES", "/tmp/env-ws")

	cfg, err := Load(CLIFlags{
		Workspaces: []string{"/tmp/cli-ws1", "/tmp/cli-ws2"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// CLI flags should override env vars
	if len(cfg.Workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(cfg.Workspaces))
	}
	if cfg.Workspaces[0] != "/tmp/cli-ws1" {
		t.Errorf("expected /tmp/cli-ws1, got %q", cfg.Workspaces[0])
	}
}

func TestLoad_PathExpansion(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	cfg, err := Load(CLIFlags{
		Workspaces: []string{"~/test-workspace"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := homeDir + "/test-workspace"
	if cfg.Workspaces[0] != expected {
		t.Errorf("expected %q, got %q", expected, cfg.Workspaces[0])
	}
}

func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"a", 1},
		{"a,b,c", 3},
		{" a , b , c ", 3},
		{"a,,b", 2},
	}

	for _, tt := range tests {
		result := ParseCommaSeparated(tt.input)
		if len(result) != tt.expected {
			t.Errorf("ParseCommaSeparated(%q): expected %d items, got %d", tt.input, tt.expected, len(result))
		}
	}
}

func TestGetFirstWorkspace(t *testing.T) {
	cfg := &Config{Workspaces: []string{"/first", "/second"}}
	if cfg.GetFirstWorkspace() != "/first" {
		t.Errorf("expected /first, got %q", cfg.GetFirstWorkspace())
	}

	empty := &Config{}
	if empty.GetFirstWorkspace() != "" {
		t.Errorf("expected empty string, got %q", empty.GetFirstWorkspace())
	}
}
