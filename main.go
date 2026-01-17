package main

import (
	"context"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
)

func main() {
	// Root flags (common to all subcommands)
	rootFlags := ff.NewFlagSet("zist")

	// collect command
	collectFlags := ff.NewFlagSet("collect").SetParent(rootFlags)
	dbPath := collectFlags.StringLong("db", "~/.zist/zist.db", "SQLite database path")
	collectCmd := &ff.Command{
		Name:      "collect",
		Usage:     "zist collect [--db PATH] HISTORY_FILE...",
		ShortHelp: "Collect commands from ZSH history files",
		Flags:     collectFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runCollect(ctx, *dbPath, args)
		},
	}

	// search command
	searchFlags := ff.NewFlagSet("search").SetParent(rootFlags)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "zist search [QUERY]",
		ShortHelp: "Search command history",
		Flags:     searchFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runSearch(ctx, args)
		},
	}

	// Root command
	rootCmd := &ff.Command{
		Name:  "zist",
		Usage: "zist [FLAGS] SUBCOMMAND ...",
		ShortHelp: "Local ZSH history aggregation tool. " +
			"Reads commands from multiple ZSH history files, " +
			"aggregates them into a local SQLite database, and provides fast search.",
		Flags:       rootFlags,
		Subcommands: []*ff.Command{collectCmd, searchCmd},
	}

	// Parse and run
	if err := rootCmd.ParseAndRun(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", ffhelp.Command(rootCmd))
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runCollect(ctx context.Context, dbPath string, historyFiles []string) error {
	fmt.Printf("Collecting from %d files into DB: %s\n", len(historyFiles), dbPath)

	db, err := InitDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	totalInserted := 0
	totalIgnored := 0

	for _, file := range historyFiles {
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

func runSearch(ctx context.Context, args []string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}
	fmt.Printf("Searching for: %s\n", query)
	// TODO: Implement search logic with fzf
	return nil
}
