package cli

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadMattermostScenarioAcceptsExpensiveLifecycleShape(t *testing.T) {
	for _, filename := range []string{
		"01-task-lifecycle.json",
		"02-calendar-lifecycle.json",
		"04-website-lifecycle.json",
		"05-message-lifecycle.json",
		"09-document-lifecycle.json",
	} {
		scenario, errorValue := loadMattermostScenario("../../tests/expensive/" + filename)
		if errorValue != nil {
			t.Fatalf("load %s: %v", filename, errorValue)
		}
		if scenario.Name == "" || len(scenario.Steps) == 0 {
			t.Fatalf("expected named ordered lifecycle steps for %s, got %#v", filename, scenario)
		}
	}
}

func TestMessageLifecycleUsesCanonicalSearchMutationLineageAndButtonApproval(t *testing.T) {
	scenario, errorValue := loadMattermostScenario("../../tests/expensive/05-message-lifecycle.json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(scenario.Steps) != 5 {
		t.Fatalf("expected message CRUD and final absence verification, got %d steps", len(scenario.Steps))
	}
	if len(scenario.InitialToolNames) != 1 || scenario.InitialToolNames[0] != "message.send" {
		t.Fatalf("unexpected initial message tools: %#v", scenario.InitialToolNames)
	}
	for _, stepIndex := range []int{1, 3} {
		step := scenario.Steps[stepIndex]
		if !containsMattermostScenarioString(step.ExpectedToolCalls, "message.search") {
			t.Fatalf("step %d does not search before mutation: %#v", stepIndex+1, step)
		}
		if step.ApprovalAction != mattermostScenarioApprovalApprove {
			t.Fatalf("step %d does not use button approval: %#v", stepIndex+1, step)
		}
	}
}

func TestMattermostScenarioMessageMutationRequiresSearchAndScenarioLineage(t *testing.T) {
	result := mattermostScenarioResult{Steps: []mattermostScenarioStepResult{{
		TaskEvents: []mattermostScenarioTaskEvent{{
			Name: "tool.message.send.result",
			Body: `{"output":{"data":{"messageIDs":["message-1"],"deliveryStatus":"sent"}}}`,
		}},
	}}}
	expected := mattermostScenarioStep{ExpectedToolCalls: []string{"message.search", "message.update"}}
	events := []mattermostScenarioTaskEvent{
		{Name: "tool.message.search.result", Body: `{"output":{"data":{"messageIDs":["message-1"],"candidates":[]}}}`},
		{Name: "tool.message.update.requested", Body: `{"input":{"messageID":"message-1","message":"수정"}}`},
	}
	if errorValue := validateMattermostScenarioMessageMutationIDs(1, expected, events, &result); errorValue != nil {
		t.Fatalf("validate exact message lineage: %v", errorValue)
	}

	events[1].Body = `{"input":{"messageID":"message-2","message":"수정"}}`
	errorValue := validateMattermostScenarioMessageMutationIDs(1, expected, events, &result)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "message.search returned it") {
		t.Fatalf("expected unobserved message ID failure, got %v", errorValue)
	}
}

func TestMattermostScenarioMessageMutationRejectsSearchResultOutsidePriorLineage(t *testing.T) {
	result := mattermostScenarioResult{Steps: []mattermostScenarioStepResult{{
		TaskEvents: []mattermostScenarioTaskEvent{{
			Name: "tool.message.update.result",
			Body: `{"output":{"data":{"messageID":"message-1","deliveryStatus":"updated","messageUpdated":true}}}`,
		}},
	}}}
	expected := mattermostScenarioStep{ExpectedToolCalls: []string{"message.search", "message.delete"}}
	events := []mattermostScenarioTaskEvent{
		{Name: "tool.message.search.result", Body: `{"output":{"data":{"messageIDs":["message-2"],"candidates":[]}}}`},
		{Name: "tool.message.delete.requested", Body: `{"input":{"messageIDs":["message-2"]}}`},
	}
	errorValue := validateMattermostScenarioMessageMutationIDs(1, expected, events, &result)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "outside the scenario mutation lineage") {
		t.Fatalf("expected scenario lineage failure, got %v", errorValue)
	}
}

func TestWebsiteLifecycleResolvesCanonicalSiteIdentityForEveryMutation(t *testing.T) {
	scenario, errorValue := loadMattermostScenario("../../tests/expensive/04-website-lifecycle.json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if scenario.Steps[0].ExpectedToolCallCounts["site.status"] != 1 || scenario.Steps[0].ExpectedExactToolCallCounts["site.create"] != 1 {
		t.Fatalf("unexpected create discovery contract: %#v", scenario.Steps[0])
	}
	for stepIndex, step := range scenario.Steps[1:] {
		if !containsMattermostScenarioString(step.ExpectedToolCalls, "site.status") {
			t.Fatalf("step %d does not resolve the site before mutation: %#v", stepIndex+2, step)
		}
		if !strings.Contains(step.Prompt, "브릿지웍스 상담 안내 사이트") {
			t.Fatalf("step %d does not identify the site independently: %q", stepIndex+2, step.Prompt)
		}
	}
}

func TestDocumentLifecycleUsesCanonicalReadAndButtonApproval(t *testing.T) {
	scenario, errorValue := loadMattermostScenario("../../tests/expensive/09-document-lifecycle.json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(scenario.Steps) != 5 {
		t.Fatalf("expected one delete turn with button approval, got %d steps", len(scenario.Steps))
	}
	if !containsMattermostScenarioString(scenario.CapabilityToolNames, "document.read") ||
		!containsMattermostScenarioString(scenario.InitialToolNames, "document.read") ||
		containsMattermostScenarioString(scenario.AllowedTools, "file.preview") {
		t.Fatalf("unexpected document tool exposure: %#v", scenario)
	}
	for _, stepIndex := range []int{1, 3} {
		if !containsMattermostScenarioString(scenario.Steps[stepIndex].ExpectedToolCalls, "document.read") {
			t.Fatalf("step %d does not use document.read: %#v", stepIndex+1, scenario.Steps[stepIndex])
		}
	}
	deleteStep := scenario.Steps[len(scenario.Steps)-1]
	if deleteStep.ApprovalAction != mattermostScenarioApprovalApprove ||
		!containsMattermostScenarioString(deleteStep.ExpectedEvents, "confirmation.requested") ||
		!containsMattermostScenarioString(deleteStep.ExpectedEvents, "approval.executed") {
		t.Fatalf("unexpected delete approval flow: %#v", deleteStep)
	}
	skillDocument, errorValue := os.ReadFile("../../assets/blueclaw-workspace/skills/document/SKILL.md")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(skillDocument), "tool-references: document.read") {
		t.Fatal("document skill does not reference document.read")
	}
}

func TestMattermostScenarioRequiresAuthoritativeSDKDProvenance(t *testing.T) {
	scenario := mattermostScenario{Name: "sdkd", RequiresSDKD: true, Steps: []mattermostScenarioStep{{Prompt: "work"}}}
	result := mattermostScenarioResult{ScenarioName: "sdkd", Steps: []mattermostScenarioStepResult{{
		Prompt: "work",
		TaskEvents: []mattermostScenarioTaskEvent{
			{Name: "llm.call", Body: `{"kind":"structured","schemaName":"blueclaw_turn_router","transport":"sdkd","provider":"openrouter","model":"router-model","selectedBackend":"remote"}`},
			{Name: "llm.call", Body: `{"kind":"chat","schemaName":"blueclaw_agent_turn_action","transport":"sdkd","provider":"openrouter","model":"low-model","selectedBackend":"remote"}`},
		},
	}}}
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue != nil {
		t.Fatalf("validate SDKD provenance: %v", errorValue)
	}
	result.Steps[0].TaskEvents[1].Body = `{"kind":"chat","schemaName":"blueclaw_agent_turn_action","transport":"capability","provider":"openrouter","model":"low-model","selectedBackend":"remote","usedFallback":true}`
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue == nil {
		t.Fatal("expected legacy fallback provenance to fail")
	}
}

func TestMattermostScenarioRejectsOperationContractFallback(t *testing.T) {
	for _, schemaName := range []string{"blueclaw_operation_contract", "blueclaw_operation_contract_review"} {
		t.Run(schemaName, func(t *testing.T) {
			scenario := mattermostScenario{Name: "sdkd", RequiresSDKD: true, Steps: []mattermostScenarioStep{{Prompt: "work"}}}
			result := mattermostScenarioResult{ScenarioName: "sdkd", Steps: []mattermostScenarioStepResult{{
				Prompt: "work",
				TaskEvents: []mattermostScenarioTaskEvent{
					{Name: "llm.call", Body: `{"schemaName":"blueclaw_turn_router","transport":"sdkd","provider":"openrouter","model":"model","selectedBackend":"remote"}`},
					{Name: "llm.call", Body: `{"schemaName":"` + schemaName + `","transport":"capability","provider":"openrouter","model":"model","selectedBackend":"remote","usedFallback":true}`},
					{Name: "llm.call", Body: `{"schemaName":"blueclaw_agent_turn_action","transport":"sdkd","provider":"openrouter","model":"model","selectedBackend":"remote"}`},
				},
			}}}

			errorValue := validateMattermostScenarioResult(scenario, &result)
			if errorValue == nil || !strings.Contains(errorValue.Error(), schemaName) {
				t.Fatalf("expected %s fallback to fail, got %v", schemaName, errorValue)
			}
		})
	}
}

func TestMattermostScenarioRequiresModelTierAtOrBelowMaximum(t *testing.T) {
	scenario := mattermostScenario{Name: "sdkd", RequiresSDKD: true, MaximumModelTier: "low", Steps: []mattermostScenarioStep{{Prompt: "work"}}}
	result := mattermostScenarioResult{ScenarioName: "sdkd", Steps: []mattermostScenarioStepResult{{
		Prompt: "work",
		TaskEvents: []mattermostScenarioTaskEvent{
			{Name: "llm.call", Body: `{"schemaName":"blueclaw_turn_router","transport":"sdkd","provider":"openrouter","model":"router-model","modelTier":"low","selectedBackend":"remote"}`},
			{Name: "llm.call", Body: `{"schemaName":"blueclaw_agent_turn_action","transport":"sdkd","provider":"openrouter","model":"low-model","modelTier":"low","selectedBackend":"remote"}`},
		},
	}}}
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue != nil {
		t.Fatalf("validate requested model tier: %v", errorValue)
	}
	result.Steps[0].TaskEvents[0].Body = strings.Replace(result.Steps[0].TaskEvents[0].Body, `"modelTier":"low"`, `"modelTier":"xlow"`, 1)
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue != nil {
		t.Fatalf("validate lower model tier: %v", errorValue)
	}
	result.Steps[0].TaskEvents[0].Body = strings.Replace(result.Steps[0].TaskEvents[0].Body, `"modelTier":"xlow"`, `"modelTier":"medium"`, 1)
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue == nil || !strings.Contains(errorValue.Error(), "model tier") {
		t.Fatalf("expected model tier failure, got %v", errorValue)
	}
}

func TestMattermostScenarioRequiresExactSelectedLLMProviderAndModel(t *testing.T) {
	scenario := mattermostScenario{
		Name:                "sdkd",
		RequiresSDKD:        true,
		MaximumModelTier:    "low",
		ExpectedLLMProvider: "openrouter",
		ExpectedLLMModel:    "xiaomi/mimo-v2.5",
		Steps:               []mattermostScenarioStep{{Prompt: "work"}},
	}
	result := mattermostScenarioResult{ScenarioName: "sdkd", Steps: []mattermostScenarioStepResult{{
		Prompt: "work",
		TaskEvents: []mattermostScenarioTaskEvent{
			{Name: "llm.call", Body: `{"schemaName":"blueclaw_turn_router","transport":"sdkd","provider":"openrouter","model":"xiaomi/mimo-v2.5","modelTier":"low","selectedBackend":"remote"}`},
			{Name: "llm.call", Body: `{"schemaName":"blueclaw_agent_turn_action","transport":"sdkd","provider":"openrouter","model":"xiaomi/mimo-v2.5","modelTier":"low","selectedBackend":"remote"}`},
		},
	}}}
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue != nil {
		t.Fatalf("validate exact model provenance: %v", errorValue)
	}
	result.Steps[0].TaskEvents[1].Body = strings.Replace(result.Steps[0].TaskEvents[1].Body, "xiaomi/mimo-v2.5", "other/model", 1)
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue == nil || !strings.Contains(errorValue.Error(), "selected model") {
		t.Fatalf("expected exact model failure, got %v", errorValue)
	}
}

func TestMattermostScenarioRequiresDirectExposureEvidence(t *testing.T) {
	scenario := mattermostScenario{
		Name:                "exposure",
		AllowedTools:        []string{"task.add"},
		InitialToolNames:    []string{"task.add"},
		SkillDirectoryPaths: []string{"skills/internkim-flow"},
		Steps:               []mattermostScenarioStep{{Prompt: "work", ExpectedToolCalls: []string{"task.add"}}},
	}
	result := mattermostScenarioResult{ScenarioName: scenario.Name, Steps: []mattermostScenarioStepResult{{
		Prompt: "work",
		TaskEvents: []mattermostScenarioTaskEvent{
			{Name: "agent.instructions_loaded", Body: `{"exposedToolNames":["task.add"],"selectedSkillToolReferences":{"internkim-flow":["task.add"]}}`},
			{Name: "agent.step_working_set", Body: `{"exposure":{"exposedToolIDs":["task.add"],"selectedSkillToolIDs":["task.add"],"selectionSource":"selected_skills","usedFallbackGroups":false}}`},
			{Name: "tool.task.add.requested"},
		},
	}}}
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue != nil {
		t.Fatalf("validate direct exposure evidence: %v", errorValue)
	}
	result.Steps[0].TaskEvents[1].Body = `{"exposure":{"exposedToolIDs":["task.add"],"pinnedGroupToolIDs":["task.add"],"selectionSource":"deterministic_palette","usedFallbackGroups":true}}`
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue == nil {
		t.Fatal("expected fallback exposure evidence to fail")
	}
}

func TestMattermostScenarioReportsStatusBeforeMissingExposure(t *testing.T) {
	scenario := mattermostScenario{
		Name:             "exposure",
		AllowedTools:     []string{"task.add"},
		InitialToolNames: []string{"task.add"},
		Steps: []mattermostScenarioStep{{
			Prompt:             "업무를 추가해줘",
			ExpectedTaskStatus: "completed",
		}},
	}
	result := mattermostScenarioResult{ScenarioName: scenario.Name, Steps: []mattermostScenarioStepResult{{
		Prompt:     "업무를 추가해줘",
		TaskStatus: "waiting_user_input",
	}}}

	errorValue := validateMattermostScenarioResult(scenario, &result)
	if errorValue == nil || !strings.Contains(errorValue.Error(), `status "waiting_user_input" does not match "completed"`) {
		t.Fatalf("expected actionable status failure, got %v", errorValue)
	}
}

func TestResolveMattermostScenarioExpectedValuesKeepsPromptNatural(t *testing.T) {
	scenario := mattermostScenario{Steps: []mattermostScenarioStep{{
		Prompt: "다가오는 금요일까지 업무를 추가해줘",
		ExpectedEventCounts: []mattermostScenarioEventCount{
			{BodyFragment: "{{nextFriday}}"},
			{OutputFragment: "{{tomorrow}}T10:00:00+09:00"},
		},
	}}}
	resolveMattermostScenarioExpectedValues(&scenario, time.Date(2026, time.July, 16, 12, 0, 0, 0, time.UTC))
	if got := scenario.Steps[0].ExpectedEventCounts[0].BodyFragment; got != "2026-07-17" {
		t.Fatalf("expected next Friday, got %q", got)
	}
	if got := scenario.Steps[0].ExpectedEventCounts[1].OutputFragment; got != "2026-07-17T10:00:00+09:00" {
		t.Fatalf("expected tomorrow in Asia/Seoul, got %q", got)
	}
	if got := scenario.Steps[0].Prompt; got != "다가오는 금요일까지 업무를 추가해줘" {
		t.Fatalf("expected natural user prompt, got %q", got)
	}
}

func TestResolveMattermostScenarioExpectedValuesUsesStrictFutureDatesInAsiaSeoul(t *testing.T) {
	scenario := mattermostScenario{Steps: []mattermostScenarioStep{{
		ExpectedEventCounts: []mattermostScenarioEventCount{
			{BodyFragment: "{{nextFriday}}"},
			{OutputFragment: "{{tomorrow}}"},
		},
	}}}

	resolveMattermostScenarioExpectedValues(&scenario, time.Date(2026, time.July, 17, 16, 0, 0, 0, time.UTC))

	if got := scenario.Steps[0].ExpectedEventCounts[0].BodyFragment; got != "2026-07-24" {
		t.Fatalf("expected strict-future Friday in Asia/Seoul, got %q", got)
	}
	if got := scenario.Steps[0].ExpectedEventCounts[1].OutputFragment; got != "2026-07-19" {
		t.Fatalf("expected tomorrow in Asia/Seoul, got %q", got)
	}
}

func TestMattermostScenarioCalendarMutationUsesListedEventID(t *testing.T) {
	expected := mattermostScenarioStep{ExpectedToolCalls: []string{"calendar.list", "calendar.update"}}
	events := []mattermostScenarioTaskEvent{
		{Name: "tool.calendar.list.result", Body: `{"output":{"data":{"events":[{"eventID":"event-1"}]}}}`},
		{Name: "tool.calendar.update.requested", Body: `{"input":{"eventID":"event-1"}}`},
	}

	if errorValue := validateMattermostScenarioCalendarMutationIDs(0, expected, events); errorValue != nil {
		t.Fatalf("validate listed calendar event ID: %v", errorValue)
	}

	events[1].Body = `{"input":{"eventID":"event-2"}}`
	if errorValue := validateMattermostScenarioCalendarMutationIDs(0, expected, events); errorValue == nil {
		t.Fatal("expected unlisted calendar event ID to fail")
	}
}

func TestMattermostScenarioCalendarMutationRequiresListBeforeMutation(t *testing.T) {
	expected := mattermostScenarioStep{ExpectedToolCalls: []string{"calendar.list", "calendar.delete"}}
	events := []mattermostScenarioTaskEvent{
		{Name: "tool.calendar.delete.requested", Body: `{"input":{"eventID":"event-1"}}`},
		{Name: "tool.calendar.list.result", Body: `{"output":{"data":{"events":[{"eventID":"event-1"}]}}}`},
	}

	if errorValue := validateMattermostScenarioCalendarMutationIDs(0, expected, events); errorValue == nil {
		t.Fatal("expected calendar mutation before list to fail")
	}
}

func TestMattermostScenarioDoesNotAcceptRecoveryChatInsteadOfAgentAction(t *testing.T) {
	scenario := mattermostScenario{Name: "sdkd", RequiresSDKD: true, Steps: []mattermostScenarioStep{{Prompt: "work"}}}
	result := mattermostScenarioResult{ScenarioName: "sdkd", Steps: []mattermostScenarioStepResult{{
		Prompt: "work",
		TaskEvents: []mattermostScenarioTaskEvent{
			{Name: "llm.call", Body: `{"kind":"structured","schemaName":"blueclaw_turn_router","transport":"sdkd","provider":"openrouter","model":"router-model","selectedBackend":"remote"}`},
			{Name: "llm.call", Body: `{"kind":"chat","schemaName":"blueclaw_agent_turn_action","transport":"sdkd","isError":true}`},
			{Name: "llm.call", Body: `{"kind":"recovery_chat","transport":"sdkd","provider":"openrouter","model":"low-model","selectedBackend":"remote"}`},
		},
	}}}
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue == nil || !strings.Contains(errorValue.Error(), "blueclaw_agent_turn_action") {
		t.Fatalf("expected failed agent action to remain a failure, got %v", errorValue)
	}
}

func TestMattermostScenarioValidatesDOCXPackageAndText(t *testing.T) {
	document := createScenarioDOCX(t, "분기 결산 운영 검토 초안 운영팀")
	expected := mattermostScenarioStep{ExpectedDocumentText: []string{"분기 결산 운영 검토", "초안", "운영팀"}}
	files := []downloadedMattermostFile{{Filename: "report.docx", ContentBase64: base64.StdEncoding.EncodeToString(document)}}
	if errorValue := validateMattermostScenarioDocuments(0, expected, files); errorValue != nil {
		t.Fatalf("validate DOCX: %v", errorValue)
	}
	expected.ExpectedDocumentText = append(expected.ExpectedDocumentText, "재무팀")
	if errorValue := validateMattermostScenarioDocuments(0, expected, files); errorValue == nil {
		t.Fatal("expected missing DOCX text to fail")
	}
}

func createScenarioDOCX(t *testing.T, text string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entries := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"_rels/.rels":         `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"word/document.xml":   `<?xml version="1.0"?><document><body><p>` + text + `</p></body></document>`,
	}
	for name, content := range entries {
		entry, errorValue := writer.Create(name)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if _, errorValue := entry.Write([]byte(content)); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if errorValue := writer.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	return buffer.Bytes()
}

func TestLoadMattermostScenarioRejectsInvalidBoundary(t *testing.T) {
	path := t.TempDir() + "/scenario.json"
	if errorValue := os.WriteFile(path, []byte(`{"name":"invalid","steps":[{"prompt":"","minimumReplyLength":-1}]}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue := loadMattermostScenario(path)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "prompt") {
		t.Fatalf("expected prompt validation error, got %v", errorValue)
	}
}

func TestValidateMattermostScenarioResultChecksRepliesEventsStatusAndAttachments(t *testing.T) {
	scenario := mattermostScenario{
		Name: "lifecycle",
		Steps: []mattermostScenarioStep{{
			Prompt:                 "do the work",
			ExpectedTaskStatus:     "completed",
			ExpectedReplyFragments: []string{"done"},
			ExpectedToolCalls:      []string{"task.add"},
			ExpectedEvents:         []string{"task.completed"},
			ExpectedEventCounts: []mattermostScenarioEventCount{{
				Name:         "tool.task.add.result",
				BodyFragment: "task.add",
				Count:        1,
			}},
			ExpectedAttachments: []string{"report.docx"},
		}},
	}
	taskDetail := mattermostScenarioTaskDetail{
		TaskRun: mattermostScenarioTaskRun{TaskRunID: "task-1", Status: "completed"},
		TaskEvents: []mattermostScenarioTaskEvent{
			{Name: "task.completed"},
			{Name: "tool.task.add.requested", Body: `{"operation":"task.add"}`},
			{Name: "tool.task.add.result", Body: "task.add report"},
		},
	}
	result := mattermostScenarioResult{
		ScenarioName: "lifecycle",
		ChannelID:    "channel-1",
		UserID:       "user-1",
		Steps: []mattermostScenarioStepResult{{
			Prompt:      "do the work",
			TaskRunID:   "task-1",
			TaskStatus:  "completed",
			BotMessage:  "done",
			TaskEvents:  taskDetail.TaskEvents,
			Attachments: []downloadedMattermostFile{{Filename: "report.docx"}},
		}},
	}
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue != nil {
		t.Fatalf("validate result: %v", errorValue)
	}
	result.Steps[0].BotMessage = "not finished"
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue == nil {
		t.Fatal("expected reply fragment failure")
	}
}

func TestMattermostScenarioCountsRecordEfficiencyButGatePresence(t *testing.T) {
	scenario := mattermostScenario{Name: "counts", Steps: []mattermostScenarioStep{{
		Prompt:                      "count",
		ExpectedToolCallCounts:      map[string]int{"task.list": 2},
		ExpectedExactToolCallCounts: map[string]int{"task.update": 0},
		ExpectedEventCounts:         []mattermostScenarioEventCount{{Name: "task.completed", Count: 2}},
	}}}
	result := mattermostScenarioResult{
		ScenarioName: "counts", ChannelID: "channel", UserID: "user",
		Steps: []mattermostScenarioStepResult{{
			Prompt: "count", TaskStatus: "completed", TaskEvents: []mattermostScenarioTaskEvent{
				{Name: "tool.task.list.requested"},
				{Name: "task.completed"},
			},
		}},
	}
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue != nil {
		t.Fatalf("expected positive presence to pass: %v", errorValue)
	}
	if len(result.EfficiencyObservations) != 2 || result.EfficiencyObservations[0].Observed != 1 {
		t.Fatalf("expected one efficiency observation, got %#v", result.EfficiencyObservations)
	}
	result.Steps[0].TaskEvents = append(result.Steps[0].TaskEvents, mattermostScenarioTaskEvent{Name: "tool.task.update.requested"})
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue == nil {
		t.Fatal("expected forbidden zero count to fail")
	}
}

func TestMattermostScenarioMissingEventReportsOutputExpectation(t *testing.T) {
	scenario := mattermostScenario{Name: "task", Steps: []mattermostScenarioStep{{
		Prompt: "업무를 추가해줘",
		ExpectedEventCounts: []mattermostScenarioEventCount{{
			Name:           "tool.task.add.result",
			OutputFragment: "2026-07-24",
			Count:          1,
		}},
	}}}
	result := mattermostScenarioResult{ScenarioName: "task", Steps: []mattermostScenarioStepResult{{
		Prompt: "업무를 추가해줘",
		TaskEvents: []mattermostScenarioTaskEvent{{
			Name: "tool.task.add.result",
			Body: `{"output":{"data":{"title":"고객지원 분기 결산"}}}`,
		}},
	}}}

	errorValue := validateMattermostScenarioResult(scenario, &result)
	if errorValue == nil || !strings.Contains(errorValue.Error(), `event "tool.task.add.result" with output containing "2026-07-24"`) {
		t.Fatalf("expected missing output detail, got %v", errorValue)
	}
}

func TestMattermostScenarioRejectsDuplicateMutatingCount(t *testing.T) {
	scenario := mattermostScenario{Name: "counts", Steps: []mattermostScenarioStep{{
		Prompt:                      "count",
		ExpectedExactToolCallCounts: map[string]int{"task.add": 1},
	}}}
	result := mattermostScenarioResult{ScenarioName: "counts", Steps: []mattermostScenarioStepResult{{
		Prompt: "count",
		TaskEvents: []mattermostScenarioTaskEvent{
			{Name: "tool.task.add.requested"},
			{Name: "tool.task.add.result"},
		},
	}}}
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue != nil {
		t.Fatalf("expected one mutation to pass: %v", errorValue)
	}
	result.Steps[0].TaskEvents = append(result.Steps[0].TaskEvents, mattermostScenarioTaskEvent{Name: "tool.task.add.requested"})
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue == nil || !strings.Contains(errorValue.Error(), "exact tool") {
		t.Fatalf("expected duplicate mutation to fail, got %v", errorValue)
	}
}

func TestMattermostScenarioTreatsReadCountMismatchAsObservation(t *testing.T) {
	scenario := mattermostScenario{Name: "counts", Steps: []mattermostScenarioStep{{
		Prompt:                 "count",
		ExpectedToolCallCounts: map[string]int{"task.list": 0},
	}}}
	result := mattermostScenarioResult{ScenarioName: "counts", Steps: []mattermostScenarioStepResult{{
		Prompt:     "count",
		TaskEvents: []mattermostScenarioTaskEvent{{Name: "tool.task.list.requested"}},
	}}}
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue != nil {
		t.Fatalf("expected read count mismatch to remain observational: %v", errorValue)
	}
	if len(result.EfficiencyObservations) != 1 || result.EfficiencyObservations[0].Observed != 1 {
		t.Fatalf("expected read mismatch observation, got %#v", result.EfficiencyObservations)
	}
}

func TestMaximumMattermostScenarioModelTierPreservesProductionMode(t *testing.T) {
	for _, modelTier := range []string{"xlow", "low", "medium", "high", "xhigh", "max"} {
		if got := maximumMattermostScenarioModelTier(testCommandConfiguration{MaximumModelTier: modelTier}); got != modelTier {
			t.Fatalf("expected %s model maximum, got %q", modelTier, got)
		}
	}
	if got := maximumMattermostScenarioModelTier(testCommandConfiguration{ShouldUseRealModels: true}); got != "" {
		t.Fatalf("expected production mode to remain unrestricted, got %q", got)
	}
}

func TestMattermostScenarioRejectsUnsupportedApprovalAction(t *testing.T) {
	scenario := mattermostScenario{Name: "approval", Steps: []mattermostScenarioStep{{Prompt: "삭제해줘", ApprovalAction: "accept"}}}
	errorValue := validateMattermostScenario(scenario)
	if errorValue == nil || !strings.Contains(errorValue.Error(), `approvalAction "accept" is unsupported`) {
		t.Fatalf("unexpected error: %v", errorValue)
	}
}

func TestMattermostScenarioOutputFragmentExcludesToolRequestEcho(t *testing.T) {
	expectation := mattermostScenarioEventCount{
		Name:           "tool.task.update.result",
		OutputFragment: "고객지원 분기 결산 검토 완료",
		Count:          1,
	}
	events := []mattermostScenarioTaskEvent{{
		Name: "tool.task.update.result",
		Body: `{"output":{"data":{"content":"고객지원 분기 결산 누락 항목 확인"}},"toolInputKey":"task.update request 고객지원 분기 결산 검토 완료"}`,
	}}
	if count := countMattermostScenarioExpectedEvents(events, expectation); count != 0 {
		t.Fatalf("expected request echo to be excluded, got %d", count)
	}
	events[0].Body = `{"output":{"data":{"content":"고객지원 분기 결산 검토 완료"}},"toolInputKey":"task.update"}`
	if count := countMattermostScenarioExpectedEvents(events, expectation); count != 1 {
		t.Fatalf("expected output data to match, got %d", count)
	}
	events[0].Body = `{"output":{"data":{"content":"고객지원 분기 결산 누락 항목 확인"}},"summary":"고객지원 분기 결산 검토 완료"}`
	if count := countMattermostScenarioExpectedEvents(events, expectation); count != 0 {
		t.Fatalf("expected summary text to be excluded, got %d", count)
	}
}

func TestMattermostScenarioOutputFragmentRequiresToolResult(t *testing.T) {
	validScenario := mattermostScenario{
		Name: "valid",
		Steps: []mattermostScenarioStep{{
			Prompt: "검토해줘",
			ExpectedEventCounts: []mattermostScenarioEventCount{{
				Name:           "tool.task.update.result",
				OutputFragment: "완료",
			}},
		}},
	}
	if errorValue := validateMattermostScenario(validScenario); errorValue != nil {
		t.Fatalf("expected direct tool result to allow outputFragment: %v", errorValue)
	}
	invalidScenario := validScenario
	invalidScenario.Steps = []mattermostScenarioStep{{
		Prompt: "검토해줘",
		ExpectedEventCounts: []mattermostScenarioEventCount{{
			Name:           "tool.task.update.requested",
			OutputFragment: "완료",
		}},
	}}
	if errorValue := validateMattermostScenario(invalidScenario); errorValue == nil {
		t.Fatal("expected outputFragment on a request event to be rejected")
	}
	invalidScenario.Steps[0].ExpectedEventCounts[0].Name = "tool.capability.invoke.result"
	if errorValue := validateMattermostScenario(invalidScenario); errorValue == nil {
		t.Fatal("expected outputFragment on the hidden dispatcher result to be rejected")
	}
}

func TestMattermostScenarioAnyToolExpectationNeedsOneCandidate(t *testing.T) {
	scenario := mattermostScenario{Name: "any", Steps: []mattermostScenarioStep{{Prompt: "any", ExpectedAnyToolCalls: []string{"file.edit", "file.write"}}}}
	result := mattermostScenarioResult{
		ScenarioName: "any", ChannelID: "channel", UserID: "user",
		Steps: []mattermostScenarioStepResult{{Prompt: "any", TaskEvents: []mattermostScenarioTaskEvent{{Name: "tool.file.write.requested"}}}},
	}
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue != nil {
		t.Fatalf("expected one any-tool candidate to pass: %v", errorValue)
	}
}

func TestMattermostScenarioTaskDetailAcceptsObjectOrArray(t *testing.T) {
	for _, document := range []string{`{"taskRun":{"taskRunID":"task-1"},"taskEvents":[]}`, `[{"taskRun":{"taskRunID":"task-1"},"taskEvents":[]}]`} {
		var detail mattermostScenarioTaskDetail
		if errorValue := json.Unmarshal([]byte(document), &detail); errorValue != nil || detail.TaskRun.TaskRunID != "task-1" {
			t.Fatalf("expected task detail shape to normalize: %v %#v", errorValue, detail)
		}
	}
}

func TestMattermostScenarioBoundaryRejectsUnknownAndTrailingData(t *testing.T) {
	path := t.TempDir() + "/scenario.json"
	for _, document := range []string{`{"name":"scenario","steps":[{"prompt":"ok"}],"unknown":true}`, `{"name":"scenario","steps":[{"prompt":"ok"}]} {"name":"extra"}`} {
		if errorValue := os.WriteFile(path, []byte(document), 0o600); errorValue != nil {
			t.Fatal(errorValue)
		}
		if _, errorValue := loadMattermostScenario(path); errorValue == nil {
			t.Fatalf("expected boundary rejection for %s", document)
		}
	}
}

func TestMattermostScenarioToolEvidenceRequiresRequestedEvent(t *testing.T) {
	events := []mattermostScenarioTaskEvent{{Name: "tool.task.add.result", Body: `{"operation":"task.add"}`}}
	if countMattermostScenarioToolEvents(events, "task.add") != 0 {
		t.Fatal("expected result events not to count as tool requests")
	}
	events = append(events, mattermostScenarioTaskEvent{Name: "tool.task.add.requested"})
	if countMattermostScenarioToolEvents(events, "task.add") != 1 {
		t.Fatal("expected direct typed request event to count")
	}
}
