package cli

import (
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
	if configuration.MaximumModelTier != "low" {
		t.Fatalf("expected low maximum model tier by default: %+v", configuration)
	}
	if configuration.TimeoutSeconds != 0 {
		t.Fatalf("expected no expensive-scenario deadline by default: %+v", configuration)
	}
}

func TestParseTestArgumentsAcceptsFlagsAfterPrompt(t *testing.T) {
	configuration, errorValue := parseTestArguments([]string{
		"슬라이드 만들어줘",
		"--timeout",
		"120",
		"--seed",
		"7",
		"--temperature",
		"0.2",
		"--real",
	}, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.Prompt != "슬라이드 만들어줘" {
		t.Fatalf("unexpected prompt: %q", configuration.Prompt)
	}
	if configuration.TimeoutSeconds != 120 {
		t.Fatalf("unexpected value flags: %+v", configuration)
	}
	if configuration.GenerationSeed == nil || *configuration.GenerationSeed != 7 || configuration.GenerationTemperature == nil || *configuration.GenerationTemperature != 0.2 {
		t.Fatalf("unexpected generation flags: %+v", configuration)
	}
	if !configuration.ShouldUseRealModels {
		t.Fatalf("expected real model option: %+v", configuration)
	}
	if configuration.MaximumModelTier != "" {
		t.Fatalf("expected real mode to omit model ceiling: %+v", configuration)
	}
}

func TestParseTestArgumentsAcceptsExpensiveSuiteControls(t *testing.T) {
	configuration, errorValue := parseTestArguments([]string{
		"expensive",
		"--scenario", "task-lifecycle",
		"--scenario", "message-lifecycle",
		"--maximum-model-tier", "high",
		"--llm-provider", "capability",
		"--llm-endpoint", "http://capability.test",
		"--llm-unix-socket", "/tmp/capability.sock",
		"--llm-execution-mode", "auto",
		"--seed", "41",
	}, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.Suite != testSuiteExpensive || configuration.Prompt != "" || configuration.MaximumModelTier != "high" {
		t.Fatalf("unexpected expensive suite configuration: %+v", configuration)
	}
	if len(configuration.ScenarioNames) != 2 || configuration.ScenarioNames[0] != "task-lifecycle" || configuration.ScenarioNames[1] != "message-lifecycle" {
		t.Fatalf("unexpected scenario selection: %+v", configuration.ScenarioNames)
	}
	if configuration.LanguageModelProvider != "capability" || configuration.LanguageModelEndpoint != "http://capability.test" || configuration.LanguageModelSocket != "/tmp/capability.sock" || configuration.LanguageModelExecutionMode != "auto" {
		t.Fatalf("unexpected live LLM configuration: %+v", configuration)
	}
}

func TestParseTestArgumentsAcceptsRunIDForNamedSuite(t *testing.T) {
	configuration, errorValue := parseTestArguments([]string{
		"expensive",
		"--run-id", "prepared-run",
	}, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.Suite != testSuiteExpensive || configuration.RunID != "prepared-run" {
		t.Fatalf("unexpected prepared run configuration: %+v", configuration)
	}
}

func TestParseTestArgumentsRejectsUnknownLanguageModelProvider(t *testing.T) {
	_, errorValue := parseTestArguments([]string{"expensive", "--llm-provider", "unknown"}, time.Now())
	if errorValue == nil || !strings.Contains(errorValue.Error(), "llm provider must be") {
		t.Fatalf("expected provider validation error, got %v", errorValue)
	}
}

func TestParseTestArgumentsAcceptsZeroTimeout(t *testing.T) {
	configuration, errorValue := parseTestArguments([]string{"expensive", "--timeout", "0"}, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.TimeoutSeconds != 0 {
		t.Fatalf("expected zero timeout, got %+v", configuration)
	}
}

func TestParseTestArgumentsRejectsNegativeTimeout(t *testing.T) {
	_, errorValue := parseTestArguments([]string{"expensive", "--timeout", "-1"}, time.Now())
	if errorValue == nil || !strings.Contains(errorValue.Error(), "--timeout must be 0 or greater") {
		t.Fatalf("expected negative timeout validation error, got %v", errorValue)
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

func TestParseTestArgumentsUsesProviderGenerationDefaults(t *testing.T) {
	configuration, errorValue := parseTestArguments([]string{"보고서 만들어줘"}, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.GenerationSeed != nil || configuration.GenerationTemperature != nil {
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
	_, errorValue := parseTestArguments([]string{"--real"}, time.Now())
	if errorValue == nil || !strings.Contains(errorValue.Error(), "usage: internkim test") {
		t.Fatalf("expected usage error, got %v", errorValue)
	}
}
