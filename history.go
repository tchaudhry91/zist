package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Command struct {
	Source    string  // Absolute file path
	Timestamp float64 // Unix timestamp with subsecond precision
	Command   string  // The command text
	Duration  int     // Execution duration in seconds
	CWD       string  // Working directory (optional, not in ZSH history)
	ExitCode  int     // Exit code (optional, not in ZSH history)
}

type History struct {
	Commands []Command
}

func ParseHistoryFile(file string) (*History, error) {
	absPath, err := filepath.Abs(file)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	f, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open history file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var history History
	var currentCommand strings.Builder
	var currentTimestamp int64
	var currentDuration int
	var hasCommand bool

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, ": ") {
			if hasCommand && currentCommand.Len() > 0 {
				history.Commands = append(history.Commands, Command{
					Source:    absPath,
					Timestamp: float64(currentTimestamp),
					Command:   strings.TrimSpace(currentCommand.String()),
					Duration:  currentDuration,
				})
				currentCommand.Reset()
			}

			metaAndCmd := strings.SplitN(line[2:], ";", 2)
			if len(metaAndCmd) != 2 {
				continue
			}

			timeAndDuration := strings.SplitN(metaAndCmd[0], ":", 2)
			if len(timeAndDuration) != 2 {
				continue
			}

			if timestamp, err := strconv.ParseInt(timeAndDuration[0], 10, 64); err == nil {
				currentTimestamp = timestamp
			}

			if duration, err := strconv.Atoi(timeAndDuration[1]); err == nil {
				currentDuration = duration
			}

			currentCommand.WriteString(metaAndCmd[1])
			hasCommand = true
		} else if hasCommand {
			currentCommand.WriteString("\n")
			currentCommand.WriteString(line)
		}
	}

	if hasCommand && currentCommand.Len() > 0 {
		history.Commands = append(history.Commands, Command{
			Source:    absPath,
			Timestamp: float64(currentTimestamp),
			Command:   strings.TrimSpace(currentCommand.String()),
			Duration:  currentDuration,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	history = addSubsecondTimestamps(history)

	return &history, nil
}

func addSubsecondTimestamps(history History) History {
	timestampMap := make(map[int64]int)
	result := make([]Command, 0, len(history.Commands))

	for _, cmd := range history.Commands {
		ts := int64(cmd.Timestamp)
		index := timestampMap[ts]
		timestampMap[ts] = index + 1

		result = append(result, Command{
			Source:    cmd.Source,
			Timestamp: float64(ts) + float64(index)*0.001,
			Command:   cmd.Command,
			Duration:  cmd.Duration,
			CWD:       cmd.CWD,
			ExitCode:  cmd.ExitCode,
		})
	}

	return History{Commands: result}
}

func FormatTimestamp(ts float64) string {
	t := time.Unix(int64(ts), int64((ts-float64(int64(ts)))*1e9))
	return t.Format("2006-01-02 15:04:05")
}
