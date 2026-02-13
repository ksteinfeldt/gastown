package user

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetCurrentUser_FromEnv(t *testing.T) {
	// Set GT_USER
	os.Setenv(EnvVarGTUser, "testuser")
	defer os.Unsetenv(EnvVarGTUser)

	u, err := GetCurrentUser()
	if err != nil {
		t.Fatalf("GetCurrentUser: %v", err)
	}
	if u != "testuser" {
		t.Errorf("user = %q, want %q", u, "testuser")
	}
}

func TestGetCurrentUser_NoContext(t *testing.T) {
	// Clear all sources
	os.Unsetenv(EnvVarGTUser)
	os.Unsetenv("TMUX")

	// Temporarily rename the current user file if it exists
	path := currentUserFilePath()
	if path != "" {
		bakPath := path + ".test.bak"
		if _, err := os.Stat(path); err == nil {
			os.Rename(path, bakPath)
			defer os.Rename(bakPath, path)
		}
	}

	u, err := GetCurrentUser()
	if err != nil {
		t.Fatalf("GetCurrentUser: %v", err)
	}
	if u != "" {
		t.Errorf("user = %q, want empty string", u)
	}
}

func TestSetCurrentUser(t *testing.T) {
	// Clear env first
	os.Unsetenv(EnvVarGTUser)
	os.Unsetenv("TMUX")

	// Use a temp home dir
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	err := SetCurrentUser("alice")
	if err != nil {
		t.Fatalf("SetCurrentUser: %v", err)
	}

	// Check env var was set
	if got := os.Getenv(EnvVarGTUser); got != "alice" {
		t.Errorf("GT_USER = %q, want %q", got, "alice")
	}

	// Check file was written
	filePath := filepath.Join(tmpHome, CurrentUserFileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("reading current user file: %v", err)
	}
	if got := string(data); got != "alice\n" {
		t.Errorf("file content = %q, want %q", got, "alice\n")
	}

	// Verify GetCurrentUser reads it back
	os.Unsetenv(EnvVarGTUser)
	u, err := GetCurrentUser()
	if err != nil {
		t.Fatalf("GetCurrentUser: %v", err)
	}
	if u != "alice" {
		t.Errorf("user = %q, want %q", u, "alice")
	}
}

func TestEnvVarPriority(t *testing.T) {
	// Set both env var and file
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Write file with "bob"
	filePath := filepath.Join(tmpHome, CurrentUserFileName)
	os.WriteFile(filePath, []byte("bob\n"), 0644)

	// Set env to "alice"
	os.Setenv(EnvVarGTUser, "alice")
	defer os.Unsetenv(EnvVarGTUser)

	// Env should win
	u, err := GetCurrentUser()
	if err != nil {
		t.Fatalf("GetCurrentUser: %v", err)
	}
	if u != "alice" {
		t.Errorf("user = %q, want %q (env should take priority)", u, "alice")
	}
}
