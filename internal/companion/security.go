package companion

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/anthropic-lab/internkim/internal/capabilities"
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
}

type ApprovalDecision struct {
	Allowed             bool   `json:"allowed"`
	UserReason          string `json:"userReason"`
	SuggestedConstraint string `json:"suggestedConstraint"`
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

type MemoryGrantStore struct {
	grants map[string]*ApprovalGrant
}

type DenialError struct {
	Denial capabilities.DenialResult
}

func (errorValue DenialError) Error() string {
	return "companion job denied"
}

func NewMemoryGrantStore() *MemoryGrantStore {
	return &MemoryGrantStore{grants: map[string]*ApprovalGrant{}}
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
	if grant := store.findGrant(envelope.GrantID, capabilityScope, resourceScope, envelope.Depth); grant != nil {
		grant.UsedJobs++
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
	})
	if errorValue != nil {
		return errorValue
	}
	if !decision.Allowed {
		return DenialError{Denial: denialForJob(envelope, request, decision.UserReason, decision.SuggestedConstraint)}
	}
	if resourceScope.Kind == "" || resourceScope.Value == "" {
		return nil
	}
	store.addGrant(envelope, capabilityScope, resourceScope)
	return nil
}

func (store *MemoryGrantStore) findGrant(grantID string, capabilityScope string, resourceScope capabilities.ResourceScope, depth int) *ApprovalGrant {
	now := time.Now().UTC()
	for _, grant := range store.grants {
		if strings.TrimSpace(grantID) != "" && grant.GrantID != grantID {
			continue
		}
		if grant.Status != "active" || now.After(grant.ExpiresAt) {
			continue
		}
		if grant.UsedJobs >= grant.MaxJobs || depth > grant.MaxDepth {
			continue
		}
		if !containsString(grant.CapabilityScopes, capabilityScope) {
			continue
		}
		if !containsResourceScope(grant.ResourceScopes, resourceScope) {
			continue
		}
		return grant
	}
	return nil
}

func (store *MemoryGrantStore) addGrant(envelope JobEnvelope, capabilityScope string, resourceScope capabilities.ResourceScope) {
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

func capabilityScopeForTool(toolName string) string {
	switch {
	case strings.HasPrefix(toolName, "browser."):
		return "browser"
	case toolName == "file.pick":
		return "file"
	case strings.HasPrefix(toolName, "desktop."):
		return "desktop"
	case toolName == "user.confirm" || toolName == "user.input":
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

func containsResourceScope(values []capabilities.ResourceScope, target capabilities.ResourceScope) bool {
	for _, value := range values {
		if value.Kind == target.Kind && value.Value == target.Value {
			return true
		}
	}
	return false
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
