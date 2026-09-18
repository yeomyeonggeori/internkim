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
)

type Descriptor = capabilityprotocol.Descriptor
type ToolDescriptor = capabilityprotocol.ToolDescriptor
type CompletionEvidenceDescriptor = capabilityprotocol.CompletionEvidenceDescriptor
type ToolResultContract = capabilityprotocol.ToolResultContract
type ResourceEffectContract = capabilityprotocol.ResourceEffectContract
type ResourceEffect = capabilityprotocol.ResourceEffect
type ToolOutcome = capabilityprotocol.ToolOutcome
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
type ApprovalTarget = capabilityprotocol.ApprovalTarget
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

func RegistryDescriptors(registry RegistryResponse) []Descriptor {
	if len(registry.Capabilities) > 0 {
		return registry.Capabilities
	}
	return registry.DeviceCapabilities
}

func descriptorsTheRecordAndTheCompanyAnswer() []Descriptor {
	descriptors := []Descriptor{}
	for _, descriptor := range capabilityprotocol.GeneratedToolDescriptorSet() {
		if descriptor.AnsweredBy != capabilityprotocol.AnsweredByRecord && descriptor.AnsweredBy != capabilityprotocol.AnsweredByCompany {
			continue
		}
		descriptors = append(descriptors, descriptor)
	}
	return descriptors
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

func TaskToolDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors(
		"task_add",
		"task_list",
		"task_update",
		"task_delete",
		"person_list",
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

func DefaultToolDescriptors() []Descriptor {
	descriptors := CompanionToolDescriptors()
	descriptors = append(descriptors, descriptorsTheRecordAndTheCompanyAnswer()...)
	return canonicalizeDescriptors(descriptors)
}

func RoutingCandidates() []string {
	return capabilityprotocol.RoutingCandidates()
}

func CompanionInstallURL() string {
	return "https://docs.intern.kim/docs/companion"
}

func CompanionConnectRecovery() *RecoveryAction {
	return &RecoveryAction{
		Kind:           "companion_connect",
		Delivery:       "dm_preferred",
		DownloadURL:    CompanionInstallURL(),
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

var toolNamespaces = buildToolNamespaces()

func buildToolNamespaces() map[string]string {
	namespaceByToolName := map[string]string{}
	for _, descriptor := range CompanionToolDescriptors() {
		namespaceByToolName[descriptor.Name] = descriptor.Namespace
	}
	for _, descriptor := range DefaultToolDescriptors() {
		namespaceByToolName[descriptor.Name] = descriptor.Namespace
	}
	return namespaceByToolName
}

func IsToolInNamespace(toolName string, namespace string) bool {
	return toolNamespaces[strings.TrimSpace(toolName)] == namespace
}
