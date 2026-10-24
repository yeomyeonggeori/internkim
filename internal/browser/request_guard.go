package browser

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sort"
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
	IsBodyMissing bool              `json:"isBodyMissing,omitempty"`
	ResourceType  string            `json:"resourceType"`
}

type HeldRequest struct {
	Request WithheldRequest
	Body    string
}

type GuardedRequests struct {
	Held            []HeldRequest
	WithheldBeacons int
}

type pendingResend struct {
	request   WithheldRequest
	body      string
	isClaimed bool
}

type RequestGuard struct {
	connection   *devtoolsConnection
	socketURL    string
	mutex        sync.Mutex
	watched      map[string]bool
	sessionTypes map[string]string
	attachment   error
	withheld     GuardedRequests
	resend       *pendingResend
	resending    sync.Mutex
	processed    atomic.Int64
	isHandedOver atomic.Bool
	answering    map[int64]chan struct{}
	nextAnswer   int64
	handled      chan struct{}
}

type attachedTarget struct {
	SessionID  string `json:"sessionId"`
	TargetInfo struct {
		TargetID string `json:"targetId"`
		Type     string `json:"type"`
	} `json:"targetInfo"`
}

type detachedTarget struct {
	SessionID string `json:"sessionId"`
}

type pausedRequest struct {
	RequestID    string `json:"requestId"`
	ResourceType string `json:"resourceType"`
	Request      struct {
		URL             string            `json:"url"`
		Method          string            `json:"method"`
		Headers         map[string]string `json:"headers"`
		PostData        string            `json:"postData"`
		HasPostData     bool              `json:"hasPostData"`
		PostDataEntries []struct {
			Bytes string `json:"bytes"`
		} `json:"postDataEntries"`
	} `json:"request"`
}

type devtoolsHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func GuardRequests(ctx context.Context, devtoolsURL string) (*RequestGuard, error) {
	setupContext, cancel := context.WithTimeout(ctx, requestGuardSetupTimeout)
	defer cancel()
	socketURL, errorValue := devtoolsBrowserSocketURL(setupContext, devtoolsURL)
	if errorValue != nil {
		return nil, errorValue
	}
	connection, errorValue := dialDevtoolsSocket(setupContext, socketURL)
	if errorValue != nil {
		return nil, errorValue
	}
	guard := &RequestGuard{
		connection:   connection,
		socketURL:    socketURL,
		watched:      map[string]bool{},
		sessionTypes: map[string]string{},
		answering:    map[int64]chan struct{}{},
		handled:      make(chan struct{}),
	}
	go guard.handleEvents()
	if errorValue := guard.watchEveryPage(setupContext); errorValue != nil {
		guard.Close()
		return nil, errorValue
	}
	return guard, nil
}

func (guard *RequestGuard) IsGuarding(ctx context.Context, devtoolsURL string) bool {
	select {
	case <-guard.handled:
		return false
	default:
	}
	socketURL, errorValue := devtoolsBrowserSocketURL(ctx, devtoolsURL)
	return errorValue == nil && socketURL == guard.socketURL
}

func isSafeRequestMethod(method string) bool {
	return safeRequestMethods[strings.ToUpper(strings.TrimSpace(method))]
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
			return errors.New("the device browser's pages could not all be watched for requests")
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
		case "Target.detachedFromTarget":
			guard.forgetDetachedTarget(message.Params)
		case "Fetch.requestPaused":
			go guard.answerPausedRequest(message.SessionID, message.Params, guard.startAnswering())
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
	guard.sessionTypes[target.SessionID] = target.TargetInfo.Type
}

func (guard *RequestGuard) forgetDetachedTarget(parameters json.RawMessage) {
	var target detachedTarget
	if json.Unmarshal(parameters, &target) != nil {
		return
	}
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	delete(guard.sessionTypes, target.SessionID)
}

func (guard *RequestGuard) startAnswering() func() {
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	guard.nextAnswer++
	identifier, answered := guard.nextAnswer, make(chan struct{})
	guard.answering[identifier] = answered
	return func() {
		guard.mutex.Lock()
		defer guard.mutex.Unlock()
		delete(guard.answering, identifier)
		close(answered)
	}
}

func (guard *RequestGuard) stillAnswering() []chan struct{} {
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	answering := make([]chan struct{}, 0, len(guard.answering))
	for _, answered := range guard.answering {
		answering = append(answering, answered)
	}
	return answering
}

func (guard *RequestGuard) answerPausedRequest(sessionID string, parameters json.RawMessage, finish func()) {
	defer finish()
	var paused pausedRequest
	if json.Unmarshal(parameters, &paused) != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestGuardAnswerTimeout)
	defer cancel()
	if guard.isHandedOver.Load() {
		guard.continueRequest(ctx, sessionID, paused.RequestID)
		return
	}
	if resend, isClaimed := guard.claimResend(paused); isClaimed {
		_, _ = guard.connection.call(ctx, sessionID, "Fetch.continueRequest", resentRequestParameters(paused.RequestID, resend))
		return
	}
	if isSafeRequestMethod(paused.Request.Method) {
		guard.continueRequest(ctx, sessionID, paused.RequestID)
		return
	}
	if !guard.carriesCookies(ctx, sessionID, paused.Request.URL) {
		guard.continueRequest(ctx, sessionID, paused.RequestID)
		return
	}
	_, _ = guard.connection.call(ctx, sessionID, "Fetch.failRequest", map[string]any{"requestId": paused.RequestID, "errorReason": "BlockedByClient"})
	guard.recordWithheld(paused, bodyOf(paused))
}

func (guard *RequestGuard) continueRequest(ctx context.Context, sessionID string, requestID string) {
	_, _ = guard.connection.call(ctx, sessionID, "Fetch.continueRequest", map[string]any{"requestId": requestID})
}

func bodyOf(paused pausedRequest) string {
	if paused.Request.PostData != "" || len(paused.Request.PostDataEntries) == 0 {
		return paused.Request.PostData
	}
	builder := strings.Builder{}
	for _, entry := range paused.Request.PostDataEntries {
		decoded, errorValue := base64.StdEncoding.DecodeString(entry.Bytes)
		if errorValue != nil {
			return ""
		}
		builder.Write(decoded)
	}
	return builder.String()
}

func (guard *RequestGuard) carriesCookies(ctx context.Context, sessionID string, address string) bool {
	answer, errorValue := guard.connection.call(ctx, sessionID, "Network.getCookies", map[string]any{"urls": []string{address}})
	if errorValue != nil {
		log.Printf("tool.browser.cookie_jar_unreadable: %v; the request is held as if it carried cookies", errorValue)
		return true
	}
	var jar struct {
		Cookies []json.RawMessage `json:"cookies"`
	}
	if json.Unmarshal(answer, &jar) != nil {
		return true
	}
	return len(jar.Cookies) > 0
}

func (guard *RequestGuard) HandOver() {
	guard.isHandedOver.Store(true)
}

func (guard *RequestGuard) TakeBack() {
	guard.isHandedOver.Store(false)
}

func (guard *RequestGuard) CookieJarDigest(ctx context.Context, address string) (string, error) {
	sessionID, isWatching := guard.aPageSession()
	if !isWatching {
		return "", errors.New("the device browser has no page to read its cookies through")
	}
	answer, errorValue := guard.connection.call(ctx, sessionID, "Network.getCookies", map[string]any{"urls": []string{address}})
	if errorValue != nil {
		return "", errorValue
	}
	var jar struct {
		Cookies []struct {
			Name   string `json:"name"`
			Value  string `json:"value"`
			Domain string `json:"domain"`
			Path   string `json:"path"`
		} `json:"cookies"`
	}
	if errorValue := json.Unmarshal(answer, &jar); errorValue != nil {
		return "", errors.New("the device browser listed its cookies in a shape it never uses")
	}
	lines := make([]string, 0, len(jar.Cookies))
	for _, cookie := range jar.Cookies {
		lines = append(lines, strings.Join([]string{cookie.Domain, cookie.Path, cookie.Name, cookie.Value}, "\x00"))
	}
	sort.Strings(lines)
	digest := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(digest[:]), nil
}

func (guard *RequestGuard) aPageSession() (string, bool) {
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	for sessionID, targetType := range guard.sessionTypes {
		if targetType == "page" {
			return sessionID, true
		}
	}
	return "", false
}

func (guard *RequestGuard) claimResend(paused pausedRequest) (pendingResend, bool) {
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	if guard.resend == nil || guard.resend.isClaimed {
		return pendingResend{}, false
	}
	isTheNavigation := paused.ResourceType == documentResourceType && paused.Request.Method == http.MethodGet && paused.Request.URL == guard.resend.request.URL
	guard.resend.isClaimed = isTheNavigation
	return *guard.resend, isTheNavigation
}

func resentRequestParameters(requestID string, resend pendingResend) map[string]any {
	headers := []devtoolsHeader{}
	for name, value := range resend.request.Headers {
		headers = append(headers, devtoolsHeader{Name: name, Value: value})
	}
	return map[string]any{
		"requestId": requestID,
		"method":    resend.request.Method,
		"postData":  base64.StdEncoding.EncodeToString([]byte(resend.body)),
		"headers":   headers,
	}
}

func (guard *RequestGuard) recordWithheld(paused pausedRequest, body string) {
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	if paused.ResourceType == beaconResourceType {
		guard.withheld.WithheldBeacons++
		return
	}
	request := WithheldRequest{
		Method:        strings.ToUpper(paused.Request.Method),
		URL:           paused.Request.URL,
		Headers:       headersWithoutCookies(paused.Request.Headers),
		IsBodyMissing: paused.Request.HasPostData && body == "",
		ResourceType:  paused.ResourceType,
	}
	guard.withheld.Held = append(guard.withheld.Held, HeldRequest{Request: request, Body: body})
}

func headersWithoutCookies(headers map[string]string) map[string]string {
	kept := map[string]string{}
	for name, value := range headers {
		if !strings.EqualFold(name, "Cookie") {
			kept[name] = value
		}
	}
	return kept
}

func (guard *RequestGuard) Settle(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, requestGuardAnswerTimeout)
	defer cancel()
	if _, errorValue := guard.connection.call(ctx, "", "Target.getTargets", map[string]any{}); errorValue != nil {
		return
	}
	received := guard.connection.received.Load()
	for guard.processed.Load() < received {
		select {
		case <-ctx.Done():
			return
		case <-guard.handled:
			return
		case <-time.After(5 * time.Millisecond):
		}
	}
	for _, answered := range guard.stillAnswering() {
		select {
		case <-answered:
		case <-ctx.Done():
			return
		}
	}
}

func (guard *RequestGuard) TakeWithheld() GuardedRequests {
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	taken := guard.withheld
	guard.withheld = GuardedRequests{}
	return taken
}

func (guard *RequestGuard) Resend(ctx context.Context, request WithheldRequest, body string, navigate func() error) error {
	guard.resending.Lock()
	defer guard.resending.Unlock()
	guard.mutex.Lock()
	guard.resend = &pendingResend{request: request, body: body}
	guard.mutex.Unlock()
	navigateError := navigate()
	guard.Settle(ctx)
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	isResent := guard.resend.isClaimed
	guard.resend = nil
	if !isResent {
		return errors.Join(errors.New("the browser never made the request again"), navigateError)
	}
	return nil
}

func (guard *RequestGuard) Close() {
	guard.connection.close()
	<-guard.handled
}
