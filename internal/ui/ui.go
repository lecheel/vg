package ui

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
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

func OpenEditor(item model.WigResultItem) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		if config.HasExecutable("wig") {
			editor = "wig"
		} else if config.HasExecutable("nvim") {
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
		buf.WriteString(fmt.Sprintf("%sSEARCH>%s %s%s %s %s(Enter: search, Esc: cancel, Backspace: del, Ctrl+U: clear)%s%s",
			color.FgBoldCyan, color.Reset, newSearchText, color.CursorBlock, color.Reset, color.FgGray, color.Reset, color.ClearLine))
	} else if inFilterMode {
		buf.WriteString(fmt.Sprintf("%sFILTER>%s %s%s %s %s(Enter/Esc: done, Backspace: del, Ctrl+U: clear)%s%s",
			color.FgBoldYellow, color.Reset, filterText, color.CursorBlock, color.Reset, color.FgGray, color.Reset, color.ClearLine))
	} else if inReplaceMode {
		buf.WriteString(fmt.Sprintf("%sREPLACE>%s %s%s %s %s(Enter: apply, Tab: inspect list, Esc: cancel)%s%s",
			color.FgBoldMagenta, color.Reset, replaceText, color.CursorBlock, color.Reset, color.FgGray, color.Reset, color.ClearLine))
	} else if replaceText != "" {
		helpText := "[Tab/R:edit replace  Enter:apply  SPC:toggle  a:all  Esc:clear  e/o:open  q:quit]"
		buf.WriteString(fmt.Sprintf("%s%s%s%s", color.FgGray, TruncateDisplayWidthEnd(helpText, termWidth-2), color.Reset, color.ClearLine))
	} else {
		helpText := "[j/k:move  SPC:del line  n:new rg  R/Tab:replace  a:all  J/K:file  g/G:jump  pgup/dn  /:filter  e/o:open  q:quit]"
		buf.WriteString(fmt.Sprintf("%s%s%s%s", color.FgGray, TruncateDisplayWidthEnd(helpText, termWidth-2), color.Reset, color.ClearLine))
	}

	os.Stdout.Write(buf.Bytes())
}

func RunTUI(results []model.WigResultItem, searchPattern string, fileTypes []string, ignoreCase bool) {
	if len(results) == 0 {
		fmt.Printf("No matches found for %q\n", searchPattern)
		return
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
	newSearchText := ""

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

		RenderTUI(entries, groups, cursor, viewportStart, filter, searchPattern, fileTypes, ignoreCase, inFilterMode, inReplaceMode, replaceText, numBuffer, excluded, activeNotice, inSearchMode, newSearchText)

		b, err := reader.ReadByte()
		if err != nil {
			break
		}

		// Search Typing Mode
		if inSearchMode {
			if b == 27 || b == 3 {
				inSearchMode = false
				newSearchText = ""
				continue
			}

			if b == '\r' || b == '\n' {
				inSearchMode = false
				query := strings.TrimSpace(newSearchText)
				newSearchText = ""
				if query == "" {
					continue
				}

				newResults, err := search.RunRipgrep(query, fileTypes, ignoreCase)
				if err != nil {
					statusNotice = fmt.Sprintf("%s❌ Search error: %v%s", color.FgBoldRed, err, color.Reset)
					statusNoticeTime = time.Now()
					continue
				}

				results = newResults
				searchPattern = query
				filter = ""
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
			}

			if b == 127 || b == 8 {
				if len(newSearchText) > 0 {
					r := []rune(newSearchText)
					newSearchText = string(r[:len(r)-1])
				}
				continue
			}

			if b == 21 {
				newSearchText = ""
				continue
			}

			if b == 23 {
				trimmed := strings.TrimRight(newSearchText, " ")
				if idx := strings.LastIndex(trimmed, " "); idx != -1 {
					newSearchText = trimmed[:idx+1]
				} else {
					newSearchText = ""
				}
				continue
			}

			if b >= 32 && b <= 126 {
				newSearchText += string(b)
				continue
			}
			continue
		}

		// Replace Typing Mode
		if inReplaceMode {
			if b == 27 {
				inReplaceMode = false
				replaceText = ""
				continue
			}

			if b == '\t' {
				inReplaceMode = false
				continue
			}

			if b == '\r' || b == '\n' {
				inReplaceMode = false
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

			if b == 127 || b == 8 {
				if len(replaceText) > 0 {
					r := []rune(replaceText)
					replaceText = string(r[:len(r)-1])
				}
				continue
			}

			if b == 21 {
				replaceText = ""
				continue
			}

			if b >= 32 && b <= 126 {
				replaceText += string(b)
				continue
			}
			continue
		}

		// Filter Editing Mode
		if inFilterMode {
			if b == 27 {
				if reader.Buffered() >= 2 {
					b1, _ := reader.ReadByte()
					b2, _ := reader.ReadByte()
					if b1 == '[' {
						switch b2 {
						case 'A':
							if cursor > 0 {
								cursor--
							}
						case 'B':
							if cursor < len(entries)-1 {
								cursor++
							}
						}
					}
					continue
				}
				inFilterMode = false
				continue
			}

			if b == '\r' || b == '\n' {
				inFilterMode = false
				continue
			}

			if b == 127 || b == 8 {
				if len(filter) > 0 {
					filter = filter[:len(filter)-1]
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

			if b == 21 {
				filter = ""
				entries, groups = buildEntries(filter)
				cursor = 0
				continue
			}

			if b == 23 {
				trimmed := strings.TrimRight(filter, " ")
				if idx := strings.LastIndex(trimmed, " "); idx != -1 {
					filter = trimmed[:idx+1]
				} else {
					filter = ""
				}
				entries, groups = buildEntries(filter)
				if cursor >= len(entries) && len(entries) > 0 {
					cursor = len(entries) - 1
				}
				if cursor < 0 {
					cursor = 0
				}
				continue
			}

			if b >= 32 && b <= 126 {
				filter += string(b)
				entries, groups = buildEntries(filter)
				if cursor >= len(entries) && len(entries) > 0 {
					cursor = len(entries) - 1
				}
				if cursor < 0 {
					cursor = 0
				}
				continue
			}
			continue
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
				entries, groups = buildEntries(filter)
				cursor = 0
			}

		case 'n':
			inSearchMode = true
			newSearchText = ""
			replaceText = ""
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

		case 'K', 'h':
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
				if replaceText != "" {
					replaceText = ""
					continue
				}
			}
			if reader.Buffered() >= 2 {
				b1, _ := reader.ReadByte()
				b2, _ := reader.ReadByte()
				if b1 == '[' {
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

func PresentMatches(results []model.WigResultItem, pattern string, fileTypes []string, ignoreCase bool, useFzf bool) {
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

	RunTUI(results, pattern, fileTypes, ignoreCase)
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
