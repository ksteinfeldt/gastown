// Package openai implements the AgentBackend interface for OpenAI's API.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/steveyegge/gastown/internal/backend"
)

// Model definitions with context windows and pricing.
var (
	// Models maps model IDs to their context window sizes.
	Models = map[string]int{
		"gpt-4o":            128000,
		"gpt-4o-mini":       128000,
		"gpt-4-turbo":       128000,
		"gpt-4":             8192,
		"gpt-3.5-turbo":     16385,
		"o1":                200000,
		"o1-mini":           128000,
		"o1-preview":        128000,
		"o3-mini":           200000,
	}

	// Pricing per million tokens (input, output) in USD.
	// Prices as of early 2025 - update as needed.
	Pricing = map[string]struct{ Input, Output float64 }{
		"gpt-4o":        {2.50, 10.00},
		"gpt-4o-mini":   {0.15, 0.60},
		"gpt-4-turbo":   {10.00, 30.00},
		"gpt-4":         {30.00, 60.00},
		"gpt-3.5-turbo": {0.50, 1.50},
		"o1":            {15.00, 60.00},
		"o1-mini":       {3.00, 12.00},
		"o1-preview":    {15.00, 60.00},
		"o3-mini":       {1.10, 4.40},
	}
)

const (
	defaultBaseURL     = "https://api.openai.com"
	defaultModel       = "gpt-4o-mini"
	defaultMaxTokens   = 4096
	defaultTemperature = 1.0
	defaultTimeout     = 5 * time.Minute
)

// Backend implements backend.AgentBackend for OpenAI's API.
type Backend struct {
	apiKey     string
	baseURL    string
	client     *http.Client
	rateLimiter *rateLimiter
}

// Option configures the OpenAI backend.
type Option func(*Backend)

// WithBaseURL sets a custom base URL (for testing or proxies).
func WithBaseURL(url string) Option {
	return func(b *Backend) {
		b.baseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(b *Backend) {
		b.client = client
	}
}

// WithRateLimit sets the rate limit (requests per minute).
func WithRateLimit(rpm int) Option {
	return func(b *Backend) {
		b.rateLimiter = newRateLimiter(rpm, time.Minute)
	}
}

// New creates a new OpenAI backend.
// Requires OPENAI_API_KEY environment variable.
func New(opts ...Option) (*Backend, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}

	b := &Backend{
		apiKey:      apiKey,
		baseURL:     defaultBaseURL,
		client:      &http.Client{Timeout: defaultTimeout},
		rateLimiter: newRateLimiter(60, time.Minute), // Default 60 RPM
	}

	for _, opt := range opts {
		opt(b)
	}

	return b, nil
}

// Name returns the backend identifier.
func (b *Backend) Name() string {
	return "openai"
}

// Capabilities returns feature flags.
func (b *Backend) Capabilities() backend.Capability {
	return backend.CapStreaming | backend.CapTools | backend.CapVision | backend.CapLongContext
}

// AvailableModels returns supported model IDs.
func (b *Backend) AvailableModels() []string {
	models := make([]string, 0, len(Models))
	for model := range Models {
		models = append(models, model)
	}
	return models
}

// DefaultModel returns the default model.
func (b *Backend) DefaultModel() string {
	return defaultModel
}

// MaxContextTokens returns the context window for a model.
func (b *Backend) MaxContextTokens(model string) int {
	if ctx, ok := Models[model]; ok {
		return ctx
	}
	return 128000 // Default for unknown models
}

// apiRequest is the request body for the chat completions API.
type apiRequest struct {
	Model       string       `json:"model"`
	Messages    []apiMessage `json:"messages"`
	MaxTokens   int          `json:"max_completion_tokens,omitempty"`
	Temperature float64      `json:"temperature,omitempty"`
	Stream      bool         `json:"stream,omitempty"`
}

// apiMessage is a message in the API request.
type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// apiResponse is the response from the chat completions API.
type apiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int `json:"index"`
		Message      apiMessage `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// apiError is an error response from the API.
type apiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// Invoke sends a prompt and returns the response.
func (b *Backend) Invoke(ctx context.Context, messages []backend.Message, opts backend.InvokeOptions) (*backend.InvokeResult, error) {
	// Wait for rate limiter
	if err := b.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit: %w", err)
	}

	// Prepare request
	model := opts.Model
	if model == "" {
		model = defaultModel
	}

	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}

	temp := opts.Temperature
	if temp == 0 {
		temp = defaultTemperature
	}

	// Convert messages
	var apiMessages []apiMessage
	for _, msg := range messages {
		apiMessages = append(apiMessages, apiMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	reqBody := apiRequest{
		Model:       model,
		Messages:    apiMessages,
		MaxTokens:   maxTokens,
		Temperature: temp,
		Stream:      false,
	}

	// O1/O3 models don't support temperature
	if isReasoningModel(model) {
		reqBody.Temperature = 0
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.apiKey)

	// Send request with retry
	var resp *http.Response
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		resp, err = b.client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		// Check for rate limiting
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			retryAfter := time.Duration(attempt+1) * 10 * time.Second
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if d, err := time.ParseDuration(ra + "s"); err == nil {
					retryAfter = d
				}
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryAfter):
				continue
			}
		}

		break
	}

	if resp == nil {
		return nil, fmt.Errorf("request failed after retries: %w", lastErr)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		var apiErr apiError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("API error (%s): %s", apiErr.Error.Type, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	// Extract content from first choice
	var content string
	if len(apiResp.Choices) > 0 {
		content = apiResp.Choices[0].Message.Content
	}

	finishReason := ""
	if len(apiResp.Choices) > 0 {
		finishReason = apiResp.Choices[0].FinishReason
	}

	return &backend.InvokeResult{
		Content:      content,
		Model:        apiResp.Model,
		InputTokens:  apiResp.Usage.PromptTokens,
		OutputTokens: apiResp.Usage.CompletionTokens,
		FinishReason: finishReason,
	}, nil
}

// InvokeStream returns a streaming response channel.
func (b *Backend) InvokeStream(ctx context.Context, messages []backend.Message, opts backend.InvokeOptions) (<-chan backend.StreamChunk, error) {
	// For now, implement as non-streaming with single chunk
	ch := make(chan backend.StreamChunk, 1)

	go func() {
		defer close(ch)

		result, err := b.Invoke(ctx, messages, opts)
		if err != nil {
			ch <- backend.StreamChunk{Error: err, Done: true}
			return
		}

		ch <- backend.StreamChunk{Content: result.Content, Done: true}
	}()

	return ch, nil
}

// EstimateCost estimates the cost for given token counts.
func (b *Backend) EstimateCost(inputTokens, outputTokens int, model string) backend.CostEstimate {
	if model == "" {
		model = defaultModel
	}

	pricing, ok := Pricing[model]
	if !ok {
		// Default to GPT-4o-mini pricing for unknown models
		pricing = Pricing[defaultModel]
	}

	inputCost := float64(inputTokens) / 1_000_000 * pricing.Input
	outputCost := float64(outputTokens) / 1_000_000 * pricing.Output

	return backend.CostEstimate{
		InputCost:  inputCost,
		OutputCost: outputCost,
		TotalCost:  inputCost + outputCost,
		Currency:   "USD",
		Model:      model,
	}
}

// CountTokens estimates token count for messages.
// Uses a simple character-based heuristic (4 chars ≈ 1 token).
func (b *Backend) CountTokens(messages []backend.Message, model string) (int, error) {
	var totalChars int
	for _, msg := range messages {
		totalChars += len(msg.Content)
		totalChars += len(msg.Role) + 10 // Role overhead
	}
	// Rough estimate: 4 characters per token
	return totalChars / 4, nil
}

// Healthy checks if the backend is reachable.
func (b *Backend) Healthy(ctx context.Context) error {
	// Simple health check - verify API key format
	if len(b.apiKey) < 10 {
		return fmt.Errorf("invalid API key format")
	}
	return nil
}

// isReasoningModel checks if a model is an O1/O3 reasoning model.
func isReasoningModel(model string) bool {
	return model == "o1" || model == "o1-mini" || model == "o1-preview" || model == "o3-mini"
}

// rateLimiter implements a simple token bucket rate limiter.
type rateLimiter struct {
	mu             sync.Mutex
	tokens         int
	maxTokens      int
	refillInterval time.Duration
	lastRefill     time.Time
}

func newRateLimiter(maxTokens int, interval time.Duration) *rateLimiter {
	return &rateLimiter{
		tokens:         maxTokens,
		maxTokens:      maxTokens,
		refillInterval: interval,
		lastRefill:     time.Now(),
	}
}

func (r *rateLimiter) Wait(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(r.lastRefill)
	if elapsed >= r.refillInterval {
		r.tokens = r.maxTokens
		r.lastRefill = now
	} else {
		// Partial refill
		refillAmount := int(float64(r.maxTokens) * (float64(elapsed) / float64(r.refillInterval)))
		r.tokens = min(r.maxTokens, r.tokens+refillAmount)
		if refillAmount > 0 {
			r.lastRefill = now
		}
	}

	if r.tokens > 0 {
		r.tokens--
		return nil
	}

	// Wait for next token
	waitTime := r.refillInterval - elapsed
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitTime):
		r.tokens = r.maxTokens - 1
		r.lastRefill = time.Now()
		return nil
	}
}

// Register registers the OpenAI backend with the global registry.
func Register() error {
	b, err := New()
	if err != nil {
		return err
	}
	backend.GetRegistry().Register(b)
	return nil
}
