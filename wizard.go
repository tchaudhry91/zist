package main

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// WizardRequest contains the input for generating a command
type WizardRequest struct {
	Query    string // Natural language query
	PWD      string // Current working directory
	Hostname string // Machine name
}

// WizardResponse contains the generated command
type WizardResponse struct {
	Command   string        `json:"command"`
	Source    string        `json:"source"` // "cache" or "llm"
	Query     string        `json:"query"`
	Latency   time.Duration `json:"latency_ms"`
	FromCache bool          `json:"from_cache"`
}

// Wizard generates shell commands from natural language
type Wizard struct {
	llm LLMClient
	db  *sql.DB
}

// NewWizard creates a new Wizard instance
func NewWizard(db *sql.DB, llm LLMClient) *Wizard {
	return &Wizard{
		llm: llm,
		db:  db,
	}
}

// Generate produces a shell command from a natural language query
func (w *Wizard) Generate(ctx context.Context, req WizardRequest) (*WizardResponse, error) {
	start := time.Now()

	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Check cache first
	cached, err := GetWizardCache(w.db, query)
	if err != nil {
		// Log but continue - cache miss is not fatal
	}
	if cached != nil {
		return &WizardResponse{
			Command:   cached.Command,
			Source:    "cache",
			Query:     query,
			Latency:   time.Since(start),
			FromCache: true,
		}, nil
	}

	// No cache hit - generate with LLM
	if w.llm == nil {
		return nil, fmt.Errorf("LLM not available and no cached result")
	}

	// Gather history context
	historyContext := w.gatherHistoryContext(query)

	// Build prompts
	systemPrompt := w.buildSystemPrompt()
	userPrompt := w.buildUserPrompt(req, historyContext)

	// Generate command
	response, err := w.llm.Complete(ctx, userPrompt, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Parse and clean the response
	command := w.parseResponse(response)
	if command == "" {
		return nil, fmt.Errorf("LLM returned empty or invalid command")
	}

	return &WizardResponse{
		Command:   command,
		Source:    "llm",
		Query:     query,
		Latency:   time.Since(start),
		FromCache: false,
	}, nil
}

// CacheCommand stores a query→command mapping (called when user runs the command)
func (w *Wizard) CacheCommand(query, command string) error {
	return SetWizardCache(w.db, query, command)
}

// gatherHistoryContext extracts relevant commands from history based on query keywords
func (w *Wizard) gatherHistoryContext(query string) []string {
	keywords := extractKeywords(query)
	if len(keywords) == 0 {
		return nil
	}

	results, err := SearchHistoryByKeywords(w.db, keywords, 10)
	if err != nil {
		return nil
	}

	var commands []string
	for _, r := range results {
		commands = append(commands, r.Command)
	}
	return commands
}

// extractKeywords pulls relevant keywords from the query for history search
func extractKeywords(query string) []string {
	// Common words to ignore
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true,
		"i": true, "me": true, "my": true, "we": true, "our": true,
		"you": true, "your": true, "he": true, "she": true, "it": true,
		"they": true, "them": true, "their": true,
		"this": true, "that": true, "these": true, "those": true,
		"what": true, "which": true, "who": true, "whom": true,
		"how": true, "when": true, "where": true, "why": true,
		"all": true, "any": true, "both": true, "each": true,
		"few": true, "more": true, "most": true, "some": true,
		"show": true, "get": true, "find": true, "list": true, "display": true,
		"give": true, "tell": true, "can": true, "please": true,
		"want": true, "need": true, "like": true,
		"to": true, "of": true, "in": true, "for": true, "on": true,
		"with": true, "at": true, "by": true, "from": true, "as": true,
		"into": true, "through": true, "during": true, "before": true,
		"after": true, "above": true, "below": true, "between": true,
		"and": true, "or": true, "but": true, "not": true,
	}

	// Extract words, keeping only meaningful ones
	words := regexp.MustCompile(`[a-zA-Z0-9_\-\.]+`).FindAllString(strings.ToLower(query), -1)

	var keywords []string
	seen := make(map[string]bool)
	for _, word := range words {
		if len(word) < 2 {
			continue
		}
		if stopWords[word] {
			continue
		}
		if seen[word] {
			continue
		}
		seen[word] = true
		keywords = append(keywords, word)
	}

	return keywords
}

func (w *Wizard) buildSystemPrompt() string {
	return `You are a shell command generator. Convert natural language requests into executable shell commands.

RULES:
- Output ONLY the shell command, nothing else
- No explanations, no markdown, no code blocks
- Use common Unix/Linux commands
- Prefer simple, readable commands
- If multiple commands needed, chain with && or use subshells
- Use appropriate flags for human-readable output where applicable
- If the request is ambiguous, make reasonable assumptions

EXAMPLES:
User: "list all files including hidden"
Output: ls -la

User: "find large files over 100MB"
Output: find . -type f -size +100M

User: "show disk usage"
Output: df -h

User: "count lines in all python files"
Output: find . -name "*.py" -exec wc -l {} +`
}

func (w *Wizard) buildUserPrompt(req WizardRequest, historyContext []string) string {
	var sb strings.Builder

	sb.WriteString("Convert this request to a shell command:\n")
	sb.WriteString(req.Query)
	sb.WriteString("\n")

	if req.PWD != "" {
		sb.WriteString("\nCurrent directory: ")
		sb.WriteString(req.PWD)
		sb.WriteString("\n")
	}

	if len(historyContext) > 0 {
		sb.WriteString("\nRelevant commands from user's history (for context/patterns):\n")
		for _, cmd := range historyContext {
			// Truncate very long commands
			if len(cmd) > 100 {
				cmd = cmd[:100] + "..."
			}
			sb.WriteString("- ")
			sb.WriteString(cmd)
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\nShell command:")

	return sb.String()
}

func (w *Wizard) parseResponse(response string) string {
	// Clean up the response
	response = strings.TrimSpace(response)

	// Remove markdown code blocks if present
	response = strings.TrimPrefix(response, "```bash")
	response = strings.TrimPrefix(response, "```shell")
	response = strings.TrimPrefix(response, "```sh")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Take only the first line if multiple lines (unless it's a multi-line command)
	lines := strings.Split(response, "\n")
	if len(lines) > 1 {
		// Check if it looks like a multi-line command (continuation or chained)
		firstLine := strings.TrimSpace(lines[0])
		if !strings.HasSuffix(firstLine, "\\") && !strings.HasSuffix(firstLine, "&&") && !strings.HasSuffix(firstLine, "|") {
			response = firstLine
		}
	}

	// Remove any leading $ or # (shell prompts)
	response = strings.TrimPrefix(response, "$ ")
	response = strings.TrimPrefix(response, "# ")

	return response
}
