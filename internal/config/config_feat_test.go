package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := LoadConfig()
	if cfg.Editor != "" {
		t.Errorf("expected empty default editor, got %q", cfg.Editor)
	}
	if cfg.SessionFile != "" {
		t.Errorf("expected empty default session file, got %q", cfg.SessionFile)
	}
	if cfg.FixedStrings {
		t.Errorf("expected fixed_strings to be false by default")
	}
}

func TestLoadConfig_Parse(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "vgrep")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create temp config dir: %v", err)
	}

	content := `# vgrep configuration test
editor = "nvim"
session_file = "~/custom/rg_search.json"
fixed_strings = true
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg := LoadConfig()
	if cfg.Editor != "nvim" {
		t.Errorf("expected editor 'nvim', got %q", cfg.Editor)
	}

	expectedSession := filepath.Join(tmpDir, "custom", "rg_search.json")
	if cfg.SessionFile != expectedSession {
		t.Errorf("expected session file %q, got %q", expectedSession, cfg.SessionFile)
	}

	if !cfg.FixedStrings {
		t.Errorf("expected fixed_strings to be true")
	}
}

func TestExpandHome(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tests := []struct {
		input    string
		expected string
	}{
		{"~", tmpDir},
		{"~/test.json", filepath.Join(tmpDir, "test.json")},
		{"/var/log/app.log", "/var/log/app.log"},
		{"relative/path.txt", "relative/path.txt"},
	}

	for _, tt := range tests {
		got := ExpandHome(tt.input)
		if got != tt.expected {
			t.Errorf("ExpandHome(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestGetWigSessionPath_Custom(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "vgrep")
	_ = os.MkdirAll(configDir, 0755)

	customSession := filepath.Join(tmpDir, "my_sessions", "session.json")
	content := "session_file = \"" + strings.ReplaceAll(customSession, "\\", "/") + "\"\n"
	_ = os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(content), 0644)

	path := GetWigSessionPath()
	if path != customSession {
		t.Errorf("GetWigSessionPath() = %q, want %q", path, customSession)
	}
}

func TestGetEditor_Resolution(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// When nothing is configured or installed in temp PATH, fallback to default
	t.Setenv("EDITOR", "")
	ed := GetEditor()
	if ed == "" {
		t.Errorf("expected fallback editor, got empty string")
	}
}
