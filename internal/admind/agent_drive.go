package admind

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

type agentMessageRequest struct {
	ConversationID string `json:"conversationID"`
	MessageID      string `json:"messageID"`
	Message        string `json:"message"`
}

type agentInboundEvent struct {
	ConversationID string `json:"conversationID"`
	MessageID      string `json:"messageID"`
	SenderID       string `json:"senderID"`
	ReplyTargetID  string `json:"replyTargetID"`
	Prompt         string `json:"prompt"`
}

func (service *Service) handleAgentMessage(responseWriter http.ResponseWriter, request *http.Request, actor publicToolGatewayActor) {
	if actorPermissionRank(actor) < publicAPIPermissionRank(publicAPIPermissionWrite) {
		http.Error(responseWriter, "write permission required", http.StatusForbidden)
		return
	}
	var payload agentMessageRequest
	if errorValue := decodeOptionalJSONBody(request.Body, &payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Message) == "" {
		http.Error(responseWriter, "message is required", http.StatusBadRequest)
		return
	}
	conversationID := strings.TrimSpace(payload.ConversationID)
	if conversationID == "" {
		conversationID = "dm:api:" + actor.Actor.PersonID + ":" + randomHex(8)
	}
	messageID := firstNonEmpty(strings.TrimSpace(payload.MessageID), "api_"+randomHex(12))
	event := agentInboundEvent{
		ConversationID: conversationID,
		MessageID:      messageID,
		SenderID:       actor.Actor.Email,
		ReplyTargetID:  conversationID,
		Prompt:         payload.Message,
	}
	var result json.RawMessage
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodPost, "/connectors/api/events", event, &result); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]any{
		"conversationID": conversationID,
		"messageID":      messageID,
		"result":         result,
	})
}

func (service *Service) handleAgentReplies(responseWriter http.ResponseWriter, request *http.Request, actor publicToolGatewayActor) {
	if actorPermissionRank(actor) < publicAPIPermissionRank(publicAPIPermissionWrite) {
		http.Error(responseWriter, "write permission required", http.StatusForbidden)
		return
	}
	conversationID := strings.TrimSpace(request.URL.Query().Get("conversationID"))
	if conversationID == "" {
		http.Error(responseWriter, "conversationID is required", http.StatusBadRequest)
		return
	}
	var replies json.RawMessage
	path := "/agent/api/replies?conversationID=" + url.QueryEscape(conversationID)
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, path, nil, &replies); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, replies)
}
