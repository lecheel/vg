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

	"vgrep/internal/config"
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
	buf.WriteString("\033[H")

	// 1. TITLE BAR
	badge := "\033[1;30;46m VGREP \033[0m"
	titleLeft := fmt.Sprintf("%s \033[1;37m%s\033[0m", badge, searchPattern)
	leftWidth := 7 + 1 + StrDisplayWidth(searchPattern)

	if ignoreCase {
		titleLeft += " \033[90m[-i]\033[0m"
		leftWidth += 5
	}
	if len(fileTypes) > 0 {
		typesStr := fmt.Sprintf("[%s]", strings.Join(fileTypes, ","))
		titleLeft += fmt.Sprintf(" \033[90m%s\033[0m", typesStr)
		leftWidth += 1 + StrDisplayWidth(typesStr)
	}

	if inFilterMode {
		filterBadge := fmt.Sprintf("  \033[1;33m/\033[0m\033[1;30;43m %s \033[0m\033[41;1;37m \033[0m", filterText)
		titleLeft += filterBadge
		leftWidth += 6 + StrDisplayWidth(filterText)
	} else if filterText != "" {
		filterBadge := fmt.Sprintf("  \033[1;33m/\033[0m\033[1;36m%s\033[0m", filterText)
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
	titleRight := fmt.Sprintf("\033[90m%s\033[0m", statsText)
	rightWidth := StrDisplayWidth(statsText)

	maxTitleWidth := termWidth - 2
	spaceCount := maxTitleWidth - leftWidth - rightWidth
	if spaceCount > 0 {
		buf.WriteString(fmt.Sprintf("%s%s%s\033[K\r\n", titleLeft, strings.Repeat(" ", spaceCount), titleRight))
	} else {
		buf.WriteString(fmt.Sprintf("%s \033[K\r\n", titleLeft))
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
				buf.WriteString("  \033[90m(no matching results)\033[0m\033[K\r\n")
			} else {
				buf.WriteString("~\033[K\r\n")
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
		resetStyle := "\033[0m"
		if entryIdx == cursor {
			cursorPrefix = "\033[1;32m> \033[0m"
			bgStyle = "\033[48;5;236m"
		}

		relNumStr := fmt.Sprintf("\033[90m%*d\033[0m", numWidth, relNum)
		if entryIdx == cursor {
			relNumStr = fmt.Sprintf("\033[1;33m%*d\033[0m", numWidth, entryIdx+1)
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
				buf.WriteString(fmt.Sprintf("%s%s %s\033[31m-\033[90;9m 📁 %s\033[0m%s\033[K\r\n",
					cursorPrefix, relNumStr, bgStyle, displayPath, resetStyle))
			} else {
				buf.WriteString(fmt.Sprintf("%s%s %s\033[1;36m📁 %s\033[0m%s\033[K\r\n",
					cursorPrefix, relNumStr, bgStyle, displayPath, resetStyle))
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
				buf.WriteString(fmt.Sprintf("%s%s %s\033[31m-%4d:\033[90;9m %s\033[0m%s\033[K\r\n",
					cursorPrefix, relNumStr, bgStyle, entry.MatchItem.Line, cleanText, resetStyle))
			} else {
				if inReplaceMode || replaceText != "" {
					if replaceText == "" {
						cleanText = HighlightText(cleanText, searchPattern, ignoreCase, "\033[1;30;43m", bgStyle)
					} else {
						cleanText = replace.ReplacePattern(cleanText, searchPattern, fmt.Sprintf("\033[1;30;42m%s\033[0m%s", replaceText, bgStyle), ignoreCase)
					}
				} else {
					if searchPattern != "" {
						cleanText = HighlightText(cleanText, searchPattern, ignoreCase, "\033[1;31m", bgStyle)
					}
					if filterText != "" {
						cleanText = HighlightText(cleanText, filterText, true, "\033[1;33;4m", bgStyle)
					}
				}

				buf.WriteString(fmt.Sprintf("%s%s %s  \033[33m%4d:\033[0m %s%s\033[K\r\n",
					cursorPrefix, relNumStr, bgStyle, entry.MatchItem.Line, cleanText, resetStyle))
			}
		}
	}

	// 3. STATUS BAR
	modeBadge := "\033[1;30;42m NORMAL \033[0;48;5;236;37m"
	modeWidth := 8
	if inFilterMode {
		modeBadge = "\033[1;30;43m FILTER \033[0;48;5;236;37m"
		modeWidth = 8
	} else if inReplaceMode {
		modeBadge = "\033[1;30;45m REPLACE \033[0;48;5;236;37m"
		modeWidth = 9
	} else if replaceText != "" {
		modeBadge = "\033[1;30;45m PREVIEW \033[0;48;5;236;37m"
		modeWidth = 9
	}

	countBadge := ""
	countWidth := 0
	if numBuffer != "" {
		countBadge = fmt.Sprintf(" \033[1;30;45m %s \033[0;48;5;236;37m", numBuffer)
		countWidth = 3 + StrDisplayWidth(numBuffer)
	}

	locStr := " No selection"
	if statusNotice != "" {
		locStr = fmt.Sprintf(" %s", statusNotice)
	} else if len(entries) > 0 && cursor < len(entries) {
		current := entries[cursor]
		if current.IsHeader {
			locStr = fmt.Sprintf(" 📁 %s", config.ShortenHome(current.FilePath))
		} else {
			locStr = fmt.Sprintf(" 📁 %s:%d:%d", config.ShortenHome(current.FilePath), current.MatchItem.Line, current.MatchItem.Char+1)
			if excluded[current.ResultIdx] {
				locStr += " \033[1;31m[REMOVED]\033[0;48;5;236;37m"
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

	buf.WriteString(fmt.Sprintf("\033[48;5;236;37m%s%s%s \033[K\033[0m\r\n",
		statusLeft, strings.Repeat(" ", statusSpaces), statusRight))

	// 4. COMMAND / HELP BAR
	if inFilterMode {
		buf.WriteString(fmt.Sprintf("\033[1;33mFILTER>\033[0m %s\033[41;1;37m \033[0m \033[90m(Enter/Esc: done, Backspace: del, Ctrl+U: clear)\033[0m\033[K", filterText))
	} else if inReplaceMode {
		buf.WriteString(fmt.Sprintf("\033[1;35mREPLACE>\033[0m %s\033[41;1;37m \033[0m \033[90m(Enter: apply, Tab: inspect list, Esc: cancel)\033[0m\033[K", replaceText))
	} else if replaceText != "" {
		helpText := "[Tab/R:edit replace  Enter:apply  SPC:toggle  a:all  Esc:clear  e/o:open  q:quit]"
		buf.WriteString(fmt.Sprintf("\033[90m%s\033[0m\033[K", TruncateDisplayWidthEnd(helpText, termWidth-2)))
	} else {
		helpText := "[j/k:move  SPC:del line  R/Tab:replace  a:all  J/K:file  g/G:jump  pgup/dn  /:filter  e/o:open  q:quit]"
		buf.WriteString(fmt.Sprintf("\033[90m%s\033[0m\033[K", TruncateDisplayWidthEnd(helpText, termWidth-2)))
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
	replaceText := ""

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

		RenderTUI(entries, groups, cursor, viewportStart, filter, searchPattern, fileTypes, ignoreCase, inFilterMode, inReplaceMode, replaceText, numBuffer, excluded, activeNotice)

		b, err := reader.ReadByte()
		if err != nil {
			break
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
					statusNotice = fmt.Sprintf("\033[1;31m❌ Replace error: %v\033[0m", err)
				} else if replacedCount > 0 {
					statusNotice = fmt.Sprintf("\033[1;32m✓ Replaced %d occurrences in %d files\033[0m", replacedCount, filesModified)
				} else {
					statusNotice = "\033[1;33mNo occurrences replaced\033[0m"
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
					statusNotice = fmt.Sprintf("\033[1;31m❌ Replace error: %v\033[0m", err)
				} else if replacedCount > 0 {
					statusNotice = fmt.Sprintf("\033[1;32m✓ Replaced %d occurrences in %d files\033[0m", replacedCount, filesModified)
				} else {
					statusNotice = "\033[1;33mNo occurrences replaced\033[0m"
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