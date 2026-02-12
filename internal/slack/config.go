// Package slack provides Slack notification integration for Gas Town.
package slack

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds Slack notification configuration.
type Config struct {
	// Enabled controls whether Slack notifications are active.
	Enabled bool `json:"enabled"`

	// WebhookURL is the Slack incoming webhook URL.
	WebhookURL string `json:"webhook_url"`

	// Channel is the default channel (can be overridden by webhook config).
	Channel string `json:"channel,omitempty"`

	// NotifyOn controls which events trigger notifications.
	NotifyOn NotifySettings `json:"notify_on"`
}

// NotifySettings controls which events trigger Slack notifications.
type NotifySettings struct {
	// JobQueued notifies when work is assigned to a polecat.
	JobQueued bool `json:"job_queued"`

	// JobStarted notifies when a polecat begins work (can be noisy).
	JobStarted bool `json:"job_started"`

	// PRCreated notifies when work is ready for merge.
	PRCreated bool `json:"pr_created"`

	// JobCompleted notifies when work is merged to main.
	JobCompleted bool `json:"job_completed"`

	// JobFailed notifies when merge fails or escalation occurs.
	JobFailed bool `json:"job_failed"`
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Enabled:    false,
		WebhookURL: "",
		Channel:    "",
		NotifyOn: NotifySettings{
			JobQueued:    true,
			JobStarted:   false, // Too noisy by default
			PRCreated:    true,
			JobCompleted: true,
			JobFailed:    true,
		},
	}
}

// ConfigPath returns the path to the Slack config file for a town.
func ConfigPath(townRoot string) string {
	return filepath.Join(townRoot, "settings", "slack.json")
}

// LoadConfig loads Slack configuration from a town's settings directory.
// Returns nil config (not error) if file doesn't exist - Slack is opt-in.
func LoadConfig(townRoot string) (*Config, error) {
	path := ConfigPath(townRoot)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file = Slack disabled, not an error
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// SaveConfig writes Slack configuration to a town's settings directory.
func SaveConfig(townRoot string, cfg *Config) error {
	path := ConfigPath(townRoot)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
