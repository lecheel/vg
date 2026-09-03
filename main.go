package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vgrep/internal/config"
	"vgrep/internal/history"
	"vgrep/internal/search"
	"vgrep/internal/ui"
)

func main() {
	var (
		viewFlag       bool
		editFlag       bool
		healthFlag     bool
		ignoreCaseFlag bool
		fzfFlag        bool
		initFlag       bool
		goFlag         bool
		rsFlag         bool
		pyFlag         bool
		cFlag          bool
		dartFlag       bool
		swiftFlag      bool
	)

	flag.BoolVar(&viewFlag, "v", false, "")
	flag.BoolVar(&viewFlag, "view", false, "View last search results stored in wig session")
	flag.BoolVar(&editFlag, "e", false, "")
	flag.BoolVar(&editFlag, "edit", false, "Edit config.toml in $EDITOR")
	flag.BoolVar(&healthFlag, "health", false, "Check installed tools and environment health")
	flag.BoolVar(&ignoreCaseFlag, "i", false, "")
	flag.BoolVar(&ignoreCaseFlag, "ignore-case", false, "Case-insensitive search")
	flag.BoolVar(&fzfFlag, "fzf", false, "Force fzf picker")
	flag.BoolVar(&initFlag, "init", false, "Clear history and session")

	// Language filters
	flag.BoolVar(&goFlag, "go", false, "Search Go files (*.go)")
	flag.BoolVar(&rsFlag, "rs", false, "Search Rust files (*.rs)")
	flag.BoolVar(&pyFlag, "py", false, "Search Python files (*.py)")
	flag.BoolVar(&cFlag, "cc", false, "Search C/C++ files")
	flag.BoolVar(&dartFlag, "dart", false, "Search Dart files (*.dart)")
	flag.BoolVar(&swiftFlag, "swift", false, "Search Swift files (*.swift)")

	flag.Parse()

	if initFlag {
		_ = os.Remove(config.GetHistoryPath())
		_ = os.Remove(config.GetWigSessionPath())
		fmt.Println("Cleared search history and wig session cache.")
		return
	}

	if healthFlag {
		config.CheckHealth()
		return
	}

	if editFlag {
		if err := config.EditConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to edit config: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if viewFlag {
		results, err := search.ReadWigSession()
		if err != nil || len(results) == 0 {
			fmt.Println("No existing wig search session found.")
			return
		}
		ui.PresentMatches(results, "last session", nil, ignoreCaseFlag, fzfFlag)
		return
	}

	projectRoot := search.FindProjectRoot()
	pattern := strings.Join(flag.Args(), " ")

	if strings.TrimSpace(pattern) == "" {
		pattern = history.SelectHistoryPattern(projectRoot, func() string {
			return ui.LaunchViewSession()
		})
		if pattern == "" {
			return
		}
	}

	if strings.HasSuffix(pattern, "_fn") {
		base := strings.TrimSuffix(pattern, "_fn")
		isGoProject := false
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			isGoProject = true
		}
		if goFlag || isGoProject {
			pattern = "func " + base
		} else {
			pattern = "fn " + base
		}
		fmt.Printf("🔍 Auto-converted function pattern: %q\n", pattern)
	}

	var fileTypes []string
	if goFlag {
		fileTypes = append(fileTypes, "*.go")
	}
	if rsFlag {
		fileTypes = append(fileTypes, "*.rs")
	}
	if pyFlag {
		fileTypes = append(fileTypes, "*.py")
	}
	if cFlag {
		fileTypes = append(fileTypes, "*.c", "*.cpp", "*.cc", "*.h", "*.hpp")
	}
	if dartFlag {
		fileTypes = append(fileTypes, "*.dart")
	}
	if swiftFlag {
		fileTypes = append(fileTypes, "*.swift")
	}

	results, err := search.RunRipgrep(pattern, fileTypes, ignoreCaseFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
		os.Exit(1)
	}

	history.AddSearchPattern(projectRoot, pattern)
	if err := search.WriteWigSession(results); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to sync wig session: %v\n", err)
	}

	ui.PresentMatches(results, pattern, fileTypes, ignoreCaseFlag, fzfFlag)
}
