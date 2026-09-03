package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"vgrep/internal/color"
)

func GetConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".config", "vgrep")
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func GetWigSessionPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".config", "wig")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "rg_search.json")
}

func ShortenHome(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		if path == home {
			return "~"
		}
		if strings.HasPrefix(path, home+string(filepath.Separator)) {
			return "~" + path[len(home):]
		}
	}
	return path
}

func GetHistoryPath() string {
	return filepath.Join(GetConfigDir(), "history.json")
}

func GetConfigFilePath() string {
	return filepath.Join(GetConfigDir(), "config.toml")
}

func HasExecutable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func CheckHealth() {
	fmt.Printf("%s=== vgrep Health Check ===%s\n\n", color.FgBoldCyan, color.Reset)

	// 1. Check Ripgrep (rg) - required
	if HasExecutable("rg") {
		rgPath, _ := exec.LookPath("rg")
		out, _ := exec.Command("rg", "--version").Output()
		ver := strings.Split(string(out), "\n")[0]
		fmt.Printf("  ✅ %sripgrep (rg)%s: %s (%s%s%s)\n", color.FgBoldGreen, color.Reset, ver, color.FgGray, ShortenHome(rgPath), color.Reset)
	} else {
		fmt.Printf("  ❌ %sripgrep (rg)%s: NOT FOUND (Required for searching)\n", color.FgBoldRed, color.Reset)
	}

	// 2. Check fzf - optional
	if HasExecutable("fzf") {
		fzfPath, _ := exec.LookPath("fzf")
		fmt.Printf("  ✅ %sfzf%s: Found (%s%s%s)\n", color.FgBoldGreen, color.Reset, color.FgGray, ShortenHome(fzfPath), color.Reset)
	} else {
		fmt.Printf("  ⚠️  %sfzf%s: NOT FOUND (Fuzzy history selection will use fallback numbered prompt)\n", color.FgBoldYellow, color.Reset)
	}

	// 3. Check rgr (repgrep) - optional for 'r' key
	if HasExecutable("rgr") {
		rgrPath, _ := exec.LookPath("rgr")
		fmt.Printf("  ✅ %srgr (repgrep)%s: Found (%s%s%s)\n", color.FgBoldGreen, color.Reset, color.FgGray, ShortenHome(rgrPath), color.Reset)
	} else {
		fmt.Printf("  ⚠️  %srgr (repgrep)%s: NOT FOUND ('r' find & replace action will be hidden)\n", color.FgBoldYellow, color.Reset)
	}

	// 4. Check Editor
	editor := os.Getenv("EDITOR")
	if editor != "" {
		if HasExecutable(editor) {
			edPath, _ := exec.LookPath(editor)
			fmt.Printf("  ✅ %s$EDITOR (%s)%s: Found (%s%s%s)\n", color.FgBoldGreen, editor, color.Reset, color.FgGray, ShortenHome(edPath), color.Reset)
		} else {
			fmt.Printf("  ❌ %s$EDITOR (%s)%s: Configured but binary not found in $PATH\n", color.FgBoldRed, editor, color.Reset)
		}
	} else {
		foundFallback := false
		for _, ed := range []string{"wig", "nvim", "vim"} {
			if HasExecutable(ed) {
				edPath, _ := exec.LookPath(ed)
				fmt.Printf("  ✅ %sEditor fallback (%s)%s: Found (%s%s%s)\n", color.FgBoldGreen, ed, color.Reset, color.FgGray, ShortenHome(edPath), color.Reset)
				foundFallback = true
				break
			}
		}
		if !foundFallback {
			fmt.Printf("  ❌ %sEditor%s: No editor found (set $EDITOR or install wig/nvim/vim)\n", color.FgBoldRed, color.Reset)
		}
	}

	// 5. Check paths
	fmt.Printf("\n  %sConfig path%s:  %s\n", color.FgBoldBlue, color.Reset, ShortenHome(GetConfigFilePath()))
	fmt.Printf("  %sHistory path%s: %s\n", color.FgBoldBlue, color.Reset, ShortenHome(GetHistoryPath()))
	fmt.Printf("  %sWig session%s:  %s\n\n", color.FgBoldBlue, color.Reset, ShortenHome(GetWigSessionPath()))
}

func EditConfig() error {
	configPath := GetConfigFilePath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultContent := `# vgrep configuration file
editor = "wig"
# session_file = "~/.config/wig/rg_search.json"
`
		if err := os.WriteFile(configPath, []byte(defaultContent), 0644); err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		if HasExecutable("wig") {
			editor = "wig"
		} else if HasExecutable("nvim") {
			editor = "nvim"
		} else {
			editor = "vim"
		}
	}

	cmd := exec.Command(editor, configPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
