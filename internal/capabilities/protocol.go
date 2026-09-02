package capabilities

import (
	"encoding/json"
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

	ToolOutcomeSucceeded = capabilityprotocol.ToolOutcomeSucceeded
	ToolOutcomeFailed    = capabilityprotocol.ToolOutcomeFailed
	ToolOutcomeDenied    = capabilityprotocol.ToolOutcomeDenied

	ToolConflictResolutionAllowDuplicate = capabilityprotocol.ToolConflictResolutionAllowDuplicate
)

type Descriptor = capabilityprotocol.Descriptor
type ToolDescriptor = capabilityprotocol.ToolDescriptor
type CompletionEvidenceDescriptor = capabilityprotocol.CompletionEvidenceDescriptor
type ToolResultContract = capabilityprotocol.ToolResultContract
type ResourceEffectContract = capabilityprotocol.ResourceEffectContract
type ResourceEffect = capabilityprotocol.ResourceEffect
type ToolOutcome = capabilityprotocol.ToolOutcome
type ToolConflictResolution = capabilityprotocol.ToolConflictResolution
type ResourceEffectIdentity = capabilityprotocol.ResourceEffectIdentity
type AvailabilityMetadata = capabilityprotocol.AvailabilityMetadata
type IdempotencyMetadata = capabilityprotocol.IdempotencyMetadata
type RegistryResponse = capabilityprotocol.RegistryResponse
type ToolInvokeRequest = capabilityprotocol.ToolInvokeRequest
type ToolInvokeContext = capabilityprotocol.ToolInvokeContext
type ToolInvokeTransport = capabilityprotocol.ToolInvokeTransport
type SiteSourceBundle = capabilityprotocol.SiteSourceBundle
type WorkspaceFile = capabilityprotocol.WorkspaceFile
type ActorContext = capabilityprotocol.ActorContext
type ToolInvokeResponse = capabilityprotocol.ToolInvokeResponse
type ResourceScope = capabilityprotocol.ResourceScope
type CompanionJobEnvelope = capabilityprotocol.CompanionJobEnvelope
type DenialResult = capabilityprotocol.DenialResult
type RecoveryAction = capabilityprotocol.RecoveryAction
type RecoveryHint = capabilityprotocol.RecoveryHint

func ProjectResourceEffects(contract *ToolResultContract, result json.RawMessage) ([]ResourceEffect, error) {
	return capabilityprotocol.ProjectResourceEffects(contract, result)
}

func canonicalizeDescriptors(descriptors []Descriptor) []Descriptor {
	return capabilityprotocol.MustCanonicalizeModelVisibleDescriptors(descriptors)
}

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
	descriptors := capabilityprotocol.MustGeneratedToolDescriptors("llm_text", "llm_structured", "embedding_create")
	descriptors = append(descriptors, WebDescriptors()...)
	descriptors = append(descriptors, FileDescriptors()...)
	descriptors = append(descriptors, PlatformMessageDescriptors()...)
	descriptors = append(descriptors, MattermostDescriptors()...)
	descriptors = append(descriptors, TaskToolDescriptors()...)
	descriptors = append(descriptors, CalendarDescriptors()...)
	descriptors = append(descriptors, LeaveDescriptors()...)
	descriptors = append(descriptors, AttendanceDescriptors()...)
	descriptors = append(descriptors, MailDescriptors()...)
	descriptors = append(descriptors, SiteAppDescriptors()...)
	descriptors = append(descriptors, CompanyDescriptors()...)
	return canonicalizeDescriptors(descriptors)
}

func CompanyDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors(
		"company_info_get",
		"company_info_set",
		"company_metric_record",
		"company_metric_list",
		"company_record_add",
		"company_record_list",
		"company_record_update",
		"company_record_delete",
		"company_document_register",
		"company_document_list",
		"company_document_search",
		"company_document_update",
	))
}

func WebDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors("web_search", "web_fetch"))
}

func FileDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors("document_read", "image_read", "image_generate"))
}

func PlatformMessageDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors(
		"message_context",
		"message_search",
		"message_send",
		"message_update",
		"message_delete",
	))
}

func MattermostDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors("channel_update"))
}

func TaskToolDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors(
		"task_add",
		"task_list",
		"task_update",
		"task_delete",
		"person_list",
	))
}

func LeaveDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors(
		"leave_list",
		"leave_balance",
		"leave_request",
		"leave_update",
		"leave_delete",
		"leave_decide",
	))
}

func AttendanceDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors(
		"attendance_list",
		"attendance_add",
		"attendance_update",
		"attendance_delete",
	))
}

func CalendarDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors(
		"event_add",
		"event_list",
		"event_update",
		"event_delete",
	))
}

func MailDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors(
		"mail_connection_status",
		"mail_connection_start",
		"mail_message_list",
		"mail_message_search",
		"mail_message_read",
		"mail_message_send",
		"mail_message_move",
		"mail_message_mark",
	))
}

func SiteAppDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors(
		"site_serve",
		"site_list",
		"site_unserve",
	))
}

func ArtifactDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors("artifact_review"))
}

func GoogleWorkspaceDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors(
		"google_docs_create",
		"google_sheets_create",
		"google_gmail_send",
		"google_calendar_event",
		"google_event_list",
		"google_drive_import_pptx",
	))
}

func completionEvidence(mode string, action string, targetKind string) *CompletionEvidenceDescriptor {
	return &CompletionEvidenceDescriptor{Mode: mode, Action: action, TargetKind: targetKind}
}

func DefaultToolDescriptors() []Descriptor {
	descriptors := CompanionToolDescriptors()
	descriptors = append(descriptors, WebDescriptors()...)
	descriptors = append(descriptors, FileDescriptors()...)
	descriptors = append(descriptors, PlatformMessageDescriptors()...)
	descriptors = append(descriptors, MattermostDescriptors()...)
	descriptors = append(descriptors, TaskToolDescriptors()...)
	descriptors = append(descriptors, CalendarDescriptors()...)
	descriptors = append(descriptors, LeaveDescriptors()...)
	descriptors = append(descriptors, AttendanceDescriptors()...)
	descriptors = append(descriptors, MailDescriptors()...)
	descriptors = append(descriptors, SiteAppDescriptors()...)
	descriptors = append(descriptors, ArtifactDescriptors()...)
	descriptors = append(descriptors, CompanyDescriptors()...)
	return canonicalizeDescriptors(descriptors)
}

func RegisteredToolDescriptors() []Descriptor {
	descriptors := DefaultToolDescriptors()
	descriptors = append(descriptors, GoogleWorkspaceDescriptors()...)
	return canonicalizeDescriptors(descriptors)
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
	isBrowserTool := toolNamespaces[strings.TrimSpace(toolName)] == "browser"
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

// Which family a tool belongs to is written on its descriptor. Reading it off the
// front of the name means every rename quietly reclassifies the tool.
var toolNamespaces = buildToolNamespaces()

func buildToolNamespaces() map[string]string {
	namespaceByToolName := map[string]string{}
	for _, descriptor := range CompanionToolDescriptors() {
		namespaceByToolName[descriptor.Name] = descriptor.Namespace
	}
	for _, descriptor := range RegisteredToolDescriptors() {
		namespaceByToolName[descriptor.Name] = descriptor.Namespace
	}
	return namespaceByToolName
}

func IsToolInNamespace(toolName string, namespace string) bool {
	return toolNamespaces[strings.TrimSpace(toolName)] == namespace
}
