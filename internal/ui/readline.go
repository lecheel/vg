package ui

import (
	"bufio"
	"time"
	"unicode/utf8"
)

type ReadlineAction int

const (
	RLContinue ReadlineAction = iota
	RLSubmit
	RLCancel
	RLTab
	RLToggleCase
	RLArrowUp
	RLArrowDown
)

// LineEditor provides full Readline-style in-memory line editing with cursor tracking.
type LineEditor struct {
	buf []rune
	pos int
}

func NewLineEditor(initial ...string) *LineEditor {
	le := &LineEditor{}
	if len(initial) > 0 && initial[0] != "" {
		le.SetText(initial[0])
	}
	return le
}

func (le *LineEditor) Text() string {
	return string(le.buf)
}

func (le *LineEditor) Runes() []rune {
	return le.buf
}

func (le *LineEditor) Pos() int {
	return le.pos
}

func (le *LineEditor) Len() int {
	return len(le.buf)
}

func (le *LineEditor) SetText(s string) {
	le.buf = []rune(s)
	le.pos = len(le.buf)
}

func (le *LineEditor) Clear() {
	le.buf = nil
	le.pos = 0
}

func (le *LineEditor) InsertRune(r rune) {
	le.buf = append(le.buf[:le.pos], append([]rune{r}, le.buf[le.pos:]...)...)
	le.pos++
}

func (le *LineEditor) InsertString(s string) {
	runes := []rune(s)
	le.buf = append(le.buf[:le.pos], append(runes, le.buf[le.pos:]...)...)
	le.pos += len(runes)
}

func (le *LineEditor) MoveHome() {
	le.pos = 0
}

func (le *LineEditor) MoveEnd() {
	le.pos = len(le.buf)
}

func (le *LineEditor) MoveLeft() {
	if le.pos > 0 {
		le.pos--
	}
}

func (le *LineEditor) MoveRight() {
	if le.pos < len(le.buf) {
		le.pos++
	}
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

func (le *LineEditor) MoveWordLeft() {
	if le.pos == 0 {
		return
	}
	i := le.pos
	for i > 0 && (le.buf[i-1] == ' ' || !isWordChar(le.buf[i-1])) {
		i--
	}
	for i > 0 && isWordChar(le.buf[i-1]) {
		i--
	}
	le.pos = i
}

func (le *LineEditor) MoveWordRight() {
	n := len(le.buf)
	if le.pos >= n {
		return
	}
	i := le.pos
	for i < n && isWordChar(le.buf[i]) {
		i++
	}
	for i < n && (le.buf[i] == ' ' || !isWordChar(le.buf[i])) {
		i++
	}
	le.pos = i
}

func (le *LineEditor) DeleteBack() bool {
	if le.pos > 0 {
		le.buf = append(le.buf[:le.pos-1], le.buf[le.pos:]...)
		le.pos--
		return true
	}
	return false
}

func (le *LineEditor) DeleteForward() bool {
	if le.pos < len(le.buf) {
		le.buf = append(le.buf[:le.pos], le.buf[le.pos+1:]...)
		return true
	}
	return false
}

func (le *LineEditor) DeleteToStart() {
	if le.pos > 0 {
		le.buf = le.buf[le.pos:]
		le.pos = 0
	}
}

func (le *LineEditor) DeleteToEnd() {
	if le.pos < len(le.buf) {
		le.buf = le.buf[:le.pos]
	}
}

func (le *LineEditor) DeleteWordBack() {
	if le.pos == 0 {
		return
	}
	i := le.pos
	for i > 0 && (le.buf[i-1] == ' ' || !isWordChar(le.buf[i-1])) {
		i--
	}
	for i > 0 && isWordChar(le.buf[i-1]) {
		i--
	}
	le.buf = append(le.buf[:i], le.buf[le.pos:]...)
	le.pos = i
}

func (le *LineEditor) DeleteWordForward() {
	n := len(le.buf)
	if le.pos >= n {
		return
	}
	i := le.pos
	for i < n && (le.buf[i] == ' ' || !isWordChar(le.buf[i])) {
		i++
	}
	for i < n && isWordChar(le.buf[i]) {
		i++
	}
	le.buf = append(le.buf[:le.pos], le.buf[i:]...)
}

func (le *LineEditor) Render(cursorStyle, resetStyle string) string {
	if len(le.buf) == 0 {
		return cursorStyle + " " + resetStyle
	}
	if le.pos >= len(le.buf) {
		return string(le.buf) + cursorStyle + " " + resetStyle
	}
	before := string(le.buf[:le.pos])
	at := string(le.buf[le.pos])
	after := string(le.buf[le.pos+1:])
	return before + cursorStyle + at + resetStyle + after
}

func parseEscapeSequence(reader *bufio.Reader) string {
	if reader.Buffered() == 0 {
		time.Sleep(25 * time.Millisecond)
		if reader.Buffered() == 0 {
			return "esc"
		}
	}

	b1, err := reader.ReadByte()
	if err != nil {
		return "esc"
	}

	switch b1 {
	case 'i', 'I':
		return "alt-i"
	case 'b', 'B':
		return "alt-b"
	case 'f', 'F':
		return "alt-f"
	case 'd', 'D':
		return "alt-d"
	case 8, 127:
		return "alt-backspace"
	case 'O':
		if reader.Buffered() > 0 {
			b2, _ := reader.ReadByte()
			if b2 == 'H' {
				return "home"
			}
			if b2 == 'F' {
				return "end"
			}
		}
		return "esc"
	case '[':
		if reader.Buffered() == 0 {
			time.Sleep(10 * time.Millisecond)
		}
		if reader.Buffered() == 0 {
			return "esc"
		}
		b2, _ := reader.ReadByte()
		switch b2 {
		case 'A':
			return "up"
		case 'B':
			return "down"
		case 'C':
			return "right"
		case 'D':
			return "left"
		case 'H':
			return "home"
		case 'F':
			return "end"
		case '1':
			if reader.Buffered() > 0 {
				b3, _ := reader.ReadByte()
				if b3 == '~' {
					return "home"
				}
				if b3 == ';' && reader.Buffered() >= 2 {
					b4, _ := reader.ReadByte()
					b5, _ := reader.ReadByte()
					if b4 == '5' {
						if b5 == 'D' {
							return "ctrl-left"
						}
						if b5 == 'C' {
							return "ctrl-right"
						}
					}
				}
			}
			return "home"
		case '3':
			if reader.Buffered() > 0 {
				b3, _ := reader.ReadByte()
				if b3 == '~' {
					return "delete"
				}
			}
			return "delete"
		case '4':
			if reader.Buffered() > 0 {
				b3, _ := reader.ReadByte()
				if b3 == '~' {
					return "end"
				}
			}
			return "end"
		default:
			for reader.Buffered() > 0 {
				_, _ = reader.ReadByte()
			}
			return "esc"
		}
	default:
		return "esc"
	}
}

func (le *LineEditor) HandleInput(b byte, reader *bufio.Reader) ReadlineAction {
	switch b {
	case 1: // Ctrl+A
		le.MoveHome()
		return RLContinue
	case 5: // Ctrl+E
		le.MoveEnd()
		return RLContinue
	case 2: // Ctrl+B
		le.MoveLeft()
		return RLContinue
	case 6: // Ctrl+F
		le.MoveRight()
		return RLContinue
	case 4: // Ctrl+D
		if len(le.buf) == 0 {
			return RLCancel
		}
		le.DeleteForward()
		return RLContinue
	case 11: // Ctrl+K
		le.DeleteToEnd()
		return RLContinue
	case 21: // Ctrl+U
		le.DeleteToStart()
		return RLContinue
	case 23: // Ctrl+W
		le.DeleteWordBack()
		return RLContinue
	case 8, 127: // Backspace, Ctrl+H
		le.DeleteBack()
		return RLContinue
	case '\r', '\n':
		return RLSubmit
	case '\t':
		return RLTab
	case 3: // Ctrl+C
		return RLCancel
	case 27: // Escape or Alt/Arrow sequence
		seq := parseEscapeSequence(reader)
		switch seq {
		case "esc":
			return RLCancel
		case "alt-i":
			return RLToggleCase
		case "alt-b", "ctrl-left":
			le.MoveWordLeft()
			return RLContinue
		case "alt-f", "ctrl-right":
			le.MoveWordRight()
			return RLContinue
		case "alt-d":
			le.DeleteWordForward()
			return RLContinue
		case "alt-backspace":
			le.DeleteWordBack()
			return RLContinue
		case "left":
			le.MoveLeft()
			return RLContinue
		case "right":
			le.MoveRight()
			return RLContinue
		case "home":
			le.MoveHome()
			return RLContinue
		case "end":
			le.MoveEnd()
			return RLContinue
		case "delete":
			le.DeleteForward()
			return RLContinue
		case "up":
			return RLArrowUp
		case "down":
			return RLArrowDown
		default:
			return RLContinue
		}
	default:
		if b >= 32 && b <= 126 {
			le.InsertRune(rune(b))
			return RLContinue
		}
		if b >= 0xc0 {
			_ = reader.UnreadByte()
			r, _, err := reader.ReadRune()
			if err == nil && r != utf8.RuneError {
				le.InsertRune(r)
				return RLContinue
			}
		}
		return RLContinue
	}
}
