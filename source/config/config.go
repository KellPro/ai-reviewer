package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all configuration for the ai-reviewer CLI.
type Config struct {
	// LLM settings
	ModelEndpoint string `json:"model_endpoint,omitempty"`
	Model         string `json:"model,omitempty"`
	APIKey        string `json:"-"` // stored in keyring, never in file
	PromptExtra   string `json:"prompt_extra,omitempty"`

	// Bitbucket auth
	BBWorkspace string `json:"bb_workspace,omitempty"`
	BBEmail     string `json:"bb_email,omitempty"`
	BBToken     string `json:"-"` // stored in keyring, never in file

	// Behavior
	Platform string `json:"-"`
	Pending  bool   `json:"pending"`
	DryRun   bool   `json:"-"` // never persisted
	Path     string `json:"path,omitempty"`
	Switch   bool   `json:"switch,omitempty"`
}

// ConfigFilePath returns the path to the config file (~/.config/ai-reviewer.json).
func ConfigFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ai-reviewer.json")
}

// DefaultConfig returns a Config populated with defaults, then layered with
// the config file, keyring secrets, and environment variables.
// Precedence (later wins): defaults → config file → keyring → env vars → CLI flags.
func DefaultConfig() *Config {
	cfg := &Config{
		ModelEndpoint: "https://api.x.ai/v1",
		Model:         "grok-4-1-fast-reasoning",
		Platform:      "cloud",
		Pending:       true,
		DryRun:        false,
		Path:          ".",
		Switch:        false,
	}

	// Layer: config file
	if fileCfg, err := LoadConfigFile(); err == nil {
		mergeConfigFile(cfg, fileCfg)
	}

	// Layer: keyring
	if v := GetSecret(KeyAPIKey); v != "" {
		cfg.APIKey = v
	}
	if v := GetSecret(KeyBBToken); v != "" {
		cfg.BBToken = v
	}

	// Layer: environment variables
	if v := os.Getenv("AI_REVIEWER_ENDPOINT"); v != "" {
		cfg.ModelEndpoint = v
	}
	if v := os.Getenv("AI_REVIEWER_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("AI_REVIEWER_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("AI_REVIEWER_PROMPT_EXTRA"); v != "" {
		cfg.PromptExtra = v
	}
	if v := os.Getenv("BITBUCKET_WORKSPACE"); v != "" {
		cfg.BBWorkspace = v
	}
	if v := os.Getenv("BITBUCKET_EMAIL"); v != "" {
		cfg.BBEmail = v
	}
	if v := os.Getenv("BITBUCKET_TOKEN"); v != "" {
		cfg.BBToken = v
	}

	return cfg
}

// Validate checks that required configuration is present.
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("LLM API key is required (set --api-key, AI_REVIEWER_API_KEY, or run 'ai-reviewer init')")
	}
	// Auth is validated later when we attempt to authenticate
	return nil
}

// SaveConfigFile writes the non-sensitive config to ~/.config/ai-reviewer.json.
func SaveConfigFile(cfg *Config) error {
	path := ConfigFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}

// LoadConfigFile reads the config from ~/.config/ai-reviewer.json.
func LoadConfigFile() (*Config, error) {
	data, err := os.ReadFile(ConfigFilePath())
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	return &cfg, nil
}

// mergeConfigFile applies non-zero values from the file config onto the base.
func mergeConfigFile(base, file *Config) {
	if file.ModelEndpoint != "" {
		base.ModelEndpoint = file.ModelEndpoint
	}
	if file.Model != "" {
		base.Model = file.Model
	}
	if file.PromptExtra != "" {
		base.PromptExtra = file.PromptExtra
	}
	if file.BBWorkspace != "" {
		base.BBWorkspace = file.BBWorkspace
	}
	if file.BBEmail != "" {
		base.BBEmail = file.BBEmail
	}
	if file.Path != "" {
		base.Path = file.Path
	}
	// Pending and Switch are bools — always apply from file if the file was loaded
	base.Pending = file.Pending
	base.Switch = file.Switch
}
