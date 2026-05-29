package capabilities

import (
	"encoding/json"
	"strings"
	"testing"
)

type schemaDocument struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Required   []string       `json:"required"`
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

func TestCompanionToolNamesComeFromDescriptors(t *testing.T) {
	descriptors := CompanionToolDescriptors()
	toolNames := CompanionToolNames()

	if len(descriptors) != len(toolNames) {
		t.Fatalf("expected descriptor and tool name counts to match")
	}
	for index, descriptor := range descriptors {
		if toolNames[index] != descriptor.Name {
			t.Fatalf("expected tool name %q, got %q", descriptor.Name, toolNames[index])
		}
	}
}

func TestGoogleWorkspaceToolsAreNotDefaultDeviceCapabilities(t *testing.T) {
	for _, descriptor := range DeviceDescriptors() {
		if descriptor.PrivacyClass == "workspace_google" {
			t.Fatalf("expected Google Workspace to be disabled by default, got %+v", descriptor)
		}
	}
	for _, toolName := range DefaultToolNames() {
		if strings.HasPrefix(toolName, "google.") {
			t.Fatalf("expected default tools to omit Google Workspace, got %+v", DefaultToolNames())
		}
	}
}

func TestMattermostToolsAreDefaultCapabilities(t *testing.T) {
	for _, toolName := range []string{"mattermost.channel.posts.list", "mattermost.channel.post", "mattermost.post.update", "mattermost.post.delete", "mattermost.channel.update"} {
		if !containsString(DefaultToolNames(), toolName) {
			t.Fatalf("expected default tools to include %q, got %+v", toolName, DefaultToolNames())
		}
	}
}

func TestFlowDescriptorMatchesQuickTaskInput(t *testing.T) {
	schema := descriptorSchema(t, FlowDescriptors(), "flow.task.add")

	assertSchemaHasProperties(t, schema, "prompt", "targetPersonHint", "weekCode", "allowDuplicate")
	assertSchemaRequires(t, schema, "prompt")
	assertSchemaOmitsProperties(t, schema, "title", "description", "assignee", "dueDate")
}

func TestPlatformDMDescriptorRequiresRecipientAndMessage(t *testing.T) {
	schema := descriptorSchema(t, PlatformMessageDescriptors(), "platform.dm.send")

	assertSchemaHasProperties(t, schema, "recipientHint", "message", "platform", "reason")
	assertSchemaRequires(t, schema, "recipientHint", "message")
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "platform.dm.send", true)
}

func TestPlatformDMInspectDescriptorIsReadOnly(t *testing.T) {
	schema := descriptorSchema(t, PlatformMessageDescriptors(), "platform.dm.inspect")

	assertSchemaHasProperties(t, schema, "recipientHint", "platform")
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "platform.dm.inspect", false)
	descriptor := descriptorForTool(t, PlatformMessageDescriptors(), "platform.dm.inspect")
	if descriptor.SideEffectClass != "read" {
		t.Fatalf("platform.dm.inspect side effect class = %q", descriptor.SideEffectClass)
	}
}

func TestMattermostDescriptorsMatchSkillInputs(t *testing.T) {
	descriptors := MattermostDescriptors()
	listSchema := descriptorSchema(t, descriptors, "mattermost.channel.posts.list")
	postSchema := descriptorSchema(t, descriptors, "mattermost.channel.post")
	updateSchema := descriptorSchema(t, descriptors, "mattermost.post.update")
	deleteSchema := descriptorSchema(t, descriptors, "mattermost.post.delete")
	channelUpdateSchema := descriptorSchema(t, descriptors, "mattermost.channel.update")

	assertSchemaHasProperties(t, listSchema, "channelID", "channelName", "page", "perPage")
	assertSchemaHasProperties(t, postSchema, "channelID", "channelName", "message", "pin")
	assertSchemaRequires(t, postSchema, "message")
	assertSchemaHasProperties(t, updateSchema, "postID", "message", "isPinned")
	assertSchemaRequires(t, updateSchema, "postID")
	assertSchemaHasProperties(t, deleteSchema, "postID")
	assertSchemaRequires(t, deleteSchema, "postID")
	assertSchemaHasProperties(t, channelUpdateSchema, "channelID", "channelName", "header", "displayName", "inviteeHints")
	assertDescriptorApproval(t, descriptors, "mattermost.channel.posts.list", false)
	assertDescriptorApproval(t, descriptors, "mattermost.channel.post", true)
	assertDescriptorApproval(t, descriptors, "mattermost.post.update", true)
	assertDescriptorApproval(t, descriptors, "mattermost.post.delete", true)
	assertDescriptorApproval(t, descriptors, "mattermost.channel.update", true)
	if descriptorForTool(t, descriptors, "mattermost.channel.update").PolicyResource != "tool:mattermost.channel.update" {
		t.Fatalf("unexpected channel update policy resource")
	}
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
	if !containsString(DefaultToolNames(), "web.search") || !containsString(DefaultToolNames(), "web.fetch") {
		t.Fatalf("expected web tools in defaults, got %+v", DefaultToolNames())
	}
}

func TestFileReadDescriptorIsReadOnlyDefaultTool(t *testing.T) {
	schema := descriptorSchema(t, FileDescriptors(), "file.read")

	assertSchemaHasProperties(t, schema, "path", "ocrMode", "maxPages", "maxOutputBytes")
	assertSchemaRequires(t, schema, "path")
	descriptor := descriptorForTool(t, FileDescriptors(), "file.read")
	if descriptor.SideEffectClass != "read" || descriptor.PrivacyClass != "workspace_document" || descriptor.RequiresApproval {
		t.Fatalf("unexpected file.read descriptor: %+v", descriptor)
	}
	if !containsString(DefaultToolNames(), "file.read") {
		t.Fatalf("expected file.read in default tools, got %+v", DefaultToolNames())
	}
}

func TestSiteAppDescriptorsUseRuntimeInputNames(t *testing.T) {
	createSchema := descriptorSchema(t, SiteAppDescriptors(), "site.app.create")
	publishSchema := descriptorSchema(t, SiteAppDescriptors(), "site.app.publish")
	statusSchema := descriptorSchema(t, SiteAppDescriptors(), "site.app.status")
	deleteSchema := descriptorSchema(t, SiteAppDescriptors(), "site.app.delete")

	assertSchemaHasProperties(t, createSchema, "slug", "title", "prompt", "designBrief", "prototypeScope")
	assertSchemaRequires(t, createSchema, "slug")
	assertSchemaOmitsProperties(t, createSchema, "name", "sourcePath")
	assertSchemaHasProperties(t, publishSchema, "siteID", "slug", "message")
	assertSchemaHasProperties(t, statusSchema, "siteID", "slug")
	assertSchemaHasProperties(t, deleteSchema, "siteID", "slug", "confirm", "userConfirmed")
	assertSchemaRequires(t, deleteSchema, "confirm", "userConfirmed")
}

func TestCapabilityApprovalFlagsMatchRiskLevel(t *testing.T) {
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "platform.dm.send", true)
	assertDescriptorApproval(t, MattermostDescriptors(), "mattermost.channel.post", true)
	assertDescriptorApproval(t, MattermostDescriptors(), "mattermost.channel.update", true)
	assertDescriptorApproval(t, CalendarDescriptors(), "calendar.event.add", false)
	assertDescriptorApproval(t, MailDescriptors(), "mail.message.send", true)
	assertDescriptorApproval(t, MailDescriptors(), "mail.message.search", false)
	assertDescriptorApproval(t, WebDescriptors(), "web.search", false)
	assertDescriptorApproval(t, WebDescriptors(), "web.fetch", false)
	assertDescriptorApproval(t, CalendarDescriptors(), "calendar.event.delete", true)
	assertDescriptorApproval(t, SiteAppDescriptors(), "site.app.create", false)
	assertDescriptorApproval(t, SiteAppDescriptors(), "site.app.publish", false)
	assertDescriptorApproval(t, SiteAppDescriptors(), "site.app.delete", true)
	assertDescriptorApproval(t, GoogleWorkspaceDescriptors(), "google.calendar.event", false)
	assertDescriptorApproval(t, GoogleWorkspaceDescriptors(), "google.gmail.send", true)
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
	descriptor := descriptorForTool(t, SiteAppDescriptors(), "site.app.publish")
	if descriptor.SideEffectClass != "site_publish" {
		t.Fatalf("site.app.publish side effect class = %q", descriptor.SideEffectClass)
	}
	if descriptor.RequiresApproval {
		t.Fatalf("site.app.publish should not require approval: %+v", descriptor)
	}
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

func containsString(values []string, target string) bool {
	return stringSliceContains(values, target)
}
