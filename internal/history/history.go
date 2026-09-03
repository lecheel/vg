package history

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"vgrep/internal/config"
	"vgrep/internal/model"
)

func LoadHistory() model.HistoryStore {
	store := make(model.HistoryStore)
	data, err := os.ReadFile(config.GetHistoryPath())
	if err != nil {
		return store
	}
	_ = json.Unmarshal(data, &store)
	return store
}

func SaveHistory(store model.HistoryStore) {
	data, err := json.MarshalIndent(store, "", "  ")
	if err == nil {
		_ = os.WriteFile(config.GetHistoryPath(), data, 0644)
	}
}

func AddSearchPattern(root, pattern string) {
	if pattern == "" {
		return
	}
	store := LoadHistory()
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
		items = append(items, model.SearchHistoryItem{
			Pattern:   pattern,
			Timestamp: now,
			UseCount:  1,
		})
	}

	if len(items) > 25 {
		items = items[len(items)-25:]
	}
	store[root] = items
	SaveHistory(store)
}

func FormatRelativeTime(ts int64) string {
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

func SelectHistoryPattern(root string, launchView func() string) string {
	store := LoadHistory()
	items := store[root]
	if len(items) == 0 {
		fmt.Println("No previous search patterns recorded for this project.")
		if launchView != nil {
			return launchView()
		}
		return ""
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].UseCount == items[j].UseCount {
			return items[i].Timestamp > items[j].Timestamp
		}
		return items[i].UseCount > items[j].UseCount
	})

	const viewLabel = "vgrep view (like vgrep -v)"

	if config.HasExecutable("fzf") {
		var menuLines []string
		menuLines = append(menuLines, fmt.Sprintf("00 | %s", viewLabel))
		for i, it := range items {
			menuLines = append(menuLines, fmt.Sprintf("%02d | [%d hits] (%s) %s", i+1, it.UseCount, FormatRelativeTime(it.Timestamp), it.Pattern))
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
					if idx == 0 && launchView != nil {
						return launchView()
					}
					if idx > 0 && idx <= len(items) {
						return items[idx-1].Pattern
					}
				}
			}
		}
		return ""
	}

	fmt.Printf("\nSaved searches for [%s]:\n", filepath.Base(root))
	fmt.Printf("  \033[33m[0]\033[0m \033[36m%s\033[0m\n", viewLabel)
	for i, it := range items {
		fmt.Printf("  \033[33m[%d]\033[0m \033[36m(%s)\033[0m %s\n", i+1, FormatRelativeTime(it.Timestamp), it.Pattern)
	}
	fmt.Print("\nSelect index (Enter to cancel): ")
	var input string
	_, _ = fmt.Scanln(&input)
	idx, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return ""
	}
	if idx == 0 && launchView != nil {
		return launchView()
	}
	if idx > 0 && idx <= len(items) {
		return items[idx-1].Pattern
	}
	return ""
}
