package formula

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestParseTeamFormula verifies mol-polecat-work-team.formula.toml parses correctly.
func TestParseTeamFormula(t *testing.T) {
	// Find the formula file relative to this test file
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	formulaDir := filepath.Join(filepath.Dir(testFile), "formulas")
	formulaPath := filepath.Join(formulaDir, "mol-polecat-work-team.formula.toml")

	// Verify file exists
	if _, err := os.Stat(formulaPath); os.IsNotExist(err) {
		t.Fatalf("formula file does not exist: %s", formulaPath)
	}

	f, err := ParseFile(formulaPath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Verify basic metadata
	if f.Name != "mol-polecat-work-team" {
		t.Errorf("Name = %q, want %q", f.Name, "mol-polecat-work-team")
	}
	if f.Version != 1 {
		t.Errorf("Version = %d, want 1", f.Version)
	}

	// Verify step count (same steps as mol-polecat-work: 10 steps)
	expectedSteps := 10
	if len(f.Steps) != expectedSteps {
		t.Errorf("len(Steps) = %d, want %d", len(f.Steps), expectedSteps)
	}

	// Verify expected step IDs exist
	expectedIDs := []string{
		"load-context",
		"branch-setup",
		"preflight-tests",
		"implement",
		"self-review",
		"run-tests",
		"commit-changes",
		"cleanup-workspace",
		"prepare-for-review",
		"submit-and-exit",
	}
	stepIDs := make(map[string]bool)
	for _, step := range f.Steps {
		stepIDs[step.ID] = true
	}
	for _, id := range expectedIDs {
		if !stepIDs[id] {
			t.Errorf("missing step ID: %q", id)
		}
	}

	// Verify implement step has team-specific content
	var implementStep *Step
	for i := range f.Steps {
		if f.Steps[i].ID == "implement" {
			implementStep = &f.Steps[i]
			break
		}
	}
	if implementStep == nil {
		t.Fatal("implement step not found")
	}
	if implementStep.Title != "Implement the solution (team-coordinated)" {
		t.Errorf("implement step title = %q, want team-coordinated title", implementStep.Title)
	}

	// Verify variables include team-specific vars
	if f.Vars == nil {
		t.Fatal("Vars is nil")
	}
	if _, ok := f.Vars["max_teammates"]; !ok {
		t.Error("missing var: max_teammates")
	}
	if _, ok := f.Vars["teammate_model"]; !ok {
		t.Error("missing var: teammate_model")
	}
	if _, ok := f.Vars["issue"]; !ok {
		t.Error("missing var: issue")
	}

	// Verify issue var is required
	if issueVar, ok := f.Vars["issue"]; ok {
		if !issueVar.Required {
			t.Error("issue var should be required")
		}
	}

	// Verify team vars have defaults
	if maxTeammatesVar, ok := f.Vars["max_teammates"]; ok {
		if maxTeammatesVar.Default == "" {
			t.Error("max_teammates var should have a default value")
		}
	}
	if teammateModelVar, ok := f.Vars["teammate_model"]; ok {
		if teammateModelVar.Default == "" {
			t.Error("teammate_model var should have a default value")
		}
	}
}

// TestTeamFormulaTopologicalSort verifies step dependencies form a valid DAG.
func TestTeamFormulaTopologicalSort(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	formulaDir := filepath.Join(filepath.Dir(testFile), "formulas")
	formulaPath := filepath.Join(formulaDir, "mol-polecat-work-team.formula.toml")

	f, err := ParseFile(formulaPath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	order, err := f.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort failed (cycle in dependencies?): %v", err)
	}

	if len(order) != len(f.Steps) {
		t.Errorf("TopologicalSort returned %d steps, want %d", len(order), len(f.Steps))
	}

	// Verify implement comes after preflight-tests
	indexOf := func(id string) int {
		for i, x := range order {
			if x == id {
				return i
			}
		}
		return -1
	}
	if indexOf("preflight-tests") >= indexOf("implement") {
		t.Error("preflight-tests should come before implement in topological order")
	}
	if indexOf("implement") >= indexOf("self-review") {
		t.Error("implement should come before self-review in topological order")
	}
	if indexOf("submit-and-exit") < indexOf("prepare-for-review") {
		t.Error("submit-and-exit should come after prepare-for-review")
	}
}

// TestTeamFormulaReadySteps verifies correct initial ready steps.
func TestTeamFormulaReadySteps(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	formulaDir := filepath.Join(filepath.Dir(testFile), "formulas")
	formulaPath := filepath.Join(formulaDir, "mol-polecat-work-team.formula.toml")

	f, err := ParseFile(formulaPath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Initially only load-context should be ready (no deps)
	ready := f.ReadySteps(map[string]bool{})
	if len(ready) != 1 {
		t.Errorf("ReadySteps({}) = %v, want exactly 1 step", ready)
	}
	if len(ready) > 0 && ready[0] != "load-context" {
		t.Errorf("first ready step = %q, want %q", ready[0], "load-context")
	}
}

// TestOriginalFormulaStillParses ensures the original formula isn't broken.
func TestOriginalFormulaStillParses(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	formulaDir := filepath.Join(filepath.Dir(testFile), "formulas")
	formulaPath := filepath.Join(formulaDir, "mol-polecat-work.formula.toml")

	f, err := ParseFile(formulaPath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if f.Name != "mol-polecat-work" {
		t.Errorf("Name = %q, want %q", f.Name, "mol-polecat-work")
	}
}
