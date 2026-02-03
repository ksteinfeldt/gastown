// Package backend provides routing logic for hybrid multi-model dispatch.
package backend

import (
	"log"
	"strings"
)

// RoutingConfig contains user-configurable routing rules.
type RoutingConfig struct {
	// Enabled is the master switch for hybrid routing.
	// When false, all tasks route to CLI agents.
	Enabled bool `json:"enabled"`

	// DefaultRoute is the default routing decision ("cli" or "api").
	DefaultRoute RoutingDecision `json:"default_route"`

	// DefaultBackend is the backend to use for API routes (legacy, prefer auto-selection).
	DefaultBackend string `json:"default_backend"`

	// DefaultModel is the model to use when not specified (legacy).
	DefaultModel string `json:"default_model,omitempty"`

	// CostThreshold is the maximum cost (USD) per task before routing to CLI.
	CostThreshold float64 `json:"cost_threshold"`

	// TokenThreshold is the maximum tokens before routing to CLI.
	TokenThreshold int `json:"token_threshold"`

	// FallbackToCLI indicates whether to fall back to CLI on API errors.
	FallbackToCLI bool `json:"fallback_to_cli"`

	// Rules are custom routing rules applied in order.
	Rules []RoutingRule `json:"rules,omitempty"`
}

// RoutingRule defines a custom routing condition.
type RoutingRule struct {
	// Name identifies this rule for logging.
	Name string `json:"name"`

	// Match conditions (all must match)
	TierMatch     []string `json:"tier_match,omitempty"`      // "simple", "moderate", "complex"
	ModelTagMatch []string `json:"model_tag_match,omitempty"` // Legacy: "model:grok-fast"
	TypeMatch     []string `json:"type_match,omitempty"`      // Issue types

	// Action
	Route   RoutingDecision `json:"route"`
	Backend string          `json:"backend,omitempty"`
	Model   string          `json:"model,omitempty"`
}

// RoutingHints contains hints extracted from task metadata.
type RoutingHints struct {
	// Title is the task title
	Title string

	// Description is the task description
	Description string

	// Tier is from MoleculeStep.Tier (legacy): "haiku", "sonnet", "opus"
	Tier string

	// ModelTag is from label (legacy): "model:grok-fast"
	ModelTag string

	// Intent is from label: "tier:fast", "tier:cheap", "tier:quality"
	Intent Intent

	// Type is the issue type
	Type string

	// EstimatedTokens is the estimated context size
	EstimatedTokens int

	// Labels are all labels from the issue
	Labels []string
}

// Router decides between API and CLI backends.
type Router struct {
	config   *RoutingConfig
	registry *Registry
	analyzer *TaskAnalyzer
}

// NewRouter creates a new router with the given config.
func NewRouter(config *RoutingConfig) *Router {
	if config == nil {
		config = DefaultRoutingConfig()
	}
	return &Router{
		config:   config,
		registry: GetRegistry(),
		analyzer: NewTaskAnalyzer(),
	}
}

// DefaultRoutingConfig returns sensible defaults.
func DefaultRoutingConfig() *RoutingConfig {
	return &RoutingConfig{
		Enabled:        false, // Opt-in
		DefaultRoute:   RouteCLI,
		DefaultBackend: "bedrock",
		DefaultModel:   "haiku",
		CostThreshold:  0.50,  // $0.50 max per API task
		TokenThreshold: 50000, // 50k tokens before CLI
		FallbackToCLI:  true,
	}
}

// Route determines the execution path for a task.
func (r *Router) Route(hints *RoutingHints) *RouteResult {
	// 1. Check if hybrid routing is enabled
	if !r.config.Enabled {
		return &RouteResult{
			Decision: RouteCLI,
			Reason:   "hybrid routing disabled",
		}
	}

	if hints == nil {
		hints = &RoutingHints{}
	}

	// 2. Extract intent from labels
	intent := ExtractIntent(hints.Labels)
	if intent == IntentAuto && hints.Intent != "" {
		intent = hints.Intent
	}

	// 3. Handle legacy model tags (backwards compatibility)
	if hints.ModelTag != "" {
		result := r.routeByModelTag(hints.ModelTag)
		if result != nil {
			return result
		}
	}

	// 4. Handle legacy tier hints (backwards compatibility)
	if hints.Tier != "" {
		result := r.routeByLegacyTier(hints.Tier)
		if result != nil {
			return result
		}
	}

	// 5. Analyze task complexity
	complexity := r.analyzer.Analyze(hints.Title, hints.Description, hints.Labels)

	log.Printf("[router] Task analysis: score=%d, minTier=%s, signals=%v",
		complexity.Score, complexity.MinTier, complexity.Signals)

	// 6. If tool use required, must use CLI
	if complexity.RequiresToolUse {
		return &RouteResult{
			Decision: RouteCLI,
			Reason:   "task requires tool use (file operations, git, etc.)",
		}
	}

	// 7. Check token threshold
	if hints.EstimatedTokens > 0 && hints.EstimatedTokens > r.config.TokenThreshold {
		return &RouteResult{
			Decision: RouteCLI,
			Reason:   "exceeds token threshold",
		}
	}

	// 8. Get available backends
	availableBackends := r.registry.List()
	if len(availableBackends) == 0 {
		return &RouteResult{
			Decision: RouteCLI,
			Reason:   "no API backends available",
		}
	}

	// 9. Select best model based on complexity, intent, and availability
	selected := SelectModel(complexity, intent, availableBackends)
	if selected == nil {
		return &RouteResult{
			Decision:      RouteCLI,
			Reason:        "no suitable model available for task complexity",
			FallbackToCLI: true,
		}
	}

	log.Printf("[router] Selected model: %s/%s (tier=%s, cost=%.4f/1K)",
		selected.Backend, selected.Model, selected.Tier, selected.CostPer1K)

	return &RouteResult{
		Decision:      RouteAPI,
		Backend:       selected.Backend,
		Model:         selected.Model,
		Reason:        r.buildReason(complexity, intent, selected),
		FallbackToCLI: r.config.FallbackToCLI,
	}
}

// buildReason constructs a human-readable reason for the routing decision.
func (r *Router) buildReason(complexity *TaskComplexity, intent Intent, selected *ModelCapability) string {
	parts := []string{}

	// Complexity info
	parts = append(parts, "complexity="+complexity.MinTier.String())

	// Intent if specified
	if intent != IntentAuto {
		parts = append(parts, "intent="+string(intent))
	}

	// Model selection
	parts = append(parts, "selected="+selected.Backend+"/"+selected.Model)

	return strings.Join(parts, ", ")
}

// routeByModelTag routes based on explicit model tag (legacy support).
func (r *Router) routeByModelTag(tag string) *RouteResult {
	// Check TierToBackend mapping for legacy tags
	if mapping, ok := TierToBackend[tag]; ok {
		// Verify backend is available
		if r.registry.Has(mapping.Backend) {
			return &RouteResult{
				Decision:      RouteAPI,
				Backend:       mapping.Backend,
				Model:         mapping.Model,
				Reason:        "legacy model tag: " + tag,
				FallbackToCLI: r.config.FallbackToCLI,
			}
		}
		// Backend not available - try fallback
		log.Printf("[router] Backend %s not available for tag %s, trying fallback", mapping.Backend, tag)
		return r.findFallbackForTag(tag)
	}

	// Check if tag is a known backend name
	if r.registry.Has(tag) {
		return &RouteResult{
			Decision:      RouteAPI,
			Backend:       tag,
			Reason:        "legacy model tag (backend): " + tag,
			FallbackToCLI: r.config.FallbackToCLI,
		}
	}

	return nil
}

// findFallbackForTag finds an alternative when the requested model is unavailable.
func (r *Router) findFallbackForTag(tag string) *RouteResult {
	// Map legacy tags to intents for fallback
	var intent Intent
	switch tag {
	case "grok-fast", "haiku":
		intent = IntentCheap
	case "grok", "sonnet":
		intent = IntentBalanced
	case "opus", "grok-4":
		intent = IntentQuality
	default:
		intent = IntentAuto
	}

	// Find best available alternative
	availableBackends := r.registry.List()
	complexity := &TaskComplexity{MinTier: TierSimple} // Assume simple for fallback

	if intent == IntentQuality {
		complexity.MinTier = TierComplex
	} else if intent == IntentBalanced {
		complexity.MinTier = TierModerate
	}

	selected := SelectModel(complexity, intent, availableBackends)
	if selected == nil {
		return nil
	}

	return &RouteResult{
		Decision:      RouteAPI,
		Backend:       selected.Backend,
		Model:         selected.Model,
		Reason:        "fallback from " + tag + " to " + selected.Backend + "/" + selected.Model,
		FallbackToCLI: r.config.FallbackToCLI,
	}
}

// routeByLegacyTier routes based on legacy tier hint.
func (r *Router) routeByLegacyTier(tier string) *RouteResult {
	tier = strings.ToLower(tier)

	// Map legacy tiers to intents
	var intent Intent
	var minTier ModelTier

	switch tier {
	case "haiku":
		intent = IntentCheap
		minTier = TierSimple
	case "sonnet":
		intent = IntentBalanced
		minTier = TierModerate
	case "opus":
		intent = IntentQuality
		minTier = TierComplex
	default:
		return nil
	}

	// Find best available model
	availableBackends := r.registry.List()
	complexity := &TaskComplexity{MinTier: minTier}

	selected := SelectModel(complexity, intent, availableBackends)
	if selected == nil {
		// No API model available, fall back to CLI
		return &RouteResult{
			Decision: RouteCLI,
			Reason:   "no API model available for tier: " + tier,
		}
	}

	return &RouteResult{
		Decision:      RouteAPI,
		Backend:       selected.Backend,
		Model:         selected.Model,
		Reason:        "legacy tier: " + tier + " → " + selected.Backend + "/" + selected.Model,
		FallbackToCLI: r.config.FallbackToCLI,
	}
}

// ExtractModelTag extracts the model tag from labels (legacy support).
func ExtractModelTag(labels []string) string {
	for _, label := range labels {
		if strings.HasPrefix(label, "model:") {
			return strings.TrimPrefix(label, "model:")
		}
	}
	return ""
}

// contains checks if a slice contains a value.
func contains(slice []string, value string) bool {
	for _, v := range slice {
		if strings.EqualFold(v, value) {
			return true
		}
	}
	return false
}
