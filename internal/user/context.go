package user

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnvVarGTUser is the environment variable for the current Gas Town user.
const EnvVarGTUser = "GT_USER"

// CurrentUserFileName is the file that stores the current user persistently.
const CurrentUserFileName = ".gt-current-user"

// GetCurrentUser determines the current user from available context.
// Priority order:
//  1. GT_USER environment variable (explicit override)
//  2. Tmux session metadata
//  3. ~/.gt-current-user file
//
// Returns ("", nil) if no user context is found.
func GetCurrentUser() (string, error) {
	// Priority 1: GT_USER environment variable
	if user := os.Getenv(EnvVarGTUser); user != "" {
		return user, nil
	}

	// Priority 2: Tmux session metadata
	if user := getUserFromTmux(); user != "" {
		return user, nil
	}

	// Priority 3: ~/.gt-current-user file
	if user, err := loadCurrentUserFile(); err == nil && user != "" {
		return user, nil
	}

	return "", nil
}

// SetCurrentUser sets the current user in available persistence layers.
func SetCurrentUser(username string) error {
	// Set in environment (for current process and children)
	if err := os.Setenv(EnvVarGTUser, username); err != nil {
		return fmt.Errorf("setting %s: %w", EnvVarGTUser, err)
	}

	// Set in tmux (if in tmux session)
	setUserInTmux(username)

	// Save to ~/.gt-current-user (persistent)
	return saveCurrentUserFile(username)
}

// getUserFromTmux tries to read GT_USER from tmux session environment.
func getUserFromTmux() string {
	// Check if we're in tmux
	if os.Getenv("TMUX") == "" {
		return ""
	}

	cmd := exec.Command("tmux", "show-environment", EnvVarGTUser)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Output format: GT_USER=alice
	line := strings.TrimSpace(string(out))
	if !strings.HasPrefix(line, EnvVarGTUser+"=") {
		return ""
	}

	return strings.TrimPrefix(line, EnvVarGTUser+"=")
}

// setUserInTmux sets GT_USER in the tmux session environment.
func setUserInTmux(username string) {
	if os.Getenv("TMUX") == "" {
		return
	}

	cmd := exec.Command("tmux", "set-environment", EnvVarGTUser, username)
	_ = cmd.Run()
}

// currentUserFilePath returns the path to the persistent current user file.
func currentUserFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, CurrentUserFileName)
}

// loadCurrentUserFile reads the username from the persistent file.
func loadCurrentUserFile() (string, error) {
	path := currentUserFilePath()
	if path == "" {
		return "", fmt.Errorf("cannot determine home directory")
	}

	data, err := os.ReadFile(path) //nolint:gosec // G304: path from user home
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

// saveCurrentUserFile writes the username to the persistent file.
func saveCurrentUserFile(username string) error {
	path := currentUserFilePath()
	if path == "" {
		return fmt.Errorf("cannot determine home directory")
	}

	return os.WriteFile(path, []byte(username+"\n"), 0644) //nolint:gosec // G306: not secret
}
