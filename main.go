package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
)

// version is set via ldflags during build
var version = "dev"

func main() {
	rootFlags := ff.NewFlagSet("zist")
	helpFlag := rootFlags.BoolLong("help", "h")
	versionFlag := rootFlags.BoolLong("version", "v")

	collectFlags := ff.NewFlagSet("collect").SetParent(rootFlags)
	dbPath := collectFlags.StringLong("db", "~/.zist/zist.db", "SQLite database path")
	quietFlag := collectFlags.BoolLong("quiet", "q")
	collectCmd := &ff.Command{
		Name:      "collect",
		Usage:     "zist collect [--db PATH] [--quiet] [PATH...]",
		ShortHelp: "Collect commands from ZSH history files (default: ~/.histories)",
		Flags:     collectFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runCollect(ctx, *dbPath, args, *quietFlag)
		},
	}

	searchFlags := ff.NewFlagSet("search").SetParent(rootFlags)
	dbPathSearch := searchFlags.StringLong("db", "~/.zist/zist.db", "SQLite database path")
	limitFlag := searchFlags.IntLong("limit", 500, "Maximum number of results")
	sinceFlag := searchFlags.StringLong("since", "", "Only show commands after this date (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")
	untilFlag := searchFlags.StringLong("until", "", "Only show commands before this date (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "zist search [--db PATH] [--limit N] [--since DATE] [--until DATE] [QUERY]",
		ShortHelp: "Search command history interactively with fzf",
		Flags:     searchFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runSearch(ctx, *dbPathSearch, args, *limitFlag, *sinceFlag, *untilFlag)
		},
	}

	installFlags := ff.NewFlagSet("install").SetParent(rootFlags)
	installCmd := &ff.Command{
		Name:      "install",
		Usage:     "zist install",
		ShortHelp: "Install ZSH integration (Ctrl+X binding and precmd hook)",
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

	wizardFlags := ff.NewFlagSet("wizard").SetParent(rootFlags)
	wizardQuery := wizardFlags.StringLong("query", "q", "")
	wizardCache := wizardFlags.StringLong("cache", "", "Cache a query→command mapping (format: query)")
	wizardCacheCmd := wizardFlags.StringLong("cache-command", "", "Command to cache (use with --cache)")
	wizardListCache := wizardFlags.BoolLong("list-cache", "List cached query→command mappings")
	wizardClearCache := wizardFlags.BoolLong("clear-cache", "Clear all cached mappings")
	wizardPWD := wizardFlags.StringLong("pwd", "", "Current working directory (default: $PWD)")
	wizardOllamaURL := wizardFlags.StringLong("ollama-url", "http://localhost:11434/v1", "Ollama endpoint")
	wizardModel := wizardFlags.StringLong("model", "qwen2.5-coder:3b", "Model name")
	wizardTimeout := wizardFlags.DurationLong("timeout", 30*time.Second, "LLM timeout")
	wizardDBPath := wizardFlags.StringLong("db", "~/.zist/zist.db", "SQLite database path")
	wizardCmd := &ff.Command{
		Name:      "wizard",
		Usage:     "zist wizard --query 'natural language' [--json]",
		ShortHelp: "Generate shell commands from natural language",
		Flags:     wizardFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runWizard(ctx, *wizardDBPath, *wizardQuery, *wizardPWD,
				*wizardOllamaURL, *wizardModel, *wizardTimeout,
				*wizardCache, *wizardCacheCmd, *wizardListCache, *wizardClearCache)
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
		Subcommands: []*ff.Command{collectCmd, searchCmd, wizardCmd, installCmd, uninstallCmd},
		Exec: func(ctx context.Context, args []string) error {
			return fmt.Errorf("no subcommand provided")
		},
	}

	if err := rootCmd.ParseAndRun(context.Background(), os.Args[1:]); err != nil {
		if *versionFlag {
			fmt.Printf("zist version %s\n", version)
			return
		}
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
			// Recursively walk the directory tree
			err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && strings.HasSuffix(d.Name(), "zsh_history") {
					files = append(files, p)
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed to walk directory %s: %w", path, err)
			}
		} else {
			files = append(files, path)
		}
	}

	return files, nil
}

func runCollect(ctx context.Context, dbPath string, historyFiles []string, quiet bool) error {
	// Default to ~/.histories if no paths specified
	if len(historyFiles) == 0 {
		historyFiles = []string{expandTilde("~/.histories")}
	}

	expandedFiles, err := expandHistoryPaths(historyFiles)
	if err != nil {
		return err
	}

	if len(expandedFiles) == 0 {
		return fmt.Errorf("no history files found")
	}

	if !quiet {
		fmt.Printf("Collecting from %d file(s) into DB: %s\n", len(expandedFiles), dbPath)
	}

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
			if !quiet {
				fmt.Printf("Error parsing %s: %v\n", file, err)
			}
			continue
		}

		inserted, ignored, err := InsertCommandsBatch(db, history.Commands, 500)
		if err != nil {
			if !quiet {
				fmt.Printf("Error inserting from %s: %v\n", file, err)
			}
			continue
		}

		if !quiet {
			fmt.Printf("%s: %d parsed, %d new, %d skipped\n", file, len(history.Commands), inserted, ignored)
		}

		totalInserted += inserted
		totalIgnored += ignored
	}

	if !quiet {
		stats, err := GetDBStats(db)
		if err != nil {
			fmt.Printf("Warning: could not get DB stats: %v\n", err)
		} else {
			fmt.Printf("\nDatabase stats:\n")
			fmt.Printf("  Total commands: %d\n", stats["total_commands"])
			fmt.Printf("  Total sources: %d\n", stats["total_sources"])
		}

		fmt.Printf("\nCollection complete: %d new, %d skipped\n", totalInserted, totalIgnored)
	}
	return nil
}

func parseDateTime(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}

	// Try full datetime format first
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	if err == nil {
		return float64(t.Unix()), nil
	}

	// Try date-only format (use start of day)
	t, err = time.ParseInLocation("2006-01-02", s, time.Local)
	if err == nil {
		return float64(t.Unix()), nil
	}

	return 0, fmt.Errorf("invalid date format: %s (use YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)", s)
}

func runSearch(ctx context.Context, dbPath string, args []string, limit int, since, until string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	sinceTs, err := parseDateTime(since)
	if err != nil {
		return err
	}

	untilTs, err := parseDateTime(until)
	if err != nil {
		return err
	}

	db, err := InitDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	commands, err := SearchCommands(db, SearchOptions{
		Query: query,
		Limit: limit,
		Since: sinceTs,
		Until: untilTs,
	})
	if err != nil {
		return fmt.Errorf("failed to search: %w", err)
	}

	if len(commands) == 0 {
		return nil
	}

	if _, err := exec.LookPath("fzf"); err != nil {
		return fmt.Errorf("fzf not found in PATH, please install it first")
	}

	// fzf with preview pane showing source and timestamp
	// Use --read0 to handle multiline commands (null-byte separated records)
	cmd := exec.CommandContext(ctx, "fzf",
		"--read0",
		"--print0",
		"--delimiter=\t",
		"--with-nth=1", // Only display the command (field 1)
		"--preview", `sh -c 'printf "Source: %s\nTime:   %s\n\nCommand:\n%s\n" "$2" "$3" "$1"' _ {1} {2} {3}`,
		"--preview-window=right:40%:wrap",
	)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	go func() {
		for _, result := range commands {
			// Tab-separated: command \t source \t timestamp, null-byte terminated
			formattedTime := FormatTimestamp(result.Timestamp)
			fmt.Fprintf(stdin, "%s\t%s\t%s\x00", result.Command, result.Source, formattedTime)
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

	// Trim null byte and whitespace from output (--print0 adds trailing null)
	selected := strings.TrimRight(string(stdout), "\x00")
	selected = strings.TrimSpace(selected)
	if selected == "" {
		return nil
	}

	// Extract just the command (first tab-separated field)
	parts := strings.SplitN(selected, "\t", 2)
	if len(parts) >= 1 {
		fmt.Println(parts[0])
	}
	return nil
}

const zshIntegration = `# BEGIN zist integration
# Ctrl+X for fuzzy history search
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

# Wizard state for caching
typeset -g _zist_wizard_query=""
typeset -g _zist_wizard_command=""

# Ctrl+G for wizard (natural language → command)
_zist_wizard() {
  local query="$BUFFER"
  [[ -z "$query" ]] && return

  local cmd
  cmd=$(zist wizard --query "$query" 2>/dev/null)

  if [[ -n "$cmd" ]]; then
    # Store for caching on execution
    _zist_wizard_query="$query"
    _zist_wizard_command="$cmd"
    BUFFER="$cmd"
    CURSOR=${#BUFFER}
  fi
  zle reset-prompt
}
zle -N _zist_wizard
bindkey '^G' _zist_wizard

# Hook into accept-line to cache wizard commands when executed
_zist_accept_line() {
  # If this was a wizard-generated command, cache it
  if [[ -n "$_zist_wizard_query" && "$BUFFER" == "$_zist_wizard_command"* ]]; then
    # Cache the actual command being run (user may have edited it)
    (zist wizard --cache "$_zist_wizard_query" --cache-command "$BUFFER" &) 2>/dev/null
  fi
  # Clear wizard state
  _zist_wizard_query=""
  _zist_wizard_command=""
  zle .accept-line
}
zle -N accept-line _zist_accept_line

# Collect history after each command
autoload -Uz add-zsh-hook
_zist_precmd() {
  (zist collect --quiet &)
}
add-zsh-hook precmd _zist_precmd
# END zist integration
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

	if strings.Contains(string(content), "# BEGIN zist integration") {
		fmt.Println("ZSH integration already installed")
		fmt.Println("  To reinstall, run: zist uninstall && zist install")
		fmt.Printf("  Or source %s and press Ctrl+X to search history\n", zshrcPath)
		return nil
	}

	newContent := string(content)
	if len(newContent) > 0 && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += "\n" + zshIntegration

	if err := os.WriteFile(zshrcPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write ~/.zshrc: %w", err)
	}

	fmt.Println("ZSH integration installed")
	fmt.Println("  Collects from: ~/.histories (default)")
	fmt.Printf("  Run: source %s\n", zshrcPath)
	fmt.Println("  Keybindings:")
	fmt.Println("    Ctrl+G - wizard (natural language → command)")
	fmt.Println("    Ctrl+X - fuzzy history search")
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

	contentStr := string(content)

	// Find the BEGIN and END markers
	beginMarker := "# BEGIN zist integration"
	endMarker := "# END zist integration"

	beginIdx := strings.Index(contentStr, beginMarker)
	if beginIdx == -1 {
		fmt.Println("ZSH integration not found")
		return nil
	}

	endIdx := strings.Index(contentStr, endMarker)
	if endIdx == -1 {
		return fmt.Errorf("found BEGIN marker but no END marker - please manually remove zist integration from %s", zshrcPath)
	}

	// Remove the block including markers and the trailing newline
	endIdx += len(endMarker)
	if endIdx < len(contentStr) && contentStr[endIdx] == '\n' {
		endIdx++
	}

	// Also remove a leading newline if present
	if beginIdx > 0 && contentStr[beginIdx-1] == '\n' {
		beginIdx--
	}

	newContent := contentStr[:beginIdx] + contentStr[endIdx:]

	// Clean up any double newlines left behind
	for strings.Contains(newContent, "\n\n\n") {
		newContent = strings.ReplaceAll(newContent, "\n\n\n", "\n\n")
	}

	if err := os.WriteFile(zshrcPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write ~/.zshrc: %w", err)
	}

	fmt.Println("ZSH integration removed")
	fmt.Printf("  Run: source %s\n", zshrcPath)
	return nil
}

func runWizard(ctx context.Context, dbPath, query, pwd, ollamaURL, model string, timeout time.Duration, cacheQuery, cacheCmd string, listCache, clearCache bool) error {
	// Initialize database
	db, err := InitDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Handle cache operations
	if clearCache {
		if err := ClearWizardCache(db); err != nil {
			return err
		}
		fmt.Println("Wizard cache cleared")
		return nil
	}

	if listCache {
		entries, err := ListWizardCache(db, 50)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("No cached mappings")
			return nil
		}
		fmt.Printf("Cached mappings (%d):\n\n", len(entries))
		for _, e := range entries {
			fmt.Printf("  Query: %s\n", e.QueryOriginal)
			fmt.Printf("  Command: %s\n", e.Command)
			fmt.Printf("  Used: %d times\n\n", e.RunCount)
		}
		return nil
	}

	if cacheQuery != "" && cacheCmd != "" {
		if err := SetWizardCache(db, cacheQuery, cacheCmd); err != nil {
			return err
		}
		fmt.Printf("Cached: %q → %s\n", cacheQuery, cacheCmd)
		return nil
	}

	// Generate command from query
	if query == "" {
		return fmt.Errorf("--query is required (or use --list-cache, --clear-cache)")
	}

	// Default PWD to current directory
	if pwd == "" {
		pwd, _ = os.Getwd()
	}

	// Create LLM client
	llmConfig := LLMConfig{
		BaseURL:     ollamaURL,
		APIKey:      "ollama",
		Model:       model,
		Timeout:     timeout,
		MaxTokens:   500,
		Temperature: 0.3,
	}

	llm, err := NewLLMClient(llmConfig)
	if err != nil {
		return fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Create wizard and generate
	wizard := NewWizard(db, llm)
	resp, err := wizard.Generate(ctx, WizardRequest{
		Query: query,
		PWD:   pwd,
	})
	if err != nil {
		return err
	}

	// Output just the command (for shell integration)
	fmt.Println(resp.Command)
	return nil
}
