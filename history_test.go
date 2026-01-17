package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseHistoryFile(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		content        string
		wantCmdCount   int
		checkFirstCmd  string
		checkLastCmd   string
		checkTimestamp float64
	}{
		{
			name: "simple commands",
			content: `: 1704384000:0;ls -la
: 1704384015:5;docker build -t app .
: 1704384020:0;git commit -m "initial commit"`,
			wantCmdCount:   3,
			checkFirstCmd:  "ls -la",
			checkLastCmd:   `git commit -m "initial commit"`,
			checkTimestamp: 1704384015,
		},
		{
			name: "multiline command",
			content: `: 1704384000:0;cat > file.txt << 'EOF'
line 1
line 2
line 3
EOF
: 1704384010:0;ls -la`,
			wantCmdCount:   2,
			checkFirstCmd:  "cat > file.txt << 'EOF'",
			checkLastCmd:   "ls -la",
			checkTimestamp: 1704384010,
		},
		{
			name: "same timestamp commands",
			content: `: 1704384000:0;cmd1
: 1704384000:0;cmd2
: 1704384000:0;cmd3
: 1704384001:0;cmd4`,
			wantCmdCount:   4,
			checkFirstCmd:  "cmd1",
			checkLastCmd:   "cmd4",
			checkTimestamp: 1704384001,
		},
		{
			name: "empty lines",
			content: `
: 1704384000:0;cmd1

: 1704384010:0;cmd2

`,
			wantCmdCount:   2,
			checkFirstCmd:  "cmd1",
			checkLastCmd:   "cmd2",
			checkTimestamp: 1704384010,
		},
		{
			name: "continuation lines only",
			content: `: 1704384000:0;echo "line 1
line 2
line 3"`,
			wantCmdCount:   1,
			checkFirstCmd:  `echo "line 1`,
			checkLastCmd:   `echo "line 1`,
			checkTimestamp: 1704384000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			historyFile := filepath.Join(tmpDir, tt.name+".hist")
			if err := os.WriteFile(historyFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write history file: %v", err)
			}

			history, err := ParseHistoryFile(historyFile)
			if err != nil {
				t.Fatalf("ParseHistoryFile() error = %v", err)
			}

			if len(history.Commands) != tt.wantCmdCount {
				t.Errorf("got %d commands, want %d", len(history.Commands), tt.wantCmdCount)
			}

			if len(history.Commands) > 0 {
				firstCmd := history.Commands[0].Command
				lines := strings.Split(firstCmd, "\n")
				if lines[0] != tt.checkFirstCmd {
					t.Errorf("first command = %q, want %q", lines[0], tt.checkFirstCmd)
				}

				lastCmd := history.Commands[len(history.Commands)-1].Command
				lastLines := strings.Split(lastCmd, "\n")
				if lastLines[0] != tt.checkLastCmd {
					t.Errorf("last command = %q, want %q", lastLines[0], tt.checkLastCmd)
				}
			}
		})
	}
}

func TestAddSubsecondTimestamps(t *testing.T) {
	input := History{
		Commands: []Command{
			{Timestamp: 1000, Command: "cmd1"},
			{Timestamp: 1000, Command: "cmd2"},
			{Timestamp: 1000, Command: "cmd3"},
			{Timestamp: 2000, Command: "cmd4"},
			{Timestamp: 2000, Command: "cmd5"},
			{Timestamp: 3000, Command: "cmd6"},
		},
	}

	result := addSubsecondTimestamps(input)

	tests := []struct {
		wantTs  float64
		wantCmd string
	}{
		{1000.000, "cmd1"},
		{1000.001, "cmd2"},
		{1000.002, "cmd3"},
		{2000.000, "cmd4"},
		{2000.001, "cmd5"},
		{3000.000, "cmd6"},
	}

	for i, tt := range tests {
		if result.Commands[i].Timestamp != tt.wantTs {
			t.Errorf("Commands[%d].Timestamp = %v, want %v", i, result.Commands[i].Timestamp, tt.wantTs)
		}
		if result.Commands[i].Command != tt.wantCmd {
			t.Errorf("Commands[%d].Command = %q, want %q", i, result.Commands[i].Command, tt.wantCmd)
		}
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		timestamp float64
		notWant   string
	}{
		{"zero", 0.0, ""},
		{"with milliseconds", 1704384000.123, ""},
		{"round", 1704384000.0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTimestamp(tt.timestamp)
			if got == tt.notWant {
				t.Errorf("FormatTimestamp(%v) returned empty string", tt.timestamp)
			}
			if len(got) != len("2006-01-02 15:04:05") {
				t.Errorf("FormatTimestamp(%v) = %v, want format YYYY-MM-DD HH:MM:SS", tt.timestamp, got)
			}
		})
	}
}

func TestParseHistoryFile_Duration(t *testing.T) {
	tmpDir := t.TempDir()

	content := `: 1704384000:5;sleep 5
: 1704384010:0;ls -la
: 1704384020:10;make`

	historyFile := filepath.Join(tmpDir, "duration.hist")
	if err := os.WriteFile(historyFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write history file: %v", err)
	}

	history, err := ParseHistoryFile(historyFile)
	if err != nil {
		t.Fatalf("ParseHistoryFile() error = %v", err)
	}

	tests := []struct {
		index   int
		wantDur int
		wantCmd string
	}{
		{0, 5, "sleep 5"},
		{1, 0, "ls -la"},
		{2, 10, "make"},
	}

	for _, tt := range tests {
		if history.Commands[tt.index].Duration != tt.wantDur {
			t.Errorf("Commands[%d].Duration = %d, want %d", tt.index, history.Commands[tt.index].Duration, tt.wantDur)
		}
		if history.Commands[tt.index].Command != tt.wantCmd {
			t.Errorf("Commands[%d].Command = %q, want %q", tt.index, history.Commands[tt.index].Command, tt.wantCmd)
		}
	}
}
