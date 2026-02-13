// Package user provides user identity management for multi-overseer Gas Town.
package user

import (
	"time"
)

// CurrentRegistryVersion is the current schema version for the user registry.
const CurrentRegistryVersion = 1

// SourceGitConfig indicates user was detected from git config.
const SourceGitConfig = "git-config"

// SourceGitHubCLI indicates user was detected from GitHub CLI.
const SourceGitHubCLI = "github-cli"

// SourceEnvironment indicates user was detected from environment variables.
const SourceEnvironment = "environment"

// SourceManual indicates user was manually added.
const SourceManual = "manual"

// User represents a human user (overseer) in the Gas Town workspace.
type User struct {
	// Username is the unique identifier for this user.
	Username string `json:"username"`

	// Name is the display name.
	Name string `json:"name"`

	// Email is the user's email address (optional).
	Email string `json:"email,omitempty"`

	// Added is when this user was registered.
	Added time.Time `json:"added"`

	// Source indicates how this user was detected/added.
	Source string `json:"source"`

	// Metadata holds optional key-value data.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Registry holds all users in the Gas Town workspace.
// Stored at mayor/users.json.
type Registry struct {
	// Version is the schema version.
	Version int `json:"version"`

	// Users is the list of registered users.
	Users []User `json:"users"`
}
