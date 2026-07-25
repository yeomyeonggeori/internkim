package buzzimport

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var errAuthorHasNoBuzzIdentity = errors.New("mattermost author has no buzz identity")

type IdentityResolver interface {
	SecretForEmail(email string) (string, error)
	PubkeyForUsername(username string) (string, bool)
}

type ChannelImportPlan struct {
	BuzzChannelID string
	Posts         []MattermostPost
	AuthorEmails  map[string]string
}

// PlanChannelImport turns a channel's posts into the messages to inject, in
// order, resolving each reply to the event its root became. Posts whose author
// has no buzz identity are reported rather than silently dropped.
func PlanChannelImport(plan ChannelImportPlan, resolver IdentityResolver) ([]ImportedMessage, []string, error) {
	if strings.TrimSpace(plan.BuzzChannelID) == "" {
		return nil, nil, errors.New("buzz channel id is required")
	}
	messages := []ImportedMessage{}
	skippedPostIDs := []string{}
	eventIDByPostID := map[string]string{}

	for _, post := range plan.Posts {
		authorEmail := strings.TrimSpace(plan.AuthorEmails[post.UserID])
		if authorEmail == "" {
			skippedPostIDs = append(skippedPostIDs, post.ID)
			continue
		}
		authorSecret, errorValue := resolver.SecretForEmail(authorEmail)
		if errorValue != nil {
			if errors.Is(errorValue, errAuthorHasNoBuzzIdentity) {
				skippedPostIDs = append(skippedPostIDs, post.ID)
				continue
			}
			return nil, nil, fmt.Errorf("resolve identity for %s: %w", authorEmail, errorValue)
		}
		message := ImportedMessage{
			ChannelID:       plan.BuzzChannelID,
			AuthorSecretHex: authorSecret,
			Text:            post.Message,
			SentAt:          post.CreatedAt,
			MentionPubkeys:  mentionPubkeysInMessage(post.Message, resolver),
		}
		if rootPostID := strings.TrimSpace(post.RootID); rootPostID != "" {
			rootEventID, isKnown := eventIDByPostID[rootPostID]
			if !isKnown {
				skippedPostIDs = append(skippedPostIDs, post.ID)
				continue
			}
			message.RootEventID = rootEventID
		}
		event, errorValue := BuildStreamEvent(message)
		if errorValue != nil {
			return nil, nil, fmt.Errorf("build event for post %s: %w", post.ID, errorValue)
		}
		eventIDByPostID[post.ID] = event.ID
		messages = append(messages, message)
	}
	return messages, skippedPostIDs, nil
}

func mentionPubkeysInMessage(message string, resolver IdentityResolver) []string {
	pubkeys := []string{}
	seen := map[string]bool{}
	for _, username := range mentionedUsernames(message) {
		pubkey, isKnown := resolver.PubkeyForUsername(username)
		if !isKnown || seen[pubkey] {
			continue
		}
		seen[pubkey] = true
		pubkeys = append(pubkeys, pubkey)
	}
	return pubkeys
}

func mentionedUsernames(message string) []string {
	usernames := []string{}
	for _, field := range strings.Fields(message) {
		if !strings.HasPrefix(field, "@") || len(field) < 2 {
			continue
		}
		username := strings.TrimFunc(field[1:], func(value rune) bool {
			return !unicode.IsLetter(value) && !unicode.IsDigit(value) && value != '.' && value != '-' && value != '_'
		})
		if username != "" {
			usernames = append(usernames, strings.ToLower(username))
		}
	}
	return usernames
}
