package model

import "encoding/json"

// WigResultItem represents an item formatted for the WIG editor.
type WigResultItem struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Char     int    `json:"char"`
	Text     string `json:"text"`
}

// RgMessage represents a raw message from ripgrep --json.
type RgMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// RgMatchData represents match data parsed from ripgrep.
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

// SearchHistoryItem stores pattern, timestamp, and hit counts.
type SearchHistoryItem struct {
	Pattern   string `json:"pattern"`
	Timestamp int64  `json:"timestamp"`
	UseCount  int    `json:"use_count"`
}

// HistoryStore maps repo/root path -> list of search items.
type HistoryStore map[string][]SearchHistoryItem

// DisplayEntry represents a line in the TUI list.
type DisplayEntry struct {
	IsHeader    bool
	FilePath    string
	ResultIdx   int // 0-based index in results, only valid if !IsHeader
	MatchItem   WigResultItem
	DisplayNum  int // 1-based match index
	LineInGroup int // 1-based index within the file
}

// FileGroup groups display entries by file.
type FileGroup struct {
	FilePath   string
	EntryIndex int // index in DisplayEntry slice
}