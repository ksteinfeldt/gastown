// Package backend provides cost tracking and estimation.
package backend

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// CostTracker tracks API costs across invocations.
type CostTracker struct {
	mu      sync.RWMutex
	entries []CostEntry
	total   float64

	// Thresholds for warnings
	WarnThreshold  float64 // Log warning when single invocation exceeds this
	AlertThreshold float64 // Log alert when session total exceeds this
}

// CostEntry records a single API invocation cost.
type CostEntry struct {
	Timestamp    time.Time
	Backend      string
	Model        string
	InputTokens  int
	OutputTokens int
	Cost         CostEstimate
}

// NewCostTracker creates a new cost tracker with default thresholds.
func NewCostTracker() *CostTracker {
	return &CostTracker{
		entries:        make([]CostEntry, 0),
		WarnThreshold:  0.10, // Warn on single invocation > $0.10
		AlertThreshold: 5.00, // Alert when session total > $5.00
	}
}

// Record records a cost entry and checks thresholds.
func (ct *CostTracker) Record(backend, model string, result *InvokeResult, cost CostEstimate) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	entry := CostEntry{
		Timestamp:    time.Now(),
		Backend:      backend,
		Model:        model,
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
		Cost:         cost,
	}

	ct.entries = append(ct.entries, entry)
	ct.total += cost.TotalCost

	// Check thresholds
	if cost.TotalCost > ct.WarnThreshold {
		log.Printf("[COST WARNING] Single invocation cost $%.4f exceeds threshold $%.2f (backend=%s, model=%s, in=%d, out=%d)",
			cost.TotalCost, ct.WarnThreshold, backend, model, result.InputTokens, result.OutputTokens)
	}

	if ct.total > ct.AlertThreshold {
		log.Printf("[COST ALERT] Session total $%.2f exceeds threshold $%.2f",
			ct.total, ct.AlertThreshold)
	}
}

// Total returns the total cost for this session.
func (ct *CostTracker) Total() float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return ct.total
}

// Entries returns all cost entries.
func (ct *CostTracker) Entries() []CostEntry {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	// Return a copy
	entries := make([]CostEntry, len(ct.entries))
	copy(entries, ct.entries)
	return entries
}

// Summary returns a summary of costs by backend.
func (ct *CostTracker) Summary() map[string]BackendCostSummary {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	summary := make(map[string]BackendCostSummary)

	for _, entry := range ct.entries {
		s := summary[entry.Backend]
		s.Invocations++
		s.InputTokens += entry.InputTokens
		s.OutputTokens += entry.OutputTokens
		s.TotalCost += entry.Cost.TotalCost
		summary[entry.Backend] = s
	}

	return summary
}

// BackendCostSummary summarizes costs for a single backend.
type BackendCostSummary struct {
	Invocations  int
	InputTokens  int
	OutputTokens int
	TotalCost    float64
}

// Reset clears all cost tracking data.
func (ct *CostTracker) Reset() {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.entries = make([]CostEntry, 0)
	ct.total = 0
}

// FormatSummary returns a human-readable cost summary.
func (ct *CostTracker) FormatSummary() string {
	summary := ct.Summary()
	total := ct.Total()

	if len(summary) == 0 {
		return "No API costs recorded"
	}

	result := fmt.Sprintf("API Cost Summary (Total: $%.4f)\n", total)
	result += "─────────────────────────────────────\n"

	for backend, s := range summary {
		result += fmt.Sprintf("  %s: %d invocations, %d in / %d out tokens, $%.4f\n",
			backend, s.Invocations, s.InputTokens, s.OutputTokens, s.TotalCost)
	}

	return result
}

// globalCostTracker is the singleton cost tracker.
var (
	globalCostTracker     *CostTracker
	globalCostTrackerOnce sync.Once
)

// GetCostTracker returns the global cost tracker.
func GetCostTracker() *CostTracker {
	globalCostTrackerOnce.Do(func() {
		globalCostTracker = NewCostTracker()
	})
	return globalCostTracker
}

// EstimateTaskCost estimates the cost for a task based on hints.
func EstimateTaskCost(hints *RoutingHints, backend AgentBackend) CostEstimate {
	if hints == nil || backend == nil {
		return CostEstimate{Currency: "USD"}
	}

	// Use estimated tokens if available
	inputTokens := hints.EstimatedTokens
	if inputTokens == 0 {
		inputTokens = 1000 // Default estimate
	}

	// Estimate output as 25% of input
	outputTokens := inputTokens / 4

	model := backend.DefaultModel()
	return backend.EstimateCost(inputTokens, outputTokens, model)
}
