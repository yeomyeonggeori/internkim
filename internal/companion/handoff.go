package companion

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

const (
	HandoffStateIdle           = "idle"
	HandoffStateWaitingForUser = "waiting_for_user"
	HandoffStateValidating     = "validating"
	HandoffStateCompleted      = "completed"
	HandoffStateDenied         = "denied"
	HandoffStateTimedOut       = "timed_out"

	defaultHandoffTimeoutSeconds = 600
	maxHandoffTimeoutSeconds     = 900
	maxHandoffValidationAttempts = 5
)

type BrowserHandoffRequest struct {
	URL             string                 `json:"url"`
	Message         string                 `json:"message"`
	SessionID       string                 `json:"sessionID,omitempty"`
	TimeoutSeconds  int                    `json:"timeoutSeconds,omitempty"`
	SuccessCriteria HandoffSuccessCriteria `json:"successCriteria,omitempty"`
}

func randomHandoffID(size int) string {
	document := make([]byte, size)
	if _, errorValue := rand.Read(document); errorValue != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(document)
}

type HandoffSuccessCriteria struct {
	URLIncludesAny  []string `json:"urlIncludesAny,omitempty"`
	URLExcludesAny  []string `json:"urlExcludesAny,omitempty"`
	TextIncludesAny []string `json:"textIncludesAny,omitempty"`
	TextExcludesAny []string `json:"textExcludesAny,omitempty"`
}

type BrowserHandoffResult struct {
	SessionID       string   `json:"sessionID"`
	URL             string   `json:"url,omitempty"`
	Origin          string   `json:"origin,omitempty"`
	Title           string   `json:"title,omitempty"`
	SnapshotText    string   `json:"snapshotText"`
	InteractiveRefs []string `json:"interactiveRefs"`
	State           string   `json:"state"`
	CompletedByUser bool     `json:"completedByUser"`
	CapturedAt      string   `json:"capturedAt"`
}

type HandoffSnapshot struct {
	Active    bool   `json:"active"`
	HandoffID string `json:"handoffID,omitempty"`
	SessionID string `json:"sessionID,omitempty"`
	State     string `json:"state,omitempty"`
	Message   string `json:"message,omitempty"`
	Origin    string `json:"origin,omitempty"`
}

type HandoffCompletion struct {
	HandoffID string `json:"handoffID"`
	SessionID string `json:"sessionID,omitempty"`
	URL       string `json:"url,omitempty"`
	Title     string `json:"title,omitempty"`
}

type BrowserHandoffStore struct {
	mutex  sync.Mutex
	active *browserHandoff
}

type browserHandoff struct {
	handoffID string
	sessionID string
	state     string
	message   string
	origin    string
	waiter    chan HandoffCompletion
}

func NewBrowserHandoffStore() *BrowserHandoffStore {
	return &BrowserHandoffStore{}
}

func (store *BrowserHandoffStore) Begin(request BrowserHandoffRequest, sessionID string) (HandoffSnapshot, error) {
	if store == nil {
		return HandoffSnapshot{}, errors.New("browser handoff store is unavailable")
	}
	origin, errorValue := webOrigin(request.URL)
	if errorValue != nil {
		return HandoffSnapshot{}, errorValue
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.active != nil {
		return HandoffSnapshot{}, errors.New("browser handoff is already active")
	}
	handoff := &browserHandoff{
		handoffID: randomHandoffID(16),
		sessionID: firstNonEmpty(sessionID, request.SessionID, "internkim"),
		state:     HandoffStateWaitingForUser,
		message:   firstNonEmpty(request.Message, "브라우저에서 필요한 작업을 마친 뒤 완료를 눌러주세요."),
		origin:    origin,
		waiter:    make(chan HandoffCompletion, 1),
	}
	store.active = handoff
	return handoff.snapshot(), nil
}

func (store *BrowserHandoffStore) Snapshot() HandoffSnapshot {
	if store == nil {
		return HandoffSnapshot{Active: false, State: HandoffStateIdle}
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.active == nil {
		return HandoffSnapshot{Active: false, State: HandoffStateIdle}
	}
	return store.active.snapshot()
}

func (store *BrowserHandoffStore) UpdateMessage(handoffID string, state string, message string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	handoff, errorValue := store.requireActive(handoffID)
	if errorValue != nil {
		return errorValue
	}
	handoff.state = state
	handoff.message = firstNonEmpty(message, handoff.message)
	return nil
}

func (store *BrowserHandoffStore) Complete(completion HandoffCompletion) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	handoff, errorValue := store.requireActive(completion.HandoffID)
	if errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(completion.SessionID) != "" && completion.SessionID != handoff.sessionID {
		return errors.New("browser handoff session mismatch")
	}
	origin, errorValue := webOrigin(completion.URL)
	if errorValue != nil {
		return errors.New("browser handoff completion url is invalid")
	}
	if origin != handoff.origin {
		return errors.New("browser handoff origin mismatch")
	}
	select {
	case handoff.waiter <- completion:
		return nil
	default:
		return errors.New("browser handoff completion is already pending")
	}
}

func (store *BrowserHandoffStore) Wait(ctx context.Context, handoffID string) (HandoffCompletion, error) {
	store.mutex.Lock()
	handoff, errorValue := store.requireActive(handoffID)
	if errorValue != nil {
		store.mutex.Unlock()
		return HandoffCompletion{}, errorValue
	}
	waiter := handoff.waiter
	store.mutex.Unlock()
	select {
	case completion := <-waiter:
		return completion, nil
	case <-ctx.Done():
		return HandoffCompletion{}, ctx.Err()
	}
}

func (store *BrowserHandoffStore) End(handoffID string, state string) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.active == nil || store.active.handoffID != handoffID {
		return
	}
	store.active.state = state
	store.active = nil
}

func (store *BrowserHandoffStore) requireActive(handoffID string) (*browserHandoff, error) {
	if store.active == nil {
		return nil, errors.New("browser handoff is not active")
	}
	if strings.TrimSpace(handoffID) == "" || store.active.handoffID != handoffID {
		return nil, errors.New("browser handoff id mismatch")
	}
	return store.active, nil
}

func (handoff *browserHandoff) snapshot() HandoffSnapshot {
	return HandoffSnapshot{
		Active:    true,
		HandoffID: handoff.handoffID,
		SessionID: handoff.sessionID,
		State:     handoff.state,
		Message:   handoff.message,
		Origin:    handoff.origin,
	}
}

func NormalizeHandoffTimeoutSeconds(value int) int {
	if value <= 0 {
		return defaultHandoffTimeoutSeconds
	}
	if value > maxHandoffTimeoutSeconds {
		return maxHandoffTimeoutSeconds
	}
	return value
}

func HandoffCriteriaSatisfied(criteria HandoffSuccessCriteria, pageURL string, text string) bool {
	normalizedURL := strings.ToLower(strings.TrimSpace(pageURL))
	normalizedText := strings.ToLower(text)
	if !containsAnyIfNeeded(normalizedURL, criteria.URLIncludesAny) {
		return false
	}
	if containsAny(normalizedURL, criteria.URLExcludesAny) {
		return false
	}
	if !containsAnyIfNeeded(normalizedText, criteria.TextIncludesAny) {
		return false
	}
	return !containsAny(normalizedText, criteria.TextExcludesAny)
}

func HandoffCriteriaEmpty(criteria HandoffSuccessCriteria) bool {
	return len(criteria.URLIncludesAny) == 0 &&
		len(criteria.URLExcludesAny) == 0 &&
		len(criteria.TextIncludesAny) == 0 &&
		len(criteria.TextExcludesAny) == 0
}

func handoffDenial(envelope JobEnvelope, request capabilities.ToolInvokeRequest, code string, constraint string) DenialError {
	return DenialError{Denial: capabilities.DenialResult{
		Status:              "denied",
		Code:                code,
		JobID:               envelope.JobID,
		ToolName:            request.ToolName,
		ResourceScope:       firstResourceScope(envelope.ResourceScope, request.ResourceScope),
		SuggestedConstraint: constraint,
	}}
}

func marshalHandoffResult(result BrowserHandoffResult) (json.RawMessage, error) {
	document, errorValue := json.Marshal(result)
	return document, errorValue
}

func containsAnyIfNeeded(value string, fragments []string) bool {
	if len(fragments) == 0 {
		return true
	}
	return containsAny(value, fragments)
}

func containsAny(value string, fragments []string) bool {
	for _, fragment := range fragments {
		normalizedFragment := strings.ToLower(strings.TrimSpace(fragment))
		if normalizedFragment == "" {
			continue
		}
		if strings.Contains(value, normalizedFragment) {
			return true
		}
	}
	return false
}

func webOrigin(value string) (string, error) {
	parsedURL, errorValue := url.Parse(strings.TrimSpace(value))
	if errorValue != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", errors.New("browser handoff url is invalid")
	}
	return parsedURL.Scheme + "://" + parsedURL.Host, nil
}

func webOriginOrEmpty(value string) string {
	origin, errorValue := webOrigin(value)
	if errorValue != nil {
		return ""
	}
	return origin
}

func handoffTimeoutContext(ctx context.Context, seconds int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, time.Duration(NormalizeHandoffTimeoutSeconds(seconds))*time.Second)
}
