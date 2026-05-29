package capabilities

import (
	"encoding/json"
	"strings"

	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol/jsonschema"
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
	descriptors := []Descriptor{
		{Name: "llm.text", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true},
		{Name: "llm.structured", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true},
		{Name: "embedding.create", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true},
		{Name: "platform.reply", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false},
	}
	descriptors = append(descriptors, DeviceBrowserDescriptors()...)
	descriptors = append(descriptors, WebDescriptors()...)
	descriptors = append(descriptors, FileDescriptors()...)
	descriptors = append(descriptors, PlatformMessageDescriptors()...)
	descriptors = append(descriptors, MattermostDescriptors()...)
	descriptors = append(descriptors, FlowDescriptors()...)
	descriptors = append(descriptors, CalendarDescriptors()...)
	descriptors = append(descriptors, MailDescriptors()...)
	descriptors = append(descriptors, SiteAppDescriptors()...)
	return descriptors
}

func WebDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "web.search", Version: "1", PrivacyClass: "public_web", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: webSearchInputSchema(), PolicyResource: "tool:web.search", SideEffectClass: "read"},
		{Name: "web.fetch", Version: "1", PrivacyClass: "public_web", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: webFetchInputSchema(), PolicyResource: "tool:web.fetch", SideEffectClass: "read"},
	}
}

func FileDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "file.read", Version: "1", PrivacyClass: "workspace_document", EstimatedLatency: "high", RequiresUserPresence: false, WorksOffline: false, InputSchema: fileReadInputSchema(), PolicyResource: "tool:file.read", SideEffectClass: "read"},
	}
}

func PlatformMessageDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "platform.dm.send", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformDMSendInputSchema(), PolicyResource: "tool:platform.dm.send", SideEffectClass: "external_send", RequiresApproval: true},
		{Name: "platform.dm.inspect", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformDMInspectInputSchema(), PolicyResource: "tool:platform.dm.send", SideEffectClass: "read"},
	}
}

func MattermostDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "mattermost.channel.posts.list", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: mattermostChannelPostsListInputSchema(), PolicyResource: "tool:mattermost.channel.posts.list", SideEffectClass: "read"},
		{Name: "mattermost.channel.post", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mattermostChannelPostInputSchema(), PolicyResource: "tool:mattermost.channel.post", SideEffectClass: "external_send", RequiresApproval: true},
		{Name: "mattermost.post.update", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mattermostPostUpdateInputSchema(), PolicyResource: "tool:mattermost.post.update", SideEffectClass: "external_write", RequiresApproval: true},
		{Name: "mattermost.post.delete", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mattermostPostDeleteInputSchema(), PolicyResource: "tool:mattermost.post.delete", SideEffectClass: "destructive", RequiresApproval: true},
		{Name: "mattermost.channel.update", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mattermostChannelUpdateInputSchema(), PolicyResource: "tool:mattermost.channel.update", SideEffectClass: "external_write", RequiresApproval: true},
	}
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
		{Name: "mail.message.search", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageSearchInputSchema(), PolicyResource: "tool:mail.message.search", SideEffectClass: "read"},
		{Name: "mail.message.read", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageReadInputSchema(), PolicyResource: "tool:mail.message.read", SideEffectClass: "read"},
		{Name: "mail.message.send", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageSendInputSchema(), PolicyResource: "tool:mail.message.send", SideEffectClass: "external_send", RequiresApproval: true},
		{Name: "mail.message.move", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageMoveInputSchema(), PolicyResource: "tool:mail.message.move", SideEffectClass: "workspace_write"},
		{Name: "mail.message.mark", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageMarkInputSchema(), PolicyResource: "tool:mail.message.mark", SideEffectClass: "workspace_write"},
	}
}

func SiteAppDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "site.app.create", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppCreateInputSchema(), PolicyResource: "tool:site.app.create", SideEffectClass: "workspace_write"},
		{Name: "site.app.publish", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "high", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppPublishInputSchema(), PolicyResource: "tool:site.app.publish", SideEffectClass: "site_publish"},
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
		{Name: "google.drive.import_pptx", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleDriveImportPPTXInputSchema(), PolicyResource: "tool:google.drive.import_pptx", SideEffectClass: "external_write"},
	}
}

func flowTaskAddInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("prompt", jsonschema.String()),
		jsonschema.Field("targetPersonHint", jsonschema.String()),
		jsonschema.Field("weekCode", jsonschema.String()),
		jsonschema.Field("allowDuplicate", jsonschema.Boolean()),
	).RawMessage()
}

func webSearchInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("query", jsonschema.String()),
		jsonschema.Field("location", jsonschema.String()),
		jsonschema.Field("language", jsonschema.String()),
		jsonschema.Field("limit", jsonschema.Integer()),
		jsonschema.Field("allowedDomains", jsonschema.Array(jsonschema.String())),
		jsonschema.Field("excludedDomains", jsonschema.Array(jsonschema.String())),
	).RawMessage()
}

func webFetchInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("urls", jsonschema.Array(jsonschema.String())),
		jsonschema.Field("maxContentTokens", jsonschema.Integer()),
		jsonschema.Field("allowedDomains", jsonschema.Array(jsonschema.String())),
		jsonschema.Field("blockedDomains", jsonschema.Array(jsonschema.String())),
	).RawMessage()
}

func fileReadInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("path", jsonschema.String()),
		jsonschema.Field("ocrMode", jsonschema.StringEnum("auto", "always", "never")),
		jsonschema.Field("maxPages", jsonschema.Integer()),
		jsonschema.Field("maxOutputBytes", jsonschema.Integer()),
	).RawMessage()
}

func platformDMSendInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("recipientHint", jsonschema.String()),
		jsonschema.Required("message", jsonschema.String()),
		jsonschema.Field("platform", jsonschema.StringEnum("mattermost")),
		jsonschema.Field("reason", jsonschema.String()),
	).RawMessage()
}

func platformDMInspectInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("recipientHint", jsonschema.String()),
		jsonschema.Field("platform", jsonschema.StringEnum("mattermost")),
	).RawMessage()
}

func mattermostChannelPostInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("channelID", jsonschema.String()),
		jsonschema.Field("channelName", jsonschema.String()),
		jsonschema.Required("message", jsonschema.String()),
		jsonschema.Field("pin", jsonschema.Boolean()),
	).RawMessage()
}

func mattermostChannelUpdateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("channelID", jsonschema.String()),
		jsonschema.Field("channelName", jsonschema.String()),
		jsonschema.Field("header", jsonschema.String()),
		jsonschema.Field("displayName", jsonschema.String()),
		jsonschema.Field("inviteeHints", jsonschema.Array(jsonschema.String())),
	).RawMessage()
}

func mattermostChannelPostsListInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("channelID", jsonschema.String()),
		jsonschema.Field("channelName", jsonschema.String()),
		jsonschema.Field("page", jsonschema.Integer()),
		jsonschema.Field("perPage", jsonschema.Integer()),
	).RawMessage()
}

func mattermostPostUpdateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("postID", jsonschema.String()),
		jsonschema.Field("message", jsonschema.String()),
		jsonschema.Field("isPinned", jsonschema.Raw(json.RawMessage(`{"type":["boolean","null"]}`))),
	).RawMessage()
}

func mattermostPostDeleteInputSchema() json.RawMessage {
	return jsonschema.Object(jsonschema.Required("postID", jsonschema.String())).RawMessage()
}

func calendarEventWriteInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("title", jsonschema.String()),
		jsonschema.Field("description", jsonschema.String()),
		jsonschema.Field("location", jsonschema.String()),
		jsonschema.Required("startISO", jsonschema.String()),
		jsonschema.Required("endISO", jsonschema.String()),
		jsonschema.Field("timeZone", jsonschema.String()),
		jsonschema.Field("isAllDay", jsonschema.Boolean()),
		jsonschema.Field("color", jsonschema.String()),
		jsonschema.Field("people", jsonschema.Array(jsonschema.String())),
		jsonschema.Field("reminderLeadHours", jsonschema.IntegerEnum(1, 2, 3, 6, 12, 24, 48)),
	).RawMessage()
}

func calendarEventListInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("startISO", jsonschema.String()),
		jsonschema.Field("endISO", jsonschema.String()),
		jsonschema.Field("query", jsonschema.String()),
		jsonschema.Field("limit", jsonschema.Integer()),
	).RawMessage()
}

func calendarEventUpdateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("eventID", jsonschema.String()),
		jsonschema.Required("title", jsonschema.String()),
		jsonschema.Field("description", jsonschema.String()),
		jsonschema.Field("location", jsonschema.String()),
		jsonschema.Required("startISO", jsonschema.String()),
		jsonschema.Required("endISO", jsonschema.String()),
		jsonschema.Field("timeZone", jsonschema.String()),
		jsonschema.Field("isAllDay", jsonschema.Boolean()),
		jsonschema.Field("color", jsonschema.String()),
		jsonschema.Field("people", jsonschema.Array(jsonschema.String())),
		jsonschema.Field("reminderLeadHours", jsonschema.IntegerEnum(1, 2, 3, 6, 12, 24, 48)),
	).RawMessage()
}

func calendarEventDeleteInputSchema() json.RawMessage {
	return jsonschema.Object(jsonschema.Required("eventID", jsonschema.String())).RawMessage()
}

func mailMessageListInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"mailbox":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":50},"cursor":{"type":"string"}},"additionalProperties":false}`)
}

func mailMessageSearchInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"mailbox":{"type":"string"},"query":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":50},"cursor":{"type":"string"}},"required":["query"],"additionalProperties":false}`)
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
	return jsonschema.Object(
		jsonschema.Required("slug", jsonschema.String()),
		jsonschema.Field("title", jsonschema.String()),
		jsonschema.Field("prompt", jsonschema.String()),
		jsonschema.Field("designBrief", jsonschema.String()),
		jsonschema.Field("prototypeScope", jsonschema.String()),
	).RawMessage()
}

func siteAppPublishInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("siteID", jsonschema.String()),
		jsonschema.Field("slug", jsonschema.String()),
		jsonschema.Field("title", jsonschema.String()),
		jsonschema.Field("visibility", jsonschema.String()),
		jsonschema.Field("message", jsonschema.String()),
		jsonschema.Field("sourceWorkspacePath", jsonschema.String()),
		jsonschema.Field("appWorkspacePath", jsonschema.String()),
	).RawMessage()
}

func siteAppLookupInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("siteID", jsonschema.String()),
		jsonschema.Field("slug", jsonschema.String()),
	).RawMessage()
}

func siteAppLifecycleInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("siteID", jsonschema.String()),
		jsonschema.Field("slug", jsonschema.String()),
		jsonschema.Field("reason", jsonschema.String()),
		jsonschema.Field("confirm", jsonschema.String()),
		jsonschema.Field("userConfirmed", jsonschema.Boolean()),
	).RawMessage()
}

func siteAppDeleteInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("siteID", jsonschema.String()),
		jsonschema.Field("slug", jsonschema.String()),
		jsonschema.Field("reason", jsonschema.String()),
		jsonschema.Required("confirm", jsonschema.String()),
		jsonschema.Required("userConfirmed", jsonschema.Boolean()),
	).RawMessage()
}

func googleDocsCreateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("title", jsonschema.String()),
		jsonschema.Field("body", jsonschema.String()),
	).RawMessage()
}

func googleSheetsCreateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("title", jsonschema.String()),
		jsonschema.Field("sheets", jsonschema.Array(jsonschema.String())),
		jsonschema.Field("values", jsonschema.Array(jsonschema.Array(jsonschema.String()))),
	).RawMessage()
}

func googleGmailSendInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("to", jsonschema.Array(jsonschema.String())),
		jsonschema.Required("subject", jsonschema.String()),
		jsonschema.Required("body", jsonschema.String()),
		jsonschema.Field("cc", jsonschema.Array(jsonschema.String())),
		jsonschema.Field("bcc", jsonschema.Array(jsonschema.String())),
	).RawMessage()
}

func googleCalendarEventInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("title", jsonschema.String()),
		jsonschema.Required("start", jsonschema.String()),
		jsonschema.Required("end", jsonschema.String()),
		jsonschema.Field("attendees", jsonschema.Array(jsonschema.String())),
		jsonschema.Field("description", jsonschema.String()),
		jsonschema.Field("location", jsonschema.String()),
	).RawMessage()
}

func googleCalendarListInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("start", jsonschema.String()),
		jsonschema.Field("end", jsonschema.String()),
		jsonschema.Field("limit", jsonschema.Integer()),
		jsonschema.Field("query", jsonschema.String()),
	).RawMessage()
}

func googleDriveImportPPTXInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("path", jsonschema.String()),
		jsonschema.Field("title", jsonschema.String()),
	).RawMessage()
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
	descriptors = append(descriptors, WebDescriptors()...)
	descriptors = append(descriptors, FileDescriptors()...)
	descriptors = append(descriptors, PlatformMessageDescriptors()...)
	descriptors = append(descriptors, MattermostDescriptors()...)
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
