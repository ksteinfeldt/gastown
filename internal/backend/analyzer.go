// Package backend provides task analysis for quality-aware routing.
package backend

import (
	"regexp"
	"strings"
)

// TaskComplexity represents the analyzed complexity of a task.
type TaskComplexity struct {
	// Score from 0-100, higher = more complex
	Score int

	// MinTier is the minimum model tier that can handle this with confidence
	MinTier ModelTier

	// RequiresToolUse indicates the task needs file/system operations
	RequiresToolUse bool

	// Signals are the detected complexity indicators
	Signals []string
}

// ModelTier represents capability levels of models.
type ModelTier int

const (
	// TierSimple - basic tasks: summarization, classification, short Q&A
	// Models: haiku, grok-3-mini
	TierSimple ModelTier = iota

	// TierModerate - code review, moderate generation, multi-step reasoning
	// Models: sonnet, grok-3
	TierModerate

	// TierComplex - architecture, complex implementation, deep analysis
	// Models: opus
	TierComplex

	// TierCLI - requires tool use, file operations, or multi-step workflows
	// Only CLI agents can handle these
	TierCLI
)

func (t ModelTier) String() string {
	switch t {
	case TierSimple:
		return "simple"
	case TierModerate:
		return "moderate"
	case TierComplex:
		return "complex"
	case TierCLI:
		return "cli"
	default:
		return "unknown"
	}
}

// ModelCapability defines what a model can handle confidently.
type ModelCapability struct {
	Backend    string
	Model      string
	Tier       ModelTier
	CostPer1K  float64 // Approximate cost per 1K tokens (input + output avg)
	SpeedScore int     // 1-10, higher = faster
}

// ModelCapabilities defines the capability and cost profile of each model.
// Ordered by cost (cheapest first within each tier).
var ModelCapabilities = []ModelCapability{
	// Tier Simple - fast and cheap, good for basic tasks
	{Backend: "grok", Model: "grok-3-mini", Tier: TierSimple, CostPer1K: 0.0001, SpeedScore: 9},
	{Backend: "bedrock", Model: "haiku", Tier: TierSimple, CostPer1K: 0.001, SpeedScore: 8},

	// Tier Moderate - balanced cost/capability
	{Backend: "grok", Model: "grok-3", Tier: TierModerate, CostPer1K: 0.01, SpeedScore: 7},
	{Backend: "bedrock", Model: "sonnet", Tier: TierModerate, CostPer1K: 0.009, SpeedScore: 6},

	// Tier Complex - highest capability API models
	{Backend: "bedrock", Model: "opus", Tier: TierComplex, CostPer1K: 0.045, SpeedScore: 4},
}

// TaskAnalyzer analyzes tasks to determine complexity and routing.
type TaskAnalyzer struct{}

// NewTaskAnalyzer creates a new task analyzer.
func NewTaskAnalyzer() *TaskAnalyzer {
	return &TaskAnalyzer{}
}

// Analyze examines a task and returns its complexity profile.
func (a *TaskAnalyzer) Analyze(title, description string, labels []string) *TaskComplexity {
	result := &TaskComplexity{
		Signals: make([]string, 0),
	}

	combined := strings.ToLower(title + " " + description)

	// Check for tool use requirements (must use CLI)
	if a.requiresToolUse(combined) {
		result.RequiresToolUse = true
		result.MinTier = TierCLI
		result.Score = 100
		result.Signals = append(result.Signals, "requires-tool-use")
		return result
	}

	// Calculate complexity score based on multiple signals
	score := 0

	// Length-based complexity
	wordCount := len(strings.Fields(combined))
	if wordCount > 200 {
		score += 25
		result.Signals = append(result.Signals, "long-description")
	} else if wordCount > 100 {
		score += 15
		result.Signals = append(result.Signals, "medium-description")
	} else if wordCount > 50 {
		score += 5
	}

	// Complex task indicators
	complexPatterns := map[string]int{
		"implement":     30,
		"refactor":      30,
		"architect":     40,
		"design":        25,
		"debug":         20,
		"optimize":      20,
		"migrate":       25,
		"integrate":     20,
		"multi-step":    20,
		"comprehensive": 15,
	}
	for pattern, points := range complexPatterns {
		if strings.Contains(combined, pattern) {
			score += points
			result.Signals = append(result.Signals, "complex:"+pattern)
		}
	}

	// Multi-step indicators
	multiStepPatterns := []string{
		"and then",
		"after that",
		"first,",
		"second,",
		"finally,",
		"step 1",
		"step 2",
	}
	for _, pattern := range multiStepPatterns {
		if strings.Contains(combined, pattern) {
			score += 25
			result.Signals = append(result.Signals, "multi-step")
			break
		}
	}

	// Numbered list detection (1. 2. 3. etc)
	numberedListRegex := regexp.MustCompile(`\d+\.\s+\w+`)
	if matches := numberedListRegex.FindAllString(combined, -1); len(matches) > 2 {
		score += 10
		result.Signals = append(result.Signals, "numbered-list")
	}

	// Simple task indicators (reduce score)
	simplePatterns := []string{
		"summarize",
		"explain",
		"what is",
		"list",
		"format",
		"convert",
		"translate",
		"classify",
		"categorize",
	}
	for _, pattern := range simplePatterns {
		if strings.Contains(combined, pattern) {
			score -= 10
			result.Signals = append(result.Signals, "simple:"+pattern)
			break
		}
	}

	// Check for explicit tier hints in labels
	for _, label := range labels {
		switch label {
		case "tier:fast", "tier:cheap":
			// User explicitly wants cheap/fast, trust them
			score = min(score, 30)
			result.Signals = append(result.Signals, "user-hint:cheap")
		case "tier:quality", "tier:powerful":
			// User explicitly wants quality
			score = max(score, 60)
			result.Signals = append(result.Signals, "user-hint:quality")
		}
	}

	// Clamp score
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	result.Score = score
	result.MinTier = a.scoreToTier(score)

	return result
}

// requiresToolUse checks if the task needs CLI tool capabilities.
func (a *TaskAnalyzer) requiresToolUse(text string) bool {
	// Patterns that indicate actual tool/command execution
	toolPatterns := []string{
		"create file",
		"create a file",
		"write to file",
		"edit file",
		"modify file",
		"delete file",
		"run test",
		"run the test",
		"execute command",
		"git commit",
		"git push",
		"git pull",
		"npm install",
		"npm run",
		"pip install",
		"make build",
		"deploy to",
		"restart service",
		"ssh into",
		"docker build",
		"docker run",
		"docker compose",
		"kubectl apply",
		"kubectl create",
	}

	for _, pattern := range toolPatterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}

// scoreToTier converts a complexity score to minimum required tier.
// Note: CLI routing is primarily based on tool use detection, not score.
// High complexity tasks should use Opus, not CLI.
func (a *TaskAnalyzer) scoreToTier(score int) ModelTier {
	switch {
	case score < 25:
		return TierSimple
	case score < 50:
		return TierModerate
	default:
		// Complex tasks (score >= 50) use Opus
		// CLI routing happens via RequiresToolUse, not score
		return TierComplex
	}
}

// Intent represents what the user wants to optimize for.
type Intent string

const (
	// IntentAuto - let the system decide based on task analysis
	IntentAuto Intent = "auto"

	// IntentFast - prioritize speed, accept quality tradeoff
	IntentFast Intent = "fast"

	// IntentCheap - prioritize cost, accept quality tradeoff
	IntentCheap Intent = "cheap"

	// IntentBalanced - balance cost, speed, and quality
	IntentBalanced Intent = "balanced"

	// IntentQuality - prioritize quality, accept higher cost
	IntentQuality Intent = "quality"
)

// ExtractIntent extracts the user's intent from labels.
func ExtractIntent(labels []string) Intent {
	for _, label := range labels {
		switch label {
		case "tier:fast":
			return IntentFast
		case "tier:cheap":
			return IntentCheap
		case "tier:balanced":
			return IntentBalanced
		case "tier:quality", "tier:powerful":
			return IntentQuality
		}
	}
	return IntentAuto
}

// SelectModel chooses the best model based on complexity, intent, and availability.
func SelectModel(complexity *TaskComplexity, intent Intent, availableBackends []string) *ModelCapability {
	// If tool use required, must use CLI
	if complexity.RequiresToolUse {
		return nil
	}

	// Determine minimum tier based on intent adjustments
	minTier := complexity.MinTier

	// Intent can lower the minimum tier (user accepts quality tradeoff)
	switch intent {
	case IntentFast, IntentCheap:
		// User explicitly wants cheap/fast - allow one tier lower
		if minTier > TierSimple {
			minTier = minTier - 1
		}
	case IntentQuality:
		// User wants quality - raise minimum tier
		if minTier < TierComplex {
			minTier = minTier + 1
		}
	}

	// Build set of available backends
	available := make(map[string]bool)
	for _, b := range availableBackends {
		available[b] = true
	}

	// Find cheapest model that meets minimum tier
	var candidates []ModelCapability
	for _, cap := range ModelCapabilities {
		if cap.Tier >= minTier && available[cap.Backend] {
			candidates = append(candidates, cap)
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Sort by cost for cheap intent, by speed for fast intent
	best := candidates[0]
	for _, c := range candidates[1:] {
		switch intent {
		case IntentFast:
			if c.SpeedScore > best.SpeedScore {
				best = c
			}
		default:
			// Default: cheapest that meets tier
			if c.CostPer1K < best.CostPer1K {
				best = c
			}
		}
	}

	return &best
}
