package replace

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"vgrep/internal/config"
	"vgrep/internal/model"
	"vgrep/internal/search"
)

func RunReplacer(pattern string, fileTypes []string, ignoreCase bool) error {
	if config.HasExecutable("rgr") {
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

func ReplacePattern(line, pattern, replacement string, ignoreCase bool) string {
	if !ignoreCase {
		return strings.ReplaceAll(line, pattern, replacement)
	}
	lowerLine := strings.ToLower(line)
	lowerPattern := strings.ToLower(pattern)
	pLen := len(pattern)
	if pLen == 0 {
		return line
	}
	var buf strings.Builder
	lastIdx := 0
	for {
		idx := strings.Index(lowerLine[lastIdx:], lowerPattern)
		if idx == -1 {
			buf.WriteString(line[lastIdx:])
			break
		}
		buf.WriteString(line[lastIdx : lastIdx+idx])
		buf.WriteString(replacement)
		lastIdx = lastIdx + idx + pLen
	}
	return buf.String()
}

func ApplyReplacement(results []model.WigResultItem, excluded map[int]bool, pattern, replacement string, ignoreCase bool) (int, int, error) {
	var targetIndices []int
	for i := range results {
		if !excluded[i] {
			targetIndices = append(targetIndices, i)
		}
	}
	if len(targetIndices) == 0 {
		return 0, 0, nil
	}

	fileGroups := make(map[string][]int)
	for _, idx := range targetIndices {
		fileGroups[results[idx].FilePath] = append(fileGroups[results[idx].FilePath], idx)
	}

	replacedCount := 0
	filesModified := 0

	for filePath, indices := range fileGroups {
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		info, err := os.Stat(filePath)
		perm := os.FileMode(0644)
		if err == nil {
			perm = info.Mode()
		}

		hasCRLF := bytes.Contains(data, []byte("\r\n"))
		normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
		lines := strings.Split(normalized, "\n")

		fileModified := false
		for _, idx := range indices {
			lineIdx := results[idx].Line - 1
			if lineIdx >= 0 && lineIdx < len(lines) {
				orig := lines[lineIdx]
				newLine := ReplacePattern(orig, pattern, replacement, ignoreCase)
				if newLine != orig {
					lines[lineIdx] = newLine
					results[idx].Text = newLine
					replacedCount++
					fileModified = true
				}
			}
		}

		if fileModified {
			sep := "\n"
			if hasCRLF {
				sep = "\r\n"
			}
			newContent := strings.Join(lines, sep)
			if err := os.WriteFile(filePath, []byte(newContent), perm); err == nil {
				filesModified++
			}
		}
	}

	_ = search.WriteWigSession(results)
	return replacedCount, filesModified, nil
}
