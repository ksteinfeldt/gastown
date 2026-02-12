package slack

import (
	"fmt"
	"strings"
	"time"
)

// EventType identifies the type of Gas Town event.
type EventType string

// Event types for Slack notifications.
const (
	EventJobQueued    EventType = "job_queued"
	EventJobStarted   EventType = "job_started"
	EventPRCreated    EventType = "pr_created"
	EventJobCompleted EventType = "job_completed"
	EventJobFailed    EventType = "job_failed"
	EventEscalation   EventType = "escalation"
)

// Field keys used in notification payloads.
const (
	FieldBead        = "bead"
	FieldTitle       = "title"
	FieldAssignee    = "assignee"
	FieldBranch      = "branch"
	FieldPR          = "pr"
	FieldPRURL       = "pr_url"
	FieldMR          = "mr"
	FieldCommit      = "commit"
	FieldStatus      = "status"
	FieldReason      = "reason"
	FieldError       = "error"
	FieldSeverity    = "severity"
	FieldDescription = "description"
	FieldSource      = "source"
	FieldRepo        = "repo"
)

// eventConfig holds display configuration for each event type.
type eventConfig struct {
	emoji string
	title string
	color string // For attachment color (not used in blocks)
}

var eventConfigs = map[EventType]eventConfig{
	EventJobQueued:    {emoji: "📋", title: "Job Queued"},
	EventJobStarted:   {emoji: "🚀", title: "Job Started"},
	EventPRCreated:    {emoji: "🔀", title: "PR Ready for Review"},
	EventJobCompleted: {emoji: "✅", title: "Job Completed"},
	EventJobFailed:    {emoji: "❌", title: "Job Failed"},
	EventEscalation:   {emoji: "🚨", title: "Escalation"},
}

// formatMessage creates a Slack message for the given event.
func formatMessage(event EventType, fields map[string]string) *slackMessage {
	cfg, ok := eventConfigs[event]
	if !ok {
		cfg = eventConfig{emoji: "📢", title: string(event)}
	}

	// Build header
	header := fmt.Sprintf("%s *%s*", cfg.emoji, cfg.title)

	// Build field blocks
	var fieldBlocks []slackText
	switch event {
	case EventJobQueued:
		fieldBlocks = formatJobQueuedFields(fields)
	case EventJobStarted:
		fieldBlocks = formatJobStartedFields(fields)
	case EventPRCreated:
		fieldBlocks = formatPRCreatedFields(fields)
	case EventJobCompleted:
		fieldBlocks = formatJobCompletedFields(fields)
	case EventJobFailed:
		fieldBlocks = formatJobFailedFields(fields)
	case EventEscalation:
		fieldBlocks = formatEscalationFields(fields)
	default:
		fieldBlocks = formatGenericFields(fields)
	}

	// Build blocks
	blocks := []slackBlock{
		{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: header},
		},
	}

	if len(fieldBlocks) > 0 {
		blocks = append(blocks, slackBlock{
			Type:   "section",
			Fields: fieldBlocks,
		})
	}

	// Add timestamp context
	blocks = append(blocks, slackBlock{
		Type: "context",
		Fields: []slackText{
			{Type: "mrkdwn", Text: fmt.Sprintf("_Gas Town • %s_", time.Now().Format("Jan 2, 15:04 MST"))},
		},
	})

	return &slackMessage{
		Text:   fmt.Sprintf("%s %s", cfg.emoji, cfg.title), // Fallback text
		Blocks: blocks,
	}
}

func formatJobQueuedFields(fields map[string]string) []slackText {
	var result []slackText
	if v := fields[FieldBead]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Bead:*\n`%s`", v)})
	}
	if v := fields[FieldTitle]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Title:*\n%s", truncate(v, 50))})
	}
	if v := fields[FieldAssignee]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Assignee:*\n%s", v)})
	}
	if v := fields[FieldRepo]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Repo:*\n%s", v)})
	}
	return result
}

func formatJobStartedFields(fields map[string]string) []slackText {
	var result []slackText
	if v := fields[FieldBead]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Bead:*\n`%s`", v)})
	}
	if v := fields[FieldAssignee]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Worker:*\n%s", v)})
	}
	return result
}

func formatPRCreatedFields(fields map[string]string) []slackText {
	var result []slackText
	if v := fields[FieldBead]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Bead:*\n`%s`", v)})
	}
	if v := fields[FieldBranch]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Branch:*\n`%s`", v)})
	}
	if v := fields[FieldPRURL]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*PR:*\n<%s|View PR>", v)})
	} else if v := fields[FieldMR]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*MR:*\n`%s`", v)})
	}
	return result
}

func formatJobCompletedFields(fields map[string]string) []slackText {
	var result []slackText
	if v := fields[FieldBead]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Bead:*\n`%s`", v)})
	}
	if v := fields[FieldBranch]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Branch:*\n`%s`", v)})
	}
	if v := fields[FieldCommit]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Commit:*\n`%s`", truncate(v, 8))})
	}
	if v := fields[FieldPRURL]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*PR:*\n<%s|View PR>", v)})
	}
	return result
}

func formatJobFailedFields(fields map[string]string) []slackText {
	var result []slackText
	if v := fields[FieldBead]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Bead:*\n`%s`", v)})
	}
	if v := fields[FieldMR]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*MR:*\n`%s`", v)})
	}
	if v := fields[FieldReason]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Reason:*\n%s", v)})
	}
	if v := fields[FieldError]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Error:*\n```%s```", truncate(v, 200))})
	}
	return result
}

func formatEscalationFields(fields map[string]string) []slackText {
	var result []slackText
	if v := fields[FieldSeverity]; v != "" {
		severityEmoji := map[string]string{
			"critical": "🔴",
			"high":     "🟠",
			"medium":   "🟡",
			"low":      "🟢",
		}
		emoji := severityEmoji[strings.ToLower(v)]
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Severity:*\n%s %s", emoji, strings.ToUpper(v))})
	}
	if v := fields[FieldBead]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Bead:*\n`%s`", v)})
	}
	if v := fields[FieldDescription]; v != "" {
		result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Description:*\n%s", truncate(v, 200))})
	}
	return result
}

func formatGenericFields(fields map[string]string) []slackText {
	var result []slackText
	for k, v := range fields {
		if v != "" {
			result = append(result, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*%s:*\n%s", k, truncate(v, 100))})
		}
	}
	return result
}

// truncate shortens a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
