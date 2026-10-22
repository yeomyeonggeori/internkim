package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const requestGuardSetupTimeout = 5 * time.Second
const requestGuardAnswerTimeout = 5 * time.Second
const beaconResourceType = "Ping"
const documentResourceType = "Document"

// RFC 9110 section 9.2.1 names these the safe methods.
var safeRequestMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodOptions: true,
	http.MethodTrace:   true,
}

type WithheldRequest struct {
	Method        string            `json:"method"`
	URL           string            `json:"url"`
	Headers       map[string]string `json:"headers,omitempty"`
	Body          string            `json:"body,omitempty"`
	IsBodyMissing bool              `json:"isBodyMissing,omitempty"`
	ResourceType  string            `json:"resourceType"`
}

type GuardedRequests struct {
	Withheld        []WithheldRequest
	WithheldBeacons int
	IsResent        bool
}

type RequestGuard struct {
	connection *devtoolsConnection
	resend     *WithheldRequest
	mutex      sync.Mutex
	watched    map[string]bool
	attachment error
	outcome    GuardedRequests
	processed  atomic.Int64
	handled    chan struct{}
}

type attachedTarget struct {
	SessionID  string `json:"sessionId"`
	TargetInfo struct {
		TargetID string `json:"targetId"`
		Type     string `json:"type"`
	} `json:"targetInfo"`
}

type pausedRequest struct {
	RequestID    string `json:"requestId"`
	ResourceType string `json:"resourceType"`
	Request      struct {
		URL         string            `json:"url"`
		Method      string            `json:"method"`
		Headers     map[string]string `json:"headers"`
		PostData    string            `json:"postData"`
		HasPostData bool              `json:"hasPostData"`
	} `json:"request"`
}

type devtoolsHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func GuardRequests(ctx context.Context, devtoolsURL string) (*RequestGuard, error) {
	return startRequestGuard(ctx, devtoolsURL, nil)
}

func GuardResending(ctx context.Context, devtoolsURL string, request WithheldRequest) (*RequestGuard, error) {
	return startRequestGuard(ctx, devtoolsURL, &request)
}

func isSafeRequestMethod(method string) bool {
	return safeRequestMethods[strings.ToUpper(strings.TrimSpace(method))]
}

func startRequestGuard(ctx context.Context, devtoolsURL string, resend *WithheldRequest) (*RequestGuard, error) {
	setupContext, cancel := context.WithTimeout(ctx, requestGuardSetupTimeout)
	defer cancel()
	connection, errorValue := dialDevtools(setupContext, devtoolsURL)
	if errorValue != nil {
		return nil, errorValue
	}
	guard := &RequestGuard{connection: connection, resend: resend, watched: map[string]bool{}, handled: make(chan struct{})}
	go guard.handleEvents()
	if errorValue := guard.watchEveryPage(setupContext); errorValue != nil {
		guard.Release()
		return nil, errorValue
	}
	return guard, nil
}

func (guard *RequestGuard) watchEveryPage(ctx context.Context) error {
	if _, errorValue := guard.connection.call(ctx, "", "Target.setAutoAttach", map[string]any{"autoAttach": true, "waitForDebuggerOnStart": true, "flatten": true}); errorValue != nil {
		return errorValue
	}
	pageTargetIDs, errorValue := guard.pageTargetIDs(ctx)
	if errorValue != nil {
		return errorValue
	}
	for !guard.isWatchingAll(pageTargetIDs) {
		if guard.attachmentError() != nil {
			return guard.attachmentError()
		}
		select {
		case <-ctx.Done():
			return errors.New("the device browser's pages could not all be watched for requests before the action")
		case <-time.After(10 * time.Millisecond):
		}
	}
	return nil
}

func (guard *RequestGuard) pageTargetIDs(ctx context.Context) ([]string, error) {
	answer, errorValue := guard.connection.call(ctx, "", "Target.getTargets", map[string]any{})
	if errorValue != nil {
		return nil, errorValue
	}
	var targets struct {
		TargetInfos []struct {
			TargetID string `json:"targetId"`
			Type     string `json:"type"`
		} `json:"targetInfos"`
	}
	if errorValue := json.Unmarshal(answer, &targets); errorValue != nil {
		return nil, errors.New("the device browser listed its pages in a shape it never uses")
	}
	pageTargetIDs := []string{}
	for _, target := range targets.TargetInfos {
		if target.Type == "page" {
			pageTargetIDs = append(pageTargetIDs, target.TargetID)
		}
	}
	return pageTargetIDs, nil
}

func (guard *RequestGuard) isWatchingAll(targetIDs []string) bool {
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	for _, targetID := range targetIDs {
		if !guard.watched[targetID] {
			return false
		}
	}
	return true
}

func (guard *RequestGuard) attachmentError() error {
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	return guard.attachment
}

func (guard *RequestGuard) handleEvents() {
	defer close(guard.handled)
	for message := range guard.connection.events {
		switch message.Method {
		case "Target.attachedToTarget":
			guard.watchAttachedTarget(message.Params)
		case "Fetch.requestPaused":
			guard.answerPausedRequest(message.SessionID, message.Params)
		}
		guard.processed.Add(1)
	}
}

func (guard *RequestGuard) watchAttachedTarget(parameters json.RawMessage) {
	var target attachedTarget
	if json.Unmarshal(parameters, &target) != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestGuardAnswerTimeout)
	defer cancel()
	_, fetchError := guard.connection.call(ctx, target.SessionID, "Fetch.enable", map[string]any{"patterns": []map[string]string{{"urlPattern": "*", "requestStage": "Request"}}})
	_, _ = guard.connection.call(ctx, target.SessionID, "Target.setAutoAttach", map[string]any{"autoAttach": true, "waitForDebuggerOnStart": true, "flatten": true})
	_, _ = guard.connection.call(ctx, target.SessionID, "Runtime.runIfWaitingForDebugger", map[string]any{})
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	if fetchError != nil && target.TargetInfo.Type == "page" {
		guard.attachment = errors.New("the device browser cannot hold a page's requests: " + fetchError.Error())
		return
	}
	guard.watched[target.TargetInfo.TargetID] = fetchError == nil
}

func (guard *RequestGuard) answerPausedRequest(sessionID string, parameters json.RawMessage) {
	var paused pausedRequest
	if json.Unmarshal(parameters, &paused) != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestGuardAnswerTimeout)
	defer cancel()
	if guard.claimsResend(paused) {
		_, _ = guard.connection.call(ctx, sessionID, "Fetch.continueRequest", resentRequestParameters(paused.RequestID, *guard.resend))
		return
	}
	if isSafeRequestMethod(paused.Request.Method) {
		_, _ = guard.connection.call(ctx, sessionID, "Fetch.continueRequest", map[string]any{"requestId": paused.RequestID})
		return
	}
	_, _ = guard.connection.call(ctx, sessionID, "Fetch.failRequest", map[string]any{"requestId": paused.RequestID, "errorReason": "BlockedByClient"})
	guard.recordWithheld(paused)
}

func (guard *RequestGuard) claimsResend(paused pausedRequest) bool {
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	if guard.resend == nil || guard.outcome.IsResent {
		return false
	}
	isTheNavigation := paused.ResourceType == documentResourceType && paused.Request.Method == http.MethodGet && paused.Request.URL == guard.resend.URL
	guard.outcome.IsResent = isTheNavigation
	return isTheNavigation
}

func resentRequestParameters(requestID string, request WithheldRequest) map[string]any {
	headers := []devtoolsHeader{}
	for name, value := range request.Headers {
		if !strings.EqualFold(name, "Cookie") {
			headers = append(headers, devtoolsHeader{Name: name, Value: value})
		}
	}
	return map[string]any{
		"requestId": requestID,
		"method":    request.Method,
		"postData":  base64.StdEncoding.EncodeToString([]byte(request.Body)),
		"headers":   headers,
	}
}

func (guard *RequestGuard) recordWithheld(paused pausedRequest) {
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	if paused.ResourceType == beaconResourceType {
		guard.outcome.WithheldBeacons++
		return
	}
	guard.outcome.Withheld = append(guard.outcome.Withheld, WithheldRequest{
		Method:        strings.ToUpper(paused.Request.Method),
		URL:           paused.Request.URL,
		Headers:       paused.Request.Headers,
		Body:          paused.Request.PostData,
		IsBodyMissing: paused.Request.HasPostData && paused.Request.PostData == "",
		ResourceType:  paused.ResourceType,
	})
}

func (guard *RequestGuard) Release() GuardedRequests {
	guard.answerEverythingAlreadyPaused()
	guard.connection.close()
	<-guard.handled
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	return guard.outcome
}

func (guard *RequestGuard) answerEverythingAlreadyPaused() {
	ctx, cancel := context.WithTimeout(context.Background(), requestGuardAnswerTimeout)
	defer cancel()
	if _, errorValue := guard.connection.call(ctx, "", "Target.getTargets", map[string]any{}); errorValue != nil {
		return
	}
	received := guard.connection.received.Load()
	for guard.processed.Load() < received {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Millisecond):
		}
	}
}
