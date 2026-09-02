package capabilityd

import (
	"fmt"
	"strings"
)

func platformMessageSearchPreview(message string, queries []string) string {
	message = strings.Join(strings.Fields(strings.TrimSpace(message)), " ")
	query := firstPlatformMessagePreviewQuery(message, queries)
	if query == "" {
		return clippedPlatformMessagePreview(message)
	}
	index := strings.Index(strings.ToLower(message), strings.ToLower(query))
	if index < 0 {
		return clippedPlatformMessagePreview(message)
	}
	runes := []rune(message)
	queryStart := len([]rune(message[:index]))
	queryLength := len([]rune(query))
	start := max(0, queryStart-80)
	end := min(len(runes), queryStart+queryLength+120)
	prefix := ""
	if start > 0 {
		prefix = "..."
	}
	suffix := ""
	if end < len(runes) {
		suffix = "..."
	}
	return prefix + string(runes[start:end]) + suffix
}

func firstPlatformMessagePreviewQuery(message string, queries []string) string {
	normalizedMessage := strings.ToLower(message)
	for _, value := range queries {
		query := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
		if query == "" {
			continue
		}
		if strings.Contains(normalizedMessage, strings.ToLower(query)) {
			return query
		}
	}
	return ""
}

func clippedPlatformMessagePreview(message string) string {
	message = strings.Join(strings.Fields(strings.TrimSpace(message)), " ")
	if len([]rune(message)) <= 240 {
		return message
	}
	runes := []rune(message)
	return string(runes[:240])
}

func platformMessageEditMatchFailure(matchCount int, currentMessage string) platformToolFailure {
	reason := "oldText was not found in that message"
	if matchCount > 1 {
		reason = fmt.Sprintf("oldText appears %d times in that message; quote a longer span that occurs once", matchCount)
	}
	return platformToolStaticFailure("invalid_input", "message_edit_match", reason+". The message currently reads:\n"+currentMessage)
}

func canonicalPlatformMessageSearchAuthor(author string) string {
	author = platformMessageAuthorLabel(author)
	if author == "" {
		return "anyone"
	}
	return author
}

func uniqueTrimmedPlatformMessageIDs(messageIDs []string) []string {
	result := []string{}
	seenMessageID := map[string]bool{}
	for _, messageID := range messageIDs {
		messageID = strings.TrimSpace(messageID)
		if messageID == "" || seenMessageID[messageID] {
			continue
		}
		seenMessageID[messageID] = true
		result = append(result, messageID)
	}
	return result
}

const maximumInputImagePartBytes = 8 * 1024 * 1024

func platformMessageAuthorLabel(author string) string {
	switch strings.TrimSpace(author) {
	case "internkim":
		return "assistant"
	default:
		return strings.TrimSpace(author)
	}
}
