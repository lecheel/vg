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
	LineNumber  int `json:"line_number"`
	AbsoluteOff int `json:"absolute_offset"`
	Submatches  []struct {
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

// --- Launch external replace tool (rgr / repgrep / rg -r) ---

func runReplacer(pattern string, fileTypes []string, ignoreCase bool) error {
	// 1. Prefer 'rgr' (repgrep/ripgrep_replace) if installed
	if hasExecutable("rgr") {
		var args []string
		if ignoreCase {
			args = append(args, "-i")
		}
		for _, ft := range fileTypes {
			args = append(args, "-g", ft)
		}
		args = append(args, pattern)

		cmd := exec.Command("rgr", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// 2. Fallback: Prompt for replacement string and run rg -r preview
	fmt.Print("\033[1;36mEnter replacement string (rgr not found):\033[0m ")
	reader := bufio.NewReader(os.Stdin)
	replacement, _ := reader.ReadString('\n')
	replacement = strings.TrimSpace(replacement)

	var args []string
	args = append(args, "-r", replacement)
	if ignoreCase {
		args = append(args, "-i")
	}
	for _, ft := range fileTypes {
		args = append(args, "-g", ft)
	}
	args = append(args, pattern)

	fmt.Printf("\n\033[1;33m--- Replacement Preview (rg -r %q %q) ---\033[0m\n\n", replacement, pattern)
	cmd := exec.Command("rg", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

	fmt.Print("\n\033[90mPress Enter to return to vgrep...\033[0m")
	_, _ = reader.ReadString('\n')
	return nil
}

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

func shortenHome(path string) string {
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

func getHistoryPath() string {
	return filepath.Join(getConfigDir(), "history.json")
}

func getConfigFilePath() string {
	return filepath.Join(getConfigDir(), "config.toml")
}

func checkHealth() {
	fmt.Println("\033[1;36m=== vgrep Health Check ===\033[0m\n")

	// 1. Check Ripgrep (rg) - required
	if hasExecutable("rg") {
		rgPath, _ := exec.LookPath("rg")
		out, _ := exec.Command("rg", "--version").Output()
		ver := strings.Split(string(out), "\n")[0]
		fmt.Printf("  ✅ \033[1;32mripgrep (rg)\033[0m: %s (\033[90m%s\033[0m)\n", ver, shortenHome(rgPath))
	} else {
		fmt.Println("  ❌ \033[1;31mripgrep (rg)\033[0m: NOT FOUND (Required for searching)")
	}

	// 2. Check fzf - optional
	if hasExecutable("fzf") {
		fzfPath, _ := exec.LookPath("fzf")
		fmt.Printf("  ✅ \033[1;32mfzf\033[0m: Found (\033[90m%s\033[0m)\n", shortenHome(fzfPath))
	} else {
		fmt.Println("  ⚠️  \033[1;33mfzf\033[0m: NOT FOUND (Fuzzy history selection will use fallback numbered prompt)")
	}

	// 3. Check rgr (repgrep) - optional for 'r' key
	if hasExecutable("rgr") {
		rgrPath, _ := exec.LookPath("rgr")
		fmt.Printf("  ✅ \033[1;32mrgr (repgrep)\033[0m: Found (\033[90m%s\033[0m)\n", shortenHome(rgrPath))
	} else {
		fmt.Println("  ⚠️  \033[1;33mrgr (repgrep)\033[0m: NOT FOUND ('r' find & replace action will be hidden)")
	}

	// 4. Check Editor
	editor := os.Getenv("EDITOR")
	if editor != "" {
		if hasExecutable(editor) {
			edPath, _ := exec.LookPath(editor)
			fmt.Printf("  ✅ \033[1;32m$EDITOR (%s)\033[0m: Found (\033[90m%s\033[0m)\n", editor, shortenHome(edPath))
		} else {
			fmt.Printf("  ❌ \033[1;31m$EDITOR (%s)\033[0m: Configured but binary not found in $PATH\n", editor)
		}
	} else {
		// Fallback check
		foundFallback := false
		for _, ed := range []string{"wig", "nvim", "vim"} {
			if hasExecutable(ed) {
				edPath, _ := exec.LookPath(ed)
				fmt.Printf("  ✅ \033[1;32mEditor fallback (%s)\033[0m: Found (\033[90m%s\033[0m)\n", ed, shortenHome(edPath))
				foundFallback = true
				break
			}
		}
		if !foundFallback {
			fmt.Println("  ❌ \033[1;31mEditor\033[0m: No editor found (set $EDITOR or install wig/nvim/vim)")
		}
	}

	// 5. Check paths
	fmt.Printf("\n  📁 \033[1;34mConfig path\033[0m:  %s\n", shortenHome(getConfigFilePath()))
	fmt.Printf("  📁 \033[1;34mHistory path\033[0m: %s\n", shortenHome(getHistoryPath()))
	fmt.Printf("  📁 \033[1;34mWig session\033[0m:  %s\n\n", shortenHome(getWigSessionPath()))
}

func editConfig() error {
	configPath := getConfigFilePath()

	// Create config.toml with defaults if it doesn't exist
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
		if hasExecutable("wig") {
			editor = "wig"
		} else if hasExecutable("nvim") {
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
		// Even with no recorded patterns, still offer the view-last-session option.
		fmt.Println("No previous search patterns recorded for this project.")
		return launchViewSession()
	}

	// Sort by count desc, then timestamp desc
	sort.Slice(items, func(i, j int) bool {
		if items[i].UseCount == items[j].UseCount {
			return items[i].Timestamp > items[j].Timestamp
		}
		return items[i].UseCount > items[j].UseCount
	})

	const viewLabel = "vgrep view (like vgrep -v)"

	if hasExecutable("fzf") {
		var menuLines []string
		// Index 00 -> view last wig session (same as `vgrep -v`)
		menuLines = append(menuLines, fmt.Sprintf("00 | %s", viewLabel))
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
					if idx == 0 {
						return launchViewSession()
					}
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
	fmt.Printf("  \033[33m[0]\033[0m \033[36m%s\033[0m\n", viewLabel)
	for i, it := range items {
		fmt.Printf("  \033[33m[%d]\033[0m \033[36m(%s)\033[0m %s\n", i+1, formatRelativeTime(it.Timestamp), it.Pattern)
	}
	fmt.Print("\nSelect index (Enter to cancel): ")
	var input string
	fmt.Scanln(&input)
	idx, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return ""
	}
	if idx == 0 {
		return launchViewSession()
	}
	if idx > 0 && idx <= len(items) {
		return items[idx-1].Pattern
	}
	return ""
}

// launchViewSession re-opens the last wig search session results (like `vgrep -v`).
// It returns an empty pattern so the caller in main() exits without re-running rg.
func launchViewSession() string {
	results, err := readWigSession()
	if err != nil || len(results) == 0 {
		fmt.Println("No existing wig search session found.")
		return ""
	}
	presentMatches(results, "last session", nil, false, false)
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

// --- Interactive TUI with Relative Line Numbers & Vim Motions ---

type displayEntry struct {
	isHeader    bool
	filePath    string
	resultIdx   int // 0-based index in results, only valid if !isHeader
	matchItem   WigResultItem
	displayNum  int // 1-based match index for [n]
	lineInGroup int // 1-based index within the file
}

type fileGroup struct {
	filePath   string
	entryIndex int // index in displayEntries
}

func getTerminalHeight() int {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err == nil {
		parts := strings.Fields(string(out))
		if len(parts) >= 1 {
			if h, err := strconv.Atoi(parts[0]); err == nil && h > 0 {
				return h
			}
		}
	}
	return 24
}

func getPageSize() int {
	// Screen height minus title header (1 line) and bottom bar / prompt (3 lines)
	pageSize := getTerminalHeight() - 4
	if pageSize < 5 {
		pageSize = 5
	}
	return pageSize
}

func setRawTerminal() (*exec.Cmd, error) {
	// Disable echo and canonical mode
	rawCmd := exec.Command("stty", "raw", "-echo")
	rawCmd.Stdin = os.Stdin
	return rawCmd, rawCmd.Run()
}

func restoreTerminal() {
	cookedCmd := exec.Command("stty", "sane")
	cookedCmd.Stdin = os.Stdin
	_ = cookedCmd.Run()
	// Show cursor and reset text attributes
	fmt.Print("\033[0m\033[?25h")
}

func enterAlternateScreen() {
	// Enter alternate screen buffer & hide cursor
	fmt.Print("\033[?1049h\033[?25l")
}

func exitAlternateScreen() {
	// Exit alternate screen buffer, restore main screen & show cursor
	fmt.Print("\033[0m\033[?25h\033[?1049l")
}

func renderTUI(entries []displayEntry, cursor int, filterText string, searchPattern string, inFilterMode bool) {
	termHeight := getTerminalHeight()
	maxRows := termHeight - 4 // reserve space for header & prompt
	if maxRows < 5 {
		maxRows = 5
	}

	// Calculate scroll viewport window
	startIdx := 0
	if cursor >= maxRows {
		startIdx = cursor - maxRows + 1
	}
	endIdx := startIdx + maxRows
	if endIdx > len(entries) {
		endIdx = len(entries)
	}

	var buf bytes.Buffer
	// Reset cursor to top-left and clear display
	buf.WriteString("\033[H\033[2J")

	// Filter string with red cursor indicator when typing in filter mode
	filterDisplay := filterText
	if inFilterMode {
		filterDisplay = fmt.Sprintf("%s\033[41;1;37m \033[0m", filterText)
	}

	// Title / Filter Header
	buf.WriteString(fmt.Sprintf("\033[1;30;46m vgrep \033[0m \033[1;36mfilter:\033[0m %s\033[K\r\n", filterDisplay))

	for i := startIdx; i < endIdx; i++ {
		entry := entries[i]
		relNum := i - cursor
		if relNum < 0 {
			relNum = -relNum
		}

		// Cursor highlight indicator
		cursorPrefix := "  "
		bgStyle := ""
		resetStyle := "\033[0m"
		if i == cursor {
			cursorPrefix = "\033[1;32m> \033[0m"
			bgStyle = "\033[48;5;236m"
		}

		relNumStr := fmt.Sprintf("\033[90m%2d\033[0m", relNum)
		if i == cursor {
			relNumStr = "\033[1;33m 0\033[0m"
		}

		if entry.isHeader {
			buf.WriteString(fmt.Sprintf("%s%s %s\033[1;36m📁 %s\033[0m%s\033[K\r\n",
				cursorPrefix, relNumStr, bgStyle, shortenHome(entry.filePath), resetStyle))
		} else {
			cleanText := strings.TrimRight(entry.matchItem.Text, "\r\n")
			// Highlight search pattern in text
			if searchPattern != "" {
				cleanText = strings.ReplaceAll(cleanText, searchPattern, fmt.Sprintf("\033[1;31m%s\033[0m%s", searchPattern, bgStyle))
			}
			buf.WriteString(fmt.Sprintf("%s%s %s  \033[33m%4d:\033[0m %s%s\033[K\r\n",
				cursorPrefix, relNumStr, bgStyle, entry.matchItem.Line, cleanText, resetStyle))
		}
	}

	// Fill remaining blank rows if any
	for i := endIdx - startIdx; i < maxRows; i++ {
		buf.WriteString("~\033[K\r\n")
	}

	// Footer Help / Vim motion bar (hide 'r' if rgr is not found)
	if hasExecutable("rgr") {
		buf.WriteString("\033[K\r\n\033[90m[j/k, <num>j/k, J/K (files), g/G, pgup/pgdn, / (filter), r (rgr replace), Enter/o (open), q (quit)]\033[0m\033[K")
	} else {
		buf.WriteString("\033[K\r\n\033[90m[j/k, <num>j/k, J/K (files), g/G, pgup/pgdn, / (filter), Enter/o (open), q (quit)]\033[0m\033[K")
	}

	os.Stdout.Write(buf.Bytes())
}

func runTUI(results []WigResultItem, searchPattern string, fileTypes []string, ignoreCase bool) {
	if len(results) == 0 {
		fmt.Printf("No matches found for %q\n", searchPattern)
		return
	}

	// Prepare data entries
	buildEntries := func(filter string) ([]displayEntry, []fileGroup) {
		var entries []displayEntry
		var groups []fileGroup
		lastFile := ""
		groupCount := 0

		filterLower := strings.ToLower(filter)

		for i, r := range results {
			if filterLower != "" {
				matched := strings.Contains(strings.ToLower(r.FilePath), filterLower) ||
					strings.Contains(strings.ToLower(r.Text), filterLower)
				if !matched {
					continue
				}
			}

			if lastFile != r.FilePath {
				lastFile = r.FilePath
				groups = append(groups, fileGroup{
					filePath:   r.FilePath,
					entryIndex: len(entries),
				})
				entries = append(entries, displayEntry{
					isHeader: true,
					filePath: r.FilePath,
				})
				groupCount = 0
			}

			groupCount++
			entries = append(entries, displayEntry{
				isHeader:    false,
				filePath:    r.FilePath,
				resultIdx:   i,
				matchItem:   r,
				displayNum:  i + 1,
				lineInGroup: groupCount,
			})
		}
		return entries, groups
	}

	filter := ""
	entries, groups := buildEntries(filter)
	if len(entries) == 0 {
		fmt.Printf("No matches for filter %q\n", filter)
		return
	}

	cursor := 0

	enterAlternateScreen()
	defer exitAlternateScreen()

	_, err := setRawTerminal()
	if err != nil {
		fmt.Println("Failed to initialize raw terminal UI.")
		return
	}
	defer restoreTerminal()

	reader := bufio.NewReader(os.Stdin)
	var numBuffer string
	inFilterMode := false

	for {
		renderTUI(entries, cursor, filter, searchPattern, inFilterMode)

		b, err := reader.ReadByte()
		if err != nil {
			break
		}

		// --- Filter Editing Mode (activated with /) ---
		if inFilterMode {
			if b == '\r' || b == '\n' || b == 27 { // Enter or Esc exits filter mode
				inFilterMode = false
			} else if b == 127 || b == 8 { // Backspace
				if len(filter) > 0 {
					filter = filter[:len(filter)-1]
					entries, groups = buildEntries(filter)
					if cursor >= len(entries) {
						cursor = len(entries) - 1
					}
					if cursor < 0 {
						cursor = 0
					}
				}
			} else if b >= 32 && b <= 126 { // Normal typing
				filter += string(b)
				entries, groups = buildEntries(filter)
				if cursor >= len(entries) {
					cursor = len(entries) - 1
				}
				if cursor < 0 {
					cursor = 0
				}
			}
			continue
		}

		// --- Normal Vim Motions Mode ---

		// 1. Handle Count prefix (e.g., 3j, 12k)
		if b >= '0' && b <= '9' {
			if !(len(numBuffer) == 0 && b == '0') { // prevent leading 0
				numBuffer += string(b)
				continue
			}
		}

		count := 1
		if len(numBuffer) > 0 {
			if c, err := strconv.Atoi(numBuffer); err == nil && c > 0 {
				count = c
			}
			numBuffer = ""
		}

		switch b {
		case 'q', 3: // 'q' or Ctrl+C to quit
			restoreTerminal()
			exitAlternateScreen()
			return

		case 'r': // Launch rgr (find & replace) on searchPattern (only active if rgr exists)
			if hasExecutable("rgr") {
				restoreTerminal()
				exitAlternateScreen()

				_ = runReplacer(searchPattern, fileTypes, ignoreCase)

				// Resume TUI
				enterAlternateScreen()
				_, _ = setRawTerminal()
				reader = bufio.NewReader(os.Stdin)
			}
			continue

		case '/': // Enter filter search mode
			inFilterMode = true

		case 'j': // Move down by count
			cursor += count
			if cursor >= len(entries) {
				cursor = len(entries) - 1
			}

		case 'k': // Move up by count
			cursor -= count
			if cursor < 0 {
				cursor = 0
			}

		case 'J', 'l': // Next file: land on first match line (Shift+J)
			for i := 0; i < count; i++ {
				found := false
				for _, g := range groups {
					// Target is the first match under the next header (g.entryIndex + 1)
					targetIdx := g.entryIndex
					if targetIdx+1 < len(entries) && !entries[targetIdx+1].isHeader {
						targetIdx = targetIdx + 1
					}

					if targetIdx > cursor {
						cursor = targetIdx
						found = true
						break
					}
				}
				if !found && len(groups) > 0 {
					lastHeader := groups[len(groups)-1].entryIndex
					if lastHeader+1 < len(entries) && !entries[lastHeader+1].isHeader {
						cursor = lastHeader + 1
					} else {
						cursor = lastHeader
					}
				}
			}

		case 'K', 'L': // Prev file: land on first match line (Shift+K)
			for i := 0; i < count; i++ {
				found := false
				for idx := len(groups) - 1; idx >= 0; idx-- {
					g := groups[idx]
					targetIdx := g.entryIndex
					if targetIdx+1 < len(entries) && !entries[targetIdx+1].isHeader {
						targetIdx = targetIdx + 1
					}

					if targetIdx < cursor {
						cursor = targetIdx
						found = true
						break
					}
				}
				if !found && len(groups) > 0 {
					firstHeader := groups[0].entryIndex
					if firstHeader+1 < len(entries) && !entries[firstHeader+1].isHeader {
						cursor = firstHeader + 1
					} else {
						cursor = firstHeader
					}
				}
			}

		case 'g': // Jump to top (including line 0 / first file header)
			cursor = 0

		case 'G': // Bottom of list
			if len(entries) > 0 {
				cursor = len(entries) - 1
			}

		case 2, 21: // Ctrl+B, Ctrl+U: Page Up (screen height minus title and bar)
			cursor -= getPageSize() * count
			if cursor < 0 {
				cursor = 0
			}

		case 6, 4: // Ctrl+F, Ctrl+D: Page Down (screen height minus title and bar)
			cursor += getPageSize() * count
			if cursor >= len(entries) {
				cursor = len(entries) - 1
			}

		case '\r', '\n', 'o': // Open selection in editor and return back to TUI on quit
			if len(entries) == 0 || cursor >= len(entries) {
				continue
			}
			target := entries[cursor]

			// 1. Temporarily restore cooked terminal mode before handing over to editor
			restoreTerminal()
			exitAlternateScreen()

			if target.isHeader {
				_ = openEditor(WigResultItem{
					FilePath: target.filePath,
					Line:     1,
				})
			} else {
				_ = openEditor(target.matchItem)
			}

			// 2. Re-enter alternate screen buffer & raw terminal mode to resume TUI
			enterAlternateScreen()
			_, _ = setRawTerminal()
			reader = bufio.NewReader(os.Stdin) // Re-create reader for fresh stdin state
			continue

		case 27: // Handle ANSI Arrow Keys & Page Up/Down
			if reader.Buffered() >= 2 {
				b1, _ := reader.ReadByte()
				b2, _ := reader.ReadByte()
				if b1 == '[' {
					switch b2 {
					case 'A': // Up
						cursor -= count
						if cursor < 0 {
							cursor = 0
						}
					case 'B': // Down
						cursor += count
						if cursor >= len(entries) {
							cursor = len(entries) - 1
						}
					case '5': // Page Up (\x1b[5~)
						if reader.Buffered() > 0 {
							b3, _ := reader.ReadByte()
							if b3 != '~' {
								_ = reader.UnreadByte()
							}
						}
						cursor -= getPageSize() * count
						if cursor < 0 {
							cursor = 0
						}
					case '6': // Page Down (\x1b[6~)
						if reader.Buffered() > 0 {
							b3, _ := reader.ReadByte()
							if b3 != '~' {
								_ = reader.UnreadByte()
							}
						}
						cursor += getPageSize() * count
						if cursor >= len(entries) {
							cursor = len(entries) - 1
						}
					}
				}
			}
		}
	}
}

// --- CLI Display & Selection ---

func presentMatches(results []WigResultItem, pattern string, fileTypes []string, ignoreCase bool, useFzf bool) {
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

	// Rule 2: FZF selection if explicitly forced
	if useFzf && hasExecutable("fzf") {
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

	// Rule 3: Interactive Relative TUI with Vim Motions (3j/3k, J/K file jump, / filter, r replace)
	runTUI(results, pattern, fileTypes, ignoreCase)
}

// --- Main Program ---

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
		os.Remove(getHistoryPath())
		os.Remove(getWigSessionPath())
		fmt.Println("Cleared search history and wig session cache.")
		return
	}

	// Health check (`--health`)
	if healthFlag {
		checkHealth()
		return
	}

	// Edit config.toml (`-e` or `--edit`)
	if editFlag {
		if err := editConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to edit config: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 1. Review last search results (`-v` or `--view`)
	if viewFlag {
		results, err := readWigSession()
		if err != nil || len(results) == 0 {
			fmt.Println("No existing wig search session found.")
			return
		}
		presentMatches(results, "last session", nil, ignoreCaseFlag, fzfFlag)
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
	presentMatches(results, pattern, fileTypes, ignoreCaseFlag, fzfFlag)
}
