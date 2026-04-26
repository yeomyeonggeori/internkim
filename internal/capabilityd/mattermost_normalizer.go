package capabilityd

import (
	"encoding/json"
	"strings"
)

type mattermostPost struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
	RootID    string `json:"root_id"`
	Type      string `json:"type"`
	CreateAt  int64  `json:"create_at"`
}

type mattermostWebSocketMessage struct {
	Event string         `json:"event"`
	Data  map[string]any `json:"data"`
}

func normalizeMattermostWebSocketPayload(payload []byte, botUserID string) (platformInboundEvent, bool, error) {
	var message mattermostWebSocketMessage
	if errorValue := json.Unmarshal(payload, &message); errorValue != nil {
		return platformInboundEvent{}, false, errorValue
	}
	if message.Event != "posted" {
		return platformInboundEvent{}, false, nil
	}

	postDocument, isFound := message.Data["post"].(string)
	if !isFound || strings.TrimSpace(postDocument) == "" {
		return platformInboundEvent{}, false, nil
	}
	var post mattermostPost
	if errorValue := json.Unmarshal([]byte(postDocument), &post); errorValue != nil {
		return platformInboundEvent{}, false, errorValue
	}
	channelType, _ := message.Data["channel_type"].(string)
	return normalizeMattermostPost(post, botUserID, channelType)
}

func normalizeMattermostPost(post mattermostPost, botUserID string, channelType string) (platformInboundEvent, bool, error) {
	if strings.TrimSpace(post.ID) == "" || strings.TrimSpace(post.UserID) == "" {
		return platformInboundEvent{}, false, nil
	}
	if strings.TrimSpace(post.Type) != "" || strings.TrimSpace(post.UserID) == strings.TrimSpace(botUserID) {
		return platformInboundEvent{}, false, nil
	}

	replyRootID := ""
	if !strings.EqualFold(strings.TrimSpace(channelType), "D") {
		replyRootID = firstNonEmpty(post.RootID, post.ID)
	}
	conversationID := mattermostConversationID(channelType, post.ChannelID, replyRootID)
	handle := platformHandle{
		Platform:       "mattermost",
		ConversationID: conversationID,
		ChannelID:      post.ChannelID,
		ChannelType:    channelType,
		RootID:         replyRootID,
		MessageID:      post.ID,
	}
	replyTargetID, errorValue := encodePlatformHandle(handle)
	if errorValue != nil {
		return platformInboundEvent{}, false, errorValue
	}
	historyCursor, errorValue := encodePlatformHandle(handle)
	if errorValue != nil {
		return platformInboundEvent{}, false, errorValue
	}

	return platformInboundEvent{
		ConversationID: conversationID,
		MessageID:      post.ID,
		ReplyTargetID:  replyTargetID,
		SenderID:       post.UserID,
		Prompt:         post.Message,
		Context: platformEventContext{
			HistoryCursor: historyCursor,
		},
	}, true, nil
}

func mattermostConversationID(channelType string, channelID string, rootID string) string {
	if strings.TrimSpace(rootID) != "" {
		return "thread:" + strings.TrimSpace(channelID) + ":" + strings.TrimSpace(rootID)
	}
	return namespacedConversationID(channelType, channelID)
}
