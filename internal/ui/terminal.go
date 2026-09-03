package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func GetTerminalSize() (int, int) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err == nil {
		parts := strings.Fields(string(out))
		if len(parts) >= 2 {
			h, errH := strconv.Atoi(parts[0])
			w, errW := strconv.Atoi(parts[1])
			if errH == nil && errW == nil && h > 0 && w > 0 {
				return h, w
			}
		}
	}
	h := 24
	w := 80
	if envLines := os.Getenv("LINES"); envLines != "" {
		if val, err := strconv.Atoi(envLines); err == nil && val > 0 {
			h = val
		}
	}
	if envCols := os.Getenv("COLUMNS"); envCols != "" {
		if val, err := strconv.Atoi(envCols); err == nil && val > 0 {
			w = val
		}
	}
	return h, w
}

func GetTerminalHeight() int {
	h, _ := GetTerminalSize()
	return h
}

func GetPageSize() int {
	h, _ := GetTerminalSize()
	pageSize := h - 3
	if pageSize < 1 {
		pageSize = 1
	}
	return pageSize
}

func SetRawTerminal() (*exec.Cmd, error) {
	rawCmd := exec.Command("stty", "raw", "-echo")
	rawCmd.Stdin = os.Stdin
	return rawCmd, rawCmd.Run()
}

func RestoreTerminal() {
	cookedCmd := exec.Command("stty", "sane")
	cookedCmd.Stdin = os.Stdin
	_ = cookedCmd.Run()
	fmt.Print("\033[0m\033[?25h")
}

func EnterAlternateScreen() {
	fmt.Print("\033[?1049h\033[?25l\033[2J")
}

func ExitAlternateScreen() {
	fmt.Print("\033[0m\033[?25h\033[?1049l")
}

func RuneWidth(r rune) int {
	if r == 0 {
		return 0
	}
	if r < 32 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	if (r >= 0x1f300 && r <= 0x1faff) || (r >= 0x2600 && r <= 0x27bf) {
		return 2
	}
	if (r >= 0x1100 && r <= 0x115f) ||
		(r >= 0x2e80 && r <= 0xa4cf) ||
		(r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) ||
		(r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) {
		return 2
	}
	return 1
}

func StrDisplayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += RuneWidth(r)
	}
	return w
}

func TruncateDisplayWidthEnd(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if StrDisplayWidth(s) <= maxWidth {
		return s
	}
	target := maxWidth - 1
	if target <= 0 {
		return "…"
	}
	curW := 0
	var runes []rune
	for _, r := range s {
		rw := RuneWidth(r)
		if curW+rw > target {
			break
		}
		curW += rw
		runes = append(runes, r)
	}
	return string(runes) + "…"
}

func TruncateDisplayWidthStart(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if StrDisplayWidth(s) <= maxWidth {
		return s
	}
	target := maxWidth - 1
	if target <= 0 {
		return "…"
	}
	runes := []rune(s)
	curW := 0
	startIdx := len(runes)
	for i := len(runes) - 1; i >= 0; i-- {
		rw := RuneWidth(runes[i])
		if curW+rw > target {
			break
		}
		curW += rw
		startIdx = i
	}
	return "…" + string(runes[startIdx:])
}

func HighlightText(text, pattern string, ignoreCase bool, highlightStyle, baseStyle string) string {
	if pattern == "" || text == "" {
		return text
	}
	if ignoreCase {
		lowerText := strings.ToLower(text)
		lowerPattern := strings.ToLower(pattern)
		var buf strings.Builder
		lastIdx := 0
		pLen := len(pattern)
		for {
			idx := strings.Index(lowerText[lastIdx:], lowerPattern)
			if idx == -1 {
				buf.WriteString(text[lastIdx:])
				break
			}
			matchStart := lastIdx + idx
			matchEnd := matchStart + pLen
			buf.WriteString(text[lastIdx:matchStart])
			buf.WriteString(highlightStyle)
			buf.WriteString(text[matchStart:matchEnd])
			buf.WriteString("\033[0m")
			buf.WriteString(baseStyle)
			lastIdx = matchEnd
		}
		return buf.String()
	}
	return strings.ReplaceAll(text, pattern, fmt.Sprintf("%s%s\033[0m%s", highlightStyle, pattern, baseStyle))
}