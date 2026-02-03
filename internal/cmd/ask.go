package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/backend"
	"github.com/steveyegge/gastown/internal/backend/bedrock"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

var askCmd = &cobra.Command{
	Use:     "ask <question>",
	GroupID: GroupWork,
	Short:   "Ask a quick question using the API backend (no agent needed)",
	Long: `Ask a quick question and get an immediate response via API.

This command uses the API backend (Bedrock by default) for stateless queries
that don't require file operations or tool use. Perfect for:

  - Quick explanations: gt ask "what is a mutex?"
  - Code understanding: gt ask "explain this error message: <error>"
  - Summaries: gt ask "summarize the purpose of the config package"
  - Classifications: gt ask "is this a bug or feature request?"

Cost-Effective:
  Uses haiku by default (cheapest Claude model). Override with --tier flag.

Examples:
  gt ask "what does the --force flag do in git push?"
  gt ask "explain this Go error: undefined: foo"
  gt ask --tier sonnet "design a REST API for user management"
  gt ask --backend grok "what's new in Go 1.22?"

Note: This is for quick questions only. For work that requires file operations,
code changes, or multi-step reasoning, use gt sling instead.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAsk,
}

var (
	askTier    string // --tier: model tier (haiku, sonnet, opus)
	askBackend string // --backend: API backend (bedrock, grok)
	askStream  bool   // --stream: stream response as it's generated
)

func init() {
	askCmd.Flags().StringVar(&askTier, "tier", "haiku", "Model tier: haiku (default, cheapest), sonnet, opus")
	askCmd.Flags().StringVar(&askBackend, "backend", "bedrock", "API backend: bedrock (default), grok")
	askCmd.Flags().BoolVar(&askStream, "stream", true, "Stream response as it's generated")

	rootCmd.AddCommand(askCmd)
}

func runAsk(cmd *cobra.Command, args []string) error {
	question := strings.Join(args, " ")

	// Get town root for config (may be empty if outside a town)
	townRoot, _ := workspace.FindFromCwd()
	_ = townRoot // May use later for config

	// Register bedrock backend
	bedrockBackend, err := bedrock.New()
	if err != nil {
		return fmt.Errorf("initializing bedrock backend: %w", err)
	}
	backend.GetRegistry().Register(bedrockBackend)

	// Select the backend
	var selectedBackend backend.AgentBackend
	switch strings.ToLower(askBackend) {
	case "bedrock":
		selectedBackend = bedrockBackend
	case "grok":
		grokBackend, err := backend.GetRegistry().Get("grok")
		if err != nil {
			return fmt.Errorf("grok backend not available (check XAI_API_KEY): %w", err)
		}
		selectedBackend = grokBackend
	default:
		return fmt.Errorf("unknown backend '%s': must be bedrock or grok", askBackend)
	}

	// Map tier to model
	var model string
	switch strings.ToLower(askTier) {
	case "haiku":
		model = "haiku"
	case "sonnet":
		model = "sonnet"
	case "opus":
		model = "opus"
	default:
		return fmt.Errorf("unknown tier '%s': must be haiku, sonnet, or opus", askTier)
	}

	// Build messages
	messages := []backend.Message{
		{
			Role:    "user",
			Content: question,
		},
	}

	// Display what we're doing
	fmt.Printf("%s Asking %s (%s)...\n\n", style.Dim.Render("→"), model, selectedBackend.Name())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if askStream {
		// Stream the response
		streamCh, err := selectedBackend.InvokeStream(ctx, messages, backend.InvokeOptions{
			Model:     model,
			MaxTokens: 4096,
		})
		if err != nil {
			return fmt.Errorf("invoking API: %w", err)
		}

		for chunk := range streamCh {
			if chunk.Error != nil {
				return fmt.Errorf("streaming error: %w", chunk.Error)
			}
			fmt.Print(chunk.Content)
		}
		fmt.Println()

		// Note: Cost estimate not available for streaming (would need token counting)
		fmt.Printf("\n%s Response complete (streaming mode - use --stream=false for cost estimate)\n", style.Dim.Render("✓"))
	} else {
		// Non-streaming response
		result, err := selectedBackend.Invoke(ctx, messages, backend.InvokeOptions{
			Model:     model,
			MaxTokens: 4096,
		})
		if err != nil {
			return fmt.Errorf("invoking API: %w", err)
		}

		fmt.Println(result.Content)

		// Show cost estimate
		cost := selectedBackend.EstimateCost(result.InputTokens, result.OutputTokens, model)
		fmt.Printf("\n%s %d input + %d output tokens, ~$%.4f\n",
			style.Dim.Render("Cost:"),
			result.InputTokens, result.OutputTokens, cost.TotalCost)
	}

	return nil
}
