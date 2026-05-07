package capabilityprotocol

import "encoding/json"

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
)

type Descriptor struct {
	Name                 string          `json:"name"`
	Version              string          `json:"version"`
	PrivacyClass         string          `json:"privacyClass"`
	EstimatedLatency     string          `json:"estimatedLatency"`
	RequiresUserPresence bool            `json:"requiresUserPresence"`
	WorksOffline         bool            `json:"worksOffline"`
	InputSchema          json.RawMessage `json:"inputSchema,omitempty"`
	OutputSchema         json.RawMessage `json:"outputSchema,omitempty"`
	PolicyResource       string          `json:"policyResource,omitempty"`
	SideEffectClass      string          `json:"sideEffectClass,omitempty"`
	RequiresApproval     bool            `json:"requiresApproval,omitempty"`
}

type RegistryResponse struct {
	LocalOnly             bool         `json:"localOnly"`
	RoutingCandidates     []string     `json:"routingCandidates"`
	DeviceCapabilities    []Descriptor `json:"deviceCapabilities,omitempty"`
	CompanionStatus       string       `json:"companionStatus,omitempty"`
	CompanionCapabilities []Descriptor `json:"companionCapabilities,omitempty"`
	Capabilities          []Descriptor `json:"capabilities,omitempty"`
}

type ToolInvokeRequest struct {
	ToolName             string            `json:"toolName"`
	Input                json.RawMessage   `json:"input"`
	Context              ToolInvokeContext `json:"context,omitempty"`
	ExecutionMode        string            `json:"executionMode"`
	RequiresUserPresence bool              `json:"requiresUserPresence"`
	PrivacyClass         string            `json:"privacyClass"`
	SessionID            string            `json:"sessionID"`
	ParentJobID          string            `json:"parentJobID,omitempty"`
	GrantID              string            `json:"grantID,omitempty"`
	ResourceScope        ResourceScope     `json:"resourceScope,omitempty"`
	TimeoutSecond        int               `json:"timeoutSecond"`
}

type ToolInvokeContext struct {
	RequesterPersonID       string `json:"requesterPersonID,omitempty"`
	RequesterEmail          string `json:"requesterEmail,omitempty"`
	RequesterName           string `json:"requesterName,omitempty"`
	RequesterPlatformUserID string `json:"requesterPlatformUserID,omitempty"`
	ConversationID          string `json:"conversationID,omitempty"`
	ConversationType        string `json:"conversationType,omitempty"`
	ChannelID               string `json:"channelID,omitempty"`
	ChannelName             string `json:"channelName,omitempty"`
	ReplyTargetID           string `json:"replyTargetID,omitempty"`
	Platform                string `json:"platform,omitempty"`
}

type ToolInvokeResponse struct {
	Provider        string          `json:"provider"`
	SelectedBackend string          `json:"selectedBackend"`
	ToolName        string          `json:"toolName"`
	Status          string          `json:"status,omitempty"`
	Content         string          `json:"content,omitempty"`
	IsError         bool            `json:"isError,omitempty"`
	Result          json.RawMessage `json:"result"`
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

func CompanionToolDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "browser.open", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false, InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string"},"startURL":{"type":"string"}},"additionalProperties":false}`), SideEffectClass: "browser"},
		{Name: "browser.snapshot", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false, InputSchema: json.RawMessage(`{"type":"object","properties":{"interactive":{"type":"boolean"}},"additionalProperties":false}`), SideEffectClass: "read"},
		{Name: "browser.screenshot", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false, InputSchema: json.RawMessage(`{"type":"object","properties":{"ttlSeconds":{"type":"integer"}},"additionalProperties":false}`), SideEffectClass: "read"},
		{Name: "browser.handoff", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false, InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string"},"message":{"type":"string"}},"additionalProperties":false}`), SideEffectClass: "handoff", RequiresApproval: true},
		{Name: "browser.click", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false, InputSchema: json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"},"ref":{"type":"string"},"selector":{"type":"string"}},"additionalProperties":false}`), SideEffectClass: "browser_write", RequiresApproval: true},
		{Name: "browser.fill", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false, InputSchema: json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"},"ref":{"type":"string"},"selector":{"type":"string"},"text":{"type":"string"}},"required":["text"],"additionalProperties":false}`), SideEffectClass: "browser_write"},
		{Name: "browser.select", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false, InputSchema: json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"},"ref":{"type":"string"},"selector":{"type":"string"},"value":{"type":"string"}},"required":["value"],"additionalProperties":false}`), SideEffectClass: "browser_write"},
		{Name: "browser.press", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false, InputSchema: json.RawMessage(`{"type":"object","properties":{"key":{"type":"string"}},"required":["key"],"additionalProperties":false}`), SideEffectClass: "browser_write"},
		{Name: "browser.wait", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false, InputSchema: json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"},"ref":{"type":"string"},"selector":{"type":"string"},"milliseconds":{"type":"integer"}},"additionalProperties":false}`), SideEffectClass: "read"},
		{Name: "user.confirm", Version: "1", PrivacyClass: "user_input", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: true, InputSchema: json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"},"reason":{"type":"string"}},"required":["message"],"additionalProperties":false}`), SideEffectClass: "approval", RequiresApproval: true},
		{Name: "user.input", Version: "1", PrivacyClass: "user_input", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: true, InputSchema: json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"},"placeholder":{"type":"string"}},"required":["message"],"additionalProperties":false}`), SideEffectClass: "approval", RequiresApproval: true},
		{Name: "file.pick", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: true, InputSchema: json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"},"accept":{"type":"array","items":{"type":"string"}},"multiple":{"type":"boolean"}},"additionalProperties":false}`), SideEffectClass: "local_file", RequiresApproval: true},
		{Name: "filesystem.mount.create", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: true},
		{Name: "filesystem.mount.list", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "filesystem.mount.pause", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "filesystem.mount.resume", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "filesystem.mount.revoke", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "filesystem.mount.status", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "filesystem.mount.stat", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "filesystem.mount.list_directory", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "filesystem.mount.read", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "filesystem.mount.write", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "filesystem.mount.mkdir", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "filesystem.mount.rename", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "filesystem.mount.delete", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "filesystem.mount.truncate", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "filesystem.mount.chmod", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "filesystem.mount.watch", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
	}
}

func CompanionLLMDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "llm.text", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "llm.structured", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "embedding.create", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: AttentionTriageToolName, Version: "1", PrivacyClass: "model_input", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
	}
}

func DeviceBrowserDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "browser.open", Version: "1", PrivacyClass: "device_browser", EstimatedLatency: "interactive", RequiresUserPresence: false, WorksOffline: false},
		{Name: "browser.snapshot", Version: "1", PrivacyClass: "device_browser", EstimatedLatency: "interactive", RequiresUserPresence: false, WorksOffline: false},
		{Name: "browser.click", Version: "1", PrivacyClass: "device_browser", EstimatedLatency: "interactive", RequiresUserPresence: false, WorksOffline: false},
		{Name: "browser.fill", Version: "1", PrivacyClass: "device_browser", EstimatedLatency: "interactive", RequiresUserPresence: false, WorksOffline: false},
		{Name: "browser.select", Version: "1", PrivacyClass: "device_browser", EstimatedLatency: "interactive", RequiresUserPresence: false, WorksOffline: false},
		{Name: "browser.press", Version: "1", PrivacyClass: "device_browser", EstimatedLatency: "interactive", RequiresUserPresence: false, WorksOffline: false},
		{Name: "browser.wait", Version: "1", PrivacyClass: "device_browser", EstimatedLatency: "interactive", RequiresUserPresence: false, WorksOffline: false},
	}
}

func RoutingCandidates() []string {
	return []string{ExecutionModeDevice, ExecutionModeCompanion, ExecutionModeRemote}
}
