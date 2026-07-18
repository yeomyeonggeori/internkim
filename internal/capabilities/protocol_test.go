package capabilities

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

type schemaDocument struct {
	Type          string         `json:"type"`
	Properties    map[string]any `json:"properties"`
	Required      []string       `json:"required"`
	MinProperties int            `json:"minProperties"`
}

func TestToolInvokeRequestRoundTrip(t *testing.T) {
	request := ToolInvokeRequest{
		ToolName:             "browser.open",
		Input:                json.RawMessage(`{"url":"https://example.com"}`),
		ExecutionMode:        ExecutionModeCompanion,
		RequiresUserPresence: true,
		PrivacyClass:         "user_browser",
		SessionID:            "session-1",
		TimeoutSecond:        30,
	}

	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	var decodedRequest ToolInvokeRequest
	if errorValue := json.Unmarshal(document, &decodedRequest); errorValue != nil {
		t.Fatal(errorValue)
	}

	if decodedRequest.ToolName != request.ToolName {
		t.Fatalf("expected tool name to round trip, got %q", decodedRequest.ToolName)
	}
	if decodedRequest.SessionID != request.SessionID {
		t.Fatalf("expected session id to round trip, got %q", decodedRequest.SessionID)
	}
	if string(decodedRequest.Input) != string(request.Input) {
		t.Fatalf("expected input to round trip, got %s", decodedRequest.Input)
	}
}

func TestLegacyToolNameReplacementsCoverNeutralTaxonomy(t *testing.T) {
	expectedReplacements := map[string]string{
		"calendar.event.add":        "calendar.add",
		"calendar.event.delete":     "calendar.delete",
		"calendar.event.list":       "calendar.list",
		"calendar.event.update":     "calendar.update",
		"flow.task.add":             "task.add",
		"flow.task.delete":          "task.delete",
		"flow.task.list":            "task.list",
		"flow.task.update":          "task.update",
		"mattermost.channel.update": "channel.update",
		"platform.message.context":  "message.context",
		"platform.message.delete":   "message.delete",
		"platform.message.search":   "message.search",
		"platform.message.send":     "message.send",
		"platform.message.update":   "message.update",
		"site.app.create":           "site.create",
		"site.app.delete":           "site.delete",
		"site.app.diff":             "site.diff",
		"site.app.history":          "site.history",
		"site.app.logs":             "site.logs",
		"site.app.preview":          "site.preview",
		"site.app.publish":          "site.publish",
		"site.app.restore":          "site.restore",
		"site.app.rollback":         "site.rollback",
		"site.app.status":           "site.status",
		"site.app.unpublish":        "site.unpublish",
	}
	replacements := LegacyToolNameReplacements()
	if !reflect.DeepEqual(replacements, expectedReplacements) {
		t.Fatalf("legacy replacements = %v, want %v", replacements, expectedReplacements)
	}

	currentToolNames := map[string]bool{}
	for _, toolName := range defaultToolNames() {
		currentToolNames[toolName] = true
	}
	for legacyToolName, currentToolName := range replacements {
		if currentToolNames[legacyToolName] {
			t.Fatalf("legacy tool %q is still advertised", legacyToolName)
		}
		if !currentToolNames[currentToolName] {
			t.Fatalf("replacement target %q is not advertised", currentToolName)
		}
	}
}

func TestLegacyToolNameReplacementsReturnsIndependentMaps(t *testing.T) {
	replacements := LegacyToolNameReplacements()
	replacements["flow.task.list"] = "changed"
	if LegacyToolNameReplacements()["flow.task.list"] != "task.list" {
		t.Fatal("expected legacy replacement map to be immutable through callers")
	}
}

func TestGoogleWorkspaceToolsAreNotDefaultDeviceCapabilities(t *testing.T) {
	for _, descriptor := range DeviceDescriptors() {
		if descriptor.PrivacyClass == "workspace_google" {
			t.Fatalf("expected Google Workspace to be disabled by default, got %+v", descriptor)
		}
	}
	for _, toolName := range defaultToolNames() {
		if strings.HasPrefix(toolName, "google.") {
			t.Fatalf("expected default tools to omit Google Workspace, got %+v", defaultToolNames())
		}
	}
}

func TestRegisteredToolDescriptorsIncludeOptionalCapabilities(t *testing.T) {
	descriptor := descriptorForTool(t, RegisteredToolDescriptors(), "google.gmail.send")
	if descriptor.CanonicalName != "google.gmail.send" {
		t.Fatalf("unexpected registered descriptor: %+v", descriptor)
	}
}

func TestCalendarConnectionStartIsNotAdvertised(t *testing.T) {
	for _, descriptor := range CalendarDescriptors() {
		if descriptor.Name == "calendar.connection.start" {
			t.Fatalf("expected personal calendar connection start to be absent, got %+v", descriptor)
		}
	}
	for _, toolName := range defaultToolNames() {
		if toolName == "calendar.connection.start" {
			t.Fatalf("expected default tools to omit personal calendar connection start, got %+v", defaultToolNames())
		}
	}
}

func TestCalendarUpdateDescriptorUsesCanonicalPartialPatchContract(t *testing.T) {
	descriptor := descriptorForTool(t, CalendarDescriptors(), "calendar.update")
	schema := descriptorSchema(t, CalendarDescriptors(), "calendar.update")

	if descriptor.Version != "3" {
		t.Fatalf("calendar.update version = %q", descriptor.Version)
	}
	assertSchemaHasProperties(t, schema, "eventID", "title", "description", "location", "startISO", "endISO", "timeZone", "isAllDay", "color", "people", "includeRequester", "reminderLeadHours")
	assertSchemaOmitsProperties(t, schema, "query")
	assertSchemaRequires(t, schema, "eventID")
	if schema.MinProperties != 2 {
		t.Fatalf("calendar.update minProperties = %d", schema.MinProperties)
	}
	for _, fieldName := range []string{"title", "description", "location", "startISO", "endISO"} {
		if stringSliceContains(schema.Required, fieldName) {
			t.Fatalf("expected omitted %s to preserve the stored value", fieldName)
		}
	}
}

func TestMattermostToolsAreDefaultCapabilities(t *testing.T) {
	for _, toolName := range []string{"message.context", "message.search", "message.send", "message.update", "message.delete", "channel.update"} {
		if !containsString(defaultToolNames(), toolName) {
			t.Fatalf("expected default tools to include %q, got %+v", toolName, defaultToolNames())
		}
	}
	for _, toolName := range []string{"platform.dm.send", "platform.dm.inspect", "mattermost.context.inspect", "mattermost.post.search", "mattermost.channel.posts.list", "mattermost.channel.post", "mattermost.post.update", "mattermost.post.delete"} {
		if containsString(defaultToolNames(), toolName) {
			t.Fatalf("expected default tools to omit old message tool %q, got %+v", toolName, defaultToolNames())
		}
	}
}

func TestFlowDescriptorUsesTypedTaskCreateInput(t *testing.T) {
	schema := descriptorSchema(t, FlowDescriptors(), "task.add")

	assertSchemaHasProperties(t, schema, "title", "goal", "size", "status", "startDate", "endDate", "targetPersonHint", "participantPersonHints")
	assertSchemaRequires(t, schema, "title")
	if stringSliceContains(schema.Required, "goal") || stringSliceContains(schema.Required, "endDate") {
		t.Fatalf("expected goal and endDate to be optional in %+v", schema.Required)
	}
	assertSchemaOmitsProperties(t, schema, "prompt", "content", "description", "assignee", "dueDate", "ownerID", "participantIDs", "weekCode", "allowDuplicate")
	for _, descriptor := range FlowDescriptors() {
		if descriptor.Name == "task.add" && descriptor.Version != "3" {
			t.Fatalf("task.add version = %q", descriptor.Version)
		}
	}
}

func TestFlowListDescriptorMatchesTaskLookupInput(t *testing.T) {
	schema := descriptorSchema(t, FlowDescriptors(), "task.list")

	assertSchemaHasProperties(t, schema, "query", "targetPersonHint", "scope", "weekFrom", "weekTo", "status", "limit")
	assertSchemaOmitsProperties(t, schema, "weekCode", "title", "description", "assignee", "dueDate")
}

func TestFlowDescriptorIncludesTaskUpdateInput(t *testing.T) {
	schema := descriptorSchema(t, FlowDescriptors(), "task.update")

	assertSchemaHasProperties(t, schema, "taskID", "title", "goal", "status", "size", "category", "type", "startDate", "endDate", "flag", "requestReason", "decisionReason")
	assertSchemaOmitsProperties(t, schema, "query", "targetPersonHint", "weekCode", "prompt", "allowDuplicate", "content")
	assertSchemaRequires(t, schema, "taskID")
	if schema.MinProperties != 2 {
		t.Fatalf("task.update minProperties = %d", schema.MinProperties)
	}
	if descriptorForTool(t, FlowDescriptors(), "task.update").Version != "3" {
		t.Fatal("task.update descriptor must use the canonical-result v3 contract")
	}
	assertDescriptorCompletionEvidence(t, FlowDescriptors(), "task.update", "success", "write_task", "task")
}

func TestFlowDescriptorIncludesTaskDeleteInput(t *testing.T) {
	schema := descriptorSchema(t, FlowDescriptors(), "task.delete")

	assertSchemaHasProperties(t, schema, "taskID")
	assertSchemaOmitsProperties(t, schema, "query", "targetPersonHint", "weekCode", "prompt", "allowDuplicate", "content")
	assertSchemaRequires(t, schema, "taskID")
	if descriptorForTool(t, FlowDescriptors(), "task.delete").Version != "3" {
		t.Fatal("task.delete descriptor must use the canonical-result v3 contract")
	}
	assertDescriptorApproval(t, FlowDescriptors(), "task.delete", true)
	assertDescriptorCompletionEvidence(t, FlowDescriptors(), "task.delete", "success", "delete_task", "task")
}

func TestFlowDescriptorsDeclareCanonicalTaskResults(t *testing.T) {
	expectedEffects := map[string]string{
		"task.add":    "created",
		"task.update": "updated",
		"task.delete": "deleted",
	}
	for toolName, expectedEffect := range expectedEffects {
		descriptor := descriptorForTool(t, FlowDescriptors(), toolName)
		if descriptor.ResultContract == nil {
			t.Fatalf("%s result contract is missing", toolName)
		}
		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
			Required   []string                   `json:"required"`
		}
		if errorValue := json.Unmarshal(descriptor.ResultContract.Schema, &schema); errorValue != nil {
			t.Fatal(errorValue)
		}
		if _, hasTaskID := schema.Properties["taskID"]; !hasTaskID || !stringSliceContains(schema.Required, "taskID") {
			t.Fatalf("%s result schema must require taskID", toolName)
		}
		if len(descriptor.ResultContract.Effects) != 1 ||
			descriptor.ResultContract.Effects[0].ObjectType != "task" ||
			descriptor.ResultContract.Effects[0].Effect != expectedEffect ||
			descriptor.ResultContract.Effects[0].ResultField != "taskID" ||
			descriptor.ResultContract.Effects[0].EffectIdentity != capabilityprotocol.ResourceEffectIdentityID {
			t.Fatalf("%s effects = %+v", toolName, descriptor.ResultContract.Effects)
		}
	}
	listDescriptor := descriptorForTool(t, FlowDescriptors(), "task.list")
	if listDescriptor.ResultContract == nil || len(listDescriptor.ResultContract.Effects) != 0 {
		t.Fatalf("task.list result contract = %+v", listDescriptor.ResultContract)
	}
}

func TestCalendarDescriptorsDeclareCanonicalResults(t *testing.T) {
	expectedEffects := map[string]string{
		"calendar.add":    "created",
		"calendar.update": "updated",
		"calendar.delete": "deleted",
	}
	for toolName, expectedEffect := range expectedEffects {
		descriptor := descriptorForTool(t, CalendarDescriptors(), toolName)
		if descriptor.ResultContract == nil {
			t.Fatalf("%s result contract is missing", toolName)
		}
		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
			Required   []string                   `json:"required"`
		}
		if errorValue := json.Unmarshal(descriptor.ResultContract.Schema, &schema); errorValue != nil {
			t.Fatal(errorValue)
		}
		if _, hasEventID := schema.Properties["eventID"]; !hasEventID || !stringSliceContains(schema.Required, "eventID") {
			t.Fatalf("%s result schema must require eventID", toolName)
		}
		if len(descriptor.ResultContract.Effects) != 1 ||
			descriptor.ResultContract.Effects[0].ObjectType != "calendar" ||
			descriptor.ResultContract.Effects[0].Effect != expectedEffect ||
			descriptor.ResultContract.Effects[0].ResultField != "eventID" ||
			descriptor.ResultContract.Effects[0].EffectIdentity != capabilityprotocol.ResourceEffectIdentityID {
			t.Fatalf("%s effects = %+v", toolName, descriptor.ResultContract.Effects)
		}
	}
	listDescriptor := descriptorForTool(t, CalendarDescriptors(), "calendar.list")
	if listDescriptor.ResultContract == nil || len(listDescriptor.ResultContract.Effects) != 0 {
		t.Fatalf("calendar.list result contract = %+v", listDescriptor.ResultContract)
	}
}

func TestPlatformMessageDescriptorsMatchMessageInputs(t *testing.T) {
	contextSchema := descriptorSchema(t, PlatformMessageDescriptors(), "message.context")
	searchSchema := descriptorSchema(t, PlatformMessageDescriptors(), "message.search")
	sendSchema := descriptorSchema(t, PlatformMessageDescriptors(), "message.send")
	updateSchema := descriptorSchema(t, PlatformMessageDescriptors(), "message.update")
	deleteSchema := descriptorSchema(t, PlatformMessageDescriptors(), "message.delete")

	assertSchemaHasProperties(t, contextSchema)
	assertSchemaHasProperties(t, searchSchema, "scope", "channelName", "channelID", "personHint", "authoredBy", "queries", "limit", "cursor")
	assertSchemaOmitsProperties(t, searchSchema, "query", "deliveryTarget")
	assertSchemaHasProperties(t, sendSchema, "targetType", "channelName", "channelID", "personHint", "personHints", "message", "pin", "reason")
	assertSchemaOmitsProperties(t, sendSchema, "deliveryTarget", "recipientHint")
	assertSchemaRequires(t, sendSchema, "targetType", "message")
	assertSchemaHasProperties(t, updateSchema, "messageID", "message", "isPinned")
	assertSchemaRequires(t, updateSchema, "messageID")
	assertSchemaHasProperties(t, deleteSchema, "messageIDs")
	assertSchemaRequires(t, deleteSchema, "messageIDs")
	assertSchemaOmitsProperties(t, deleteSchema, "scope", "targetType", "authoredBy", "query", "queries", "limit", "cursor")
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message.context", false)
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message.search", false)
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message.send", true)
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message.update", true)
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message.delete", true)
	assertDescriptorCompletionEvidence(t, PlatformMessageDescriptors(), "message.send", "success", "send_message", "message")
	assertDescriptorCompletionEvidence(t, PlatformMessageDescriptors(), "message.update", "success", "update_message", "message")
	assertDescriptorCompletionEvidence(t, PlatformMessageDescriptors(), "message.delete", "success", "delete_message", "message")
}

func TestMattermostDescriptorsMatchSkillInputs(t *testing.T) {
	descriptors := MattermostDescriptors()
	channelUpdateSchema := descriptorSchema(t, descriptors, "channel.update")

	assertSchemaHasProperties(t, channelUpdateSchema, "channelID", "channelName", "header", "displayName", "inviteeHints")
	assertDescriptorApproval(t, descriptors, "channel.update", true)
	if descriptorForTool(t, descriptors, "channel.update").PolicyResource != "tool:channel.update" {
		t.Fatalf("unexpected channel update policy resource")
	}
	assertDescriptorCompletionEvidence(t, descriptors, "channel.update", "success", "update_channel", "channel")
}

func TestWebDescriptorsAreReadOnlyDefaultTools(t *testing.T) {
	searchSchema := descriptorSchema(t, WebDescriptors(), "web.search")
	fetchSchema := descriptorSchema(t, WebDescriptors(), "web.fetch")

	assertSchemaHasProperties(t, searchSchema, "query", "location", "language", "limit", "allowedDomains", "excludedDomains")
	assertSchemaRequires(t, searchSchema, "query")
	assertSchemaHasProperties(t, fetchSchema, "urls", "maxContentTokens", "allowedDomains", "blockedDomains")
	assertSchemaRequires(t, fetchSchema, "urls")
	for _, descriptor := range WebDescriptors() {
		if descriptor.SideEffectClass != "read" {
			t.Fatalf("expected %s to be read-only, got %q", descriptor.Name, descriptor.SideEffectClass)
		}
	}
	assertDescriptorApproval(t, WebDescriptors(), "web.search", false)
	assertDescriptorApproval(t, WebDescriptors(), "web.fetch", false)
	if !containsString(defaultToolNames(), "web.search") || !containsString(defaultToolNames(), "web.fetch") {
		t.Fatalf("expected web tools in defaults, got %+v", defaultToolNames())
	}
}

func TestDocumentReadDescriptorIsReadOnlyDefaultTool(t *testing.T) {
	schema := descriptorSchema(t, FileDescriptors(), "document.read")

	assertSchemaHasProperties(t, schema, "path", "maxPages", "maxOutputBytes")
	assertSchemaOmitsProperties(t, schema, "materialID")
	assertSchemaRequires(t, schema, "path")
	assertSchemaOmitsProperties(t, schema, "ocrMode")
	descriptor := descriptorForTool(t, FileDescriptors(), "document.read")
	if descriptor.SideEffectClass != "read" || descriptor.PrivacyClass != "workspace_document" || descriptor.RequiresApproval {
		t.Fatalf("unexpected document.read descriptor: %+v", descriptor)
	}
	if descriptor.ResultContract == nil || len(descriptor.ResultContract.Effects) != 0 {
		t.Fatalf("document.read result contract = %+v", descriptor.ResultContract)
	}
	if !containsString(defaultToolNames(), "document.read") {
		t.Fatalf("expected document.read in default tools, got %+v", defaultToolNames())
	}
}

func TestImageReadDescriptorIsReadOnlyDefaultTool(t *testing.T) {
	schema := descriptorSchema(t, FileDescriptors(), "image.read")

	assertSchemaHasProperties(t, schema, "path")
	assertSchemaOmitsProperties(t, schema, "materialID")
	assertSchemaRequires(t, schema, "path")
	descriptor := descriptorForTool(t, FileDescriptors(), "image.read")
	if descriptor.SideEffectClass != "read" || descriptor.PrivacyClass != "workspace_document" || descriptor.RequiresApproval {
		t.Fatalf("unexpected image.read descriptor: %+v", descriptor)
	}
	if descriptor.ResultContract == nil || len(descriptor.ResultContract.Effects) != 0 {
		t.Fatalf("image.read result contract = %+v", descriptor.ResultContract)
	}
	if !containsString(defaultToolNames(), "image.read") {
		t.Fatalf("expected image.read in default tools, got %+v", defaultToolNames())
	}
}

func TestSiteAppDescriptorsUseRuntimeInputNames(t *testing.T) {
	createSchema := descriptorSchema(t, SiteAppDescriptors(), "site.create")
	previewSchema := descriptorSchema(t, SiteAppDescriptors(), "site.preview")
	publishSchema := descriptorSchema(t, SiteAppDescriptors(), "site.publish")
	statusSchema := descriptorSchema(t, SiteAppDescriptors(), "site.status")
	historySchema := descriptorSchema(t, SiteAppDescriptors(), "site.history")
	diffSchema := descriptorSchema(t, SiteAppDescriptors(), "site.diff")
	deleteSchema := descriptorSchema(t, SiteAppDescriptors(), "site.delete")

	assertSchemaHasProperties(t, createSchema, "slug", "title", "prompt", "designBrief", "prototypeScope")
	assertSchemaRequires(t, createSchema, "slug")
	assertSchemaOmitsProperties(t, createSchema, "name", "sourcePath")
	assertSchemaHasProperties(t, previewSchema, "siteID", "slug", "message")
	assertSchemaHasProperties(t, publishSchema, "siteID", "slug", "message")
	assertSchemaHasProperties(t, statusSchema, "siteID", "slug", "scope", "checkLive")
	assertSchemaHasProperties(t, historySchema, "siteID", "slug")
	assertSchemaHasProperties(t, diffSchema, "siteID", "slug", "fromRevision", "toRevision")
	assertSchemaHasProperties(t, deleteSchema, "siteID", "slug", "confirm", "userConfirmed")
	assertSchemaRequires(t, deleteSchema, "confirm", "userConfirmed")
}

func TestArtifactReviewDescriptorUsesImageEvidenceInputs(t *testing.T) {
	schema := descriptorSchema(t, ArtifactDescriptors(), "artifact.review")

	assertSchemaHasProperties(t, schema, "artifactKind", "intent", "rubric", "evidence", "expectedText", "previousIssues")
	assertSchemaRequires(t, schema, "artifactKind", "intent", "rubric", "evidence")
	descriptor := descriptorForTool(t, ArtifactDescriptors(), "artifact.review")
	if descriptor.SideEffectClass != "read" || descriptor.PrivacyClass != "workspace_document" || descriptor.RequiresApproval {
		t.Fatalf("unexpected artifact.review descriptor: %+v", descriptor)
	}
}

func TestCapabilityApprovalFlagsMatchRiskLevel(t *testing.T) {
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message.send", true)
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message.delete", true)
	assertDescriptorApproval(t, MattermostDescriptors(), "channel.update", true)
	assertDescriptorApproval(t, CalendarDescriptors(), "calendar.add", false)
	assertDescriptorApproval(t, MailDescriptors(), "mail.connection.start", true)
	assertDescriptorApproval(t, MailDescriptors(), "mail.message.send", true)
	assertDescriptorApproval(t, MailDescriptors(), "mail.message.search", false)
	assertDescriptorApproval(t, WebDescriptors(), "web.search", false)
	assertDescriptorApproval(t, WebDescriptors(), "web.fetch", false)
	assertDescriptorApproval(t, CalendarDescriptors(), "calendar.delete", true)
	assertDescriptorApproval(t, SiteAppDescriptors(), "site.create", false)
	assertDescriptorApproval(t, SiteAppDescriptors(), "site.preview", false)
	assertDescriptorApproval(t, SiteAppDescriptors(), "site.publish", false)
	assertDescriptorApproval(t, SiteAppDescriptors(), "site.delete", true)
	assertDescriptorApproval(t, GoogleWorkspaceDescriptors(), "google.calendar.event", false)
	assertDescriptorApproval(t, GoogleWorkspaceDescriptors(), "google.gmail.send", true)
}

func TestCapabilityDescriptorsExposeCompletionEvidence(t *testing.T) {
	assertDescriptorCompletionEvidence(t, DeviceDescriptors(), "platform.reply", "success", "send_reply", "message")
	assertDescriptorCompletionEvidence(t, PlatformMessageDescriptors(), "message.send", "success", "send_message", "message")
	assertDescriptorCompletionEvidence(t, MailDescriptors(), "mail.message.send", "success", "send_email", "email")
	assertDescriptorCompletionEvidence(t, CalendarDescriptors(), "calendar.add", "success", "write_calendar", "calendar")
	assertDescriptorCompletionEvidence(t, SiteAppDescriptors(), "site.publish", "success", "publish_site", "site")
	assertDescriptorCompletionEvidence(t, GoogleWorkspaceDescriptors(), "google.gmail.send", "success", "send_email", "email")
	assertDescriptorCompletionEvidence(t, GoogleWorkspaceDescriptors(), "google.calendar.event", "success", "write_calendar", "calendar")
}

func TestMailDescriptorsMatchSkillInputs(t *testing.T) {
	descriptors := MailDescriptors()
	listSchema := descriptorSchema(t, descriptors, "mail.message.list")
	searchSchema := descriptorSchema(t, descriptors, "mail.message.search")
	readSchema := descriptorSchema(t, descriptors, "mail.message.read")
	sendSchema := descriptorSchema(t, descriptors, "mail.message.send")

	assertSchemaHasProperties(t, listSchema, "mailbox", "limit", "cursor")
	assertSchemaOmitsProperties(t, listSchema, "query")
	assertSchemaHasProperties(t, searchSchema, "mailbox", "query", "limit", "cursor")
	assertSchemaRequires(t, searchSchema, "query")
	assertSchemaHasProperties(t, readSchema, "mailbox", "uid")
	assertSchemaRequires(t, readSchema, "mailbox", "uid")
	assertSchemaHasProperties(t, sendSchema, "to", "subject", "body", "cc", "bcc")
	assertSchemaRequires(t, sendSchema, "to", "subject", "body")
}

func TestSiteAppPublishDescriptorDoesNotLookLikeGenericExternalPublish(t *testing.T) {
	descriptor := descriptorForTool(t, SiteAppDescriptors(), "site.publish")
	if descriptor.SideEffectClass != "site_publish" {
		t.Fatalf("site.publish side effect class = %q", descriptor.SideEffectClass)
	}
	if descriptor.RequiresApproval {
		t.Fatalf("site.publish should not require approval: %+v", descriptor)
	}
}

func TestSiteAppCreateDescriptorHasContentSchema(t *testing.T) {
	schema := descriptorSchema(t, SiteAppDescriptors(), "site.create")
	assertSchemaHasProperties(t, schema, "content")

	contentSchema, isObject := schema.Properties["content"].(map[string]any)
	if !isObject {
		t.Fatalf("expected content property to be an object schema, got %#v", schema.Properties["content"])
	}
	if contentSchema["type"] != "object" {
		t.Fatalf("expected content schema type object, got %v", contentSchema["type"])
	}
	contentProperties, isObject := contentSchema["properties"].(map[string]any)
	if !isObject {
		t.Fatalf("expected content schema properties, got %#v", contentSchema["properties"])
	}
	for _, propertyName := range []string{"siteName", "tagline", "heroActionLabel", "heroActionHref", "sections"} {
		if _, exists := contentProperties[propertyName]; !exists {
			t.Fatalf("expected content schema property %q, got %+v", propertyName, contentProperties)
		}
	}
	contentRequired, isSlice := contentSchema["required"].([]any)
	if !isSlice || !containsAnyString(contentRequired, "siteName") || !containsAnyString(contentRequired, "sections") {
		t.Fatalf("expected content schema to require siteName and sections, got %v", contentSchema["required"])
	}

	sectionsSchema, isObject := contentProperties["sections"].(map[string]any)
	if !isObject || sectionsSchema["type"] != "array" {
		t.Fatalf("expected sections to be an array schema, got %#v", contentProperties["sections"])
	}
	sectionItemSchema, isObject := sectionsSchema["items"].(map[string]any)
	if !isObject {
		t.Fatalf("expected sections items schema, got %#v", sectionsSchema["items"])
	}
	sectionItemProperties, isObject := sectionItemSchema["properties"].(map[string]any)
	if !isObject {
		t.Fatalf("expected section item properties, got %#v", sectionItemSchema["properties"])
	}
	for _, propertyName := range []string{"title", "body"} {
		if _, exists := sectionItemProperties[propertyName]; !exists {
			t.Fatalf("expected section item property %q, got %+v", propertyName, sectionItemProperties)
		}
	}
	sectionItemRequired, isSlice := sectionItemSchema["required"].([]any)
	if !isSlice || !containsAnyString(sectionItemRequired, "title") || !containsAnyString(sectionItemRequired, "body") {
		t.Fatalf("expected section item schema to require title and body, got %v", sectionItemSchema["required"])
	}
}

func containsAnyString(values []any, target string) bool {
	for _, value := range values {
		if stringValue, isString := value.(string); isString && stringValue == target {
			return true
		}
	}
	return false
}

func TestGoogleWorkspaceDescriptorsMatchSkillInputs(t *testing.T) {
	descriptors := GoogleWorkspaceDescriptors()
	docsSchema := descriptorSchema(t, descriptors, "google.docs.create")
	sheetsSchema := descriptorSchema(t, descriptors, "google.sheets.create")
	gmailSchema := descriptorSchema(t, descriptors, "google.gmail.send")
	eventSchema := descriptorSchema(t, descriptors, "google.calendar.event")
	listSchema := descriptorSchema(t, descriptors, "google.calendar.list")

	assertSchemaHasProperties(t, docsSchema, "title", "body")
	assertSchemaOmitsProperties(t, docsSchema, "content")
	assertSchemaHasProperties(t, sheetsSchema, "title", "sheets", "values")
	assertSchemaOmitsProperties(t, sheetsSchema, "rows")
	assertSchemaHasProperties(t, gmailSchema, "to", "subject", "body", "cc", "bcc")
	assertSchemaHasProperties(t, eventSchema, "title", "start", "end", "attendees", "description", "location")
	assertSchemaRequires(t, eventSchema, "title", "start", "end")
	assertSchemaHasProperties(t, listSchema, "start", "end", "limit", "query")
	assertSchemaOmitsProperties(t, listSchema, "timeMin", "timeMax")
}

func TestCapabilityDescriptorSchemasAreCanonicalObjects(t *testing.T) {
	descriptorGroups := [][]Descriptor{
		CompanionToolDescriptors(),
		WebDescriptors(),
		FileDescriptors(),
		PlatformMessageDescriptors(),
		MattermostDescriptors(),
		FlowDescriptors(),
		CalendarDescriptors(),
		MailDescriptors(),
		SiteAppDescriptors(),
		ArtifactDescriptors(),
		GoogleWorkspaceDescriptors(),
	}
	for _, descriptors := range descriptorGroups {
		for _, descriptor := range descriptors {
			if len(descriptor.InputSchema) == 0 {
				continue
			}
			schema := decodeSchema(t, descriptor.Name, descriptor.InputSchema)
			if schema.Type != "object" {
				t.Fatalf("expected %s schema to be object, got %+v", descriptor.Name, schema)
			}
			if schema.Properties == nil {
				t.Fatalf("expected %s schema to have properties", descriptor.Name)
			}
			assertRequiredFieldsHaveProperties(t, descriptor.Name, schema)
			assertSchemaDocumentOmitsKeywords(t, descriptor.Name, descriptor.InputSchema, "oneOf", "anyOf", "allOf")
		}
	}
}

func TestDefaultDescriptorsSatisfyCanonicalProviderContract(t *testing.T) {
	descriptors := DefaultToolDescriptors()
	if errorValue := capabilityprotocol.ValidateDescriptorSet(descriptors); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, descriptor := range descriptors {
		if descriptor.Name == "" || descriptor.CanonicalName == "" || descriptor.Namespace == "" || descriptor.ModelName == "" {
			t.Fatalf("descriptor identity is incomplete: %+v", descriptor)
		}
		if !descriptor.InputSchemaStrict || !descriptor.OutputSchemaStrict || len(descriptor.InputSchema) == 0 || len(descriptor.OutputSchema) == 0 {
			t.Fatalf("descriptor schemas are not strict: %+v", descriptor)
		}
	}
}

func TestSendDescriptorsOwnIdempotencyMetadata(t *testing.T) {
	for _, toolName := range []string{"message.send", "mail.message.send", "google.gmail.send"} {
		descriptor := descriptorForTool(t, append(DefaultToolDescriptors(), GoogleWorkspaceDescriptors()...), toolName)
		if !descriptor.Idempotency.Supported || descriptor.Idempotency.Scope != "operation" {
			t.Fatalf("%s must explicitly support operation idempotency: %+v", toolName, descriptor)
		}
	}
	if descriptorForTool(t, FlowDescriptors(), "task.add").Idempotency.Supported {
		t.Fatal("task.add must not inherit idempotency from its name")
	}
}

func assertDescriptorApproval(t *testing.T, descriptors []Descriptor, toolName string, expectedApproval bool) {
	t.Helper()
	for _, descriptor := range descriptors {
		if descriptor.Name != toolName {
			continue
		}
		if descriptor.RequiresApproval != expectedApproval {
			t.Fatalf("expected %s requiresApproval=%v, got %+v", toolName, expectedApproval, descriptor)
		}
		return
	}
	t.Fatalf("descriptor %s not found", toolName)
}

func assertDescriptorCompletionEvidence(t *testing.T, descriptors []Descriptor, toolName string, mode string, action string, targetKind string) {
	t.Helper()
	descriptor := descriptorForTool(t, descriptors, toolName)
	if descriptor.CompletionEvidence == nil {
		t.Fatalf("expected %s to define completion evidence", toolName)
	}
	if descriptor.CompletionEvidence.Mode != mode || descriptor.CompletionEvidence.Action != action || descriptor.CompletionEvidence.TargetKind != targetKind {
		t.Fatalf("unexpected completion evidence for %s: %+v", toolName, descriptor.CompletionEvidence)
	}
}

func descriptorSchema(t *testing.T, descriptors []Descriptor, toolName string) schemaDocument {
	t.Helper()
	return decodeSchema(t, toolName, descriptorForTool(t, descriptors, toolName).InputSchema)
}

func descriptorForTool(t *testing.T, descriptors []Descriptor, toolName string) Descriptor {
	t.Helper()
	for _, descriptor := range descriptors {
		if descriptor.Name == toolName {
			return descriptor
		}
	}
	t.Fatalf("descriptor %s not found", toolName)
	return Descriptor{}
}

func decodeSchema(t *testing.T, toolName string, document json.RawMessage) schemaDocument {
	t.Helper()
	var schema schemaDocument
	if errorValue := json.Unmarshal(document, &schema); errorValue != nil {
		t.Fatalf("schema for %s is invalid: %v", toolName, errorValue)
	}
	return schema
}

func assertSchemaDocumentOmitsType(t *testing.T, toolName string, document json.RawMessage, schemaType string) {
	t.Helper()
	var value any
	if errorValue := json.Unmarshal(document, &value); errorValue != nil {
		t.Fatalf("schema for %s is invalid: %v", toolName, errorValue)
	}
	if schemaDocumentContainsType(value, schemaType) {
		t.Fatalf("schema for %s must not use type %q: %s", toolName, schemaType, string(document))
	}
}

func schemaDocumentContainsType(value any, schemaType string) bool {
	document, isDocument := value.(map[string]any)
	if isDocument {
		if document["type"] == schemaType {
			return true
		}
		for _, fieldValue := range document {
			if schemaDocumentContainsType(fieldValue, schemaType) {
				return true
			}
		}
		return false
	}
	values, isValues := value.([]any)
	if isValues {
		for _, item := range values {
			if schemaDocumentContainsType(item, schemaType) {
				return true
			}
		}
	}
	return false
}

func assertSchemaHasProperties(t *testing.T, schema schemaDocument, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, exists := schema.Properties[name]; !exists {
			t.Fatalf("expected schema property %q in %+v", name, schema.Properties)
		}
	}
}

func assertSchemaOmitsProperties(t *testing.T, schema schemaDocument, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, exists := schema.Properties[name]; exists {
			t.Fatalf("expected schema to omit property %q in %+v", name, schema.Properties)
		}
	}
}

func assertSchemaRequires(t *testing.T, schema schemaDocument, names ...string) {
	t.Helper()
	for _, name := range names {
		if !stringSliceContains(schema.Required, name) {
			t.Fatalf("expected schema to require %q in %+v", name, schema.Required)
		}
	}
}

func assertRequiredFieldsHaveProperties(t *testing.T, toolName string, schema schemaDocument) {
	t.Helper()
	for _, fieldName := range schema.Required {
		if _, isFound := schema.Properties[fieldName]; !isFound {
			t.Fatalf("schema for %s requires missing property %q", toolName, fieldName)
		}
	}
}

func assertSchemaDocumentOmitsKeywords(t *testing.T, toolName string, document json.RawMessage, keywords ...string) {
	t.Helper()
	var value any
	if errorValue := json.Unmarshal(document, &value); errorValue != nil {
		t.Fatalf("schema for %s is invalid: %v", toolName, errorValue)
	}
	for _, keyword := range keywords {
		if schemaValueContainsKey(value, keyword) {
			t.Fatalf("schema for %s must not contain %s: %s", toolName, keyword, string(document))
		}
	}
}

func schemaValueContainsKey(value any, key string) bool {
	document, isObject := value.(map[string]any)
	if isObject {
		if _, isFound := document[key]; isFound {
			return true
		}
		for _, fieldValue := range document {
			if schemaValueContainsKey(fieldValue, key) {
				return true
			}
		}
		return false
	}
	values, isArray := value.([]any)
	if isArray {
		for _, item := range values {
			if schemaValueContainsKey(item, key) {
				return true
			}
		}
	}
	return false
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func defaultToolNames() []string {
	toolNames := make([]string, 0, len(DefaultToolDescriptors()))
	for _, descriptor := range DefaultToolDescriptors() {
		toolNames = append(toolNames, descriptor.Name)
	}
	return toolNames
}

func containsString(values []string, target string) bool {
	return stringSliceContains(values, target)
}
