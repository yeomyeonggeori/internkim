package capabilityprotocol

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

const (
	ExecutionModeDevice    = "device"
	ExecutionModeCompanion = "companion"
	ExecutionModeRemote    = "remote"
	ExecutionModeAuto      = "auto"

	LLMBackendDevice         = "device_local"
	LLMBackendCompanionLocal = "companion_local"
	LLMBackendRemote         = "remote"

	AttentionTriageToolName = "attention.triage"

	CapabilityAvailable    = "ok"
	CapabilityNotConnected = "not_connected"
	CapabilityNotReady     = "not_ready"
	CapabilityNotAllowed   = "not_allowed"

	ToolOutcomeSucceeded ToolOutcome = "succeeded"
	ToolOutcomeFailed    ToolOutcome = "failed"
	ToolOutcomeDenied    ToolOutcome = "denied"

	ToolConflictResolutionAllowDuplicate ToolConflictResolution = "allow_duplicate"

	ResourceEffectIdentityID   ResourceEffectIdentity = "id"
	ResourceEffectIdentityPath ResourceEffectIdentity = "path"
	ResourceEffectIdentityURL  ResourceEffectIdentity = "url"
)

type ToolOutcome string
type ToolConflictResolution string
type ResourceEffectIdentity string

type ProtocolIdentity struct {
	ProtocolVersion       string `json:"protocolVersion"`
	AggregateProtocolHash string `json:"aggregateProtocolHash"`
}

func GeneratedProtocolIdentity() ProtocolIdentity {
	return ProtocolIdentity{
		ProtocolVersion:       GeneratedProtocolVersion(),
		AggregateProtocolHash: GeneratedAggregateProtocolHash(),
	}
}

func (identity ProtocolIdentity) Validate() error {
	if identity.ProtocolVersion == "" || identity.ProtocolVersion != strings.TrimSpace(identity.ProtocolVersion) {
		return errors.New("protocol version must be a non-empty trimmed string")
	}
	aggregateProtocolHash := identity.AggregateProtocolHash
	if aggregateProtocolHash != strings.TrimSpace(aggregateProtocolHash) {
		return errors.New("aggregate protocol hash must be a 64-character lowercase hexadecimal hash")
	}
	if len(aggregateProtocolHash) != 64 || aggregateProtocolHash != strings.ToLower(aggregateProtocolHash) {
		return errors.New("aggregate protocol hash must be a 64-character lowercase hexadecimal hash")
	}
	if _, errorValue := hex.DecodeString(aggregateProtocolHash); errorValue != nil {
		return errors.New("aggregate protocol hash must be a 64-character lowercase hexadecimal hash")
	}
	return nil
}

type Descriptor struct {
	Name                 string                        `json:"name"`
	CanonicalName        string                        `json:"canonicalName"`
	Namespace            string                        `json:"namespace"`
	ModelName            string                        `json:"modelName"`
	ModelVisibility      string                        `json:"modelVisibility"`
	ModelVisible         bool                          `json:"modelVisible"`
	Description          string                        `json:"description,omitempty"`
	Version              string                        `json:"version"`
	PrivacyClass         string                        `json:"privacyClass"`
	EstimatedLatency     string                        `json:"estimatedLatency"`
	RequiresUserPresence bool                          `json:"requiresUserPresence"`
	WorksOffline         bool                          `json:"worksOffline"`
	InputSchema          json.RawMessage               `json:"inputSchema,omitempty"`
	InputIntentSchema    json.RawMessage               `json:"inputIntentSchema,omitempty"`
	OutputSchema         json.RawMessage               `json:"outputSchema,omitempty"`
	InputSchemaStrict    bool                          `json:"inputSchemaStrict"`
	OutputSchemaStrict   bool                          `json:"outputSchemaStrict"`
	ResultContract       *ToolResultContract           `json:"resultContract,omitempty"`
	PolicyResource       string                        `json:"policyResource,omitempty"`
	SideEffectClass      string                        `json:"sideEffectClass,omitempty"`
	SideEffect           string                        `json:"sideEffect"`
	RequiresApproval     bool                          `json:"requiresApproval,omitempty"`
	CompletionEvidence   *CompletionEvidenceDescriptor `json:"completionEvidence,omitempty"`
	Availability         AvailabilityMetadata          `json:"availability"`
	Idempotency          IdempotencyMetadata           `json:"idempotency"`
}

type ToolDescriptor = Descriptor

type AvailabilityMetadata struct {
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

type IdempotencyMetadata struct {
	Supported bool   `json:"supported"`
	Required  bool   `json:"required"`
	Scope     string `json:"scope,omitempty"`
}

type CompletionEvidenceDescriptor struct {
	Mode       string `json:"mode,omitempty"`
	Action     string `json:"action,omitempty"`
	TargetKind string `json:"targetKind,omitempty"`
}

type ToolResultContract struct {
	Schema            json.RawMessage          `json:"schema"`
	Effects           []ResourceEffectContract `json:"effects,omitempty"`
	EvidenceCondition *EvidenceCondition       `json:"evidenceCondition,omitempty"`
}

type EvidenceCondition struct {
	ResultField string          `json:"resultField"`
	Equals      json.RawMessage `json:"equals"`
}

type ResourceEffectContract struct {
	ObjectType     string                 `json:"objectType"`
	Effect         string                 `json:"effect"`
	ResultField    string                 `json:"resultField"`
	EffectIdentity ResourceEffectIdentity `json:"effectIdentity"`
	When           *EvidenceCondition     `json:"when,omitempty"`
}

type ResourceEffect struct {
	ObjectType  string `json:"objectType"`
	Effect      string `json:"effect"`
	ID          string `json:"id,omitempty"`
	Path        string `json:"path,omitempty"`
	URL         string `json:"url,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
	Durability  string `json:"durability,omitempty"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Summary     string `json:"summary,omitempty"`
}

type RegistryResponse struct {
	ProtocolIdentity
	LocalOnly             bool         `json:"localOnly"`
	RoutingCandidates     []string     `json:"routingCandidates"`
	DeviceCapabilities    []Descriptor `json:"deviceCapabilities,omitempty"`
	CompanionStatus       string       `json:"companionStatus,omitempty"`
	CompanionCapabilities []Descriptor `json:"companionCapabilities,omitempty"`
	Capabilities          []Descriptor `json:"capabilities,omitempty"`
}

type ToolInvokeRequest struct {
	ToolName             string              `json:"toolName"`
	Input                json.RawMessage     `json:"input"`
	IdempotencyKey       string              `json:"idempotencyKey,omitempty"`
	Context              ToolInvokeContext   `json:"context,omitempty"`
	Actor                ActorContext        `json:"actor,omitempty"`
	Transport            ToolInvokeTransport `json:"transport,omitempty"`
	ExecutionMode        string              `json:"executionMode"`
	RequiresUserPresence bool                `json:"requiresUserPresence"`
	PrivacyClass         string              `json:"privacyClass"`
	SessionID            string              `json:"sessionID"`
	ParentJobID          string              `json:"parentJobID,omitempty"`
	GrantID              string              `json:"grantID,omitempty"`
	ResourceScope        ResourceScope       `json:"resourceScope,omitempty"`
	TimeoutSecond        int                 `json:"timeoutSecond"`
}

type ToolInvokeTransport struct {
	SiteSourceBundle *SiteSourceBundle `json:"siteSourceBundle,omitempty"`
}

type SiteSourceBundle struct {
	WorkspacePath string `json:"workspacePath"`
	ContentBase64 string `json:"contentBase64"`
	Format        string `json:"format"`
	SHA256        string `json:"sha256"`
}

type ToolInvokeContext struct {
	RequesterPersonID       string                 `json:"requesterPersonID,omitempty"`
	RequesterEmail          string                 `json:"requesterEmail,omitempty"`
	RequesterName           string                 `json:"requesterName,omitempty"`
	RequesterPlatformUserID string                 `json:"requesterPlatformUserID,omitempty"`
	TaskSource              string                 `json:"taskSource,omitempty"`
	IsScheduledRun          bool                   `json:"isScheduledRun,omitempty"`
	IsApprovalContinuation  bool                   `json:"isApprovalContinuation,omitempty"`
	ConversationID          string                 `json:"conversationID,omitempty"`
	ConversationType        string                 `json:"conversationType,omitempty"`
	ChannelID               string                 `json:"channelID,omitempty"`
	ChannelName             string                 `json:"channelName,omitempty"`
	ReplyTargetID           string                 `json:"replyTargetID,omitempty"`
	Platform                string                 `json:"platform,omitempty"`
	ConflictResolution      ToolConflictResolution `json:"conflictResolution,omitempty"`
}

type ActorContext struct {
	PersonID    string   `json:"personID,omitempty"`
	Email       string   `json:"email,omitempty"`
	DisplayName string   `json:"displayName,omitempty"`
	Source      string   `json:"source,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	IsAdmin     bool     `json:"isAdmin,omitempty"`
}

type ToolInvokeResponse struct {
	Provider        string           `json:"provider"`
	SelectedBackend string           `json:"selectedBackend"`
	ToolName        string           `json:"toolName"`
	Outcome         ToolOutcome      `json:"outcome,omitempty"`
	Effects         []ResourceEffect `json:"effects,omitempty"`
	Status          string           `json:"status,omitempty"`
	Content         string           `json:"content,omitempty"`
	IsError         bool             `json:"isError,omitempty"`
	Message         string           `json:"message,omitempty"`
	ErrorCode       string           `json:"errorCode,omitempty"`
	FailureStage    string           `json:"failureStage,omitempty"`
	Retryable       bool             `json:"retryable,omitempty"`
	SafeRetry       bool             `json:"safeRetry,omitempty"`
	Result          json.RawMessage  `json:"result"`
}

type ResourceScope struct {
	Kind  string `json:"kind,omitempty"`
	Value string `json:"value,omitempty"`
}

type CompanionJobEnvelope struct {
	JobID         string            `json:"jobID"`
	ParentJobID   string            `json:"parentJobID,omitempty"`
	GrantID       string            `json:"grantID,omitempty"`
	ToolName      string            `json:"toolName"`
	PrivacyClass  string            `json:"privacyClass"`
	ResourceScope ResourceScope     `json:"resourceScope,omitempty"`
	CreatedAt     string            `json:"createdAt,omitempty"`
	ExpiresAt     string            `json:"expiresAt,omitempty"`
	Depth         int               `json:"depth"`
	Request       ToolInvokeRequest `json:"request"`
}

type DenialResult struct {
	Status              string          `json:"status"`
	Code                string          `json:"code"`
	JobID               string          `json:"jobID"`
	ToolName            string          `json:"toolName"`
	ResourceScope       ResourceScope   `json:"resourceScope,omitempty"`
	UserReason          string          `json:"userReason,omitempty"`
	SuggestedConstraint string          `json:"suggestedConstraint,omitempty"`
	Recovery            *RecoveryAction `json:"recovery,omitempty"`
}

type RecoveryAction struct {
	Kind           string `json:"kind"`
	Delivery       string `json:"delivery"`
	DownloadURL    string `json:"downloadURL,omitempty"`
	ConnectCommand string `json:"connectCommand,omitempty"`
	PlatformUserID string `json:"platformUserID,omitempty"`
}

func RoutingCandidates() []string {
	return []string{ExecutionModeDevice, ExecutionModeCompanion, ExecutionModeRemote}
}
