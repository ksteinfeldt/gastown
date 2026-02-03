package grok

import (
	"context"
	"os"
	"testing"

	"github.com/steveyegge/gastown/internal/backend"
)

func TestGrokAPI(t *testing.T) {
	// Skip if no API key
	if os.Getenv("XAI_API_KEY") == "" {
		t.Skip("XAI_API_KEY not set")
	}

	b, err := New()
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}

	messages := []backend.Message{
		{Role: "user", Content: "Say hello in 3 words or less"},
	}

	ctx := context.Background()
	result, err := b.Invoke(ctx, messages, backend.InvokeOptions{
		Model:     "grok-3-mini",
		MaxTokens: 50,
	})
	if err != nil {
		t.Fatalf("Failed to invoke: %v", err)
	}

	t.Logf("Model: %s", result.Model)
	t.Logf("Response: %s", result.Content)
	t.Logf("Tokens: in=%d, out=%d", result.InputTokens, result.OutputTokens)

	if result.Content == "" {
		t.Error("Expected non-empty response")
	}
}
