package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
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
	person, failure, isRefused := service.resolvePersonHint(ctx, input.DeliveryTarget.PersonHint, request.Context.ResponseLanguage)
	if isRefused {
		return platformDMErrorResponse(request.ToolName, failure), nil
	}
	messageID, failure, hasFailure := service.postChatdDirectMessage(ctx, request, person, input.Message)
	if hasFailure {
		return platformDMErrorResponse(request.ToolName, failure), nil
	}
	return platformToolSuccessResponse(request.ToolName, "sent", platformMessageSendResult{
		MessageIDs:     []string{messageID},
		DeliveryStatus: "sent",
	})
}

func (service Service) invokeChatdMessageToTheAgent(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageSendInput) (capabilities.ToolInvokeResponse, error) {
	senderEmail := strings.TrimSpace(request.Context.RequesterEmail)
	messageID, failure, hasFailure := service.sendChatdDirectMessageAsRequester(ctx, senderEmail, "", input.Message)
	if hasFailure {
		return platformDMErrorResponse(request.ToolName, failure), nil
	}
	return platformToolSuccessResponse(request.ToolName, "sent", platformMessageSendResult{
		MessageIDs:     []string{messageID},
		DeliveryStatus: "sent",
	})
}

func (service Service) invokeChatdDirectMessageBroadcast(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageSendInput) (capabilities.ToolInvokeResponse, error) {
	language := request.Context.ResponseLanguage
	people, failure, isRefused := service.resolvePersonHints(ctx, input.DeliveryTarget.PersonHints, language)
	if isRefused {
		return platformDMErrorResponse(request.ToolName, failure), nil
	}
	messageIDs := []string{}
	failures := []platformMessageFailureResult{}
	for _, person := range people {
		messageID, failure, hasFailure := service.postChatdDirectMessage(ctx, request, person, input.Message)
		if hasFailure {
			shown := shownPerson(person, language)
			failures = append(failures, platformMessageFailureResult{PersonID: shown.PersonID, Name: shown.Name, ErrorCode: failure.ErrorCode, Message: failure.Message})
			continue
		}
		if !slices.Contains(messageIDs, messageID) {
			messageIDs = append(messageIDs, messageID)
		}
	}
	if len(messageIDs) == 0 {
		return everyDeliveryFailedResponse(request.ToolName, failures), nil
	}
	return platformToolSuccessResponse(request.ToolName, "sent", platformMessageSendResult{
		MessageIDs:     messageIDs,
		DeliveryStatus: "sent",
		Failures:       failures,
	})
}

func everyDeliveryFailedResponse(toolName string, failures []platformMessageFailureResult) capabilities.ToolInvokeResponse {
	response := platformDMErrorResponse(toolName, platformDMStaticFailure("broadcast_all_failed", "message_send", "every recipient delivery failed; see failures"))
	response.Result, _ = json.Marshal(map[string]any{"failures": failures})
	response.Content = string(response.Result)
	return response
}

// The runtime speaks for itself when it tells somebody about a decision the
// company made; a person asking for a message to be passed on is somebody
// else's words, and they go out under that person's name.
func (service Service) postChatdDirectMessage(ctx context.Context, request capabilities.ToolInvokeRequest, recipient directoryPerson, message string) (string, platformDMFailure, bool) {
	if speaksAsTheRequester(request.Context) {
		return service.sendChatdDirectMessageAsRequester(ctx, strings.TrimSpace(request.Context.RequesterEmail), recipient.Email, message)
	}
	pubkeyHex, errorValue := service.directoryBuzzKey(ctx, recipient.Email)
	if errorValue != nil {
		return "", platformDMUnavailableFailure(errorValue), true
	}
	var response chatdDirectMessagePostResponse
	postRequest := map[string]any{"counterpartPubkeyHex": pubkeyHex, "message": strings.TrimSpace(message)}
	if errorValue := service.chatdRequest(ctx, "dm.post", postRequest, &response); errorValue != nil {
		return "", platformDMFailureForError("message_send", "operation_failed", errorValue, false), true
	}
	if strings.TrimSpace(response.MessageID) == "" {
		return "", platformDMStaticFailure("post_create_failed", "message_send", "chatd did not return a message ID"), true
	}
	return response.MessageID, platformDMFailure{}, false
}

func speaksAsTheRequester(toolContext capabilities.ToolInvokeContext) bool {
	return strings.TrimSpace(toolContext.RequesterEmail) != "" && strings.TrimSpace(toolContext.TaskSource) != capabilities.TaskSourcePlaneTelling
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
func (service Service) sendChatdDirectMessageAsRequester(ctx context.Context, senderEmail string, recipientEmail string, message string) (string, platformDMFailure, bool) {
	requestBody, errorValue := json.Marshal(map[string]string{
		"senderEmail":    senderEmail,
		"recipientEmail": recipientEmail,
		"message":        strings.TrimSpace(message),
	})
	if errorValue != nil {
		return "", platformDMUnavailableFailure(errorValue), true
	}
	endpoint := strings.TrimRight(service.Configuration.AdmindBaseURL, "/") + "/admin/api/directory/direct-message"
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if errorValue != nil {
		return "", platformDMUnavailableFailure(errorValue), true
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return "", platformDMUnavailableFailure(errorValue), true
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		failure := platformDMStaticFailure("sender_not_addressable", "message_send",
			"this message was to go out as "+senderEmail+" and could not: "+strings.TrimSpace(string(body)))
		return "", failure, true
	}
	var sent chatdDirectMessagePostResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&sent); errorValue != nil {
		return "", platformDMUnavailableFailure(errorValue), true
	}
	if strings.TrimSpace(sent.MessageID) == "" {
		return "", platformDMStaticFailure("post_create_failed", "message_send", "the company directory did not return a message ID"), true
	}
	return sent.MessageID, platformDMFailure{}, false
}
