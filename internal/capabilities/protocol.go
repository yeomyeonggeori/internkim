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
		{Name: "llm.text", Description: "Generate free-form text using the device LLM. Internal capability used by Blueclaw's LLM backend; not directly called by the agent loop.", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true},
		{Name: "llm.structured", Description: "Generate structured JSON output using the device LLM. Internal capability used by Blueclaw's LLM backend for structured extraction; not directly called by the agent loop.", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true},
		{Name: "embedding.create", Description: "Generate vector embeddings for text using the device embedding model. Internal capability used for semantic search and memory retrieval; not directly called by the agent loop.", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true},
		{Name: "platform.reply", Description: "Send a reply in the current platform conversation context and return delivery evidence such as visibility and native attachment count. Internal shorthand used by the platform reply path; use message.send for explicit delivery targeting.", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, SideEffectClass: "platform_reply", CompletionEvidence: completionEvidence("success", "send_reply", "message")},
	}
	descriptors = append(descriptors, WebDescriptors()...)
	descriptors = append(descriptors, FileDescriptors()...)
	descriptors = append(descriptors, PlatformMessageDescriptors()...)
	descriptors = append(descriptors, MattermostDescriptors()...)
	descriptors = append(descriptors, FlowDescriptors()...)
	descriptors = append(descriptors, CalendarDescriptors()...)
	descriptors = append(descriptors, MailDescriptors()...)
	descriptors = append(descriptors, SiteAppDescriptors()...)
	descriptors = append(descriptors, CompanyDescriptors()...)
	return descriptors
}

func CompanyDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "company.info.get", Description: "Read the company master profile (name, representative, address, contact, bank account, country-specific legal attributes such as 사업자등록번호). Pass language ('ko' or 'en') to get the view for that document language plus missingFields listing empty core fields. Call this before creating any company letterhead document; if missingFields is empty, never ask the user for company info again.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyInfoGetInputSchema(), PolicyResource: "tool:company.info.get", SideEffectClass: "read"},
		{Name: "company.info.set", Description: "Save or update the company master profile. Partial update: only provided fields are written, into the given language's slot for localized fields. Use after the user supplies company details, or when they report a change ('회사 주소 바뀌었어'). Put country-specific identifiers (사업자등록번호, 법인등록번호, 업태, 종목, EIN …) into legalAttributes as a label-to-value JSON object string.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyInfoSetInputSchema(), PolicyResource: "tool:company.info.set", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_company", "company")},
		{Name: "company.metric.record", Description: "Record or correct one company metric value for a period — annual revenue, operating profit, MAU, employee count, GMV and similar time-series numbers. Provide metric key, year, and value; add quarter (1-4) OR month (1-12) for sub-annual periods, never both. Same call overwrites the same period.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyMetricRecordInputSchema(), PolicyResource: "tool:company.metric.record", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_company", "company")},
		{Name: "company.metric.list", Description: "List recorded company metrics sorted by period. Filter by metric key and year range. Use for IR decks, business plans, and grant applications that need revenue/headcount/usage time series.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyMetricListInputSchema(), PolicyResource: "tool:company.metric.list", SideEffectClass: "read"},
		{Name: "company.record.add", Description: "Add one company history/asset record: milestones (연혁), funding rounds, products, patents, certifications, awards, client references, government grants. Set category, date, title; put structured details (amount, investors, round …) into attributes as a label-to-value JSON object string.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyRecordAddInputSchema(), PolicyResource: "tool:company.record.add", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_company", "company")},
		{Name: "company.record.list", Description: "List company history/asset records, newest first. Filter by category (history, funding, product, certification, ip, award, reference, grant …) or keyword query. Use to build 연혁 sections, funding tables, and product overviews.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyRecordListInputSchema(), PolicyResource: "tool:company.record.list", SideEffectClass: "read"},
		{Name: "company.record.update", Description: "Update fields on an existing company record identified by id from a prior company.record.list result. Only provided fields change.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyRecordUpdateInputSchema(), PolicyResource: "tool:company.record.update", SideEffectClass: "workspace_write"},
		{Name: "company.record.delete", Description: "Delete a company record by id from a prior company.record.list result. Use only when the user asks to remove a wrong entry.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyRecordDeleteInputSchema(), PolicyResource: "tool:company.record.delete", SideEffectClass: "destructive", RequiresApproval: true},
		{Name: "company.document.register", Description: "Register a company document in the document ledger and, for kind=issued, receive the official document number to print in the document plus the storage directory to save the final file in. Call BEFORE rendering an official document so the number appears in it. Always include a 2-3 sentence summary of the document's key terms (parties, amounts, dates) so later questions can be answered without re-reading the file.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyDocumentRegisterInputSchema(), PolicyResource: "tool:company.document.register", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_company", "company")},
		{Name: "company.document.list", Description: "List registered company documents newest first, with their numbers, counterparts, file paths, and summaries. Filter by type, counterpart, or keyword. Use to answer 'what quotes did we send to X'.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyDocumentListInputSchema(), PolicyResource: "tool:company.document.list", SideEffectClass: "read"},
		{Name: "company.document.search", Description: "Semantically search registered company documents by a natural-language question ('ABC와 맺은 계약 조건'). Returns best-matching documents with summaries — answer from the summary first and open the file only when detail is needed.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyDocumentSearchInputSchema(), PolicyResource: "tool:company.document.search", SideEffectClass: "read"},
		{Name: "company.document.update", Description: "Update a registered document's file path, title, counterpart, or summary by id from a prior list/search result. Use when a file was moved or renamed so the ledger keeps tracking it.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyDocumentUpdateInputSchema(), PolicyResource: "tool:company.document.update", SideEffectClass: "workspace_write"},
	}
}

func WebDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "web.search", Description: "Search the public web and return ranked result snippets. Use this when you need current information, facts, or links that are not already in context. Do not use for workspace data, calendar, mail, or tasks — those have dedicated tools.", Version: "1", PrivacyClass: "public_web", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: webSearchInputSchema(), PolicyResource: "tool:web.search", SideEffectClass: "read"},
		{Name: "web.fetch", Description: "Fetch and return the text content of one or more public URLs. Use after web.search when you need the full page content, not just a snippet. Do not fetch localhost or private network addresses — those are blocked.", Version: "1", PrivacyClass: "public_web", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: webFetchInputSchema(), PolicyResource: "tool:web.fetch", SideEffectClass: "read"},
	}
}

func FileDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "document.read", Description: "Read a workspace document (PDF, Office, HTML, text, etc.) and return its content as Markdown. Use this to read files at /workspace paths; do not run a shell command to cat files. For image files use image.read instead.", Version: "1", PrivacyClass: "workspace_document", EstimatedLatency: "high", RequiresUserPresence: false, WorksOffline: false, InputSchema: documentReadInputSchema(), PolicyResource: "tool:document.read", SideEffectClass: "read"},
		{Name: "image.read", Description: "Read a workspace image file (PNG, JPG, etc.) and return it as a base64-encoded attachment for vision analysis. Use this for image files at /workspace paths; for text/document files use document.read instead.", Version: "1", PrivacyClass: "workspace_document", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true, InputSchema: imageReadInputSchema(), PolicyResource: "tool:image.read", SideEffectClass: "read"},
		{Name: "image.generate", Description: "Generate a new image from a text prompt and save it to a workspace path. Provide an absolute /workspace output path ending in .png. Optionally set aspectRatio. Returns the saved image as an attachment. Use image.read instead if you need to read an existing image file.", Version: "1", PrivacyClass: "workspace_document", EstimatedLatency: "high", RequiresUserPresence: false, WorksOffline: false, InputSchema: imageGenerateInputSchema(), PolicyResource: "tool:image.generate", SideEffectClass: "external_write"},
	}
}

func PlatformMessageDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "message.context", Description: "Return metadata about the current Mattermost conversation context — the active channel, thread, and requester identity. Call this first when you need to know where the conversation is happening before sending or searching messages.", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformMessageContextInputSchema(), PolicyResource: "tool:message.context", SideEffectClass: "read"},
		{Name: "message.search", Description: "Search past Mattermost messages in a channel, thread, or DM conversation. Use this to find what was said, retrieve prior messages, or check history. Do not use this to send a message — use message.send.", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformMessageSearchInputSchema(), PolicyResource: "tool:message.search", SideEffectClass: "read"},
		{Name: "message.send", Description: "Send a Mattermost message to a channel, thread, or DM. Requires approval before delivering. For DM to multiple people set personHints instead of personHint to fan out with a single approval.", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformMessageSendInputSchema(), PolicyResource: "tool:message.send", SideEffectClass: "external_send", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "send_message", "message")},
		{Name: "message.update", Description: "Edit the text of an existing Mattermost message or change its pinned state. Requires the messageID from a prior search or send result — never invent an ID. Requires approval.", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformMessageUpdateInputSchema(), PolicyResource: "tool:message.update", SideEffectClass: "external_write", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "update_message", "message")},
		{Name: "message.delete", Description: "Permanently delete one or more Mattermost messages by their IDs (up to 25 at once). Requires the messageIDs from a prior search result — never invent IDs. Requires approval; this action is irreversible.", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformMessageDeleteInputSchema(), PolicyResource: "tool:message.delete", SideEffectClass: "destructive", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "delete_message", "message")},
	}
}

func MattermostDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "channel.update", Description: "Update a Mattermost channel's display name, header text, or member list. Provide channelID or channelName to identify the channel. At least one of displayName, header, or inviteeHints must be set. Requires approval.", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mattermostChannelUpdateInputSchema(), PolicyResource: "tool:channel.update", SideEffectClass: "external_write", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "update_channel", "channel")},
	}
}

func FlowDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "task.add", Description: "Create a new workspace task from a natural-language prompt. Use this to add a todo or assignment for the requester or another team member. Do not use this to update an existing task — use task.update.", Version: "1", PrivacyClass: "workspace_task", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: flowTaskAddInputSchema(), PolicyResource: "tool:task.add", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_task", "task")},
		{Name: "task.list", Description: "List workspace tasks with optional filters. Use this to answer 'what tasks does X have', 'what is on my plate', or 'show incomplete items this week'. Leave targetPersonHint empty to list all people.", Version: "1", PrivacyClass: "workspace_task", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: flowTaskListInputSchema(), PolicyResource: "tool:task.list", SideEffectClass: "read"},
		{Name: "task.update", Description: "Update fields on an existing task — title, status, dates, size, category, and more. Identify the task by taskID (from a prior list result) or by a text query. If no patch fields are provided the task is automatically marked complete.", Version: "1", PrivacyClass: "workspace_task", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: flowTaskUpdateInputSchema(), PolicyResource: "tool:task.update", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_task", "task")},
		{Name: "task.delete", Description: "Permanently delete a task. Identify the task by taskID or by query plus weekCode and targetPersonHint. Requires approval; this action is irreversible.", Version: "1", PrivacyClass: "workspace_task", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: flowTaskDeleteInputSchema(), PolicyResource: "tool:task.delete", SideEffectClass: "destructive", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "delete_task", "task")},
	}
}

func CalendarDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "calendar.add", Description: "Create a new calendar event. Provide title, startISO, and endISO at minimum. Use ISO 8601 with timezone for times, e.g. 2026-06-23T14:00:00+09:00. Do not call this to update an existing event — use calendar.update.", Version: "1", PrivacyClass: "workspace_calendar", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: calendarEventWriteInputSchema(), PolicyResource: "tool:calendar.add", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_calendar", "calendar")},
		{Name: "calendar.list", Description: "List the requester's calendar events within a time window. Use this to answer any 'what is on my calendar' question (today, this week, a date range): compute the concrete startISO/endISO window yourself and call it directly — do not run a shell command and do not ask the user for their calendar. Returns the events in the window (possibly empty).", Version: "1", PrivacyClass: "workspace_calendar", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: calendarEventListInputSchema(), PolicyResource: "tool:calendar.list", SideEffectClass: "read"},
		{Name: "calendar.update", Description: "Update an existing calendar event. Identify it by eventID from a prior calendar.list result, or set query to a distinctive keyword (a person or topic name) and the runtime finds it across all dates — never invent an eventID. All required fields (title, startISO, endISO) must be re-supplied even if unchanged.", Version: "1", PrivacyClass: "workspace_calendar", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: calendarEventUpdateInputSchema(), PolicyResource: "tool:calendar.update", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_calendar", "calendar")},
		{Name: "calendar.delete", Description: "Delete a calendar event. Identify it by eventID from a prior calendar.list result, or set query to a distinctive keyword (a person or topic name) and the runtime finds it across all dates — never invent an eventID. Requires approval; this action is irreversible.", Version: "1", PrivacyClass: "workspace_calendar", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: calendarEventDeleteInputSchema(), PolicyResource: "tool:calendar.delete", SideEffectClass: "destructive", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "write_calendar", "calendar")},
	}
}

func MailDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "mail.connection.status", Description: "Check whether the requester's email account is connected. Call this before any mail read or write operation when you are unsure if mail is set up.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: emptyInputSchema(), PolicyResource: "tool:mail.connection.status", SideEffectClass: "read"},
		{Name: "mail.connection.start", Description: "Start the email account connection flow and return setup instructions for the requester. Requires the user to be present (RequiresUserPresence=true). Only call this when mail.connection.status reports mail is not connected.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false, InputSchema: emptyInputSchema(), PolicyResource: "tool:mail.connection.start", SideEffectClass: "connect", RequiresApproval: true},
		{Name: "mail.message.list", Description: "List emails in a mailbox folder with optional pagination. Use this to browse recent messages; use mail.message.search when you need to find by keyword or subject.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageListInputSchema(), PolicyResource: "tool:mail.message.list", SideEffectClass: "read"},
		{Name: "mail.message.search", Description: "Search emails by keyword, sender, or subject. Returns matching messages with their UIDs for use with mail.message.read. Do not put pagination cursors in the query field.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageSearchInputSchema(), PolicyResource: "tool:mail.message.search", SideEffectClass: "read"},
		{Name: "mail.message.read", Description: "Fetch the full content of a single email by its mailbox name and UID. The UID must come from a prior mail.message.list or mail.message.search result — never invent a UID.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageReadInputSchema(), PolicyResource: "tool:mail.message.read", SideEffectClass: "read"},
		{Name: "mail.message.send", Description: "Send an email from the requester's connected mail account. Provide at least one recipient in 'to', a subject, and a body. Requires approval before sending.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageSendInputSchema(), PolicyResource: "tool:mail.message.send", SideEffectClass: "external_send", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "send_email", "email")},
		{Name: "mail.message.move", Description: "Move an email to a different mailbox folder (e.g. Archive, Trash). The UID and mailbox must come from a prior list or search result. Use this to archive or sort messages.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageMoveInputSchema(), PolicyResource: "tool:mail.message.move", SideEffectClass: "workspace_write"},
		{Name: "mail.message.mark", Description: "Set the read (seen) or starred (flagged) status of an email. The UID and mailbox must come from a prior list or search result. Omit a flag field to leave it unchanged.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageMarkInputSchema(), PolicyResource: "tool:mail.message.mark", SideEffectClass: "workspace_write"},
	}
}

func SiteAppDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "site.create", Description: "Create a new workspace site or web app. Provide a unique slug (URL identifier) and a prompt or design brief describing what to build. Do not call this to update an existing site — use site.publish or site.restore.", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppCreateInputSchema(), PolicyResource: "tool:site.create", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "create_site", "site")},
		{Name: "site.preview", Description: "Preview a site without publishing it publicly. Use this to verify the site renders correctly before committing to a public publish. Identify the site by siteID or slug. Content-only edits (app/public/site-content.json) publish directly with no build; a build is needed only after app/src or app config changes.", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "high", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppPublishInputSchema(), PolicyResource: "tool:site.preview", SideEffectClass: "external_publish"},
		{Name: "site.publish", Description: "Publish a site so it is publicly accessible. Identify the site by siteID or slug. Use site.preview first if you want to check output before going live. Content-only edits (app/public/site-content.json) publish directly with no build; a build is needed only after app/src or app config changes.", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "high", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppPublishInputSchema(), PolicyResource: "tool:site.publish", SideEffectClass: "site_publish", CompletionEvidence: completionEvidence("success", "publish_site", "site")},
		{Name: "site.status", Description: "Check the current status of a site (live, unpublished, building, etc.). Set scope to 'mine' to list all sites owned by the requester without needing a siteID or slug.", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLookupInputSchema(), PolicyResource: "tool:site.status", SideEffectClass: "read"},
		{Name: "site.history", Description: "Retrieve the deployment history (list of past revisions and their publish timestamps) for a site. Identify the site by siteID or slug. Use this before rollback to pick a target revision.", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLookupInputSchema(), PolicyResource: "tool:site.history", SideEffectClass: "read"},
		{Name: "site.diff", Description: "Show the source code diff between two revisions of a site. Provide fromRevision and toRevision from a prior site.history result. Omit both to diff the current draft against the last published revision.", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppDiffInputSchema(), PolicyResource: "tool:site.diff", SideEffectClass: "read"},
		{Name: "site.logs", Description: "Fetch build and deployment logs for a site. Use this to diagnose why a publish or preview failed. Identify the site by siteID or slug.", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLookupInputSchema(), PolicyResource: "tool:site.logs", SideEffectClass: "read"},
		{Name: "site.rollback", Description: "Roll back a site to a previous published revision. Provide the revision from a site.history result. Requires approval; this replaces the live site.", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLifecycleInputSchema(), PolicyResource: "tool:site.rollback", SideEffectClass: "external_publish", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "publish_site", "site")},
		{Name: "site.unpublish", Description: "Take a site offline so it is no longer publicly accessible. The source and revision history are preserved. Requires approval.", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLifecycleInputSchema(), PolicyResource: "tool:site.unpublish", SideEffectClass: "external_publish", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "publish_site", "site")},
		{Name: "site.restore", Description: "Restore the editable source files of a site from a prior revision into the workspace. Use this to recover from a bad edit or to undo source changes without republishing.", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLifecycleInputSchema(), PolicyResource: "tool:site.restore", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "publish_site", "site")},
		{Name: "site.repair", Description: "Re-create any missing managed scaffold files in a site's editable workspace without overwriting existing edits. Use this when site.status reports the workspace is missing or unhealthy.", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppLookupInputSchema(), PolicyResource: "tool:site.repair", SideEffectClass: "workspace_write"},
		{Name: "site.delete", Description: "Permanently delete a site and all its revisions. Requires confirm set to DELETE and userConfirmed=true after explicit user approval. Requires approval; this action is irreversible.", Version: "1", PrivacyClass: "workspace_site", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: siteAppDeleteInputSchema(), PolicyResource: "tool:site.delete", SideEffectClass: "destructive", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "publish_site", "site")},
	}
}

func ArtifactDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "artifact.review", Description: "Review rendered artifact screenshots (site, slides, PPTX, DOCX, PDF) against an intent and rubric using vision analysis. Returns a list of issues with severity (blocking/warning/info) and suggested fixes. Use this after building a site or generating a document to catch layout, text-fit, and visual hierarchy problems before delivery.", Version: "1", PrivacyClass: "workspace_document", EstimatedLatency: "high", RequiresUserPresence: false, WorksOffline: false, InputSchema: artifactReviewInputSchema(), PolicyResource: "tool:artifact.review", SideEffectClass: "read"},
	}
}

func GoogleWorkspaceDescriptors() []Descriptor {
	return []Descriptor{
		{Name: "google.docs.create", Description: "Create a new Google Doc in the requester's Google Drive. Provide a title and optionally the initial body text (plain text or Markdown). Returns a link to the created document.", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleDocsCreateInputSchema(), PolicyResource: "tool:google.docs.create", SideEffectClass: "external_write"},
		{Name: "google.sheets.create", Description: "Create a new Google Spreadsheet in the requester's Google Drive. Optionally provide sheet names and initial cell values. Returns a link to the created spreadsheet.", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleSheetsCreateInputSchema(), PolicyResource: "tool:google.sheets.create", SideEffectClass: "external_write"},
		{Name: "google.gmail.send", Description: "Send an email via the requester's connected Gmail account. Provide at least one recipient in 'to', a subject, and a body. Requires approval before sending.", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleGmailSendInputSchema(), PolicyResource: "tool:google.gmail.send", SideEffectClass: "external_send", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "send_email", "email")},
		{Name: "google.calendar.event", Description: "Create a new Google Calendar event. Provide title, start, and end as ISO 8601 timestamps. Add attendees as email addresses. Returns a link to the created event.", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleCalendarEventInputSchema(), PolicyResource: "tool:google.calendar.event", SideEffectClass: "external_write", CompletionEvidence: completionEvidence("success", "write_calendar", "calendar")},
		{Name: "google.calendar.list", Description: "List events from the requester's Google Calendar within an optional time window. Use start and end (ISO 8601) to bound the window; use query to filter by event title keyword.", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleCalendarListInputSchema(), PolicyResource: "tool:google.calendar.list", SideEffectClass: "read"},
		{Name: "google.drive.import_pptx", Description: "Import a PPTX file from the workspace into Google Drive as a native Google Slides presentation. Provide the workspace path to the .pptx file. Returns a link to the created presentation.", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleDriveImportPPTXInputSchema(), PolicyResource: "tool:google.drive.import_pptx", SideEffectClass: "external_write"},
	}
}

func completionEvidence(mode string, action string, targetKind string) *CompletionEvidenceDescriptor {
	return &CompletionEvidenceDescriptor{Mode: mode, Action: action, TargetKind: targetKind}
}

func flowTaskAddInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("prompt", jsonschema.String().WithDescription("Natural-language description of the task to create, e.g. 'Prepare the Q3 budget report by Friday'. The system uses this to generate the structured task fields.")),
		jsonschema.Field("targetPersonHint", jsonschema.String().WithDescription("Name or email of the person the task belongs to, e.g. 'Alice' or 'alice@example.com'. Leave empty to assign to the requester themselves.")),
		jsonschema.Field("weekCode", jsonschema.String().WithDescription("Work-week the task belongs to in YYYY-WNN format, e.g. '2026-W26'. Leave empty to use the current week.")),
		jsonschema.Field("allowDuplicate", jsonschema.Boolean().WithDescription("If true, create the task even if a similar one already exists. Defaults to false, which deduplicates by content.")),
	).RawMessage()
}

func flowTaskListInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("query", jsonschema.String().WithDescription("Free-text keyword filter matched against task titles and content, e.g. 'budget'. Do not put dates, week codes, or person names here — use the dedicated fields instead.")),
		jsonschema.Field("targetPersonHint", jsonschema.String().WithDescription("Name or email of the person whose tasks to list. Leave empty to list tasks for all people. Example: 'Alice' or 'alice@example.com'.")),
		jsonschema.Field("weekFrom", jsonschema.Integer().WithDescription("Start of the week range as an offset from this week: 0 this week, -1 last week, 1 next week. Omit both weekFrom and weekTo to list the current week; widen the range for other periods.")),
		jsonschema.Field("weekTo", jsonschema.Integer().WithDescription("End of the week range as an offset from this week. Omit both weekFrom and weekTo to list the current week.")),
		jsonschema.Field("status", jsonschema.String().WithDescription("Filter by task status. Accepted values: '예정', '진행', '완료', '요청', '일시정지', '기각', '중단' (or English equivalents: 'planned', 'in_progress', 'done', 'requested', 'paused', 'rejected', 'cancelled'). Leave empty to return all statuses.")),
		jsonschema.Field("limit", jsonschema.Integer().WithDescription("Maximum number of tasks to return. Defaults to 50.")),
	).RawMessage()
}

func flowTaskUpdateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("taskID", jsonschema.String().WithDescription("ID of the task to update, from a prior task.list result. Provide either taskID or query+weekCode+targetPersonHint to identify the task.")),
		jsonschema.Field("query", jsonschema.String().WithDescription("Keyword search to locate the task when taskID is unknown, e.g. 'budget report'. Used together with weekCode and targetPersonHint.")),
		jsonschema.Field("targetPersonHint", jsonschema.String().WithDescription("Name or email of the task owner when identifying by query, e.g. 'Alice'. Leave empty for the requester's own tasks.")),
		jsonschema.Field("weekCode", jsonschema.String().WithDescription("Work-week of the task in YYYY-WNN format, e.g. '2026-W26'. Helps disambiguate when multiple tasks match the query.")),
		jsonschema.Field("content", jsonschema.String().WithDescription("New title/description text for the task. Omit to leave the content unchanged.")),
		jsonschema.Field("goal", jsonschema.String().WithDescription("Definition of done or success criterion for this task. Omit to leave unchanged.")),
		jsonschema.Field("status", jsonschema.String().WithDescription("New task status. Accepted values: '예정', '진행', '완료', '요청', '일시정지', '기각', '중단'. Providing no patch fields at all automatically sets status to '완료'.")),
		jsonschema.Field("size", jsonschema.String().WithDescription("Effort size estimate for the task, e.g. 'S', 'M', 'L', 'XL'. Omit to leave unchanged.")),
		jsonschema.Field("category", jsonschema.String().WithDescription("Business category label for the task. Omit to leave unchanged.")),
		jsonschema.Field("type", jsonschema.String().WithDescription("Task type classification, e.g. 'task', 'milestone'. Omit to leave unchanged.")),
		jsonschema.Field("startDate", jsonschema.String().WithDescription("Task start date in YYYY-MM-DD format, e.g. '2026-06-23'. Omit to leave unchanged.")),
		jsonschema.Field("endDate", jsonschema.String().WithDescription("Task due date in YYYY-MM-DD format, e.g. '2026-06-30'. Omit to leave unchanged.")),
		jsonschema.Field("flag", jsonschema.Integer().WithDescription("Numeric flag bitmask for internal task classification. Omit to leave unchanged.")),
		jsonschema.Field("requestReason", jsonschema.String().WithDescription("Reason for requesting this task (used in approval or delegation flows). Omit to leave unchanged.")),
		jsonschema.Field("decisionReason", jsonschema.String().WithDescription("Reason for the approval or rejection decision. Omit to leave unchanged.")),
	).RawMessage()
}

func flowTaskDeleteInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("taskID", jsonschema.String().WithDescription("ID of the task to delete, from a prior task.list result. Provide either taskID or query+weekCode+targetPersonHint.")),
		jsonschema.Field("query", jsonschema.String().WithDescription("Keyword to locate the task when taskID is unknown, e.g. 'old planning task'. Used together with weekCode and targetPersonHint.")),
		jsonschema.Field("targetPersonHint", jsonschema.String().WithDescription("Name or email of the task owner when identifying by query, e.g. 'Alice'. Leave empty for the requester's own tasks.")),
		jsonschema.Field("weekCode", jsonschema.String().WithDescription("Work-week of the task in YYYY-WNN format, e.g. '2026-W26'. Helps disambiguate when multiple tasks match the query.")),
	).RawMessage()
}

func webSearchInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("query", jsonschema.String().WithDescription("The search query string, e.g. 'current weather in Seoul' or 'TypeScript best practices 2026'. Do not put URLs here.")),
		jsonschema.Field("location", jsonschema.String().WithDescription("Optional location bias for geographically relevant results, e.g. 'Seoul, South Korea'. Omit for global results.")),
		jsonschema.Field("language", jsonschema.String().WithDescription("Optional BCP 47 language code to bias results language, e.g. 'ko' for Korean or 'en' for English.")),
		jsonschema.Field("limit", jsonschema.Integer().WithDescription("Maximum number of results to return. Defaults to 5; maximum is 10.")),
		jsonschema.Field("allowedDomains", jsonschema.Array(jsonschema.String()).WithDescription("Restrict results to these domains only, e.g. [\"github.com\", \"docs.python.org\"]. Omit to search all domains.")),
		jsonschema.Field("excludedDomains", jsonschema.Array(jsonschema.String()).WithDescription("Exclude results from these domains, e.g. [\"pinterest.com\"]. Omit to include all domains.")),
	).RawMessage()
}

func webFetchInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("urls", jsonschema.Array(jsonschema.String()).WithDescription("List of fully-qualified public URLs to fetch, e.g. [\"https://example.com/article\"]. Maximum 10 URLs per call. Localhost and private IPs are blocked.")),
		jsonschema.Field("maxContentTokens", jsonschema.Integer().WithDescription("Soft cap on tokens returned per URL. Defaults to 50000; maximum is 100000. Reduce when fetching many URLs.")),
		jsonschema.Field("allowedDomains", jsonschema.Array(jsonschema.String()).WithDescription("If set, only URLs from these domains are fetched; others are skipped with an error. Useful for safety when the URL list is dynamic.")),
		jsonschema.Field("blockedDomains", jsonschema.Array(jsonschema.String()).WithDescription("Domains to refuse fetching even if present in urls, e.g. [\"malicious.example\"]. Supplements the built-in block list.")),
	).RawMessage()
}

func documentReadInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("materialID", jsonschema.String().WithDescription("Internal material ID from a prior file reference. Provide either materialID or path, not both.")),
		jsonschema.Field("path", jsonschema.String().WithDescription("Absolute workspace path to the file, e.g. /workspace/shared/report.pdf. Provide either path or materialID.")),
		jsonschema.Field("maxPages", jsonschema.Integer().WithDescription("Maximum number of pages to extract from a PDF. Range 0–500; 0 means use the default. Omit for non-paginated files.")),
		jsonschema.Field("maxOutputBytes", jsonschema.Integer().WithDescription("Soft cap on the returned Markdown size in bytes. Defaults to 200000 (200 KB); maximum is 1000000 (1 MB). Reduce for very large files when you only need a summary.")),
	).RawMessage()
}

func imageReadInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("materialID", jsonschema.String().WithDescription("Internal material ID from a prior file reference. Provide either materialID or path, not both.")),
		jsonschema.Field("path", jsonschema.String().WithDescription("Absolute workspace path to the image file, e.g. /workspace/shared/logo.png. Supported formats: PNG, JPG, WEBP. Maximum file size 8 MB.")),
	).RawMessage()
}

func imageGenerateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("prompt", jsonschema.String().WithDescription("Detailed description of the image to generate. Write it like describing a scene to an artist, not a keyword list.")),
		jsonschema.Required("path", jsonschema.String().WithDescription("Absolute workspace path to save the generated PNG, e.g. /workspace/shared/logo.png. Must be under /workspace and end in .png.")),
		jsonschema.Field("aspectRatio", jsonschema.StringEnum("1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3").WithDescription("Output aspect ratio. Defaults to 1:1 if omitted.")),
	).RawMessage()
}

func platformMessageContextInputSchema() json.RawMessage {
	return jsonschema.Object().RawMessage()
}

func platformMessageSearchInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("scope", jsonschema.StringEnum("currentThread", "currentChannel", "directMessage", "channel").WithDescription("Where to search. 'currentThread' and 'currentChannel' use the active conversation context. 'directMessage' searches a DM with a person; set personHint. 'channel' searches a specific channel; set channelName or channelID.")),
		jsonschema.Field("channelName", jsonschema.String().WithDescription("For scope=channel: the channel name as shown in Mattermost (without the # prefix), e.g. 'general'. Provide channelName or channelID.")),
		jsonschema.Field("channelID", jsonschema.String().WithDescription("For scope=channel: the internal Mattermost channel ID from a prior context or search result. Provide channelName or channelID.")),
		jsonschema.Field("personHint", jsonschema.String().WithDescription("For scope=directMessage: name or email of the DM counterpart, e.g. 'Alice' or 'alice@example.com'.")),
		jsonschema.Field("authoredBy", jsonschema.StringEnum("assistant", "requester", "anyone").WithDescription("Filter by author. 'assistant' returns only bot messages, 'requester' returns only the requesting user's messages, 'anyone' returns all. Defaults to 'anyone'.")),
		jsonschema.Field("queries", jsonschema.Array(jsonschema.String()).WithDescription("One or more keyword search strings matched against message content. Each entry is a separate query; results are unioned. Do not put author names or dates here.")),
		jsonschema.Field("limit", jsonschema.Integer().WithDescription("Maximum number of messages to return. Defaults to 20.")),
		jsonschema.Field("cursor", jsonschema.String().WithDescription("Pagination cursor from a previous search response. Omit on the first call.")),
	).RawMessage()
}

func platformMessageSendInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("targetType", jsonschema.StringEnum("directMessage", "currentThread", "currentChannel", "channel").WithDescription("Where to deliver the message. 'directMessage' sends a DM to personHint or personHints. 'currentThread' replies in the active thread. 'currentChannel' posts in the active channel. 'channel' posts in the channel named by channelName or channelID.")),
		jsonschema.Required("message", jsonschema.String().WithDescription("The message text to send. Supports Markdown formatting.")),
		jsonschema.Field("channelName", jsonschema.String().WithDescription("For targetType=channel: the channel name as shown in Mattermost (without the # prefix), e.g. 'general' or '광장'. Provide channelName or channelID.")),
		jsonschema.Field("channelID", jsonschema.String().WithDescription("For targetType=channel: the internal Mattermost channel ID from a prior context or search result. Provide channelName or channelID.")),
		jsonschema.Field("personHint", jsonschema.String().WithDescription("For targetType=directMessage: name or email of the single recipient, e.g. 'Alice' or 'alice@example.com'. Use personHints for multiple recipients.")),
		jsonschema.Field("personHints", jsonschema.Array(jsonschema.String()).WithDescription("For targetType=directMessage: send the same DM to several people at once. Takes precedence over personHint; the tool fans out with one approval and returns a per-recipient delivery rollup.")),
		jsonschema.Field("pin", jsonschema.Boolean().WithDescription("If true, pin the message in the channel after sending. Defaults to false.")),
		jsonschema.Field("reason", jsonschema.String().WithDescription("Optional human-readable reason for this send shown in the approval prompt, e.g. 'weekly status update'. Helps the approver understand intent.")),
	).RawMessage()
}

func platformMessageUpdateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("messageID", jsonschema.String().WithDescription("ID of the message to update. Must come from a prior message.search or message.send result — never invent an ID.")),
		jsonschema.Field("message", jsonschema.String().WithDescription("New text content for the message. Omit to leave the text unchanged.")),
		jsonschema.Field("isPinned", jsonschema.Boolean().WithDescription("Set to true to pin the message or false to unpin it. Omit to leave pin state unchanged.")),
	).RawMessage()
}

func platformMessageDeleteInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("messageIDs", jsonschema.Array(jsonschema.String()).WithDescription("IDs of messages to delete. Must come from a prior message.search result — never invent IDs. Maximum 25 IDs per call.")),
	).RawMessage()
}

func mattermostChannelUpdateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("channelID", jsonschema.String().WithDescription("Internal Mattermost channel ID from a prior context or search result. Provide either channelID or channelName.")),
		jsonschema.Field("channelName", jsonschema.String().WithDescription("Channel name as shown in Mattermost (without the # prefix), e.g. 'general'. Provide either channelName or channelID.")),
		jsonschema.Field("header", jsonschema.String().WithDescription("New header text displayed below the channel name. Set to empty string to clear the header.")),
		jsonschema.Field("displayName", jsonschema.String().WithDescription("New human-readable channel display name shown in the sidebar, e.g. 'Team Announcements'.")),
		jsonschema.Field("inviteeHints", jsonschema.Array(jsonschema.String()).WithDescription("Names or emails of people to invite to the channel, e.g. [\"Alice\", \"bob@example.com\"]. They will be added as members.")),
	).RawMessage()
}

func mattermostChannelPostsListInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("channelID", jsonschema.String().WithDescription("Internal Mattermost channel ID. Provide either channelID or channelName.")),
		jsonschema.Field("channelName", jsonschema.String().WithDescription("Channel name without the # prefix, e.g. 'general'. Provide either channelName or channelID.")),
		jsonschema.Field("page", jsonschema.Integer().WithDescription("Zero-based page index for pagination. Defaults to 0 (first page).")),
		jsonschema.Field("perPage", jsonschema.Integer().WithDescription("Number of posts to return per page. Defaults to 60; maximum is 200.")),
	).RawMessage()
}

func mattermostContextInspectInputSchema() json.RawMessage {
	return jsonschema.Object().RawMessage()
}

func mattermostPostSearchInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("scope", jsonschema.String().WithDescription("Search scope: 'currentThread', 'currentChannel', 'directMessage', or 'channel'.")),
		jsonschema.Field("channelID", jsonschema.String().WithDescription("Internal Mattermost channel ID. Used when scope is 'channel'. Provide either channelID or channelName.")),
		jsonschema.Field("channelName", jsonschema.String().WithDescription("Channel name without the # prefix. Used when scope is 'channel'. Provide either channelName or channelID.")),
		jsonschema.Field("personHint", jsonschema.String().WithDescription("Name or email of a person. Used when scope is 'directMessage' to identify the DM partner.")),
		jsonschema.Field("rootPostID", jsonschema.String().WithDescription("Mattermost post ID of the thread root. Used when scope is 'currentThread' to identify the thread.")),
		jsonschema.Field("authoredBy", jsonschema.String().WithDescription("Filter by author: 'internkim' (bot messages), 'requester' (the calling user), or 'anyone'.")),
		jsonschema.Field("queries", jsonschema.Array(jsonschema.String()).WithDescription("Keyword search strings matched against post content. Each entry is a separate query; results are unioned.")),
		jsonschema.Field("limit", jsonschema.Integer().WithDescription("Maximum number of posts to return. Defaults to 25; maximum is 25.")),
	).RawMessage()
}

func mattermostPostUpdateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("postID", jsonschema.String().WithDescription("Mattermost post ID to update. Must come from a prior search result — never invent an ID.")),
		jsonschema.Field("message", jsonschema.String().WithDescription("New text content for the post. Omit to leave the text unchanged.")),
		jsonschema.Field("isPinned", jsonschema.Boolean().WithDescription("Set to true to pin the post or false to unpin. Omit to leave pin state unchanged.")),
	).RawMessage()
}

func mattermostPostDeleteInputSchema() json.RawMessage {
	return jsonschema.Object(jsonschema.Required("postIDs", jsonschema.Array(jsonschema.String()).WithDescription("Mattermost post IDs to delete. Must come from a prior search result — never invent IDs. Maximum 25 per call."))).RawMessage()
}

func calendarEventWriteInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("title", jsonschema.String().WithDescription("Event title shown in the calendar, e.g. 'Team standup'.")),
		jsonschema.Field("description", jsonschema.String().WithDescription("Optional event notes or agenda visible to all attendees.")),
		jsonschema.Field("location", jsonschema.String().WithDescription("Physical or virtual location, e.g. 'Conference Room B' or 'https://meet.example.com/xyz'.")),
		jsonschema.Required("startISO", jsonschema.String().WithDescription("Event start time as ISO 8601 with timezone, e.g. 2026-06-23T14:00:00+09:00. Resolve relative times like 'tomorrow 2pm' yourself before calling.")),
		jsonschema.Required("endISO", jsonschema.String().WithDescription("Event end time as ISO 8601 with timezone, e.g. 2026-06-23T15:00:00+09:00. Must be after startISO.")),
		jsonschema.Field("timeZone", jsonschema.String().WithDescription("IANA timezone identifier for the event, e.g. 'Asia/Seoul'. Defaults to the workspace timezone if omitted.")),
		jsonschema.Field("isAllDay", jsonschema.Boolean().WithDescription("Set to true for an all-day event. When true, startISO and endISO should use date-only format (YYYY-MM-DD).")),
		jsonschema.Field("color", jsonschema.String().WithDescription("Optional color label for the event, e.g. 'tomato', 'blueberry'. Supported values depend on the calendar provider.")),
		jsonschema.Field("people", jsonschema.Array(jsonschema.String()).WithDescription("Attendee/person hints such as names, @handles, or emails, e.g. [\"Alice\", \"@bob\"]. For ordinary personal meetings, include the other attendees; the runtime adds the requester by default on calendar.add.")),
		jsonschema.Field("includeRequester", jsonschema.Boolean().WithDescription("Set false only for delegated, announcement, all-hands, or someone else's calendar events where the requester is not an attendee. Defaults to true for calendar.add and false for calendar.update.")),
		jsonschema.Field("reminderLeadHours", jsonschema.Integer().WithDescription("Send a reminder this many hours before the event, e.g. 1 for a 1-hour-before reminder. Omit to use the calendar default.")),
	).RawMessage()
}

func emptyInputSchema() json.RawMessage {
	return jsonschema.Object().RawMessage()
}

func calendarEventListInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("startISO", jsonschema.String().WithDescription("Inclusive start of the time window to list, as ISO 8601 with timezone, e.g. 2026-06-23T00:00:00+09:00. Resolve relative ranges like today / this week to concrete dates yourself before calling. Omit both startISO and endISO to default to today through the next 7 days in the workspace timezone; only widen the range when the user explicitly asks for past or far-future events.")),
		jsonschema.Field("endISO", jsonschema.String().WithDescription("Exclusive end of the time window, as ISO 8601 with timezone. Pair with startISO to bound the listing; for a single day use the next day at 00:00.")),
		jsonschema.Field("query", jsonschema.String().WithDescription("Optional free-text filter matched against event titles. Do NOT put a date or date range here — the time window goes in startISO/endISO. Leave empty to list everything in the window.")),
		jsonschema.Field("limit", jsonschema.Integer().WithDescription("Optional maximum number of events to return.")),
	).RawMessage()
}

func calendarEventUpdateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("eventID", jsonschema.String().WithDescription("ID of the event to update from a prior calendar.list result. Omit and set query instead to let the runtime find the event by name.")),
		jsonschema.Field("query", jsonschema.String().WithDescription("Distinctive keyword from the event title or attendee (e.g. a person or topic name) used to find the event when no eventID is given. The runtime resolves it across all dates and fails if it matches zero or several events.")),
		jsonschema.Required("title", jsonschema.String().WithDescription("Event title. Must be re-supplied even if unchanged.")),
		jsonschema.Field("description", jsonschema.String().WithDescription("Event notes or agenda. Omit to clear the existing description.")),
		jsonschema.Field("location", jsonschema.String().WithDescription("Physical or virtual location. Omit to clear the existing location.")),
		jsonschema.Required("startISO", jsonschema.String().WithDescription("Event start time as ISO 8601 with timezone, e.g. 2026-06-23T14:00:00+09:00. Must be re-supplied even if unchanged.")),
		jsonschema.Required("endISO", jsonschema.String().WithDescription("Event end time as ISO 8601 with timezone, e.g. 2026-06-23T15:00:00+09:00. Must be re-supplied even if unchanged.")),
		jsonschema.Field("timeZone", jsonschema.String().WithDescription("IANA timezone identifier, e.g. 'Asia/Seoul'. Omit to keep the existing timezone.")),
		jsonschema.Field("isAllDay", jsonschema.Boolean().WithDescription("Set to true for an all-day event; false for a timed event.")),
		jsonschema.Field("color", jsonschema.String().WithDescription("Color label for the event. Omit to keep the existing color.")),
		jsonschema.Field("people", jsonschema.Array(jsonschema.String()).WithDescription("Updated attendee/person hints such as names, @handles, or emails. This replaces the existing attendee list entirely after resolution.")),
		jsonschema.Field("includeRequester", jsonschema.Boolean().WithDescription("Set true only when the requester should be added as an attendee during this update. Defaults to false for calendar.update.")),
		jsonschema.Field("reminderLeadHours", jsonschema.Integer().WithDescription("Reminder lead time in hours. Omit to keep the existing reminder setting.")),
	).RawMessage()
}

func calendarEventDeleteInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("eventID", jsonschema.String().WithDescription("ID of the calendar event to delete from a prior calendar.list result. Omit and set query instead to let the runtime find the event by name.")),
		jsonschema.Field("query", jsonschema.String().WithDescription("Distinctive keyword from the event title or attendee used to find the event when no eventID is given. The runtime resolves it across all dates and fails if it matches zero or several events.")),
	).RawMessage()
}

func mailMessageListInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("mailbox", jsonschema.String().WithDescription("IMAP mailbox folder to list, e.g. 'INBOX', 'Sent', 'Archive'. Defaults to INBOX when omitted.")),
		jsonschema.Field("limit", jsonschema.Integer().WithDescription("Maximum number of messages to return. Defaults to 20.")),
		jsonschema.Field("cursor", jsonschema.String().WithDescription("Pagination cursor from a previous list response. Omit on the first call.")),
	).RawMessage()
}

func mailMessageSearchInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("mailbox", jsonschema.String().WithDescription("IMAP mailbox folder to search, e.g. 'INBOX'. Defaults to all folders when omitted.")),
		jsonschema.Required("query", jsonschema.String().WithDescription("Search terms matched against subject, sender, and body, e.g. 'invoice Q2 2026'. Required.")),
		jsonschema.Field("limit", jsonschema.Integer().WithDescription("Maximum number of matching messages to return. Defaults to 20.")),
		jsonschema.Field("cursor", jsonschema.String().WithDescription("Pagination cursor from a previous search response. Omit on the first call.")),
	).RawMessage()
}

func mailMessageReadInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("mailbox", jsonschema.String().WithDescription("IMAP mailbox folder the message is in, e.g. 'INBOX'. Must come from a prior list or search result.")),
		jsonschema.Required("uid", jsonschema.String().WithDescription("IMAP UID of the message to read. Must come from a prior mail.message.list or mail.message.search result — never invent a UID.")),
	).RawMessage()
}

func mailMessageSendInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("to", jsonschema.Array(jsonschema.String()).WithDescription("Primary recipient email addresses, e.g. [\"alice@example.com\"]. At least one required.")),
		jsonschema.Required("subject", jsonschema.String().WithDescription("Email subject line.")),
		jsonschema.Required("body", jsonschema.String().WithDescription("Email body text. Plain text or HTML.")),
		jsonschema.Field("cc", jsonschema.Array(jsonschema.String()).WithDescription("CC recipient email addresses.")),
		jsonschema.Field("bcc", jsonschema.Array(jsonschema.String()).WithDescription("BCC recipient email addresses. Recipients cannot see each other.")),
	).RawMessage()
}

func mailMessageMoveInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("mailbox", jsonschema.String().WithDescription("Current IMAP mailbox folder of the message, e.g. 'INBOX'. Must come from a prior list or search result.")),
		jsonschema.Required("uid", jsonschema.String().WithDescription("IMAP UID of the message to move. Must come from a prior list or search result — never invent a UID.")),
		jsonschema.Required("targetMailbox", jsonschema.String().WithDescription("Destination IMAP folder, e.g. 'Archive', 'Trash', 'Work/Projects'. The folder must already exist.")),
	).RawMessage()
}

func mailMessageMarkInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("mailbox", jsonschema.String().WithDescription("IMAP mailbox folder of the message, e.g. 'INBOX'. Must come from a prior list or search result.")),
		jsonschema.Required("uid", jsonschema.String().WithDescription("IMAP UID of the message to mark. Must come from a prior list or search result — never invent a UID.")),
		jsonschema.Field("seen", jsonschema.Boolean().WithDescription("Set to true to mark as read, false to mark as unread. Omit to leave the read state unchanged.")),
		jsonschema.Field("flagged", jsonschema.Boolean().WithDescription("Set to true to star/flag the message, false to remove the flag. Omit to leave the flagged state unchanged.")),
	).RawMessage()
}

func siteAppCreateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("slug", jsonschema.String().WithDescription("URL-safe identifier for the site, used as the subdomain or path segment. Lowercase, hyphens allowed, e.g. 'team-dashboard'. Must be unique across the workspace.")),
		jsonschema.Field("title", jsonschema.String().WithDescription("Human-readable site name shown in the dashboard, e.g. 'Team Dashboard'.")),
		jsonschema.Field("prompt", jsonschema.String().WithDescription("Natural-language description of what to build, e.g. 'A status page that shows on-call rotation and incident history'. The more specific, the better the generated output.")),
		jsonschema.Field("designBrief", jsonschema.String().WithDescription("Visual style or layout guidance for the site, e.g. 'minimal dark theme, data-dense tables, no sidebars'.")),
		jsonschema.Field("prototypeScope", jsonschema.String().WithDescription("Scope limit for the initial prototype, e.g. 'single page, static data only, no server logic'.")),
		jsonschema.Field("description", jsonschema.String().WithDescription("Short public-facing description of the site's purpose.")),
		jsonschema.Field("idea", jsonschema.String().WithDescription("Core concept or value proposition, e.g. 'make sprint progress visible to the whole company'.")),
		jsonschema.Field("purpose", jsonschema.String().WithDescription("Why this site exists and who it serves, e.g. 'internal tool for the ops team to track vendor SLAs'.")),
		jsonschema.Field("audience", jsonschema.String().WithDescription("Intended audience for the site, e.g. 'all employees' or 'engineering managers'.")),
		jsonschema.Field("archetype", jsonschema.String().WithDescription("Site archetype or template type, e.g. 'dashboard', 'landing-page', 'wiki'.")),
		jsonschema.Field("domainKeywords", jsonschema.Array(jsonschema.String()).WithDescription("Domain-specific keywords to include in generation context, e.g. [\"inventory\", \"reorder\", \"supplier\"].")),
		jsonschema.Field("content", siteContentSchema().WithDescription("Structured page content rendered into app/public/site-content.json. Provide this for a basic content site instead of prompt/designBrief — it publishes immediately with no build. Omit to derive default content from title/description.")),
	).RawMessage()
}

func siteContentSchema() jsonschema.Schema {
	return jsonschema.Object(
		jsonschema.Required("siteName", jsonschema.String().WithDescription("Site name shown as the page title and header, e.g. 'Team Dashboard'.")),
		jsonschema.Field("tagline", jsonschema.String().WithDescription("Short subtitle shown under the site name. Omit for no tagline.")),
		jsonschema.Field("heroActionLabel", jsonschema.String().WithDescription("Label for the primary call-to-action button, e.g. 'Get started'. Omit to hide the hero action.")),
		jsonschema.Field("heroActionHref", jsonschema.String().WithDescription("URL the hero action button links to. Set this whenever heroActionLabel is set.")),
		jsonschema.Required("sections", jsonschema.Array(jsonschema.Object(
			jsonschema.Required("title", jsonschema.String().WithDescription("Section heading.")),
			jsonschema.Required("body", jsonschema.String().WithDescription("Section body text.")),
		)).WithDescription("Ordered content sections rendered below the hero. At least one section is required.")),
	)
}

func siteAppPublishInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("siteID", jsonschema.String().WithDescription("Internal site ID from a prior site.status or site.create result. Provide either siteID or slug.")),
		jsonschema.Field("slug", jsonschema.String().WithDescription("URL-safe site slug used to identify the site, e.g. 'team-dashboard'. Provide either slug or siteID.")),
		jsonschema.Field("title", jsonschema.String().WithDescription("Human-readable site title. Omit to keep the existing title.")),
		jsonschema.Field("visibility", jsonschema.String().WithDescription("Visibility of the published site, e.g. 'public' or 'internal'. Defaults to the site's current visibility setting.")),
		jsonschema.Field("description", jsonschema.String().WithDescription("Short description of the site. Omit to keep the existing description.")),
		jsonschema.Field("idea", jsonschema.String().WithDescription("Core concept for this publish iteration. Omit to keep the existing value.")),
		jsonschema.Field("purpose", jsonschema.String().WithDescription("Purpose of this publish iteration. Omit to keep the existing value.")),
		jsonschema.Field("audience", jsonschema.String().WithDescription("Intended audience. Omit to keep the existing value.")),
		jsonschema.Field("archetype", jsonschema.String().WithDescription("Site archetype for this publish. Omit to keep the existing value.")),
		jsonschema.Field("domainKeywords", jsonschema.Array(jsonschema.String()).WithDescription("Domain keywords for generation context. Omit to keep existing keywords.")),
		jsonschema.Field("message", jsonschema.String().WithDescription("Changelog message for this publish revision, e.g. 'Updated hero section copy'. Shown in site history.")),
		jsonschema.Field("sourceWorkspacePath", jsonschema.String().WithDescription("Workspace path containing the editable site source files, e.g. /workspace/circles/staff/sites/team-dashboard. Provide when the source location differs from the default.")),
		jsonschema.Field("appWorkspacePath", jsonschema.String().WithDescription("Workspace path for the built app output. Provide when the build output location differs from the default.")),
	).RawMessage()
}

func siteAppLookupInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("siteID", jsonschema.String().WithDescription("Internal site ID from a prior site.create or site.status result. Provide either siteID or slug.")),
		jsonschema.Field("slug", jsonschema.String().WithDescription("URL-safe site slug, e.g. 'team-dashboard'. Provide either slug or siteID.")),
		jsonschema.Field("scope", jsonschema.StringEnum("conversation", "mine").WithDescription("Lookup scope when no siteID or slug is given. 'conversation' returns sites referenced in the current conversation; 'mine' returns all sites owned by the requester.")),
		jsonschema.Field("checkLive", jsonschema.Boolean().WithDescription("If true, also probe the live URL to verify the site is actually reachable. Defaults to false.")),
	).RawMessage()
}

func siteAppDiffInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("siteID", jsonschema.String().WithDescription("Internal site ID. Provide either siteID or slug.")),
		jsonschema.Field("slug", jsonschema.String().WithDescription("URL-safe site slug, e.g. 'team-dashboard'. Provide either slug or siteID.")),
		jsonschema.Field("fromRevision", jsonschema.String().WithDescription("Starting revision for the diff, from a prior site.history result. Omit to diff against the last published revision.")),
		jsonschema.Field("toRevision", jsonschema.String().WithDescription("Ending revision for the diff, from a prior site.history result. Omit to diff against the current draft.")),
	).RawMessage()
}

func siteAppLifecycleInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("siteID", jsonschema.String().WithDescription("Internal site ID from a prior site.status result. Provide either siteID or slug.")),
		jsonschema.Field("slug", jsonschema.String().WithDescription("URL-safe site slug, e.g. 'team-dashboard'. Provide either slug or siteID.")),
		jsonschema.Field("reason", jsonschema.String().WithDescription("Human-readable reason for this lifecycle action, e.g. 'reverting bad CSS change'. Shown in the approval prompt.")),
		jsonschema.Field("revision", jsonschema.String().WithDescription("Target revision identifier from a prior site.history result. Required for rollback; omit for unpublish/restore.")),
		jsonschema.Field("confirm", jsonschema.String().WithDescription("Confirmation token. For owner override on rollback, unpublish, or restore set this to CONFIRM after explicit user approval.")),
		jsonschema.Field("userConfirmed", jsonschema.Boolean().WithDescription("Set to true when the user has explicitly confirmed the action. Required alongside confirm for destructive operations.")),
	).RawMessage()
}

func artifactReviewInputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"artifactKind":{"type":"string","enum":["site","slides","pptx","docx","pdf"]},"intent":{"type":"string"},"rubric":{"type":"string"},"evidence":{"type":"array","items":{"type":"object","properties":{"role":{"type":"string"},"path":{"type":"string"},"mimeType":{"type":"string","enum":["image/png","image/jpeg"]},"label":{"type":"string"}},"required":["role","path","mimeType","label"]}},"expectedText":{"type":"array","items":{"type":"object","properties":{"target":{"type":"string"},"text":{"type":"string"}},"required":["target","text"]}},"previousIssues":{"type":"array","items":{"type":"object","properties":{"severity":{"type":"string"},"category":{"type":"string"},"target":{"type":"string"},"message":{"type":"string"},"suggestedFix":{"type":"string"}},"required":["severity","category","target","message","suggestedFix"]}}},"required":["artifactKind","intent","rubric","evidence"]}`)
}

func siteAppDeleteInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("siteID", jsonschema.String().WithDescription("Internal site ID from a prior site.status result. Provide either siteID or slug.")),
		jsonschema.Field("slug", jsonschema.String().WithDescription("URL-safe site slug, e.g. 'team-dashboard'. Provide either slug or siteID.")),
		jsonschema.Field("reason", jsonschema.String().WithDescription("Human-readable reason for deleting the site. Shown in the approval prompt.")),
		jsonschema.Required("confirm", jsonschema.String().WithDescription(`Must be set to the exact string "DELETE" after explicit user approval. This prevents accidental deletes.`)),
		jsonschema.Required("userConfirmed", jsonschema.Boolean().WithDescription("Must be set to true when the user has explicitly confirmed the deletion. Both confirm and userConfirmed are required.")),
	).RawMessage()
}

func googleDocsCreateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("title", jsonschema.String().WithDescription("Title of the new Google Doc, e.g. 'Q3 OKR Review'.")),
		jsonschema.Field("body", jsonschema.String().WithDescription("Initial document body as plain text or Markdown. Omit to create an empty document.")),
	).RawMessage()
}

func googleSheetsCreateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("title", jsonschema.String().WithDescription("Title of the new Google Spreadsheet, e.g. 'Budget Tracker 2026'.")),
		jsonschema.Field("sheets", jsonschema.Array(jsonschema.String()).WithDescription("Names of sheets (tabs) to create, e.g. [\"January\", \"February\"]. Omit to create a single default sheet.")),
		jsonschema.Field("values", jsonschema.Array(jsonschema.Array(jsonschema.String())).WithDescription("Initial cell values as a 2D array of strings (rows × columns), e.g. [[\"Name\",\"Amount\"],[\"Alice\",\"500\"]]. Written to the first sheet.")),
	).RawMessage()
}

func googleGmailSendInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("to", jsonschema.Array(jsonschema.String()).WithDescription("Primary recipient email addresses, e.g. [\"alice@example.com\"]. At least one required.")),
		jsonschema.Required("subject", jsonschema.String().WithDescription("Email subject line.")),
		jsonschema.Required("body", jsonschema.String().WithDescription("Email body text. Plain text or HTML.")),
		jsonschema.Field("cc", jsonschema.Array(jsonschema.String()).WithDescription("CC recipient email addresses.")),
		jsonschema.Field("bcc", jsonschema.Array(jsonschema.String()).WithDescription("BCC recipient email addresses. Recipients cannot see each other.")),
	).RawMessage()
}

func googleCalendarEventInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("title", jsonschema.String().WithDescription("Event title shown in Google Calendar, e.g. 'Design Review'.")),
		jsonschema.Required("start", jsonschema.String().WithDescription("Event start time as ISO 8601 with timezone, e.g. 2026-06-23T14:00:00+09:00. Resolve relative times before calling.")),
		jsonschema.Required("end", jsonschema.String().WithDescription("Event end time as ISO 8601 with timezone, e.g. 2026-06-23T15:00:00+09:00. Must be after start.")),
		jsonschema.Field("attendees", jsonschema.Array(jsonschema.String()).WithDescription("Attendee email addresses to invite, e.g. [\"alice@example.com\", \"bob@example.com\"].")),
		jsonschema.Field("description", jsonschema.String().WithDescription("Event notes or agenda visible to all attendees.")),
		jsonschema.Field("location", jsonschema.String().WithDescription("Physical or virtual location, e.g. 'Room 3B' or 'https://meet.google.com/xyz'.")),
	).RawMessage()
}

func googleCalendarListInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("start", jsonschema.String().WithDescription("Start of the listing window as ISO 8601 with timezone, e.g. 2026-06-23T00:00:00+09:00. Omit to default to the current time.")),
		jsonschema.Field("end", jsonschema.String().WithDescription("End of the listing window as ISO 8601 with timezone. Pair with start to bound the window.")),
		jsonschema.Field("limit", jsonschema.Integer().WithDescription("Maximum number of events to return. Defaults to 20.")),
		jsonschema.Field("query", jsonschema.String().WithDescription("Free-text filter matched against event titles. Do not put dates here — use start/end instead.")),
	).RawMessage()
}

func googleDriveImportPPTXInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("path", jsonschema.String().WithDescription("Absolute workspace path to the .pptx file to import, e.g. /workspace/shared/presentation.pptx.")),
		jsonschema.Field("title", jsonschema.String().WithDescription("Title for the Google Slides presentation. Defaults to the filename without extension.")),
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
	descriptors = append(descriptors, CompanyDescriptors()...)
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

func companyInfoGetInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("language", jsonschema.String().WithDescription("Document language to resolve the profile for, e.g. 'ko' or 'en'. Defaults to 'ko'. The response's missingFields lists core fields still empty for this language.")),
	).RawMessage()
}

func companyInfoSetInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("language", jsonschema.String().WithDescription("Language slot the localized values belong to, e.g. 'ko' or 'en'.")),
		jsonschema.Field("name", jsonschema.String().WithDescription("Legal company name, e.g. '주식회사 여명거리'.")),
		jsonschema.Field("brandName", jsonschema.String().WithDescription("Service or brand name when it differs from the legal name.")),
		jsonschema.Field("slogan", jsonschema.String().WithDescription("Company slogan for letterheads and introductions.")),
		jsonschema.Field("description", jsonschema.String().WithDescription("One-line company description for proposals and IR material.")),
		jsonschema.Field("representative", jsonschema.String().WithDescription("Representative's name; use the romanized name for the 'en' slot.")),
		jsonschema.Field("representativeTitle", jsonschema.String().WithDescription("Representative's title. Defaults to 대표이사 (ko) / CEO (en).")),
		jsonschema.Field("address", jsonschema.String().WithDescription("Registered head-office address.")),
		jsonschema.Field("officeAddress", jsonschema.String().WithDescription("Working office address when it differs from the registered address.")),
		jsonschema.Field("jurisdiction", jsonschema.String().WithDescription("Jurisdiction of incorporation for contract preambles, e.g. '대한민국' or 'the State of Delaware'.")),
		jsonschema.Field("bankAccount", jsonschema.String().WithDescription("One-line bank account: bank, account number, holder. Use the 'en' slot for international wire details (SWIFT/IBAN).")),
		jsonschema.Field("legalAttributes", jsonschema.String().WithDescription("JSON object string of country-specific label-to-value pairs, e.g. {\"사업자등록번호\": \"123-45-67890\", \"업태\": \"서비스\"}. Labels print on documents as-is.")),
		jsonschema.Field("foundedDate", jsonschema.String().WithDescription("Founding date in YYYY-MM-DD format.")),
		jsonschema.Field("capital", jsonschema.String().WithDescription("Paid-in capital, e.g. '5억 원'.")),
		jsonschema.Field("fiscalYearEnd", jsonschema.String().WithDescription("Fiscal year end month, e.g. '12월'.")),
		jsonschema.Field("employeeCount", jsonschema.Integer().WithDescription("Official employee headcount. Independent of platform member count.")),
		jsonschema.Field("phone", jsonschema.String().WithDescription("Main company phone number.")),
		jsonschema.Field("fax", jsonschema.String().WithDescription("Fax number.")),
		jsonschema.Field("email", jsonschema.String().WithDescription("Main company email address.")),
		jsonschema.Field("website", jsonschema.String().WithDescription("Company website URL.")),
	).RawMessage()
}

func companyMetricRecordInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("metric", jsonschema.String().WithDescription("Metric key in lowerCamelCase, e.g. 'annualRevenue', 'operatingProfit', 'mau', 'employees'. Reuse the same key across periods.")),
		jsonschema.Required("year", jsonschema.Integer().WithDescription("Four-digit year the value belongs to, e.g. 2025.")),
		jsonschema.Required("value", jsonschema.Number().WithDescription("Numeric value, e.g. 1200000000 for 12억 원. Use the raw number, not a formatted string.")),
		jsonschema.Field("quarter", jsonschema.Integer().WithDescription("Quarter 1-4 for a quarterly value. Leave out for annual or monthly values; never combine with month.")),
		jsonschema.Field("month", jsonschema.Integer().WithDescription("Month 1-12 for a monthly value. Leave out for annual or quarterly values; never combine with quarter.")),
		jsonschema.Field("unit", jsonschema.String().WithDescription("Unit of the value, e.g. 'KRW', 'USD', '명', '건'.")),
		jsonschema.Field("note", jsonschema.String().WithDescription("Source or context note, e.g. '재무제표 기준'.")),
	).RawMessage()
}

func companyMetricListInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("metric", jsonschema.String().WithDescription("Metric key to filter by, e.g. 'annualRevenue'. Leave empty for all metrics.")),
		jsonschema.Field("fromYear", jsonschema.Integer().WithDescription("Earliest year to include.")),
		jsonschema.Field("toYear", jsonschema.Integer().WithDescription("Latest year to include.")),
	).RawMessage()
}

func companyRecordAddInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("category", jsonschema.String().WithDescription("Record category: 'history' (연혁), 'funding', 'product', 'certification', 'ip', 'award', 'reference', 'grant', or another short kebab-case label.")),
		jsonschema.Required("title", jsonschema.String().WithDescription("Short title, e.g. '시드 투자 유치' or '김인턴 정식 출시'.")),
		jsonschema.Field("date", jsonschema.String().WithDescription("Date of the event in YYYY-MM-DD or YYYY-MM format. Used to sort the company timeline.")),
		jsonschema.Field("detail", jsonschema.String().WithDescription("One-to-three sentence description.")),
		jsonschema.Field("attributes", jsonschema.String().WithDescription("JSON object string of structured details, e.g. {\"round\": \"Seed\", \"amount\": \"20억 원\", \"investors\": \"ABC벤처스\"}.")),
	).RawMessage()
}

func companyRecordListInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("category", jsonschema.String().WithDescription("Category to filter by, e.g. 'funding' or 'history'. Leave empty for all.")),
		jsonschema.Field("query", jsonschema.String().WithDescription("Keyword filter matched against title, detail, and attributes.")),
	).RawMessage()
}

func companyRecordUpdateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("id", jsonschema.String().WithDescription("Record id from a prior company.record.list result.")),
		jsonschema.Field("category", jsonschema.String().WithDescription("New category. Omit to keep unchanged.")),
		jsonschema.Field("date", jsonschema.String().WithDescription("New date. Omit to keep unchanged.")),
		jsonschema.Field("title", jsonschema.String().WithDescription("New title. Omit to keep unchanged.")),
		jsonschema.Field("detail", jsonschema.String().WithDescription("New detail text. Omit to keep unchanged.")),
		jsonschema.Field("attributes", jsonschema.String().WithDescription("JSON object string replacing the stored attributes. Omit to keep unchanged.")),
	).RawMessage()
}

func companyRecordDeleteInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("id", jsonschema.String().WithDescription("Record id from a prior company.record.list result. Never invent an id.")),
	).RawMessage()
}

func companyDocumentRegisterInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("documentType", jsonschema.String().WithDescription("Document type slug from the paperwork catalog, e.g. 'quote', 'service-agreement', 'employment-certificate'.")),
		jsonschema.Required("title", jsonschema.String().WithDescription("Document title including the counterpart, e.g. 'ABC물산 김인턴 도입 컨설팅 견적서'.")),
		jsonschema.Required("summary", jsonschema.String().WithDescription("2-3 sentence summary of the document's key terms: parties, amounts, dates, obligations. Written so later questions can be answered without opening the file.")),
		jsonschema.Field("kind", jsonschema.String().WithDescription("'issued' for documents the company creates (default, gets a document number), 'received' for documents from counterparts, 'internal' for internal-only files.")),
		jsonschema.Field("counterpart", jsonschema.String().WithDescription("Counterpart company or person name, e.g. 'ABC물산'.")),
		jsonschema.Field("language", jsonschema.String().WithDescription("Document language, e.g. 'ko' or 'en'.")),
		jsonschema.Field("filePath", jsonschema.String().WithDescription("Workspace path of the file if it already exists. For issued documents you can also set it later with company.document.update after saving.")),
	).RawMessage()
}

func companyDocumentListInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Field("type", jsonschema.String().WithDescription("Document type slug to filter by, e.g. 'quote'. Leave empty for all types.")),
		jsonschema.Field("counterpart", jsonschema.String().WithDescription("Counterpart name to filter by, e.g. 'ABC물산'.")),
		jsonschema.Field("query", jsonschema.String().WithDescription("Keyword filter matched against title, summary, and counterpart.")),
	).RawMessage()
}

func companyDocumentSearchInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("query", jsonschema.String().WithDescription("Natural-language question or topic, e.g. 'ABC물산이랑 맺은 용역 계약 대금 조건'.")),
		jsonschema.Field("limit", jsonschema.Integer().WithDescription("Maximum documents to return. Defaults to 5.")),
	).RawMessage()
}

func companyDocumentUpdateInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("id", jsonschema.String().WithDescription("Document id from a prior company.document.list or search result.")),
		jsonschema.Field("filePath", jsonschema.String().WithDescription("New workspace path after the file was saved, moved, or renamed.")),
		jsonschema.Field("title", jsonschema.String().WithDescription("Corrected title. Omit to keep unchanged.")),
		jsonschema.Field("counterpart", jsonschema.String().WithDescription("Corrected counterpart. Omit to keep unchanged.")),
		jsonschema.Field("summary", jsonschema.String().WithDescription("Replacement summary. Omit to keep unchanged.")),
	).RawMessage()
}
