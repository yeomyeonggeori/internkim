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
		ToolName:             "browser_open",
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
		"calendar.event.add":        "calendar_add",
		"calendar.event.delete":     "calendar_delete",
		"calendar.event.list":       "calendar_list",
		"calendar.event.update":     "calendar_update",
		"flow.task.add":             "task_add",
		"flow.task.delete":          "task_delete",
		"flow.task.list":            "task_list",
		"flow.task.update":          "task_update",
		"mattermost_channel_update": "channel_update",
		"platform.message.context":  "message_context",
		"platform.message.delete":   "message_delete",
		"platform.message.search":   "message_search",
		"platform.message.send":     "message_send",
		"platform.message.update":   "message_update",
		"site.app.create":           "site_serve",
		"site.app.delete":           "site_unserve",
		"site.app.preview":          "site_serve",
		"site.app.publish":          "site_serve",
		"site.app.status":           "site_list",
		"site.create":               "site_serve",
		"site.delete":               "site_unserve",
		"site.preview":              "site_serve",
		"site.publish":              "site_serve",
		"site.status":               "site_list",
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
	if LegacyToolNameReplacements()["flow.task.list"] != "task_list" {
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
	if descriptor.CanonicalName != "google.gmail.send" || descriptor.ModelVisibility != capabilityprotocol.ModelVisibilityHidden || descriptor.ModelVisible {
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
	descriptor := descriptorForTool(t, CalendarDescriptors(), "calendar_update")
	schema := descriptorSchema(t, CalendarDescriptors(), "calendar_update")

	if descriptor.Version != "3" {
		t.Fatalf("calendar_update version = %q", descriptor.Version)
	}
	assertSchemaHasProperties(t, schema, "eventHint", "title", "description", "location", "startISO", "endISO", "timeZone", "isAllDay", "color", "people", "includeRequester", "reminderLeadHours")
	assertSchemaOmitsProperties(t, schema, "query", "eventID")
	assertSchemaRequires(t, schema, "eventHint")
	if schema.MinProperties != 2 {
		t.Fatalf("calendar_update minProperties = %d", schema.MinProperties)
	}
	for _, fieldName := range []string{"title", "description", "location", "startISO", "endISO"} {
		if stringSliceContains(schema.Required, fieldName) {
			t.Fatalf("expected omitted %s to preserve the stored value", fieldName)
		}
	}
}

func TestCalendarDescriptorIncludesEventDeleteInput(t *testing.T) {
	schema := descriptorSchema(t, CalendarDescriptors(), "calendar_delete")

	assertSchemaHasProperties(t, schema, "eventHint")
	assertSchemaOmitsProperties(t, schema, "query", "eventID")
	assertSchemaRequires(t, schema, "eventHint")
	if descriptorForTool(t, CalendarDescriptors(), "calendar_delete").Version != "2" {
		t.Fatal("calendar_delete descriptor must use the canonical-result v2 contract")
	}
}

func TestMattermostToolsAreDefaultCapabilities(t *testing.T) {
	for _, toolName := range []string{"message_context", "message_search", "message_send", "message_update", "message_delete", "channel_update"} {
		if !containsString(defaultToolNames(), toolName) {
			t.Fatalf("expected default tools to include %q, got %+v", toolName, defaultToolNames())
		}
	}
	for _, toolName := range []string{"platform.dm.send", "platform.dm.inspect", "mattermost_context_inspect", "mattermost_post_search", "mattermost_channel_posts_list", "mattermost_channel_post", "mattermost_post_update", "mattermost_post_delete"} {
		if containsString(defaultToolNames(), toolName) {
			t.Fatalf("expected default tools to omit old message tool %q, got %+v", toolName, defaultToolNames())
		}
	}
}

func TestRegisteredDescriptorsRequireTypedContractsWhenModelVisible(t *testing.T) {
	for _, descriptors := range [][]Descriptor{DeviceDescriptors(), RegisteredToolDescriptors()} {
		if errorValue := capabilityprotocol.ValidateModelVisibleCapabilityDescriptorSet(descriptors); errorValue != nil {
			t.Fatal(errorValue)
		}
		for _, descriptor := range descriptors {
			if descriptor.ModelVisibility == capabilityprotocol.ModelVisibilityVisible || descriptor.ModelVisible {
				if descriptor.ResultContract == nil || len(descriptor.ResultContract.Schema) == 0 {
					t.Fatalf("model-visible descriptor lacks a result contract: %+v", descriptor)
				}
			}
		}
	}
}

func TestWebDescriptorsUseCanonicalSearchAndHideFetch(t *testing.T) {
	searchDescriptor := descriptorForTool(t, WebDescriptors(), "web_search")
	if searchDescriptor.ModelVisibility != capabilityprotocol.ModelVisibilityVisible || !searchDescriptor.ModelVisible {
		t.Fatalf("web_search must remain model-visible: %+v", searchDescriptor)
	}
	if searchDescriptor.RequiresApproval || searchDescriptor.SideEffectClass != "read" || searchDescriptor.ResultContract == nil {
		t.Fatalf("unexpected web_search descriptor: %+v", searchDescriptor)
	}
	if len(searchDescriptor.ResultContract.Effects) != 0 {
		t.Fatalf("web_search must not expose effects: %+v", searchDescriptor.ResultContract.Effects)
	}
	searchResultSchema := decodeSchema(t, "web_search result", searchDescriptor.ResultContract.Schema)
	assertSchemaHasProperties(t, searchResultSchema, "provider", "remoteLLMInvolved", "compatibility", "query", "answer", "results")
	assertSchemaRequires(t, searchResultSchema, "provider", "remoteLLMInvolved", "compatibility", "query", "answer", "results")

	fetchDescriptor := descriptorForTool(t, WebDescriptors(), "web_fetch")
	if fetchDescriptor.ModelVisibility != capabilityprotocol.ModelVisibilityHidden || fetchDescriptor.ModelVisible {
		t.Fatalf("web_fetch must remain registered but hidden: %+v", fetchDescriptor)
	}
}

func TestFlowDescriptorUsesTypedTaskCreateInput(t *testing.T) {
	schema := descriptorSchema(t, FlowDescriptors(), "task_add")

	assertSchemaHasProperties(t, schema, "title", "goal", "size", "status", "startDate", "endDate", "targetPersonHint", "participantPersonHints")
	assertSchemaRequires(t, schema, "title")
	if stringSliceContains(schema.Required, "goal") || stringSliceContains(schema.Required, "endDate") {
		t.Fatalf("expected goal and endDate to be optional in %+v", schema.Required)
	}
	assertSchemaOmitsProperties(t, schema, "prompt", "content", "description", "assignee", "dueDate", "ownerID", "participantIDs", "weekCode", "allowDuplicate")
	for _, descriptor := range FlowDescriptors() {
		if descriptor.Name == "task_add" && descriptor.Version != "3" {
			t.Fatalf("task_add version = %q", descriptor.Version)
		}
	}
}

func TestFlowListDescriptorMatchesTaskLookupInput(t *testing.T) {
	schema := descriptorSchema(t, FlowDescriptors(), "task_list")

	assertSchemaHasProperties(t, schema, "query", "targetPersonHint", "scope", "weekFrom", "weekTo", "status", "limit")
	assertSchemaOmitsProperties(t, schema, "weekCode", "title", "description", "assignee", "dueDate")
}

func TestFlowDescriptorIncludesTaskUpdateInput(t *testing.T) {
	schema := descriptorSchema(t, FlowDescriptors(), "task_update")

	assertSchemaHasProperties(t, schema, "taskHint", "title", "goal", "status", "size", "category", "type", "startDate", "endDate", "flag", "requestReason", "decisionReason")
	assertSchemaOmitsProperties(t, schema, "query", "targetPersonHint", "weekCode", "prompt", "allowDuplicate", "content", "taskID")
	assertSchemaRequires(t, schema, "taskHint")
	if schema.MinProperties != 2 {
		t.Fatalf("task_update minProperties = %d", schema.MinProperties)
	}
	if descriptorForTool(t, FlowDescriptors(), "task_update").Version != "3" {
		t.Fatal("task_update descriptor must use the canonical-result v3 contract")
	}
	assertDescriptorCompletionEvidence(t, FlowDescriptors(), "task_update", "success", "write_task", "task")
}

func TestFlowDescriptorIncludesTaskDeleteInput(t *testing.T) {
	schema := descriptorSchema(t, FlowDescriptors(), "task_delete")

	assertSchemaHasProperties(t, schema, "taskHint")
	assertSchemaOmitsProperties(t, schema, "query", "targetPersonHint", "weekCode", "prompt", "allowDuplicate", "content", "taskID")
	assertSchemaRequires(t, schema, "taskHint")
	if descriptorForTool(t, FlowDescriptors(), "task_delete").Version != "3" {
		t.Fatal("task_delete descriptor must use the canonical-result v3 contract")
	}
	assertDescriptorApproval(t, FlowDescriptors(), "task_delete", true)
	assertDescriptorCompletionEvidence(t, FlowDescriptors(), "task_delete", "success", "delete_task", "task")
}

func TestFlowDescriptorsDeclareCanonicalTaskResults(t *testing.T) {
	expectedEffects := map[string]string{
		"task_add":    "created",
		"task_update": "updated",
		"task_delete": "deleted",
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
	listDescriptor := descriptorForTool(t, FlowDescriptors(), "task_list")
	if listDescriptor.ResultContract == nil || len(listDescriptor.ResultContract.Effects) != 0 {
		t.Fatalf("task_list result contract = %+v", listDescriptor.ResultContract)
	}
}

func TestCalendarDescriptorsDeclareCanonicalResults(t *testing.T) {
	expectedEffects := map[string]string{
		"calendar_add":    "created",
		"calendar_update": "updated",
		"calendar_delete": "deleted",
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
	listDescriptor := descriptorForTool(t, CalendarDescriptors(), "calendar_list")
	if listDescriptor.ResultContract == nil || len(listDescriptor.ResultContract.Effects) != 0 {
		t.Fatalf("calendar_list result contract = %+v", listDescriptor.ResultContract)
	}
}

func TestPlatformMessageDescriptorsMatchMessageInputs(t *testing.T) {
	descriptors := PlatformMessageDescriptors()
	contextSchema := descriptorSchema(t, descriptors, "message_context")
	searchSchema := descriptorSchema(t, descriptors, "message_search")
	sendSchema := descriptorSchema(t, descriptors, "message_send")
	updateSchema := descriptorSchema(t, descriptors, "message_update")
	deleteSchema := descriptorSchema(t, descriptors, "message_delete")

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
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message_context", false)
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message_search", false)
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message_send", true)
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message_update", true)
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message_delete", true)
	assertDescriptorCompletionEvidence(t, PlatformMessageDescriptors(), "message_send", "success", "send_message", "message")
	assertDescriptorCompletionEvidence(t, PlatformMessageDescriptors(), "message_update", "success", "update_message", "message")
	assertDescriptorCompletionEvidence(t, PlatformMessageDescriptors(), "message_delete", "success", "delete_message", "message")
	expectedEffects := map[string]struct {
		effect      string
		resultField string
	}{
		"message_send":   {effect: "sent", resultField: "messageIDs"},
		"message_update": {effect: "updated", resultField: "messageID"},
		"message_delete": {effect: "deleted", resultField: "messageIDs"},
	}
	for toolName, expected := range expectedEffects {
		descriptor := descriptorForTool(t, descriptors, toolName)
		if descriptor.Version != "2" || descriptor.ModelVisibility != capabilityprotocol.ModelVisibilityVisible || !descriptor.ModelVisible {
			t.Fatalf("%s must use its visible generated v2 descriptor: %+v", toolName, descriptor)
		}
		if len(descriptor.ResultContract.Effects) != 1 {
			t.Fatalf("%s effects = %+v", toolName, descriptor.ResultContract.Effects)
		}
		effect := descriptor.ResultContract.Effects[0]
		if effect.ObjectType != "message" || effect.Effect != expected.effect || effect.ResultField != expected.resultField || effect.EffectIdentity != capabilityprotocol.ResourceEffectIdentityID {
			t.Fatalf("%s effect = %+v", toolName, effect)
		}
	}
	for _, toolName := range []string{"message_context", "message_search"} {
		descriptor := descriptorForTool(t, descriptors, toolName)
		if descriptor.Version != "2" || descriptor.ResultContract == nil || len(descriptor.ResultContract.Effects) != 0 {
			t.Fatalf("%s generated read contract = %+v", toolName, descriptor)
		}
	}
}

func TestMattermostDescriptorsMatchSkillInputs(t *testing.T) {
	descriptors := MattermostDescriptors()
	channelUpdateSchema := descriptorSchema(t, descriptors, "channel_update")

	assertSchemaHasProperties(t, channelUpdateSchema, "channelID", "channelName", "header", "displayName", "inviteeHints")
	assertDescriptorApproval(t, descriptors, "channel_update", true)
	if descriptorForTool(t, descriptors, "channel_update").PolicyResource != "tool:channel_update" {
		t.Fatalf("unexpected channel update policy resource")
	}
	assertDescriptorCompletionEvidence(t, descriptors, "channel_update", "success", "update_channel", "channel")
	descriptor := descriptorForTool(t, descriptors, "channel_update")
	if descriptor.Version != "2" || descriptor.ModelVisibility != capabilityprotocol.ModelVisibilityVisible || !descriptor.ModelVisible || len(descriptor.ResultContract.Effects) != 1 {
		t.Fatalf("channel_update must use its visible generated v2 descriptor: %+v", descriptor)
	}
	effect := descriptor.ResultContract.Effects[0]
	if effect.ObjectType != "channel" || effect.Effect != "updated" || effect.ResultField != "channelID" || effect.EffectIdentity != capabilityprotocol.ResourceEffectIdentityID {
		t.Fatalf("channel_update effect = %+v", effect)
	}
}

func TestWebDescriptorsAreReadOnlyDefaultTools(t *testing.T) {
	searchSchema := descriptorSchema(t, WebDescriptors(), "web_search")
	fetchSchema := descriptorSchema(t, WebDescriptors(), "web_fetch")

	assertSchemaHasProperties(t, searchSchema, "query", "location", "language", "limit", "allowedDomains", "excludedDomains")
	assertSchemaRequires(t, searchSchema, "query")
	assertSchemaHasProperties(t, fetchSchema, "urls", "maxContentTokens", "allowedDomains", "blockedDomains")
	assertSchemaRequires(t, fetchSchema, "urls")
	for _, descriptor := range WebDescriptors() {
		if descriptor.SideEffectClass != "read" {
			t.Fatalf("expected %s to be read-only, got %q", descriptor.Name, descriptor.SideEffectClass)
		}
	}
	assertDescriptorApproval(t, WebDescriptors(), "web_search", false)
	assertDescriptorApproval(t, WebDescriptors(), "web_fetch", false)
	if !containsString(defaultToolNames(), "web_search") || !containsString(defaultToolNames(), "web_fetch") {
		t.Fatalf("expected web tools in defaults, got %+v", defaultToolNames())
	}
}

func TestDocumentReadDescriptorIsReadOnlyDefaultTool(t *testing.T) {
	schema := descriptorSchema(t, FileDescriptors(), "document_read")

	assertSchemaHasProperties(t, schema, "path", "maxPages", "maxOutputBytes")
	assertSchemaOmitsProperties(t, schema, "materialID")
	assertSchemaRequires(t, schema, "path")
	assertSchemaOmitsProperties(t, schema, "ocrMode")
	descriptor := descriptorForTool(t, FileDescriptors(), "document_read")
	if descriptor.SideEffectClass != "read" || descriptor.PrivacyClass != "workspace_document" || descriptor.RequiresApproval {
		t.Fatalf("unexpected document_read descriptor: %+v", descriptor)
	}
	if descriptor.ResultContract == nil || len(descriptor.ResultContract.Effects) != 0 {
		t.Fatalf("document_read result contract = %+v", descriptor.ResultContract)
	}
	if !containsString(defaultToolNames(), "document_read") {
		t.Fatalf("expected document_read in default tools, got %+v", defaultToolNames())
	}
}

func TestImageReadDescriptorIsReadOnlyDefaultTool(t *testing.T) {
	schema := descriptorSchema(t, FileDescriptors(), "image_read")

	assertSchemaHasProperties(t, schema, "path")
	assertSchemaOmitsProperties(t, schema, "materialID")
	assertSchemaRequires(t, schema, "path")
	descriptor := descriptorForTool(t, FileDescriptors(), "image_read")
	if descriptor.SideEffectClass != "read" || descriptor.PrivacyClass != "workspace_document" || descriptor.RequiresApproval {
		t.Fatalf("unexpected image_read descriptor: %+v", descriptor)
	}
	if descriptor.ResultContract == nil || len(descriptor.ResultContract.Effects) != 0 {
		t.Fatalf("image_read result contract = %+v", descriptor.ResultContract)
	}
	if !containsString(defaultToolNames(), "image_read") {
		t.Fatalf("expected image_read in default tools, got %+v", defaultToolNames())
	}
}

func TestWebsiteBrowserDescriptorsUseCanonicalGeneratedContracts(t *testing.T) {
	for _, descriptorSet := range []struct {
		descriptors []Descriptor
		toolNames   []string
	}{
		{descriptors: CompanionToolDescriptors(), toolNames: []string{"browser_open", "browser_snapshot", "browser_screenshot", "browser_click"}},
		{descriptors: DeviceBrowserDescriptors(), toolNames: []string{"browser_open", "browser_snapshot", "browser_click"}},
	} {
		for _, toolName := range descriptorSet.toolNames {
			descriptor := descriptorForTool(t, descriptorSet.descriptors, toolName)
			if descriptor.ResultContract == nil || len(descriptor.ResultContract.Effects) != 0 {
				t.Fatalf("%s result contract = %+v", toolName, descriptor.ResultContract)
			}
		}
	}

	openSchema := descriptorSchema(t, CompanionToolDescriptors(), "browser_open")
	snapshotSchema := descriptorSchema(t, CompanionToolDescriptors(), "browser_snapshot")
	clickSchema := descriptorSchema(t, CompanionToolDescriptors(), "browser_click")
	assertSchemaHasProperties(t, openSchema, "url")
	assertSchemaRequires(t, openSchema, "url")
	assertSchemaOmitsProperties(t, openSchema, "startURL")
	assertSchemaOmitsProperties(t, snapshotSchema, "interactive")
	assertSchemaHasProperties(t, clickSchema, "target", "ref", "selector")
	if clickSchema.MinProperties != 1 {
		t.Fatalf("browser_click minProperties = %d, want 1", clickSchema.MinProperties)
	}
}

func TestUncontractedToolsStayRegisteredButHiddenFromModels(t *testing.T) {
	hiddenDefaultToolNames := []string{
		"file_pick",
		"filesystem.mount.create",
		"filesystem.mount.list",
		"filesystem.mount.pause",
		"filesystem.mount.resume",
		"filesystem.mount.revoke",
		"filesystem.mount.status",
		"filesystem.mount.stat",
		"filesystem.mount.list_directory",
		"filesystem.mount.read",
		"filesystem.mount.write",
		"filesystem.mount.mkdir",
		"filesystem.mount.rename",
		"filesystem.mount.delete",
		"filesystem.mount.truncate",
		"filesystem.mount.chmod",
		"filesystem.mount.watch",
		"browser_handoff",
		"browser_fill",
		"browser_select",
		"browser_press",
		"browser_wait",
		"image_generate",
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
		"web_fetch",
		"mail_connection_status",
		"mail_connection_start",
		"mail_message_list",
		"mail_message_search",
		"mail_message_read",
		"mail_message_send",
		"mail_message_move",
		"mail_message_mark",
	}
	defaultDescriptors := DefaultToolDescriptors()
	for _, toolName := range hiddenDefaultToolNames {
		descriptor := descriptorForTool(t, defaultDescriptors, toolName)
		if descriptor.ModelVisibility != capabilityprotocol.ModelVisibilityHidden || descriptor.ModelVisible {
			t.Fatalf("%s must remain registered but hidden: %+v", toolName, descriptor)
		}
	}
	for _, toolName := range []string{"browser_fill", "browser_select", "browser_press", "browser_wait"} {
		descriptor := descriptorForTool(t, DeviceBrowserDescriptors(), toolName)
		if descriptor.ModelVisibility != capabilityprotocol.ModelVisibilityHidden || descriptor.ModelVisible {
			t.Fatalf("device %s must remain registered but hidden: %+v", toolName, descriptor)
		}
	}
}

func TestContractedDefaultToolsRemainModelVisible(t *testing.T) {
	defaultDescriptors := DefaultToolDescriptors()
	for _, toolName := range []string{
		"browser_open",
		"browser_snapshot",
		"browser_screenshot",
		"browser_click",
		"document_read",
		"image_read",
		"message_context",
		"message_search",
		"message_send",
		"message_update",
		"message_delete",
		"channel_update",
	} {
		descriptor := descriptorForTool(t, defaultDescriptors, toolName)
		if descriptor.ModelVisibility != capabilityprotocol.ModelVisibilityVisible || !descriptor.ModelVisible || descriptor.ResultContract == nil {
			t.Fatalf("%s must remain typed and model-visible: %+v", toolName, descriptor)
		}
	}
}

func TestSiteAppDescriptorsUseCanonicalGeneratedContracts(t *testing.T) {
	descriptors := SiteAppDescriptors()
	expectedToolNames := []string{"site_serve", "site_list", "site_unserve"}
	actualToolNames := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		actualToolNames = append(actualToolNames, descriptor.Name)
		if descriptor.ResultContract == nil {
			t.Fatalf("%s result contract is missing", descriptor.Name)
		}
		if descriptor.Name == "site_unserve" && !descriptor.RequiresApproval {
			t.Fatalf("site_unserve must require runtime approval")
		}
		if descriptor.Name == "site_unserve" && descriptor.RequiresUserPresence {
			t.Fatalf("site_unserve executes on the device; requiring user presence routes it to the companion")
		}
	}
	if !reflect.DeepEqual(actualToolNames, expectedToolNames) {
		t.Fatalf("site tools = %v, want %v", actualToolNames, expectedToolNames)
	}
	for _, removedToolName := range []string{"site.create", "site.status", "site.preview", "site.publish", "site.delete", "site.edit"} {
		if containsString(actualToolNames, removedToolName) {
			t.Fatalf("removed site tool %q is still model-visible", removedToolName)
		}
	}

	serveSchema := descriptorSchema(t, descriptors, "site_serve")
	listSchema := descriptorSchema(t, descriptors, "site_list")
	unserveSchema := descriptorSchema(t, descriptors, "site_unserve")

	assertSchemaHasProperties(t, serveSchema, "title", "sourceWorkspacePath", "mode", "siteReference")
	assertSchemaRequires(t, serveSchema, "title", "sourceWorkspacePath", "mode")
	assertSchemaOmitsProperties(t, serveSchema, "slug", "content", "prompt", "siteID")
	assertSchemaHasProperties(t, listSchema, "siteReference")
	assertSchemaHasProperties(t, unserveSchema, "siteReference", "reason")
	assertSchemaRequires(t, unserveSchema, "siteReference")
	assertSchemaOmitsProperties(t, unserveSchema, "siteID", "confirm", "userConfirmed")
}

func TestArtifactReviewDescriptorUsesImageEvidenceInputs(t *testing.T) {
	schema := descriptorSchema(t, ArtifactDescriptors(), "artifact_review")

	assertSchemaHasProperties(t, schema, "artifactKind", "intent", "rubric", "evidence", "expectedText", "previousIssues")
	assertSchemaRequires(t, schema, "artifactKind", "intent", "rubric", "evidence")
	descriptor := descriptorForTool(t, ArtifactDescriptors(), "artifact_review")
	if descriptor.SideEffectClass != "read" || descriptor.PrivacyClass != "workspace_document" || descriptor.RequiresApproval {
		t.Fatalf("unexpected artifact_review descriptor: %+v", descriptor)
	}
	if descriptor.ResultContract == nil || len(descriptor.ResultContract.Effects) != 0 {
		t.Fatalf("artifact_review result contract = %+v", descriptor.ResultContract)
	}
	if descriptor.ResultContract.EvidenceCondition == nil ||
		descriptor.ResultContract.EvidenceCondition.ResultField != "passed" ||
		string(descriptor.ResultContract.EvidenceCondition.Equals) != "true" {
		t.Fatalf("artifact_review evidence condition = %+v", descriptor.ResultContract.EvidenceCondition)
	}
}

func TestCapabilityApprovalFlagsMatchRiskLevel(t *testing.T) {
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message_send", true)
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message_delete", true)
	assertDescriptorApproval(t, MattermostDescriptors(), "channel_update", true)
	assertDescriptorApproval(t, CalendarDescriptors(), "calendar_add", false)
	assertDescriptorApproval(t, MailDescriptors(), "mail_connection_start", true)
	assertDescriptorApproval(t, MailDescriptors(), "mail_message_send", true)
	assertDescriptorApproval(t, MailDescriptors(), "mail_message_search", false)
	assertDescriptorApproval(t, WebDescriptors(), "web_search", false)
	assertDescriptorApproval(t, WebDescriptors(), "web_fetch", false)
	assertDescriptorApproval(t, CalendarDescriptors(), "calendar_delete", true)
	assertDescriptorApproval(t, SiteAppDescriptors(), "site_serve", false)
	assertDescriptorApproval(t, SiteAppDescriptors(), "site_list", false)
	assertDescriptorApproval(t, SiteAppDescriptors(), "site_unserve", true)
	assertDescriptorApproval(t, GoogleWorkspaceDescriptors(), "google.calendar.event", false)
	assertDescriptorApproval(t, GoogleWorkspaceDescriptors(), "google.gmail.send", true)
}

func TestCapabilityDescriptorsExposeCompletionEvidence(t *testing.T) {
	assertDescriptorCompletionEvidence(t, PlatformMessageDescriptors(), "message_send", "success", "send_message", "message")
	assertDescriptorCompletionEvidence(t, MailDescriptors(), "mail_message_send", "success", "send_email", "email")
	assertDescriptorCompletionEvidence(t, CalendarDescriptors(), "calendar_add", "success", "write_calendar", "calendar")
	assertDescriptorCompletionEvidence(t, SiteAppDescriptors(), "site_serve", "success", "serve_site", "site")
	assertDescriptorCompletionEvidence(t, GoogleWorkspaceDescriptors(), "google.gmail.send", "success", "send_email", "email")
	assertDescriptorCompletionEvidence(t, GoogleWorkspaceDescriptors(), "google.calendar.event", "success", "write_calendar", "calendar")
}

func TestMailDescriptorsMatchSkillInputs(t *testing.T) {
	descriptors := MailDescriptors()
	listSchema := descriptorSchema(t, descriptors, "mail_message_list")
	searchSchema := descriptorSchema(t, descriptors, "mail_message_search")
	readSchema := descriptorSchema(t, descriptors, "mail_message_read")
	sendSchema := descriptorSchema(t, descriptors, "mail_message_send")

	assertSchemaHasProperties(t, listSchema, "mailbox", "limit", "cursor")
	assertSchemaOmitsProperties(t, listSchema, "query")
	assertSchemaHasProperties(t, searchSchema, "mailbox", "query", "limit", "cursor")
	assertSchemaRequires(t, searchSchema, "query")
	assertSchemaHasProperties(t, readSchema, "mailbox", "uid")
	assertSchemaRequires(t, readSchema, "mailbox", "uid")
	assertSchemaHasProperties(t, sendSchema, "to", "subject", "body", "cc", "bcc")
	assertSchemaRequires(t, sendSchema, "to", "subject", "body")
}

func TestSiteAppServeDescriptorDoesNotLookLikeGenericExternalPublish(t *testing.T) {
	descriptor := descriptorForTool(t, SiteAppDescriptors(), "site_serve")
	if descriptor.SideEffectClass != "site_publish" {
		t.Fatalf("site_serve side effect class = %q", descriptor.SideEffectClass)
	}
	if descriptor.RequiresApproval {
		t.Fatalf("site_serve should not require approval: %+v", descriptor)
	}
}

func TestSiteAppDescriptorsDeclareExactResultContracts(t *testing.T) {
	descriptors := SiteAppDescriptors()

	serveDescriptor := descriptorForTool(t, descriptors, "site_serve")
	if serveDescriptor.ResultContract == nil || len(serveDescriptor.ResultContract.Effects) != 2 {
		t.Fatalf("site_serve result contract = %+v", serveDescriptor.ResultContract)
	}
	previewEffect := serveDescriptor.ResultContract.Effects[0]
	if previewEffect.ObjectType != "website" ||
		previewEffect.Effect != "previewed" ||
		previewEffect.ResultField != "previewURL" ||
		previewEffect.EffectIdentity != capabilityprotocol.ResourceEffectIdentityURL ||
		previewEffect.When == nil ||
		previewEffect.When.ResultField != "mode" ||
		string(previewEffect.When.Equals) != `"preview"` {
		t.Fatalf("site_serve preview effect = %+v", previewEffect)
	}
	publishEffect := serveDescriptor.ResultContract.Effects[1]
	if publishEffect.ObjectType != "website" ||
		publishEffect.Effect != "published" ||
		publishEffect.ResultField != "publishedURL" ||
		publishEffect.EffectIdentity != capabilityprotocol.ResourceEffectIdentityURL ||
		publishEffect.When == nil ||
		publishEffect.When.ResultField != "mode" ||
		string(publishEffect.When.Equals) != `"publish"` {
		t.Fatalf("site_serve publish effect = %+v", publishEffect)
	}

	listDescriptor := descriptorForTool(t, descriptors, "site_list")
	if listDescriptor.ResultContract == nil || len(listDescriptor.ResultContract.Effects) != 0 {
		t.Fatalf("site_list result contract = %+v", listDescriptor.ResultContract)
	}

	unserveDescriptor := descriptorForTool(t, descriptors, "site_unserve")
	if unserveDescriptor.ResultContract == nil || len(unserveDescriptor.ResultContract.Effects) != 1 {
		t.Fatalf("site_unserve result contract = %+v", unserveDescriptor.ResultContract)
	}
	unserveEffect := unserveDescriptor.ResultContract.Effects[0]
	if unserveEffect.ObjectType != "website" ||
		unserveEffect.Effect != "deleted" ||
		unserveEffect.ResultField != "siteID" ||
		unserveEffect.EffectIdentity != capabilityprotocol.ResourceEffectIdentityID {
		t.Fatalf("site_unserve effect = %+v", unserveEffect)
	}

	serveResultSchema := decodeSchema(t, "site_serve result", serveDescriptor.ResultContract.Schema)
	listResultSchema := decodeSchema(t, "site_list result", listDescriptor.ResultContract.Schema)
	unserveResultSchema := decodeSchema(t, "site_unserve result", unserveDescriptor.ResultContract.Schema)
	assertSchemaRequires(t, serveResultSchema, "siteID", "slug", "mode", "sourceSHA256")
	assertSchemaHasProperties(t, serveResultSchema, "previewURL", "publishedURL")
	assertSchemaRequires(t, listResultSchema, "sites")
	assertSchemaRequires(t, unserveResultSchema, "siteID", "slug", "unserved")
	assertDescriptorCompletionEvidence(t, descriptors, "site_unserve", "success", "delete_site", "site")
}

func TestSiteServeEffectsProjectByMode(t *testing.T) {
	serveDescriptor := descriptorForTool(t, SiteAppDescriptors(), "site_serve")
	previewResult := json.RawMessage(`{"siteID":"site-1","slug":"demo","mode":"preview","previewURL":"https://demo.example/__preview/p-1","sourceSHA256":"` + strings.Repeat("a", 64) + `"}`)
	publishResult := json.RawMessage(`{"siteID":"site-1","slug":"demo","mode":"publish","publishedURL":"https://demo.example","sourceSHA256":"` + strings.Repeat("a", 64) + `"}`)

	previewEffects, errorValue := ProjectResourceEffects(serveDescriptor.ResultContract, previewResult)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(previewEffects) != 1 || previewEffects[0].Effect != "previewed" || previewEffects[0].URL != "https://demo.example/__preview/p-1" {
		t.Fatalf("preview effects = %+v", previewEffects)
	}

	publishEffects, errorValue := ProjectResourceEffects(serveDescriptor.ResultContract, publishResult)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(publishEffects) != 1 || publishEffects[0].Effect != "published" || publishEffects[0].URL != "https://demo.example" {
		t.Fatalf("publish effects = %+v", publishEffects)
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
	for _, toolName := range []string{"message_send", "mail_message_send", "google.gmail.send"} {
		descriptor := descriptorForTool(t, append(DefaultToolDescriptors(), GoogleWorkspaceDescriptors()...), toolName)
		if !descriptor.Idempotency.Supported || descriptor.Idempotency.Scope != "operation" {
			t.Fatalf("%s must explicitly support operation idempotency: %+v", toolName, descriptor)
		}
	}
	if descriptorForTool(t, FlowDescriptors(), "task_add").Idempotency.Supported {
		t.Fatal("task_add must not inherit idempotency from its name")
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
