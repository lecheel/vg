package ui

import (
	"bufio"
	"strings"
	"testing"
)

func TestLineEditor_BasicEditing(t *testing.T) {
	le := NewLineEditor()
	if le.Text() != "" || le.Pos() != 0 {
		t.Fatalf("expected empty editor")
	}

	le.InsertString("hello world")
	if le.Text() != "hello world" || le.Pos() != 11 {
		t.Errorf("got text %q, pos %d", le.Text(), le.Pos())
	}

	// Move home (Ctrl+A)
	le.MoveHome()
	if le.Pos() != 0 {
		t.Errorf("expected pos 0 after MoveHome, got %d", le.Pos())
	}

	// Move end (Ctrl+E)
	le.MoveEnd()
	if le.Pos() != 11 {
		t.Errorf("expected pos 11 after MoveEnd, got %d", le.Pos())
	}

	// Move word left (Alt+B)
	le.MoveWordLeft()
	if le.Pos() != 6 {
		t.Errorf("expected pos 6 after MoveWordLeft, got %d", le.Pos())
	}

	// Move word right (Alt+F)
	le.MoveWordRight()
	if le.Pos() != 11 {
		t.Errorf("expected pos 11 after MoveWordRight, got %d", le.Pos())
	}
}

func TestLineEditor_Deletions(t *testing.T) {
	le := NewLineEditor("abcdef")
	le.MoveHome()
	le.MoveRight()
	le.MoveRight() // Pos at 2 ("ab|cdef")

	// Delete forward (Ctrl+D / Delete)
	le.DeleteForward()
	if le.Text() != "abdef" {
		t.Errorf("expected 'abdef', got %q", le.Text())
	}

	// Delete backward (Backspace)
	le.DeleteBack()
	if le.Text() != "adef" || le.Pos() != 1 {
		t.Errorf("expected 'adef' at pos 1, got %q at %d", le.Text(), le.Pos())
	}

	// Delete to start (Ctrl+U)
	le.DeleteToStart()
	if le.Text() != "def" || le.Pos() != 0 {
		t.Errorf("expected 'def' at pos 0, got %q at %d", le.Text(), le.Pos())
	}

	// Delete to end (Ctrl+K)
	le.SetText("one two three")
	le.MoveHome()
	le.MoveWordRight() // "one |two three"
	le.DeleteToEnd()
	if le.Text() != "one " {
		t.Errorf("expected 'one ', got %q", le.Text())
	}

	// Delete word back (Ctrl+W)
	le.SetText("foo bar baz")
	le.MoveEnd()
	le.DeleteWordBack()
	if le.Text() != "foo bar " {
		t.Errorf("expected 'foo bar ', got %q", le.Text())
	}
}

func TestLineEditor_RenderCursor(t *testing.T) {
	le := NewLineEditor("abc")
	le.MoveHome() // cursor at 'a'
	rendered := le.Render("[C]", "[/C]")
	if rendered != "[C]a[/C]bc" {
		t.Errorf("expected '[C]a[/C]bc', got %q", rendered)
	}

	le.MoveEnd() // cursor at end
	rendered = le.Render("[C]", "[/C]")
	if rendered != "abc[C] [/C]" {
		t.Errorf("expected 'abc[C] [/C]', got %q", rendered)
	}
}

func TestLineEditor_HandleInputCtrlKeys(t *testing.T) {
	le := NewLineEditor("testing")
	reader := bufio.NewReader(strings.NewReader(""))

	// Ctrl+A -> MoveHome
	action := le.HandleInput(1, reader)
	if action != RLContinue || le.Pos() != 0 {
		t.Errorf("Ctrl+A failed, pos=%d", le.Pos())
	}

	// Ctrl+E -> MoveEnd
	action = le.HandleInput(5, reader)
	if action != RLContinue || le.Pos() != 7 {
		t.Errorf("Ctrl+E failed, pos=%d", le.Pos())
	}

	// Enter -> RLSubmit
	action = le.HandleInput('\n', reader)
	if action != RLSubmit {
		t.Errorf("Enter expected RLSubmit, got %v", action)
	}

	// Tab -> RLTab
	action = le.HandleInput('\t', reader)
	if action != RLTab {
		t.Errorf("Tab expected RLTab, got %v", action)
	}
}

func TestLineEditor_HandleInputAltQ(t *testing.T) {
	le := NewLineEditor("test")
	reader := bufio.NewReader(strings.NewReader("q"))
	action := le.HandleInput(27, reader)
	if action != RLQuit {
		t.Errorf("expected RLQuit on Alt+q, got %v", action)
	}

	readerUpper := bufio.NewReader(strings.NewReader("Q"))
	actionUpper := le.HandleInput(27, readerUpper)
	if actionUpper != RLQuit {
		t.Errorf("expected RLQuit on Alt+Q, got %v", actionUpper)
	}
}
