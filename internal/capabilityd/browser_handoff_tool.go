package capabilityd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	browserruntime "github.com/yeomyeonggeori/internkim/internal/browser"
	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const browserHandoffToolName = "browser_handoff"
const handoffInputField = "handoff"
const browserHandoffLifetime = 15 * time.Minute
const handoffWatchInterval = 2 * time.Second

type browserHandoffInput struct {
	Step    string `json:"step"`
	Handoff string `json:"handoff"`
}

type browserHandoffResult struct {
	Ended                string `json:"ended"`
	Origin               string `json:"origin"`
	SignInOrigin         string `json:"signInOrigin"`
	SignInCookiesChanged *bool  `json:"signInCookiesChanged,omitempty"`
}

func (service Service) invokeBrowserHandoffTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input browserHandoffInput
	if errorValue := decodeBrowserToolInput(request.Input, &input); errorValue != nil {
		return capabilityInvalidInputResponse(request.ToolName, errorValue), nil
	}
	if handoffID := strings.TrimSpace(input.Handoff); handoffID != "" {
		return service.reportTheHandedBackBrowser(ctx, request, handoffID)
	}
	return service.handTheBrowserOver(ctx, request)
}

func (service Service) handTheBrowserOver(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	requester := requesterKeyOf(request.Context)
	if requester == "" || strings.TrimSpace(request.Context.ConversationID) == "" {
		return deviceBrowserFailedResponse(request.ToolName, "a browser is handed only to a person who asked in a conversation, and this call came from none, so nothing was handed over"), nil
	}
	if service.browserHandoffs.isHandedOver(requester) {
		return service.handedOverResponse(request.ToolName, requester), nil
	}
	browserRuntime, errorValue := service.deviceBrowserRuntime(ctx, request.Context)
	if errors.Is(errorValue, browserruntime.ErrDeviceBrowsersFull) {
		return service.deviceBrowsersFullResponse(request.ToolName), nil
	}
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	guard, errorValue := service.browserRequestGuards.guardFor(ctx, browserRuntime.CDPURL)
	if errorValue != nil {
		return deviceBrowserFailedResponse(request.ToolName, "the device browser could not be prepared for the hand-over: "+errorValue.Error()), nil
	}
	handoff, refusal := service.handoffOfTheShownPage(ctx, browserRuntime, guard, requester)
	if refusal != "" {
		return deviceBrowserFailedResponse(request.ToolName, refusal), nil
	}
	if errorValue := service.registerHandoffWithTheRelay(ctx, handoff, browserRuntime.CDPURL); errorValue != nil {
		return deviceBrowserFailedResponse(request.ToolName, errorValue.Error()+"; nothing was handed over"), nil
	}
	service.beginHandoff(handoff, guard)
	return handoffRequiredResponse(request.ToolName, handoff), nil
}

func (service Service) handoffOfTheShownPage(ctx context.Context, browserRuntime browserruntime.AgentBrowserRuntime, guard pageRequestGuard, requester string) (openHandoff, string) {
	shownURL, errorValue := browserRuntime.CurrentURL(ctx)
	if errorValue != nil {
		return openHandoff{}, "the device browser could not say which page it shows: " + errorValue.Error()
	}
	if !isWebPageAddress(shownURL) {
		return openHandoff{}, "the device browser shows no web page, so there is nothing to hand over; open the page that needs the requester first"
	}
	digest, digestError := guard.CookieJarDigest(ctx, shownURL)
	if digestError != nil {
		log.Printf("tool.browser.handoff_cookies_unread: %v; whether the sign-in changed them will not be known", digestError)
	}
	return openHandoff{
		handoffID:    newWorkID(),
		requester:    requester,
		signInOrigin: originOf(shownURL),
		cookieDigest: digest,
		expiresAt:    time.Now().Add(browserHandoffLifetime),
		guard:        guard,
	}, ""
}

func (service Service) beginHandoff(handoff openHandoff, guard pageRequestGuard) {
	dropped := guard.TakeWithheld()
	if len(dropped.Held) > 0 {
		log.Printf("tool.browser.handoff_dropped_withheld: requests=%s; the requester drives the page now", withheldRequestTitles(dropped.Held))
	}
	service.handedElsewhere.forget(handoff.requester)
	service.browserHandoffs.open(handoff)
	if service.DeviceBrowsers != nil {
		service.DeviceBrowsers.HoldUntil(handoff.requester, handoff.expiresAt)
	}
	log.Printf("tool.browser.handed_over: handoff=%s origin=%s expiresAt=%s", handoff.handoffID, handoff.signInOrigin, handoff.expiresAt.UTC().Format(time.RFC3339))
	go service.watchUntilTheHandoffEnds(handoff)
}

func (service Service) watchUntilTheHandoffEnds(handoff openHandoff) {
	ticker := time.NewTicker(handoffWatchInterval)
	defer ticker.Stop()
	for range ticker.C {
		if _, isOpen := service.browserHandoffs.openFor(handoff.requester); !isOpen {
			return
		}
		if service.hasTheHandoffEnded(handoff) {
			service.endHandoff(handoff)
			return
		}
	}
}

func (service Service) hasTheHandoffEnded(handoff openHandoff) bool {
	if time.Now().After(handoff.expiresAt.Add(handoffWatchInterval)) {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), handoffWatchInterval)
	defer cancel()
	status, errorValue := service.handoffStatusAtTheRelay(ctx, handoff.handoffID)
	if errorValue != nil {
		log.Printf("tool.browser.handoff_status_unread: handoff=%s: %v", handoff.handoffID, errorValue)
		return false
	}
	return status.hasEnded()
}

func (service Service) endHandoff(handoff openHandoff) {
	if _, isEnded := service.browserHandoffs.end(handoff.requester, handoff.handoffID); !isEnded {
		return
	}
	if service.DeviceBrowsers != nil {
		service.DeviceBrowsers.Release(handoff.requester)
	}
	log.Printf("tool.browser.handed_back: handoff=%s", handoff.handoffID)
}

func (service Service) reportTheHandedBackBrowser(ctx context.Context, request capabilities.ToolInvokeRequest, handoffID string) (capabilities.ToolInvokeResponse, error) {
	if !isCarriedOutOnAHold(request) {
		return deviceBrowserFailedResponse(request.ToolName, handoffInputField+" is set only by the runtime once the requester takes the browser; call browser_handoff without it"), nil
	}
	if refusal, isRefused := service.refusalUnlessTheLedgerRedeems(ctx, request); isRefused {
		return refusal, nil
	}
	status, errorValue := service.handoffStatusAtTheRelay(ctx, handoffID)
	if errorValue != nil {
		return deviceBrowserFailedResponse(request.ToolName, errorValue.Error()), nil
	}
	requester := requesterKeyOf(request.Context)
	if open, isOpen := service.browserHandoffs.openFor(requester); isOpen && open.handoffID == handoffID {
		service.endHandoff(open)
	}
	handoff, isKnown := service.browserHandoffs.takeEnded(requester, handoffID)
	browserRuntime, errorValue := service.deviceBrowserRuntime(ctx, request.Context)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := service.handedBackResult(ctx, browserRuntime, status, handoff, isKnown)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	document, errorValue := json.Marshal(result)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponseFrom(request.ToolName, "ok", document, capabilityResponseOrigin{
		Provider:        "device",
		SelectedBackend: capabilities.LLMBackendDevice,
	})
}

func (service Service) handedBackResult(ctx context.Context, browserRuntime browserruntime.AgentBrowserRuntime, status relayHandoffStatus, handoff openHandoff, isKnown bool) (browserHandoffResult, error) {
	shownURL, errorValue := browserRuntime.CurrentURL(ctx)
	if errorValue != nil {
		return browserHandoffResult{}, errorValue
	}
	result := browserHandoffResult{
		Ended:        status.endedAs(),
		Origin:       originOf(shownURL),
		SignInOrigin: firstNonEmpty(handoff.signInOrigin, status.SignInOrigin),
	}
	if !isKnown || handoff.cookieDigest == "" {
		return result, nil
	}
	guard, errorValue := service.browserRequestGuards.guardFor(ctx, browserRuntime.CDPURL)
	if errorValue != nil {
		return result, nil
	}
	digest, errorValue := guard.CookieJarDigest(ctx, result.SignInOrigin)
	if errorValue != nil {
		return result, nil
	}
	isChanged := digest != handoff.cookieDigest
	result.SignInCookiesChanged = &isChanged
	return result, nil
}

func (service Service) handedOverResponse(toolName string, requester string) capabilities.ToolInvokeResponse {
	handoff, _ := service.browserHandoffs.openFor(requester)
	return deviceBrowserFailedResponse(toolName, fmt.Sprintf(
		"the requester has this browser in their own hands at %s until they hand it back or the hand-over ends at %s, so nothing was done in it; wait for the hand-over to end",
		handoff.signInOrigin, handoff.expiresAt.UTC().Format(time.RFC3339)))
}

func handoffRequiredResponse(toolName string, handoff openHandoff) capabilities.ToolInvokeResponse {
	message := "The browser at " + handoff.signInOrigin + " is ready for the requester to take. It is handed to them when they are asked, and the call answers once they hand it back."
	result, _ := json.Marshal(struct {
		ErrorCode      string                      `json:"errorCode"`
		FailureStage   string                      `json:"failureStage"`
		Message        string                      `json:"message"`
		ApprovalTarget capabilities.ApprovalTarget `json:"approvalTarget"`
	}{ErrorCode: "approval_required", FailureStage: "authorization", Message: message, ApprovalTarget: capabilities.ApprovalTarget{
		InputField: handoffInputField,
		ID:         handoff.handoffID,
		Title:      handoff.signInOrigin,
		HandoffID:  handoff.handoffID,
	}})
	return capabilities.ToolInvokeResponse{
		Provider:        "device",
		SelectedBackend: capabilities.LLMBackendDevice,
		ToolName:        strings.TrimSpace(toolName),
		Outcome:         capabilities.ToolOutcomeDenied,
		Status:          "denied",
		Content:         message,
		IsError:         true,
		Message:         message,
		ErrorCode:       "approval_required",
		FailureStage:    "authorization",
		Result:          result,
	}
}

func isWebPageAddress(address string) bool {
	return strings.HasPrefix(address, "https://") || strings.HasPrefix(address, "http://")
}
