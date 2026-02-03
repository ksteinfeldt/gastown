package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Client sends notifications to Slack via incoming webhooks.
type Client struct {
	webhookURL string
	channel    string
	enabled    bool
	httpClient *http.Client
	notifyOn   NotifySettings
}

// NewClient creates a new Slack client from configuration.
func NewClient(cfg *Config) *Client {
	if cfg == nil || !cfg.Enabled || cfg.WebhookURL == "" {
		return &Client{enabled: false}
	}

	return &Client{
		webhookURL: cfg.WebhookURL,
		channel:    cfg.Channel,
		enabled:    true,
		notifyOn:   cfg.NotifyOn,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// slackMessage represents a Slack webhook payload.
type slackMessage struct {
	Channel     string        `json:"channel,omitempty"`
	Text        string        `json:"text,omitempty"`
	Blocks      []slackBlock  `json:"blocks,omitempty"`
	Attachments []interface{} `json:"attachments,omitempty"`
}

// slackBlock represents a Slack Block Kit block.
type slackBlock struct {
	Type   string      `json:"type"`
	Text   *slackText  `json:"text,omitempty"`
	Fields []slackText `json:"fields,omitempty"`
}

// slackText represents text in a Slack block.
type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Post sends a message to Slack.
// Returns error if the request fails, but callers should generally ignore errors
// since Slack notifications are best-effort.
func (c *Client) Post(ctx context.Context, event EventType, fields map[string]string) error {
	if !c.enabled {
		return nil
	}

	// Check if this event type should be notified
	if !c.shouldNotify(event) {
		return nil
	}

	msg := formatMessage(event, fields)
	if c.channel != "" {
		msg.Channel = c.channel
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending to slack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}

	return nil
}

// shouldNotify checks if the given event type should trigger a notification.
func (c *Client) shouldNotify(event EventType) bool {
	switch event {
	case EventJobQueued:
		return c.notifyOn.JobQueued
	case EventJobStarted:
		return c.notifyOn.JobStarted
	case EventPRCreated:
		return c.notifyOn.PRCreated
	case EventJobCompleted:
		return c.notifyOn.JobCompleted
	case EventJobFailed, EventEscalation:
		return c.notifyOn.JobFailed
	default:
		return true
	}
}

// Global client for convenient access from hook points.
var (
	globalClient *Client
	globalMu     sync.RWMutex
)

// SetGlobalClient sets the global Slack client.
// Call this during initialization after loading config.
func SetGlobalClient(c *Client) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalClient = c
}

// GetGlobalClient returns the global Slack client.
func GetGlobalClient() *Client {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalClient
}

// Notify sends a notification using the global client.
// This is fire-and-forget - errors are logged but not returned.
// Safe to call even if Slack is not configured.
func Notify(event EventType, fields map[string]string) {
	globalMu.RLock()
	client := globalClient
	globalMu.RUnlock()

	if client == nil || !client.enabled {
		return
	}

	// Fire and forget in a goroutine to avoid blocking
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.Post(ctx, event, fields); err != nil {
			log.Printf("[slack] notification failed: %v", err)
		}
	}()
}

// Initialize loads config and sets up the global client.
// Call this from cmd initialization with the town root.
func Initialize(townRoot string) error {
	cfg, err := LoadConfig(townRoot)
	if err != nil {
		return fmt.Errorf("loading slack config: %w", err)
	}

	SetGlobalClient(NewClient(cfg))
	return nil
}
