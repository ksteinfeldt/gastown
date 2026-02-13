package user

import (
	"os/exec"
	"strings"
	"time"
)

// Detect attempts to detect a user identity from available sources.
// Priority order:
//  1. Git config (user.name + user.email)
//  2. GitHub CLI (gh api user)
//  3. Environment ($USER or whoami)
func Detect(workDir string) *User {
	if u := detectFromGitConfig(workDir); u != nil {
		return u
	}

	if u := detectFromGitHub(); u != nil {
		return u
	}

	return detectFromEnvironment()
}

// detectFromGitConfig attempts to get user identity from git config.
func detectFromGitConfig(dir string) *User {
	nameCmd := exec.Command("git", "config", "user.name")
	if dir != "" {
		nameCmd.Dir = dir
	}
	nameOut, err := nameCmd.Output()
	if err != nil {
		return nil
	}
	name := strings.TrimSpace(string(nameOut))
	if name == "" {
		return nil
	}

	u := &User{
		Name:   name,
		Source: SourceGitConfig,
		Added:  time.Now().UTC(),
	}

	// Try to get email
	emailCmd := exec.Command("git", "config", "user.email")
	if dir != "" {
		emailCmd.Dir = dir
	}
	if emailOut, err := emailCmd.Output(); err == nil {
		u.Email = strings.TrimSpace(string(emailOut))
	}

	// Derive username from email or name
	u.Username = deriveUsername(name, u.Email)

	return u
}

// detectFromGitHub attempts to get user identity from GitHub CLI.
func detectFromGitHub() *User {
	cmd := exec.Command("gh", "api", "user", "--jq", ".login + \"|\" + .name + \"|\" + .email")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	parts := strings.Split(strings.TrimSpace(string(out)), "|")
	if len(parts) < 1 || parts[0] == "" {
		return nil
	}

	u := &User{
		Username: parts[0],
		Source:   SourceGitHubCLI,
		Added:    time.Now().UTC(),
	}

	if len(parts) >= 2 && parts[1] != "" {
		u.Name = parts[1]
	} else {
		u.Name = parts[0]
	}

	if len(parts) >= 3 && parts[2] != "" {
		u.Email = parts[2]
	}

	return u
}

// detectFromEnvironment falls back to OS environment variables.
func detectFromEnvironment() *User {
	username := exec.Command("whoami")
	out, err := username.Output()

	var name string
	if err == nil {
		name = strings.TrimSpace(string(out))
	}
	if name == "" {
		name = "user"
	}

	return &User{
		Username: name,
		Name:     name,
		Source:   SourceEnvironment,
		Added:    time.Now().UTC(),
	}
}

// deriveUsername generates a username from a name and email.
// If email is provided, uses the local part. Otherwise lowercases and
// hyphenates the name.
func deriveUsername(name, email string) string {
	if email != "" {
		if idx := strings.Index(email, "@"); idx > 0 {
			return strings.ToLower(email[:idx])
		}
	}

	// Fall back to lowercased name with spaces replaced
	lower := strings.ToLower(name)
	lower = strings.ReplaceAll(lower, " ", "-")
	// Remove characters that aren't alphanumeric or hyphens
	var cleaned strings.Builder
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			cleaned.WriteRune(r)
		}
	}
	result := cleaned.String()
	if result == "" {
		return "user"
	}
	return result
}
