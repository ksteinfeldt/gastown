package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/user"
	"github.com/steveyegge/gastown/internal/workspace"
)

var userCmd = &cobra.Command{
	Use:     "user",
	GroupID: GroupConfig,
	Short:   "Manage users in the workspace",
	Long: `Manage human users (overseers) in the Gas Town workspace.

Gas Town supports multiple human users collaborating in a single workspace.
Each user has a unique username, owns their rigs, and has their own identity
for work attribution.

Examples:
  gt user list              # Show all users
  gt user whoami            # Show current user
  gt user add alice         # Add a new user
  gt user switch bob        # Switch to another user`,
	RunE: requireSubcommand,
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all users in the workspace",
	Long: `List all registered users in the Gas Town workspace.

Shows each user's username, display name, and email.
The current user is marked with an asterisk (*).`,
	RunE: runUserList,
}

var userWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current user identity",
	Long: `Show the currently active user identity.

User identity is determined by:
1. GT_USER environment variable
2. Tmux session metadata
3. ~/.gt-current-user file

If no user is set, shows the detected identity from git/GitHub.`,
	RunE: runUserWhoami,
}

var userAddCmd = &cobra.Command{
	Use:   "add <username>",
	Short: "Add a new user to the workspace",
	Long: `Register a new user in the Gas Town workspace.

If --name and --email are not provided, they are detected from
git config or GitHub CLI.

Examples:
  gt user add alice                           # Auto-detect identity
  gt user add bob --name "Bob Jones" --email bob@example.com`,
	Args: cobra.ExactArgs(1),
	RunE: runUserAdd,
}

var userSwitchCmd = &cobra.Command{
	Use:   "switch <username>",
	Short: "Switch to a different user",
	Long: `Switch the current active user identity.

This sets the GT_USER environment variable, tmux session metadata,
and the persistent ~/.gt-current-user file.

Example:
  gt user switch alice`,
	Args: cobra.ExactArgs(1),
	RunE: runUserSwitch,
}

var (
	userAddName  string
	userAddEmail string
)

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userListCmd)
	userCmd.AddCommand(userWhoamiCmd)
	userCmd.AddCommand(userAddCmd)
	userCmd.AddCommand(userSwitchCmd)

	userAddCmd.Flags().StringVar(&userAddName, "name", "", "Display name for the user")
	userAddCmd.Flags().StringVar(&userAddEmail, "email", "", "Email address for the user")
}

func runUserList(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	rm := user.NewRegistryManager(townRoot)
	users, err := rm.List()
	if err != nil {
		if errors.Is(err, user.ErrRegistryNotFound) {
			fmt.Println("No users registered. Run 'gt user add <username>' to add the first user.")
			return nil
		}
		return err
	}

	if len(users) == 0 {
		fmt.Println("No users registered. Run 'gt user add <username>' to add the first user.")
		return nil
	}

	currentUser, _ := user.GetCurrentUser()

	fmt.Printf("Users in %s:\n", townRoot)
	for _, u := range users {
		marker := "  "
		if u.Username == currentUser {
			marker = "* "
		}

		display := u.Username
		if u.Name != "" && u.Name != u.Username {
			display = fmt.Sprintf("%s (%s)", u.Username, u.Name)
		}
		if u.Email != "" {
			display += fmt.Sprintf(" <%s>", u.Email)
		}

		fmt.Printf("  %s%s\n", marker, display)
	}

	return nil
}

func runUserWhoami(cmd *cobra.Command, args []string) error {
	currentUser, err := user.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("getting current user: %w", err)
	}

	if currentUser == "" {
		fmt.Println(style.Dim.Render("No user set."))
		fmt.Println("Run 'gt user add <username>' to register, or set GT_USER environment variable.")
		return nil
	}

	fmt.Printf("%s %s\n", style.Bold.Render("Current user:"), currentUser)

	// Try to get full details from registry
	townRoot, err := workspace.FindFromCwd()
	if err == nil && townRoot != "" {
		rm := user.NewRegistryManager(townRoot)
		u, err := rm.Get(currentUser)
		if err == nil {
			if u.Name != "" && u.Name != u.Username {
				fmt.Printf("  Name:  %s\n", u.Name)
			}
			if u.Email != "" {
				fmt.Printf("  Email: %s\n", u.Email)
			}
			fmt.Printf("  %s %s\n", style.Dim.Render("Source:"), style.Dim.Render(u.Source))
		}
	}

	return nil
}

func runUserAdd(cmd *cobra.Command, args []string) error {
	username := strings.TrimSpace(args[0])
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	rm := user.NewRegistryManager(townRoot)

	// Check if user already exists
	exists, err := rm.Exists(username)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("user '%s' already exists", username)
	}

	newUser := user.User{
		Username: username,
		Name:     userAddName,
		Email:    userAddEmail,
		Source:   user.SourceManual,
		Added:    time.Now().UTC(),
	}

	// Auto-detect missing fields
	if newUser.Name == "" || newUser.Email == "" {
		detected := user.Detect(townRoot)
		if detected != nil {
			if newUser.Name == "" {
				newUser.Name = detected.Name
			}
			if newUser.Email == "" {
				newUser.Email = detected.Email
			}
			if newUser.Source == user.SourceManual && (userAddName == "" || userAddEmail == "") {
				newUser.Source = detected.Source
			}
		}
	}

	// Ensure name has at least the username
	if newUser.Name == "" {
		newUser.Name = username
	}

	if err := rm.Add(newUser); err != nil {
		return err
	}

	fmt.Printf("✓ Added user '%s'\n", username)

	// If this is the first user, set as current
	users, err := rm.List()
	if err == nil && len(users) == 1 {
		if err := user.SetCurrentUser(username); err == nil {
			fmt.Printf("✓ Set as current user\n")
		}
	}

	// Migrate existing overseer identity if this is the first user
	migrateOverseerToUser(townRoot, username, &newUser)

	return nil
}

func runUserSwitch(cmd *cobra.Command, args []string) error {
	username := strings.TrimSpace(args[0])

	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return err
	}

	rm := user.NewRegistryManager(townRoot)

	// Verify user exists
	exists, err := rm.Exists(username)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("user '%s' not found. Run 'gt user list' to see available users", username)
	}

	if err := user.SetCurrentUser(username); err != nil {
		return fmt.Errorf("switching user: %w", err)
	}

	fmt.Printf("✓ Switched to user: %s\n", username)
	return nil
}

// migrateOverseerToUser copies overseer identity fields to the user if they're
// missing, bridging the single-overseer to multi-overseer transition.
func migrateOverseerToUser(townRoot, username string, u *user.User) {
	overseerPath := config.OverseerConfigPath(townRoot)
	overseer, err := config.LoadOverseerConfig(overseerPath)
	if err != nil {
		return
	}

	// If user was auto-detected and matches overseer, assign existing rigs
	if u.Email == overseer.Email || u.Name == overseer.Name {
		assignExistingRigsToUser(townRoot, username)
	}
}

// assignExistingRigsToUser assigns all rigs without an owner to the given user.
func assignExistingRigsToUser(townRoot, username string) {
	rigsPath := config.RigsConfigPath(townRoot)
	rigsConfig, err := config.LoadRigsConfig(rigsPath)
	if err != nil {
		return
	}

	modified := false
	for name, entry := range rigsConfig.Rigs {
		if entry.Owner == "" {
			entry.Owner = username
			rigsConfig.Rigs[name] = entry
			modified = true
		}
	}

	if modified {
		if err := config.SaveRigsConfig(rigsPath, rigsConfig); err != nil {
			fmt.Printf("  Warning: could not assign rigs to %s: %v\n", username, err)
			return
		}
		fmt.Printf("✓ Assigned existing rigs to user '%s'\n", username)
	}
}
