package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type browserHandoffInput struct {
	Message string `json:"message"`
	URL     string `json:"url"`
}

type relayHandoffRequest struct {
	Message     string                 `json:"message"`
	Requester   relayHandoffRequester  `json:"requester"`
	Addressing  relayHandoffAddressing `json:"addressing"`
	DevtoolsURL string                 `json:"devtoolsURL"`
}

type relayHandoffRequester struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type relayHandoffAddressing struct {
	Platform         string `json:"platform"`
	ConversationID   string `json:"conversationID"`
	ConversationType string `json:"conversationType,omitempty"`
	ReplyTargetID    string `json:"replyTargetID,omitempty"`
	ResponseLanguage string `json:"responseLanguage,omitempty"`
}

type browserHandoffResult struct {
	HandoffID string      `json:"handoffID"`
	Status    string      `json:"status"`
	OpenURL   string      `json:"openURL"`
	ExpiresAt string      `json:"expiresAt"`
	Page      handoffPage `json:"page"`
}

type handoffPage struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
	Text  string `json:"text,omitempty"`
}

const handoffPageTextLimit = 1500

func (service Service) invokeBrowserHandoffTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input browserHandoffInput
	if errorValue := decodeBrowserToolInput(request.Input, &input); errorValue != nil {
		return capabilityInvalidInputResponse(request.ToolName, errorValue), nil
	}
	handoffRequest, hasConversation := relayHandoffRequestOf(request.Context, input.Message)
	if !hasConversation {
		return deviceBrowserFailedResponse(request.ToolName, "a browser handoff is handed to the person who asked in a conversation, and this call came from none; ask them in a conversation instead"), nil
	}
	browserRuntime, errorValue := service.deviceBrowserRuntime(ctx, request.Context)
	if errors.Is(errorValue, browserruntime.ErrDeviceBrowsersFull) {
		return service.deviceBrowsersFullResponse(request.ToolName), nil
	}
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	page, errorValue := pageToHandOver(ctx, browserRuntime, strings.TrimSpace(input.URL))
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if !isWebPageAddress(page.URL) {
		return deviceBrowserFailedResponse(request.ToolName, "the device browser has no web page open, so there is nothing to hand over; call browser_handoff again with the url the requester should start from"), nil
	}
	handoffRequest.DevtoolsURL = browserRuntime.CDPURL
	result, errorValue := service.beginRelayHandoff(ctx, handoffRequest)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result.Page = page
	service.holdBrowserUntilTheHandoffExpires(request.Context.RequesterEmail, result.ExpiresAt)
	document, errorValue := json.Marshal(result)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponseFrom(request.ToolName, "ok", document, capabilityResponseOrigin{
		Provider:        "device",
		SelectedBackend: capabilities.LLMBackendDevice,
	})
}

func pageToHandOver(ctx context.Context, browserRuntime browserruntime.AgentBrowserRuntime, address string) (handoffPage, error) {
	page, errorValue := pageShownAfterOpening(ctx, browserRuntime, address)
	if errorValue != nil || !isWebPageAddress(page.URL) {
		return page, errorValue
	}
	text, errorValue := browserRuntime.VisibleText(ctx)
	if errorValue != nil {
		return handoffPage{}, errorValue
	}
	page.Text = excerptOf(text, handoffPageTextLimit)
	return page, nil
}

func pageShownAfterOpening(ctx context.Context, browserRuntime browserruntime.AgentBrowserRuntime, address string) (handoffPage, error) {
	if address == "" {
		observation, errorValue := browserRuntime.Observe(ctx, browserruntime.ObserveRequest{})
		return handoffPage{URL: observation.URL, Title: observation.Title}, errorValue
	}
	navigation, errorValue := browserRuntime.Navigate(ctx, browserruntime.NavigateRequest{URL: address})
	return handoffPage{URL: navigation.URL, Title: navigation.Title}, errorValue
}

func isWebPageAddress(address string) bool {
	return strings.HasPrefix(address, "https://") || strings.HasPrefix(address, "http://")
}

func excerptOf(text string, limit int) string {
	characters := []rune(strings.TrimSpace(text))
	if len(characters) <= limit {
		return string(characters)
	}
	return string(characters[:limit]) + "…"
}

func relayHandoffRequestOf(requestContext capabilities.ToolInvokeContext, message string) (relayHandoffRequest, bool) {
	email := strings.ToLower(strings.TrimSpace(requestContext.RequesterEmail))
	platform := strings.TrimSpace(requestContext.Platform)
	conversationID := strings.TrimSpace(requestContext.ConversationID)
	if email == "" || platform == "" || conversationID == "" {
		return relayHandoffRequest{}, false
	}
	return relayHandoffRequest{
		Message:   strings.TrimSpace(message),
		Requester: relayHandoffRequester{Email: email, Name: strings.TrimSpace(requestContext.RequesterName)},
		Addressing: relayHandoffAddressing{
			Platform:         platform,
			ConversationID:   conversationID,
			ConversationType: strings.TrimSpace(requestContext.ConversationType),
			ReplyTargetID:    strings.TrimSpace(requestContext.ReplyTargetID),
			ResponseLanguage: strings.TrimSpace(requestContext.ResponseLanguage),
		},
	}, true
}

func (service Service) beginRelayHandoff(ctx context.Context, handoffRequest relayHandoffRequest) (browserHandoffResult, error) {
	body, errorValue := json.Marshal(handoffRequest)
	if errorValue != nil {
		return browserHandoffResult{}, errorValue
	}
	address := strings.TrimRight(service.Configuration.WithDefaults().RelayBaseURL, "/") + "/browser-handoffs"
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(body))
	if errorValue != nil {
		return browserHandoffResult{}, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return browserHandoffResult{}, fmt.Errorf("the relay at %s did not take the browser handoff: %w", address, errorValue)
	}
	defer response.Body.Close()
	answer, errorValue := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if errorValue != nil {
		return browserHandoffResult{}, errorValue
	}
	if response.StatusCode != http.StatusCreated {
		return browserHandoffResult{}, fmt.Errorf("the relay at %s answered %d to the browser handoff: %s", address, response.StatusCode, strings.TrimSpace(string(answer)))
	}
	var result browserHandoffResult
	if errorValue := json.Unmarshal(answer, &result); errorValue != nil {
		return browserHandoffResult{}, fmt.Errorf("the relay answered the browser handoff with something other than a handoff: %w", errorValue)
	}
	result.Status = "waiting"
	return result, nil
}

func (service Service) holdBrowserUntilTheHandoffExpires(requesterEmail string, expiresAt string) {
	expiry, errorValue := time.Parse(time.RFC3339Nano, expiresAt)
	if errorValue != nil {
		log.Printf("the relay's browser handoff expiry %q is not a time, so the requester's browser is not held: %v", expiresAt, errorValue)
		return
	}
	service.DeviceBrowsers.HoldUntil(requesterEmail, expiry)
}
