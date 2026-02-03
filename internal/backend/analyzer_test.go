package backend

import (
	"testing"
)

func TestTaskAnalyzerSimpleTasks(t *testing.T) {
	analyzer := NewTaskAnalyzer()

	tests := []struct {
		name        string
		title       string
		description string
		wantTier    ModelTier
		wantToolUse bool
	}{
		{
			name:        "simple summarize task",
			title:       "Summarize this",
			description: "Summarize the main points",
			wantTier:    TierSimple,
			wantToolUse: false,
		},
		{
			name:        "simple explain task",
			title:       "Explain API",
			description: "What is a REST API?",
			wantTier:    TierSimple,
			wantToolUse: false,
		},
		{
			name:        "short classification",
			title:       "Classify",
			description: "Is this a bug or feature request?",
			wantTier:    TierSimple,
			wantToolUse: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.Analyze(tt.title, tt.description, nil)
			if result.MinTier != tt.wantTier {
				t.Errorf("MinTier = %s, want %s (score=%d, signals=%v)",
					result.MinTier, tt.wantTier, result.Score, result.Signals)
			}
			if result.RequiresToolUse != tt.wantToolUse {
				t.Errorf("RequiresToolUse = %v, want %v", result.RequiresToolUse, tt.wantToolUse)
			}
		})
	}
}

func TestTaskAnalyzerComplexTasks(t *testing.T) {
	analyzer := NewTaskAnalyzer()

	tests := []struct {
		name        string
		title       string
		description string
		minTier     ModelTier // minimum expected tier
	}{
		{
			name:        "implement feature",
			title:       "Implement user auth",
			description: "Implement a complete user authentication system with OAuth support",
			minTier:     TierModerate,
		},
		{
			name:        "refactor code",
			title:       "Refactor database layer",
			description: "Refactor the database layer to use repository pattern",
			minTier:     TierModerate,
		},
		{
			name:        "architect system",
			title:       "Architect microservices",
			description: "Design and architect a microservices-based system for handling payments",
			minTier:     TierComplex,
		},
		{
			name:        "multi-step task",
			title:       "Setup CI/CD",
			description: "First, create the Dockerfile. Second, write the GitHub Actions workflow. Finally, configure deployment.",
			minTier:     TierModerate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.Analyze(tt.title, tt.description, nil)
			if result.MinTier < tt.minTier {
				t.Errorf("MinTier = %s, want >= %s (score=%d, signals=%v)",
					result.MinTier, tt.minTier, result.Score, result.Signals)
			}
		})
	}
}

func TestTaskAnalyzerToolUse(t *testing.T) {
	analyzer := NewTaskAnalyzer()

	tests := []struct {
		name        string
		description string
		wantToolUse bool
	}{
		{
			name:        "create file",
			description: "Create a file called config.json",
			wantToolUse: true,
		},
		{
			name:        "git commit",
			description: "Make the changes and git commit them",
			wantToolUse: true,
		},
		{
			name:        "run tests",
			description: "Run the test suite and fix failures",
			wantToolUse: true,
		},
		{
			name:        "npm install",
			description: "Install dependencies with npm install",
			wantToolUse: true,
		},
		{
			name:        "docker build",
			description: "Build the image with docker build and docker run it",
			wantToolUse: true,
		},
		{
			name:        "no tool use",
			description: "Explain how Docker works",
			wantToolUse: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.Analyze("Task", tt.description, nil)
			if result.RequiresToolUse != tt.wantToolUse {
				t.Errorf("RequiresToolUse = %v, want %v", result.RequiresToolUse, tt.wantToolUse)
			}
		})
	}
}

func TestTaskAnalyzerIntentLabels(t *testing.T) {
	analyzer := NewTaskAnalyzer()

	tests := []struct {
		name     string
		labels   []string
		maxScore int // score should be at or below this
	}{
		{
			name:     "tier:cheap reduces complexity",
			labels:   []string{"tier:cheap"},
			maxScore: 30,
		},
		{
			name:     "tier:fast reduces complexity",
			labels:   []string{"tier:fast"},
			maxScore: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a moderately complex task
			result := analyzer.Analyze("Implement feature", "Implement a new feature with multiple components", tt.labels)
			if result.Score > tt.maxScore {
				t.Errorf("Score = %d, want <= %d with labels %v", result.Score, tt.maxScore, tt.labels)
			}
		})
	}
}

func TestExtractIntent(t *testing.T) {
	tests := []struct {
		labels []string
		want   Intent
	}{
		{[]string{"tier:fast"}, IntentFast},
		{[]string{"tier:cheap"}, IntentCheap},
		{[]string{"tier:balanced"}, IntentBalanced},
		{[]string{"tier:quality"}, IntentQuality},
		{[]string{"tier:powerful"}, IntentQuality},
		{[]string{"bug", "urgent"}, IntentAuto},
		{[]string{}, IntentAuto},
		{nil, IntentAuto},
	}

	for _, tt := range tests {
		got := ExtractIntent(tt.labels)
		if got != tt.want {
			t.Errorf("ExtractIntent(%v) = %s, want %s", tt.labels, got, tt.want)
		}
	}
}

func TestSelectModel(t *testing.T) {
	tests := []struct {
		name       string
		complexity *TaskComplexity
		intent     Intent
		available  []string
		wantNil    bool
		wantTier   ModelTier
	}{
		{
			name:       "simple task with cheap intent gets simple model",
			complexity: &TaskComplexity{MinTier: TierSimple},
			intent:     IntentCheap,
			available:  []string{"grok", "bedrock"},
			wantNil:    false,
			wantTier:   TierSimple,
		},
		{
			name:       "complex task gets complex model",
			complexity: &TaskComplexity{MinTier: TierComplex},
			intent:     IntentAuto,
			available:  []string{"bedrock"},
			wantNil:    false,
			wantTier:   TierComplex,
		},
		{
			name:       "tool use returns nil",
			complexity: &TaskComplexity{RequiresToolUse: true},
			intent:     IntentAuto,
			available:  []string{"grok", "bedrock"},
			wantNil:    true,
		},
		{
			name:       "no backends returns nil",
			complexity: &TaskComplexity{MinTier: TierSimple},
			intent:     IntentAuto,
			available:  []string{},
			wantNil:    true,
		},
		{
			name:       "quality intent upgrades tier",
			complexity: &TaskComplexity{MinTier: TierSimple},
			intent:     IntentQuality,
			available:  []string{"bedrock"},
			wantNil:    false,
			wantTier:   TierModerate, // Upgraded from simple
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SelectModel(tt.complexity, tt.intent, tt.available)
			if tt.wantNil {
				if result != nil {
					t.Errorf("SelectModel() = %+v, want nil", result)
				}
				return
			}
			if result == nil {
				t.Errorf("SelectModel() = nil, want non-nil")
				return
			}
			if result.Tier < tt.wantTier {
				t.Errorf("SelectModel().Tier = %s, want >= %s", result.Tier, tt.wantTier)
			}
		})
	}
}

func TestSelectModelFallback(t *testing.T) {
	// When grok is unavailable, should fall back to bedrock
	complexity := &TaskComplexity{MinTier: TierSimple}

	// Only bedrock available
	result := SelectModel(complexity, IntentCheap, []string{"bedrock"})
	if result == nil {
		t.Fatal("SelectModel() = nil, want non-nil")
	}
	if result.Backend != "bedrock" {
		t.Errorf("Backend = %s, want bedrock (fallback)", result.Backend)
	}
}
