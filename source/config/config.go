package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the ai-reviewer CLI.
type Config struct {
	// LLM settings
	ModelEndpoint string
	Model         string
	APIKey        string
	PromptExtra   string

	// Bitbucket auth
	BBEmail string
	BBToken string

	// Behavior
	Platform string // "cloud" or "server" (auto-detected from URL)
	Pending  bool
	DryRun   bool
}

// DefaultConfig returns a Config populated with defaults and environment variables.
func DefaultConfig() *Config {
	return &Config{
		ModelEndpoint: envOrDefault("AI_REVIEWER_ENDPOINT", "https://api.x.ai/v1"),
		Model:         envOrDefault("AI_REVIEWER_MODEL", "grok-code-fast-1"),
		APIKey:        os.Getenv("AI_REVIEWER_API_KEY"),
		PromptExtra:   os.Getenv("AI_REVIEWER_PROMPT_EXTRA"),
		BBEmail:       os.Getenv("BITBUCKET_EMAIL"),
		BBToken:       os.Getenv("BITBUCKET_TOKEN"),
		Platform:      "cloud",
		Pending:       true,
		DryRun:        false,
	}
}

// Validate checks that required configuration is present.
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("LLM API key is required (set --api-key or AI_REVIEWER_API_KEY)")
	}
	// Auth is validated later when we attempt to authenticate
	return nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
