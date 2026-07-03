package llmbackend

import "fmt"

// DefaultContextWindowTokens is the assumed usable context window for any backend
// that does not declare its own via ContextWindowAware. Remote providers such as
// OpenRouter are deliberately chosen for their large context windows, so the
// default is generous rather than a small, arbitrary ceiling.
const DefaultContextWindowTokens int64 = 1_000_000

// contextWindowInputBudgetRatio caps how much of the context window compaction will
// fill with input messages, leaving headroom for the model's completion.
const contextWindowInputBudgetRatio = 0.82

// recentMessagesPreservedDuringCompaction is how many of the most recent messages
// are kept verbatim when compacting an oversized message list.
const recentMessagesPreservedDuringCompaction = 6

// ContextWindowAware lets a backend declare its own usable context window in
// tokens. Backends that don't implement it are assumed to have DefaultContextWindowTokens.
type ContextWindowAware interface {
	ContextWindowTokens() int64
}

func contextWindowTokensFor(candidate any) int64 {
	if contextWindowAware, isContextWindowAware := candidate.(ContextWindowAware); isContextWindowAware {
		if tokens := contextWindowAware.ContextWindowTokens(); tokens > 0 {
			return tokens
		}
	}
	return DefaultContextWindowTokens
}

func estimatedMessagesTokens(messages []Message) int64 {
	var totalBytes int64
	for _, message := range messages {
		totalBytes += int64(len(message.Content))
		for _, part := range message.Parts {
			totalBytes += int64(len(part.Text))
		}
	}
	return (totalBytes + 3) / 4
}

func inputTokenBudget(contextWindowTokens int64) int64 {
	return int64(float64(contextWindowTokens) * contextWindowInputBudgetRatio)
}

// compactMessagesForContextWindow keeps leading system/instruction messages and the
// most recent turns verbatim, collapsing everything in between into a single
// condensed message so the prompt fits the backend's usable context window. This
// mirrors how coding agents commonly handle context overflow: track a token budget,
// then drop/summarize the oldest content while preserving the system prompt and
// recent turns, rather than failing the call outright.
//
// If the message list still can't fit after compaction (for example, a single
// message is enormous on its own), it returns a clear error instead of looping.
func compactMessagesForContextWindow(messages []Message, contextWindowTokens int64) ([]Message, error) {
	budget := inputTokenBudget(contextWindowTokens)
	if estimatedMessagesTokens(messages) <= budget {
		return messages, nil
	}

	leadingSystemCount := leadingSystemMessageCount(messages)
	recentCount := recentMessagesPreservedDuringCompaction
	if remainingAfterSystem := len(messages) - leadingSystemCount; recentCount > remainingAfterSystem {
		recentCount = remainingAfterSystem
	}
	middleStart := leadingSystemCount
	middleEnd := len(messages) - recentCount
	if middleEnd <= middleStart {
		return nil, fmt.Errorf("prompt is too large for the model's context window and has no middle content left to compact: estimated %d tokens exceeds %d token budget", estimatedMessagesTokens(messages), budget)
	}

	compacted := make([]Message, 0, leadingSystemCount+1+recentCount)
	compacted = append(compacted, messages[:leadingSystemCount]...)
	compacted = append(compacted, condensedMiddleMessage(messages[middleStart:middleEnd]))
	compacted = append(compacted, messages[middleEnd:]...)

	if estimatedMessagesTokens(compacted) > budget {
		return nil, fmt.Errorf("prompt still exceeds the model's context window after compaction: estimated %d tokens exceeds %d token budget", estimatedMessagesTokens(compacted), budget)
	}
	return compacted, nil
}

func leadingSystemMessageCount(messages []Message) int {
	count := 0
	for _, message := range messages {
		if message.Role != "system" {
			break
		}
		count++
	}
	return count
}

func condensedMiddleMessage(droppedMessages []Message) Message {
	droppedBytes := int64(0)
	for _, message := range droppedMessages {
		droppedBytes += int64(len(message.Content))
		for _, part := range message.Parts {
			droppedBytes += int64(len(part.Text))
		}
	}
	return Message{
		Role: "system",
		Content: fmt.Sprintf(
			"Earlier turns in this task included %d messages (%d bytes) condensed for length to fit the model's context window.",
			len(droppedMessages), droppedBytes,
		),
	}
}
