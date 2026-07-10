package capabilityd

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gitlab.com/eastriver/internkim/internal/identity"
)

type mattermostHistoryPost struct {
	ID       string   `json:"id"`
	UserID   string   `json:"user_id"`
	Message  string   `json:"message"`
	RootID   string   `json:"root_id"`
	Type     string   `json:"type"`
	CreateAt int64    `json:"create_at"`
	FileIDs  []string `json:"file_ids"`
	Metadata struct {
		Mentions []string `json:"mentions"`
	} `json:"metadata"`
}

func (service Service) enrichMattermostEvent(ctx context.Context, event platformInboundEvent) platformInboundEvent {
	handle, errorValue := decodePlatformHandle(event.Context.HistoryCursor)
	if errorValue != nil {
		return event
	}
	previousContext := event.Context
	nextContext := service.mattermostContext(ctx, handle, 20)
	nextContext.ConversationType = previousContext.ConversationType
	nextContext.ChannelID = previousContext.ChannelID
	nextContext.ChannelName = previousContext.ChannelName
	nextContext.Addressing = previousContext.Addressing
	nextContext.AttachmentsOnly = previousContext.AttachmentsOnly
	nextContext.InputAttachments = append([]platformInputAttachment{}, previousContext.InputAttachments...)
	nextContext.Materials = append(nextContext.Materials, previousContext.Materials...)
	nextContext.Materials = append(nextContext.Materials, previousContext.InputAttachments...)
	nextContext.Sender = service.mattermostSender(ctx, event.SenderID)
	nextContext.ReceivedAt = time.Now().UTC().Format(time.RFC3339)
	event.Context = nextContext
	return event
}

func mattermostContextTimestamp(createAt int64) string {
	if createAt <= 0 {
		return ""
	}
	return time.UnixMilli(createAt).UTC().Format(time.RFC3339)
}

func (service Service) mattermostContext(ctx context.Context, handle platformHandle, limit int) platformEventContext {
	limit = normalizedHistoryLimit(limit)
	posts, errorValue := service.mattermostHistoryPosts(ctx, handle, limit+1)
	if errorValue != nil {
		return platformEventContext{HistoryCursor: mustEncodePlatformHandle(handle)}
	}

	previousPosts := postsBeforeMessage(posts, handle.MessageID)
	hasMoreBefore := len(previousPosts) > limit
	if hasMoreBefore {
		previousPosts = previousPosts[len(previousPosts)-limit:]
	}

	messages := make([]platformContextMessage, 0, len(previousPosts))
	materials := []platformInputAttachment{}
	senderByUserID := map[string]platformContextSender{}
	for _, post := range previousPosts {
		if strings.TrimSpace(post.Type) != "" {
			continue
		}
		senderInfo, hasSender := senderByUserID[post.UserID]
		if !hasSender {
			senderInfo = service.mattermostSender(ctx, post.UserID)
			senderByUserID[post.UserID] = senderInfo
		}
		text := service.mattermostTextWithReadableMentions(ctx, post.Message, post.Metadata.Mentions, senderByUserID)
		inputAttachments := mattermostInputAttachments(post.ID, post.FileIDs)
		materials = append(materials, inputAttachments...)
		if strings.TrimSpace(text) == "" && len(inputAttachments) == 0 {
			continue
		}
		messages = append(messages, platformContextMessage{
			Speaker:            senderInfo.Name,
			SpeakerCallingName: senderInfo.CallingName,
			SpeakerHandle:      senderInfo.Handle,
			Text:               text,
			SentAt:             mattermostContextTimestamp(post.CreateAt),
			InputAttachments:   inputAttachments,
		})
	}

	return platformEventContext{
		Messages:      messages,
		Materials:     materials,
		HasMoreBefore: hasMoreBefore,
		HistoryCursor: mustEncodePlatformHandle(handle),
	}
}

func (service Service) mattermostHistoryPosts(ctx context.Context, handle platformHandle, limit int) ([]mattermostHistoryPost, error) {
	var response struct {
		Order []string                         `json:"order"`
		Posts map[string]mattermostHistoryPost `json:"posts"`
	}
	path := "/api/v4/channels/" + url.PathEscape(handle.ChannelID) + "/posts?per_page=" + strconv.Itoa(normalizedHistoryLimit(limit))
	if strings.TrimSpace(handle.RootID) != "" {
		path = "/api/v4/posts/" + url.PathEscape(handle.RootID) + "/thread"
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, nil, &response); errorValue != nil {
		return nil, errorValue
	}

	posts := make([]mattermostHistoryPost, 0, len(response.Posts))
	for _, post := range response.Posts {
		if strings.TrimSpace(handle.RootID) == "" && strings.TrimSpace(post.RootID) != "" {
			continue
		}
		posts = append(posts, post)
	}
	sort.SliceStable(posts, func(leftIndex int, rightIndex int) bool {
		return posts[leftIndex].CreateAt < posts[rightIndex].CreateAt
	})
	return posts, nil
}

func postsBeforeMessage(posts []mattermostHistoryPost, messageID string) []mattermostHistoryPost {
	for index, post := range posts {
		if post.ID == messageID {
			return posts[:index]
		}
	}
	filteredPosts := make([]mattermostHistoryPost, 0, len(posts))
	for _, post := range posts {
		if post.ID != messageID {
			filteredPosts = append(filteredPosts, post)
		}
	}
	return filteredPosts
}

func (service Service) mattermostSpeaker(ctx context.Context, userID string) string {
	return service.mattermostSender(ctx, userID).Name
}

func (service Service) mattermostSender(ctx context.Context, userID string) platformContextSender {
	if strings.TrimSpace(userID) == "" {
		return platformContextSender{Platform: "mattermost", Name: "unknown"}
	}
	var response struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Username  string `json:"username"`
		Nickname  string `json:"nickname"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/"+url.PathEscape(userID), nil, &response)
	if errorValue != nil {
		return platformContextSender{
			Platform: "mattermost",
			SenderID: userID,
			UserID:   userID,
			Name:     userID,
		}
	}
	senderID := firstNonEmpty(response.ID, userID)
	canonicalName := firstNonEmpty(
		strings.TrimSpace(response.Nickname),
		strings.TrimSpace(response.FirstName),
		response.Username,
		senderID,
	)
	return platformContextSender{
		Platform:    "mattermost",
		SenderID:    senderID,
		UserID:      senderID,
		Handle:      response.Username,
		Email:       response.Email,
		Name:        canonicalName,
		CallingName: identity.CallingName(canonicalName),
	}
}

func (service Service) mattermostTextWithReadableMentions(ctx context.Context, message string, mentionUserIDs []string, senderByUserID map[string]platformContextSender) string {
	text := strings.TrimSpace(message)
	if text == "" || len(mentionUserIDs) == 0 {
		return text
	}
	for _, userID := range mentionUserIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		senderInfo, hasSender := senderByUserID[userID]
		if !hasSender {
			senderInfo = service.mattermostSender(ctx, userID)
			senderByUserID[userID] = senderInfo
		}
		text = annotateMattermostMention(text, senderInfo)
	}
	return text
}

func annotateMattermostMention(text string, sender platformContextSender) string {
	replacement := mattermostReadableMention(sender)
	if replacement == "" {
		return text
	}
	for _, alias := range mattermostMentionAliases(sender) {
		text = replaceMattermostMentionAlias(text, alias, replacement)
	}
	return text
}

func mattermostReadableMention(sender platformContextSender) string {
	handle := strings.TrimPrefix(strings.TrimSpace(sender.Handle), "@")
	name := firstNonEmpty(strings.TrimSpace(sender.Name), strings.TrimSpace(sender.CallingName))
	if handle == "" {
		return ""
	}
	if name == "" || strings.EqualFold(name, handle) {
		return "@" + handle
	}
	return "@" + handle + "(" + name + ")"
}

func mattermostMentionAliases(sender platformContextSender) []string {
	aliases := []string{}
	for _, value := range []string{sender.Handle, sender.CallingName, sender.Name} {
		alias := strings.TrimPrefix(strings.TrimSpace(value), "@")
		if alias == "" || containsString(aliases, alias) {
			continue
		}
		aliases = append(aliases, alias)
	}
	return aliases
}

func replaceMattermostMentionAlias(text string, alias string, replacement string) string {
	alias = strings.TrimPrefix(strings.TrimSpace(alias), "@")
	if text == "" || alias == "" || replacement == "" {
		return text
	}
	token := "@" + alias
	var builder strings.Builder
	startIndex := 0
	for {
		matchOffset := strings.Index(text[startIndex:], token)
		if matchOffset < 0 {
			builder.WriteString(text[startIndex:])
			break
		}
		matchStart := startIndex + matchOffset
		matchEnd := matchStart + len(token)
		if !isMattermostMentionTokenBoundary(text, matchStart, matchEnd) {
			builder.WriteString(text[startIndex:matchEnd])
			startIndex = matchEnd
			continue
		}
		builder.WriteString(text[startIndex:matchStart])
		builder.WriteString(replacement)
		startIndex = matchEnd
	}
	return builder.String()
}

func isMattermostMentionTokenBoundary(text string, matchStart int, matchEnd int) bool {
	if matchStart > 0 {
		previousRune, _ := utf8.DecodeLastRuneInString(text[:matchStart])
		if isMattermostMentionAdjacentRune(previousRune) {
			return false
		}
	}
	if matchEnd < len(text) {
		nextRune, _ := utf8.DecodeRuneInString(text[matchEnd:])
		if isMattermostMentionAdjacentRune(nextRune) {
			return false
		}
	}
	return true
}

func isMattermostMentionAdjacentRune(value rune) bool {
	return unicode.IsLetter(value) || unicode.IsDigit(value) || value == '_' || value == '-' || value == '.'
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func mustEncodePlatformHandle(handle platformHandle) string {
	value, errorValue := encodePlatformHandle(handle)
	if errorValue != nil {
		return ""
	}
	return value
}
