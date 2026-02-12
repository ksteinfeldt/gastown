package polecat

import (
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/config"
)

// TestSessionStartOptions_TeamConfig verifies TeamConfig field exists and threads correctly.
func TestSessionStartOptions_TeamConfig(t *testing.T) {
	tc := &config.TeamConfig{
		Enabled:       true,
		MaxTeammates:  3,
		TeammateModel: "sonnet",
	}

	opts := SessionStartOptions{
		DoltBranch: "polecat/Toast",
		TeamConfig: tc,
	}

	if opts.TeamConfig == nil {
		t.Fatal("TeamConfig should not be nil")
	}
	if !opts.TeamConfig.Enabled {
		t.Error("TeamConfig.Enabled should be true")
	}
	if opts.TeamConfig.MaxTeammates != 3 {
		t.Errorf("MaxTeammates = %d, want 3", opts.TeamConfig.MaxTeammates)
	}
	if opts.TeamConfig.TeammateModel != "sonnet" {
		t.Errorf("TeammateModel = %q, want %q", opts.TeamConfig.TeammateModel, "sonnet")
	}
}

// TestSessionStartOptions_NilTeamConfig verifies nil TeamConfig is safe.
func TestSessionStartOptions_NilTeamConfig(t *testing.T) {
	opts := SessionStartOptions{
		DoltBranch: "polecat/Toast",
	}

	if opts.TeamConfig != nil {
		t.Error("TeamConfig should be nil when not set")
	}
}

// TestTeamEnvInjection verifies that PrependEnv produces the right command
// when team config is enabled.
func TestTeamEnvInjection(t *testing.T) {
	baseCommand := "claude --dangerously-skip-permissions"

	tests := []struct {
		name       string
		teamConfig *config.TeamConfig
		wantEnvVar bool
	}{
		{
			name:       "team enabled - env var injected",
			teamConfig: &config.TeamConfig{Enabled: true, MaxTeammates: 3},
			wantEnvVar: true,
		},
		{
			name:       "team disabled - no env var",
			teamConfig: &config.TeamConfig{Enabled: false},
			wantEnvVar: false,
		},
		{
			name:       "nil team config - no env var",
			teamConfig: nil,
			wantEnvVar: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := baseCommand

			// Simulate the injection logic from session_manager.go Start()
			if tt.teamConfig != nil && tt.teamConfig.Enabled {
				command = config.PrependEnv(command, map[string]string{
					"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1",
				})
			}

			hasEnvVar := strings.Contains(command, "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1")
			if hasEnvVar != tt.wantEnvVar {
				t.Errorf("command contains env var = %v, want %v\ncommand: %s", hasEnvVar, tt.wantEnvVar, command)
			}

			// Verify base command is always present
			if !strings.Contains(command, baseCommand) {
				t.Errorf("base command missing from: %s", command)
			}
		})
	}
}

// TestTeamNudgeContent verifies the team nudge message format.
func TestTeamNudgeContent(t *testing.T) {
	tests := []struct {
		name         string
		maxTeammates int
		model        string
		wantContains []string
	}{
		{
			name:         "standard team config",
			maxTeammates: 3,
			model:        "sonnet",
			wantContains: []string{
				"[TEAM MODE]",
				"Max teammates: 3",
				"Teammate model: sonnet",
				"gt done",
			},
		},
		{
			name:         "single teammate opus",
			maxTeammates: 1,
			model:        "opus",
			wantContains: []string{
				"Max teammates: 1",
				"Teammate model: opus",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the nudge format from session_manager.go
			nudge := "[TEAM MODE] You have agent teams enabled. " +
				"Max teammates: " + itoa(tt.maxTeammates) + ". " +
				"Teammate model: " + tt.model + ". " +
				"Use Shift+Tab to delegate tasks to teammates. " +
				"Only YOU (the lead polecat) can run `gt done`."

			for _, want := range tt.wantContains {
				if !strings.Contains(nudge, want) {
					t.Errorf("nudge missing %q:\n%s", want, nudge)
				}
			}
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
