package capabilities

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
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
		ExecutionMode:        ExecutionModeDevice,
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
		"calendar.event.add":       "event_add",
		"calendar.event.delete":    "event_delete",
		"calendar.event.list":      "event_list",
		"calendar.event.update":    "event_update",
		"flow.task.add":            "task_add",
		"flow.task.delete":         "task_delete",
		"flow.task.list":           "task_list",
		"flow.task.update":         "task_update",
		"platform.message.context": "message_context",
		"platform.message.delete":  "message_delete",
		"platform.message.search":  "message_search",
		"platform.message.send":    "message_send",
		"platform.message.update":  "message_update",
		"calendar_add":             "event_add",
		"calendar_delete":          "event_delete",
		"calendar_list":            "event_list",
		"calendar_update":          "event_update",
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
	descriptor := descriptorForTool(t, CalendarDescriptors(), "event_update")
	schema := descriptorSchema(t, CalendarDescriptors(), "event_update")

	if descriptor.Version != "3" {
		t.Fatalf("event_update version = %q", descriptor.Version)
	}
	assertSchemaHasProperties(t, schema, "eventHint", "title", "note", "location", "startsAt", "endsAt", "isWholeDay", "participantPersonHints", "notifyMinutesBefore")
	assertSchemaOmitsProperties(t, schema, "query", "eventID")
	assertSchemaRequires(t, schema, "eventHint")
	if schema.MinProperties != 2 {
		t.Fatalf("event_update minProperties = %d", schema.MinProperties)
	}
	for _, fieldName := range []string{"title", "description", "location", "startISO", "endISO"} {
		if stringSliceContains(schema.Required, fieldName) {
			t.Fatalf("expected omitted %s to preserve the stored value", fieldName)
		}
	}
}

func TestCalendarDescriptorIncludesEventDeleteInput(t *testing.T) {
	schema := descriptorSchema(t, CalendarDescriptors(), "event_delete")

	assertSchemaHasProperties(t, schema, "eventHint")
	assertSchemaOmitsProperties(t, schema, "query", "eventID")
	assertSchemaRequires(t, schema, "eventHint")
	if descriptorForTool(t, CalendarDescriptors(), "event_delete").Version != "2" {
		t.Fatal("event_delete descriptor must use the canonical-result v2 contract")
	}
}

func TestRegisteredDescriptorsRequireTypedContractsWhenModelVisible(t *testing.T) {
	descriptors := DefaultToolDescriptors()
	if errorValue := capabilityprotocol.ValidateModelVisibleCapabilityDescriptorSet(descriptors); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, descriptor := range descriptors {
		if descriptor.ModelVisibility != capabilityprotocol.ModelVisibilityVisible && !descriptor.ModelVisible {
			continue
		}
		if descriptor.ResultContract == nil || len(descriptor.ResultContract.Schema) == 0 {
			t.Fatalf("model-visible descriptor lacks a result contract: %+v", descriptor)
		}
	}
}

func TestWebDescriptorsUseCanonicalSearchAndFetch(t *testing.T) {
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
	if fetchDescriptor.ModelVisibility != capabilityprotocol.ModelVisibilityVisible || !fetchDescriptor.ModelVisible || fetchDescriptor.ResultContract == nil {
		t.Fatalf("web_fetch must be model-visible with a result contract: %+v", fetchDescriptor)
	}
	fetchResultSchema := decodeSchema(t, "web_fetch result", fetchDescriptor.ResultContract.Schema)
	assertSchemaRequires(t, fetchResultSchema, "provider", "remoteLLMInvolved", "compatibility", "results", "errors")
}

func TestMailDescriptorsShowTheToolsTheMailSkillCalls(t *testing.T) {
	for _, toolName := range []string{"mail_connection_status", "mail_connection_start", "mail_message_list", "mail_message_search", "mail_message_read", "mail_message_send"} {
		descriptor := descriptorForTool(t, MailDescriptors(), toolName)
		if !descriptor.ModelVisible || descriptor.ResultContract == nil {
			t.Fatalf("%s must be model-visible with a result contract: %+v", toolName, descriptor)
		}
	}
}

func TestFlowDescriptorUsesTypedTaskCreateInput(t *testing.T) {
	schema := descriptorSchema(t, TaskToolDescriptors(), "task_add")

	assertSchemaHasProperties(t, schema, "title", "size", "status", "business", "type", "startsAt", "endsAt", "participantPersonHints", "parentTaskHint")
	assertSchemaRequires(t, schema, "title")
	if stringSliceContains(schema.Required, "goal") || stringSliceContains(schema.Required, "endDate") {
		t.Fatalf("expected goal and endDate to be optional in %+v", schema.Required)
	}
	assertSchemaOmitsProperties(t, schema, "prompt", "content", "description", "assignee", "dueDate", "ownerID", "participantIDs", "weekCode", "allowDuplicate")
	for _, descriptor := range TaskToolDescriptors() {
		if descriptor.Name == "task_add" && descriptor.Version != "7" {
			t.Fatalf("task_add version = %q", descriptor.Version)
		}
	}
}

func TestFlowListDescriptorMatchesTaskLookupInput(t *testing.T) {
	schema := descriptorSchema(t, TaskToolDescriptors(), "task_list")

	assertSchemaHasProperties(t, schema, "query", "personHints", "scope", "weekFrom", "weekTo", "status", "everyWeek", "limit")
	assertSchemaOmitsProperties(t, schema, "weekCode", "title", "description", "assignee", "dueDate")
}

func TestTaskBoardDescriptorKeepsHistoryReadSeparate(t *testing.T) {
	schema := descriptorSchema(t, TaskToolDescriptors(), "task_board_get")
	assertSchemaRequires(t, schema, "boardWeek")
	assertSchemaHasProperties(t, schema, "boardWeek", "personHints", "scope")
	assertSchemaOmitsProperties(t, schema, "everyWeek", "weekFrom", "weekTo")
	assertSchemaOmitsProperties(t, descriptorSchema(t, TaskToolDescriptors(), "task_list"), "boardWeek")
}

func TestFlowDescriptorIncludesTaskUpdateInput(t *testing.T) {
	schema := descriptorSchema(t, TaskToolDescriptors(), "task_update")

	assertSchemaHasProperties(t, schema, "taskHint", "title", "status", "size", "business", "type", "startsAt", "endsAt", "participantPersonHints", "parentTaskHint", "childTaskHints")
	assertSchemaOmitsProperties(t, schema, "query", "targetPersonHint", "ownerPersonHint", "requestReason", "decisionReason", "weekCode", "prompt", "allowDuplicate", "content", "taskID")
	assertSchemaRequires(t, schema, "taskHint")
	if schema.MinProperties != 2 {
		t.Fatalf("task_update minProperties = %d", schema.MinProperties)
	}
	if descriptorForTool(t, TaskToolDescriptors(), "task_update").Version != "6" {
		t.Fatal("task_update descriptor must use the canonical-result v6 contract")
	}
	assertDescriptorCompletionEvidence(t, TaskToolDescriptors(), "task_update", "success", "write_task", "task")
}

func TestFlowDescriptorIncludesTaskDeleteInput(t *testing.T) {
	schema := descriptorSchema(t, TaskToolDescriptors(), "task_delete")

	assertSchemaHasProperties(t, schema, "taskHint")
	assertSchemaOmitsProperties(t, schema, "query", "targetPersonHint", "ownerPersonHint", "requestReason", "decisionReason", "weekCode", "prompt", "allowDuplicate", "content", "taskID")
	assertSchemaRequires(t, schema, "taskHint")
	if descriptorForTool(t, TaskToolDescriptors(), "task_delete").Version != "3" {
		t.Fatal("task_delete descriptor must use the canonical-result v3 contract")
	}
	assertDescriptorApproval(t, TaskToolDescriptors(), "task_delete", true)
	assertDescriptorCompletionEvidence(t, TaskToolDescriptors(), "task_delete", "success", "delete_task", "task")
}

func TestFlowDescriptorsDeclareCanonicalTaskResults(t *testing.T) {
	expectedEffects := map[string]string{
		"task_add":    "created",
		"task_update": "updated",
		"task_delete": "deleted",
	}
	for toolName, expectedEffect := range expectedEffects {
		descriptor := descriptorForTool(t, TaskToolDescriptors(), toolName)
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
	listDescriptor := descriptorForTool(t, TaskToolDescriptors(), "task_list")
	if listDescriptor.ResultContract == nil || len(listDescriptor.ResultContract.Effects) != 0 {
		t.Fatalf("task_list result contract = %+v", listDescriptor.ResultContract)
	}
}

func TestCalendarDescriptorsDeclareCanonicalResults(t *testing.T) {
	expectedEffects := map[string]string{
		"event_add":    "created",
		"event_update": "updated",
		"event_delete": "deleted",
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
	listDescriptor := descriptorForTool(t, CalendarDescriptors(), "event_list")
	if listDescriptor.ResultContract == nil || len(listDescriptor.ResultContract.Effects) != 0 {
		t.Fatalf("event_list result contract = %+v", listDescriptor.ResultContract)
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
	assertSchemaHasProperties(t, updateSchema, "messageID", "oldText", "newText", "isPinned")
	assertSchemaOmitsProperties(t, updateSchema, "message")
	assertSchemaRequires(t, updateSchema, "messageID")
	assertSchemaHasProperties(t, deleteSchema, "messageIDs")
	assertSchemaRequires(t, deleteSchema, "messageIDs")
	assertSchemaOmitsProperties(t, deleteSchema, "scope", "targetType", "authoredBy", "query", "queries", "limit", "cursor")
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message_context", false)
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message_search", false)
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message_send", true)
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message_update", false)
	assertDescriptorApproval(t, PlatformMessageDescriptors(), "message_delete", true)
	assertDescriptorCompletionEvidence(t, PlatformMessageDescriptors(), "message_send", "success", "send_message", "message")
	assertDescriptorCompletionEvidence(t, PlatformMessageDescriptors(), "message_update", "success", "update_message", "message")
	assertDescriptorCompletionEvidence(t, PlatformMessageDescriptors(), "message_delete", "success", "delete_message", "message")
	expectedEffects := map[string]struct {
		effect      string
		resultField string
		version     string
	}{
		"message_send":   {effect: "sent", resultField: "messageIDs", version: "3"},
		"message_update": {effect: "updated", resultField: "messageID", version: "2"},
		"message_delete": {effect: "deleted", resultField: "messageIDs", version: "2"},
	}
	for toolName, expected := range expectedEffects {
		descriptor := descriptorForTool(t, descriptors, toolName)
		if descriptor.Version != expected.version || descriptor.ModelVisibility != capabilityprotocol.ModelVisibilityVisible || !descriptor.ModelVisible {
			t.Fatalf("%s must use its visible generated v%s descriptor: %+v", toolName, expected.version, descriptor)
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
		{descriptors: BrowserToolDescriptors(), toolNames: []string{"browser_open", "browser_snapshot", "browser_screenshot", "browser_click", "browser_fill", "browser_select", "browser_press", "browser_wait"}},
	} {
		for _, toolName := range descriptorSet.toolNames {
			descriptor := descriptorForTool(t, descriptorSet.descriptors, toolName)
			if descriptor.ResultContract == nil || len(descriptor.ResultContract.Effects) != 0 {
				t.Fatalf("%s result contract = %+v", toolName, descriptor.ResultContract)
			}
		}
	}

	openSchema := descriptorSchema(t, BrowserToolDescriptors(), "browser_open")
	snapshotSchema := descriptorSchema(t, BrowserToolDescriptors(), "browser_snapshot")
	clickSchema := descriptorSchema(t, BrowserToolDescriptors(), "browser_click")
	assertSchemaHasProperties(t, openSchema, "url")
	assertSchemaRequires(t, openSchema, "url")
	assertSchemaOmitsProperties(t, openSchema, "startURL")
	assertSchemaOmitsProperties(t, snapshotSchema, "interactive")
	assertSchemaHasProperties(t, clickSchema, "target", "ref", "selector")
	if clickSchema.MinProperties != 1 {
		t.Fatalf("browser_click minProperties = %d, want 1", clickSchema.MinProperties)
	}
}

func TestContractedDefaultToolsRemainModelVisible(t *testing.T) {
	defaultDescriptors := DefaultToolDescriptors()
	for _, toolName := range []string{
		"browser_open",
		"browser_snapshot",
		"browser_screenshot",
		"browser_click",
		"browser_fill",
		"browser_select",
		"browser_press",
		"browser_wait",
		"image_generate",
		"mail_message_mark",
		"mail_message_move",
		"message_context",
		"message_search",
		"message_send",
		"message_update",
		"message_delete",
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
		"company_document_upload",
		"company_document_download",
		"company_image_upload",
	} {
		descriptor := descriptorForTool(t, defaultDescriptors, toolName)
		if descriptor.ModelVisibility != capabilityprotocol.ModelVisibilityVisible || !descriptor.ModelVisible || descriptor.ResultContract == nil {
			t.Fatalf("%s must remain typed and model-visible: %+v", toolName, descriptor)
		}
	}
}

// The agent offers one `read` tool and routes to these by content type, so they
// keep their wire contract while leaving the catalog the model reads.
func TestContractedReadBackendsStayTypedAndHidden(t *testing.T) {
	defaultDescriptors := DefaultToolDescriptors()
	for _, toolName := range []string{"document_read", "image_read"} {
		descriptor := descriptorForTool(t, defaultDescriptors, toolName)
		if descriptor.ModelVisibility != capabilityprotocol.ModelVisibilityHidden || descriptor.ModelVisible {
			t.Fatalf("%s must stay hidden from the model: %+v", toolName, descriptor)
		}
		if descriptor.ResultContract == nil {
			t.Fatalf("%s must keep its typed result contract: %+v", toolName, descriptor)
		}
	}
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
	assertDescriptorApproval(t, CalendarDescriptors(), "event_add", false)
	assertDescriptorApproval(t, MailDescriptors(), "mail_connection_start", true)
	assertDescriptorApproval(t, MailDescriptors(), "mail_message_send", true)
	assertDescriptorApproval(t, MailDescriptors(), "mail_message_search", false)
	assertDescriptorApproval(t, WebDescriptors(), "web_search", false)
	assertDescriptorApproval(t, WebDescriptors(), "web_fetch", false)
	assertDescriptorApproval(t, CalendarDescriptors(), "event_delete", true)
}

func TestCapabilityDescriptorsExposeCompletionEvidence(t *testing.T) {
	assertDescriptorCompletionEvidence(t, PlatformMessageDescriptors(), "message_send", "success", "send_message", "message")
	assertDescriptorCompletionEvidence(t, MailDescriptors(), "mail_message_send", "success", "send_email", "email")
	assertDescriptorCompletionEvidence(t, CalendarDescriptors(), "event_add", "success", "write_calendar", "calendar")
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

func TestCapabilityDescriptorSchemasAreCanonicalObjects(t *testing.T) {
	descriptorGroups := [][]Descriptor{
		BrowserToolDescriptors(),
		WebDescriptors(),
		FileDescriptors(),
		PlatformMessageDescriptors(),
		TaskToolDescriptors(),
		CalendarDescriptors(),
		MailDescriptors(),
		ArtifactDescriptors(),
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
	for _, toolName := range []string{"message_send", "mail_message_send"} {
		descriptor := descriptorForTool(t, DefaultToolDescriptors(), toolName)
		if !descriptor.Idempotency.Supported || descriptor.Idempotency.Scope != "operation" {
			t.Fatalf("%s must explicitly support operation idempotency: %+v", toolName, descriptor)
		}
	}
	if descriptorForTool(t, TaskToolDescriptors(), "task_add").Idempotency.Supported {
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
