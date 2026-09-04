package search

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"vgrep/internal/config"
	"vgrep/internal/model"
)

func FindProjectRoot() string {
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

func RunRipgrep(pattern string, fileTypes []string, ignoreCase bool, fixedStringsOpt ...bool) ([]model.WigResultItem, error) {
	args := []string{"--json"}
	if ignoreCase {
		args = append(args, "-i")
	}

	isFixed := false
	if len(fixedStringsOpt) > 0 && fixedStringsOpt[0] {
		isFixed = true
	} else if config.LoadConfig().FixedStrings {
		isFixed = true
	}

	if isFixed {
		args = append(args, "-F")
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

	var results []model.WigResultItem
	scanner := bufio.NewScanner(stdout)
	cwd, _ := os.Getwd()

	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		var msg model.RgMessage
		if err := json.Unmarshal(lineBytes, &msg); err != nil {
			continue
		}

		if msg.Type == "match" {
			var match model.RgMatchData
			if err := json.Unmarshal(msg.Data, &match); err == nil {
				filePath := match.Path.Text
				if !filepath.IsAbs(filePath) {
					filePath = filepath.Join(cwd, filePath)
				}

				charPos := 0
				if len(match.Submatches) > 0 {
					charPos = match.Submatches[0].Start
				}

				results = append(results, model.WigResultItem{
					FilePath: filePath,
					Line:     match.LineNumber,
					Char:     charPos,
					Text:     match.Lines.Text,
				})
			}
		}
	}

	_ = cmd.Wait()
	return results, nil
}

func WriteWigSession(results []model.WigResultItem) error {
	sessionPath := config.GetWigSessionPath()
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sessionPath, data, 0644)
}

func ReadWigSession() ([]model.WigResultItem, error) {
	sessionPath := config.GetWigSessionPath()
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, err
	}
	var results []model.WigResultItem
	err = json.Unmarshal(data, &results)
	return results, err
}
