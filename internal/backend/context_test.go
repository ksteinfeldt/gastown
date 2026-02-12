package backend

import (
	"testing"
)

func TestContextManagerPrepareContext(t *testing.T) {
	cm := NewContextManager()

	tests := []struct {
		name      string
		messages  []Message
		maxTokens int
		strategy  TruncationStrategy
		wantCount int // Expected message count
	}{
		{
			name:      "empty messages",
			messages:  []Message{},
			maxTokens: 1000,
			strategy:  TruncateOldest,
			wantCount: 0,
		},
		{
			name: "messages fit",
			messages: []Message{
				{Role: "system", Content: "You are helpful"},
				{Role: "user", Content: "Hello"},
			},
			maxTokens: 10000,
			strategy:  TruncateOldest,
			wantCount: 2,
		},
		{
			name: "messages all fit when maxTokens is sufficient",
			messages: []Message{
				{Role: "system", Content: "System prompt"},
				{Role: "user", Content: "First message"},
				{Role: "assistant", Content: "First response"},
				{Role: "user", Content: "Second message"},
				{Role: "assistant", Content: "Second response"},
				{Role: "user", Content: "Third message"},
			},
			maxTokens: 10000, // Large enough for all messages
			strategy:  TruncateOldest,
			wantCount: 6, // All messages fit
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := cm.PrepareContext(tt.messages, tt.maxTokens, tt.strategy)
			if err != nil {
				t.Fatalf("PrepareContext() error = %v", err)
			}
			if len(result) != tt.wantCount {
				t.Errorf("PrepareContext() returned %d messages, want %d", len(result), tt.wantCount)
			}
		})
	}
}

func TestBuildMessagesFromText(t *testing.T) {
	tests := []struct {
		system    string
		user      string
		wantCount int
	}{
		{"", "", 0},
		{"System", "", 1},
		{"", "User", 1},
		{"System", "User", 2},
	}

	for _, tt := range tests {
		result := BuildMessagesFromText(tt.system, tt.user)
		if len(result) != tt.wantCount {
			t.Errorf("BuildMessagesFromText(%q, %q) = %d messages, want %d",
				tt.system, tt.user, len(result), tt.wantCount)
		}
	}
}

func TestContextManagerEstimateTokens(t *testing.T) {
	cm := NewContextManager()

	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant"},
		{Role: "user", Content: "Hello, how are you?"},
	}

	tokens := cm.estimateTokens(messages)
	if tokens <= 0 {
		t.Errorf("estimateTokens() = %d, want > 0", tokens)
	}

	// Verify longer content produces more tokens
	longMessages := []Message{
		{Role: "system", Content: "You are a helpful assistant"},
		{Role: "user", Content: "This is a much longer message that should produce more tokens because it contains many more characters than the shorter message above."},
	}

	longTokens := cm.estimateTokens(longMessages)
	if longTokens <= tokens {
		t.Errorf("Longer message should have more tokens: %d <= %d", longTokens, tokens)
	}
}
