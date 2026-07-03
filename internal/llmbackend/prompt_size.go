package llmbackend

import "fmt"

// LlamaCppLocalContextWindowTokens is a conservative ceiling on the prompt size the
// on-device llama.cpp server can process without spending minutes on a call that
// cannot succeed. The service unit (see runtime/blueclaw.LlamaCppServiceUnit) starts
// llama-server without an explicit context-size flag, so this is not read from a
// live configuration value; correct it if the launch args ever pin a real -c value.
const LlamaCppLocalContextWindowTokens = 8192

func estimatedPromptTokens(messages []Message) int64 {
	var totalBytes int64
	for _, message := range messages {
		totalBytes += int64(len(message.Content))
		for _, part := range message.Parts {
			totalBytes += int64(len(part.Text))
		}
	}
	return (totalBytes + 3) / 4
}

func promptExceedsContextWindow(messages []Message, contextWindowTokens int64) error {
	estimatedTokens := estimatedPromptTokens(messages)
	if estimatedTokens <= contextWindowTokens {
		return nil
	}
	return fmt.Errorf("prompt is too large for the local model's context window: estimated %d tokens exceeds %d token ceiling", estimatedTokens, contextWindowTokens)
}
