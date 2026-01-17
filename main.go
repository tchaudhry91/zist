package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
)

func main() {
	rootFlags := ff.NewFlagSet("zist")
	helpFlag := rootFlags.BoolLong("help", "h")

	collectFlags := ff.NewFlagSet("collect").SetParent(rootFlags)
	dbPath := collectFlags.StringLong("db", "~/.zist/zist.db", "SQLite database path")
	collectCmd := &ff.Command{
		Name:      "collect",
		Usage:     "zist collect [--db PATH] HISTORY_FILE... | DIRECTORY...",
		ShortHelp: "Collect commands from ZSH history files (or all *zsh_history files in a directory)",
		Flags:     collectFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runCollect(ctx, *dbPath, args)
		},
	}

	searchFlags := ff.NewFlagSet("search").SetParent(rootFlags)
	dbPathSearch := searchFlags.StringLong("db", "~/.zist/zist.db", "SQLite database path")
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "zist search [--db PATH] [QUERY]",
		ShortHelp: "Search command history interactively with fzf",
		Flags:     searchFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runSearch(ctx, *dbPathSearch, args)
		},
	}

	installFlags := ff.NewFlagSet("install").SetParent(rootFlags)
	installCmd := &ff.Command{
		Name:      "install",
		Usage:     "zist install",
		ShortHelp: "Install ZSH integration (Ctrl+X binding)",
		Flags:     installFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runInstall(ctx)
		},
	}

	uninstallFlags := ff.NewFlagSet("uninstall").SetParent(rootFlags)
	uninstallCmd := &ff.Command{
		Name:      "uninstall",
		Usage:     "zist uninstall",
		ShortHelp: "Remove ZSH integration",
		Flags:     uninstallFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runUninstall(ctx)
		},
	}

	var rootCmd *ff.Command

	rootCmd = &ff.Command{
		Name:  "zist",
		Usage: "zist [FLAGS] SUBCOMMAND ...",
		ShortHelp: "Local ZSH history aggregation tool. " +
			"Reads commands from multiple ZSH history files, " +
			"aggregates them into a local SQLite database, and provides fast search.",
		Flags:       rootFlags,
		Subcommands: []*ff.Command{collectCmd, searchCmd, installCmd, uninstallCmd},
		Exec: func(ctx context.Context, args []string) error {
			return fmt.Errorf("no subcommand provided")
		},
	}

	if err := rootCmd.ParseAndRun(context.Background(), os.Args[1:]); err != nil {
		if *helpFlag {
			fmt.Println(ffhelp.Command(rootCmd))
			return
		}
		fmt.Println(ffhelp.Command(rootCmd))
		if err.Error() == "no subcommand provided" {
			os.Exit(0)
		}
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

func expandHistoryPaths(paths []string) ([]string, error) {
	var files []string

	for _, path := range paths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("failed to stat %s: %w", path, err)
		}

		if fileInfo.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read directory %s: %w", path, err)
			}

			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), "zsh_history") {
					files = append(files, filepath.Join(path, entry.Name()))
				}
			}
		} else {
			files = append(files, path)
		}
	}

	return files, nil
}

func runCollect(ctx context.Context, dbPath string, historyFiles []string) error {
	expandedFiles, err := expandHistoryPaths(historyFiles)
	if err != nil {
		return err
	}

	if len(expandedFiles) == 0 {
		return fmt.Errorf("no history files found")
	}

	fmt.Printf("Collecting from %d file(s) into DB: %s\n", len(expandedFiles), dbPath)

	db, err := InitDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	totalInserted := 0
	totalIgnored := 0

	for _, file := range expandedFiles {
		history, err := ParseHistoryFile(file)
		if err != nil {
			fmt.Printf("Error parsing %s: %v\n", file, err)
			continue
		}

		inserted, ignored, err := InsertCommandsBatch(db, history.Commands, 500)
		if err != nil {
			fmt.Printf("Error inserting from %s: %v\n", file, err)
			continue
		}

		fmt.Printf("%s: %d parsed, %d new, %d skipped\n", file, len(history.Commands), inserted, ignored)

		totalInserted += inserted
		totalIgnored += ignored
	}

	stats, err := GetDBStats(db)
	if err != nil {
		fmt.Printf("Warning: could not get DB stats: %v\n", err)
	} else {
		fmt.Printf("\nDatabase stats:\n")
		fmt.Printf("  Total commands: %d\n", stats["total_commands"])
		fmt.Printf("  Total sources: %d\n", stats["total_sources"])
	}

	fmt.Printf("\n✓ Collection complete: %d new, %d skipped\n", totalInserted, totalIgnored)
	return nil
}

func runSearch(ctx context.Context, dbPath string, args []string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	db, err := InitDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	commands, err := SearchCommands(db, query)
	if err != nil {
		return fmt.Errorf("failed to search: %w", err)
	}

	if len(commands) == 0 {
		return nil
	}

	if _, err := exec.LookPath("fzf"); err != nil {
		return fmt.Errorf("fzf not found in PATH, please install it first")
	}

	cmd := exec.CommandContext(ctx, "fzf")
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	go func() {
		for _, result := range commands {
			fmt.Fprintf(stdin, "%s|||%s\n", result.Command, result.Source)
		}
		stdin.Close()
	}()

	stdout, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 130 {
				return nil
			}
			return fmt.Errorf("fzf failed: %w", err)
		}
		return fmt.Errorf("fzf failed: %w", err)
	}

	selected := strings.TrimSpace(string(stdout))
	if selected == "" {
		return nil
	}

	parts := strings.SplitN(selected, "|||", 2)
	if len(parts) >= 1 {
		fmt.Println(parts[0])
	}
	return nil
}

const zshIntegration = `# zist integration - Ctrl+X for fuzzy search
_zist_search() {
  local buf=$LBUFFER
  local selected=$(zist search "$buf" 2>/dev/null)
  if [[ -n "$selected" ]]; then
    LBUFFER="$selected"
  fi
  zle reset-prompt
}
zle -N _zist_search
bindkey '^X' _zist_search

# zist precmd hook - collect history after each command
if [[ -z "$(declare -f precmd)" ]]; then
  precmd() {
    zist collect &
  }
else
  # Append to existing precmd function
  precmd() {
    zist collect &
    ret=$?
    Oldprecmd
    return $ret
  }
  Oldprecmd=precmd
fi
`

func runInstall(ctx context.Context) error {
	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	zshrcPath := filepath.Join(usr.HomeDir, ".zshrc")

	content, err := os.ReadFile(zshrcPath)
	if err != nil {
		return fmt.Errorf("failed to read ~/.zshrc: %w", err)
	}

	if strings.Contains(string(content), "# zist integration") {
		fmt.Println("✓ ZSH integration already installed")
		fmt.Printf("  Run: source %s\n", zshrcPath)
		fmt.Println("  Then press Ctrl+X to search history")
		return nil
	}

	newContent := string(content)
	if !strings.HasSuffix(strings.TrimSpace(newContent), "\n") {
		newContent += "\n"
	}
	newContent += zshIntegration

	if err := os.WriteFile(zshrcPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write ~/.zshrc: %w", err)
	}

	fmt.Println("✓ ZSH integration installed")
	fmt.Printf("  Run: source %s\n", zshrcPath)
	fmt.Println("  Then press Ctrl+X to search history")
	return nil
}

func runUninstall(ctx context.Context) error {
	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	zshrcPath := filepath.Join(usr.HomeDir, ".zshrc")

	content, err := os.ReadFile(zshrcPath)
	if err != nil {
		return fmt.Errorf("failed to read ~/.zshrc: %w", err)
	}

	if !strings.Contains(string(content), "# zist integration") {
		fmt.Println("✓ ZSH integration not found")
		return nil
	}

	lines := strings.Split(string(content), "\n")
	newLines := []string{}
	skipUntilMatch := false

	for _, line := range lines {
		if strings.Contains(line, "# zist integration") {
			skipUntilMatch = true
			continue
		}

		if skipUntilMatch {
			if strings.HasPrefix(line, "bindkey '^X'") ||
				strings.HasPrefix(line, "zle -N _zist_search") ||
				strings.HasPrefix(line, "_zist_search() {") ||
				strings.HasPrefix(line, "  local buf=") ||
				strings.HasPrefix(line, "  local selected=") ||
				strings.HasPrefix(line, "  if [[ -n") ||
				strings.HasPrefix(line, "  fi") ||
				strings.HasPrefix(line, "  zle reset-prompt") ||
				strings.HasPrefix(line, "}") ||
				strings.HasPrefix(line, "if [[ -z \"$(declare -f") ||
				strings.HasPrefix(line, "  precmd() {") ||
				strings.HasPrefix(line, "    zist collect") ||
				strings.HasPrefix(line, "    ret=$") ||
				strings.HasPrefix(line, "    Oldprecmd=") ||
				strings.HasPrefix(line, "    return $ret") ||
				strings.HasPrefix(line, "  }") ||
				strings.HasPrefix(line, "else") ||
				strings.HasPrefix(line, "  # Append to existing") ||
				strings.HasPrefix(line, "  }") {
				continue
			}
			if strings.TrimSpace(line) == "}" {
				skipUntilMatch = false
				continue
			}
		}

		newLines = append(newLines, line)
	}

	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(zshrcPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write ~/.zshrc: %w", err)
	}

	fmt.Println("✓ ZSH integration removed")
	fmt.Printf("  Run: source %s\n", zshrcPath)
	return nil
}
