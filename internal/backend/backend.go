// Package backend provides direct API access to LLM providers for lightweight tasks.
// This complements Gas Town's CLI agent approach by enabling fast, cheap API calls
// for simple tasks that don't require full Claude Code capabilities.
package backend

import (
	"context"
	"fmt"
	"sync"
)

// Capability flags for backend feature detection.
type Capability uint32

const (
	// CapStreaming indicates the backend supports streaming responses.
	CapStreaming Capability = 1 << iota
	// CapTools indicates the backend supports tool/function calling.
	CapTools
	// CapVision indicates the backend supports image inputs.
	CapVision
	// CapLongContext indicates the backend has >100k context window.
	CapLongContext
)

// Message represents a conversation message for API backends.
type Message struct {
	Role    string `json:"role"`    // "user", "assistant", "system"
	Content string `json:"content"`
}

// InvokeOptions configures a backend invocation.
type InvokeOptions struct {
	// Model overrides the default model selection.
	Model string `json:"model,omitempty"`

	// MaxTokens is the maximum response tokens.
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls randomness (0.0-1.0).
	Temperature float64 `json:"temperature,omitempty"`

	// SystemMsg is the system prompt (if separate from messages).
	SystemMsg string `json:"system_msg,omitempty"`

	// Stream requests a streaming response.
	Stream bool `json:"stream,omitempty"`
}

// InvokeResult contains the backend response.
type InvokeResult struct {
	// Content is the response text.
	Content string `json:"content"`

	// Model is the actual model used.
	Model string `json:"model"`

	// InputTokens is the token count for the prompt.
	InputTokens int `json:"input_tokens"`

	// OutputTokens is the token count for the response.
	OutputTokens int `json:"output_tokens"`

	// FinishReason indicates why generation stopped.
	// Common values: "stop", "length", "content_filter"
	FinishReason string `json:"finish_reason"`
}

// StreamChunk is a piece of a streaming response.
type StreamChunk struct {
	Content string
	Done    bool
	Error   error
}

// CostEstimate contains pricing information.
type CostEstimate struct {
	// InputCost is the cost for input tokens.
	InputCost float64 `json:"input_cost"`

	// OutputCost is the cost for output tokens.
	OutputCost float64 `json:"output_cost"`

	// TotalCost is the combined cost.
	TotalCost float64 `json:"total_cost"`

	// Currency is the currency code (always "USD").
	Currency string `json:"currency"`

	// Model is the model used for the estimate.
	Model string `json:"model"`
}

// AgentBackend is the interface for direct API model backends.
type AgentBackend interface {
	// Name returns the backend identifier (e.g., "claude", "openai", "grok").
	Name() string

	// Capabilities returns feature flags for this backend.
	Capabilities() Capability

	// AvailableModels returns model IDs this backend supports.
	AvailableModels() []string

	// DefaultModel returns the default model for this backend.
	DefaultModel() string

	// Invoke sends a prompt with context and returns the response.
	Invoke(ctx context.Context, messages []Message, opts InvokeOptions) (*InvokeResult, error)

	// InvokeStream returns a streaming response channel.
	InvokeStream(ctx context.Context, messages []Message, opts InvokeOptions) (<-chan StreamChunk, error)

	// EstimateCost estimates cost for given token counts.
	EstimateCost(inputTokens, outputTokens int, model string) CostEstimate

	// CountTokens estimates token count for messages.
	CountTokens(messages []Message, model string) (int, error)

	// MaxContextTokens returns the context window size for a model.
	MaxContextTokens(model string) int

	// Healthy checks if the backend is reachable.
	Healthy(ctx context.Context) error
}

// Registry manages available backends.
type Registry struct {
	mu       sync.RWMutex
	backends map[string]AgentBackend
}

// globalRegistry is the singleton registry instance.
var (
	globalRegistry     *Registry
	globalRegistryOnce sync.Once
)

// GetRegistry returns the global backend registry.
func GetRegistry() *Registry {
	globalRegistryOnce.Do(func() {
		globalRegistry = &Registry{
			backends: make(map[string]AgentBackend),
		}
	})
	return globalRegistry
}

// Register adds a backend to the registry.
func (r *Registry) Register(backend AgentBackend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backends[backend.Name()] = backend
}

// Get retrieves a backend by name.
func (r *Registry) Get(name string) (AgentBackend, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	backend, ok := r.backends[name]
	if !ok {
		return nil, fmt.Errorf("backend %q not registered", name)
	}
	return backend, nil
}

// List returns all registered backend names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.backends))
	for name := range r.backends {
		names = append(names, name)
	}
	return names
}

// Has checks if a backend is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.backends[name]
	return ok
}

// RoutingDecision indicates whether to use API or CLI.
type RoutingDecision string

const (
	// RouteAPI routes the task to an API backend.
	RouteAPI RoutingDecision = "api"
	// RouteCLI routes the task to a CLI agent (default Gas Town behavior).
	RouteCLI RoutingDecision = "cli"
)

// RouteResult contains the routing decision and metadata.
type RouteResult struct {
	// Decision is whether to use API or CLI.
	Decision RoutingDecision `json:"decision"`

	// Backend is the backend name for API routes.
	Backend string `json:"backend,omitempty"`

	// Model is the specific model to use.
	Model string `json:"model,omitempty"`

	// Reason explains why this routing was chosen.
	Reason string `json:"reason,omitempty"`

	// FallbackToCLI indicates whether to fall back to CLI on API error.
	FallbackToCLI bool `json:"fallback_to_cli,omitempty"`
}

// TierToBackend maps tier hints to recommended backends/models.
// The default mappings use Bedrock (AWS) for Claude models.
// To use direct Anthropic API, change "bedrock" to "claude" and
// set ANTHROPIC_API_KEY environment variable.
var TierToBackend = map[string]struct {
	Backend string
	Model   string
}{
	// Bedrock (AWS) mappings - default for Claude tiers
	"haiku":  {Backend: "bedrock", Model: "haiku"},
	"sonnet": {Backend: "bedrock", Model: "sonnet"},
	"opus":   {Backend: "bedrock", Model: "opus"},
	// Grok mappings
	"grok-fast": {Backend: "grok", Model: "grok-3-mini"},
	"grok":      {Backend: "grok", Model: "grok-3"},
	"grok-4":    {Backend: "grok", Model: "grok-4"},
	// OpenAI mappings
	"gpt4":    {Backend: "openai", Model: "gpt-4o"},
	"o1":      {Backend: "openai", Model: "o1"},
	"o3-mini": {Backend: "openai", Model: "o3-mini"},
}

// ResetRegistryForTesting clears all registry state.
// This is intended for use in tests only.
func ResetRegistryForTesting() {
	if globalRegistry != nil {
		globalRegistry.mu.Lock()
		globalRegistry.backends = make(map[string]AgentBackend)
		globalRegistry.mu.Unlock()
	}
}
