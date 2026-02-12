package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		enabled bool
	}{
		{
			name:    "nil config",
			cfg:     nil,
			enabled: false,
		},
		{
			name: "disabled config",
			cfg: &Config{
				Enabled:    false,
				WebhookURL: "https://hooks.slack.com/test",
			},
			enabled: false,
		},
		{
			name: "empty webhook",
			cfg: &Config{
				Enabled:    true,
				WebhookURL: "",
			},
			enabled: false,
		},
		{
			name: "valid config",
			cfg: &Config{
				Enabled:    true,
				WebhookURL: "https://hooks.slack.com/test",
			},
			enabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.cfg)
			if client.enabled != tt.enabled {
				t.Errorf("NewClient().enabled = %v, want %v", client.enabled, tt.enabled)
			}
		})
	}
}

func TestClientPost(t *testing.T) {
	var receivedPayload slackMessage

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type")
		}

		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			t.Errorf("failed to decode payload: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(&Config{
		Enabled:    true,
		WebhookURL: server.URL,
		NotifyOn: NotifySettings{
			JobQueued:    true,
			JobCompleted: true,
		},
	})

	ctx := context.Background()
	err := client.Post(ctx, EventJobQueued, map[string]string{
		FieldBead:     "gt-abc123",
		FieldTitle:    "Fix the bug",
		FieldAssignee: "rig1/polecats/p1",
	})

	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}

	// Verify payload structure
	if receivedPayload.Text == "" {
		t.Error("expected non-empty fallback text")
	}
	if len(receivedPayload.Blocks) == 0 {
		t.Error("expected blocks in payload")
	}
}

func TestClientPostDisabled(t *testing.T) {
	client := NewClient(&Config{Enabled: false})

	ctx := context.Background()
	err := client.Post(ctx, EventJobQueued, map[string]string{
		FieldBead: "gt-abc123",
	})

	if err != nil {
		t.Errorf("disabled client should not error: %v", err)
	}
}

func TestClientPostEventFiltering(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(&Config{
		Enabled:    true,
		WebhookURL: server.URL,
		NotifyOn: NotifySettings{
			JobQueued:    true,
			JobStarted:   false, // Disabled
			JobCompleted: true,
		},
	})

	ctx := context.Background()

	// Should send
	_ = client.Post(ctx, EventJobQueued, map[string]string{})
	if callCount != 1 {
		t.Errorf("expected 1 call for enabled event, got %d", callCount)
	}

	// Should not send (disabled)
	_ = client.Post(ctx, EventJobStarted, map[string]string{})
	if callCount != 1 {
		t.Errorf("expected no additional calls for disabled event, got %d", callCount)
	}

	// Should send
	_ = client.Post(ctx, EventJobCompleted, map[string]string{})
	if callCount != 2 {
		t.Errorf("expected 2 calls total, got %d", callCount)
	}
}

func TestClientPostTimeout(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(&Config{
		Enabled:    true,
		WebhookURL: server.URL,
		NotifyOn:   NotifySettings{JobQueued: true},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := client.Post(ctx, EventJobQueued, map[string]string{})
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	settingsDir := filepath.Join(tmpDir, "settings")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		t.Fatalf("failed to create settings dir: %v", err)
	}

	// Test missing file returns default config
	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig should not error on missing file: %v", err)
	}
	if cfg.Enabled {
		t.Error("default config should be disabled")
	}

	// Test valid config file
	configJSON := `{
		"enabled": true,
		"webhook_url": "https://hooks.slack.com/test",
		"channel": "#test",
		"notify_on": {
			"job_queued": true,
			"job_started": false,
			"pr_created": true,
			"job_completed": true,
			"job_failed": true
		}
	}`
	configPath := filepath.Join(settingsDir, "slack.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err = LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if !cfg.Enabled {
		t.Error("config should be enabled")
	}
	if cfg.WebhookURL != "https://hooks.slack.com/test" {
		t.Errorf("unexpected webhook URL: %s", cfg.WebhookURL)
	}
	if cfg.Channel != "#test" {
		t.Errorf("unexpected channel: %s", cfg.Channel)
	}
	if cfg.NotifyOn.JobStarted {
		t.Error("job_started should be false")
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Enabled:    true,
		WebhookURL: "https://hooks.slack.com/test",
		Channel:    "#gastown",
		NotifyOn: NotifySettings{
			JobQueued:    true,
			JobCompleted: true,
		},
	}

	if err := SaveConfig(tmpDir, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify file was created
	path := ConfigPath(tmpDir)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to parse saved config: %v", err)
	}

	if loaded.WebhookURL != cfg.WebhookURL {
		t.Errorf("webhook URL mismatch: got %s, want %s", loaded.WebhookURL, cfg.WebhookURL)
	}
}

func TestFormatMessage(t *testing.T) {
	tests := []struct {
		name   string
		event  EventType
		fields map[string]string
	}{
		{
			name:  "job queued",
			event: EventJobQueued,
			fields: map[string]string{
				FieldBead:     "gt-abc123",
				FieldTitle:    "Fix authentication bug",
				FieldAssignee: "rig1/polecats/p1",
			},
		},
		{
			name:  "job completed",
			event: EventJobCompleted,
			fields: map[string]string{
				FieldBead:   "gt-abc123",
				FieldBranch: "gt-abc123",
				FieldCommit: "a1b2c3d4e5f6",
			},
		},
		{
			name:  "escalation",
			event: EventEscalation,
			fields: map[string]string{
				FieldSeverity:    "high",
				FieldBead:        "gt-xyz789",
				FieldDescription: "Merge conflict could not be resolved automatically",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := formatMessage(tt.event, tt.fields)

			if msg.Text == "" {
				t.Error("expected non-empty fallback text")
			}
			if len(msg.Blocks) < 2 {
				t.Error("expected at least 2 blocks (header + fields)")
			}

			// Verify it's valid JSON
			data, err := json.Marshal(msg)
			if err != nil {
				t.Errorf("message should be valid JSON: %v", err)
			}
			if len(data) == 0 {
				t.Error("expected non-empty JSON output")
			}
		})
	}
}

func TestGlobalClient(t *testing.T) {
	// Reset global client
	SetGlobalClient(nil)

	if GetGlobalClient() != nil {
		t.Error("expected nil global client")
	}

	client := NewClient(&Config{
		Enabled:    true,
		WebhookURL: "https://hooks.slack.com/test",
	})
	SetGlobalClient(client)

	if GetGlobalClient() != client {
		t.Error("expected global client to be set")
	}

	// Cleanup
	SetGlobalClient(nil)
}

func TestNotifyWithNilClient(t *testing.T) {
	// Reset global client
	SetGlobalClient(nil)

	// Should not panic
	Notify(EventJobQueued, map[string]string{
		FieldBead: "gt-abc123",
	})
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a long string", 10, "this is..."},
		{"", 10, ""},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
