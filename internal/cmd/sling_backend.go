// Package cmd provides API backend dispatch for the sling command.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/backend"
	"github.com/steveyegge/gastown/internal/backend/bedrock"
	"github.com/steveyegge/gastown/internal/backend/claude"
	"github.com/steveyegge/gastown/internal/backend/grok"
	"github.com/steveyegge/gastown/internal/backend/openai"
	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/config"
)

// BackendDispatcher handles API backend routing and execution.
type BackendDispatcher struct {
	config         *config.BackendConfig
	router         *backend.Router
	contextManager *backend.ContextManager
	costTracker    *backend.CostTracker
	initialized    bool
}

// NewBackendDispatcher creates a dispatcher with the given config.
func NewBackendDispatcher(cfg *config.BackendConfig) *BackendDispatcher {
	if cfg == nil {
		cfg = config.NewBackendConfig()
	}

	// Convert config to routing config
	routingCfg := &backend.RoutingConfig{
		Enabled:        cfg.Enabled,
		DefaultBackend: cfg.DefaultBackend,
		DefaultModel:   cfg.DefaultModel,
		CostThreshold:  cfg.CostThreshold,
		TokenThreshold: cfg.TokenThreshold,
		FallbackToCLI:  cfg.FallbackToCLI,
	}

	if cfg.Routing != nil {
		if cfg.Routing.DefaultRoute == "api" {
			routingCfg.DefaultRoute = backend.RouteAPI
		} else {
			routingCfg.DefaultRoute = backend.RouteCLI
		}

		// Convert routing rules
		for _, rule := range cfg.Routing.Rules {
			var route backend.RoutingDecision
			if rule.Route == "api" {
				route = backend.RouteAPI
			} else {
				route = backend.RouteCLI
			}

			routingCfg.Rules = append(routingCfg.Rules, backend.RoutingRule{
				Name:          rule.Name,
				TierMatch:     rule.TierMatch,
				ModelTagMatch: rule.ModelTagMatch,
				TypeMatch:     rule.TypeMatch,
				Route:         route,
				Backend:       rule.Backend,
				Model:         rule.Model,
			})
		}
	}

	return &BackendDispatcher{
		config:         cfg,
		router:         backend.NewRouter(routingCfg),
		contextManager: backend.NewContextManager(),
		costTracker:    backend.GetCostTracker(),
	}
}

// Initialize registers available backends based on config.
func (d *BackendDispatcher) Initialize() error {
	if d.initialized {
		return nil
	}

	// Register Claude backend if enabled
	if entry, ok := d.config.Backends["claude"]; ok && entry.Enabled {
		if err := claude.Register(); err != nil {
			log.Printf("[backend] Claude backend unavailable: %v", err)
		} else {
			log.Printf("[backend] Claude backend registered")
		}
	}

	// Register OpenAI backend if enabled
	if entry, ok := d.config.Backends["openai"]; ok && entry.Enabled {
		if err := openai.Register(); err != nil {
			log.Printf("[backend] OpenAI backend unavailable: %v", err)
		} else {
			log.Printf("[backend] OpenAI backend registered")
		}
	}

	// Register Grok backend if enabled
	if entry, ok := d.config.Backends["grok"]; ok && entry.Enabled {
		if err := grok.Register(); err != nil {
			log.Printf("[backend] Grok backend unavailable: %v", err)
		} else {
			log.Printf("[backend] Grok backend registered")
		}
	}

	// Register Bedrock backend if enabled
	if entry, ok := d.config.Backends["bedrock"]; ok && entry.Enabled {
		if err := bedrock.Register(); err != nil {
			log.Printf("[backend] Bedrock backend unavailable: %v", err)
		} else {
			log.Printf("[backend] Bedrock backend registered")
		}
	}

	d.initialized = true
	return nil
}

// ShouldRouteToAPI determines if a task should use API backend.
func (d *BackendDispatcher) ShouldRouteToAPI(issue *beads.Issue, step *beads.MoleculeStep) (*backend.RouteResult, bool) {
	if !d.config.Enabled {
		return nil, false
	}

	// Initialize backends before routing (so router can check availability)
	if err := d.Initialize(); err != nil {
		log.Printf("[backend] Failed to initialize backends: %v", err)
		return nil, false
	}

	// Extract routing hints
	hints := d.extractHints(issue, step)

	// Get routing decision
	result := d.router.Route(hints)

	return result, result.Decision == backend.RouteAPI
}

// extractHints extracts routing hints from issue and molecule step.
func (d *BackendDispatcher) extractHints(issue *beads.Issue, step *beads.MoleculeStep) *backend.RoutingHints {
	hints := &backend.RoutingHints{}

	if issue != nil {
		hints.Title = issue.Title
		hints.Description = issue.Description
		hints.Type = issue.Type
		hints.Labels = issue.Labels

		// Extract model tag from labels (legacy support)
		hints.ModelTag = backend.ExtractModelTag(issue.Labels)

		// Extract intent from labels
		hints.Intent = backend.ExtractIntent(issue.Labels)

		// Estimate tokens from description
		hints.EstimatedTokens = len(issue.Description) / 4 // Rough estimate
	}

	if step != nil {
		hints.Tier = step.Tier
	}

	return hints
}

// ExecuteAPIBackend executes a task via API backend.
func (d *BackendDispatcher) ExecuteAPIBackend(
	ctx context.Context,
	route *backend.RouteResult,
	issue *beads.Issue,
	step *beads.MoleculeStep,
) (*BackendExecutionResult, error) {
	if err := d.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing backends: %w", err)
	}

	// Get the backend
	registry := backend.GetRegistry()
	b, err := registry.Get(route.Backend)
	if err != nil {
		if route.FallbackToCLI {
			return &BackendExecutionResult{
				FallbackToCLI: true,
				Reason:        fmt.Sprintf("backend %s not available: %v", route.Backend, err),
			}, nil
		}
		return nil, fmt.Errorf("backend %s not available: %w", route.Backend, err)
	}

	// Build messages from issue context
	messages := d.buildMessages(issue, step)

	// Prepare context (trim if needed)
	model := route.Model
	if model == "" {
		model = b.DefaultModel()
	}

	maxTokens := b.MaxContextTokens(model)
	messages, err = d.contextManager.PrepareContext(messages, maxTokens, backend.TruncateOldest)
	if err != nil {
		if route.FallbackToCLI {
			return &BackendExecutionResult{
				FallbackToCLI: true,
				Reason:        fmt.Sprintf("context preparation failed: %v", err),
			}, nil
		}
		return nil, fmt.Errorf("preparing context: %w", err)
	}

	// Estimate cost before invocation
	tokenEstimate, _ := b.CountTokens(messages, model)
	costEstimate := b.EstimateCost(tokenEstimate, tokenEstimate/4, model)

	// Check cost threshold
	if costEstimate.TotalCost > d.config.CostThreshold {
		if route.FallbackToCLI {
			return &BackendExecutionResult{
				FallbackToCLI: true,
				Reason:        fmt.Sprintf("estimated cost $%.4f exceeds threshold $%.2f", costEstimate.TotalCost, d.config.CostThreshold),
			}, nil
		}
		log.Printf("[backend] Warning: estimated cost $%.4f exceeds threshold $%.2f", costEstimate.TotalCost, d.config.CostThreshold)
	}

	// Invoke the backend
	startTime := time.Now()
	result, err := b.Invoke(ctx, messages, backend.InvokeOptions{
		Model:     model,
		MaxTokens: 4096, // Default response limit
	})
	duration := time.Since(startTime)

	if err != nil {
		if route.FallbackToCLI {
			return &BackendExecutionResult{
				FallbackToCLI: true,
				Reason:        fmt.Sprintf("backend invocation failed: %v", err),
			}, nil
		}
		return nil, fmt.Errorf("backend invocation failed: %w", err)
	}

	// Record actual cost
	actualCost := b.EstimateCost(result.InputTokens, result.OutputTokens, model)
	d.costTracker.Record(route.Backend, model, result, actualCost)

	log.Printf("[backend] %s/%s completed in %v (in=%d, out=%d, cost=$%.4f)",
		route.Backend, model, duration, result.InputTokens, result.OutputTokens, actualCost.TotalCost)

	return &BackendExecutionResult{
		Success:      true,
		Content:      result.Content,
		Model:        result.Model,
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
		Cost:         actualCost,
		Duration:     duration,
	}, nil
}

// buildMessages constructs the message list for API invocation.
func (d *BackendDispatcher) buildMessages(issue *beads.Issue, step *beads.MoleculeStep) []backend.Message {
	var messages []backend.Message

	// System prompt
	systemPrompt := buildSystemPrompt(issue, step)
	if systemPrompt != "" {
		messages = append(messages, backend.Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	// User prompt from issue
	userPrompt := buildUserPrompt(issue, step)
	if userPrompt != "" {
		messages = append(messages, backend.Message{
			Role:    "user",
			Content: userPrompt,
		})
	}

	return messages
}

// buildSystemPrompt constructs the system prompt for API invocation.
func buildSystemPrompt(issue *beads.Issue, step *beads.MoleculeStep) string {
	var parts []string

	parts = append(parts, "You are an AI assistant helping with a software development task.")
	parts = append(parts, "Provide clear, concise responses focused on the task at hand.")
	parts = append(parts, "If you need to write code, ensure it is correct and well-documented.")

	if step != nil && step.Instructions != "" {
		parts = append(parts, "")
		parts = append(parts, "Step instructions:")
		parts = append(parts, step.Instructions)
	}

	return strings.Join(parts, "\n")
}

// buildUserPrompt constructs the user prompt from issue context.
func buildUserPrompt(issue *beads.Issue, step *beads.MoleculeStep) string {
	if issue == nil {
		return ""
	}

	var parts []string

	if issue.Title != "" {
		parts = append(parts, fmt.Sprintf("Task: %s", issue.Title))
	}

	if issue.Description != "" {
		parts = append(parts, "")
		parts = append(parts, "Description:")
		parts = append(parts, issue.Description)
	}

	// Include any attached args
	if fields := beads.ParseAttachmentFields(issue); fields != nil && fields.AttachedArgs != "" {
		parts = append(parts, "")
		parts = append(parts, "Additional instructions:")
		parts = append(parts, fields.AttachedArgs)
	}

	return strings.Join(parts, "\n")
}

// BackendExecutionResult contains the result of API backend execution.
type BackendExecutionResult struct {
	// Success indicates the API call completed successfully.
	Success bool

	// FallbackToCLI indicates the task should fall back to CLI agent.
	FallbackToCLI bool

	// Reason explains the result (success message or failure reason).
	Reason string

	// Content is the response from the API.
	Content string

	// Model is the model that was used.
	Model string

	// InputTokens is the number of input tokens.
	InputTokens int

	// OutputTokens is the number of output tokens.
	OutputTokens int

	// Cost is the estimated cost for this invocation.
	Cost backend.CostEstimate

	// Duration is how long the API call took.
	Duration time.Duration
}

// globalDispatcher is the singleton dispatcher instance.
var globalDispatcher *BackendDispatcher

// GetBackendDispatcher returns the global backend dispatcher.
// Initializes with default config if not already set.
func GetBackendDispatcher() *BackendDispatcher {
	if globalDispatcher == nil {
		globalDispatcher = NewBackendDispatcher(nil)
	}
	return globalDispatcher
}

// SetBackendDispatcher sets the global backend dispatcher.
func SetBackendDispatcher(d *BackendDispatcher) {
	globalDispatcher = d
}

// InitializeBackendDispatcher initializes the global dispatcher with config.
func InitializeBackendDispatcher(townRoot, rigPath string) *BackendDispatcher {
	cfg := config.ResolveBackendConfig(townRoot, rigPath)
	d := NewBackendDispatcher(cfg)
	SetBackendDispatcher(d)
	return d
}

// TryAPIBackendForBead checks if a bead should be handled by API backend.
// Returns (handled, error) - if handled is true, the bead was processed via API.
// If handled is false, the caller should continue with CLI dispatch.
func TryAPIBackendForBead(beadID, townRoot, rigPath string) (bool, error) {
	// Initialize dispatcher with config
	dispatcher := InitializeBackendDispatcher(townRoot, rigPath)

	// Check if API routing is enabled at all
	if !dispatcher.config.Enabled {
		return false, nil
	}

	// Fetch the issue to check routing hints
	issue, err := fetchIssueForRouting(beadID, townRoot)
	if err != nil {
		// Can't fetch issue - fall back to CLI
		log.Printf("[backend] Could not fetch issue %s for routing: %v", beadID, err)
		return false, nil
	}

	// Check if we should route to API
	route, shouldRoute := dispatcher.ShouldRouteToAPI(issue, nil)
	if !shouldRoute {
		return false, nil
	}

	log.Printf("[backend] Routing bead %s to API backend: %s/%s (reason: %s)",
		beadID, route.Backend, route.Model, route.Reason)

	// Execute via API backend
	ctx := context.Background()
	result, err := dispatcher.ExecuteAPIBackend(ctx, route, issue, nil)
	if err != nil {
		if route.FallbackToCLI {
			log.Printf("[backend] API execution failed, falling back to CLI: %v", err)
			return false, nil
		}
		return false, fmt.Errorf("API backend execution failed: %w", err)
	}

	if result.FallbackToCLI {
		log.Printf("[backend] API backend requested CLI fallback: %s", result.Reason)
		return false, nil
	}

	// API execution succeeded - update bead status
	if result.Success {
		log.Printf("[backend] API backend completed successfully for %s", beadID)
		// The bead is handled - caller should not dispatch to CLI
		fmt.Printf("Bead %s completed via API backend (%s)\n", beadID, result.Model)
		fmt.Printf("Response:\n%s\n", result.Content)
		return true, nil
	}

	return false, nil
}

// fetchIssueForRouting fetches an issue's details for routing decisions.
func fetchIssueForRouting(beadID, townRoot string) (*beads.Issue, error) {
	cmd := exec.Command("bd", "--no-daemon", "show", beadID, "--json", "--allow-stale")
	if townRoot != "" {
		cmd.Dir = townRoot
	}

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("bd show failed: %w", err)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("bead not found")
	}

	// bd show returns an array, even for single IDs
	var issues []beads.Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		// Try as single object (for backwards compatibility)
		var issue beads.Issue
		if err := json.Unmarshal(out, &issue); err != nil {
			return nil, fmt.Errorf("parsing issue: %w", err)
		}
		return &issue, nil
	}

	if len(issues) == 0 {
		return nil, fmt.Errorf("bead not found")
	}

	return &issues[0], nil
}
