package ui

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"vgrep/internal/color"
	"vgrep/internal/config"
	"vgrep/internal/history"
	"vgrep/internal/model"
	"vgrep/internal/replace"
	"vgrep/internal/search"
)

func formatShortcuts(items [][2]string) string {
	var parts []string
	for _, it := range items {
		key := it[0]
		desc := it[1]
		if desc == "" {
			parts = append(parts, fmt.Sprintf("%s%s%s", color.FgGold, key, color.FgGray))
		} else {
			parts = append(parts, fmt.Sprintf("%s%s%s:%s", color.FgGold, key, color.FgGray, desc))
		}
	}
	return fmt.Sprintf("%s[%s%s]%s", color.FgGray, strings.Join(parts, "  "), color.FgGray, color.Reset)
}

func BuildEditorCommand(editor string, item model.WigResultItem) *exec.Cmd {
	baseEditor := filepath.Base(editor)
	line := item.Line
	if line < 1 {
		line = 1
	}
	col := item.Char + 1
	if col < 1 {
		col = 1
	}

	switch {
	case baseEditor == "wig":
		return exec.Command(editor, item.FilePath, fmt.Sprintf("+%d:%d", line, col))
	case strings.Contains(baseEditor, "nvim") || strings.Contains(baseEditor, "vim") || baseEditor == "vi":
		return exec.Command(editor, fmt.Sprintf("+call cursor(%d, %d)", line, col), item.FilePath)
	case baseEditor == "code":
		return exec.Command(editor, "-g", fmt.Sprintf("%s:%d:%d", item.FilePath, line, col))
	case baseEditor == "hx" || baseEditor == "helix":
		return exec.Command(editor, fmt.Sprintf("%s:%d:%d", item.FilePath, line, col))
	case baseEditor == "nano":
		return exec.Command(editor, fmt.Sprintf("+%d,%d", line, col), item.FilePath)
	case strings.HasPrefix(baseEditor, "emacs"):
		return exec.Command(editor, fmt.Sprintf("+%d:%d", line, col), item.FilePath)
	default:
		return exec.Command(editor, fmt.Sprintf("+%d", line), item.FilePath)
	}
}

func OpenEditor(item model.WigResultItem) error {
	editor := config.GetEditor()
	cmd := BuildEditorCommand(editor, item)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RenderTUI(
	entries []model.DisplayEntry,
	groups []model.FileGroup,
	cursor int,
	viewportStart int,
	filterText string,
	searchPattern string,
	fileTypes []string,
	ignoreCase bool,
	inFilterMode bool,
	inReplaceMode bool,
	replaceText string,
	numBuffer string,
	excluded map[int]bool,
	statusNotice string,
	inSearchMode bool,
	newSearchText string,
	fixedStrings bool,
	searchEditor *LineEditor,
	filterEditor *LineEditor,
	replaceEditor *LineEditor,
) {
	termHeight, termWidth := GetTerminalSize()
	if termHeight < 5 {
		termHeight = 5
	}
	if termWidth < 20 {
		termWidth = 20
	}

	maxRows := termHeight - 3
	if maxRows < 1 {
		maxRows = 1
	}

	var buf bytes.Buffer
	buf.WriteString(color.CursorHome)

	// 1. TITLE BAR
	badge := fmt.Sprintf("%s VGREP %s", color.BadgeVgrep, color.Reset)
	titleLeft := fmt.Sprintf("%s %s%s%s", badge, color.FgBoldWhite, searchPattern, color.Reset)
	leftWidth := 7 + 1 + StrDisplayWidth(searchPattern)

	if ignoreCase {
		titleLeft += fmt.Sprintf(" %s[-i]%s", color.FgGray, color.Reset)
		leftWidth += 5
	}
	if fixedStrings {
		titleLeft += fmt.Sprintf(" %s[-F]%s", color.FgGold, color.Reset)
		leftWidth += 5
	}
	if len(fileTypes) > 0 {
		typesStr := fmt.Sprintf("[%s]", strings.Join(fileTypes, ","))
		titleLeft += fmt.Sprintf(" %s%s%s", color.FgGray, typesStr, color.Reset)
		leftWidth += 1 + StrDisplayWidth(typesStr)
	}

	if inFilterMode {
		filterBadge := fmt.Sprintf("  %s/%s%s %s %s%s %s", color.FgBoldYellow, color.Reset, color.BadgeFilter, filterText, color.Reset, color.CursorBlock, color.Reset)
		titleLeft += filterBadge
		leftWidth += 6 + StrDisplayWidth(filterText)
	} else if filterText != "" {
		filterBadge := fmt.Sprintf("  %s/%s%s%s%s", color.FgBoldYellow, color.Reset, color.FgBoldCyan, filterText, color.Reset)
		titleLeft += filterBadge
		leftWidth += 3 + StrDisplayWidth(filterText)
	}

	matchCount := 0
	removedCount := 0
	for _, e := range entries {
		if !e.IsHeader {
			matchCount++
			if excluded[e.ResultIdx] {
				removedCount++
			}
		}
	}
	statsText := fmt.Sprintf("%d matches in %d files", matchCount, len(groups))
	if removedCount > 0 {
		statsText = fmt.Sprintf("%d matches (%d removed) in %d files", matchCount, removedCount, len(groups))
	}
	titleRight := fmt.Sprintf("%s%s%s", color.FgGray, statsText, color.Reset)
	rightWidth := StrDisplayWidth(statsText)

	maxTitleWidth := termWidth - 2
	spaceCount := maxTitleWidth - leftWidth - rightWidth
	if spaceCount > 0 {
		buf.WriteString(fmt.Sprintf("%s%s%s%s\r\n", titleLeft, strings.Repeat(" ", spaceCount), titleRight, color.ClearLine))
	} else {
		buf.WriteString(fmt.Sprintf("%s %s\r\n", titleLeft, color.ClearLine))
	}

	numWidth := len(strconv.Itoa(len(entries)))
	if numWidth < 2 {
		numWidth = 2
	}

	// 2. MATCH ENTRIES VIEWPORT
	for row := 0; row < maxRows; row++ {
		entryIdx := viewportStart + row
		if entryIdx >= len(entries) {
			if len(entries) == 0 && row == 0 {
				buf.WriteString(fmt.Sprintf("  %s(no matching results)%s%s\r\n", color.FgGray, color.Reset, color.ClearLine))
			} else {
				buf.WriteString(fmt.Sprintf("~%s\r\n", color.ClearLine))
			}
			continue
		}

		entry := entries[entryIdx]
		relNum := entryIdx - cursor
		if relNum < 0 {
			relNum = -relNum
		}

		cursorPrefix := "  "
		bgStyle := ""
		resetStyle := color.Reset
		if entryIdx == cursor {
			cursorPrefix = fmt.Sprintf("%s> %s", color.FgBoldGreen, color.Reset)
			bgStyle = color.BgDarkGray
		}

		relNumStr := fmt.Sprintf("%s%*d%s", color.FgGray, numWidth, relNum, color.Reset)
		if entryIdx == cursor {
			relNumStr = fmt.Sprintf("%s%*d%s", color.FgBoldYellow, numWidth, entryIdx+1, color.Reset)
		}

		if entry.IsHeader {
			allRemoved := true
			matchInFile := false
			for _, e := range entries {
				if !e.IsHeader && e.FilePath == entry.FilePath {
					matchInFile = true
					if !excluded[e.ResultIdx] {
						allRemoved = false
						break
					}
				}
			}

			prefixWidth := numWidth + 6
			maxPathLen := termWidth - prefixWidth - 2
			if maxPathLen < 10 {
				maxPathLen = 10
			}
			displayPath := config.ShortenHome(entry.FilePath)
			displayPath = TruncateDisplayWidthStart(displayPath, maxPathLen)

			if matchInFile && allRemoved {
				buf.WriteString(fmt.Sprintf("%s%s %s%s-%s %s%s%s%s\r\n",
					cursorPrefix, relNumStr, bgStyle, color.FgRed, color.StrikethroughDim, displayPath, color.Reset, resetStyle, color.ClearLine))
			} else {
				buf.WriteString(fmt.Sprintf("%s%s %s%s%s%s%s%s\r\n",
					cursorPrefix, relNumStr, bgStyle, color.FgBoldCyan, displayPath, color.Reset, resetStyle, color.ClearLine))
			}
		} else {
			isRemoved := excluded[entry.ResultIdx]
			cleanText := strings.ReplaceAll(entry.MatchItem.Text, "\t", "    ")
			cleanText = strings.TrimRight(cleanText, "\r\n")

			prefixWidth := numWidth + 12
			maxTextLen := termWidth - prefixWidth - 2
			if maxTextLen < 10 {
				maxTextLen = 10
			}
			cleanText = TruncateDisplayWidthEnd(cleanText, maxTextLen)

			if isRemoved {
				buf.WriteString(fmt.Sprintf("%s%s %s%s-%4d:%s %s%s%s%s\r\n",
					cursorPrefix, relNumStr, bgStyle, color.FgRed, entry.MatchItem.Line, color.StrikethroughDim, cleanText, color.Reset, resetStyle, color.ClearLine))
			} else {
				if inReplaceMode || replaceText != "" {
					if replaceText == "" {
						cleanText = HighlightText(cleanText, searchPattern, ignoreCase, color.HighlightMatch, bgStyle)
					} else {
						cleanText = replace.ReplacePattern(cleanText, searchPattern, fmt.Sprintf("%s%s%s%s", color.HighlightSubst, replaceText, color.Reset, bgStyle), ignoreCase)
					}
				} else {
					if searchPattern != "" {
						cleanText = HighlightText(cleanText, searchPattern, ignoreCase, color.HighlightSearch, bgStyle)
					}
					if filterText != "" {
						cleanText = HighlightText(cleanText, filterText, true, color.HighlightFilter, bgStyle)
					}
				}

				buf.WriteString(fmt.Sprintf("%s%s %s  %s%4d:%s %s%s%s\r\n",
					cursorPrefix, relNumStr, bgStyle, color.FgYellow, entry.MatchItem.Line, color.Reset, cleanText, resetStyle, color.ClearLine))
			}
		}
	}

	// 3. STATUS BAR
	modeBadge := fmt.Sprintf("%s NORMAL %s", color.BadgeNormal, color.StatusResetBg)
	modeWidth := 8
	if inSearchMode {
		modeBadge = fmt.Sprintf("%s SEARCH %s", color.BadgeSearch, color.StatusResetBg)
		modeWidth = 8
	} else if inFilterMode {
		modeBadge = fmt.Sprintf("%s FILTER %s", color.BadgeFilter, color.StatusResetBg)
		modeWidth = 8
	} else if inReplaceMode {
		modeBadge = fmt.Sprintf("%s REPLACE %s", color.BadgeReplace, color.StatusResetBg)
		modeWidth = 9
	} else if replaceText != "" {
		modeBadge = fmt.Sprintf("%s PREVIEW %s", color.BadgeReplace, color.StatusResetBg)
		modeWidth = 9
	}

	countBadge := ""
	countWidth := 0
	if numBuffer != "" {
		countBadge = fmt.Sprintf(" %s %s %s", color.BadgeReplace, numBuffer, color.StatusResetBg)
		countWidth = 3 + StrDisplayWidth(numBuffer)
	}

	locStr := " No selection"
	if statusNotice != "" {
		locStr = fmt.Sprintf(" %s", statusNotice)
	} else if len(entries) > 0 && cursor < len(entries) {
		current := entries[cursor]
		if current.IsHeader {
			locStr = fmt.Sprintf(" %s", config.ShortenHome(current.FilePath))
		} else {
			locStr = fmt.Sprintf(" %s:%d:%d", config.ShortenHome(current.FilePath), current.MatchItem.Line, current.MatchItem.Char+1)
			if excluded[current.ResultIdx] {
				locStr += fmt.Sprintf(" %s[REMOVED]%s", color.FgBoldRed, color.StatusResetBg)
			}
		}
	}

	pctStr := "[---]"
	if len(entries) > 0 {
		if len(entries) == 1 {
			pctStr = "[All]"
		} else if cursor == 0 {
			pctStr = "[Top]"
		} else if cursor == len(entries)-1 {
			pctStr = "[Bot]"
		} else {
			pct := (cursor * 100) / (len(entries) - 1)
			pctStr = fmt.Sprintf("[%2d%%]", pct)
		}
	}

	posStr := "0/0"
	if len(entries) > 0 {
		posStr = fmt.Sprintf("%d/%d", cursor+1, len(entries))
	}

	statusRight := fmt.Sprintf("%s %s", posStr, pctStr)
	statusRightWidth := StrDisplayWidth(statusRight)

	maxStatusWidth := termWidth - 2
	availForLoc := maxStatusWidth - modeWidth - countWidth - statusRightWidth - 3
	if availForLoc < 5 {
		locStr = ""
	} else {
		locStr = TruncateDisplayWidthStart(locStr, availForLoc)
	}

	statusLeft := fmt.Sprintf("%s%s%s", modeBadge, countBadge, locStr)
	statusLeftWidth := modeWidth + countWidth + StrDisplayWidth(locStr)

	statusSpaces := maxStatusWidth - statusLeftWidth - statusRightWidth
	if statusSpaces < 1 {
		statusSpaces = 1
	}

	buf.WriteString(fmt.Sprintf("%s%s%s%s %s%s\r\n",
		color.StatusBarBg, statusLeft, strings.Repeat(" ", statusSpaces), statusRight, color.ClearLine, color.Reset))

	// 4. COMMAND / HELP BAR
	if inSearchMode {
		promptLabel := fmt.Sprintf("%sSEARCH>%s", color.FgBoldCyan, color.Reset)
		if ignoreCase {
			promptLabel = fmt.Sprintf("%sSEARCH %s[-i]%s>%s", color.FgBoldCyan, color.FgGold, color.FgBoldCyan, color.Reset)
		}
		renderedInput := newSearchText + color.CursorBlock + " " + color.Reset
		if searchEditor != nil {
			renderedInput = searchEditor.Render(color.CursorBlock, color.Reset)
		}
		hints := fmt.Sprintf("(%sEnter%s: search, %sAlt+i%s: -i, %sEsc%s: cancel)",
			color.FgGold, color.FgGray, color.FgGold, color.FgGray, color.FgGold, color.FgGray)
		buf.WriteString(fmt.Sprintf("%s %s %s%s%s%s",
			promptLabel, renderedInput, color.FgGray, hints, color.Reset, color.ClearLine))
	} else if inFilterMode {
		renderedFilter := filterText + color.CursorBlock + " " + color.Reset
		if filterEditor != nil {
			renderedFilter = filterEditor.Render(color.CursorBlock, color.Reset)
		}
		hints := fmt.Sprintf("(%sEnter/Esc%s: done, %sCtrl+U%s: clear)",
			color.FgGold, color.FgGray, color.FgGold, color.FgGray)
		buf.WriteString(fmt.Sprintf("%sFILTER>%s %s %s%s%s%s",
			color.FgBoldYellow, color.Reset, renderedFilter, color.FgGray, hints, color.Reset, color.ClearLine))
	} else if inReplaceMode {
		promptLabel := fmt.Sprintf("%sREPLACE>%s", color.FgBoldMagenta, color.Reset)
		if ignoreCase {
			promptLabel = fmt.Sprintf("%sREPLACE %s[-i]%s>%s", color.FgBoldMagenta, color.FgGold, color.FgBoldMagenta, color.Reset)
		}
		renderedReplace := replaceText + color.CursorBlock + " " + color.Reset
		if replaceEditor != nil {
			renderedReplace = replaceEditor.Render(color.CursorBlock, color.Reset)
		}
		hints := fmt.Sprintf("(%sEnter%s: apply, %sTab%s: inspect, %sAlt+i%s: -i, %sEsc%s: cancel)",
			color.FgGold, color.FgGray, color.FgGold, color.FgGray, color.FgGold, color.FgGray, color.FgGold, color.FgGray)
		buf.WriteString(fmt.Sprintf("%s %s %s%s%s%s",
			promptLabel, renderedReplace, color.FgGray, hints, color.Reset, color.ClearLine))
	} else if replaceText != "" {
		items := [][2]string{
			{"Tab", "edit replace"},
			{"Enter", "apply"},
			{"SPC", "toggle"},
			{"a", "all"},
			{"Esc", "clear"},
			{"e/o", "open"},
			{"q", "quit"},
		}
		helpStr := formatShortcuts(items)
		buf.WriteString(fmt.Sprintf("%s%s", TruncateDisplayWidthEnd(helpStr, termWidth-2), color.ClearLine))
	} else {
		var items [][2]string
		if config.HasExecutable("rgr") {
			items = [][2]string{
				{"j/k", "move"},
				{"SPC", "del line"},
				{"n", "new rg"},
				{"F", "-F Literal/Regex"},
				{"r", "rgr"},
				{"Tab", "replace"},
				{"a", "all"},
				{"J/K", "file"},
				{"g/G", "jump"},
				{"/", "filter"},
				{"e/o", "open"},
				{"q", "quit"},
			}
		} else {
			items = [][2]string{
				{"j/k", "move"},
				{"SPC", "del line"},
				{"n", "new rg"},
				{"F", "-F"},
				{"Tab", "replace"},
				{"a", "all"},
				{"J/K", "file"},
				{"g/G", "jump"},
				{"/", "filter"},
				{"e/o", "open"},
				{"q", "quit"},
			}
		}
		helpStr := formatShortcuts(items)
		buf.WriteString(fmt.Sprintf("%s%s", TruncateDisplayWidthEnd(helpStr, termWidth-2), color.ClearLine))
	}

	os.Stdout.Write(buf.Bytes())
}

func RunTUI(results []model.WigResultItem, searchPattern string, fileTypes []string, ignoreCase bool, fixedStringsOpt ...bool) {
	if len(results) == 0 {
		fmt.Printf("No matches found for %q\n", searchPattern)
		return
	}

	fixedStrings := false
	if len(fixedStringsOpt) > 0 && fixedStringsOpt[0] {
		fixedStrings = true
	} else if config.LoadConfig().FixedStrings {
		fixedStrings = true
	}

	buildEntries := func(filter string) ([]model.DisplayEntry, []model.FileGroup) {
		var entries []model.DisplayEntry
		var groups []model.FileGroup
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
				groups = append(groups, model.FileGroup{
					FilePath:   r.FilePath,
					EntryIndex: len(entries),
				})
				entries = append(entries, model.DisplayEntry{
					IsHeader: true,
					FilePath: r.FilePath,
				})
				groupCount = 0
			}

			groupCount++
			entries = append(entries, model.DisplayEntry{
				IsHeader:    false,
				FilePath:    r.FilePath,
				ResultIdx:   i,
				MatchItem:   r,
				DisplayNum:  i + 1,
				LineInGroup: groupCount,
			})
		}
		return entries, groups
	}

	filter := ""
	entries, groups := buildEntries(filter)

	cursor := 0
	viewportStart := 0
	excluded := make(map[int]bool)

	inFilterMode := false
	inReplaceMode := false
	inSearchMode := false
	replaceText := ""

	searchEditor := NewLineEditor()
	filterEditor := NewLineEditor()
	replaceEditor := NewLineEditor()

	statusNotice := ""
	statusNoticeTime := time.Time{}

	EnterAlternateScreen()
	defer ExitAlternateScreen()

	_, err := SetRawTerminal()
	if err != nil {
		fmt.Println("Failed to initialize raw terminal UI.")
		return
	}
	defer RestoreTerminal()

	reader := bufio.NewReader(os.Stdin)
	var numBuffer string

	for {
		termHeight, _ := GetTerminalSize()
		maxRows := termHeight - 3
		if maxRows < 1 {
			maxRows = 1
		}

		if cursor < viewportStart {
			viewportStart = cursor
		} else if cursor >= viewportStart+maxRows {
			viewportStart = cursor - maxRows + 1
		}
		if len(entries) > 0 && viewportStart > len(entries)-maxRows {
			viewportStart = len(entries) - maxRows
		}
		if viewportStart < 0 {
			viewportStart = 0
		}

		activeNotice := ""
		if statusNotice != "" && time.Since(statusNoticeTime) < 4*time.Second {
			activeNotice = statusNotice
		}

		newSearchText := searchEditor.Text()
		RenderTUI(entries, groups, cursor, viewportStart, filter, searchPattern, fileTypes, ignoreCase, inFilterMode, inReplaceMode, replaceText, numBuffer, excluded, activeNotice, inSearchMode, newSearchText, fixedStrings, searchEditor, filterEditor, replaceEditor)

		b, err := reader.ReadByte()
		if err != nil {
			break
		}

		// Search Typing Mode
		if inSearchMode {
			action := searchEditor.HandleInput(b, reader)
			switch action {
			case RLQuit:
				RestoreTerminal()
				ExitAlternateScreen()
				return
			case RLCancel:
				inSearchMode = false
				searchEditor.Clear()
				continue
			case RLToggleCase:
				ignoreCase = !ignoreCase
				continue
			case RLSubmit:
				inSearchMode = false
				query := strings.TrimSpace(searchEditor.Text())
				searchEditor.Clear()
				if query == "" {
					continue
				}

				newResults, err := search.RunRipgrep(query, fileTypes, ignoreCase, fixedStrings)
				if err != nil {
					statusNotice = fmt.Sprintf("%s❌ Search error: %v%s", color.FgBoldRed, err, color.Reset)
					statusNoticeTime = time.Now()
					continue
				}

				results = newResults
				searchPattern = query
				filter = ""
				filterEditor.Clear()
				excluded = make(map[int]bool)
				cursor = 0
				viewportStart = 0
				entries, groups = buildEntries(filter)

				if len(results) > 0 {
					_ = search.WriteWigSession(results)
					history.AddSearchPattern(search.FindProjectRoot(), query)
					statusNotice = fmt.Sprintf("%s✓ Found %d matches for %q%s", color.FgBoldGreen, len(results), query, color.Reset)
				} else {
					statusNotice = fmt.Sprintf("%sNo matches found for %q%s", color.FgBoldYellow, query, color.Reset)
				}
				statusNoticeTime = time.Now()
				continue
			default:
				continue
			}
		}

		// Replace Typing Mode
		if inReplaceMode {
			action := replaceEditor.HandleInput(b, reader)
			switch action {
			case RLQuit:
				RestoreTerminal()
				ExitAlternateScreen()
				return
			case RLCancel:
				inReplaceMode = false
				replaceEditor.Clear()
				replaceText = ""
				continue
			case RLToggleCase:
				ignoreCase = !ignoreCase
				continue
			case RLTab:
				inReplaceMode = false
				replaceText = replaceEditor.Text()
				continue
			case RLSubmit:
				inReplaceMode = false
				replaceText = replaceEditor.Text()
				replacedCount, filesModified, err := replace.ApplyReplacement(results, excluded, searchPattern, replaceText, ignoreCase)
				if err != nil {
					statusNotice = fmt.Sprintf("%s❌ Replace error: %v%s", color.FgBoldRed, err, color.Reset)
				} else if replacedCount > 0 {
					statusNotice = fmt.Sprintf("%s✓ Replaced %d occurrences in %d files%s", color.FgBoldGreen, replacedCount, filesModified, color.Reset)
				} else {
					statusNotice = fmt.Sprintf("%sNo occurrences replaced%s", color.FgBoldYellow, color.Reset)
				}
				statusNoticeTime = time.Now()
				replaceText = ""
				replaceEditor.Clear()
				entries, groups = buildEntries(filter)
				continue
			default:
				replaceText = replaceEditor.Text()
				continue
			}
		}

		// Filter Editing Mode
		if inFilterMode {
			action := filterEditor.HandleInput(b, reader)
			switch action {
			case RLQuit:
				RestoreTerminal()
				ExitAlternateScreen()
				return
			case RLCancel, RLSubmit:
				inFilterMode = false
				continue
			case RLArrowUp:
				if cursor > 0 {
					cursor--
				}
				continue
			case RLArrowDown:
				if cursor < len(entries)-1 {
					cursor++
				}
				continue
			default:
				if filter != filterEditor.Text() {
					filter = filterEditor.Text()
					entries, groups = buildEntries(filter)
					if cursor >= len(entries) && len(entries) > 0 {
						cursor = len(entries) - 1
					}
					if cursor < 0 {
						cursor = 0
					}
				}
				continue
			}
		}

		// Normal Mode
		if b >= '0' && b <= '9' {
			if !(len(numBuffer) == 0 && b == '0') {
				numBuffer += string(b)
				continue
			}
		}

		count := 1
		hasCount := len(numBuffer) > 0
		if hasCount {
			if c, err := strconv.Atoi(numBuffer); err == nil && c > 0 {
				count = c
			}
			numBuffer = ""
		}

		switch b {
		case 'q', 3:
			RestoreTerminal()
			ExitAlternateScreen()
			return

		case 'c':
			if filter != "" {
				filter = ""
				filterEditor.Clear()
				entries, groups = buildEntries(filter)
				cursor = 0
			}

		case 'n':
			inSearchMode = true
			searchEditor.Clear()
			replaceText = ""
			replaceEditor.Clear()
			continue

		case 'F':
			fixedStrings = !fixedStrings
			newResults, err := search.RunRipgrep(searchPattern, fileTypes, ignoreCase, fixedStrings)
			if err != nil {
				statusNotice = fmt.Sprintf("%s❌ Ripgrep error: %v%s", color.FgBoldRed, err, color.Reset)
			} else {
				results = newResults
				entries, groups = buildEntries(filter)
				if cursor >= len(entries) && len(entries) > 0 {
					cursor = len(entries) - 1
				}
				if fixedStrings {
					statusNotice = fmt.Sprintf("%s✓ Literal mode enabled (-F)%s", color.FgBoldGreen, color.Reset)
				} else {
					statusNotice = fmt.Sprintf("%s✓ Regex mode enabled%s", color.FgBoldYellow, color.Reset)
				}
			}
			statusNoticeTime = time.Now()
			continue

		case ' ':
			if len(entries) == 0 || cursor >= len(entries) {
				continue
			}
			target := entries[cursor]
			if target.IsHeader {
				allRemoved := true
				var fileIndices []int
				for _, e := range entries {
					if !e.IsHeader && e.FilePath == target.FilePath {
						fileIndices = append(fileIndices, e.ResultIdx)
						if !excluded[e.ResultIdx] {
							allRemoved = false
						}
					}
				}
				for _, idx := range fileIndices {
					excluded[idx] = !allRemoved
				}
			} else {
				excluded[target.ResultIdx] = !excluded[target.ResultIdx]
				if cursor < len(entries)-1 {
					cursor++
				}
			}
			continue
		case 'r': // Launch external rgr replacer if available
			if config.HasExecutable("rgr") {
				RestoreTerminal()
				ExitAlternateScreen()

				_ = replace.RunReplacer(searchPattern, fileTypes, ignoreCase)

				EnterAlternateScreen()
				_, _ = SetRawTerminal()
				reader = bufio.NewReader(os.Stdin)
			}
			continue

		case 'R', '\t':
			inReplaceMode = true
			replaceEditor.SetText(replaceText)
			continue

		case 'a':
			allRemoved := true
			for i := range results {
				if !excluded[i] {
					allRemoved = false
					break
				}
			}
			for i := range results {
				excluded[i] = !allRemoved
			}
			continue

		case '/':
			inFilterMode = true
			filterEditor.SetText(filter)

		case 'j':
			cursor += count
			if cursor >= len(entries) && len(entries) > 0 {
				cursor = len(entries) - 1
			}

		case 'k':
			cursor -= count
			if cursor < 0 {
				cursor = 0
			}

		case 'J', 'l':
			for i := 0; i < count; i++ {
				found := false
				for _, g := range groups {
					targetIdx := g.EntryIndex
					if targetIdx+1 < len(entries) && !entries[targetIdx+1].IsHeader {
						targetIdx = targetIdx + 1
					}

					if targetIdx > cursor {
						cursor = targetIdx
						found = true
						break
					}
				}
				if !found && len(groups) > 0 {
					lastHeader := groups[len(groups)-1].EntryIndex
					if lastHeader+1 < len(entries) && !entries[lastHeader+1].IsHeader {
						cursor = lastHeader + 1
					} else {
						cursor = lastHeader
					}
				}
			}

		case 'K', 'L', 'h':
			for i := 0; i < count; i++ {
				found := false
				for idx := len(groups) - 1; idx >= 0; idx-- {
					g := groups[idx]
					targetIdx := g.EntryIndex
					if targetIdx+1 < len(entries) && !entries[targetIdx+1].IsHeader {
						targetIdx = targetIdx + 1
					}

					if targetIdx < cursor {
						cursor = targetIdx
						found = true
						break
					}
				}
				if !found && len(groups) > 0 {
					firstHeader := groups[0].EntryIndex
					if firstHeader+1 < len(entries) && !entries[firstHeader+1].IsHeader {
						cursor = firstHeader + 1
					} else {
						cursor = firstHeader
					}
				}
			}

		case 'g':
			if hasCount {
				cursor = count - 1
				if cursor >= len(entries) && len(entries) > 0 {
					cursor = len(entries) - 1
				}
				if cursor < 0 {
					cursor = 0
				}
			} else {
				cursor = 0
			}

		case 'G':
			if hasCount {
				cursor = count - 1
				if cursor >= len(entries) && len(entries) > 0 {
					cursor = len(entries) - 1
				}
				if cursor < 0 {
					cursor = 0
				}
			} else if len(entries) > 0 {
				cursor = len(entries) - 1
			}

		case 2, 21:
			cursor -= GetPageSize() * count
			if cursor < 0 {
				cursor = 0
			}

		case 6, 4:
			cursor += GetPageSize() * count
			if cursor >= len(entries) && len(entries) > 0 {
				cursor = len(entries) - 1
			}

		case '\r', '\n':
			if replaceText != "" {
				replacedCount, filesModified, err := replace.ApplyReplacement(results, excluded, searchPattern, replaceText, ignoreCase)
				if err != nil {
					statusNotice = fmt.Sprintf("%s❌ Replace error: %v%s", color.FgBoldRed, err, color.Reset)
				} else if replacedCount > 0 {
					statusNotice = fmt.Sprintf("%s✓ Replaced %d occurrences in %d files%s", color.FgBoldGreen, replacedCount, filesModified, color.Reset)
				} else {
					statusNotice = fmt.Sprintf("%sNo occurrences replaced%s", color.FgBoldYellow, color.Reset)
				}
				statusNoticeTime = time.Now()
				replaceText = ""
				entries, groups = buildEntries(filter)
				continue
			}
			fallthrough
		case 'o', 'e':
			if len(entries) == 0 || cursor >= len(entries) {
				continue
			}
			target := entries[cursor]

			RestoreTerminal()
			ExitAlternateScreen()

			if target.IsHeader {
				_ = OpenEditor(model.WigResultItem{
					FilePath: target.FilePath,
					Line:     1,
				})
			} else {
				_ = OpenEditor(target.MatchItem)
			}

			EnterAlternateScreen()
			_, _ = SetRawTerminal()
			reader = bufio.NewReader(os.Stdin)
			continue

		case 27:
			if reader.Buffered() == 0 {
				time.Sleep(20 * time.Millisecond)
				if reader.Buffered() == 0 {
					if replaceText != "" {
						replaceText = ""
						continue
					}
				}
			}
			if reader.Buffered() > 0 {
				b1, _ := reader.ReadByte()
				if b1 == 'q' || b1 == 'Q' {
					RestoreTerminal()
					ExitAlternateScreen()
					return
				}
				if b1 == 'i' || b1 == 'I' {
					ignoreCase = !ignoreCase
					newResults, err := search.RunRipgrep(searchPattern, fileTypes, ignoreCase, fixedStrings)
					if err == nil {
						results = newResults
						entries, groups = buildEntries(filter)
						if cursor >= len(entries) && len(entries) > 0 {
							cursor = len(entries) - 1
						}
						if ignoreCase {
							statusNotice = fmt.Sprintf("%s✓ Case-insensitive enabled (-i)%s", color.FgBoldGreen, color.Reset)
						} else {
							statusNotice = fmt.Sprintf("%s✓ Case-sensitive enabled%s", color.FgBoldYellow, color.Reset)
						}
						statusNoticeTime = time.Now()
					}
					continue
				}
				if b1 == '[' && reader.Buffered() > 0 {
					b2, _ := reader.ReadByte()
					switch b2 {
					case 'A':
						cursor -= count
						if cursor < 0 {
							cursor = 0
						}
					case 'B':
						cursor += count
						if cursor >= len(entries) && len(entries) > 0 {
							cursor = len(entries) - 1
						}
					case '5':
						if reader.Buffered() > 0 {
							b3, _ := reader.ReadByte()
							if b3 != '~' {
								_ = reader.UnreadByte()
							}
						}
						cursor -= GetPageSize() * count
						if cursor < 0 {
							cursor = 0
						}
					case '6':
						if reader.Buffered() > 0 {
							b3, _ := reader.ReadByte()
							if b3 != '~' {
								_ = reader.UnreadByte()
							}
						}
						cursor += GetPageSize() * count
						if cursor >= len(entries) && len(entries) > 0 {
							cursor = len(entries) - 1
						}
					}
				}
			}
		}
	}
}

func PresentMatches(results []model.WigResultItem, pattern string, fileTypes []string, ignoreCase bool, useFzf bool, fixedStringsOpt ...bool) {
	if len(results) == 0 {
		fmt.Printf("No matches found for %q\n", pattern)
		return
	}

	if len(results) == 1 {
		fmt.Printf("1 match found. Opening %s:%d...\n", results[0].FilePath, results[0].Line)
		_ = OpenEditor(results[0])
		return
	}

	if useFzf && config.HasExecutable("fzf") {
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
					_ = OpenEditor(results[idx-1])
					return
				}
			}
		}
		return
	}

	RunTUI(results, pattern, fileTypes, ignoreCase, fixedStringsOpt...)
}

func LaunchViewSession() string {
	results, err := search.ReadWigSession()
	if err != nil || len(results) == 0 {
		fmt.Println("No existing wig search session found.")
		return ""
	}
	PresentMatches(results, "last session", nil, false, false)
	return ""
}
