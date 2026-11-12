package capabilityd

import (
	"context"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

type chatdMessageSearchRequest struct {
	ReplyTargetID      string   `json:"replyTargetID,omitempty"`
	ChannelID          string   `json:"channelID,omitempty"`
	ChannelName        string   `json:"channelName,omitempty"`
	RootMessageID      string   `json:"rootMessageID,omitempty"`
	MessageIDs         []string `json:"messageIDs,omitempty"`
	AuthoredBy         string   `json:"authoredBy,omitempty"`
	RequesterPubkeyHex string   `json:"requesterPubkeyHex,omitempty"`
	ReadAsRequester    bool     `json:"readAsRequester,omitempty"`
	Queries            []string `json:"queries"`
	Limit              int      `json:"limit,omitempty"`
}

type chatdMessageSearchCandidate struct {
	MessageID           string  `json:"messageID"`
	ChannelID           string  `json:"channelID"`
	RootMessageID       string  `json:"rootMessageID,omitempty"`
	AuthorPubkeyHex     string  `json:"authorPubkeyHex"`
	AuthoredByAssistant bool    `json:"authoredByAssistant"`
	Editable            bool    `json:"editable"`
	Deletable           bool    `json:"deletable"`
	CreatedAt           int64   `json:"createdAt"`
	Text                string  `json:"text"`
	Score               float64 `json:"score"`
}

type chatdMessageSearchResponse struct {
	ChannelID  string                        `json:"channelID"`
	Candidates []chatdMessageSearchCandidate `json:"candidates"`
}

func (service Service) invokeChatdPlatformMessageSearch(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageSearchInput) (capabilities.ToolInvokeResponse, error) {
	input, question, isAsking := service.withSearchedChannelResolved(ctx, request, input)
	if isAsking {
		return question, nil
	}
	input, refusal, isRefused := service.withRequesterKnown(ctx, request, input)
	if isRefused {
		return refusal, nil
	}
	input, refusal, isRefused = service.withDirectConversationResolved(ctx, request, input)
	if isRefused {
		return refusal, nil
	}
	searchRequest, scope, failure, hasFailure := chatdMessageSearchRequestFromInput(request.Context, input)
	if hasFailure {
		return platformToolErrorResponse(request.ToolName, failure), nil
	}
	var response chatdMessageSearchResponse
	if errorValue := service.chatdRequest(ctx, "message.search", searchRequest, &response); errorValue != nil {
		failure := platformToolStaticFailure("message_search_failed", "platform_search", errorValue.Error())
		return platformToolErrorResponse(request.ToolName, failure), nil
	}
	response = service.withMessagesOnlyTheRequesterReads(ctx, request, searchRequest, response)
	result := canonicalChatdMessageSearchResult(response, input, scope, searchRequest.RequesterPubkeyHex)
	return platformToolSuccessResponse(request.ToolName, "ok", result)
}

func (service Service) withRequesterKnown(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageSearchInput) (platformMessageSearchInput, capabilities.ToolInvokeResponse, bool) {
	input.RequesterPubkeyHex = strings.TrimSpace(request.Context.RequesterPlatformUserID)
	if input.RequesterPubkeyHex != "" || !readsAsTheRequester(request.Context, input) {
		return input, capabilities.ToolInvokeResponse{}, false
	}
	requesterPubkeyHex, refusal, isRefused := service.requesterMessengerKey(ctx, request)
	input.RequesterPubkeyHex = requesterPubkeyHex
	return input, refusal, isRefused
}

func readsAsTheRequester(toolContext capabilities.ToolInvokeContext, input platformMessageSearchInput) bool {
	return input.AuthoredBy == "requester" || normalizedChatdMessageSearchScope(input.Scope, toolContext, input) == "directMessage"
}

func (service Service) withDirectConversationResolved(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageSearchInput) (platformMessageSearchInput, capabilities.ToolInvokeResponse, bool) {
	if normalizedChatdMessageSearchScope(input.Scope, request.Context, input) != "directMessage" || len(input.MessageIDs) > 0 {
		return input, capabilities.ToolInvokeResponse{}, false
	}
	if input.DeliveryTarget.PersonHint != "" && !isAskedByTheRequesterThemself(request.Context) {
		failure := platformToolStaticFailure("requester_only", "authorization",
			"a direct conversation between two people is read only by one of them, calling the API as themself; the agent reads its own direct conversation with the requester, with personHint left out")
		return input, platformToolErrorResponse(request.ToolName, failure), true
	}
	counterpart, refusal, isRefused := service.directMessageCounterpart(ctx, request, input.DeliveryTarget.PersonHint)
	if isRefused {
		return input, refusal, true
	}
	conversationID, failure, hasFailure := service.directConversationWith(ctx, input.RequesterPubkeyHex, counterpart)
	if hasFailure {
		return input, platformToolErrorResponse(request.ToolName, failure), true
	}
	input.DeliveryTarget.ChannelID = conversationID
	return input, capabilities.ToolInvokeResponse{}, false
}

func isAskedByTheRequesterThemself(toolContext capabilities.ToolInvokeContext) bool {
	return strings.TrimSpace(toolContext.TaskSource) == capabilities.TaskSourcePublicAPI
}

func (service Service) directMessageCounterpart(ctx context.Context, request capabilities.ToolInvokeRequest, personHint string) (messengerPerson, capabilities.ToolInvokeResponse, bool) {
	if personHint != "" {
		counterparts, refusal, isRefused := service.messengerPeople(ctx, request, []string{personHint})
		if isRefused {
			return messengerPerson{}, refusal, true
		}
		return counterparts[0], capabilities.ToolInvokeResponse{}, false
	}
	var agent chatdIdentitySelfResponse
	if errorValue := service.chatdRequest(ctx, "identity.self", map[string]any{}, &agent); errorValue != nil {
		return messengerPerson{}, platformToolErrorResponse(request.ToolName, platformToolFailureForError("bot_lookup", "platform_unavailable", errorValue)), true
	}
	return messengerPerson{PersonHint: agent.Name, PubkeyHex: agent.PubkeyHex}, capabilities.ToolInvokeResponse{}, false
}

func (service Service) withMessagesOnlyTheRequesterReads(ctx context.Context, request capabilities.ToolInvokeRequest, searchRequest chatdMessageSearchRequest, response chatdMessageSearchResponse) chatdMessageSearchResponse {
	unread := messageIDsNotAmong(searchRequest.MessageIDs, response.Candidates)
	if len(unread) == 0 || searchRequest.ReadAsRequester || !isAskedByTheRequesterThemself(request.Context) {
		return response
	}
	if searchRequest.RequesterPubkeyHex == "" && strings.TrimSpace(request.Context.RequesterEmail) == "" {
		return response
	}
	if searchRequest.RequesterPubkeyHex == "" {
		requesterPubkeyHex, _, isRefused := service.requesterMessengerKey(ctx, request)
		if isRefused {
			return response
		}
		searchRequest.RequesterPubkeyHex = requesterPubkeyHex
	}
	searchRequest.MessageIDs = unread
	searchRequest.ReadAsRequester = true
	var readAsRequester chatdMessageSearchResponse
	if service.chatdRequest(ctx, "message.search", searchRequest, &readAsRequester) != nil {
		return response
	}
	response.Candidates = append(response.Candidates, readAsRequester.Candidates...)
	return response
}

func messageIDsNotAmong(messageIDs []string, candidates []chatdMessageSearchCandidate) []string {
	read := map[string]bool{}
	for _, candidate := range candidates {
		read[candidate.MessageID] = true
	}
	unread := []string{}
	for _, messageID := range messageIDs {
		if !read[messageID] {
			unread = append(unread, messageID)
		}
	}
	return unread
}

func (service Service) withSearchedChannelResolved(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageSearchInput) (platformMessageSearchInput, capabilities.ToolInvokeResponse, bool) {
	channelName := input.DeliveryTarget.ChannelName
	if normalizedChatdMessageSearchScope(input.Scope, request.Context, input) != "channel" || input.DeliveryTarget.ChannelID != "" || channelName == "" {
		return input, capabilities.ToolInvokeResponse{}, false
	}
	channels, errorValue := service.chatdChannels(ctx)
	if errorValue != nil {
		return input, capabilities.ToolInvokeResponse{}, false
	}
	resolution := resolveHint(channelName, channels, nil)
	if resolution.Outcome != hintResolved {
		return input, platformDMErrorResponse(request.ToolName, messageChannelQuestion(channelName, resolution)), true
	}
	input.DeliveryTarget.ChannelID = strings.TrimSpace(resolution.Match.ChannelID)
	input.DeliveryTarget.ChannelName = ""
	return input, capabilities.ToolInvokeResponse{}, false
}

func chatdMessageSearchRequestFromInput(toolContext capabilities.ToolInvokeContext, input platformMessageSearchInput) (chatdMessageSearchRequest, string, platformToolFailure, bool) {
	searchRequest := chatdMessageSearchRequest{
		MessageIDs:         input.MessageIDs,
		AuthoredBy:         input.AuthoredBy,
		RequesterPubkeyHex: input.RequesterPubkeyHex,
		Queries:            input.Queries,
		Limit:              input.Limit,
	}
	scope := normalizedChatdMessageSearchScope(input.Scope, toolContext, input)
	searchRequest.ReadAsRequester = scope == "directMessage" && isAskedByTheRequesterThemself(toolContext)
	if len(input.MessageIDs) > 0 {
		return searchRequest, scope, platformToolFailure{}, false
	}
	switch scope {
	case "directMessage":
		searchRequest.ChannelID = input.DeliveryTarget.ChannelID
	case "channel":
		if input.DeliveryTarget.ChannelID == "" && input.DeliveryTarget.ChannelName == "" {
			failure := platformToolStaticFailure("invalid_input", "input_decode", "scope=channel needs channelID or channelName")
			return chatdMessageSearchRequest{}, scope, failure, true
		}
		searchRequest.ChannelID = input.DeliveryTarget.ChannelID
		searchRequest.ChannelName = input.DeliveryTarget.ChannelName
	case "currentThread":
		rootMessageID := buzzReplyTargetRoot(toolContext.ReplyTargetID)
		if rootMessageID == "" {
			failure := platformToolStaticFailure("thread_not_available", "context", "current conversation does not have a thread root")
			return chatdMessageSearchRequest{}, scope, failure, true
		}
		searchRequest.ReplyTargetID = toolContext.ReplyTargetID
		searchRequest.RootMessageID = rootMessageID
	default:
		searchRequest.ReplyTargetID = toolContext.ReplyTargetID
		searchRequest.ChannelID = strings.TrimSpace(toolContext.ChannelID)
		if searchRequest.ReplyTargetID == "" && searchRequest.ChannelID == "" {
			failure := platformToolStaticFailure("channel_not_available", "context",
				"this request did not arrive on a conversation; name a channelName or channelID, or search a direct conversation with scope=directMessage")
			return chatdMessageSearchRequest{}, scope, failure, true
		}
	}
	return searchRequest, scope, platformToolFailure{}, false
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
			Editable:      candidate.Editable,
			Deletable:     candidate.Deletable,
		}
		if isReadByID {
			mapped.Text = candidate.Text
		} else {
			mapped.Preview = platformMessageSearchPreview(candidate.Text, input.Queries)
		}
		if mapped.Deletable {
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
	if !hasArrivedOnAConversation(request.Context) {
		failure := platformToolStaticFailure("conversation_not_available", "context",
			"this request did not arrive on a conversation, so there is none to return; name a channelName, channelID or person in the tool that needs one")
		return platformToolErrorResponse(request.ToolName, failure), nil
	}
	var botIdentity chatdIdentitySelfResponse
	if errorValue := service.chatdRequest(ctx, "identity.self", map[string]any{}, &botIdentity); errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolFailureForError("bot_lookup", "platform_unavailable", errorValue)), nil
	}
	toolContext := request.Context
	result := platformMessageContextResult{
		Platform:                service.companyMessenger(),
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
	return platformToolSuccessResponse(request.ToolName, "ok", result)
}

func hasArrivedOnAConversation(toolContext capabilities.ToolInvokeContext) bool {
	return strings.TrimSpace(toolContext.ConversationID) != "" || strings.TrimSpace(toolContext.ReplyTargetID) != "" || strings.TrimSpace(toolContext.ChannelID) != ""
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
