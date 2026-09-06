package replace

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"

	"vgrep/internal/config"
	"vgrep/internal/model"
	"vgrep/internal/search"
)

// FileBackup captures the raw bytes and permissions of a file before replacement.
type FileBackup struct {
	Path string
	Data []byte
	Perm os.FileMode
}

// ReplaceUndo stores backup data for a single replacement undo operation.
type ReplaceUndo struct {
	Files   []FileBackup
	Results []model.WigResultItem
}

var (
	undoMu   sync.Mutex
	lastUndo *ReplaceUndo
)

// CanUndo returns true if there is a previous replacement available to undo.
func CanUndo() bool {
	undoMu.Lock()
	defer undoMu.Unlock()
	return lastUndo != nil && len(lastUndo.Files) > 0
}

// ClearUndo clears any stored undo state.
func ClearUndo() {
	undoMu.Lock()
	defer undoMu.Unlock()
	lastUndo = nil
}

// Undo restores the files and results from the last replacement operation once.
func Undo() ([]model.WigResultItem, int, error) {
	undoMu.Lock()
	defer undoMu.Unlock()

	if lastUndo == nil || len(lastUndo.Files) == 0 {
		return nil, 0, errors.New("nothing to undo")
	}

	undo := lastUndo
	lastUndo = nil // Single-use undo: cannot undo more than once

	var lastErr error
	filesRestored := 0
	for _, fb := range undo.Files {
		if err := os.WriteFile(fb.Path, fb.Data, fb.Perm); err != nil {
			lastErr = err
		} else {
			filesRestored++
		}
	}

	_ = search.WriteWigSession(undo.Results)
	return undo.Results, filesRestored, lastErr
}

func RunReplacer(pattern string, fileTypes []string, ignoreCase bool) error {
	if !config.HasExecutable("rgr") {
		return nil
	}

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

	var backups []FileBackup
	savedResults := make([]model.WigResultItem, len(results))
	copy(savedResults, results)

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
				backups = append(backups, FileBackup{
					Path: filePath,
					Data: data,
					Perm: perm,
				})
			}
		}
	}

	if filesModified > 0 {
		undoMu.Lock()
		lastUndo = &ReplaceUndo{
			Files:   backups,
			Results: savedResults,
		}
		undoMu.Unlock()
	}

	_ = search.WriteWigSession(results)
	return replacedCount, filesModified, nil
}
