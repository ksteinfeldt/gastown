// Package backend provides context management for API backends.
package backend

import (
	"fmt"
)

// TruncationStrategy defines how to handle context overflow.
type TruncationStrategy string

const (
	// TruncateOldest removes oldest messages first (keeping system + recent).
	TruncateOldest TruncationStrategy = "truncate_oldest"

	// TruncateMiddle keeps first and last messages, removes middle.
	TruncateMiddle TruncationStrategy = "truncate_middle"

	// TruncateLongest removes the longest messages first.
	TruncateLongest TruncationStrategy = "truncate_longest"
)

// ContextManager handles context preparation for API backends.
type ContextManager struct {
	// DefaultStrategy is the default truncation strategy.
	DefaultStrategy TruncationStrategy

	// ReserveTokens is the number of tokens to reserve for the response.
	ReserveTokens int
}

// NewContextManager creates a new context manager with defaults.
func NewContextManager() *ContextManager {
	return &ContextManager{
		DefaultStrategy: TruncateOldest,
		ReserveTokens:   4096, // Reserve for response
	}
}

// PrepareContext trims/summarizes context to fit model limits.
func (cm *ContextManager) PrepareContext(
	messages []Message,
	maxTokens int,
	strategy TruncationStrategy,
) ([]Message, error) {
	if len(messages) == 0 {
		return messages, nil
	}

	// Estimate current tokens
	currentTokens := cm.estimateTokens(messages)

	// Account for response reserve
	availableTokens := maxTokens - cm.ReserveTokens
	if availableTokens <= 0 {
		return nil, fmt.Errorf("max_tokens (%d) too small for response reserve (%d)", maxTokens, cm.ReserveTokens)
	}

	if currentTokens <= availableTokens {
		return messages, nil // Fits as-is
	}

	if strategy == "" {
		strategy = cm.DefaultStrategy
	}

	switch strategy {
	case TruncateOldest:
		return cm.truncateOldest(messages, availableTokens)
	case TruncateMiddle:
		return cm.truncateMiddle(messages, availableTokens)
	case TruncateLongest:
		return cm.truncateLongest(messages, availableTokens)
	default:
		return cm.truncateOldest(messages, availableTokens)
	}
}

// truncateOldest removes oldest messages first (keeping system + recent).
func (cm *ContextManager) truncateOldest(messages []Message, maxTokens int) ([]Message, error) {
	if len(messages) < 2 {
		return messages, nil
	}

	// Separate system message from conversation
	var systemMsg *Message
	var conversation []Message

	for i, msg := range messages {
		if msg.Role == "system" && i == 0 {
			systemMsg = &messages[i]
		} else {
			conversation = append(conversation, msg)
		}
	}

	// Calculate system message tokens
	systemTokens := 0
	if systemMsg != nil {
		systemTokens = cm.estimateMessageTokens(*systemMsg)
	}

	availableForConversation := maxTokens - systemTokens
	if availableForConversation <= 0 {
		// System message alone exceeds limit - truncate it
		if systemMsg != nil {
			truncated := cm.truncateMessage(*systemMsg, maxTokens)
			return []Message{truncated}, nil
		}
		return nil, fmt.Errorf("cannot fit any messages in %d tokens", maxTokens)
	}

	// Add messages from the end until we hit the limit
	var result []Message
	currentTokens := 0

	for i := len(conversation) - 1; i >= 0; i-- {
		msgTokens := cm.estimateMessageTokens(conversation[i])
		if currentTokens+msgTokens > availableForConversation {
			break
		}
		result = append([]Message{conversation[i]}, result...)
		currentTokens += msgTokens
	}

	// Prepend system message if present
	if systemMsg != nil {
		result = append([]Message{*systemMsg}, result...)
	}

	return result, nil
}

// truncateMiddle keeps first and last messages, removes middle.
func (cm *ContextManager) truncateMiddle(messages []Message, maxTokens int) ([]Message, error) {
	if len(messages) <= 2 {
		return messages, nil
	}

	// Separate system message
	var systemMsg *Message
	var conversation []Message

	for i, msg := range messages {
		if msg.Role == "system" && i == 0 {
			systemMsg = &messages[i]
		} else {
			conversation = append(conversation, msg)
		}
	}

	if len(conversation) <= 2 {
		result := conversation
		if systemMsg != nil {
			result = append([]Message{*systemMsg}, result...)
		}
		return result, nil
	}

	// Calculate system message tokens
	systemTokens := 0
	if systemMsg != nil {
		systemTokens = cm.estimateMessageTokens(*systemMsg)
	}

	availableForConversation := maxTokens - systemTokens

	// Always keep first and last message
	first := conversation[0]
	last := conversation[len(conversation)-1]
	firstTokens := cm.estimateMessageTokens(first)
	lastTokens := cm.estimateMessageTokens(last)

	remaining := availableForConversation - firstTokens - lastTokens
	if remaining <= 0 {
		// Just keep first and last
		result := []Message{first, last}
		if systemMsg != nil {
			result = append([]Message{*systemMsg}, result...)
		}
		return result, nil
	}

	// Add middle messages from both ends toward center
	middle := conversation[1 : len(conversation)-1]
	var kept []Message
	left, right := 0, len(middle)-1
	currentTokens := 0

	for left <= right {
		// Try to add from left
		if left <= right {
			leftTokens := cm.estimateMessageTokens(middle[left])
			if currentTokens+leftTokens <= remaining {
				kept = append(kept, middle[left])
				currentTokens += leftTokens
				left++
			} else {
				break
			}
		}

		// Try to add from right
		if left <= right {
			rightTokens := cm.estimateMessageTokens(middle[right])
			if currentTokens+rightTokens <= remaining {
				// Insert at correct position
				kept = append(kept, Message{}) // placeholder
				copy(kept[left+1:], kept[left:])
				kept[left] = middle[right]
				currentTokens += rightTokens
				right--
			} else {
				break
			}
		}
	}

	// Reconstruct: first + kept + last
	result := []Message{first}
	result = append(result, kept...)
	result = append(result, last)

	if systemMsg != nil {
		result = append([]Message{*systemMsg}, result...)
	}

	return result, nil
}

// truncateLongest removes the longest messages first.
func (cm *ContextManager) truncateLongest(messages []Message, maxTokens int) ([]Message, error) {
	// Make a copy to avoid modifying original
	msgs := make([]Message, len(messages))
	copy(msgs, messages)

	for cm.estimateTokens(msgs) > maxTokens && len(msgs) > 1 {
		// Find longest non-system message
		longestIdx := -1
		longestLen := 0

		for i, msg := range msgs {
			if msg.Role == "system" {
				continue
			}
			if len(msg.Content) > longestLen {
				longestLen = len(msg.Content)
				longestIdx = i
			}
		}

		if longestIdx < 0 {
			break // Only system message left
		}

		// Remove the longest message
		msgs = append(msgs[:longestIdx], msgs[longestIdx+1:]...)
	}

	return msgs, nil
}

// truncateMessage truncates a single message to fit token limit.
func (cm *ContextManager) truncateMessage(msg Message, maxTokens int) Message {
	// Rough estimate: 4 chars per token
	maxChars := maxTokens * 4

	if len(msg.Content) <= maxChars {
		return msg
	}

	// Truncate with ellipsis
	truncated := msg.Content[:maxChars-3] + "..."
	return Message{
		Role:    msg.Role,
		Content: truncated,
	}
}

// estimateTokens estimates total tokens for a message list.
func (cm *ContextManager) estimateTokens(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += cm.estimateMessageTokens(msg)
	}
	return total
}

// estimateMessageTokens estimates tokens for a single message.
func (cm *ContextManager) estimateMessageTokens(msg Message) int {
	// Rough estimate: 4 characters per token
	// Add overhead for role and message structure
	chars := len(msg.Content) + len(msg.Role) + 10
	return chars / 4
}

// BuildMessagesFromText creates a message list from a simple prompt.
func BuildMessagesFromText(systemPrompt, userPrompt string) []Message {
	var messages []Message

	if systemPrompt != "" {
		messages = append(messages, Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	if userPrompt != "" {
		messages = append(messages, Message{
			Role:    "user",
			Content: userPrompt,
		})
	}

	return messages
}
