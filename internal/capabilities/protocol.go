package capabilities

import (
	"strings"

	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

const (
	ExecutionModeDevice    = capabilityprotocol.ExecutionModeDevice
	ExecutionModeCompanion = capabilityprotocol.ExecutionModeCompanion
	ExecutionModeRemote    = capabilityprotocol.ExecutionModeRemote
	ExecutionModeAuto      = capabilityprotocol.ExecutionModeAuto

	LLMBackendDevice         = capabilityprotocol.LLMBackendDevice
	LLMBackendCompanionLocal = capabilityprotocol.LLMBackendCompanionLocal
	LLMBackendRemote         = capabilityprotocol.LLMBackendRemote

	AttentionTriageToolName = capabilityprotocol.AttentionTriageToolName

	CapabilityAvailable    = capabilityprotocol.CapabilityAvailable
	CapabilityNotConnected = capabilityprotocol.CapabilityNotConnected
	CapabilityNotReady     = capabilityprotocol.CapabilityNotReady
	CapabilityNotAllowed   = capabilityprotocol.CapabilityNotAllowed
)

type Descriptor = capabilityprotocol.Descriptor
type RegistryResponse = capabilityprotocol.RegistryResponse
type ToolInvokeRequest = capabilityprotocol.ToolInvokeRequest
type ToolInvokeContext = capabilityprotocol.ToolInvokeContext
type ToolInvokeResponse = capabilityprotocol.ToolInvokeResponse
type ResourceScope = capabilityprotocol.ResourceScope
type CompanionJobEnvelope = capabilityprotocol.CompanionJobEnvelope
type DenialResult = capabilityprotocol.DenialResult
type RecoveryAction = capabilityprotocol.RecoveryAction

func CompanionToolDescriptors() []Descriptor {
	return capabilityprotocol.CompanionToolDescriptors()
}

func CompanionLLMDescriptors() []Descriptor {
	return capabilityprotocol.CompanionLLMDescriptors()
}

func DeviceBrowserDescriptors() []Descriptor {
	return capabilityprotocol.DeviceBrowserDescriptors()
}

func DeviceDescriptors() []Descriptor {
	return append([]Descriptor{
		{Name: "llm.text", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true},
		{Name: "llm.structured", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true},
		{Name: "embedding.create", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true},
		{Name: "platform.reply", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false},
	}, append(append(DeviceBrowserDescriptors(), FlowDescriptors()...), SiteAppDescriptors()...)...)
}

func FlowDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "flow.task.add", Version: "1", PrivacyClass: "workspace_task", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false},
	}
}

func SiteAppDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "site.app.create", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false},
		{Name: "site.app.publish", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "high", RequiresUserPresence: false, WorksOffline: false},
		{Name: "site.app.status", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false},
		{Name: "site.app.logs", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false},
		{Name: "site.app.rollback", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false},
		{Name: "site.app.unpublish", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false},
		{Name: "site.app.restore", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false},
		{Name: "site.app.delete", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false},
	}
}

func GoogleWorkspaceDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "google.docs.create", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false},
		{Name: "google.sheets.create", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false},
		{Name: "google.gmail.send", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false},
		{Name: "google.calendar.event", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false},
		{Name: "google.calendar.list", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false},
		{Name: "google.drive.import_pptx", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false},
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

func DefaultToolNames() []string {
	toolNames := CompanionToolNames()
	for _, descriptor := range FlowDescriptors() {
		toolNames = append(toolNames, descriptor.Name)
	}
	for _, descriptor := range SiteAppDescriptors() {
		toolNames = append(toolNames, descriptor.Name)
	}
	return toolNames
}

func RoutingCandidates() []string {
	return capabilityprotocol.RoutingCandidates()
}

func CompanionMacOSBetaDownloadURL() string {
	return "https://gitlab.com/eastriver/internkim/-/releases/permalink/latest/downloads/internkim-companion-beta-macos-aarch64.dmg"
}

func CompanionConnectRecovery() *RecoveryAction {
	return &RecoveryAction{
		Kind:           "companion_connect",
		Delivery:       "dm_preferred",
		DownloadURL:    CompanionMacOSBetaDownloadURL(),
		ConnectCommand: "/connect",
	}
}

func CapabilityUnavailableUserReason(toolName string, code string) string {
	isBrowserTool := strings.HasPrefix(strings.TrimSpace(toolName), "browser.")
	switch code {
	case CapabilityNotReady:
		if isBrowserTool {
			return "Companion은 연결되어 있지만 브라우저 런타임이 준비되지 않았습니다."
		}
		return "Companion은 연결되어 있지만 이 기능이 준비되지 않았습니다."
	case CapabilityNotAllowed:
		return "이 요청을 실행할 수 있는 Companion 권한이 없습니다."
	default:
		if isBrowserTool {
			return "Companion이 연결되어 있지 않아 브라우저를 열 수 없습니다."
		}
		return "Companion이 연결되어 있지 않습니다."
	}
}
