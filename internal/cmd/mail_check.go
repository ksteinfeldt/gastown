package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/mail"
	"github.com/steveyegge/gastown/internal/style"
)

const mailCheckCacheTTL = 30 * time.Second

var mailCheckCacheDir string

type mailCheckCacheEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Address   string    `json:"address"`
	Unread    int       `json:"unread"`
	Subjects  []string  `json:"subjects,omitempty"`
}

func mailCheckCachePath(address string) string {
	filename := strings.ReplaceAll(address, "/", "_") + ".json"
	return filepath.Join(mailCheckCacheDir, filename)
}

func loadMailCheckCache(address string) *mailCheckCacheEntry {
	if mailCheckCacheDir == "" {
		return nil
	}

	path := mailCheckCachePath(address)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var entry mailCheckCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}

	if entry.Address != address {
		return nil
	}

	if time.Since(entry.Timestamp) > mailCheckCacheTTL {
		return nil
	}

	return &entry
}

func saveMailCheckCache(entry *mailCheckCacheEntry) {
	if mailCheckCacheDir == "" {
		return
	}

	os.MkdirAll(mailCheckCacheDir, 0755)

	path := mailCheckCachePath(entry.Address)
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(path, data, 0644)
}

func runMailCheck(cmd *cobra.Command, args []string) error {
	// Determine which inbox (priority: --identity flag, auto-detect)
	address := ""
	if mailCheckIdentity != "" {
		address = mailCheckIdentity
	} else {
		address = detectSender()
	}

	// All mail uses town beads (two-level architecture)
	workDir, err := findMailWorkDir()
	if err != nil {
		if mailCheckInject {
			// Inject mode: always exit 0, silent on error
			return nil
		}
		return fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	// Get mailbox
	router := mail.NewRouter(workDir)
	mailbox, err := router.GetMailbox(address)
	if err != nil {
		if mailCheckInject {
			return nil
		}
		return fmt.Errorf("getting mailbox: %w", err)
	}

	// Count unread
	_, unread, err := mailbox.Count()
	if err != nil {
		if mailCheckInject {
			return nil
		}
		return fmt.Errorf("counting messages: %w", err)
	}

	// JSON output
	if mailCheckJSON {
		result := map[string]interface{}{
			"address": address,
			"unread":  unread,
			"has_new": unread > 0,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	// Inject mode: output system-reminder if mail exists
	if mailCheckInject {
		if unread > 0 {
			// Get subjects for context
			messages, _ := mailbox.ListUnread()
			var subjects []string
			for _, msg := range messages {
				subjects = append(subjects, fmt.Sprintf("- %s from %s: %s", msg.ID, msg.From, msg.Subject))
			}

			fmt.Println("<system-reminder>")
			fmt.Printf("You have %d unread message(s) in your inbox.\n\n", unread)
			for _, s := range subjects {
				fmt.Println(s)
			}
			fmt.Println()
			fmt.Println("Run 'gt mail inbox' to see your messages, or 'gt mail read <id>' for a specific message.")
			fmt.Println("</system-reminder>")
		}
		return nil
	}

	// Normal mode
	if unread > 0 {
		fmt.Printf("%s %d unread message(s)\n", style.Bold.Render("📬"), unread)
		return NewSilentExit(0)
	}
	fmt.Println("No new mail")
	return NewSilentExit(1)
}
