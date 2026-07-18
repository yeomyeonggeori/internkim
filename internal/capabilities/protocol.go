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
type ActorContext = capabilityprotocol.ActorContext
type ToolInvokeResponse = capabilityprotocol.ToolInvokeResponse
type ResourceScope = capabilityprotocol.ResourceScope
type CompanionJobEnvelope = capabilityprotocol.CompanionJobEnvelope
type DenialResult = capabilityprotocol.DenialResult
type RecoveryAction = capabilityprotocol.RecoveryAction

func ProjectResourceEffects(contract *ToolResultContract, result json.RawMessage) ([]ResourceEffect, error) {
	return capabilityprotocol.ProjectResourceEffects(contract, result)
}

func canonicalizeDescriptors(descriptors []Descriptor) []Descriptor {
	return capabilityprotocol.MustCanonicalizeModelVisibleDescriptors(descriptors)
}

func hideUncontractedManualDescriptors(descriptors []Descriptor) []Descriptor {
	hiddenDescriptors := make([]Descriptor, len(descriptors))
	copy(hiddenDescriptors, descriptors)
	for index := range hiddenDescriptors {
		hiddenDescriptors[index].ModelVisibility = capabilityprotocol.ModelVisibilityHidden
		hiddenDescriptors[index].ModelVisible = false
	}
	return hiddenDescriptors
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
	descriptors := []Descriptor{
		{Name: "llm.text", CanonicalName: "llm.text", Namespace: "llm", ModelName: "llm.text", ModelVisibility: capabilityprotocol.ModelVisibilityHidden, ModelVisible: false, Description: "Generate free-form text using the device LLM. Internal capability used by Blueclaw's LLM backend; not directly called by the agent loop.", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true, InputSchema: capabilityprotocol.TextLLMInputSchema(), SideEffectClass: capabilityprotocol.SideEffectComputation, OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, PolicyResource: "tool:llm.text", SideEffect: capabilityprotocol.SideEffectComputation, Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "llm.structured", CanonicalName: "llm.structured", Namespace: "llm", ModelName: "llm.structured", ModelVisibility: capabilityprotocol.ModelVisibilityHidden, ModelVisible: false, Description: "Generate structured JSON output using the device LLM. Internal capability used by Blueclaw's LLM backend for structured extraction; not directly called by the agent loop.", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true, InputSchema: capabilityprotocol.StructuredLLMInputSchema(), SideEffectClass: capabilityprotocol.SideEffectComputation, OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, PolicyResource: "tool:llm.structured", SideEffect: capabilityprotocol.SideEffectComputation, Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "embedding.create", CanonicalName: "embedding.create", Namespace: "embedding", ModelName: "embedding.create", ModelVisibility: capabilityprotocol.ModelVisibilityHidden, ModelVisible: false, Description: "Generate vector embeddings for text using the device embedding model. Internal capability used for semantic search and memory retrieval; not directly called by the agent loop.", Version: "1", PrivacyClass: "model_input", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true, InputSchema: capabilityprotocol.EmbeddingInputSchema(), SideEffectClass: capabilityprotocol.SideEffectComputation, OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, PolicyResource: "tool:embedding.create", SideEffect: capabilityprotocol.SideEffectComputation, Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "platform.reply", CanonicalName: "platform.reply", Namespace: "platform", ModelName: "platform.reply", ModelVisibility: capabilityprotocol.ModelVisibilityHidden, ModelVisible: false, Description: "Send a reply in the current platform conversation context and return delivery evidence such as visibility and native attachment count. Internal shorthand used by the platform reply path; use message.send for explicit delivery targeting.", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformReplyInputSchema(), SideEffectClass: "platform_reply", CompletionEvidence: completionEvidence("success", "send_reply", "message"), OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, PolicyResource: "tool:platform.reply", SideEffect: "platform_reply", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
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
	return canonicalizeDescriptors(descriptors)
}

func platformReplyInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("replyTargetID", jsonschema.String()),
		jsonschema.Required("message", jsonschema.String()),
		jsonschema.Field("rawEventID", jsonschema.String()),
		jsonschema.Field("outboxID", jsonschema.String()),
		jsonschema.Field("replyKind", jsonschema.String()),
		jsonschema.Field("attachments", jsonschema.Array(jsonschema.Object(
			jsonschema.Field("devicePath", jsonschema.String()),
			jsonschema.Field("filename", jsonschema.String()),
			jsonschema.Field("contentType", jsonschema.String()),
			jsonschema.Field("sizeBytes", jsonschema.Integer()),
			jsonschema.Field("title", jsonschema.String()),
			jsonschema.Field("contentBase64", jsonschema.String()),
		))),
		jsonschema.Field("recoveryActions", jsonschema.Array(jsonschema.Object(
			jsonschema.Field("kind", jsonschema.String()),
			jsonschema.Field("delivery", jsonschema.String()),
			jsonschema.Field("downloadURL", jsonschema.String()),
			jsonschema.Field("connectCommand", jsonschema.String()),
			jsonschema.Field("platformUserID", jsonschema.String()),
		))),
		jsonschema.Field("interaction", jsonschema.Object(
			jsonschema.Required("interactionID", jsonschema.String()),
			jsonschema.Required("taskRunID", jsonschema.String()),
			jsonschema.Required("kind", jsonschema.String()),
			jsonschema.Field("message", jsonschema.String()),
			jsonschema.Field("question", jsonschema.String()),
			jsonschema.Field("options", jsonschema.Array(jsonschema.Object(
				jsonschema.Required("key", jsonschema.String()),
				jsonschema.Required("label", jsonschema.String()),
				jsonschema.Field("shortLabel", jsonschema.String()),
				jsonschema.Field("value", jsonschema.String()),
			))),
			jsonschema.Field("recommendedOptionKey", jsonschema.String()),
			jsonschema.Field("selectionMode", jsonschema.String()),
			jsonschema.Field("responseLanguage", jsonschema.String()),
			jsonschema.Field("targetPlatformUserID", jsonschema.String()),
		)),
	).RawMessage()
}

func CompanyDescriptors() []Descriptor {
	return canonicalizeDescriptors(hideUncontractedManualDescriptors([]Descriptor{
		{Name: "company.info.get", CanonicalName: "company.info.get", Namespace: "company", ModelName: "company.info.get", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Read the company master profile (name, representative, address, contact, bank account, country-specific legal attributes such as 사업자등록번호). Pass language ('ko' or 'en') to get the view for that document language plus missingFields listing empty core fields. Call this before creating any company letterhead document; if missingFields is empty, never ask the user for company info again.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyInfoGetInputSchema(), PolicyResource: "tool:company.info.get", SideEffectClass: "read", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "read", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "company.info.set", CanonicalName: "company.info.set", Namespace: "company", ModelName: "company.info.set", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Save or update the company master profile. Partial update: only provided fields are written, into the given language's slot for localized fields. Use after the user supplies company details, or when they report a change ('회사 주소 바뀌었어'). Put country-specific identifiers (사업자등록번호, 법인등록번호, 업태, 종목, EIN …) into legalAttributes as a label-to-value JSON object string.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyInfoSetInputSchema(), PolicyResource: "tool:company.info.set", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_company", "company"), OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "workspace_write", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "company.metric.record", CanonicalName: "company.metric.record", Namespace: "company", ModelName: "company.metric.record", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Record or correct one company metric value for a period — annual revenue, operating profit, MAU, employee count, GMV and similar time-series numbers. Monetary values use an allowed currency plus a stable USD equivalent; non-monetary values use unit. Add quarter (1-4) OR month (1-12) for sub-annual periods, never both. Same call overwrites the same period.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyMetricRecordInputSchema(), PolicyResource: "tool:company.metric.record", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_company", "company"), OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "workspace_write", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "company.metric.list", CanonicalName: "company.metric.list", Namespace: "company", ModelName: "company.metric.list", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "List recorded company metrics sorted by period. Filter by metric key and year range. Use for IR decks, business plans, and grant applications that need revenue/headcount/usage time series.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyMetricListInputSchema(), PolicyResource: "tool:company.metric.list", SideEffectClass: "read", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "read", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "company.record.add", CanonicalName: "company.record.add", Namespace: "company", ModelName: "company.record.add", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Add one company history/asset record: milestones (연혁), funding rounds, products, patents, certifications, awards, client references, government grants. Set category, date, title; put structured details (amount, investors, round …) into attributes as a label-to-value JSON object string.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyRecordAddInputSchema(), PolicyResource: "tool:company.record.add", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_company", "company"), OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "workspace_write", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "company.record.list", CanonicalName: "company.record.list", Namespace: "company", ModelName: "company.record.list", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "List company history/asset records, newest first. Filter by category (history, funding, product, certification, ip, award, reference, grant …) or keyword query. Use to build 연혁 sections, funding tables, and product overviews.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyRecordListInputSchema(), PolicyResource: "tool:company.record.list", SideEffectClass: "read", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "read", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "company.record.update", CanonicalName: "company.record.update", Namespace: "company", ModelName: "company.record.update", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Update fields on an existing company record identified by id from a prior company.record.list result. Only provided fields change.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyRecordUpdateInputSchema(), PolicyResource: "tool:company.record.update", SideEffectClass: "workspace_write", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "workspace_write", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "company.record.delete", CanonicalName: "company.record.delete", Namespace: "company", ModelName: "company.record.delete", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Delete a company record by id from a prior company.record.list result. Use only when the user asks to remove a wrong entry.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyRecordDeleteInputSchema(), PolicyResource: "tool:company.record.delete", SideEffectClass: "destructive", RequiresApproval: true, OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "destructive", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "company.document.register", CanonicalName: "company.document.register", Namespace: "company", ModelName: "company.document.register", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Register a company document in the document ledger and, for kind=issued, receive the official document number to print in the document plus the storage directory to save the final file in. Call BEFORE rendering an official document so the number appears in it. Always include a 2-3 sentence summary of the document's key terms (parties, amounts, dates) so later questions can be answered without re-reading the file.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyDocumentRegisterInputSchema(), PolicyResource: "tool:company.document.register", SideEffectClass: "workspace_write", CompletionEvidence: completionEvidence("success", "write_company", "company"), OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "workspace_write", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "company.document.list", CanonicalName: "company.document.list", Namespace: "company", ModelName: "company.document.list", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "List registered company documents newest first, with their numbers, counterparts, file paths, and summaries. Filter by type, counterpart, or keyword. Use to answer 'what quotes did we send to X'.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyDocumentListInputSchema(), PolicyResource: "tool:company.document.list", SideEffectClass: "read", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "read", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "company.document.search", CanonicalName: "company.document.search", Namespace: "company", ModelName: "company.document.search", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Semantically search registered company documents by a natural-language question ('ABC와 맺은 계약 조건'). Returns best-matching documents with summaries — answer from the summary first and open the file only when detail is needed.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyDocumentSearchInputSchema(), PolicyResource: "tool:company.document.search", SideEffectClass: "read", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "read", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "company.document.update", CanonicalName: "company.document.update", Namespace: "company", ModelName: "company.document.update", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Update a registered document's file path, title, counterpart, or summary by id from a prior list/search result. Use when a file was moved or renamed so the ledger keeps tracking it.", Version: "1", PrivacyClass: "workspace_company", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: true, InputSchema: companyDocumentUpdateInputSchema(), PolicyResource: "tool:company.document.update", SideEffectClass: "workspace_write", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "workspace_write", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
	}))
}

func WebDescriptors() []Descriptor {
	descriptors := capabilityprotocol.MustGeneratedToolDescriptors("web.search")
	descriptors = append(descriptors, hideUncontractedManualDescriptors([]Descriptor{
		{Name: "web.fetch", CanonicalName: "web.fetch", Namespace: "web", ModelName: "web.fetch", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Fetch and return the text content of one or more public URLs. Use after web.search when you need the full page content, not just a snippet. Do not fetch localhost or private network addresses — those are blocked.", Version: "1", PrivacyClass: "public_web", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: webFetchInputSchema(), PolicyResource: "tool:web.fetch", SideEffectClass: "read", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "read", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
	})...)
	return canonicalizeDescriptors(descriptors)
}

func FileDescriptors() []Descriptor {
	descriptors := capabilityprotocol.MustGeneratedToolDescriptors("document.read", "image.read")
	descriptors = append(descriptors,
		Descriptor{Name: "image.generate", CanonicalName: "image.generate", Namespace: "image", ModelName: "image.generate", ModelVisibility: capabilityprotocol.ModelVisibilityHidden, ModelVisible: false, Description: "Generate a new image from a text prompt and save it to a workspace path. Provide an absolute /workspace output path ending in .png. Optionally set aspectRatio. Returns the saved image as an attachment. Use image.read instead if you need to read an existing image file.", Version: "1", PrivacyClass: "workspace_document", EstimatedLatency: "high", RequiresUserPresence: false, WorksOffline: false, InputSchema: imageGenerateInputSchema(), PolicyResource: "tool:image.generate", SideEffectClass: "external_write", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "external_write", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
	)
	return canonicalizeDescriptors(descriptors)
}

func PlatformMessageDescriptors() []Descriptor {
	return canonicalizeDescriptors(hideUncontractedManualDescriptors([]Descriptor{
		{Name: "message.context", CanonicalName: "message.context", Namespace: "message", ModelName: "message.context", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Return metadata about the current Mattermost conversation context — the active channel, thread, and requester identity. Call this first when you need to know where the conversation is happening before sending or searching messages.", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformMessageContextInputSchema(), PolicyResource: "tool:message.context", SideEffectClass: "read", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "read", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "message.search", CanonicalName: "message.search", Namespace: "message", ModelName: "message.search", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Search past Mattermost messages in a channel, thread, or DM conversation. Use this to find what was said, retrieve prior messages, or check history. Do not use this to send a message — use message.send.", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformMessageSearchInputSchema(), PolicyResource: "tool:message.search", SideEffectClass: "read", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "read", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "message.send", CanonicalName: "message.send", Namespace: "message", ModelName: "message.send", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Send a Mattermost message to a channel, thread, or DM. Requires approval before delivering. For DM to multiple people set personHints instead of personHint to fan out with a single approval.", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformMessageSendInputSchema(), PolicyResource: "tool:message.send", SideEffectClass: "external_send", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "send_message", "message"), Idempotency: IdempotencyMetadata{Supported: true, Scope: "operation"}, OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "external_send", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}},
		{Name: "message.update", CanonicalName: "message.update", Namespace: "message", ModelName: "message.update", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Edit the text of an existing Mattermost message or change its pinned state. Requires the messageID from a prior search or send result — never invent an ID. Requires approval.", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformMessageUpdateInputSchema(), PolicyResource: "tool:message.update", SideEffectClass: "external_write", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "update_message", "message"), OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "external_write", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "message.delete", CanonicalName: "message.delete", Namespace: "message", ModelName: "message.delete", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Permanently delete one or more Mattermost messages by their IDs (up to 25 at once). Requires the messageIDs from a prior search result — never invent IDs. Requires approval; this action is irreversible.", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: platformMessageDeleteInputSchema(), PolicyResource: "tool:message.delete", SideEffectClass: "destructive", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "delete_message", "message"), OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "destructive", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
	}))
}

func MattermostDescriptors() []Descriptor {
	return canonicalizeDescriptors(hideUncontractedManualDescriptors([]Descriptor{
		{Name: "channel.update", CanonicalName: "channel.update", Namespace: "channel", ModelName: "channel.update", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Update a Mattermost channel's display name, header text, or member list. Provide channelID or channelName to identify the channel. At least one of displayName, header, or inviteeHints must be set. Requires approval.", Version: "1", PrivacyClass: "platform_message", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mattermostChannelUpdateInputSchema(), PolicyResource: "tool:channel.update", SideEffectClass: "external_write", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "update_channel", "channel"), OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "external_write", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
	}))
}

func FlowDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors(
		"task.add",
		"task.list",
		"task.update",
		"task.delete",
	))
}

func CalendarDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors(
		"calendar.add",
		"calendar.list",
		"calendar.update",
		"calendar.delete",
	))
}

func MailDescriptors() []Descriptor {
	return canonicalizeDescriptors(hideUncontractedManualDescriptors([]Descriptor{
		{Name: "mail.connection.status", CanonicalName: "mail.connection.status", Namespace: "mail", ModelName: "mail.connection.status", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Check whether the requester's email account is connected. Call this before any mail read or write operation when you are unsure if mail is set up.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "low", RequiresUserPresence: false, WorksOffline: false, InputSchema: emptyInputSchema(), PolicyResource: "tool:mail.connection.status", SideEffectClass: "read", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "read", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "mail.connection.start", CanonicalName: "mail.connection.start", Namespace: "mail", ModelName: "mail.connection.start", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Start the email account connection flow and return setup instructions for the requester. Requires the user to be present (RequiresUserPresence=true). Only call this when mail.connection.status reports mail is not connected.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "interactive", RequiresUserPresence: true, WorksOffline: false, InputSchema: emptyInputSchema(), PolicyResource: "tool:mail.connection.start", SideEffectClass: "connect", RequiresApproval: true, OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "connect", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "mail.message.list", CanonicalName: "mail.message.list", Namespace: "mail", ModelName: "mail.message.list", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "List emails in a mailbox folder with optional pagination. Use this to browse recent messages; use mail.message.search when you need to find by keyword or subject.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageListInputSchema(), PolicyResource: "tool:mail.message.list", SideEffectClass: "read", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "read", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "mail.message.search", CanonicalName: "mail.message.search", Namespace: "mail", ModelName: "mail.message.search", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Search emails by keyword, sender, or subject. Returns matching messages with their UIDs for use with mail.message.read. Do not put pagination cursors in the query field.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageSearchInputSchema(), PolicyResource: "tool:mail.message.search", SideEffectClass: "read", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "read", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "mail.message.read", CanonicalName: "mail.message.read", Namespace: "mail", ModelName: "mail.message.read", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Fetch the full content of a single email by its mailbox name and UID. The UID must come from a prior mail.message.list or mail.message.search result — never invent a UID.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageReadInputSchema(), PolicyResource: "tool:mail.message.read", SideEffectClass: "read", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "read", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "mail.message.send", CanonicalName: "mail.message.send", Namespace: "mail", ModelName: "mail.message.send", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Send an email from the requester's connected mail account. Provide at least one recipient in 'to', a subject, and a body. Requires approval before sending.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageSendInputSchema(), PolicyResource: "tool:mail.message.send", SideEffectClass: "external_send", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "send_email", "email"), Idempotency: IdempotencyMetadata{Supported: true, Scope: "operation"}, OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "external_send", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}},
		{Name: "mail.message.move", CanonicalName: "mail.message.move", Namespace: "mail", ModelName: "mail.message.move", ModelVisibility: capabilityprotocol.ModelVisibilityHidden, ModelVisible: false, Description: "Move an email to a different mailbox folder (e.g. Archive, Trash). The UID and mailbox must come from a prior list or search result. Use this to archive or sort messages.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageMoveInputSchema(), PolicyResource: "tool:mail.message.move", SideEffectClass: "workspace_write", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "workspace_write", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "mail.message.mark", CanonicalName: "mail.message.mark", Namespace: "mail", ModelName: "mail.message.mark", ModelVisibility: capabilityprotocol.ModelVisibilityHidden, ModelVisible: false, Description: "Set the read (seen) or starred (flagged) status of an email. The UID and mailbox must come from a prior list or search result. Omit a flag field to leave it unchanged.", Version: "1", PrivacyClass: "workspace_mail", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: mailMessageMarkInputSchema(), PolicyResource: "tool:mail.message.mark", SideEffectClass: "workspace_write", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "workspace_write", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
	}))
}

func SiteAppDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors(
		"site.create",
		"site.status",
		"site.preview",
		"site.publish",
		"site.delete",
	))
}

func ArtifactDescriptors() []Descriptor {
	return canonicalizeDescriptors(capabilityprotocol.MustGeneratedToolDescriptors("artifact.review"))
}

func GoogleWorkspaceDescriptors() []Descriptor {
	return canonicalizeDescriptors(hideUncontractedManualDescriptors([]Descriptor{
		{Name: "google.docs.create", CanonicalName: "google.docs.create", Namespace: "google", ModelName: "google.docs.create", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Create a new Google Doc in the requester's Google Drive. Provide a title and optionally the initial body text (plain text or Markdown). Returns a link to the created document.", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleDocsCreateInputSchema(), PolicyResource: "tool:google.docs.create", SideEffectClass: "external_write", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "external_write", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "google.sheets.create", CanonicalName: "google.sheets.create", Namespace: "google", ModelName: "google.sheets.create", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Create a new Google Spreadsheet in the requester's Google Drive. Optionally provide sheet names and initial cell values. Returns a link to the created spreadsheet.", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleSheetsCreateInputSchema(), PolicyResource: "tool:google.sheets.create", SideEffectClass: "external_write", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "external_write", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "google.gmail.send", CanonicalName: "google.gmail.send", Namespace: "google", ModelName: "google.gmail.send", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Send an email via the requester's connected Gmail account. Provide at least one recipient in 'to', a subject, and a body. Requires approval before sending.", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleGmailSendInputSchema(), PolicyResource: "tool:google.gmail.send", SideEffectClass: "external_send", RequiresApproval: true, CompletionEvidence: completionEvidence("success", "send_email", "email"), Idempotency: IdempotencyMetadata{Supported: true, Scope: "operation"}, OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "external_send", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}},
		{Name: "google.calendar.event", CanonicalName: "google.calendar.event", Namespace: "google", ModelName: "google.calendar.event", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Create a new Google Calendar event. Provide title, start, and end as ISO 8601 timestamps. Add attendees as email addresses. Returns a link to the created event.", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleCalendarEventInputSchema(), PolicyResource: "tool:google.calendar.event", SideEffectClass: "external_write", CompletionEvidence: completionEvidence("success", "write_calendar", "calendar"), OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "external_write", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "google.calendar.list", CanonicalName: "google.calendar.list", Namespace: "google", ModelName: "google.calendar.list", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "List events from the requester's Google Calendar within an optional time window. Use start and end (ISO 8601) to bound the window; use query to filter by event title keyword.", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleCalendarListInputSchema(), PolicyResource: "tool:google.calendar.list", SideEffectClass: "read", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "read", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
		{Name: "google.drive.import_pptx", CanonicalName: "google.drive.import_pptx", Namespace: "google", ModelName: "google.drive.import_pptx", ModelVisibility: capabilityprotocol.ModelVisibilityVisible, ModelVisible: true, Description: "Import a PPTX file from the workspace into Google Drive as a native Google Slides presentation. Provide the workspace path to the .pptx file. Returns a link to the created presentation.", Version: "1", PrivacyClass: "workspace_google", EstimatedLatency: "medium", RequiresUserPresence: false, WorksOffline: false, InputSchema: googleDriveImportPPTXInputSchema(), PolicyResource: "tool:google.drive.import_pptx", SideEffectClass: "external_write", OutputSchema: capabilityprotocol.ToolInvokeOutputSchema(), InputSchemaStrict: true, OutputSchemaStrict: true, SideEffect: "external_write", Availability: capabilityprotocol.AvailabilityMetadata{State: capabilityprotocol.AvailabilityOK}, Idempotency: capabilityprotocol.IdempotencyMetadata{Scope: "operation"}},
	}))
}

func completionEvidence(mode string, action string, targetKind string) *CompletionEvidenceDescriptor {
	return &CompletionEvidenceDescriptor{Mode: mode, Action: action, TargetKind: targetKind}
}

func webFetchInputSchema() json.RawMessage {
	return jsonschema.Object(
		jsonschema.Required("urls", jsonschema.Array(jsonschema.String()).WithDescription("List of fully-qualified public URLs to fetch, e.g. [\"https://example.com/article\"]. Maximum 10 URLs per call. Localhost and private IPs are blocked.")),
		jsonschema.Field("maxContentTokens", jsonschema.Integer().WithDescription("Soft cap on tokens returned per URL. Defaults to 50000; maximum is 100000. Reduce when fetching many URLs.")),
		jsonschema.Field("allowedDomains", jsonschema.Array(jsonschema.String()).WithDescription("If set, only URLs from these domains are fetched; others are skipped with an error. Useful for safety when the URL list is dynamic.")),
		jsonschema.Field("blockedDomains", jsonschema.Array(jsonschema.String()).WithDescription("Domains to refuse fetching even if present in urls, e.g. [\"malicious.example\"]. Supplements the built-in block list.")),
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

func emptyInputSchema() json.RawMessage {
	return jsonschema.Object().RawMessage()
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
		jsonschema.Field("currency", jsonschema.StringEnum("USD", "KRW", "EUR", "JPY", "GBP", "CNY", "HKD", "SGD", "AUD", "CAD", "CHF", "INR").WithDescription("ISO currency of a monetary value. Use currency instead of unit for money.")),
		jsonschema.Field("valueUSD", jsonschema.Number().WithDescription("Stable USD equivalent for a non-USD monetary value. Required when currency is not USD; omitted for non-monetary values.")),
		jsonschema.Field("unit", jsonschema.String().WithDescription("Unit for a non-monetary value, e.g. '명', '건', 'sites'. Do not combine with currency.")),
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
