package cli

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestLoadMattermostScenarioAcceptsExpensiveLifecycleShape(t *testing.T) {
	for _, filename := range []string{
		"01-task-lifecycle.json",
		"02-calendar-lifecycle.json",
		"04-website-lifecycle.json",
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

func TestMattermostScenarioRequiresAuthoritativeSDKDProvenance(t *testing.T) {
	scenario := mattermostScenario{Name: "sdkd", RequiresSDKD: true, Steps: []mattermostScenarioStep{{Prompt: "work"}}}
	result := mattermostScenarioResult{ScenarioName: "sdkd", Steps: []mattermostScenarioStepResult{{
		Prompt:     "work",
		TaskEvents: []mattermostScenarioTaskEvent{{Name: "llm.call", Body: `{"kind":"chat","transport":"sdkd","model":"low-model"}`}},
	}}}
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue != nil {
		t.Fatalf("validate SDKD provenance: %v", errorValue)
	}
	result.Steps[0].TaskEvents[0].Body = `{"kind":"chat","transport":"capability","model":"low-model","usedFallback":true}`
	if errorValue := validateMattermostScenarioResult(scenario, &result); errorValue == nil {
		t.Fatal("expected legacy fallback provenance to fail")
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
				Name:         "tool.capability.invoke.result",
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
			{Name: "tool.capability.invoke.result", Body: "task.add report"},
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
		Prompt:                 "count",
		ExpectedToolCallCounts: map[string]int{"task.add": 2, "task.update": 0},
		ExpectedEventCounts:    []mattermostScenarioEventCount{{Name: "task.completed", Count: 2}},
	}}}
	result := mattermostScenarioResult{
		ScenarioName: "counts", ChannelID: "channel", UserID: "user",
		Steps: []mattermostScenarioStepResult{{
			Prompt: "count", TaskStatus: "completed", TaskEvents: []mattermostScenarioTaskEvent{
				{Name: "tool.task.add.requested"},
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

func TestMattermostScenarioRejectsUnsupportedApprovalAction(t *testing.T) {
	scenario := mattermostScenario{Name: "approval", Steps: []mattermostScenarioStep{{Prompt: "삭제해줘", ApprovalAction: "accept"}}}
	errorValue := validateMattermostScenario(scenario)
	if errorValue == nil || !strings.Contains(errorValue.Error(), `approvalAction "accept" is unsupported`) {
		t.Fatalf("unexpected error: %v", errorValue)
	}
}

func TestMattermostScenarioOutputFragmentExcludesCapabilityRequestEcho(t *testing.T) {
	expectation := mattermostScenarioEventCount{
		Name:           "tool.capability.invoke.result",
		OutputFragment: "고객지원 분기 결산 검토 완료",
		Count:          1,
	}
	events := []mattermostScenarioTaskEvent{{
		Name: "tool.capability.invoke.result",
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
	events := []mattermostScenarioTaskEvent{{Name: "tool.capability.invoke.result", Body: `{"operation":"task.add"}`}}
	if countMattermostScenarioToolEvents(events, "task.add") != 0 {
		t.Fatal("expected result events not to count as tool requests")
	}
	events = append(events,
		mattermostScenarioTaskEvent{Name: "tool.capability.invoke.requested", Body: `{"operation":"task.add"}`},
		mattermostScenarioTaskEvent{Name: "tool.capability.invoke.requested", Body: `{"input":{"operation":"task.add"}}`},
	)
	if countMattermostScenarioToolEvents(events, "task.add") != 2 {
		t.Fatal("expected both capability request shapes to count")
	}
}
