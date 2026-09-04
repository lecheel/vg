package search

import (
	"os"
	"path/filepath"
	"testing"

	"vgrep/internal/config"
	"vgrep/internal/model"
)

func TestRunRipgrep_FixedStringsFlag(t *testing.T) {
	if !config.HasExecutable("rg") {
		t.Skip("ripgrep (rg) not installed, skipping test")
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "sample.txt")
	// Content contains characters that would fail or match differently in standard regex
	content := "foo[bar]\nfooXbar\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Change working directory to temp dir for test
	origWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origWd) }()

	// Searching "foo[bar]" with fixedStrings=true should match literal "foo[bar]"
	results, err := RunRipgrep("foo[bar]", nil, false, true)
	if err != nil {
		t.Fatalf("RunRipgrep with fixed strings failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 match for literal 'foo[bar]', got %d", len(results))
	}
}

func TestRunRipgrep_ConfigDefaultFixedStrings(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "vgrep")
	_ = os.MkdirAll(configDir, 0755)
	_ = os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("fixed_strings = true\n"), 0644)

	cfg := config.LoadConfig()
	if !cfg.FixedStrings {
		t.Errorf("expected config.FixedStrings to be true")
	}
}

func TestWigSessionReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	expected := []model.WigResultItem{
		{
			FilePath: "/project/main.go",
			Line:     42,
			Char:     10,
			Text:     "func main() {}",
		},
	}

	if err := WriteWigSession(expected); err != nil {
		t.Fatalf("WriteWigSession failed: %v", err)
	}

	got, err := ReadWigSession()
	if err != nil {
		t.Fatalf("ReadWigSession failed: %v", err)
	}

	if len(got) != len(expected) {
		t.Fatalf("expected %d items, got %d", len(expected), len(got))
	}
	if got[0].FilePath != expected[0].FilePath || got[0].Line != expected[0].Line || got[0].Char != expected[0].Char {
		t.Errorf("mismatch in session item: got %+v, want %+v", got[0], expected[0])
	}
}

func TestFindProjectRoot(t *testing.T) {
	root := FindProjectRoot()
	if root == "" {
		t.Errorf("expected valid non-empty project root")
	}
}
