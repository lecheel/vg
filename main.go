package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// --- Structs for WIG editor integration ---

type WigResultItem struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Char     int    `json:"char"`
	Text     string `json:"text"`
}

// --- Structs for Ripgrep JSON output parsing ---

type RgMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type RgMatchData struct {
	Path struct {
		Text string `json:"text"`
	} `json:"path"`
	Lines struct {
		Text string `json:"text"`
	} `json:"lines"`
	LineNumber   int `json:"line_number"`
	AbsoluteOff  int `json:"absolute_offset"`
	Submatches   []struct {
		Match struct {
			Text string `json:"text"`
		} `json:"match"`
		Start int `json:"start"`
		End   int `json:"end"`
	} `json:"submatches"`
}

// --- Structs for Search History (Scoped per project) ---

type SearchHistoryItem struct {
	Pattern   string `json:"pattern"`
	Timestamp int64  `json:"timestamp"`
	UseCount  int    `json:"use_count"`
}

// ProjectHistory maps repo/root path -> list of search items
type HistoryStore map[string][]SearchHistoryItem

// --- Helper paths ---

func getConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".config", "vgrep")
	os.MkdirAll(dir, 0755)
	return dir
}

func getWigSessionPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".config", "wig")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "rg_search.json")
}

func getHistoryPath() string {
	return filepath.Join(getConfigDir(), "history.json")
}

// --- Project Root Detector ---

func findProjectRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}

	markers := []string{".git", "go.mod", "Cargo.toml", "pubspec.yaml", "pyproject.toml", "package.json"}

	dir := cwd
	for {
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd
}

// --- History Management ---

func loadHistory() HistoryStore {
	store := make(HistoryStore)
	data, err := os.ReadFile(getHistoryPath())
	if err != nil {
		return store
	}
	json.Unmarshal(data, &store)
	return store
}

func saveHistory(store HistoryStore) {
	data, err := json.MarshalIndent(store, "", "  ")
	if err == nil {
		os.WriteFile(getHistoryPath(), data, 0644)
	}
}

func addSearchPattern(root, pattern string) {
	if pattern == "" {
		return
	}
	store := loadHistory()
	items := store[root]

	now := time.Now().Unix()
	found := false
	for i, item := range items {
		if item.Pattern == pattern {
			items[i].UseCount++
			items[i].Timestamp = now
			found = true
			break
		}
	}

	if !found {
		items = append(items, SearchHistoryItem{
			Pattern:   pattern,
			Timestamp: now,
			UseCount:  1,
		})
	}

	// Keep max 25 per project
	if len(items) > 25 {
		items = items[len(items)-25:]
	}
	store[root] = items
	saveHistory(store)
}

func formatRelativeTime(ts int64) string {
	diff := time.Now().Unix() - ts
	if diff < 60 {
		return "just now"
	} else if diff < 3600 {
		return fmt.Sprintf("%dm ago", diff/60)
	} else if diff < 86400 {
		return fmt.Sprintf("%dh ago", diff/3600)
	}
	return fmt.Sprintf("%dd ago", diff/86400)
}

// --- History Selector (fzf or interactive prompt) ---

func selectHistoryPattern(root string) string {
	store := loadHistory()
	items := store[root]
	if len(items) == 0 {
		fmt.Println("No previous search patterns recorded for this project.")
		return ""
	}

	// Sort by count desc, then timestamp desc
	sort.Slice(items, func(i, j int) bool {
		if items[i].UseCount == items[j].UseCount {
			return items[i].Timestamp > items[j].Timestamp
		}
		return items[i].UseCount > items[j].UseCount
	})

	if hasExecutable("fzf") {
		var menuLines []string
		for i, it := range items {
			menuLines = append(menuLines, fmt.Sprintf("%02d | [%d hits] (%s) %s", i+1, it.UseCount, formatRelativeTime(it.Timestamp), it.Pattern))
		}

		cmd := exec.Command("fzf", "--reverse", "--prompt=Project Searches> ", "--header=Project: "+filepath.Base(root))
		cmd.Stdin = strings.NewReader(strings.Join(menuLines, "\n"))
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			selected := strings.TrimSpace(out.String())
			if selected != "" {
				parts := strings.SplitN(selected, " | ", 2)
				if len(parts) == 2 {
					idx, _ := strconv.Atoi(parts[0])
					if idx > 0 && idx <= len(items) {
						return items[idx-1].Pattern
					}
				}
			}
		}
		return ""
	}

	// Fallback CLI menu
	fmt.Printf("\nSaved searches for [%s]:\n", filepath.Base(root))
	for i, it := range items {
		fmt.Printf("  \033[33m[%d]\033[0m \033[36m(%s)\033[0m %s\n", i+1, formatRelativeTime(it.Timestamp), it.Pattern)
	}
	fmt.Print("\nSelect index (Enter to cancel): ")
	var input string
	fmt.Scanln(&input)
	idx, err := strconv.Atoi(strings.TrimSpace(input))
	if err == nil && idx > 0 && idx <= len(items) {
		return items[idx-1].Pattern
	}
	return ""
}

// --- Ripgrep Execution & Output Parsing ---

func runRipgrep(pattern string, fileTypes []string, ignoreCase bool) ([]WigResultItem, error) {
	args := []string{"--json"}
	if ignoreCase {
		args = append(args, "-i")
	}

	for _, ft := range fileTypes {
		args = append(args, "-g", ft)
	}

	args = append(args, pattern)

	cmd := exec.Command("rg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ripgrep (is 'rg' installed?): %w", err)
	}

	var results []WigResultItem
	scanner := bufio.NewScanner(stdout)

	cwd, _ := os.Getwd()

	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		var msg RgMessage
		if err := json.Unmarshal(lineBytes, &msg); err != nil {
			continue
		}

		if msg.Type == "match" {
			var match RgMatchData
			if err := json.Unmarshal(msg.Data, &match); err == nil {
				filePath := match.Path.Text
				if !filepath.IsAbs(filePath) {
					filePath = filepath.Join(cwd, filePath)
				}

				charPos := 0
				if len(match.Submatches) > 0 {
					charPos = match.Submatches[0].Start
				}

				results = append(results, WigResultItem{
					FilePath: filePath,
					Line:     match.LineNumber,
					Char:     charPos,
					Text:     match.Lines.Text,
				})
			}
		}
	}

	_ = cmd.Wait() // rg returns exit code 1 if no matches found
	return results, nil
}

// --- Save Results into WIG Session ---

func writeWigSession(results []WigResultItem) error {
	sessionPath := getWigSessionPath()
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sessionPath, data, 0644)
}

func readWigSession() ([]WigResultItem, error) {
	sessionPath := getWigSessionPath()
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, err
	}
	var results []WigResultItem
	err = json.Unmarshal(data, &results)
	return results, err
}

// --- Editor Launching ---

func openEditor(item WigResultItem) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// Default preference chain: wig -> nvim -> vim
		if hasExecutable("wig") {
			editor = "wig"
		} else if hasExecutable("nvim") {
			editor = "nvim"
		} else {
			editor = "vim"
		}
	}

	var cmd *exec.Cmd
	if editor == "wig" {
		cmd = exec.Command("wig", item.FilePath, fmt.Sprintf("+%d", item.Line))
	} else {
		cmd = exec.Command(editor, fmt.Sprintf("+%d", item.Line), item.FilePath)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func hasExecutable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// --- CLI Display & Selection ---

func presentMatches(results []WigResultItem, pattern string, useFzf bool) {
	if len(results) == 0 {
		fmt.Printf("No matches found for %q\n", pattern)
		return
	}

	// Rule 1: Single match -> directly open
	if len(results) == 1 {
		fmt.Printf("1 match found. Opening %s:%d...\n", results[0].FilePath, results[0].Line)
		openEditor(results[0])
		return
	}

	// Rule 2: FZF selection if requested or if results are large (>50)
	if (useFzf || len(results) > 50) && hasExecutable("fzf") {
		var lines []string
		for i, r := range results {
			lines = append(lines, fmt.Sprintf("%03d | %s:%d:%d | %s", i+1, r.FilePath, r.Line, r.Char, strings.TrimSpace(r.Text)))
		}

		cmd := exec.Command("fzf", "--reverse", "--ansi", "--prompt=vgrep> ")
		cmd.Stdin = strings.NewReader(strings.Join(lines, "\n"))
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			selected := strings.TrimSpace(out.String())
			parts := strings.SplitN(selected, " | ", 2)
			if len(parts) == 2 {
				idx, _ := strconv.Atoi(parts[0])
				if idx > 0 && idx <= len(results) {
					openEditor(results[idx-1])
					return
				}
			}
		}
		return
	}

	// Rule 3: Formatted grouped terminal list
	fmt.Printf("\n\033[1;32mFound %d matches:\033[0m\n", len(results))
	lastFile := ""
	for i, r := range results {
		if lastFile != r.FilePath {
			fmt.Printf("\n\033[1;36m📁 %s\033[0m\n", r.FilePath)
			lastFile = r.FilePath
		}
		fmt.Printf("  \033[1;32m[%d]\033[0m \033[33m%4d:\033[0m %s", i+1, r.Line, r.Text)
	}

	fmt.Printf("\n\033[1;33mSelect an item to open (1-%d), or press Enter to exit:\033[0m\n\033[1;36mvgrep>\033[0m ", len(results))
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if idx, err := strconv.Atoi(input); err == nil && idx > 0 && idx <= len(results) {
		openEditor(results[idx-1])
	}
}

// --- Main Program ---

func main() {
	var (
		viewFlag       bool
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

	flag.BoolVar(&viewFlag, "v", false, "View last search results stored in wig session")
	flag.BoolVar(&viewFlag, "view", false, "View last search results stored in wig session")
	flag.BoolVar(&ignoreCaseFlag, "i", false, "Case-insensitive search")
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
		os.Remove(getHistoryPath())
		os.Remove(getWigSessionPath())
		fmt.Println("Cleared search history and wig session cache.")
		return
	}

	// 1. Review last search results (`-v` or `--view`)
	if viewFlag {
		results, err := readWigSession()
		if err != nil || len(results) == 0 {
			fmt.Println("No existing wig search session found.")
			return
		}
		presentMatches(results, "last session", fzfFlag)
		return
	}

	projectRoot := findProjectRoot()
	pattern := strings.Join(flag.Args(), " ")

	// 2. No pattern provided -> Open project-aware history
	if strings.TrimSpace(pattern) == "" {
		pattern = selectHistoryPattern(projectRoot)
		if pattern == "" {
			return
		}
	}

	// 3. Shorthand pattern conversion (e.g. "myFunc_fn" -> "func myFunc" / "fn myFunc")
	if strings.HasSuffix(pattern, "_fn") {
		base := strings.TrimSuffix(pattern, "_fn")
		if goFlag || _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			pattern = "func " + base
		} else {
			pattern = "fn " + base
		}
		fmt.Printf("🔍 Auto-converted function pattern: %q\n", pattern)
	}

	// 4. Determine file globs
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

	// 5. Execute ripgrep
	results, err := runRipgrep(pattern, fileTypes, ignoreCaseFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
		os.Exit(1)
	}

	// 6. Record pattern in project history & write wig session JSON
	addSearchPattern(projectRoot, pattern)
	if err := writeWigSession(results); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to sync wig session: %v\n", err)
	}

	// 7. Interactive Display & Jump
	presentMatches(results, pattern, fzfFlag)
}
