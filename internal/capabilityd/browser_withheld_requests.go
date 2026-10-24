package capabilityd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	browserruntime "github.com/yeomyeonggeori/internkim/internal/browser"
	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const withheldRequestInputField = "withheldRequest"
const shownBodyLimit = 4000
const ledgerBodyLimit = 64 << 10
const keptBodiesLimit = 64

var browserActions = map[string]string{
	"browser_click":  "click",
	"browser_fill":   "fill",
	"browser_select": "select",
	"browser_press":  "press",
}

type withheldCall struct {
	Requests     []withheldRequestRecord `json:"requests"`
	IsActionDone bool                    `json:"isActionDone"`
}

type withheldRequestRecord struct {
	browserruntime.WithheldRequest
	BodyBase64 string `json:"bodyBase64,omitempty"`
	KeptBodyID string `json:"keptBodyID,omitempty"`
}

type keptBody struct {
	requester string
	body      string
}

type keptRequestBodies struct {
	mutex  sync.Mutex
	bodies map[string]keptBody
	order  []string
}

func (kept *keptRequestBodies) keep(requester string, body string) string {
	kept.mutex.Lock()
	defer kept.mutex.Unlock()
	if kept.bodies == nil {
		kept.bodies = map[string]keptBody{}
	}
	for len(kept.order) >= keptBodiesLimit {
		delete(kept.bodies, kept.order[0])
		kept.order = kept.order[1:]
	}
	identifier := newWorkID()
	kept.bodies[identifier] = keptBody{requester: requester, body: body}
	kept.order = append(kept.order, identifier)
	return identifier
}

func (kept *keptRequestBodies) take(requester string, identifier string) (string, bool) {
	kept.mutex.Lock()
	defer kept.mutex.Unlock()
	found, isKept := kept.bodies[identifier]
	if !isKept || found.requester != requester {
		return "", false
	}
	delete(kept.bodies, identifier)
	return found.body, true
}

type pageRequestGuard interface {
	IsGuarding(context.Context, string) bool
	Settle(context.Context)
	TakeWithheld() browserruntime.GuardedRequests
	Resend(context.Context, browserruntime.WithheldRequest, string, func() error) error
	HandOver()
	TakeBack()
	CookieJarDigest(context.Context, string) (string, error)
	Close()
}

type browserRequestGuards struct {
	attach func(context.Context, string) (pageRequestGuard, error)
	mutex  sync.Mutex
	guards map[string]pageRequestGuard
}

func attachRequestGuard(ctx context.Context, devtoolsURL string) (pageRequestGuard, error) {
	guard, errorValue := browserruntime.GuardRequests(ctx, devtoolsURL)
	if errorValue != nil {
		return nil, errorValue
	}
	return guard, nil
}

func (guards *browserRequestGuards) guardFor(ctx context.Context, devtoolsURL string) (pageRequestGuard, error) {
	guards.mutex.Lock()
	defer guards.mutex.Unlock()
	if guard, isKnown := guards.guards[devtoolsURL]; isKnown {
		if guard.IsGuarding(ctx, devtoolsURL) {
			return guard, nil
		}
		delete(guards.guards, devtoolsURL)
		go guard.Close()
	}
	attach := guards.attach
	if attach == nil {
		attach = attachRequestGuard
	}
	guard, errorValue := attach(ctx, devtoolsURL)
	if errorValue != nil {
		return nil, errorValue
	}
	if guards.guards == nil {
		guards.guards = map[string]pageRequestGuard{}
	}
	guards.guards[devtoolsURL] = guard
	return guard, nil
}

func (service Service) runGuardedBrowserCall(ctx context.Context, browserRuntime browserruntime.AgentBrowserRuntime, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	guard, errorValue := service.browserRequestGuards.guardFor(ctx, browserRuntime.CDPURL)
	if errorValue != nil {
		log.Printf("tool.browser.request_guard_unavailable: tool=%s: %v", request.ToolName, errorValue)
		return deviceBrowserFailedResponse(request.ToolName, "the device browser could not hold its pages' requests for the requester's approval, so this call was not done: "+errorValue.Error()), nil
	}
	if withheldRequestOf(request.Input) != "" {
		return service.resendWithheldRequests(ctx, browserRuntime, guard, request)
	}
	if earlier := takeWithheld(guard, request.ToolName); len(earlier.Held) > 0 {
		return service.withheldRequestsResponse(request, earlier, false), nil
	}
	response, actionError := service.runBrowserAction(ctx, browserRuntime, request)
	guard.Settle(ctx)
	if caused := takeWithheld(guard, request.ToolName); len(caused.Held) > 0 {
		_, isAction := browserActions[request.ToolName]
		return service.withheldRequestsResponse(request, caused, isAction), nil
	}
	return response, actionError
}

func takeWithheld(guard pageRequestGuard, toolName string) browserruntime.GuardedRequests {
	guarded := guard.TakeWithheld()
	if guarded.WithheldBeacons > 0 {
		log.Printf("tool.browser.beacons_withheld: tool=%s count=%d", toolName, guarded.WithheldBeacons)
	}
	return guarded
}

func withheldRequestOf(input json.RawMessage) string {
	var document struct {
		WithheldRequest string `json:"withheldRequest"`
	}
	if json.Unmarshal(input, &document) != nil {
		return ""
	}
	return strings.TrimSpace(document.WithheldRequest)
}

func (service Service) withheldRequestsResponse(request capabilities.ToolInvokeRequest, guarded browserruntime.GuardedRequests, isActionDone bool) capabilities.ToolInvokeResponse {
	toolName := request.ToolName
	titles := withheldRequestTitles(guarded.Held)
	message := fmt.Sprintf("This call made the page send %s, which would carry this browser's cookies for that site. Nothing was sent: it goes out only if the requester approves exactly this.", titles)
	if !isActionDone {
		message = fmt.Sprintf("Since the last browser call, the page tried to send %s on its own, which would carry this browser's cookies for that site. Nothing was sent, and %s was not run: the request goes out only if the requester approves exactly this, and %s runs after it.", titles, toolName, toolName)
	}
	log.Printf("tool.browser.requests_withheld: tool=%s requests=%s", toolName, titles)
	keepBody := func(body string) string { return service.keptRequestBodies.keep(requesterKeyOf(request.Context), body) }
	target := approvalTargetOf(guarded.Held, isActionDone, keepBody)
	result, _ := json.Marshal(struct {
		ErrorCode      string                      `json:"errorCode"`
		FailureStage   string                      `json:"failureStage"`
		Message        string                      `json:"message"`
		ApprovalTarget capabilities.ApprovalTarget `json:"approvalTarget"`
	}{ErrorCode: "approval_required", FailureStage: "authorization", Message: message, ApprovalTarget: target})
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

func approvalTargetOf(held []browserruntime.HeldRequest, isActionDone bool, keepBody func(string) string) capabilities.ApprovalTarget {
	call := withheldCall{IsActionDone: isActionDone}
	blocks := make([]string, 0, len(held))
	for _, request := range held {
		call.Requests = append(call.Requests, recordOf(request, keepBody))
		blocks = append(blocks, shownRequestBlock(request))
	}
	identity, _ := json.Marshal(call)
	return capabilities.ApprovalTarget{
		InputField: withheldRequestInputField,
		ID:         string(identity),
		Title:      withheldRequestTitles(held),
		Preview:    strings.Join(blocks, "\n\n"),
	}
}

func recordOf(held browserruntime.HeldRequest, keepBody func(string) string) withheldRequestRecord {
	record := withheldRequestRecord{WithheldRequest: held.Request}
	if len(held.Body) > ledgerBodyLimit {
		record.KeptBodyID = keepBody(held.Body)
		return record
	}
	record.BodyBase64 = base64.StdEncoding.EncodeToString([]byte(held.Body))
	return record
}

func requesterKeyOf(toolContext capabilities.ToolInvokeContext) string {
	return strings.ToLower(strings.TrimSpace(toolContext.RequesterEmail))
}

func withheldRequestTitles(held []browserruntime.HeldRequest) string {
	titles := make([]string, 0, len(held))
	for _, request := range held {
		titles = append(titles, request.Request.Method+" "+originOf(request.Request.URL))
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

func shownRequestBlock(held browserruntime.HeldRequest) string {
	lines := append([]string{held.Request.Method + " " + held.Request.URL}, shownBodyLines(held)...)
	return strings.Join(lines, "\n")
}

func shownBodyLines(held browserruntime.HeldRequest) []string {
	if held.Request.IsBodyMissing {
		return []string{"(the browser did not report what this request carries)"}
	}
	if held.Body == "" {
		return nil
	}
	if fields := browserruntime.FormFields(browserruntime.ContentTypeOf(held.Request.Headers), held.Body); len(fields) > 0 {
		return formFieldLines(fields)
	}
	return []string{shownRawBody(held.Body)}
}

func formFieldLines(fields []browserruntime.FormField) []string {
	lines := []string{}
	for _, field := range fields {
		if field.Filename != "" {
			lines = append(lines, fmt.Sprintf("%s: (file %s, %d bytes)", field.Name, field.Filename, len(field.Value)))
			continue
		}
		lines = append(lines, field.Name+": "+shownRawBody(field.Value))
	}
	return lines
}

func shownRawBody(body string) string {
	if len(body) <= shownBodyLimit {
		return body
	}
	return body[:shownBodyLimit] + fmt.Sprintf("\n(%d more bytes not shown)", len(body)-shownBodyLimit)
}

func (service Service) refusalOfAnUnapprovedResend(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, bool) {
	if withheldRequestOf(request.Input) == "" {
		return capabilities.ToolInvokeResponse{}, false
	}
	if !isCarriedOutOnAHold(request) {
		return deviceBrowserFailedResponse(request.ToolName, withheldRequestInputField+" is set only by the requester's approval of a withheld request; call the tool without it"), true
	}
	return service.refusalUnlessTheLedgerRedeems(ctx, request)
}

func (service Service) resendWithheldRequests(ctx context.Context, browserRuntime browserruntime.AgentBrowserRuntime, guard pageRequestGuard, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var call withheldCall
	if errorValue := json.Unmarshal([]byte(withheldRequestOf(request.Input)), &call); errorValue != nil || len(call.Requests) == 0 {
		return deviceBrowserFailedResponse(request.ToolName, "the approved withheld request is not one this browser recorded; nothing was sent"), nil
	}
	bodies, errorValue := service.approvedBodies(requesterKeyOf(request.Context), call.Requests)
	if errorValue != nil {
		return deviceBrowserFailedResponse(request.ToolName, errorValue.Error()), nil
	}
	sent := []string{}
	for index, withheld := range call.Requests {
		navigate := func() error {
			_, navigateError := browserRuntime.Navigate(ctx, browserruntime.NavigateRequest{URL: withheld.URL})
			return navigateError
		}
		if errorValue := guard.Resend(ctx, withheld.WithheldRequest, bodies[index], navigate); errorValue != nil {
			log.Printf("tool.browser.resend_failed: request=%s %s: %v", withheld.Method, withheld.URL, errorValue)
			return deviceBrowserFailedResponse(request.ToolName, fmt.Sprintf("the approved %s %s was not sent: %v; sent before it: %s", withheld.Method, withheld.URL, errorValue, strings.Join(sent, ", "))), nil
		}
		sent = append(sent, withheld.Method+" "+withheld.URL)
	}
	log.Printf("tool.browser.requests_resent: tool=%s requests=%s", request.ToolName, strings.Join(sent, ", "))
	sentNote := "The requester approved, and the browser sent: " + strings.Join(sent, ", ") + "."
	if call.IsActionDone {
		return resentActionResponse(request, sentNote)
	}
	return service.callAfterTheApprovedRequests(ctx, browserRuntime, request, sentNote)
}

func (service Service) approvedBodies(requester string, requests []withheldRequestRecord) ([]string, error) {
	bodies := make([]string, 0, len(requests))
	for _, withheld := range requests {
		if withheld.IsBodyMissing {
			return nil, fmt.Errorf("the approved %s %s was not sent: the browser never reported what it carries", withheld.Method, withheld.URL)
		}
		if withheld.KeptBodyID == "" {
			decoded, errorValue := base64.StdEncoding.DecodeString(withheld.BodyBase64)
			if errorValue != nil {
				return nil, fmt.Errorf("the approved %s %s carries a body this browser cannot read; nothing was sent", withheld.Method, withheld.URL)
			}
			bodies = append(bodies, string(decoded))
			continue
		}
		body, isKept := service.keptRequestBodies.take(requester, withheld.KeptBodyID)
		if !isKept {
			return nil, fmt.Errorf("the approved %s %s carried a body too large for the ledger, which only this browser service's memory kept, and the service has restarted since or sent it already; nothing was sent, so repeat the action to ask the requester again", withheld.Method, withheld.URL)
		}
		bodies = append(bodies, body)
	}
	return bodies, nil
}

func (service Service) callAfterTheApprovedRequests(ctx context.Context, browserRuntime browserruntime.AgentBrowserRuntime, request capabilities.ToolInvokeRequest, sentNote string) (capabilities.ToolInvokeResponse, error) {
	var input map[string]json.RawMessage
	if errorValue := json.Unmarshal(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	delete(input, withheldRequestInputField)
	request.Input, _ = json.Marshal(input)
	response, errorValue := service.runGuardedBrowserCall(ctx, browserRuntime, request)
	if errorValue != nil || response.IsError {
		return response, errorValue
	}
	response.Content = strings.TrimSpace(sentNote + "\n\n" + firstNonEmpty(response.Content, string(response.Result)))
	return response, nil
}

func resentActionResponse(request capabilities.ToolInvokeRequest, sentNote string) (capabilities.ToolInvokeResponse, error) {
	if _, isAction := browserActions[request.ToolName]; !isAction {
		return capabilities.ToolInvokeResponse{}, errors.New("an approved request withheld after " + request.ToolName + " ran has no action to report")
	}
	var input struct {
		Selector string `json:"selector"`
		Ref      string `json:"ref"`
		Target   string `json:"target"`
	}
	_ = json.Unmarshal(request.Input, &input)
	target, _ := browserruntime.ActionTarget(input.Selector, input.Ref, input.Target)
	document, errorValue := json.Marshal(browserruntime.ActionResult{
		OK:         true,
		Action:     browserActions[request.ToolName],
		Target:     target,
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponseFrom(request.ToolName, "ok", document, capabilityResponseOrigin{
		Provider:        "device",
		SelectedBackend: capabilities.LLMBackendDevice,
		Content:         sentNote + " Read the page to see what the site answered.",
	})
}
