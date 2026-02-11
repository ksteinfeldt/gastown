package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestTeamConfig_JSONRoundTrip verifies TeamConfig serializes and deserializes correctly.
func TestTeamConfig_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := &TeamConfig{
		Enabled:       true,
		MaxTeammates:  5,
		TeammateModel: "opus",
		DelegateMode:  true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var loaded TeamConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if loaded.Enabled != true {
		t.Error("Enabled should be true")
	}
	if loaded.MaxTeammates != 5 {
		t.Errorf("MaxTeammates = %d, want 5", loaded.MaxTeammates)
	}
	if loaded.TeammateModel != "opus" {
		t.Errorf("TeammateModel = %q, want %q", loaded.TeammateModel, "opus")
	}
	if loaded.DelegateMode != true {
		t.Error("DelegateMode should be true")
	}
}

// TestTeamConfig_OmitEmpty verifies omitempty fields are excluded when zero.
func TestTeamConfig_OmitEmpty(t *testing.T) {
	t.Parallel()

	tc := &TeamConfig{
		Enabled: true,
	}

	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	jsonStr := string(data)
	if contains(jsonStr, "max_teammates") {
		t.Error("zero MaxTeammates should be omitted from JSON")
	}
	if contains(jsonStr, "teammate_model") {
		t.Error("empty TeammateModel should be omitted from JSON")
	}
	if contains(jsonStr, "delegate_mode") {
		t.Error("false DelegateMode should be omitted from JSON")
	}
}

// TestRigSettings_TeamConfig_RoundTrip verifies TeamConfig survives RigSettings save/load.
func TestRigSettings_TeamConfig_RoundTrip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings", "config.json")

	original := &RigSettings{
		Type:    "rig-settings",
		Version: 1,
		Team: &TeamConfig{
			Enabled:       true,
			MaxTeammates:  4,
			TeammateModel: "haiku",
			DelegateMode:  true,
		},
	}

	if err := SaveRigSettings(settingsPath, original); err != nil {
		t.Fatalf("SaveRigSettings: %v", err)
	}

	loaded, err := LoadRigSettings(settingsPath)
	if err != nil {
		t.Fatalf("LoadRigSettings: %v", err)
	}

	if loaded.Team == nil {
		t.Fatal("Team config is nil after round-trip")
	}
	if !loaded.Team.Enabled {
		t.Error("Team.Enabled should be true")
	}
	if loaded.Team.MaxTeammates != 4 {
		t.Errorf("Team.MaxTeammates = %d, want 4", loaded.Team.MaxTeammates)
	}
	if loaded.Team.TeammateModel != "haiku" {
		t.Errorf("Team.TeammateModel = %q, want %q", loaded.Team.TeammateModel, "haiku")
	}
	if !loaded.Team.DelegateMode {
		t.Error("Team.DelegateMode should be true")
	}
}

// TestRigSettings_NoTeamConfig verifies Team is nil when not set.
func TestRigSettings_NoTeamConfig(t *testing.T) {
	t.Parallel()

	settingsJSON := `{
		"type": "rig-settings",
		"version": 1
	}`

	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(settingsPath, []byte(settingsJSON), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := LoadRigSettings(settingsPath)
	if err != nil {
		t.Fatalf("LoadRigSettings: %v", err)
	}

	if loaded.Team != nil {
		t.Errorf("Team should be nil when not set in JSON, got %+v", loaded.Team)
	}
}

// TestRigSettings_PartialTeamConfig verifies partial TeamConfig loads correctly.
func TestRigSettings_PartialTeamConfig(t *testing.T) {
	t.Parallel()

	settingsJSON := `{
		"type": "rig-settings",
		"version": 1,
		"team": {
			"enabled": true
		}
	}`

	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(settingsPath, []byte(settingsJSON), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := LoadRigSettings(settingsPath)
	if err != nil {
		t.Fatalf("LoadRigSettings: %v", err)
	}

	if loaded.Team == nil {
		t.Fatal("Team should not be nil")
	}
	if !loaded.Team.Enabled {
		t.Error("Team.Enabled should be true")
	}
	if loaded.Team.MaxTeammates != 0 {
		t.Errorf("Team.MaxTeammates should be 0 (unset), got %d", loaded.Team.MaxTeammates)
	}
	if loaded.Team.TeammateModel != "" {
		t.Errorf("Team.TeammateModel should be empty (unset), got %q", loaded.Team.TeammateModel)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && json.Valid([]byte(s)) && jsonContains(s, substr)
}

func jsonContains(s, key string) bool {
	for i := 0; i <= len(s)-len(key); i++ {
		if s[i:i+len(key)] == key {
			return true
		}
	}
	return false
}
