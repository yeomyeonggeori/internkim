package capabilities

import "encoding/json"

const (
	ExecutionModeDevice      = "device"
	ExecutionModeCompanion   = "companion"
	ExecutionModeUserDesktop = "user_desktop"
	ExecutionModeRemote      = "remote"
	ExecutionModeAuto        = "auto"

	LLMBackendDevice         = "device_local"
	LLMBackendCompanionLocal = "companion_local"
	LLMBackendRemote         = "remote"
)

type Descriptor struct {
	Name                 string `json:"name"`
	Version              string `json:"version"`
	PrivacyClass         string `json:"privacyClass"`
	EstimatedLatency     string `json:"estimatedLatency"`
	RequiresUserPresence bool   `json:"requiresUserPresence"`
	WorksOffline         bool   `json:"worksOffline"`
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
	ToolName             string          `json:"toolName"`
	Input                json.RawMessage `json:"input"`
	ExecutionMode        string          `json:"executionMode"`
	RequiresUserPresence bool            `json:"requiresUserPresence"`
	PrivacyClass         string          `json:"privacyClass"`
	SessionID            string          `json:"sessionID"`
	ParentJobID          string          `json:"parentJobID,omitempty"`
	GrantID              string          `json:"grantID,omitempty"`
	ResourceScope        ResourceScope   `json:"resourceScope,omitempty"`
	TimeoutSecond        int             `json:"timeoutSecond"`
}

type ToolInvokeResponse struct {
	Provider        string          `json:"provider"`
	SelectedBackend string          `json:"selectedBackend"`
	ToolName        string          `json:"toolName"`
	Status          string          `json:"status,omitempty"`
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
	Status              string        `json:"status"`
	Code                string        `json:"code"`
	JobID               string        `json:"jobID"`
	ToolName            string        `json:"toolName"`
	ResourceScope       ResourceScope `json:"resourceScope,omitempty"`
	UserReason          string        `json:"userReason,omitempty"`
	SuggestedConstraint string        `json:"suggestedConstraint,omitempty"`
}

func CompanionToolDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "browser.session.start", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false},
		{Name: "browser.navigate", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false},
		{Name: "browser.observe", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false},
		{Name: "browser.screenshot", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false},
		{Name: "browser.click", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false},
		{Name: "browser.fill", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false},
		{Name: "browser.select", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false},
		{Name: "browser.press", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false},
		{Name: "browser.wait", Version: "1", PrivacyClass: "user_browser", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false},
		{Name: "user.confirm", Version: "1", PrivacyClass: "user_input", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: true},
		{Name: "user.input", Version: "1", PrivacyClass: "user_input", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: true},
		{Name: "file.pick", Version: "1", PrivacyClass: "local_file", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: true},
	}
}

func CompanionLLMDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "llm.text", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "llm.structured", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
		{Name: "embedding.create", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true},
	}
}

func DeviceDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "llm.text", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true},
		{Name: "llm.structured", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true},
		{Name: "embedding.create", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false},
		{Name: "platform.reply", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false},
	}
}

func CompanionToolNames() []string {
	descriptors := CompanionToolDescriptors()
	toolNames := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		toolNames = append(toolNames, descriptor.Name)
	}
	return toolNames
}

func RoutingCandidates() []string {
	return []string{ExecutionModeDevice, ExecutionModeCompanion, ExecutionModeRemote}
}
