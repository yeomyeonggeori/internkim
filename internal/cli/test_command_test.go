package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseTestArgumentsUsesPromptAndDefaults(t *testing.T) {
	now := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	configuration, errorValue := parseTestArguments([]string{"저번 달 업무 보고서 워드 파일로 만들어줘"}, now)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.Prompt != "저번 달 업무 보고서 워드 파일로 만들어줘" {
		t.Fatalf("unexpected prompt: %q", configuration.Prompt)
	}
	if configuration.DownloadDirectoryPath != "/tmp/internkim-test-20260625T103000" {
		t.Fatalf("unexpected download directory: %s", configuration.DownloadDirectoryPath)
	}
	if configuration.OutputFilePath != "" {
		t.Fatalf("unexpected output file path: %s", configuration.OutputFilePath)
	}
	if !configuration.ShouldOpenFiles {
		t.Fatal("expected files to open by default")
	}
	if configuration.ShouldReuseFleet || configuration.ShouldKeepArtifacts {
		t.Fatalf("unexpected reuse/keep defaults: %+v", configuration)
	}
	if configuration.MaximumModelTier != "xlow" {
		t.Fatalf("expected xlow maximum model tier by default: %+v", configuration)
	}
}

func TestParseTestArgumentsAcceptsFlagsAfterPrompt(t *testing.T) {
	configuration, errorValue := parseTestArguments([]string{
		"슬라이드 만들어줘",
		"--reuse",
		"--keep",
		"--no-open",
		"-o",
		"/tmp/custom.docx",
		"--result-json",
		"/tmp/result.json",
		"--timeout",
		"120",
		"--seed",
		"7",
		"--temperature",
		"0.2",
		"--real",
		"--auto-confirm",
		"--expect-public-url",
		"--expect-tool",
		"site.publish",
	}, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.Prompt != "슬라이드 만들어줘" {
		t.Fatalf("unexpected prompt: %q", configuration.Prompt)
	}
	if !configuration.ShouldReuseFleet || !configuration.ShouldKeepArtifacts || configuration.ShouldOpenFiles {
		t.Fatalf("unexpected boolean flags: %+v", configuration)
	}
	if configuration.OutputFilePath != "/tmp/custom.docx" || configuration.ResultJSONPath != "/tmp/result.json" || configuration.TimeoutSeconds != 120 {
		t.Fatalf("unexpected value flags: %+v", configuration)
	}
	if configuration.GenerationSeed != 7 || configuration.GenerationTemperature != 0.2 {
		t.Fatalf("unexpected generation flags: %+v", configuration)
	}
	if !configuration.ShouldUseRealModels || !configuration.ShouldAutoConfirm {
		t.Fatalf("expected real model and auto confirm options: %+v", configuration)
	}
	if configuration.MaximumModelTier != "" {
		t.Fatalf("expected real mode to omit model ceiling: %+v", configuration)
	}
	if !configuration.ShouldExpectPublicURL || len(configuration.ExpectedTools) != 1 || configuration.ExpectedTools[0] != "site.publish" {
		t.Fatalf("unexpected site verification flags: %+v", configuration)
	}
}

func TestParseTestArgumentsAcceptsExpensiveSuiteControls(t *testing.T) {
	configuration, errorValue := parseTestArguments([]string{
		"expensive",
		"--scenario", "task-lifecycle",
		"--scenario", "direct-message-send",
		"--maximum-model-tier", "high",
		"--seed", "41",
	}, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.Suite != testSuiteExpensive || configuration.Prompt != "" || configuration.MaximumModelTier != "high" {
		t.Fatalf("unexpected expensive suite configuration: %+v", configuration)
	}
	if len(configuration.ScenarioNames) != 2 || configuration.ScenarioNames[0] != "task-lifecycle" || configuration.ScenarioNames[1] != "direct-message-send" {
		t.Fatalf("unexpected scenario selection: %+v", configuration.ScenarioNames)
	}
}

func TestRunTestArgumentsAcceptsHelpWithoutStartingFleet(t *testing.T) {
	if errorValue := runTestArguments([]string{"--help"}); errorValue != nil {
		t.Fatalf("expected help to exit successfully, got %v", errorValue)
	}
}

func TestParseTestArgumentsRealModeRemovesTierCeiling(t *testing.T) {
	configuration, errorValue := parseTestArguments([]string{"full", "--real"}, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.Suite != testSuiteFull || !configuration.ShouldUseRealModels || configuration.MaximumModelTier != "" {
		t.Fatalf("unexpected real suite configuration: %+v", configuration)
	}
}

func TestParseTestArgumentsRejectsRealModeWithTierCeiling(t *testing.T) {
	_, errorValue := parseTestArguments([]string{"expensive", "--real", "--maximum-model-tier", "low"}, time.Now())
	if errorValue == nil || !strings.Contains(errorValue.Error(), "cannot be combined") {
		t.Fatalf("expected real model tier conflict, got %v", errorValue)
	}
}

func TestRunSequentialExpensiveScenariosStopsAtFirstFailure(t *testing.T) {
	scenarios := []expensiveScenarioReference{{Name: "first"}, {Name: "second"}, {Name: "third"}}
	runNames := []string{}
	errorValue := runSequentialExpensiveScenarios(scenarios, func(scenario expensiveScenarioReference) error {
		runNames = append(runNames, scenario.Name)
		if scenario.Name == "second" {
			return errors.New("step failed")
		}
		return nil
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "second") {
		t.Fatalf("expected second scenario failure, got %v", errorValue)
	}
	if strings.Join(runNames, ",") != "first,second" {
		t.Fatalf("expected fail-fast execution, got %v", runNames)
	}
}

func TestParseTestArgumentsDefaultsToFixedGenerationOptions(t *testing.T) {
	configuration, errorValue := parseTestArguments([]string{"보고서 만들어줘"}, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.GenerationSeed != 41 || configuration.GenerationTemperature != 0 {
		t.Fatalf("unexpected default generation options: %+v", configuration)
	}
	if configuration.ShouldUseRealModels {
		t.Fatalf("expected test model default: %+v", configuration)
	}
}

func TestParseTestArgumentsAcceptsRunIDForDisposableFleet(t *testing.T) {
	configuration, errorValue := parseTestArguments([]string{
		"슬라이드 만들어줘",
		"--run-id",
		"quality-check",
	}, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.RunID != "quality-check" {
		t.Fatalf("unexpected run ID: %+v", configuration)
	}
}

func TestParseTestArgumentsRequiresPrompt(t *testing.T) {
	_, errorValue := parseTestArguments([]string{"--reuse"}, time.Now())
	if errorValue == nil || !strings.Contains(errorValue.Error(), "usage: internkim test") {
		t.Fatalf("expected usage error, got %v", errorValue)
	}
}

func TestParseTestArgumentsRejectsRunIDWithReuse(t *testing.T) {
	_, errorValue := parseTestArguments([]string{"프롬프트", "--reuse", "--run-id", "run-a"}, time.Now())
	if errorValue == nil || !strings.Contains(errorValue.Error(), "--run-id requires") {
		t.Fatalf("expected run-id reuse error, got %v", errorValue)
	}
}

func TestWriteTestDownloadedMattermostFilesWritesSingleAttachmentToOutputPath(t *testing.T) {
	outputFilePath := filepath.Join(t.TempDir(), "nested", "report.docx")
	output := `{"downloadedFiles":[{"fileID":"file-1","filename":"ignored.docx","contentBase64":"ZG9jeA=="}]}`
	downloadedFilePaths, errorValue := writeTestDownloadedMattermostFiles(output, outputFilePath, filepath.Join(t.TempDir(), "downloads"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(downloadedFilePaths) != 1 || downloadedFilePaths[0] != outputFilePath {
		t.Fatalf("unexpected downloaded file paths: %v", downloadedFilePaths)
	}
	content, errorValue := os.ReadFile(outputFilePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(content) != "docx" {
		t.Fatalf("unexpected output file content: %s", string(content))
	}
}

func TestWriteTestDownloadedMattermostFilesRejectsOutputPathForMultipleAttachments(t *testing.T) {
	output := `{"downloadedFiles":[{"fileID":"file-1","filename":"one.txt","contentBase64":"MQ=="},{"fileID":"file-2","filename":"two.txt","contentBase64":"Mg=="}]}`
	_, errorValue := writeTestDownloadedMattermostFiles(output, filepath.Join(t.TempDir(), "result.txt"), filepath.Join(t.TempDir(), "downloads"))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "-o can only write one Mattermost attachment") {
		t.Fatalf("expected multiple attachment output path error, got %v", errorValue)
	}
}

func TestWriteTestResultJSONIncludesDownloadedFilePaths(t *testing.T) {
	resultJSONPath := filepath.Join(t.TempDir(), "result.json")
	verificationOutput := mattermostVerificationOutput{
		BotMessage:    "done",
		SitePublicURL: "https://example.example.test",
		SiteHTMLText:  "Banchan Table",
		SiteStyleMetrics: map[string]any{
			"typographyScore": 0.91,
		},
		DownloadedFiles: []downloadedMattermostFile{{FileID: "file-1", Filename: "report.pdf", ContentBase64: "cGRm"}},
	}
	errorValue := writeTestResultJSON(resultJSONPath, verificationOutput, []string{"/tmp/report.pdf"}, nil, "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	content, errorValue := os.ReadFile(resultJSONPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	document := string(content)
	for _, expectedText := range []string{"Banchan Table", "https://example.example.test", "/tmp/report.pdf", "typographyScore"} {
		if !strings.Contains(document, expectedText) {
			t.Fatalf("result JSON missing %q: %s", expectedText, document)
		}
	}
}

func TestWriteTestResultJSONIncludesTaskDetailOrFailSoftReason(t *testing.T) {
	verificationOutput := mattermostVerificationOutput{BotMessage: "done"}

	withDetailPath := filepath.Join(t.TempDir(), "with-detail.json")
	if errorValue := writeTestResultJSON(withDetailPath, verificationOutput, nil, json.RawMessage(`{"taskRun":{"status":"completed"}}`), ""); errorValue != nil {
		t.Fatal(errorValue)
	}
	withDetailContent, errorValue := os.ReadFile(withDetailPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(withDetailContent), `"status": "completed"`) {
		t.Fatalf("result JSON missing taskDetail: %s", string(withDetailContent))
	}
	if strings.Contains(string(withDetailContent), "taskDetailError") {
		t.Fatalf("result JSON should omit taskDetailError when detail was fetched: %s", string(withDetailContent))
	}

	withoutDetailPath := filepath.Join(t.TempDir(), "without-detail.json")
	if errorValue := writeTestResultJSON(withoutDetailPath, verificationOutput, nil, nil, "no taskRunID was returned by the Mattermost verification"); errorValue != nil {
		t.Fatal(errorValue)
	}
	withoutDetailContent, errorValue := os.ReadFile(withoutDetailPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(withoutDetailContent), "no taskRunID was returned by the Mattermost verification") {
		t.Fatalf("result JSON missing taskDetailError: %s", string(withoutDetailContent))
	}
	if strings.Contains(string(withoutDetailContent), `"taskDetail"`) {
		t.Fatalf("result JSON should omit taskDetail when fetch failed: %s", string(withoutDetailContent))
	}
}
