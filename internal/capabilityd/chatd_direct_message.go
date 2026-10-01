package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

type chatdDirectMessagePostResponse struct {
	ChannelID string `json:"channelID"`
	MessageID string `json:"messageID"`
}

func (service Service) invokeChatdDirectMessageSend(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageSendInput) (capabilities.ToolInvokeResponse, error) {
	if len(input.DeliveryTarget.PersonHints) > 0 {
		return service.invokeChatdDirectMessageBroadcast(ctx, request, input)
	}
	result, failure, hasFailure := service.sendChatdDirectMessageToHint(ctx, request, input.DeliveryTarget.PersonHint, input.Message)
	if hasFailure {
		return platformDMErrorResponse(request.ToolName, failure), nil
	}
	return platformToolSuccessResponse(request.ToolName, "sent", platformMessageSendResult{
		MessageIDs:     []string{result.DispatchID},
		DeliveryStatus: "sent",
	})
}

func (service Service) invokeChatdDirectMessageBroadcast(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageSendInput) (capabilities.ToolInvokeResponse, error) {
	results := make([]platformMessageBroadcastResult, 0, len(input.DeliveryTarget.PersonHints))
	for _, personHint := range input.DeliveryTarget.PersonHints {
		result, failure, hasFailure := service.sendChatdDirectMessageToHint(ctx, request, personHint, input.Message)
		if hasFailure {
			results = append(results, platformMessageBroadcastResult{PersonHint: personHint, Status: "failed", ErrorCode: failure.ErrorCode, Message: failure.Message})
			continue
		}
		results = append(results, result)
	}
	messageIDs, failures := canonicalPlatformMessageBroadcastResult(results)
	rollup := map[string]any{
		"platform":    service.companyMessenger(),
		"results":     results,
		"sentCount":   len(messageIDs),
		"failedCount": len(failures),
	}
	if len(messageIDs) == 0 {
		response := platformDMErrorResponse(request.ToolName, platformDMStaticFailure("broadcast_all_failed", "message_send", "every recipient delivery failed; see results"))
		response.Result, _ = json.Marshal(rollup)
		return response, nil
	}
	return platformToolSuccessResponse(request.ToolName, "sent", platformMessageSendResult{
		MessageIDs:     messageIDs,
		DeliveryStatus: "sent",
		Failures:       failures,
	})
}

// The runtime speaks for itself when it tells somebody about a decision the
// company made; a person asking for a message to be passed on is somebody
// else's words, and they go out under that person's name.
const platformTellingTaskSource = "plane_telling"

func (service Service) sendChatdDirectMessageToHint(ctx context.Context, request capabilities.ToolInvokeRequest, personHint string, message string) (platformMessageBroadcastResult, platformDMFailure, bool) {
	named, failure, hasFailure := service.namedDirectoryPerson(ctx, personHint, request.Context.ResponseLanguage)
	if hasFailure {
		return platformMessageBroadcastResult{}, failure, true
	}
	senderEmail := strings.TrimSpace(request.Context.RequesterEmail)
	if senderEmail != "" && strings.TrimSpace(request.Context.TaskSource) != platformTellingTaskSource {
		return service.sendChatdDirectMessageAsRequester(ctx, senderEmail, named, message)
	}
	pubkeyHex, errorValue := service.directoryBuzzKey(ctx, named.Email)
	if errorValue != nil {
		return platformMessageBroadcastResult{}, platformDMUnavailableFailure(errorValue), true
	}
	var response chatdDirectMessagePostResponse
	postRequest := map[string]any{"counterpartPubkeyHex": pubkeyHex, "message": strings.TrimSpace(message)}
	if errorValue := service.chatdRequest(ctx, "dm.post", postRequest, &response); errorValue != nil {
		return platformMessageBroadcastResult{}, platformDMFailureForError("message_send", "operation_failed", errorValue, false), true
	}
	if strings.TrimSpace(response.MessageID) == "" {
		return platformMessageBroadcastResult{}, platformDMStaticFailure("post_create_failed", "message_send", "chatd did not return a message ID"), true
	}
	return platformMessageBroadcastResult{
		PersonHint:  personHint,
		PersonID:    named.MemberID,
		DisplayName: named.Name,
		DispatchID:  response.MessageID,
		Status:      "sent",
	}, platformDMFailure{}, false
}

// The company derives Buzz identity from a member's address, and only the host
// that holds the seed may do it. This asks for the public half alone.
func (service Service) directoryBuzzKey(ctx context.Context, email string) (string, error) {
	requestBody, errorValue := json.Marshal(map[string]string{"email": strings.ToLower(strings.TrimSpace(email))})
	if errorValue != nil {
		return "", errorValue
	}
	endpoint := strings.TrimRight(service.Configuration.AdmindBaseURL, "/") + "/admin/api/directory/buzz-key"
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if errorValue != nil {
		return "", errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return "", errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return "", fmt.Errorf("buzz key lookup answered %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var document struct {
		PubkeyHex string `json:"pubkeyHex"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&document); errorValue != nil {
		return "", errorValue
	}
	if strings.TrimSpace(document.PubkeyHex) == "" {
		return "", fmt.Errorf("buzz key lookup returned no key")
	}
	return document.PubkeyHex, nil
}

// admind holds the seed, so it is the only process that can act as a member.
// It is handed two addresses and answers with what it sent; no key crosses into
// this one. A sender the company cannot key fails here rather than going out
// under the agent's name, because a message nobody agreed to sign is worse than
// one that did not arrive.
func (service Service) sendChatdDirectMessageAsRequester(ctx context.Context, senderEmail string, named directoryPerson, message string) (platformMessageBroadcastResult, platformDMFailure, bool) {
	requestBody, errorValue := json.Marshal(map[string]string{
		"senderEmail":    senderEmail,
		"recipientEmail": named.Email,
		"message":        strings.TrimSpace(message),
	})
	if errorValue != nil {
		return platformMessageBroadcastResult{}, platformDMUnavailableFailure(errorValue), true
	}
	endpoint := strings.TrimRight(service.Configuration.AdmindBaseURL, "/") + "/admin/api/directory/direct-message"
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if errorValue != nil {
		return platformMessageBroadcastResult{}, platformDMUnavailableFailure(errorValue), true
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return platformMessageBroadcastResult{}, platformDMUnavailableFailure(errorValue), true
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		failure := platformDMStaticFailure("sender_not_addressable", "message_send",
			"this message was to go out as "+senderEmail+" and could not: "+strings.TrimSpace(string(body)))
		return platformMessageBroadcastResult{}, failure, true
	}
	var sent chatdDirectMessagePostResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&sent); errorValue != nil {
		return platformMessageBroadcastResult{}, platformDMUnavailableFailure(errorValue), true
	}
	if strings.TrimSpace(sent.MessageID) == "" {
		return platformMessageBroadcastResult{}, platformDMStaticFailure("post_create_failed", "message_send", "the company directory did not return a message ID"), true
	}
	return platformMessageBroadcastResult{
		PersonID:    named.MemberID,
		DisplayName: named.Name,
		DispatchID:  sent.MessageID,
		Status:      "sent",
	}, platformDMFailure{}, false
}
