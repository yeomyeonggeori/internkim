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

func TestFlowDescriptorMatchesQuickTaskInput(t *testing.T) {
	schema := descriptorSchema(t, FlowDescriptors(), "flow.task.add")

	assertSchemaHasProperties(t, schema, "prompt", "targetPersonHint", "weekCode", "allowDuplicate")
	assertSchemaRequires(t, schema, "prompt")
	assertSchemaOmitsProperties(t, schema, "title", "description", "assignee", "dueDate")
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
	assertDescriptorApproval(t, CalendarDescriptors(), "calendar.event.add", false)
	assertDescriptorApproval(t, CalendarDescriptors(), "calendar.event.delete", true)
	assertDescriptorApproval(t, SiteAppDescriptors(), "site.app.create", false)
	assertDescriptorApproval(t, SiteAppDescriptors(), "site.app.publish", false)
	assertDescriptorApproval(t, SiteAppDescriptors(), "site.app.delete", true)
	assertDescriptorApproval(t, GoogleWorkspaceDescriptors(), "google.calendar.event", false)
	assertDescriptorApproval(t, GoogleWorkspaceDescriptors(), "google.gmail.send", true)
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
		FlowDescriptors(),
		CalendarDescriptors(),
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
	for _, descriptor := range descriptors {
		if descriptor.Name != toolName {
			continue
		}
		return decodeSchema(t, toolName, descriptor.InputSchema)
	}
	t.Fatalf("descriptor %s not found", toolName)
	return schemaDocument{}
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
