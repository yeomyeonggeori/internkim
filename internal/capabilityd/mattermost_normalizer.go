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

type mattermostPostMetadata struct {
	ChannelType string
	ChannelName string
	Mentions    []string
}

func normalizeMattermostWebSocketPayload(payload []byte, botUserID string) (platformInboundEvent, bool, error) {
	post, metadata, hasPost, errorValue := mattermostWebSocketPost(payload)
	if errorValue != nil || !hasPost {
		return platformInboundEvent{}, false, errorValue
	}
	isBotMentioned := containsString(metadata.Mentions, strings.TrimSpace(botUserID))
	return normalizeMattermostPost(post, botUserID, metadata.ChannelType, metadata.ChannelName, isBotMentioned)
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
		Mentions:    parseMattermostMentionsList(message.Data["mentions"]),
	}
	return post, metadata, true, nil
}

func parseMattermostMentionsList(raw any) []string {
	document, isString := raw.(string)
	if !isString || strings.TrimSpace(document) == "" {
		return nil
	}
	var mentions []string
	if errorValue := json.Unmarshal([]byte(document), &mentions); errorValue != nil {
		return nil
	}
	return mentions
}

func containsString(values []string, target string) bool {
	if strings.TrimSpace(target) == "" {
		return false
	}
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func normalizeMattermostPost(post mattermostPost, botUserID string, channelType string, channelName string, isBotMentioned bool) (platformInboundEvent, bool, error) {
	if strings.TrimSpace(post.ID) == "" || strings.TrimSpace(post.UserID) == "" {
		return platformInboundEvent{}, false, nil
	}
	if strings.TrimSpace(post.Type) != "" || strings.TrimSpace(post.UserID) == strings.TrimSpace(botUserID) {
		return platformInboundEvent{}, false, nil
	}

	isDirect := strings.EqualFold(strings.TrimSpace(channelType), "D")
	if !isDirect && !isDefaultMattermostChannel(channelName) && !isBotMentioned {
		return platformInboundEvent{}, false, nil
	}

	replyRootID := ""
	if isDirect {
		replyRootID = mattermostDirectReplyRootID(post)
	} else {
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
			HistoryCursor:    historyCursor,
			ConversationType: channelType,
			ChannelID:        post.ChannelID,
			ChannelName:      channelName,
		},
	}, true, nil
}

func isDefaultMattermostChannel(channelName string) bool {
	switch strings.ToLower(strings.TrimSpace(channelName)) {
	case "town-square":
		return true
	}
	return false
}

func mattermostDirectReplyRootID(post mattermostPost) string {
	return strings.TrimSpace(post.RootID)
}

func mattermostConversationID(channelType string, channelID string, rootID string) string {
	if strings.TrimSpace(rootID) != "" {
		return "thread:" + strings.TrimSpace(channelID) + ":" + strings.TrimSpace(rootID)
	}
	return namespacedConversationID(channelType, channelID)
}
