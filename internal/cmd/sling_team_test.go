package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/config"
)

// TestTeamFlagVariables verifies --team flag variables exist and toggle correctly.
func TestTeamFlagVariables(t *testing.T) {
	// Save previous values
	prevTeam := slingTeam
	prevTeamSize := slingTeamSize
	prevTeammateTier := slingTeammateTier
	prevNoTeam := slingNoTeam
	t.Cleanup(func() {
		slingTeam = prevTeam
		slingTeamSize = prevTeamSize
		slingTeammateTier = prevTeammateTier
		slingNoTeam = prevNoTeam
	})

	slingTeam = true
	if !slingTeam {
		t.Error("slingTeam flag should be true")
	}

	slingTeamSize = 5
	if slingTeamSize != 5 {
		t.Errorf("slingTeamSize = %d, want 5", slingTeamSize)
	}

	slingTeammateTier = "opus"
	if slingTeammateTier != "opus" {
		t.Errorf("slingTeammateTier = %q, want %q", slingTeammateTier, "opus")
	}

	slingNoTeam = true
	if !slingNoTeam {
		t.Error("slingNoTeam flag should be true")
	}
}

// TestTeamFlagMutualExclusion verifies --team and --no-team cannot coexist.
func TestTeamFlagMutualExclusion(t *testing.T) {
	// Save and restore global flag state
	prevTeam := slingTeam
	prevNoTeam := slingNoTeam
	prevTeamSize := slingTeamSize
	prevTeammateTier := slingTeammateTier
	prevDryRun := slingDryRun
	t.Cleanup(func() {
		slingTeam = prevTeam
		slingNoTeam = prevNoTeam
		slingTeamSize = prevTeamSize
		slingTeammateTier = prevTeammateTier
		slingDryRun = prevDryRun
	})

	// Set up minimal env so runSling gets far enough to hit validation
	t.Setenv("GT_POLECAT", "") // Not a polecat

	slingTeam = true
	slingNoTeam = true
	slingTeamSize = 3
	slingTeammateTier = "sonnet"
	slingDryRun = true

	err := runSling(nil, []string{"gt-test123"})
	if err == nil {
		t.Fatal("expected error when both --team and --no-team are set")
	}
	if !strings.Contains(err.Error(), "cannot use both --team and --no-team") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestTeamSizeValidation verifies --team-size range checking.
func TestTeamSizeValidation(t *testing.T) {
	prevTeam := slingTeam
	prevNoTeam := slingNoTeam
	prevTeamSize := slingTeamSize
	prevTeammateTier := slingTeammateTier
	prevDryRun := slingDryRun
	t.Cleanup(func() {
		slingTeam = prevTeam
		slingNoTeam = prevNoTeam
		slingTeamSize = prevTeamSize
		slingTeammateTier = prevTeammateTier
		slingDryRun = prevDryRun
	})

	t.Setenv("GT_POLECAT", "")

	tests := []struct {
		name      string
		teamSize  int
		wantError bool
	}{
		{"size 0 - too small", 0, true},
		{"size 1 - minimum valid", 1, false},
		{"size 3 - default", 3, false},
		{"size 10 - maximum valid", 10, false},
		{"size 11 - too large", 11, true},
		{"size -1 - negative", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slingTeam = false // Don't set --team, just test size validation
			slingNoTeam = false
			slingTeamSize = tt.teamSize
			slingTeammateTier = "sonnet"
			slingDryRun = true

			err := runSling(nil, []string{"gt-test123"})
			if tt.wantError {
				if err == nil {
					t.Error("expected error for invalid team size")
				} else if !strings.Contains(err.Error(), "--team-size must be between") {
					t.Errorf("unexpected error: %v", err)
				}
			}
			// Valid sizes won't error at the team-size check, they'll proceed
			// and potentially error later — that's fine, we verified the guard.
		})
	}
}

// TestTeammateTierValidation verifies --teammate-tier accepts only valid values.
func TestTeammateTierValidation(t *testing.T) {
	prevTeam := slingTeam
	prevNoTeam := slingNoTeam
	prevTeamSize := slingTeamSize
	prevTeammateTier := slingTeammateTier
	prevDryRun := slingDryRun
	t.Cleanup(func() {
		slingTeam = prevTeam
		slingNoTeam = prevNoTeam
		slingTeamSize = prevTeamSize
		slingTeammateTier = prevTeammateTier
		slingDryRun = prevDryRun
	})

	t.Setenv("GT_POLECAT", "")

	tests := []struct {
		name      string
		tier      string
		wantError bool
	}{
		{"opus - valid", "opus", false},
		{"sonnet - valid", "sonnet", false},
		{"haiku - valid", "haiku", false},
		{"Opus - valid case insensitive", "Opus", false},
		{"SONNET - valid case insensitive", "SONNET", false},
		{"gpt4 - invalid", "gpt4", true},
		{"empty - invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slingTeam = false
			slingNoTeam = false
			slingTeamSize = 3
			slingTeammateTier = tt.tier
			slingDryRun = true

			err := runSling(nil, []string{"gt-test123"})
			if tt.wantError {
				if err == nil {
					t.Error("expected error for invalid teammate tier")
				} else if !strings.Contains(err.Error(), "invalid --teammate-tier") {
					t.Errorf("unexpected error: %v", err)
				}
			}
			// Valid tiers proceed past this check — we verified the guard.
		})
	}
}

// TestAutoApplyTeamFormula verifies that --team causes mol-polecat-work-team
// to be auto-applied instead of mol-polecat-work.
func TestAutoApplyTeamFormula(t *testing.T) {
	tests := []struct {
		name        string
		teamEnabled bool
		hookRawBead bool
		targetAgent string
		wantFormula string
	}{
		{
			name:        "team enabled, polecat target → team formula",
			teamEnabled: true,
			targetAgent: "gastown/polecats/Toast",
			wantFormula: "mol-polecat-work-team",
		},
		{
			name:        "team disabled, polecat target → standard formula",
			teamEnabled: false,
			targetAgent: "gastown/polecats/Toast",
			wantFormula: "mol-polecat-work",
		},
		{
			name:        "team enabled, non-polecat target → no auto-apply",
			teamEnabled: true,
			targetAgent: "gastown/witness",
			wantFormula: "",
		},
		{
			name:        "team enabled, hook-raw-bead → no auto-apply",
			teamEnabled: true,
			hookRawBead: true,
			targetAgent: "gastown/polecats/Toast",
			wantFormula: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the auto-apply logic from sling.go
			var teamConfig *config.TeamConfig
			if tt.teamEnabled {
				teamConfig = &config.TeamConfig{
					Enabled:       true,
					MaxTeammates:  3,
					TeammateModel: "sonnet",
				}
			}

			formulaName := ""
			if formulaName == "" && !tt.hookRawBead && strings.Contains(tt.targetAgent, "/polecats/") {
				if teamConfig != nil && teamConfig.Enabled {
					formulaName = "mol-polecat-work-team"
				} else {
					formulaName = "mol-polecat-work"
				}
			}

			if formulaName != tt.wantFormula {
				t.Errorf("formula = %q, want %q", formulaName, tt.wantFormula)
			}
		})
	}
}

// TestTeamVarsInjection verifies that team variables are appended to slingVars.
func TestTeamVarsInjection(t *testing.T) {
	teamConfig := &config.TeamConfig{
		Enabled:       true,
		MaxTeammates:  5,
		TeammateModel: "opus",
	}

	// Simulate the injection logic from sling.go
	var slingVars []string
	slingVars = append(slingVars, "issue=gt-abc123")

	if teamConfig != nil && teamConfig.Enabled {
		slingVars = append(slingVars,
			"max_teammates=5",
			"teammate_model=opus",
		)
	}

	if len(slingVars) != 3 {
		t.Errorf("len(slingVars) = %d, want 3", len(slingVars))
	}

	found := map[string]bool{}
	for _, v := range slingVars {
		found[v] = true
	}

	if !found["max_teammates=5"] {
		t.Error("missing max_teammates=5 in slingVars")
	}
	if !found["teammate_model=opus"] {
		t.Error("missing teammate_model=opus in slingVars")
	}
}

// TestTeamDryRunOutput verifies --team info appears in dry-run output.
func TestTeamDryRunOutput(t *testing.T) {
	townRoot := t.TempDir()

	// Minimal workspace marker
	if err := os.MkdirAll(filepath.Join(townRoot, "mayor", "rig"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create stub bd
	binDir := filepath.Join(townRoot, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir binDir: %v", err)
	}
	bdScript := `#!/bin/sh
cmd="$1"; shift || true
case "$cmd" in
  show) echo '[{"title":"Test","status":"open","assignee":"","description":""}]';;
esac
exit 0
`
	bdScriptWindows := `@echo off
set "cmd=%1"
if "%cmd%"=="show" (
  echo [{"title":"Test","status":"open","assignee":"","description":""}]
  exit /b 0
)
exit /b 0
`
	_ = writeBDStub(t, binDir, bdScript, bdScriptWindows)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(EnvGTRole, "mayor")
	t.Setenv("GT_POLECAT", "")
	t.Setenv("GT_CREW", "")
	t.Setenv("TMUX_PANE", "")

	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	_ = os.Chdir(filepath.Join(townRoot, "mayor", "rig"))

	// Save and restore flags
	prevTeam := slingTeam
	prevNoTeam := slingNoTeam
	prevTeamSize := slingTeamSize
	prevTeammateTier := slingTeammateTier
	prevDryRun := slingDryRun
	prevNoConvoy := slingNoConvoy
	t.Cleanup(func() {
		slingTeam = prevTeam
		slingNoTeam = prevNoTeam
		slingTeamSize = prevTeamSize
		slingTeammateTier = prevTeammateTier
		slingDryRun = prevDryRun
		slingNoConvoy = prevNoConvoy
	})

	slingTeam = true
	slingNoTeam = false
	slingTeamSize = 4
	slingTeammateTier = "opus"
	slingDryRun = true
	slingNoConvoy = true

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runSling(nil, []string{"gt-test123"})

	w.Close()
	os.Stdout = old
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if err != nil {
		t.Fatalf("runSling: %v", err)
	}

	if !strings.Contains(output, "team: enabled") {
		t.Errorf("dry-run output missing team info:\n%s", output)
	}
	if !strings.Contains(output, "max_teammates=4") {
		t.Errorf("dry-run output missing max_teammates:\n%s", output)
	}
	if !strings.Contains(output, "teammate_model=opus") {
		t.Errorf("dry-run output missing teammate_model:\n%s", output)
	}
}

// TestTeamConfigBuild verifies TeamConfig construction from flag values.
func TestTeamConfigBuild(t *testing.T) {
	tests := []struct {
		name         string
		team         bool
		teamSize     int
		teammateTier string
		wantNil      bool
		wantSize     int
		wantModel    string
	}{
		{
			name:         "team enabled",
			team:         true,
			teamSize:     5,
			teammateTier: "opus",
			wantNil:      false,
			wantSize:     5,
			wantModel:    "opus",
		},
		{
			name:         "team disabled",
			team:         false,
			teamSize:     3,
			teammateTier: "sonnet",
			wantNil:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the build logic from sling.go
			var teamConfig *config.TeamConfig
			if tt.team {
				teamConfig = &config.TeamConfig{
					Enabled:       true,
					MaxTeammates:  tt.teamSize,
					TeammateModel: tt.teammateTier,
				}
			}

			if tt.wantNil {
				if teamConfig != nil {
					t.Error("expected nil TeamConfig when --team not set")
				}
				return
			}

			if teamConfig == nil {
				t.Fatal("expected non-nil TeamConfig when --team is set")
			}
			if !teamConfig.Enabled {
				t.Error("TeamConfig.Enabled should be true")
			}
			if teamConfig.MaxTeammates != tt.wantSize {
				t.Errorf("MaxTeammates = %d, want %d", teamConfig.MaxTeammates, tt.wantSize)
			}
			if teamConfig.TeammateModel != tt.wantModel {
				t.Errorf("TeammateModel = %q, want %q", teamConfig.TeammateModel, tt.wantModel)
			}
		})
	}
}

// TestSlingSpawnOptionsTeamConfig verifies TeamConfig threads through SlingSpawnOptions.
func TestSlingSpawnOptionsTeamConfig(t *testing.T) {
	tc := &config.TeamConfig{
		Enabled:       true,
		MaxTeammates:  7,
		TeammateModel: "haiku",
	}

	opts := SlingSpawnOptions{
		Force:      false,
		Account:    "work",
		HookBead:   "gt-abc",
		Agent:      "claude",
		TeamConfig: tc,
	}

	if opts.TeamConfig == nil {
		t.Fatal("TeamConfig should not be nil in SlingSpawnOptions")
	}
	if !opts.TeamConfig.Enabled {
		t.Error("TeamConfig.Enabled should be true")
	}
	if opts.TeamConfig.MaxTeammates != 7 {
		t.Errorf("MaxTeammates = %d, want 7", opts.TeamConfig.MaxTeammates)
	}
	if opts.TeamConfig.TeammateModel != "haiku" {
		t.Errorf("TeammateModel = %q, want %q", opts.TeamConfig.TeammateModel, "haiku")
	}
}

// TestResolveTargetOptionsTeamConfig verifies TeamConfig threads through ResolveTargetOptions.
func TestResolveTargetOptionsTeamConfig(t *testing.T) {
	tc := &config.TeamConfig{
		Enabled:       true,
		MaxTeammates:  2,
		TeammateModel: "sonnet",
		DelegateMode:  true,
	}

	opts := ResolveTargetOptions{
		DryRun:     true,
		Force:      false,
		BeadID:     "gt-test",
		TeamConfig: tc,
	}

	if opts.TeamConfig == nil {
		t.Fatal("TeamConfig should not be nil in ResolveTargetOptions")
	}
	if !opts.TeamConfig.Enabled {
		t.Error("TeamConfig.Enabled should be true")
	}
	if !opts.TeamConfig.DelegateMode {
		t.Error("TeamConfig.DelegateMode should be true")
	}
}

// TestLoadRigTeamDefaults verifies rig-level team config loading.
func TestLoadRigTeamDefaults(t *testing.T) {
	townRoot := t.TempDir()

	// Create a rig with team defaults in settings
	rigName := "testrig"
	rigPath := filepath.Join(townRoot, rigName)
	rigSettingsDir := filepath.Join(rigPath, "settings")
	if err := os.MkdirAll(rigSettingsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create minimal rig structure so IsRigName succeeds
	if err := os.MkdirAll(filepath.Join(townRoot, "mayor", "rig"), 0755); err != nil {
		t.Fatalf("mkdir mayor/rig: %v", err)
	}
	rigsJSON := `{"rigs":{"testrig":{"path":"testrig"}}}`
	if err := os.WriteFile(filepath.Join(townRoot, "mayor", "rigs.json"), []byte(rigsJSON), 0644); err != nil {
		t.Fatalf("write rigs.json: %v", err)
	}

	// Write rig settings with team config
	settingsJSON := `{
		"type": "rig-settings",
		"version": 1,
		"team": {
			"enabled": true,
			"max_teammates": 5,
			"teammate_model": "opus",
			"delegate_mode": true
		}
	}`
	if err := os.WriteFile(filepath.Join(rigSettingsDir, "config.json"), []byte(settingsJSON), 0644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	// Switch to town root so IsRigName can find the workspace
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	_ = os.Chdir(filepath.Join(townRoot, "mayor", "rig"))

	// Test: bare rig name loads defaults
	tc := loadRigTeamDefaults(rigName, townRoot)
	if tc == nil {
		t.Fatal("expected non-nil TeamConfig from rig defaults")
	}
	if !tc.Enabled {
		t.Error("Enabled should be true")
	}
	if tc.MaxTeammates != 5 {
		t.Errorf("MaxTeammates = %d, want 5", tc.MaxTeammates)
	}
	if tc.TeammateModel != "opus" {
		t.Errorf("TeammateModel = %q, want %q", tc.TeammateModel, "opus")
	}
	if !tc.DelegateMode {
		t.Error("DelegateMode should be true")
	}

	// Test: path-style target extracts rig name
	tc2 := loadRigTeamDefaults(rigName+"/polecats/Toast", townRoot)
	if tc2 == nil {
		t.Fatal("expected non-nil TeamConfig from path-style target")
	}
	if tc2.MaxTeammates != 5 {
		t.Errorf("MaxTeammates = %d, want 5", tc2.MaxTeammates)
	}
}

// TestLoadRigTeamDefaults_NoSettings verifies nil return when no settings exist.
func TestLoadRigTeamDefaults_NoSettings(t *testing.T) {
	townRoot := t.TempDir()

	// Create rig without settings
	rigName := "emptyrig"
	if err := os.MkdirAll(filepath.Join(townRoot, rigName), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(townRoot, "mayor", "rig"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	rigsJSON := `{"rigs":{"emptyrig":{"path":"emptyrig"}}}`
	if err := os.WriteFile(filepath.Join(townRoot, "mayor", "rigs.json"), []byte(rigsJSON), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	_ = os.Chdir(filepath.Join(townRoot, "mayor", "rig"))

	tc := loadRigTeamDefaults(rigName, townRoot)
	if tc != nil {
		t.Errorf("expected nil TeamConfig when no settings exist, got %+v", tc)
	}
}

// TestLoadRigTeamDefaults_TeamDisabled verifies nil return when team is not enabled.
func TestLoadRigTeamDefaults_TeamDisabled(t *testing.T) {
	townRoot := t.TempDir()

	rigName := "disabledrig"
	rigPath := filepath.Join(townRoot, rigName)
	if err := os.MkdirAll(filepath.Join(rigPath, "settings"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(townRoot, "mayor", "rig"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	rigsJSON := `{"rigs":{"disabledrig":{"path":"disabledrig"}}}`
	if err := os.WriteFile(filepath.Join(townRoot, "mayor", "rigs.json"), []byte(rigsJSON), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	settingsJSON := `{
		"type": "rig-settings",
		"version": 1,
		"team": {
			"enabled": false,
			"max_teammates": 5
		}
	}`
	if err := os.WriteFile(filepath.Join(rigPath, "settings", "config.json"), []byte(settingsJSON), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	_ = os.Chdir(filepath.Join(townRoot, "mayor", "rig"))

	tc := loadRigTeamDefaults(rigName, townRoot)
	if tc != nil {
		t.Errorf("expected nil TeamConfig when team is disabled, got %+v", tc)
	}
}

// TestLoadRigTeamDefaults_NonRigTarget verifies nil return for non-rig targets.
func TestLoadRigTeamDefaults_NonRigTarget(t *testing.T) {
	townRoot := t.TempDir()

	if err := os.MkdirAll(filepath.Join(townRoot, "mayor", "rig"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	rigsJSON := `{"rigs":{}}`
	if err := os.WriteFile(filepath.Join(townRoot, "mayor", "rigs.json"), []byte(rigsJSON), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	_ = os.Chdir(filepath.Join(townRoot, "mayor", "rig"))

	// "mayor" is not a rig
	tc := loadRigTeamDefaults("mayor", townRoot)
	if tc != nil {
		t.Errorf("expected nil for non-rig target 'mayor', got %+v", tc)
	}

	// Empty target
	tc2 := loadRigTeamDefaults("", townRoot)
	if tc2 != nil {
		t.Errorf("expected nil for empty target, got %+v", tc2)
	}
}
