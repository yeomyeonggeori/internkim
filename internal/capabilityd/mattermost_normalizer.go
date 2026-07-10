package capabilityd

import (
	"encoding/json"
	"strings"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

type mattermostPost struct {
	ID        string   `json:"id"`
	UserID    string   `json:"user_id"`
	ChannelID string   `json:"channel_id"`
	Message   string   `json:"message"`
	RootID    string   `json:"root_id"`
	Type      string   `json:"type"`
	CreateAt  int64    `json:"create_at"`
	FileIDs   []string `json:"file_ids"`
	Metadata  struct {
		Mentions []string `json:"mentions"`
	} `json:"metadata"`
}

type mattermostWebSocketMessage struct {
	Event string         `json:"event"`
	Data  map[string]any `json:"data"`
}

type mattermostPostMetadata struct {
	ChannelType string
	ChannelName string
}

func normalizeMattermostWebSocketPayload(payload []byte, botUserID string, botUsername string) (platformInboundEvent, bool, error) {
	post, metadata, hasPost, errorValue := mattermostWebSocketPost(payload)
	if errorValue != nil || !hasPost {
		return platformInboundEvent{}, false, errorValue
	}
	addressing := mattermostAddressingFromMessage(post.Message, botUsername)
	return normalizeMattermostPost(post, botUserID, metadata.ChannelType, metadata.ChannelName, addressing)
}

func mattermostWebSocketPost(payload []byte) (mattermostPost, mattermostPostMetadata, bool, error) {
	var message mattermostWebSocketMessage
	if errorValue := json.Unmarshal(payload, &message); errorValue != nil {
		return mattermostPost{}, mattermostPostMetadata{}, false, errorValue
	}
	if message.Event != "posted" {
		return mattermostPost{}, mattermostPostMetadata{}, false, nil
	}

	postDocument, isFound := message.Data["post"].(string)
	if !isFound || strings.TrimSpace(postDocument) == "" {
		return mattermostPost{}, mattermostPostMetadata{}, false, nil
	}
	var post mattermostPost
	if errorValue := json.Unmarshal([]byte(postDocument), &post); errorValue != nil {
		return mattermostPost{}, mattermostPostMetadata{}, false, errorValue
	}
	channelType, _ := message.Data["channel_type"].(string)
	channelName, _ := message.Data["channel_name"].(string)
	metadata := mattermostPostMetadata{
		ChannelType: channelType,
		ChannelName: channelName,
	}
	return post, metadata, true, nil
}

func normalizeMattermostPost(post mattermostPost, botUserID string, channelType string, channelName string, addressing platformAddressing) (platformInboundEvent, bool, error) {
	if strings.TrimSpace(post.ID) == "" || strings.TrimSpace(post.UserID) == "" {
		return platformInboundEvent{}, false, nil
	}
	if strings.TrimSpace(post.Type) != "" || strings.TrimSpace(post.UserID) == strings.TrimSpace(botUserID) {
		return platformInboundEvent{}, false, nil
	}
	if strings.TrimSpace(channelName) == mattermostdefaults.AttendanceChannelName {
		return platformInboundEvent{}, false, nil
	}

	isDirect := strings.EqualFold(strings.TrimSpace(channelType), "D")
	if !isDirect && mattermostMentionsExcludeBot(post.Message, addressing) {
		return platformInboundEvent{}, false, nil
	}
	if !isDirect && !addressing.BotMentioned && isURLOnlyMessage(post.Message) {
		return platformInboundEvent{}, false, nil
	}
	replyRootID := ""
	if isDirect {
		replyRootID = mattermostDirectReplyRootID(post)
	} else {
		replyRootID = firstNonEmpty(post.RootID, post.ID)
	}
	conversationID := mattermostConversationID(channelType, post.ChannelID, replyRootID)
	replyHandle := platformHandle{
		Platform:       "mattermost",
		ConversationID: conversationID,
		ChannelID:      post.ChannelID,
		ChannelType:    channelType,
		RootID:         replyRootID,
		MessageID:      post.ID,
	}
	historyHandle := mattermostHistoryHandle(replyHandle, post, isDirect)
	replyTargetID, errorValue := encodePlatformHandle(replyHandle)
	if errorValue != nil {
		return platformInboundEvent{}, false, errorValue
	}
	historyCursor, errorValue := encodePlatformHandle(historyHandle)
	if errorValue != nil {
		return platformInboundEvent{}, false, errorValue
	}

	inputAttachments := mattermostInputAttachments(post.ID, post.FileIDs)
	prompt := strings.TrimSpace(post.Message)
	attachmentsOnly := prompt == "" && len(inputAttachments) > 0
	if attachmentsOnly {
		prompt = "User attached file(s)."
	}

	return platformInboundEvent{
		ConversationID: conversationID,
		MessageID:      post.ID,
		ReplyTargetID:  replyTargetID,
		SenderID:       post.UserID,
		Prompt:         prompt,
		Context: platformEventContext{
			HistoryCursor:    historyCursor,
			ConversationType: channelType,
			ChannelID:        post.ChannelID,
			ChannelName:      channelName,
			Addressing:       addressing,
			AttachmentsOnly:  attachmentsOnly,
			InputAttachments: inputAttachments,
			Materials:        inputAttachments,
		},
	}, true, nil
}

func mattermostInputAttachments(messageID string, fileIDs []string) []platformInputAttachment {
	attachments := []platformInputAttachment{}
	for _, fileID := range fileIDs {
		fileID = strings.TrimSpace(fileID)
		if fileID == "" {
			continue
		}
		attachments = append(attachments, platformInputAttachment{
			Platform:  "mattermost",
			FileID:    fileID,
			MessageID: messageID,
		})
	}
	return attachments
}

func mattermostDirectReplyRootID(post mattermostPost) string {
	return firstNonEmpty(post.RootID, post.ID)
}

func mattermostAddressingFromMessage(message string, botUsername string) platformAddressing {
	botUsername = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(botUsername), "@"))
	addressing := platformAddressing{}
	for _, mention := range mattermostMentionTokens(message) {
		if strings.EqualFold(mention, botUsername) {
			addressing.BotMentioned = true
			continue
		}
		if isMattermostBroadcastMention(mention) {
			continue
		}
		addressing.OtherPersonMentioned = true
	}
	return addressing
}

func mattermostMentionsExcludeBot(message string, addressing platformAddressing) bool {
	if addressing.BotMentioned {
		return false
	}
	return len(mattermostMentionTokens(message)) > 0
}

func mattermostMentionTokens(message string) []string {
	fields := strings.Fields(message)
	mentions := make([]string, 0, len(fields))
	for _, field := range fields {
		token := strings.Trim(field, " \t\r\n.,;:!?()[]{}<>\"'")
		if strings.HasPrefix(token, "@") && len(token) > 1 {
			mentions = append(mentions, strings.TrimPrefix(token, "@"))
		}
	}
	return mentions
}

func isURLOnlyMessage(message string) bool {
	tokens := strings.Fields(strings.TrimSpace(message))
	if len(tokens) == 0 {
		return false
	}
	for _, token := range tokens {
		if !strings.HasPrefix(token, "http://") && !strings.HasPrefix(token, "https://") {
			return false
		}
	}
	return true
}

func isMattermostBroadcastMention(mention string) bool {
	switch strings.ToLower(strings.TrimPrefix(strings.TrimSpace(mention), "@")) {
	case "all", "channel", "here":
		return true
	}
	return false
}

func mattermostHistoryHandle(replyHandle platformHandle, post mattermostPost, isDirect bool) platformHandle {
	if !isDirect || strings.TrimSpace(post.RootID) != "" {
		return replyHandle
	}
	return platformHandle{
		Platform:       replyHandle.Platform,
		ConversationID: namespacedConversationID(replyHandle.ChannelType, replyHandle.ChannelID),
		ChannelID:      replyHandle.ChannelID,
		ChannelType:    replyHandle.ChannelType,
		MessageID:      replyHandle.MessageID,
	}
}

func mattermostConversationID(channelType string, channelID string, rootID string) string {
	if strings.TrimSpace(rootID) != "" {
		return "thread:" + strings.TrimSpace(channelID) + ":" + strings.TrimSpace(rootID)
	}
	return namespacedConversationID(channelType, channelID)
}
