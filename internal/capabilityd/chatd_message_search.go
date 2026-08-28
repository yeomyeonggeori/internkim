package capabilityd

import (
	"context"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type chatdMessageSearchRequest struct {
	ReplyTargetID      string   `json:"replyTargetID,omitempty"`
	ChannelID          string   `json:"channelID,omitempty"`
	ChannelName        string   `json:"channelName,omitempty"`
	RootMessageID      string   `json:"rootMessageID,omitempty"`
	MessageIDs         []string `json:"messageIDs,omitempty"`
	AuthoredBy         string   `json:"authoredBy,omitempty"`
	RequesterPubkeyHex string   `json:"requesterPubkeyHex,omitempty"`
	Queries            []string `json:"queries"`
	Limit              int      `json:"limit,omitempty"`
}

type chatdMessageSearchCandidate struct {
	MessageID           string  `json:"messageID"`
	ChannelID           string  `json:"channelID"`
	RootMessageID       string  `json:"rootMessageID,omitempty"`
	AuthorPubkeyHex     string  `json:"authorPubkeyHex"`
	AuthoredByAssistant bool    `json:"authoredByAssistant"`
	CreatedAt           int64   `json:"createdAt"`
	Text                string  `json:"text"`
	Score               float64 `json:"score"`
}

type chatdMessageSearchResponse struct {
	ChannelID  string                        `json:"channelID"`
	Candidates []chatdMessageSearchCandidate `json:"candidates"`
}

func (service Service) invokeChatdPlatformMessageSearch(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageSearchInput) (capabilities.ToolInvokeResponse, error) {
	searchRequest, scope, failure, hasFailure := chatdMessageSearchRequestFromInput(request.Context, input)
	if hasFailure {
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	var response chatdMessageSearchResponse
	if errorValue := service.chatdPlatformRequest(ctx, request.Context.Platform, "message.search", searchRequest, &response); errorValue != nil {
		failure := mattermostToolStaticFailure("message_search_failed", "platform_search", errorValue.Error())
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	result := canonicalChatdMessageSearchResult(response, input, scope, request.Context.RequesterPlatformUserID)
	return mattermostToolSuccessResponse(request.ToolName, "ok", result)
}

func chatdMessageSearchRequestFromInput(toolContext capabilities.ToolInvokeContext, input platformMessageSearchInput) (chatdMessageSearchRequest, string, mattermostToolFailure, bool) {
	searchRequest := chatdMessageSearchRequest{
		MessageIDs:         input.MessageIDs,
		AuthoredBy:         input.AuthoredBy,
		RequesterPubkeyHex: strings.TrimSpace(toolContext.RequesterPlatformUserID),
		Queries:            input.Queries,
		Limit:              input.Limit,
	}
	scope := normalizedChatdMessageSearchScope(input.Scope, toolContext, input)
	switch scope {
	case "directMessage":
		failure := mattermostToolStaticFailure("unsupported_scope", "platform_route",
			"scope=directMessage is not yet routed for platform "+toolContext.Platform+"; search the current conversation or a named channel")
		return chatdMessageSearchRequest{}, scope, failure, true
	case "channel":
		if input.DeliveryTarget.ChannelID == "" && input.DeliveryTarget.ChannelName == "" {
			failure := mattermostToolStaticFailure("invalid_input", "input_decode", "scope=channel needs channelID or channelName")
			return chatdMessageSearchRequest{}, scope, failure, true
		}
		searchRequest.ChannelID = input.DeliveryTarget.ChannelID
		searchRequest.ChannelName = input.DeliveryTarget.ChannelName
	case "currentThread":
		rootMessageID := buzzReplyTargetRoot(toolContext.ReplyTargetID)
		if rootMessageID == "" {
			failure := mattermostToolStaticFailure("thread_not_available", "context", "current conversation does not have a thread root")
			return chatdMessageSearchRequest{}, scope, failure, true
		}
		searchRequest.ReplyTargetID = toolContext.ReplyTargetID
		searchRequest.RootMessageID = rootMessageID
	default:
		searchRequest.ReplyTargetID = toolContext.ReplyTargetID
		searchRequest.ChannelID = strings.TrimSpace(toolContext.ChannelID)
		if searchRequest.ReplyTargetID == "" && searchRequest.ChannelID == "" {
			failure := mattermostToolStaticFailure("channel_not_available", "context",
				"name a channelName or channelID to search; the current conversation is not available")
			return chatdMessageSearchRequest{}, scope, failure, true
		}
	}
	return searchRequest, scope, mattermostToolFailure{}, false
}

func normalizedChatdMessageSearchScope(scope string, toolContext capabilities.ToolInvokeContext, input platformMessageSearchInput) string {
	scope = strings.TrimSpace(scope)
	if scope != "" {
		return scope
	}
	if input.DeliveryTarget.ChannelID != "" || input.DeliveryTarget.ChannelName != "" {
		return "channel"
	}
	if buzzReplyTargetRoot(toolContext.ReplyTargetID) != "" {
		return "currentThread"
	}
	return "currentChannel"
}

func canonicalChatdMessageSearchResult(response chatdMessageSearchResponse, input platformMessageSearchInput, scope string, requesterPlatformUserID string) platformMessageSearchResult {
	isReadByID := len(input.MessageIDs) > 0
	candidates := make([]platformMessageSearchCandidateResult, 0, len(response.Candidates))
	deletableMessageIDs := []string{}
	for _, candidate := range response.Candidates {
		mapped := platformMessageSearchCandidateResult{
			MessageID:     candidate.MessageID,
			ChannelID:     candidate.ChannelID,
			RootMessageID: candidate.RootMessageID,
			UserID:        candidate.AuthorPubkeyHex,
			AuthoredBy:    chatdMessageSearchCandidateAuthor(candidate, requesterPlatformUserID),
			CreatedAt:     candidate.CreatedAt,
			Deletable:     candidate.AuthoredByAssistant,
		}
		if isReadByID {
			mapped.Text = candidate.Text
		} else {
			mapped.Preview = mattermostPostSearchPreview(candidate.Text, input.Queries)
		}
		if !mapped.Deletable {
			mapped.ProtectedReason = "message deletion is allowed only for the assistant's own messages"
		} else {
			deletableMessageIDs = append(deletableMessageIDs, candidate.MessageID)
		}
		candidates = append(candidates, mapped)
	}
	return platformMessageSearchResult{
		Scope:      scope,
		Queries:    input.Queries,
		AuthoredBy: canonicalPlatformMessageSearchAuthor(input.AuthoredBy),
		MessageIDs: deletableMessageIDs,
		Candidates: candidates,
	}
}

func chatdMessageSearchCandidateAuthor(candidate chatdMessageSearchCandidate, requesterPlatformUserID string) string {
	if candidate.AuthoredByAssistant {
		return "assistant"
	}
	if strings.TrimSpace(requesterPlatformUserID) != "" && strings.TrimSpace(candidate.AuthorPubkeyHex) == strings.TrimSpace(requesterPlatformUserID) {
		return "requester"
	}
	return "anyone"
}

type chatdIdentitySelfResponse struct {
	PubkeyHex string `json:"pubkeyHex"`
	Name      string `json:"name"`
}

func (service Service) invokeChatdPlatformMessageContext(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var botIdentity chatdIdentitySelfResponse
	if errorValue := service.chatdPlatformRequest(ctx, request.Context.Platform, "identity.self", map[string]any{}, &botIdentity); errorValue != nil {
		return mattermostToolErrorResponse(request.ToolName, mattermostToolFailureForError("bot_lookup", "platform_unavailable", errorValue)), nil
	}
	toolContext := request.Context
	result := platformMessageContextResult{
		Platform:                toolContext.Platform,
		ConversationID:          toolContext.ConversationID,
		ConversationType:        toolContext.ConversationType,
		ChannelID:               firstNonEmpty(strings.TrimSpace(toolContext.ChannelID), buzzReplyTargetChannel(toolContext.ReplyTargetID)),
		ChannelName:             toolContext.ChannelName,
		ReplyTargetID:           toolContext.ReplyTargetID,
		RootMessageID:           buzzReplyTargetRoot(toolContext.ReplyTargetID),
		RequesterPersonID:       toolContext.RequesterPersonID,
		RequesterPlatformUserID: toolContext.RequesterPlatformUserID,
		BotUserID:               botIdentity.PubkeyHex,
		BotUsername:             botIdentity.Name,
	}
	return mattermostToolSuccessResponse(request.ToolName, "ok", result)
}

func buzzReplyTargetChannel(replyTargetID string) string {
	parts := strings.Split(strings.TrimSpace(replyTargetID), ":")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func buzzReplyTargetRoot(replyTargetID string) string {
	parts := strings.Split(strings.TrimSpace(replyTargetID), ":")
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}
