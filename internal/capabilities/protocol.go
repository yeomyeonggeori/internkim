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
	}, append(append(append(append(DeviceBrowserDescriptors(), FlowDescriptors()...), CalendarDescriptors()...), MailDescriptors()...), SiteAppDescriptors()...)...)
}

func FlowDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "flow.task.add", Version: "1", PrivacyClass: "workspace_task", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: flowTaskAddInputSchema(), PolicyResource: "tool:flow.task.add", SideEffectClass: "workspace_write"},
	}
}

func CalendarDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "calendar.event.add", Version: "1", PrivacyClass: "workspace_calendar", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: calendarEventWriteInputSchema(), PolicyResource: "tool:calendar.event.add", SideEffectClass: "workspace_write"},
		{Name: "calendar.event.list", Version: "1", PrivacyClass: "workspace_calendar", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: calendarEventListInputSchema(), PolicyResource: "tool:calendar.event.list", SideEffectClass: "read"},
		{Name: "calendar.event.update", Version: "1", PrivacyClass: "workspace_calendar", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: calendarEventUpdateInputSchema(), PolicyResource: "tool:calendar.event.update", SideEffectClass: "workspace_write"},
		{Name: "calendar.event.delete", Version: "1", PrivacyClass: "workspace_calendar", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: calendarEventDeleteInputSchema(), PolicyResource: "tool:calendar.event.delete", SideEffectClass: "destructive", RequiresApproval: true},
	}
}

func MailDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "mail.message.list", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageListInputSchema(), PolicyResource: "tool:mail.message.list", SideEffectClass: "read"},
		{Name: "mail.message.read", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageReadInputSchema(), PolicyResource: "tool:mail.message.read", SideEffectClass: "read"},
		{Name: "mail.message.send", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageSendInputSchema(), PolicyResource: "tool:mail.message.send", SideEffectClass: "external_send", RequiresApproval: true},
		{Name: "mail.message.move", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageMoveInputSchema(), PolicyResource: "tool:mail.message.move", SideEffectClass: "workspace_write"},
		{Name: "mail.message.mark", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageMarkInputSchema(), PolicyResource: "tool:mail.message.mark", SideEffectClass: "workspace_write"},
	}
}

func SiteAppDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "site.app.create", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppCreateInputSchema(), PolicyResource: "tool:site.app.create", SideEffectClass: "workspace_write"},
		{Name: "site.app.publish", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "high", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppPublishInputSchema(), PolicyResource: "tool:site.app.publish", SideEffectClass: "external_publish"},
		{Name: "site.app.status", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLookupInputSchema(), PolicyResource: "tool:site.app.status", SideEffectClass: "read"},
		{Name: "site.app.logs", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLookupInputSchema(), PolicyResource: "tool:site.app.logs", SideEffectClass: "read"},
		{Name: "site.app.rollback", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLifecycleInputSchema(), PolicyResource: "tool:site.app.rollback", SideEffectClass: "external_publish", RequiresApproval: true},
		{Name: "site.app.unpublish", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLifecycleInputSchema(), PolicyResource: "tool:site.app.unpublish", SideEffectClass: "external_publish", RequiresApproval: true},
		{Name: "site.app.restore", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLifecycleInputSchema(), PolicyResource: "tool:site.app.restore", SideEffectClass: "workspace_write"},
		{Name: "site.app.delete", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppDeleteInputSchema(), PolicyResource: "tool:site.app.delete", SideEffectClass: "destructive", RequiresApproval: true},
	}
}

func GoogleWorkspaceDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "google.docs.create", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleDocsCreateInputSchema(), PolicyResource: "tool:google.docs.create", SideEffectClass: "external_write"},
		{Name: "google.sheets.create", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleSheetsCreateInputSchema(), PolicyResource: "tool:google.sheets.create", SideEffectClass: "external_write"},
		{Name: "google.gmail.send", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleGmailSendInputSchema(), PolicyResource: "tool:google.gmail.send", SideEffectClass: "external_send", RequiresApproval: true},
		{Name: "google.calendar.event", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleCalendarEventInputSchema(), PolicyResource: "tool:google.calendar.event", SideEffectClass: "external_write"},
		{Name: "google.calendar.list", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleCalendarListInputSchema(), PolicyResource: "tool:google.calendar.list", SideEffectClass: "read"},
		{Name: "google.drive.import_pptx", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"title":{"type":"string"}},"required":["path"],"additionalProperties":false}`), PolicyResource: "tool:google.drive.import_pptx", SideEffectClass: "external_write"},
	}
}

func flowTaskAddInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"prompt":{"type":"string"},"targetPersonHint":{"type":"string"},"weekCode":{"type":"string"},"allowDuplicate":{"type":"boolean"}},"required":["prompt"],"additionalProperties":false}`)
}

func calendarEventWriteInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"},"description":{"type":"string"},"location":{"type":"string"},"startISO":{"type":"string"},"endISO":{"type":"string"},"timeZone":{"type":"string"},"isAllDay":{"type":"boolean"},"color":{"type":"string"},"people":{"oneOf":[{"type":"string"},{"type":"array","items":{"type":"string"}}]},"reminderLeadHours":{"type":"integer","enum":[1,2,3,6,12,24,48]}},"required":["title","startISO","endISO"],"additionalProperties":false}`)
}

func calendarEventListInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"startISO":{"type":"string"},"endISO":{"type":"string"},"query":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":50}},"additionalProperties":false}`)
}

func calendarEventUpdateInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"eventID":{"type":"string"},"title":{"type":"string"},"description":{"type":"string"},"location":{"type":"string"},"startISO":{"type":"string"},"endISO":{"type":"string"},"timeZone":{"type":"string"},"isAllDay":{"type":"boolean"},"color":{"type":"string"},"people":{"oneOf":[{"type":"string"},{"type":"array","items":{"type":"string"}}]},"reminderLeadHours":{"type":"integer","enum":[1,2,3,6,12,24,48]}},"required":["eventID","title","startISO","endISO"],"additionalProperties":false}`)
}

func calendarEventDeleteInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"eventID":{"type":"string"}},"required":["eventID"],"additionalProperties":false}`)
}

func mailMessageListInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"mailbox":{"type":"string"},"query":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":100},"cursor":{"type":"string"}},"additionalProperties":false}`)
}

func mailMessageReadInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"mailbox":{"type":"string"},"uid":{"type":"string"}},"required":["mailbox","uid"],"additionalProperties":false}`)
}

func mailMessageSendInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"to":{"type":"array","items":{"type":"string"}},"cc":{"type":"array","items":{"type":"string"}},"bcc":{"type":"array","items":{"type":"string"}},"subject":{"type":"string"},"body":{"type":"string"}},"required":["to","subject","body"],"additionalProperties":false}`)
}

func mailMessageMoveInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"mailbox":{"type":"string"},"uid":{"type":"string"},"targetMailbox":{"type":"string"}},"required":["mailbox","uid","targetMailbox"],"additionalProperties":false}`)
}

func mailMessageMarkInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"mailbox":{"type":"string"},"uid":{"type":"string"},"seen":{"type":"boolean"},"flagged":{"type":"boolean"}},"required":["mailbox","uid"],"additionalProperties":false}`)
}

func siteAppCreateInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"slug":{"type":"string"},"title":{"type":"string"}},"required":["slug"],"additionalProperties":false}`)
}

func siteAppPublishInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"siteID":{"type":"string"},"slug":{"type":"string"},"title":{"type":"string"},"visibility":{"type":"string"},"message":{"type":"string"}},"additionalProperties":false}`)
}

func siteAppLookupInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"siteID":{"type":"string"},"slug":{"type":"string"}},"additionalProperties":false}`)
}

func siteAppLifecycleInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"siteID":{"type":"string"},"slug":{"type":"string"},"reason":{"type":"string"},"confirm":{"type":"string"},"userConfirmed":{"type":"boolean"}},"additionalProperties":false}`)
}

func siteAppDeleteInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"siteID":{"type":"string"},"slug":{"type":"string"},"reason":{"type":"string"},"confirm":{"type":"string"},"userConfirmed":{"type":"boolean"}},"required":["confirm","userConfirmed"],"additionalProperties":false}`)
}

func googleDocsCreateInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"},"body":{"type":"string"}},"required":["title"],"additionalProperties":false}`)
}

func googleSheetsCreateInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"},"sheets":{"type":"array","items":{"type":"string"}},"values":{"type":"array","items":{"type":"array"}}},"required":["title"],"additionalProperties":false}`)
}

func googleGmailSendInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"to":{"type":"array","items":{"type":"string"}},"subject":{"type":"string"},"body":{"type":"string"},"cc":{"type":"array","items":{"type":"string"}},"bcc":{"type":"array","items":{"type":"string"}}},"required":["to","subject","body"],"additionalProperties":false}`)
}

func googleCalendarEventInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"},"start":{"type":"string"},"end":{"type":"string"},"attendees":{"type":"array","items":{"type":"string"}},"description":{"type":"string"},"location":{"type":"string"}},"required":["title","start","end"],"additionalProperties":false}`)
}

func googleCalendarListInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"start":{"type":"string"},"end":{"type":"string"},"limit":{"type":"integer"},"query":{"type":"string"}},"additionalProperties":false}`)
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
	toolNames := []string{}
	for _, descriptor := range DefaultToolDescriptors() {
		toolNames = append(toolNames, descriptor.Name)
	}
	return toolNames
}

func DefaultToolDescriptors() []Descriptor {
	descriptors := CompanionToolDescriptors()
	descriptors = append(descriptors, FlowDescriptors()...)
	descriptors = append(descriptors, CalendarDescriptors()...)
	descriptors = append(descriptors, MailDescriptors()...)
	descriptors = append(descriptors, SiteAppDescriptors()...)
	return descriptors
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
