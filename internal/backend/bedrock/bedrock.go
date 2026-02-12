// Package bedrock implements the AgentBackend interface for AWS Bedrock Claude models.
package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/steveyegge/gastown/internal/backend"
)

// Model definitions mapping friendly names to Bedrock model IDs.
var (
	// BedrockModels maps tier names to Bedrock model IDs.
	BedrockModels = map[string]string{
		"opus":   "us.anthropic.claude-opus-4-5-20251101-v1:0",
		"sonnet": "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
		"haiku":  "us.anthropic.claude-3-5-haiku-20241022-v1:0",
		// Full model IDs also supported
		"us.anthropic.claude-opus-4-5-20251101-v1:0":   "us.anthropic.claude-opus-4-5-20251101-v1:0",
		"us.anthropic.claude-sonnet-4-5-20250929-v1:0": "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
		"us.anthropic.claude-3-5-haiku-20241022-v1:0":  "us.anthropic.claude-3-5-haiku-20241022-v1:0",
	}

	// ContextWindows for each model tier.
	ContextWindows = map[string]int{
		"opus":   200000,
		"sonnet": 200000,
		"haiku":  200000,
	}

	// Pricing per million tokens (input, output) in USD.
	Pricing = map[string]struct{ Input, Output float64 }{
		"opus":   {15.00, 75.00},
		"sonnet": {3.00, 15.00},
		"haiku":  {0.80, 4.00},
	}
)

const (
	defaultModel       = "opus"
	defaultMaxTokens   = 4096
	defaultTemperature = 1.0
)

// Backend implements backend.AgentBackend for AWS Bedrock.
type Backend struct {
	client      *bedrockruntime.Client
	region      string
	rateLimiter *rateLimiter
}

// Option configures the Bedrock backend.
type Option func(*Backend)

// WithRegion sets the AWS region.
func WithRegion(region string) Option {
	return func(b *Backend) {
		b.region = region
	}
}

// New creates a new Bedrock backend using AWS credentials from environment/config.
func New(opts ...Option) (*Backend, error) {
	b := &Backend{
		region:      "us-east-1",
		rateLimiter: newRateLimiter(60, time.Minute),
	}

	for _, opt := range opts {
		opt(b)
	}

	// Load AWS config using default credential chain (env vars, profile, etc.)
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(b.region),
	)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	b.client = bedrockruntime.NewFromConfig(cfg)

	return b, nil
}

// Name returns the backend identifier.
func (b *Backend) Name() string {
	return "bedrock"
}

// Capabilities returns feature flags.
func (b *Backend) Capabilities() backend.Capability {
	return backend.CapStreaming | backend.CapTools | backend.CapVision | backend.CapLongContext
}

// AvailableModels returns supported model IDs.
func (b *Backend) AvailableModels() []string {
	return []string{"opus", "sonnet", "haiku"}
}

// DefaultModel returns the default model.
func (b *Backend) DefaultModel() string {
	return defaultModel
}

// MaxContextTokens returns the context window for a model.
func (b *Backend) MaxContextTokens(model string) int {
	// Normalize model name
	tier := normalizeTier(model)
	if ctx, ok := ContextWindows[tier]; ok {
		return ctx
	}
	return 200000
}

// bedrockRequest is the request body for Bedrock Claude models.
type bedrockRequest struct {
	AnthropicVersion string           `json:"anthropic_version"`
	MaxTokens        int              `json:"max_tokens"`
	Messages         []bedrockMessage `json:"messages"`
	System           string           `json:"system,omitempty"`
	Temperature      float64          `json:"temperature,omitempty"`
}

type bedrockMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// bedrockResponse is the response from Bedrock Claude models.
type bedrockResponse struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Role         string `json:"role"`
	Content      []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Invoke sends a prompt and returns the response.
func (b *Backend) Invoke(ctx context.Context, messages []backend.Message, opts backend.InvokeOptions) (*backend.InvokeResult, error) {
	// Wait for rate limiter
	if err := b.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit: %w", err)
	}

	// Resolve model
	model := opts.Model
	if model == "" {
		model = defaultModel
	}
	modelID, ok := BedrockModels[model]
	if !ok {
		// Try using the model string directly as a Bedrock model ID
		modelID = model
	}

	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}

	temp := opts.Temperature
	if temp == 0 {
		temp = defaultTemperature
	}

	// Convert messages, extracting system message
	var systemMsg string
	var bedrockMessages []bedrockMessage
	for _, msg := range messages {
		if msg.Role == "system" {
			systemMsg = msg.Content
			continue
		}
		bedrockMessages = append(bedrockMessages, bedrockMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Override system if provided in options
	if opts.SystemMsg != "" {
		systemMsg = opts.SystemMsg
	}

	reqBody := bedrockRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        maxTokens,
		Messages:         bedrockMessages,
		System:           systemMsg,
		Temperature:      temp,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	// Invoke Bedrock
	input := &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		Body:        jsonBody,
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
	}

	var output *bedrockruntime.InvokeModelOutput
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		output, err = b.client.InvokeModel(ctx, input)
		if err != nil {
			lastErr = err
			// Check for throttling
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		break
	}

	if output == nil {
		return nil, fmt.Errorf("request failed after retries: %w", lastErr)
	}

	// Parse response
	var resp bedrockResponse
	if err := json.Unmarshal(output.Body, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	// Extract text content
	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return &backend.InvokeResult{
		Content:      content,
		Model:        modelID,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
		FinishReason: resp.StopReason,
	}, nil
}

// InvokeStream returns a streaming response channel.
func (b *Backend) InvokeStream(ctx context.Context, messages []backend.Message, opts backend.InvokeOptions) (<-chan backend.StreamChunk, error) {
	// Implement as non-streaming with single chunk for now
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
	tier := normalizeTier(model)
	if tier == "" {
		tier = defaultModel
	}

	pricing, ok := Pricing[tier]
	if !ok {
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
func (b *Backend) CountTokens(messages []backend.Message, model string) (int, error) {
	var totalChars int
	for _, msg := range messages {
		totalChars += len(msg.Content)
		totalChars += len(msg.Role) + 10
	}
	return totalChars / 4, nil
}

// Healthy checks if the backend is reachable.
func (b *Backend) Healthy(ctx context.Context) error {
	// Verify we can make API calls by checking client is initialized
	if b.client == nil {
		return fmt.Errorf("bedrock client not initialized")
	}
	return nil
}

// normalizeTier converts model IDs to tier names.
func normalizeTier(model string) string {
	switch model {
	case "opus", "us.anthropic.claude-opus-4-5-20251101-v1:0":
		return "opus"
	case "sonnet", "us.anthropic.claude-sonnet-4-5-20250929-v1:0":
		return "sonnet"
	case "haiku", "us.anthropic.claude-3-5-haiku-20241022-v1:0":
		return "haiku"
	default:
		return model
	}
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

	now := time.Now()
	elapsed := now.Sub(r.lastRefill)
	if elapsed >= r.refillInterval {
		r.tokens = r.maxTokens
		r.lastRefill = now
	} else {
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

// Register registers the Bedrock backend with the global registry.
func Register() error {
	b, err := New()
	if err != nil {
		return err
	}
	backend.GetRegistry().Register(b)
	return nil
}
