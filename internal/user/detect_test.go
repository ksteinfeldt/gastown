package user

import (
	"testing"
)

func TestDeriveUsername(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{name: "Alice Smith", email: "alice@example.com", want: "alice"},
		{name: "Bob Jones", email: "bob.jones@corp.co", want: "bob.jones"},
		{name: "Charlie", email: "", want: "charlie"},
		{name: "Dave Williams", email: "", want: "dave-williams"},
		{name: "Ève Dupont", email: "", want: "ve-dupont"},
		{name: "", email: "user@test.com", want: "user"},
		{name: "", email: "", want: "user"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_"+tt.email, func(t *testing.T) {
			got := deriveUsername(tt.name, tt.email)
			if got != tt.want {
				t.Errorf("deriveUsername(%q, %q) = %q, want %q", tt.name, tt.email, got, tt.want)
			}
		})
	}
}

func TestDetect_FallsBackToEnvironment(t *testing.T) {
	// Detect with a non-git directory should fall back to environment
	u := Detect(t.TempDir())
	if u == nil {
		t.Fatal("Detect returned nil")
	}
	if u.Username == "" {
		t.Error("username should not be empty")
	}
	if u.Name == "" {
		t.Error("name should not be empty")
	}
	// Source should be one of the known sources
	validSources := map[string]bool{
		SourceGitConfig:   true,
		SourceGitHubCLI:   true,
		SourceEnvironment: true,
	}
	if !validSources[u.Source] {
		t.Errorf("source = %q, want one of git-config, github-cli, environment", u.Source)
	}
}
