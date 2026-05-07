package companion

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
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
	ResumePrompt    string                 `json:"resumePrompt,omitempty"`
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
	HandoffID       string   `json:"handoffID"`
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
	Active       bool   `json:"active"`
	HandoffID    string `json:"handoffID,omitempty"`
	SessionID    string `json:"sessionID,omitempty"`
	State        string `json:"state,omitempty"`
	Message      string `json:"message,omitempty"`
	ResumePrompt string `json:"resumePrompt,omitempty"`
	Origin       string `json:"origin,omitempty"`
}

type HandoffCompletion struct {
	HandoffID       string   `json:"handoffID"`
	SessionID       string   `json:"sessionID,omitempty"`
	URL             string   `json:"url,omitempty"`
	Title           string   `json:"title,omitempty"`
	SnapshotText    string   `json:"snapshotText,omitempty"`
	InteractiveRefs []string `json:"interactiveRefs,omitempty"`
	CapturedAt      string   `json:"capturedAt,omitempty"`
}

type BrowserHandoffStore struct {
	mutex               sync.Mutex
	active              *browserHandoff
	completedHandoffID  string
	completedCompletion HandoffCompletion
	statePath           string
}

type browserHandoff struct {
	handoffID    string
	sessionID    string
	state        string
	message      string
	resumePrompt string
	origin       string
	waiter       chan HandoffCompletion
	restored     bool
}

func NewBrowserHandoffStore() *BrowserHandoffStore {
	return &BrowserHandoffStore{}
}

func NewPersistentBrowserHandoffStore(statePath string) *BrowserHandoffStore {
	store := &BrowserHandoffStore{statePath: strings.TrimSpace(statePath)}
	store.load()
	return store
}

func (store *BrowserHandoffStore) Begin(request BrowserHandoffRequest, sessionID string) (HandoffSnapshot, error) {
	snapshot, reused, errorValue := store.BeginOrReuse(request, sessionID)
	if errorValue != nil {
		return HandoffSnapshot{}, errorValue
	}
	if reused {
		return HandoffSnapshot{}, errors.New("browser handoff is already active")
	}
	return snapshot, nil
}

func (store *BrowserHandoffStore) BeginOrReuse(request BrowserHandoffRequest, sessionID string) (HandoffSnapshot, bool, error) {
	if store == nil {
		return HandoffSnapshot{}, false, errors.New("browser handoff store is unavailable")
	}
	origin, errorValue := webOrigin(request.URL)
	if errorValue != nil {
		return HandoffSnapshot{}, false, errorValue
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.active != nil {
		if store.active.origin == origin && !store.active.restored {
			return store.active.snapshot(), true, nil
		}
		if !store.active.restored {
			return HandoffSnapshot{}, false, errors.New("browser handoff is already active")
		}
	}
	handoff := &browserHandoff{
		handoffID:    randomHandoffID(16),
		sessionID:    firstNonEmpty(sessionID, request.SessionID, "internkim"),
		state:        HandoffStateWaitingForUser,
		message:      firstNonEmpty(request.Message, "브라우저에서 필요한 작업을 마친 뒤 완료를 눌러주세요."),
		resumePrompt: strings.TrimSpace(request.ResumePrompt),
		origin:       origin,
		waiter:       make(chan HandoffCompletion, 1),
	}
	store.active = handoff
	store.completedHandoffID = ""
	store.completedCompletion = HandoffCompletion{}
	_ = store.saveLocked()
	return handoff.snapshot(), false, nil
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
	return store.saveLocked()
}

func (store *BrowserHandoffStore) UpdateSessionID(handoffID string, sessionID string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	handoff, errorValue := store.requireActive(handoffID)
	if errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	handoff.sessionID = strings.TrimSpace(sessionID)
	return store.saveLocked()
}

func (store *BrowserHandoffStore) Complete(completion HandoffCompletion) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	handoff, errorValue := store.validateCompletionLocked(completion)
	if errorValue != nil {
		return errorValue
	}
	if handoff == nil {
		return nil
	}
	handoff.state = HandoffStateCompleted
	store.completedHandoffID = handoff.handoffID
	store.completedCompletion = completion
	store.active = nil
	_ = store.saveLocked()
	select {
	case handoff.waiter <- completion:
	default:
	}
	return nil
}

func (store *BrowserHandoffStore) ValidateCompletion(completion HandoffCompletion) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	_, errorValue := store.validateCompletionLocked(completion)
	return errorValue
}

func (store *BrowserHandoffStore) validateCompletionLocked(completion HandoffCompletion) (*browserHandoff, error) {
	handoff, errorValue := store.requireActive(completion.HandoffID)
	if errorValue != nil {
		if store.completedHandoffID != "" && completion.HandoffID == store.completedHandoffID {
			return nil, nil
		}
		return nil, errorValue
	}
	if strings.TrimSpace(completion.SessionID) != "" && completion.SessionID != handoff.sessionID {
		return nil, errors.New("browser handoff session mismatch")
	}
	if _, errorValue := webOrigin(completion.URL); errorValue != nil {
		return nil, errors.New("browser handoff completion url is invalid")
	}
	return handoff, nil
}

func (store *BrowserHandoffStore) Wait(ctx context.Context, handoffID string) (HandoffCompletion, error) {
	store.mutex.Lock()
	handoff, errorValue := store.requireActive(handoffID)
	if errorValue != nil {
		if store.completedHandoffID != "" && store.completedHandoffID == handoffID {
			completion := store.completedCompletion
			store.mutex.Unlock()
			return completion, nil
		}
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
	_ = store.saveLocked()
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
		Active:       true,
		HandoffID:    handoff.handoffID,
		SessionID:    handoff.sessionID,
		State:        handoff.state,
		Message:      handoff.message,
		ResumePrompt: handoff.resumePrompt,
		Origin:       handoff.origin,
	}
}

type browserHandoffDocument struct {
	HandoffID    string `json:"handoffID"`
	SessionID    string `json:"sessionID"`
	State        string `json:"state"`
	Message      string `json:"message"`
	ResumePrompt string `json:"resumePrompt,omitempty"`
	Origin       string `json:"origin"`
}

func (store *BrowserHandoffStore) load() {
	if store.statePath == "" {
		return
	}
	document, errorValue := os.ReadFile(store.statePath)
	if errorValue != nil {
		return
	}
	var activeHandoff browserHandoffDocument
	if errorValue := json.Unmarshal(document, &activeHandoff); errorValue != nil {
		return
	}
	if strings.TrimSpace(activeHandoff.HandoffID) == "" {
		return
	}
	store.active = &browserHandoff{
		handoffID:    activeHandoff.HandoffID,
		sessionID:    firstNonEmpty(activeHandoff.SessionID, "internkim"),
		state:        firstNonEmpty(activeHandoff.State, HandoffStateWaitingForUser),
		message:      firstNonEmpty(activeHandoff.Message, "브라우저에서 필요한 작업을 마친 뒤 완료를 눌러주세요."),
		resumePrompt: strings.TrimSpace(activeHandoff.ResumePrompt),
		origin:       activeHandoff.Origin,
		waiter:       make(chan HandoffCompletion, 1),
		restored:     true,
	}
}

func (store *BrowserHandoffStore) saveLocked() error {
	if store.statePath == "" {
		return nil
	}
	if store.active == nil {
		if errorValue := os.Remove(store.statePath); errorValue != nil && !errors.Is(errorValue, os.ErrNotExist) {
			return errorValue
		}
		return nil
	}
	if errorValue := os.MkdirAll(filepath.Dir(store.statePath), 0o700); errorValue != nil {
		return errorValue
	}
	document, errorValue := json.MarshalIndent(browserHandoffDocument{
		HandoffID:    store.active.handoffID,
		SessionID:    store.active.sessionID,
		State:        store.active.state,
		Message:      store.active.message,
		ResumePrompt: store.active.resumePrompt,
		Origin:       store.active.origin,
	}, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	return os.WriteFile(store.statePath, document, 0o600)
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
