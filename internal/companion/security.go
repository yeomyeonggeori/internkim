package companion

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

const (
	defaultGrantMaxJobs  = 20
	defaultGrantMaxDepth = 8
	defaultGrantTTL      = 8 * time.Hour
)

type ApprovalHandler interface {
	Approve(ctx context.Context, request ApprovalRequest) (ApprovalDecision, error)
}

type ApprovalRequest struct {
	JobID           string                     `json:"jobID"`
	ToolName        string                     `json:"toolName"`
	CapabilityScope string                     `json:"capabilityScope"`
	ResourceScope   capabilities.ResourceScope `json:"resourceScope"`
	TimeoutSeconds  int                        `json:"timeoutSeconds,omitempty"`
}

type ApprovalDecision struct {
	Allowed             bool   `json:"allowed"`
	UserReason          string `json:"userReason"`
	SuggestedConstraint string `json:"suggestedConstraint"`
	RememberSession     bool   `json:"rememberSession,omitempty"`
}

type JobEnvelope struct {
	JobID         string                     `json:"jobID"`
	ParentJobID   string                     `json:"parentJobID,omitempty"`
	GrantID       string                     `json:"grantID,omitempty"`
	ToolName      string                     `json:"toolName"`
	PrivacyClass  string                     `json:"privacyClass"`
	ResourceScope capabilities.ResourceScope `json:"resourceScope,omitempty"`
	Depth         int                        `json:"depth"`
}

type ApprovalGrant struct {
	GrantID          string
	AnchorJobID      string
	CapabilityScopes []string
	ResourceScopes   []capabilities.ResourceScope
	MaxJobs          int
	MaxDepth         int
	ExpiresAt        time.Time
	Status           string
	UsedJobs         int
}

type GrantSnapshot struct {
	GrantID          string                       `json:"grantID"`
	AnchorJobID      string                       `json:"anchorJobID"`
	CapabilityScopes []string                     `json:"capabilityScopes"`
	ResourceScopes   []capabilities.ResourceScope `json:"resourceScopes"`
	MaxJobs          int                          `json:"maxJobs"`
	MaxDepth         int                          `json:"maxDepth"`
	ExpiresAt        time.Time                    `json:"expiresAt"`
	Status           string                       `json:"status"`
	UsedJobs         int                          `json:"usedJobs"`
	DisplayName      string                       `json:"displayName"`
}

type MemoryGrantStore struct {
	mutex      sync.Mutex
	grants     map[string]*ApprovalGrant
	jobLineage map[string]string
}

type DenialError struct {
	Denial capabilities.DenialResult
}

func (errorValue DenialError) Error() string {
	return "companion job denied"
}

func NewMemoryGrantStore() *MemoryGrantStore {
	return &MemoryGrantStore{grants: map[string]*ApprovalGrant{}, jobLineage: map[string]string{}}
}

func (store *MemoryGrantStore) Authorize(ctx context.Context, envelope JobEnvelope, request capabilities.ToolInvokeRequest, approvalHandler ApprovalHandler) error {
	capabilityScope := capabilityScopeForTool(firstNonEmpty(envelope.ToolName, request.ToolName))
	if capabilityScope == "" {
		return nil
	}
	if capabilityScope == "user_input" {
		return nil
	}
	resourceScope := firstResourceScope(envelope.ResourceScope, request.ResourceScope)
	if store.consumeGrant(envelope, capabilityScope, resourceScope) {
		return nil
	}
	if approvalHandler == nil {
		return DenialError{Denial: denialForJob(envelope, request, "", "approval UI is unavailable")}
	}
	decision, errorValue := approvalHandler.Approve(ctx, ApprovalRequest{
		JobID:           envelope.JobID,
		ToolName:        firstNonEmpty(envelope.ToolName, request.ToolName),
		CapabilityScope: capabilityScope,
		ResourceScope:   resourceScope,
		TimeoutSeconds:  approvalTimeoutSeconds(ctx),
	})
	if errorValue != nil {
		return errorValue
	}
	if !decision.Allowed {
		return DenialError{Denial: denialForJob(envelope, request, decision.UserReason, decision.SuggestedConstraint)}
	}
	grantResourceScope, shouldRemember := approvalGrantResourceScope(decision, resourceScope)
	if !shouldRemember {
		return nil
	}
	store.addGrant(envelope, capabilityScope, grantResourceScope)
	return nil
}

func approvalTimeoutSeconds(ctx context.Context) int {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 1
	}
	return int((remaining + time.Second - time.Nanosecond) / time.Second)
}

func approvalGrantResourceScope(decision ApprovalDecision, resourceScope capabilities.ResourceScope) (capabilities.ResourceScope, bool) {
	if resourceScope.Kind != "" && resourceScope.Value != "" {
		return resourceScope, true
	}
	if !decision.RememberSession {
		return capabilities.ResourceScope{}, false
	}
	return capabilities.ResourceScope{Kind: "session", Value: "current"}, true
}

func (store *MemoryGrantStore) ListActive() []GrantSnapshot {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	now := time.Now().UTC()
	snapshots := []GrantSnapshot{}
	for _, grant := range store.grants {
		if grant.Status != "active" || now.After(grant.ExpiresAt) {
			continue
		}
		snapshots = append(snapshots, grantSnapshot(*grant))
	}
	return snapshots
}

func (store *MemoryGrantStore) Revoke(grantID string) bool {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	grant, ok := store.grants[grantID]
	if !ok {
		return false
	}
	grant.Status = "revoked"
	return true
}

func (store *MemoryGrantStore) consumeGrant(envelope JobEnvelope, capabilityScope string, resourceScope capabilities.ResourceScope) bool {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.recordJobLineageLocked(envelope.JobID, envelope.ParentJobID)
	grant := store.findGrantLocked(envelope, capabilityScope, resourceScope)
	if grant == nil {
		return false
	}
	grant.UsedJobs++
	return true
}

func (store *MemoryGrantStore) recordJobLineageLocked(jobID string, parentJobID string) {
	if strings.TrimSpace(jobID) == "" {
		return
	}
	if _, alreadyRecorded := store.jobLineage[jobID]; alreadyRecorded {
		return
	}
	store.jobLineage[jobID] = parentJobID
}

func (store *MemoryGrantStore) findGrantLocked(envelope JobEnvelope, capabilityScope string, resourceScope capabilities.ResourceScope) *ApprovalGrant {
	now := time.Now().UTC()
	for _, grant := range store.grants {
		if strings.TrimSpace(envelope.GrantID) != "" && grant.GrantID != envelope.GrantID {
			continue
		}
		if grant.Status != "active" || now.After(grant.ExpiresAt) {
			continue
		}
		if grant.UsedJobs >= grant.MaxJobs || envelope.Depth > grant.MaxDepth {
			continue
		}
		if !containsString(grant.CapabilityScopes, capabilityScope) {
			continue
		}
		if !store.grantCoversResourceScopeLocked(grant, resourceScope, envelope) {
			continue
		}
		return grant
	}
	return nil
}

func (store *MemoryGrantStore) grantCoversResourceScopeLocked(grant *ApprovalGrant, target capabilities.ResourceScope, envelope JobEnvelope) bool {
	for _, grantedResourceScope := range grant.ResourceScopes {
		if grantedResourceScope.Kind == target.Kind && grantedResourceScope.Value == target.Value {
			return true
		}
		if isSessionWildcardResourceScope(grantedResourceScope) && store.isSameTaskLineageLocked(envelope, grant.AnchorJobID) {
			return true
		}
	}
	return false
}

func (store *MemoryGrantStore) isSameTaskLineageLocked(envelope JobEnvelope, anchorJobID string) bool {
	if strings.TrimSpace(anchorJobID) == "" {
		return false
	}
	if envelope.JobID == anchorJobID {
		return true
	}
	visitedJobIDs := map[string]bool{}
	currentJobID := envelope.ParentJobID
	for hops := 0; hops <= defaultGrantMaxDepth+1; hops++ {
		if currentJobID == "" {
			return false
		}
		if currentJobID == anchorJobID {
			return true
		}
		if visitedJobIDs[currentJobID] {
			return false
		}
		visitedJobIDs[currentJobID] = true
		currentJobID = store.jobLineage[currentJobID]
	}
	return false
}

func (store *MemoryGrantStore) addGrant(envelope JobEnvelope, capabilityScope string, resourceScope capabilities.ResourceScope) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	grantID := firstNonEmpty(envelope.GrantID, "grant-"+envelope.JobID)
	store.grants[grantID] = &ApprovalGrant{
		GrantID:          grantID,
		AnchorJobID:      envelope.JobID,
		CapabilityScopes: []string{capabilityScope},
		ResourceScopes:   []capabilities.ResourceScope{resourceScope},
		MaxJobs:          defaultGrantMaxJobs,
		MaxDepth:         defaultGrantMaxDepth,
		ExpiresAt:        time.Now().UTC().Add(defaultGrantTTL),
		Status:           "active",
		UsedJobs:         1,
	}
}

func grantSnapshot(grant ApprovalGrant) GrantSnapshot {
	return GrantSnapshot{
		GrantID:          grant.GrantID,
		AnchorJobID:      grant.AnchorJobID,
		CapabilityScopes: append([]string{}, grant.CapabilityScopes...),
		ResourceScopes:   append([]capabilities.ResourceScope{}, grant.ResourceScopes...),
		MaxJobs:          grant.MaxJobs,
		MaxDepth:         grant.MaxDepth,
		ExpiresAt:        grant.ExpiresAt,
		Status:           grant.Status,
		UsedJobs:         grant.UsedJobs,
		DisplayName:      grantDisplayName(grant),
	}
}

func grantDisplayName(grant ApprovalGrant) string {
	capabilityScope := "Companion"
	if len(grant.CapabilityScopes) > 0 {
		capabilityScope = grant.CapabilityScopes[0]
	}
	resourceScope := "this task"
	if len(grant.ResourceScopes) > 0 && grant.ResourceScopes[0].Value != "" {
		resourceScope = grant.ResourceScopes[0].Value
	}
	if len(grant.ResourceScopes) > 0 && grant.ResourceScopes[0].Kind == "session" {
		resourceScope = "this session"
	}
	return titleCapabilityScope(capabilityScope) + " access to " + resourceScope
}

func titleCapabilityScope(value string) string {
	if value == "" {
		return "Companion"
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func capabilityScopeForTool(toolName string) string {
	switch {
	case strings.HasPrefix(toolName, "browser_"):
		return "browser"
	case toolName == "file_pick":
		return "file"
	case strings.HasPrefix(toolName, "desktop."):
		return "desktop"
	case toolName == "user_confirm" || toolName == "user.input":
		return "user_input"
	default:
		return ""
	}
}

func denialForJob(envelope JobEnvelope, request capabilities.ToolInvokeRequest, userReason string, suggestedConstraint string) capabilities.DenialResult {
	return capabilities.DenialResult{
		Status:              "denied",
		Code:                "user_denied",
		JobID:               envelope.JobID,
		ToolName:            firstNonEmpty(envelope.ToolName, request.ToolName),
		ResourceScope:       firstResourceScope(envelope.ResourceScope, request.ResourceScope),
		UserReason:          sanitizeDenialText(userReason),
		SuggestedConstraint: sanitizeDenialText(suggestedConstraint),
	}
}

func firstResourceScope(values ...capabilities.ResourceScope) capabilities.ResourceScope {
	for _, value := range values {
		if value.Kind != "" || value.Value != "" {
			return value
		}
	}
	return capabilities.ResourceScope{}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func isSessionWildcardResourceScope(value capabilities.ResourceScope) bool {
	return value.Kind == "session" && value.Value == "current"
}

func sanitizeDenialText(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if len(trimmedValue) > 240 {
		return trimmedValue[:240]
	}
	return trimmedValue
}

func DenialResponse(denial capabilities.DenialResult) (capabilities.ToolInvokeResponse, error) {
	document, errorValue := json.Marshal(denial)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if denial.ToolName == "" {
		return capabilities.ToolInvokeResponse{}, errors.New("denial tool name is required")
	}
	return capabilities.ToolInvokeResponse{
		Provider: "companion",
		ToolName: denial.ToolName,
		Status:   "denied",
		Result:   document,
	}, nil
}
