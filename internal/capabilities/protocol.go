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
type CompletionEvidenceDescriptor = capabilityprotocol.CompletionEvidenceDescriptor
type RegistryResponse = capabilityprotocol.RegistryResponse
type ToolInvokeRequest = capabilityprotocol.ToolInvokeRequest
type ToolInvokeContext = capabilityprotocol.ToolInvokeContext
type ActorContext = capabilityprotocol.ActorContext
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
		{Name: "document.read", Version: "1", PrivacyClass: "workspace_document", EstimatedLatency: "high", RequiresUserPresence: false, WorksOffline: false, InputSchema: documentReadInputSchema(), PolicyResource: "tool:document.read", SideEffectClass: "read"},
		{Name: "image.read", Version: "1", PrivacyClass: "workspace_document", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true, InputSchema: imageReadInputSchema(), PolicyResource: "tool:image.read", SideEffectClass: "read"},
	}
}

func PlatformMessageDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "platform.message.context", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformMessageContextInputSchema(), PolicyResource: "tool:platform.message.context", SideEffectClass: "read"},
		{Name: "platform.message.search", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformMessageSearchInputSchema(), PolicyResource: "tool:platform.message.search", SideEffectClass: "read"},
		{Name: "platform.message.send", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformMessageSendInputSchema(), PolicyResource: "tool:platform.message.send", SideEffectClass: "external_send", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "send_message", "message")},
		{Name: "platform.message.update", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformMessageUpdateInputSchema(), PolicyResource: "tool:platform.message.update", SideEffectClass: "external_write", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "update_message", "message")},
		{Name: "platform.message.delete", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformMessageDeleteInputSchema(), PolicyResource: "tool:platform.message.delete", SideEffectClass: "destructive", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "delete_message", "message")},
	}
}

func MattermostDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "mattermost.channel.update", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mattermostChannelUpdateInputSchema(), PolicyResource: "tool:mattermost.channel.update", SideEffectClass: "external_write", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "update_channel", "channel")},
	}
}

func FlowDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "flow.task.add", Version: "1", PrivacyClass: "workspace_task", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: flowTaskAddInputSchema(), PolicyResource: "tool:flow.task.add", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_task", "task")},
		{Name: "flow.task.list", Version: "1", PrivacyClass: "workspace_task", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: flowTaskListInputSchema(), PolicyResource: "tool:flow.task.list", SideEffectClass: "read"},
		{Name: "flow.task.update", Version: "1", PrivacyClass: "workspace_task", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: flowTaskUpdateInputSchema(), PolicyResource: "tool:flow.task.update", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_task", "task")},
		{Name: "flow.task.delete", Version: "1", PrivacyClass: "workspace_task", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: flowTaskDeleteInputSchema(), PolicyResource: "tool:flow.task.delete", SideEffectClass: "destructive", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "delete_task", "task")},
	}
}

func CalendarDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "calendar.connection.status", Version: "1", PrivacyClass: "workspace_calendar", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: emptyInputSchema(), PolicyResource: "tool:calendar.connection.status", SideEffectClass: "read"},
		{Name: "calendar.connection.start", Version: "1", PrivacyClass: "workspace_calendar", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false, InputSchema: emptyInputSchema(), PolicyResource: "tool:calendar.connection.start", SideEffectClass: "connect"},
		{Name: "calendar.event.add", Version: "1", PrivacyClass: "workspace_calendar", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: calendarEventWriteInputSchema(), PolicyResource: "tool:calendar.event.add", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_calendar", "calendar")},
		{Name: "calendar.event.list", Version: "1", PrivacyClass: "workspace_calendar", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: calendarEventListInputSchema(), PolicyResource: "tool:calendar.event.list", SideEffectClass: "read"},
		{Name: "calendar.event.update", Version: "1", PrivacyClass: "workspace_calendar", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: calendarEventUpdateInputSchema(), PolicyResource: "tool:calendar.event.update", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_calendar", "calendar")},
		{Name: "calendar.event.delete", Version: "1", PrivacyClass: "workspace_calendar", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: calendarEventDeleteInputSchema(), PolicyResource: "tool:calendar.event.delete", SideEffectClass: "destructive", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "write_calendar", "calendar")},
	}
}

func MailDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "mail.connection.status", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: emptyInputSchema(), PolicyResource: "tool:mail.connection.status", SideEffectClass: "read"},
		{Name: "mail.connection.start", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false, InputSchema: emptyInputSchema(), PolicyResource: "tool:mail.connection.start", SideEffectClass: "connect", RequiresApproval: true},
		{Name: "mail.message.list", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageListInputSchema(), PolicyResource: "tool:mail.message.list", SideEffectClass: "read"},
		{Name: "mail.message.search", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageSearchInputSchema(), PolicyResource: "tool:mail.message.search", SideEffectClass: "read"},
		{Name: "mail.message.read", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageReadInputSchema(), PolicyResource: "tool:mail.message.read", SideEffectClass: "read"},
		{Name: "mail.message.send", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageSendInputSchema(), PolicyResource: "tool:mail.message.send", SideEffectClass: "external_send", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "send_email", "email")},
		{Name: "mail.message.move", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageMoveInputSchema(), PolicyResource: "tool:mail.message.move", SideEffectClass: "workspace_write"},
		{Name: "mail.message.mark", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageMarkInputSchema(), PolicyResource: "tool:mail.message.mark", SideEffectClass: "workspace_write"},
	}
}

func SiteAppDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "site.app.create", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppCreateInputSchema(), PolicyResource: "tool:site.app.create", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "create_site", "site")},
		{Name: "site.app.preview", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "high", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppPublishInputSchema(), PolicyResource: "tool:site.app.preview", SideEffectClass: "external_publish"},
		{Name: "site.app.publish", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "high", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppPublishInputSchema(), PolicyResource: "tool:site.app.publish", SideEffectClass: "site_publish", CompletionEvidence: completionEvidence("success", "publish_site", "site")},
		{Name: "site.app.status", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLookupInputSchema(), PolicyResource: "tool:site.app.status", SideEffectClass: "read"},
		{Name: "site.app.history", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLookupInputSchema(), PolicyResource: "tool:site.app.history", SideEffectClass: "read"},
		{Name: "site.app.diff", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppDiffInputSchema(), PolicyResource: "tool:site.app.diff", SideEffectClass: "read"},
		{Name: "site.app.logs", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLookupInputSchema(), PolicyResource: "tool:site.app.logs", SideEffectClass: "read"},
		{Name: "site.app.rollback", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLifecycleInputSchema(), PolicyResource: "tool:site.app.rollback", SideEffectClass: "external_publish", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "publish_site", "site")},
		{Name: "site.app.unpublish", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLifecycleInputSchema(), PolicyResource: "tool:site.app.unpublish", SideEffectClass: "external_publish", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "publish_site", "site")},
		{Name: "site.app.restore", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLifecycleInputSchema(), PolicyResource: "tool:site.app.restore", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "publish_site", "site")},
		{Name: "site.app.delete", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppDeleteInputSchema(), PolicyResource: "tool:site.app.delete", SideEffectClass: "destructive", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "publish_site", "site")},
	}
}

func ArtifactDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "artifact.review", Version: "1", PrivacyClass: "workspace_document", EstimatedLatency: "high", RequiresUserPresence: false, WorksOffline: false, InputSchema: artifactReviewInputSchema(), PolicyResource: "tool:artifact.review", SideEffectClass: "read"},
	}
}

func GoogleWorkspaceDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "google.docs.create", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleDocsCreateInputSchema(), PolicyResource: "tool:google.docs.create", SideEffectClass: "external_write"},
		{Name: "google.sheets.create", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleSheetsCreateInputSchema(), PolicyResource: "tool:google.sheets.create", SideEffectClass: "external_write"},
		{Name: "google.gmail.send", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleGmailSendInputSchema(), PolicyResource: "tool:google.gmail.send", SideEffectClass: "external_send", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "send_email", "email")},
		{Name: "google.calendar.event", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleCalendarEventInputSchema(), PolicyResource: "tool:google.calendar.event", SideEffectClass: "external_write", CompletionEvidence: completionEvidence("success", "write_calendar", "calendar")},
		{Name: "google.calendar.list", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleCalendarListInputSchema(), PolicyResource: "tool:google.calendar.list", SideEffectClass: "read"},
		{Name: "google.drive.import_pptx", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleDriveImportPPTXInputSchema(), PolicyResource: "tool:google.drive.import_pptx", SideEffectClass: "external_write"},
	}
}

func completionEvidence(mode string, action string, targetKind string) *CompletionEvidenceDescriptor {
	return &CompletionEvidenceDescriptor{Mode: mode, Action: action, TargetKind: targetKind}
}

func flowTaskAddInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("prompt", jsonschema.String()),
		jsonschema.Field("targetPersonHint", jsonschema.String()),
		jsonschema.Field("weekCode", jsonschema.String()),
		jsonschema.Field("allowDuplicate", jsonschema.Boolean()),
	).RawMessage()
}

func flowTaskListInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("query", jsonschema.String()),
		jsonschema.Field("targetPersonHint", jsonschema.String()),
		jsonschema.Field("weekCode", jsonschema.String()),
		jsonschema.Field("status", jsonschema.String()),
		jsonschema.Field("limit", jsonschema.Integer()),
	).RawMessage()
}

func flowTaskUpdateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("taskID", jsonschema.String()),
		jsonschema.Field("query", jsonschema.String()),
		jsonschema.Field("targetPersonHint", jsonschema.String()),
		jsonschema.Field("weekCode", jsonschema.String()),
		jsonschema.Field("content", jsonschema.String()),
		jsonschema.Field("goal", jsonschema.String()),
		jsonschema.Field("status", jsonschema.String()),
		jsonschema.Field("size", jsonschema.String()),
		jsonschema.Field("category", jsonschema.String()),
		jsonschema.Field("type", jsonschema.String()),
		jsonschema.Field("startDate", jsonschema.String()),
		jsonschema.Field("endDate", jsonschema.String()),
		jsonschema.Field("flag", jsonschema.Integer()),
		jsonschema.Field("requestReason", jsonschema.String()),
		jsonschema.Field("decisionReason", jsonschema.String()),
	).RawMessage()
}

func flowTaskDeleteInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("taskID", jsonschema.String()),
		jsonschema.Field("query", jsonschema.String()),
		jsonschema.Field("targetPersonHint", jsonschema.String()),
		jsonschema.Field("weekCode", jsonschema.String()),
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

func documentReadInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("materialID", jsonschema.String()),
		jsonschema.Field("path", jsonschema.String()),
		jsonschema.Field("maxPages", jsonschema.Integer()),
		jsonschema.Field("maxOutputBytes", jsonschema.Integer()),
	).RawMessage()
}

func imageReadInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("materialID", jsonschema.String()),
		jsonschema.Field("path", jsonschema.String()),
	).RawMessage()
}

func platformMessageDeliveryTargetSchema() jsonschema.Schema {
	return jsonschema.Object(
		jsonschema.Required("type", jsonschema.StringEnum("directMessage", "currentThread", "currentChannel", "channel")),
		jsonschema.Field("personHint", jsonschema.String()),
		jsonschema.Field("personHints", jsonschema.Array(jsonschema.String()).WithDescription("Send the same directMessage to several people at once. When set, this takes precedence over personHint; the tool fans out with one approval and returns a per-recipient delivery rollup.")),
		jsonschema.Field("channelID", jsonschema.String()),
		jsonschema.Field("channelName", jsonschema.String()),
	)
}

func platformMessageContextInputSchema() json.RawMessage {
	return jsonschema.Object().RawMessage()
}

func platformMessageSearchInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("scope", jsonschema.StringEnum("currentThread", "currentChannel", "directMessage", "channel")),
		jsonschema.Field("deliveryTarget", platformMessageDeliveryTargetSchema()),
		jsonschema.Field("authoredBy", jsonschema.StringEnum("assistant", "requester", "anyone")),
		jsonschema.Field("queries", jsonschema.Array(jsonschema.String())),
		jsonschema.Field("limit", jsonschema.Integer()),
		jsonschema.Field("cursor", jsonschema.String()),
	).RawMessage()
}

func platformMessageSendInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("deliveryTarget", platformMessageDeliveryTargetSchema()),
		jsonschema.Field("recipientHint", jsonschema.String()),
		jsonschema.Required("message", jsonschema.String()),
		jsonschema.Field("pin", jsonschema.Boolean()),
		jsonschema.Field("reason", jsonschema.String()),
	).RawMessage()
}

func platformMessageUpdateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("messageID", jsonschema.String()),
		jsonschema.Field("message", jsonschema.String()),
		jsonschema.Field("isPinned", jsonschema.Boolean()),
	).RawMessage()
}

func platformMessageDeleteInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("messageIDs", jsonschema.Array(jsonschema.String())),
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

func mattermostContextInspectInputSchema() json.RawMessage {
	return jsonschema.Object().RawMessage()
}

func mattermostPostSearchInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("scope", jsonschema.String()),
		jsonschema.Field("channelID", jsonschema.String()),
		jsonschema.Field("channelName", jsonschema.String()),
		jsonschema.Field("personHint", jsonschema.String()),
		jsonschema.Field("rootPostID", jsonschema.String()),
		jsonschema.Field("authoredBy", jsonschema.String()),
		jsonschema.Field("queries", jsonschema.Array(jsonschema.String())),
		jsonschema.Field("limit", jsonschema.Integer()),
	).RawMessage()
}

func mattermostPostUpdateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("postID", jsonschema.String()),
		jsonschema.Field("message", jsonschema.String()),
		jsonschema.Field("isPinned", jsonschema.Boolean()),
	).RawMessage()
}

func mattermostPostDeleteInputSchema() json.RawMessage {
	return jsonschema.Object(jsonschema.Required("postIDs", jsonschema.Array(jsonschema.String()))).RawMessage()
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
		jsonschema.Field("reminderLeadHours", jsonschema.Integer()),
	).RawMessage()
}

func emptyInputSchema() json.RawMessage {
	return jsonschema.Object().RawMessage()
}

func calendarEventListInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("startISO", jsonschema.String().WithDescription("Inclusive start of the time window to list, as ISO 8601 with timezone, e.g. 2026-06-23T00:00:00+09:00. Resolve relative ranges like 오늘/이번 주/this week to concrete dates yourself before calling.")),
		jsonschema.Field("endISO", jsonschema.String().WithDescription("Exclusive end of the time window, as ISO 8601 with timezone. Pair with startISO to bound the listing; for a single day use the next day at 00:00.")),
		jsonschema.Field("query", jsonschema.String().WithDescription("Optional free-text filter matched against event titles. Do NOT put a date or date range here — the time window goes in startISO/endISO. Leave empty to list everything in the window.")),
		jsonschema.Field("limit", jsonschema.Integer().WithDescription("Optional maximum number of events to return.")),
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
		jsonschema.Field("reminderLeadHours", jsonschema.Integer()),
	).RawMessage()
}

func calendarEventDeleteInputSchema() json.RawMessage {
	return jsonschema.Object(jsonschema.Required("eventID", jsonschema.String())).RawMessage()
}

func mailMessageListInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"mailbox":{"type":"string"},"limit":{"type":"number"},"cursor":{"type":"string"}}}`)
}

func mailMessageSearchInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"mailbox":{"type":"string"},"query":{"type":"string"},"limit":{"type":"number"},"cursor":{"type":"string"}},"required":["query"]}`)
}

func mailMessageReadInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"mailbox":{"type":"string"},"uid":{"type":"string"}},"required":["mailbox","uid"]}`)
}

func mailMessageSendInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"to":{"type":"array","items":{"type":"string"}},"cc":{"type":"array","items":{"type":"string"}},"bcc":{"type":"array","items":{"type":"string"}},"subject":{"type":"string"},"body":{"type":"string"}},"required":["to","subject","body"]}`)
}

func mailMessageMoveInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"mailbox":{"type":"string"},"uid":{"type":"string"},"targetMailbox":{"type":"string"}},"required":["mailbox","uid","targetMailbox"]}`)
}

func mailMessageMarkInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"mailbox":{"type":"string"},"uid":{"type":"string"},"seen":{"type":"boolean"},"flagged":{"type":"boolean"}},"required":["mailbox","uid"]}`)
}

func siteAppCreateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("slug", jsonschema.String()),
		jsonschema.Field("title", jsonschema.String()),
		jsonschema.Field("prompt", jsonschema.String()),
		jsonschema.Field("designBrief", jsonschema.String()),
		jsonschema.Field("prototypeScope", jsonschema.String()),
		jsonschema.Field("description", jsonschema.String()),
		jsonschema.Field("idea", jsonschema.String()),
		jsonschema.Field("purpose", jsonschema.String()),
		jsonschema.Field("audience", jsonschema.String()),
		jsonschema.Field("archetype", jsonschema.String()),
		jsonschema.Field("domainKeywords", jsonschema.Array(jsonschema.String())),
	).RawMessage()
}

func siteAppPublishInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("siteID", jsonschema.String()),
		jsonschema.Field("slug", jsonschema.String()),
		jsonschema.Field("title", jsonschema.String()),
		jsonschema.Field("visibility", jsonschema.String()),
		jsonschema.Field("description", jsonschema.String()),
		jsonschema.Field("idea", jsonschema.String()),
		jsonschema.Field("purpose", jsonschema.String()),
		jsonschema.Field("audience", jsonschema.String()),
		jsonschema.Field("archetype", jsonschema.String()),
		jsonschema.Field("domainKeywords", jsonschema.Array(jsonschema.String())),
		jsonschema.Field("message", jsonschema.String()),
		jsonschema.Field("sourceWorkspacePath", jsonschema.String()),
		jsonschema.Field("appWorkspacePath", jsonschema.String()),
	).RawMessage()
}

func siteAppLookupInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("siteID", jsonschema.String()),
		jsonschema.Field("slug", jsonschema.String()),
		jsonschema.Field("scope", jsonschema.StringEnum("conversation", "mine")),
		jsonschema.Field("checkLive", jsonschema.Boolean()),
	).RawMessage()
}

func siteAppDiffInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("siteID", jsonschema.String()),
		jsonschema.Field("slug", jsonschema.String()),
		jsonschema.Field("fromRevision", jsonschema.String()),
		jsonschema.Field("toRevision", jsonschema.String()),
	).RawMessage()
}

func siteAppLifecycleInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("siteID", jsonschema.String()),
		jsonschema.Field("slug", jsonschema.String()),
		jsonschema.Field("reason", jsonschema.String()),
		jsonschema.Field("revision", jsonschema.String()),
		jsonschema.Field("confirm", jsonschema.String()),
		jsonschema.Field("userConfirmed", jsonschema.Boolean()),
	).RawMessage()
}

func artifactReviewInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"artifactKind":{"type":"string","enum":["site","slides","pptx","docx","pdf"]},"intent":{"type":"string"},"rubric":{"type":"string"},"evidence":{"type":"array","items":{"type":"object","properties":{"role":{"type":"string"},"path":{"type":"string"},"mimeType":{"type":"string","enum":["image/png","image/jpeg"]},"label":{"type":"string"}},"required":["role","path","mimeType","label"]}},"expectedText":{"type":"array","items":{"type":"object","properties":{"target":{"type":"string"},"text":{"type":"string"}},"required":["target","text"]}},"previousIssues":{"type":"array","items":{"type":"object","properties":{"severity":{"type":"string"},"category":{"type":"string"},"target":{"type":"string"},"message":{"type":"string"},"suggestedFix":{"type":"string"}},"required":["severity","category","target","message","suggestedFix"]}}},"required":["artifactKind","intent","rubric","evidence"]}`)
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
	descriptors = append(descriptors, ArtifactDescriptors()...)
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
