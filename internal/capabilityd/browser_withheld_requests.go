package capabilityd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/url"
	"strings"
	"sync"
	"time"

	browserruntime "github.com/yeomyeonggeori/internkim/internal/browser"
	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const withheldRequestInputField = "withheldRequest"
const browserActionSettle = time.Second
const withheldRequestLifetime = 24 * time.Hour
const shownBodyLimit = 4000

var guardedBrowserActions = map[string]string{
	"browser_click":  "click",
	"browser_fill":   "fill",
	"browser_select": "select",
	"browser_press":  "press",
}

type withheldBrowserCall struct {
	ID         string                           `json:"withheldRequestID"`
	Requests   []browserruntime.WithheldRequest `json:"withheldRequests"`
	requester  string
	toolName   string
	input      string
	withheldAt time.Time
}

type withheldBrowserRequests struct {
	mutex sync.Mutex
	calls map[string]withheldBrowserCall
}

func (store *withheldBrowserRequests) keep(call withheldBrowserCall) {
	if store == nil {
		return
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.calls == nil {
		store.calls = map[string]withheldBrowserCall{}
	}
	for identifier, kept := range store.calls {
		if time.Since(kept.withheldAt) > withheldRequestLifetime {
			delete(store.calls, identifier)
		}
	}
	store.calls[call.ID] = call
}

func (store *withheldBrowserRequests) latestFor(requester string, toolName string, input string) (withheldBrowserCall, bool) {
	if store == nil {
		return withheldBrowserCall{}, false
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	latest, isFound := withheldBrowserCall{}, false
	for _, call := range store.calls {
		if call.requester != requester || call.toolName != toolName || call.input != input {
			continue
		}
		if !isFound || call.withheldAt.After(latest.withheldAt) {
			latest, isFound = call, true
		}
	}
	return latest, isFound
}

func (store *withheldBrowserRequests) take(identifier string, requester string) (withheldBrowserCall, bool) {
	if store == nil {
		return withheldBrowserCall{}, false
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	call, isFound := store.calls[identifier]
	if !isFound || call.requester != requester {
		return withheldBrowserCall{}, false
	}
	delete(store.calls, identifier)
	return call, true
}

func (service Service) runGuardedBrowserAction(ctx context.Context, browserRuntime browserruntime.AgentBrowserRuntime, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	guard, errorValue := browserruntime.GuardRequests(ctx, browserRuntime.CDPURL)
	if errorValue != nil {
		log.Printf("tool.browser.request_guard_unavailable: tool=%s: %v", request.ToolName, errorValue)
		return deviceBrowserFailedResponse(request.ToolName, "the device browser could not hold this action's requests for the requester's approval, so the action was not done: "+errorValue.Error()), nil
	}
	response, actionError := service.runBrowserAction(ctx, browserRuntime, request)
	waitForTheActionToSettle(ctx)
	guarded := guard.Release()
	if guarded.WithheldBeacons > 0 {
		log.Printf("tool.browser.beacons_withheld: tool=%s count=%d", request.ToolName, guarded.WithheldBeacons)
	}
	if len(guarded.Withheld) == 0 {
		return response, actionError
	}
	call := withheldBrowserCall{
		ID:         newWorkID(),
		Requests:   guarded.Withheld,
		requester:  requesterKeyOf(request.Context),
		toolName:   request.ToolName,
		input:      canonicalBrowserActionInput(request.Input),
		withheldAt: time.Now(),
	}
	service.withheldBrowserRequests.keep(call)
	log.Printf("tool.browser.requests_withheld: tool=%s id=%s requests=%s", request.ToolName, call.ID, withheldRequestTitles(call.Requests))
	return withheldRequestsResponse(request.ToolName, call), nil
}

func waitForTheActionToSettle(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-time.After(browserActionSettle):
	}
}

func requesterKeyOf(toolContext capabilities.ToolInvokeContext) string {
	return strings.ToLower(strings.TrimSpace(toolContext.RequesterEmail))
}

func canonicalBrowserActionInput(input json.RawMessage) string {
	document := map[string]any{}
	if json.Unmarshal(input, &document) != nil {
		return strings.TrimSpace(string(input))
	}
	delete(document, withheldRequestInputField)
	canonical, _ := json.Marshal(document)
	return string(canonical)
}

func withheldRequestIDOf(input json.RawMessage) string {
	var document struct {
		WithheldRequest string `json:"withheldRequest"`
	}
	if json.Unmarshal(input, &document) != nil {
		return ""
	}
	return strings.TrimSpace(document.WithheldRequest)
}

func withheldRequestsResponse(toolName string, call withheldBrowserCall) capabilities.ToolInvokeResponse {
	message := fmt.Sprintf("This action made the page send %s, which changes data. Nothing was sent: it goes out only if the requester approves exactly this.", withheldRequestTitles(call.Requests))
	result, _ := json.Marshal(struct {
		ErrorCode    string `json:"errorCode"`
		FailureStage string `json:"failureStage"`
		Message      string `json:"message"`
		withheldBrowserCall
	}{ErrorCode: "approval_required", FailureStage: "authorization", Message: message, withheldBrowserCall: shownCall(call)})
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

func shownCall(call withheldBrowserCall) withheldBrowserCall {
	shown := withheldBrowserCall{ID: call.ID}
	for _, request := range call.Requests {
		request.Headers = shownHeaders(request.Headers)
		shown.Requests = append(shown.Requests, request)
	}
	return shown
}

func shownHeaders(headers map[string]string) map[string]string {
	for name, value := range headers {
		if strings.EqualFold(name, "Content-Type") {
			return map[string]string{"Content-Type": value}
		}
	}
	return nil
}

func (service Service) keepWithheldCallOfAnswer(request capabilities.ToolInvokeRequest, answer capabilities.ToolInvokeResponse) {
	if answer.ErrorCode != "approval_required" {
		return
	}
	var call withheldBrowserCall
	if json.Unmarshal(answer.Result, &call) != nil || call.ID == "" || len(call.Requests) == 0 {
		return
	}
	call.requester = requesterKeyOf(request.Context)
	call.toolName = request.ToolName
	call.input = canonicalBrowserActionInput(request.Input)
	call.withheldAt = time.Now()
	service.withheldBrowserRequests.keep(call)
}

func (service Service) resolveWithheldBrowserRequests(_ context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	call, isFound := service.withheldBrowserRequests.latestFor(requesterKeyOf(request.Context), request.ToolName, canonicalBrowserActionInput(request.Input))
	if !isFound {
		return deviceBrowserFailedResponse(request.ToolName, "this browser holds no request withheld from this call, so there is nothing to approve; nothing was sent"), nil
	}
	return capabilityToolTargetResponse(request.ToolName, approvalTargetOf(call)), nil
}

func approvalTargetOf(call withheldBrowserCall) capabilities.ApprovalTarget {
	blocks := make([]string, 0, len(call.Requests))
	for _, request := range call.Requests {
		blocks = append(blocks, shownRequestBlock(request))
	}
	return capabilities.ApprovalTarget{
		InputField: withheldRequestInputField,
		ID:         call.ID,
		Title:      withheldRequestTitles(call.Requests),
		Preview:    strings.Join(blocks, "\n\n"),
	}
}

func withheldRequestTitles(requests []browserruntime.WithheldRequest) string {
	titles := make([]string, 0, len(requests))
	for _, request := range requests {
		titles = append(titles, request.Method+" "+originOf(request.URL))
	}
	return strings.Join(titles, ", ")
}

func originOf(address string) string {
	parsed, errorValue := url.Parse(address)
	if errorValue != nil || parsed.Host == "" {
		return address
	}
	return parsed.Scheme + "://" + parsed.Host
}

func shownRequestBlock(request browserruntime.WithheldRequest) string {
	lines := append([]string{request.Method + " " + request.URL}, shownBodyLines(request)...)
	return strings.Join(lines, "\n")
}

func shownBodyLines(request browserruntime.WithheldRequest) []string {
	if request.IsBodyMissing {
		return []string{"(the browser did not report what this request carries)"}
	}
	if request.Body == "" {
		return nil
	}
	if isFormEncoded(shownHeaders(request.Headers)["Content-Type"]) {
		return formFieldLines(request.Body)
	}
	return []string{shownRawBody(request.Body)}
}

func isFormEncoded(contentType string) bool {
	mediaType, _, errorValue := mime.ParseMediaType(contentType)
	return errorValue == nil && mediaType == "application/x-www-form-urlencoded"
}

func formFieldLines(body string) []string {
	lines := []string{}
	for _, field := range strings.Split(body, "&") {
		name, value, _ := strings.Cut(field, "=")
		lines = append(lines, formComponent(name)+": "+formComponent(value))
	}
	return lines
}

func formComponent(component string) string {
	decoded, errorValue := url.QueryUnescape(component)
	if errorValue != nil {
		return component
	}
	return decoded
}

func shownRawBody(body string) string {
	if len(body) <= shownBodyLimit {
		return body
	}
	return body[:shownBodyLimit] + fmt.Sprintf("\n(%d more bytes not shown)", len(body)-shownBodyLimit)
}

func (service Service) refusalOfAnUnapprovedResend(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, bool) {
	if withheldRequestIDOf(request.Input) == "" {
		return capabilities.ToolInvokeResponse{}, false
	}
	if !isCarriedOutOnAHold(request) {
		return deviceBrowserFailedResponse(request.ToolName, withheldRequestInputField+" is set only by the requester's approval of a withheld request; call the action without it"), true
	}
	return service.refusalUnlessTheLedgerRedeems(ctx, request)
}

func (service Service) resendWithheldRequests(ctx context.Context, browserRuntime browserruntime.AgentBrowserRuntime, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	call, isHeld := service.withheldBrowserRequests.take(withheldRequestIDOf(request.Input), requesterKeyOf(request.Context))
	if !isHeld {
		return deviceBrowserFailedResponse(request.ToolName, "the approved request is no longer held by this browser (it was sent already, or the browser service restarted); nothing was sent"), nil
	}
	sent := []string{}
	for _, withheld := range call.Requests {
		if errorValue := resendWithheldRequest(ctx, browserRuntime, withheld); errorValue != nil {
			log.Printf("tool.browser.resend_failed: id=%s request=%s %s: %v", call.ID, withheld.Method, withheld.URL, errorValue)
			return deviceBrowserFailedResponse(request.ToolName, fmt.Sprintf("the approved %s %s was not sent: %v; sent before it: %s", withheld.Method, withheld.URL, errorValue, strings.Join(sent, ", "))), nil
		}
		sent = append(sent, withheld.Method+" "+withheld.URL)
	}
	log.Printf("tool.browser.requests_resent: tool=%s id=%s", request.ToolName, call.ID)
	return resentActionResponse(request, sent)
}

func resendWithheldRequest(ctx context.Context, browserRuntime browserruntime.AgentBrowserRuntime, withheld browserruntime.WithheldRequest) error {
	if withheld.IsBodyMissing {
		return errors.New("the browser never reported what it carries")
	}
	guard, errorValue := browserruntime.GuardResending(ctx, browserRuntime.CDPURL, withheld)
	if errorValue != nil {
		return errorValue
	}
	_, navigateError := browserRuntime.Navigate(ctx, browserruntime.NavigateRequest{URL: withheld.URL})
	waitForTheActionToSettle(ctx)
	outcome := guard.Release()
	if len(outcome.Withheld) > 0 {
		log.Printf("tool.browser.requests_withheld_after_resend: requests=%s", withheldRequestTitles(outcome.Withheld))
	}
	if !outcome.IsResent {
		return fmt.Errorf("the browser never made the request again (%v)", navigateError)
	}
	return nil
}

func resentActionResponse(request capabilities.ToolInvokeRequest, sent []string) (capabilities.ToolInvokeResponse, error) {
	var input struct {
		Selector string `json:"selector"`
		Ref      string `json:"ref"`
		Target   string `json:"target"`
	}
	_ = json.Unmarshal(request.Input, &input)
	target, _ := browserruntime.ActionTarget(input.Selector, input.Ref, input.Target)
	document, errorValue := json.Marshal(browserruntime.ActionResult{
		OK:         true,
		Action:     guardedBrowserActions[request.ToolName],
		Target:     target,
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponseFrom(request.ToolName, "ok", document, capabilityResponseOrigin{
		Provider:        "device",
		SelectedBackend: capabilities.LLMBackendDevice,
		Content:         "The requester approved, and the browser sent: " + strings.Join(sent, ", ") + ". Read the page to see what the site answered.",
	})
}
