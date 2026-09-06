package replace

import (
	"os"
	"path/filepath"
	"testing"

	"vgrep/internal/model"
)

func TestReplacePattern_EmptyReplacement(t *testing.T) {
	line := "func fooBar() { foo(); }"
	pattern := "foo"

	// Case sensitive empty replacement
	got := ReplacePattern(line, pattern, "", false)
	want := "func Bar() { (); }"
	if got != want {
		t.Errorf("ReplacePattern() = %q, want %q", got, want)
	}

	// Case insensitive empty replacement
	gotCase := ReplacePattern("FooBar foo", "foo", "", true)
	wantCase := "Bar "
	if gotCase != wantCase {
		t.Errorf("ReplacePattern(ignoreCase) = %q, want %q", gotCase, wantCase)
	}
}

func TestApplyReplacement_EmptyReplacement(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	content := "alpha beta gamma\nhello alpha world\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	results := []model.WigResultItem{
		{FilePath: filePath, Line: 1, Char: 0, Text: "alpha beta gamma"},
		{FilePath: filePath, Line: 2, Char: 6, Text: "hello alpha world"},
	}

	// Replace "alpha " with empty string ""
	replacedCount, filesModified, err := ApplyReplacement(results, nil, "alpha ", "", false)
	if err != nil {
		t.Fatalf("ApplyReplacement failed: %v", err)
	}
	if replacedCount != 2 {
		t.Errorf("expected 2 replacements, got %d", replacedCount)
	}
	if filesModified != 1 {
		t.Errorf("expected 1 file modified, got %d", filesModified)
	}

	data, _ := os.ReadFile(filePath)
	wantData := "beta gamma\nhello world\n"
	if string(data) != wantData {
		t.Errorf("file content = %q, want %q", string(data), wantData)
	}
}
