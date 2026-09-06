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

func TestApplyReplacement_UndoOnce(t *testing.T) {
	ClearUndo()
	if CanUndo() {
		t.Errorf("expected CanUndo to be false initially")
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "undo_test.txt")
	origContent := "foo original line\nsecond foo line\n"
	if err := os.WriteFile(filePath, []byte(origContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	results := []model.WigResultItem{
		{FilePath: filePath, Line: 1, Char: 0, Text: "foo original line"},
		{FilePath: filePath, Line: 2, Char: 7, Text: "second foo line"},
	}

	replacedCount, filesModified, err := ApplyReplacement(results, nil, "foo", "bar", false)
	if err != nil {
		t.Fatalf("ApplyReplacement failed: %v", err)
	}
	if replacedCount != 2 || filesModified != 1 {
		t.Fatalf("unexpected replace count: %d, %d", replacedCount, filesModified)
	}

	// Verify file was modified
	data, _ := os.ReadFile(filePath)
	if string(data) != "bar original line\nsecond bar line\n" {
		t.Errorf("unexpected file content after replace: %q", string(data))
	}

	// Verify CanUndo is true
	if !CanUndo() {
		t.Fatalf("expected CanUndo to be true after replacement")
	}

	// Perform Undo
	restoredResults, filesRestored, err := Undo()
	if err != nil {
		t.Fatalf("Undo failed: %v", err)
	}
	if filesRestored != 1 {
		t.Errorf("expected 1 file restored, got %d", filesRestored)
	}

	// Verify file content is back to original
	dataAfterUndo, _ := os.ReadFile(filePath)
	if string(dataAfterUndo) != origContent {
		t.Errorf("expected file content %q after undo, got %q", origContent, string(dataAfterUndo))
	}

	// Verify results were restored
	if restoredResults[0].Text != "foo original line" || restoredResults[1].Text != "second foo line" {
		t.Errorf("results not restored properly: %+v", restoredResults)
	}

	// Verify Undo for once: CanUndo is now false
	if CanUndo() {
		t.Errorf("expected CanUndo to be false after undoing once")
	}

	// Attempting Undo a second time should fail
	_, _, errSecond := Undo()
	if errSecond == nil {
		t.Errorf("expected second undo to return an error, got nil")
	}
}

func TestClearUndo(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "clear_undo.txt")
	_ = os.WriteFile(filePath, []byte("test line\n"), 0644)
	results := []model.WigResultItem{
		{FilePath: filePath, Line: 1, Char: 0, Text: "test line"},
	}

	_, _, _ = ApplyReplacement(results, nil, "test", "demo", false)
	if !CanUndo() {
		t.Fatalf("expected CanUndo to be true")
	}

	ClearUndo()
	if CanUndo() {
		t.Errorf("expected CanUndo to be false after ClearUndo")
	}
}
