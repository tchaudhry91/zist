package main

import (
	"context"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"
)

// LLMConfig holds configuration for the LLM client
type LLMConfig struct {
	BaseURL     string        // "http://localhost:11434/v1" (Ollama) or "https://api.openai.com/v1"
	APIKey      string        // Required for OpenAI, "ollama" for local
	Model       string        // "qwen2.5-coder:3b" or "gpt-4o-mini"
	Timeout     time.Duration // Default: 5s
	MaxTokens   int           // Default: 100
	Temperature float32       // Default: 0.3
}

// Message represents a chat message
type Message struct {
	Role    string // "system", "user", or "assistant"
	Content string
}

// LLMClient interface for LLM operations
type LLMClient interface {
	Complete(ctx context.Context, prompt, system string) (string, error)
	Chat(ctx context.Context, messages []Message) (string, error)
	IsAvailable(ctx context.Context) bool
}

// OpenAIClient implements LLMClient using the OpenAI-compatible API
type OpenAIClient struct {
	client *openai.Client
	config LLMConfig
}

// DefaultLLMConfig returns a config suitable for local Ollama
func DefaultLLMConfig() LLMConfig {
	return LLMConfig{
		BaseURL:     "http://localhost:11434/v1",
		APIKey:      "ollama",
		Model:       "qwen2.5-coder:3b",
		Timeout:     5 * time.Second,
		MaxTokens:   100,
		Temperature: 0.3,
	}
}

// NewLLMClient creates a new LLM client with the given configuration
func NewLLMClient(config LLMConfig) (LLMClient, error) {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:11434/v1"
	}
	if config.APIKey == "" {
		config.APIKey = "ollama"
	}
	if config.Model == "" {
		config.Model = "qwen2.5-coder:3b"
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 100
	}
	if config.Temperature == 0 {
		config.Temperature = 0.3
	}

	openaiConfig := openai.DefaultConfig(config.APIKey)
	openaiConfig.BaseURL = config.BaseURL

	client := openai.NewClientWithConfig(openaiConfig)

	return &OpenAIClient{
		client: client,
		config: config,
	}, nil
}

// Complete performs a single-turn completion with optional system prompt
func (c *OpenAIClient) Complete(ctx context.Context, prompt, system string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	messages := []openai.ChatCompletionMessage{}

	if system != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: system,
		})
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: prompt,
	})

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       c.config.Model,
		Messages:    messages,
		MaxTokens:   c.config.MaxTokens,
		Temperature: c.config.Temperature,
	})
	if err != nil {
		return "", fmt.Errorf("LLM completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	return resp.Choices[0].Message.Content, nil
}

// Chat performs a multi-turn conversation
func (c *OpenAIClient) Chat(ctx context.Context, messages []Message) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	openaiMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		role := openai.ChatMessageRoleUser
		switch msg.Role {
		case "system":
			role = openai.ChatMessageRoleSystem
		case "assistant":
			role = openai.ChatMessageRoleAssistant
		case "user":
			role = openai.ChatMessageRoleUser
		}
		openaiMessages[i] = openai.ChatCompletionMessage{
			Role:    role,
			Content: msg.Content,
		}
	}

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       c.config.Model,
		Messages:    openaiMessages,
		MaxTokens:   c.config.MaxTokens,
		Temperature: c.config.Temperature,
	})
	if err != nil {
		return "", fmt.Errorf("LLM chat failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	return resp.Choices[0].Message.Content, nil
}

// IsAvailable checks if the LLM endpoint is reachable
func (c *OpenAIClient) IsAvailable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err := c.client.ListModels(ctx)
	return err == nil
}
