package backend

import (
	"context"
	"testing"
)

func TestRouterDisabled(t *testing.T) {
	config := &RoutingConfig{
		Enabled: false,
	}
	router := NewRouter(config)

	result := router.Route(&RoutingHints{})
	if result.Decision != RouteCLI {
		t.Errorf("Expected RouteCLI when disabled, got %s", result.Decision)
	}
	if result.Reason != "hybrid routing disabled" {
		t.Errorf("Expected 'hybrid routing disabled', got %s", result.Reason)
	}
}

func TestRouterAutoSelectsModel(t *testing.T) {
	ResetRegistryForTesting()

	// Register mock backends
	GetRegistry().Register(&mockBackend{name: "bedrock"})
	GetRegistry().Register(&mockBackend{name: "grok"})

	config := &RoutingConfig{
		Enabled:       true,
		FallbackToCLI: true,
	}
	router := NewRouter(config)

	tests := []struct {
		name        string
		hints       *RoutingHints
		wantDec     RoutingDecision
		wantBackend string // empty means any API backend is fine
	}{
		{
			name: "simple task routes to cheap model",
			hints: &RoutingHints{
				Title:       "Summarize",
				Description: "Summarize this document",
			},
			wantDec: RouteAPI,
		},
		{
			name: "complex task routes to capable model",
			hints: &RoutingHints{
				Title:       "Implement authentication",
				Description: "Implement a comprehensive OAuth authentication system with refresh tokens",
			},
			wantDec: RouteAPI,
		},
		{
			name: "tool use routes to CLI",
			hints: &RoutingHints{
				Title:       "Create file",
				Description: "Create a file called config.json with the settings",
			},
			wantDec: RouteCLI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.Route(tt.hints)
			if result.Decision != tt.wantDec {
				t.Errorf("Decision = %s, want %s (reason: %s)", result.Decision, tt.wantDec, result.Reason)
			}
			if tt.wantBackend != "" && result.Backend != tt.wantBackend {
				t.Errorf("Backend = %s, want %s", result.Backend, tt.wantBackend)
			}
		})
	}
}

func TestRouterIntentBasedRouting(t *testing.T) {
	ResetRegistryForTesting()

	// Register mock backends
	GetRegistry().Register(&mockBackend{name: "bedrock"})
	GetRegistry().Register(&mockBackend{name: "grok"})

	config := &RoutingConfig{
		Enabled:       true,
		FallbackToCLI: true,
	}
	router := NewRouter(config)

	tests := []struct {
		name    string
		hints   *RoutingHints
		wantDec RoutingDecision
	}{
		{
			name: "tier:cheap label routes to API",
			hints: &RoutingHints{
				Title:       "Some task",
				Description: "Do something",
				Labels:      []string{"tier:cheap"},
			},
			wantDec: RouteAPI,
		},
		{
			name: "tier:quality label routes to API",
			hints: &RoutingHints{
				Title:       "Some task",
				Description: "Do something",
				Labels:      []string{"tier:quality"},
			},
			wantDec: RouteAPI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.Route(tt.hints)
			if result.Decision != tt.wantDec {
				t.Errorf("Decision = %s, want %s (reason: %s)", result.Decision, tt.wantDec, result.Reason)
			}
		})
	}
}

func TestRouterLegacyModelTags(t *testing.T) {
	ResetRegistryForTesting()

	// Register mock backends
	GetRegistry().Register(&mockBackend{name: "bedrock"})
	GetRegistry().Register(&mockBackend{name: "grok"})

	config := &RoutingConfig{
		Enabled:       true,
		FallbackToCLI: true,
	}
	router := NewRouter(config)

	tests := []struct {
		name        string
		hints       *RoutingHints
		wantDec     RoutingDecision
		wantBackend string
	}{
		{
			name: "model:grok-fast routes to grok",
			hints: &RoutingHints{
				ModelTag: "grok-fast",
			},
			wantDec:     RouteAPI,
			wantBackend: "grok",
		},
		{
			name: "model:opus routes to bedrock",
			hints: &RoutingHints{
				ModelTag: "opus",
			},
			wantDec:     RouteAPI,
			wantBackend: "bedrock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.Route(tt.hints)
			if result.Decision != tt.wantDec {
				t.Errorf("Decision = %s, want %s", result.Decision, tt.wantDec)
			}
			if result.Backend != tt.wantBackend {
				t.Errorf("Backend = %s, want %s", result.Backend, tt.wantBackend)
			}
		})
	}
}

func TestRouterFallbackWhenBackendUnavailable(t *testing.T) {
	ResetRegistryForTesting()

	// Only register bedrock (grok unavailable)
	GetRegistry().Register(&mockBackend{name: "bedrock"})

	config := &RoutingConfig{
		Enabled:       true,
		FallbackToCLI: true,
	}
	router := NewRouter(config)

	// Request grok-fast but it's unavailable
	result := router.Route(&RoutingHints{
		ModelTag: "grok-fast",
	})

	// Should fall back to bedrock, not CLI
	if result.Decision != RouteAPI {
		t.Errorf("Decision = %s, want RouteAPI (fallback)", result.Decision)
	}
	if result.Backend != "bedrock" {
		t.Errorf("Backend = %s, want bedrock (fallback)", result.Backend)
	}
}

func TestRouterNoBackendsAvailable(t *testing.T) {
	ResetRegistryForTesting()
	// No backends registered

	config := &RoutingConfig{
		Enabled:       true,
		FallbackToCLI: true,
	}
	router := NewRouter(config)

	result := router.Route(&RoutingHints{
		Title:       "Simple task",
		Description: "Do something simple",
	})

	// Should route to CLI when no backends available
	if result.Decision != RouteCLI {
		t.Errorf("Decision = %s, want RouteCLI (no backends)", result.Decision)
	}
}

func TestExtractModelTag(t *testing.T) {
	tests := []struct {
		labels []string
		want   string
	}{
		{[]string{"model:grok-fast"}, "grok-fast"},
		{[]string{"bug", "model:claude-haiku", "urgent"}, "claude-haiku"},
		{[]string{"bug", "urgent"}, ""},
		{[]string{}, ""},
		{nil, ""},
	}

	for _, tt := range tests {
		got := ExtractModelTag(tt.labels)
		if got != tt.want {
			t.Errorf("ExtractModelTag(%v) = %q, want %q", tt.labels, got, tt.want)
		}
	}
}

// mockBackend is a simple mock for testing
type mockBackend struct {
	name string
}

func (m *mockBackend) Name() string                                              { return m.name }
func (m *mockBackend) Capabilities() Capability                                  { return 0 }
func (m *mockBackend) AvailableModels() []string                                 { return nil }
func (m *mockBackend) DefaultModel() string                                      { return "default" }
func (m *mockBackend) MaxContextTokens(model string) int                         { return 100000 }
func (m *mockBackend) CountTokens(messages []Message, model string) (int, error) { return 0, nil }
func (m *mockBackend) EstimateCost(input, output int, model string) CostEstimate { return CostEstimate{} }
func (m *mockBackend) Healthy(_ context.Context) error                           { return nil }
func (m *mockBackend) Invoke(_ context.Context, _ []Message, _ InvokeOptions) (*InvokeResult, error) {
	return nil, nil
}
func (m *mockBackend) InvokeStream(_ context.Context, _ []Message, _ InvokeOptions) (<-chan StreamChunk, error) {
	return nil, nil
}
