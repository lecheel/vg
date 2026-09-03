package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	fmt.Println("\033[1;36m=== vgrep Health Check ===\033[0m\n")

	// 1. Check Ripgrep (rg) - required
	if HasExecutable("rg") {
		rgPath, _ := exec.LookPath("rg")
		out, _ := exec.Command("rg", "--version").Output()
		ver := strings.Split(string(out), "\n")[0]
		fmt.Printf("  ✅ \033[1;32mripgrep (rg)\033[0m: %s (\033[90m%s\033[0m)\n", ver, ShortenHome(rgPath))
	} else {
		fmt.Println("  ❌ \033[1;31mripgrep (rg)\033[0m: NOT FOUND (Required for searching)")
	}

	// 2. Check fzf - optional
	if HasExecutable("fzf") {
		fzfPath, _ := exec.LookPath("fzf")
		fmt.Printf("  ✅ \033[1;32mfzf\033[0m: Found (\033[90m%s\033[0m)\n", ShortenHome(fzfPath))
	} else {
		fmt.Println("  ⚠️  \033[1;33mfzf\033[0m: NOT FOUND (Fuzzy history selection will use fallback numbered prompt)")
	}

	// 3. Check rgr (repgrep) - optional for 'r' key
	if HasExecutable("rgr") {
		rgrPath, _ := exec.LookPath("rgr")
		fmt.Printf("  ✅ \033[1;32mrgr (repgrep)\033[0m: Found (\033[90m%s\033[0m)\n", ShortenHome(rgrPath))
	} else {
		fmt.Println("  ⚠️  \033[1;33mrgr (repgrep)\033[0m: NOT FOUND ('r' find & replace action will be hidden)")
	}

	// 4. Check Editor
	editor := os.Getenv("EDITOR")
	if editor != "" {
		if HasExecutable(editor) {
			edPath, _ := exec.LookPath(editor)
			fmt.Printf("  ✅ \033[1;32m$EDITOR (%s)\033[0m: Found (\033[90m%s\033[0m)\n", editor, ShortenHome(edPath))
		} else {
			fmt.Printf("  ❌ \033[1;31m$EDITOR (%s)\033[0m: Configured but binary not found in $PATH\n", editor)
		}
	} else {
		foundFallback := false
		for _, ed := range []string{"wig", "nvim", "vim"} {
			if HasExecutable(ed) {
				edPath, _ := exec.LookPath(ed)
				fmt.Printf("  ✅ \033[1;32mEditor fallback (%s)\033[0m: Found (\033[90m%s\033[0m)\n", ed, ShortenHome(edPath))
				foundFallback = true
				break
			}
		}
		if !foundFallback {
			fmt.Println("  ❌ \033[1;31mEditor\033[0m: No editor found (set $EDITOR or install wig/nvim/vim)")
		}
	}

	// 5. Check paths
	fmt.Printf("\n  📁 \033[1;34mConfig path\033[0m:  %s\n", ShortenHome(GetConfigFilePath()))
	fmt.Printf("  📁 \033[1;34mHistory path\033[0m: %s\n", ShortenHome(GetHistoryPath()))
	fmt.Printf("  📁 \033[1;34mWig session\033[0m:  %s\n\n", ShortenHome(GetWigSessionPath()))
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
