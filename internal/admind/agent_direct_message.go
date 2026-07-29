package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type channelParticipant struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarURL,omitempty"`
}

type agentDirectMessage struct {
	ID           string                   `json:"id"`
	ThreadRootID string                   `json:"threadRootId,omitempty"`
	Sender       channelParticipant       `json:"sender"`
	Text         string                   `json:"text"`
	SentAt       string                   `json:"sentAt"`
	IsError      bool                     `json:"isError,omitempty"`
	Reactions    []agentMessageReaction   `json:"reactions,omitempty"`
	Attachments  []agentMessageAttachment `json:"attachments,omitempty"`
}

type agentMessageAttachment struct {
	Kind      string `json:"kind"`
	URL       string `json:"url"`
	Filename  string `json:"filename,omitempty"`
	MimeType  string `json:"mimeType,omitempty"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
}

type agentMessageReaction struct {
	Emoji    string `json:"emoji"`
	Count    int    `json:"count"`
	ImageURL string `json:"imageURL,omitempty"`
}

type agentConversationResponse struct {
	ConversationID string               `json:"conversationID"`
	CurrentUserID  string               `json:"currentUserId"`
	Messages       []agentDirectMessage `json:"messages"`
}

type agentDirectMessageRequest struct {
	Message       string                         `json:"message"`
	Attachments   []agentDirectMessageAttachment `json:"attachments"`
	ReplyToRootID string                         `json:"replyToRootId"`
}

type agentDirectMessageAttachment struct {
	Filename      string `json:"filename"`
	ContentType   string `json:"contentType"`
	ContentBase64 string `json:"contentBase64"`
}

type chatdDirectMessageChannel struct {
	ChannelID     string `json:"channelID"`
	ReplyTargetID string `json:"replyTargetID"`
	HistoryCursor string `json:"historyCursor"`
	UserPubkeyHex string `json:"userPubkeyHex"`
	BotName       string `json:"botName"`
	BotAvatarURL  string `json:"botAvatarURL"`
}

type chatdHistoryMessage struct {
	ID              string                   `json:"id"`
	ThreadRootID    string                   `json:"threadRootId"`
	Speaker         string                   `json:"speaker"`
	SenderID        string                   `json:"senderId"`
	SenderAvatarURL string                   `json:"senderAvatarUrl"`
	Text            string                   `json:"text"`
	SentAt          string                   `json:"sentAt"`
	IsBot           bool                     `json:"isBot"`
	IsError         bool                     `json:"isError"`
	Reactions       []agentMessageReaction   `json:"reactions"`
	Attachments     []agentMessageAttachment `json:"attachments"`
}

type chatdHistoryResponse struct {
	Messages []chatdHistoryMessage `json:"messages"`
}

func (service *Service) handleAgentDirectMessage(responseWriter http.ResponseWriter, request *http.Request) {
	actorEmail := service.webActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "unauthorized", http.StatusUnauthorized)
		return
	}
	channelID := request.URL.Query().Get("channelId")
	switch request.Method {
	case http.MethodGet:
		service.writeAgentConversation(responseWriter, request, actorEmail, channelID)
	case http.MethodPost:
		service.sendAgentDirectMessage(responseWriter, request, actorEmail, channelID)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) writeAgentConversation(responseWriter http.ResponseWriter, request *http.Request, actorEmail string, channelID string) {
	channel, errorValue := service.ensureAgentDirectMessageChannel(request.Context(), actorEmail, channelID)
	if errorValue != nil {
		service.writeAgentDirectMessageError(responseWriter, errorValue)
		return
	}
	var history chatdHistoryResponse
	historyRequest := map[string]any{"historyCursor": channel.HistoryCursor, "limit": 100}
	if errorValue := service.chatdPlatformRequest(request.Context(), "history.fetch", historyRequest, &history); errorValue != nil {
		service.writeAgentDirectMessageError(responseWriter, errorValue)
		return
	}
	service.writeJSON(responseWriter, service.rewriteConversationMedia(agentConversationResponse{
		ConversationID: channel.ChannelID,
		CurrentUserID:  channel.UserPubkeyHex,
		Messages:       agentDirectMessagesFromHistory(history.Messages, channel),
	}))
}

func (service *Service) sendAgentDirectMessage(responseWriter http.ResponseWriter, request *http.Request, actorEmail string, channelID string) {
	var requestDocument agentDirectMessageRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&requestDocument); errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(requestDocument.Message) == "" && len(requestDocument.Attachments) == 0 {
		http.Error(responseWriter, "message is required", http.StatusBadRequest)
		return
	}
	userSecretHex, errorValue := service.personBuzzSecret(request.Context(), actorEmail)
	if errorValue != nil {
		service.writeAgentDirectMessageError(responseWriter, errorValue)
		return
	}
	sendRequest := map[string]any{
		"userSecretHex": userSecretHex,
		"message":       requestDocument.Message,
		"attachments":   requestDocument.Attachments,
	}
	if channelID != "" {
		sendRequest["channelId"] = channelID
	}
	if requestDocument.ReplyToRootID != "" {
		sendRequest["replyToRootId"] = requestDocument.ReplyToRootID
	}
	if errorValue := service.chatdPlatformRequest(request.Context(), "dm.send", sendRequest, nil); errorValue != nil {
		service.writeAgentDirectMessageError(responseWriter, errorValue)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"accepted": true})
}

func (service *Service) ensureAgentDirectMessageChannel(ctx context.Context, actorEmail string, channelID string) (chatdDirectMessageChannel, error) {
	userSecretHex, errorValue := service.personBuzzSecret(ctx, actorEmail)
	if errorValue != nil {
		return chatdDirectMessageChannel{}, errorValue
	}
	var channel chatdDirectMessageChannel
	ensureRequest := map[string]any{"userSecretHex": userSecretHex}
	if channelID != "" {
		ensureRequest["channelId"] = channelID
	}
	if errorValue := service.chatdPlatformRequest(ctx, "dm.ensure", ensureRequest, &channel); errorValue != nil {
		return chatdDirectMessageChannel{}, errorValue
	}
	return channel, nil
}

type agentConversationSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	AvatarURL string `json:"avatarURL,omitempty"`
}

type agentConversationsResponse struct {
	Conversations []agentConversationSummary `json:"conversations"`
}

type agentPerson struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarURL,omitempty"`
}

type agentPeopleResponse struct {
	People []agentPerson `json:"people"`
}

func (service *Service) handleAgentPeople(responseWriter http.ResponseWriter, request *http.Request) {
	actorEmail := service.webActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "unauthorized", http.StatusUnauthorized)
		return
	}
	userSecretHex, errorValue := service.personBuzzSecret(request.Context(), actorEmail)
	if errorValue != nil {
		service.writeAgentDirectMessageError(responseWriter, errorValue)
		return
	}
	var response agentPeopleResponse
	listRequest := map[string]any{"userSecretHex": userSecretHex}
	if errorValue := service.chatdPlatformRequest(request.Context(), "people.list", listRequest, &response); errorValue != nil {
		service.writeAgentDirectMessageError(responseWriter, errorValue)
		return
	}
	for index := range response.People {
		response.People[index].AvatarURL = service.rewriteBuzzMedia(response.People[index].AvatarURL)
	}
	service.writeJSON(responseWriter, response)
}

type ensureDirectMessageRequest struct {
	PersonID string `json:"personId"`
}

func (service *Service) handleEnsureDirectMessage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	actorEmail := service.webActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "unauthorized", http.StatusUnauthorized)
		return
	}
	var requestDocument ensureDirectMessageRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&requestDocument); errorValue != nil || strings.TrimSpace(requestDocument.PersonID) == "" {
		http.Error(responseWriter, "personId is required", http.StatusBadRequest)
		return
	}
	userSecretHex, errorValue := service.personBuzzSecret(request.Context(), actorEmail)
	if errorValue != nil {
		service.writeAgentDirectMessageError(responseWriter, errorValue)
		return
	}
	var channel chatdDirectMessageChannel
	ensureRequest := map[string]any{
		"userSecretHex":        userSecretHex,
		"counterpartPubkeyHex": requestDocument.PersonID,
	}
	if errorValue := service.chatdPlatformRequest(request.Context(), "dm.ensure", ensureRequest, &channel); errorValue != nil {
		service.writeAgentDirectMessageError(responseWriter, errorValue)
		return
	}
	service.writeJSON(responseWriter, map[string]string{"channelId": channel.ChannelID})
}

func (service *Service) handleAgentChannels(responseWriter http.ResponseWriter, request *http.Request) {
	actorEmail := service.webActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "unauthorized", http.StatusUnauthorized)
		return
	}
	userSecretHex, errorValue := service.personBuzzSecret(request.Context(), actorEmail)
	if errorValue != nil {
		service.writeAgentDirectMessageError(responseWriter, errorValue)
		return
	}
	var response agentConversationsResponse
	listRequest := map[string]any{"userSecretHex": userSecretHex}
	if errorValue := service.chatdPlatformRequest(request.Context(), "conversations.list", listRequest, &response); errorValue != nil {
		service.writeAgentDirectMessageError(responseWriter, errorValue)
		return
	}
	for index := range response.Conversations {
		response.Conversations[index].AvatarURL = service.rewriteBuzzMedia(response.Conversations[index].AvatarURL)
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) chatdPlatformRequest(ctx context.Context, capabilityName string, requestBody any, responseValue any) error {
	endpoint := strings.TrimRight(strings.TrimSpace(service.Configuration.ChatdEndpoint), "/")
	if endpoint == "" {
		return errors.New("chatd endpoint is not configured")
	}
	platform := strings.TrimSpace(service.Configuration.ChatdPlatform)
	if platform == "" {
		return errors.New("chatd platform is not configured")
	}
	document, errorValue := json.Marshal(requestBody)
	if errorValue != nil {
		return errorValue
	}
	requestURL := endpoint + "/v1/platform/" + platform + "/" + capabilityName
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("chatd %s returned %d", capabilityName, response.StatusCode)
	}
	if responseValue == nil {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(responseValue)
}

func (service *Service) writeAgentDirectMessageError(responseWriter http.ResponseWriter, errorValue error) {
	if errors.Is(errorValue, errBuzzIdentitySecretMissing) {
		http.Error(responseWriter, "identity_not_provisioned", http.StatusConflict)
		return
	}
	http.Error(responseWriter, "agent_unreachable", http.StatusBadGateway)
}

func agentDirectMessagesFromHistory(historyMessages []chatdHistoryMessage, channel chatdDirectMessageChannel) []agentDirectMessage {
	messages := make([]agentDirectMessage, 0, len(historyMessages))
	for index, historyMessage := range historyMessages {
		isCurrentUser := historyMessage.SenderID != "" && historyMessage.SenderID == channel.UserPubkeyHex
		sender := channelParticipant{
			ID:        historyMessage.SenderID,
			Name:      historyMessage.Speaker,
			AvatarURL: historyMessage.SenderAvatarURL,
		}
		if !isCurrentUser {
			if channel.BotName != "" {
				sender.Name = channel.BotName
			}
			if sender.AvatarURL == "" {
				sender.AvatarURL = channel.BotAvatarURL
			}
		}
		messageID := historyMessage.ID
		if messageID == "" {
			messageID = strings.TrimSpace(historyMessage.SentAt) + ":" + strconv.Itoa(index)
		}
		messages = append(messages, agentDirectMessage{
			ID:           messageID,
			ThreadRootID: historyMessage.ThreadRootID,
			Sender:       sender,
			Text:         historyMessage.Text,
			SentAt:       historyMessage.SentAt,
			IsError:      historyMessage.IsError,
			Reactions:    historyMessage.Reactions,
			Attachments:  historyMessage.Attachments,
		})
	}
	sort.SliceStable(messages, func(leftIndex int, rightIndex int) bool {
		return messages[leftIndex].SentAt < messages[rightIndex].SentAt
	})
	return messages
}
