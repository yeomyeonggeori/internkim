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

type agentDirectMessage struct {
	ID     string `json:"id"`
	Author string `json:"author"`
	Text   string `json:"text"`
	SentAt string `json:"sentAt"`
}

type agentConversationResponse struct {
	ConversationID string               `json:"conversationID"`
	Messages       []agentDirectMessage `json:"messages"`
}

type agentDirectMessageRequest struct {
	Message string `json:"message"`
}

type chatdDirectMessageChannel struct {
	ChannelID     string `json:"channelID"`
	ReplyTargetID string `json:"replyTargetID"`
	HistoryCursor string `json:"historyCursor"`
}

type chatdHistoryMessage struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
	SentAt  string `json:"sentAt"`
	IsBot   bool   `json:"isBot"`
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
	switch request.Method {
	case http.MethodGet:
		service.writeAgentConversation(responseWriter, request, actorEmail)
	case http.MethodPost:
		service.sendAgentDirectMessage(responseWriter, request, actorEmail)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) writeAgentConversation(responseWriter http.ResponseWriter, request *http.Request, actorEmail string) {
	channel, errorValue := service.ensureAgentDirectMessageChannel(request.Context(), actorEmail)
	if errorValue != nil {
		service.writeAgentDirectMessageError(responseWriter, errorValue)
		return
	}
	var history chatdHistoryResponse
	historyRequest := map[string]any{"historyCursor": channel.HistoryCursor, "limit": 50}
	if errorValue := service.chatdPlatformRequest(request.Context(), "history.fetch", historyRequest, &history); errorValue != nil {
		service.writeAgentDirectMessageError(responseWriter, errorValue)
		return
	}
	service.writeJSON(responseWriter, agentConversationResponse{
		ConversationID: channel.ChannelID,
		Messages:       agentDirectMessagesFromHistory(history.Messages),
	})
}

func (service *Service) sendAgentDirectMessage(responseWriter http.ResponseWriter, request *http.Request, actorEmail string) {
	var requestDocument agentDirectMessageRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&requestDocument); errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(requestDocument.Message) == "" {
		http.Error(responseWriter, "message is required", http.StatusBadRequest)
		return
	}
	userSecretHex, errorValue := service.readBuzzIdentitySecret(actorEmail)
	if errorValue != nil {
		service.writeAgentDirectMessageError(responseWriter, errorValue)
		return
	}
	sendRequest := map[string]any{"userSecretHex": userSecretHex, "message": requestDocument.Message}
	if errorValue := service.chatdPlatformRequest(request.Context(), "dm.send", sendRequest, nil); errorValue != nil {
		service.writeAgentDirectMessageError(responseWriter, errorValue)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"accepted": true})
}

func (service *Service) ensureAgentDirectMessageChannel(ctx context.Context, actorEmail string) (chatdDirectMessageChannel, error) {
	userSecretHex, errorValue := service.readBuzzIdentitySecret(actorEmail)
	if errorValue != nil {
		return chatdDirectMessageChannel{}, errorValue
	}
	var channel chatdDirectMessageChannel
	ensureRequest := map[string]any{"userSecretHex": userSecretHex}
	if errorValue := service.chatdPlatformRequest(ctx, "dm.ensure", ensureRequest, &channel); errorValue != nil {
		return chatdDirectMessageChannel{}, errorValue
	}
	return channel, nil
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

func agentDirectMessagesFromHistory(historyMessages []chatdHistoryMessage) []agentDirectMessage {
	messages := make([]agentDirectMessage, 0, len(historyMessages))
	for index, historyMessage := range historyMessages {
		author := "me"
		if historyMessage.IsBot {
			author = "agent"
		}
		messages = append(messages, agentDirectMessage{
			ID:     strings.TrimSpace(historyMessage.SentAt) + ":" + strconv.Itoa(index),
			Author: author,
			Text:   historyMessage.Text,
			SentAt: historyMessage.SentAt,
		})
	}
	sort.SliceStable(messages, func(leftIndex int, rightIndex int) bool {
		return messages[leftIndex].SentAt < messages[rightIndex].SentAt
	})
	return messages
}
