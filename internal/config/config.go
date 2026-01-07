package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// WritingStyle defines the tone of code review comments
type WritingStyle string

const (
	StyleCorporate        WritingStyle = "corporate"
	StylePassiveAggressive WritingStyle = "passive_aggressive"
	StyleTechBro          WritingStyle = "tech_bro"
	StyleAcademic         WritingStyle = "academic"
)

// Config holds all user configuration
type Config struct {
	// GitHub settings
	GitHubToken string `yaml:"github_token"`

	// AI settings - generic OpenAI-compatible API
	AIApiURL string `yaml:"ai_api_url"`
	AIApiKey string `yaml:"ai_api_key"`
	AIModel  string `yaml:"ai_model"`

	// Review behavior
	WritingStyle     WritingStyle `yaml:"writing_style"`
	NitpickyLevel    int          `yaml:"nitpicky_level"` // 1-10
	LikedReviewers   []string     `yaml:"liked_reviewers"`
	DislikedReviewers []string    `yaml:"disliked_reviewers"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		AIApiURL:      "https://api.openai.com/v1",
		AIModel:       "gpt-4",
		WritingStyle:  StylePassiveAggressive,
		NitpickyLevel: 5,
	}
}

// ConfigDir returns the config directory path
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find home directory: %w", err)
	}
	return filepath.Join(home, ".salty-reviewer"), nil
}

// ConfigPath returns the full path to the config file
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads the config from disk
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config not found. Run 'salty init' first")
		}
		return nil, fmt.Errorf("could not read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("could not parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes the config to disk
func (c *Config) Save() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}

	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("could not encode config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("could not write config: %w", err)
	}

	return nil
}

// Validate checks that the config has required fields
func (c *Config) Validate() error {
	if c.GitHubToken == "" {
		return fmt.Errorf("github_token is required")
	}
	if c.AIApiKey == "" {
		return fmt.Errorf("ai_api_key is required")
	}
	if c.NitpickyLevel < 1 || c.NitpickyLevel > 10 {
		return fmt.Errorf("nitpicky_level must be between 1 and 10")
	}
	return nil
}

// IsLikedReviewer checks if a user is in the liked list
func (c *Config) IsLikedReviewer(username string) bool {
	for _, u := range c.LikedReviewers {
		if u == username {
			return true
		}
	}
	return false
}

// IsDislikedReviewer checks if a user is in the disliked list
func (c *Config) IsDislikedReviewer(username string) bool {
	for _, u := range c.DislikedReviewers {
		if u == username {
			return true
		}
	}
	return false
}

// AddLikedReviewer adds a user to the liked list
func (c *Config) AddLikedReviewer(username string) {
	if !c.IsLikedReviewer(username) {
		c.LikedReviewers = append(c.LikedReviewers, username)
	}
	// Remove from disliked if present
	c.removeFromDisliked(username)
}

// AddDislikedReviewer adds a user to the disliked list
func (c *Config) AddDislikedReviewer(username string) {
	if !c.IsDislikedReviewer(username) {
		c.DislikedReviewers = append(c.DislikedReviewers, username)
	}
	// Remove from liked if present
	c.removeFromLiked(username)
}

func (c *Config) removeFromLiked(username string) {
	for i, u := range c.LikedReviewers {
		if u == username {
			c.LikedReviewers = append(c.LikedReviewers[:i], c.LikedReviewers[i+1:]...)
			return
		}
	}
}

func (c *Config) removeFromDisliked(username string) {
	for i, u := range c.DislikedReviewers {
		if u == username {
			c.DislikedReviewers = append(c.DislikedReviewers[:i], c.DislikedReviewers[i+1:]...)
			return
		}
	}
}

// GetReviewerBias returns a multiplier for nitpicky level based on reviewer preference
// Returns: -2 to +3 adjustment to nitpicky level
func (c *Config) GetReviewerBias(username string) int {
	if c.IsLikedReviewer(username) {
		return -2 // Go easier on liked reviewers
	}
	if c.IsDislikedReviewer(username) {
		return 3 // Extra scrutiny for disliked reviewers
	}
	return 0
}
