package capabilityd

import (
	"context"
	"regexp"
	"strings"
)

var mattermostReplyMentionPattern = regexp.MustCompile(`(^|[^A-Za-z0-9._-])@([\p{L}\p{N}._-]+)`)

func (service Service) normalizeMattermostReplyMentions(ctx context.Context, message string) string {
	if strings.Contains(message, "`") {
		return message
	}
	matches := mattermostReplyMentionPattern.FindAllStringSubmatchIndex(message, -1)
	if len(matches) == 0 {
		return message
	}
	resolvedMentions := map[string]string{}
	var builder strings.Builder
	lastIndex := 0
	for _, match := range matches {
		matchStart := match[0]
		matchEnd := match[1]
		prefixStart := match[2]
		prefixEnd := match[3]
		mentionStart := match[4]
		mentionEnd := match[5]
		mentionText := message[mentionStart:mentionEnd]
		replacement := service.resolvedMattermostReplyMention(ctx, mentionText, resolvedMentions)
		if replacement == "" {
			continue
		}
		builder.WriteString(message[lastIndex:matchStart])
		builder.WriteString(message[prefixStart:prefixEnd])
		builder.WriteString(replacement)
		lastIndex = matchEnd
	}
	if lastIndex == 0 {
		return message
	}
	builder.WriteString(message[lastIndex:])
	return builder.String()
}

func (service Service) resolvedMattermostReplyMention(ctx context.Context, mentionText string, resolvedMentions map[string]string) string {
	normalizedMentionText := strings.ToLower(strings.TrimSpace(mentionText))
	if normalizedMentionText == "" {
		return ""
	}
	if replacement, found := resolvedMentions[normalizedMentionText]; found {
		return replacement
	}
	resolution, errorValue := service.fetchPlatformDMRecipientResolution(ctx, mentionText)
	if errorValue != nil || resolution.Status != "resolved" || resolution.Recipient == nil {
		resolvedMentions[normalizedMentionText] = ""
		return ""
	}
	replacement := mattermostMentionForUsername(resolution.Recipient.Username)
	resolvedMentions[normalizedMentionText] = replacement
	return replacement
}
