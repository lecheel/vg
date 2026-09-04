package ui

import (
	"strings"
	"testing"

	"vgrep/internal/color"
	"vgrep/internal/model"
)

func TestBuildEditorCommand_ColumnPlacement(t *testing.T) {
	item := model.WigResultItem{
		FilePath: "/workspace/src/app.go",
		Line:     42,
		Char:     15, // 0-based index -> 16 1-based column
	}

	tests := []struct {
		name         string
		editor       string
		wantSubstr   string
		wantArgCount int
	}{
		{
			name:       "Wig editor uses +line:col",
			editor:     "wig",
			wantSubstr: "+42:16",
		},
		{
			name:       "Neovim uses cursor function",
			editor:     "nvim",
			wantSubstr: "+call cursor(42, 16)",
		},
		{
			name:       "Vim uses cursor function",
			editor:     "/usr/bin/vim",
			wantSubstr: "+call cursor(42, 16)",
		},
		{
			name:       "Vi uses cursor function",
			editor:     "vi",
			wantSubstr: "+call cursor(42, 16)",
		},
		{
			name:       "VS Code uses -g flag",
			editor:     "code",
			wantSubstr: "/workspace/src/app.go:42:16",
		},
		{
			name:       "Helix uses file:line:col",
			editor:     "hx",
			wantSubstr: "/workspace/src/app.go:42:16",
		},
		{
			name:       "Nano uses +line,col",
			editor:     "nano",
			wantSubstr: "+42,16",
		},
		{
			name:       "Emacs uses +line:col",
			editor:     "emacs",
			wantSubstr: "+42:16",
		},
		{
			name:       "Generic fallback uses +line",
			editor:     "customed",
			wantSubstr: "+42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := BuildEditorCommand(tt.editor, item)
			argsJoined := strings.Join(cmd.Args, " ")
			if !strings.Contains(argsJoined, tt.wantSubstr) {
				t.Errorf("BuildEditorCommand(%q) args = %q, expected substring %q", tt.editor, argsJoined, tt.wantSubstr)
			}
		})
	}
}

func TestBuildEditorCommand_Clamping(t *testing.T) {
	item := model.WigResultItem{
		FilePath: "/workspace/main.go",
		Line:     0,
		Char:     -5,
	}

	cmd := BuildEditorCommand("nvim", item)
	argsJoined := strings.Join(cmd.Args, " ")
	if !strings.Contains(argsJoined, "+call cursor(1, 1)") {
		t.Errorf("expected clamped cursor at (1, 1), got args %q", argsJoined)
	}
}

func TestFormatShortcuts_Styling(t *testing.T) {
	items := [][2]string{
		{"n", "new rg"},
		{"F", "-F"},
		{"q", "quit"},
	}

	formatted := formatShortcuts(items)

	// Verify gold color is applied to keys
	if !strings.Contains(formatted, color.FgGold+"n"+color.FgGray) {
		t.Errorf("expected gold styling on key 'n'")
	}
	if !strings.Contains(formatted, color.FgGold+"F"+color.FgGray) {
		t.Errorf("expected gold styling on key 'F'")
	}
	if !strings.Contains(formatted, ":-F") {
		t.Errorf("expected description ':-F'")
	}
}

func TestSearchPrompt_AltIToggleHint(t *testing.T) {
	// Verify that Alt+i hint uses gold styling
	goldAltI := color.FgGold + "Alt+i" + color.FgGray
	if !strings.Contains(goldAltI, "Alt+i") {
		t.Errorf("expected Alt+i in hint")
	}
}
