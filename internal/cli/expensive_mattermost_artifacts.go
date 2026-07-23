package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
)

type expensiveMattermostArtifactManifest struct {
	SchemaVersion        int                                     `json:"schemaVersion"`
	ScenarioName         string                                  `json:"scenarioName"`
	TurnCount            int                                     `json:"turnCount"`
	DiagnosticResultPath string                                  `json:"diagnosticResultPath"`
	Steps                []expensiveMattermostArtifactStepRecord `json:"steps"`
}

type expensiveMattermostArtifactStepRecord struct {
	StepIndex           int      `json:"stepIndex"`
	TaskRunID           string   `json:"taskRunID"`
	TaskStatus          string   `json:"taskStatus"`
	DiagnosticEventPath string   `json:"diagnosticEventPath"`
	EvidencePaths       []string `json:"evidencePaths,omitempty"`
	LLMCallCount        int      `json:"llmCallCount"`
	AgentStepCount      int      `json:"agentStepCount"`
	ToolCallCount       int      `json:"toolCallCount"`
	ProcessingMS        int64    `json:"processingMs"`
}

func writeExpensiveMattermostEvidence(directoryPath string, result mattermostScenarioResult) error {
	if errorValue := os.MkdirAll(directoryPath, 0o755); errorValue != nil {
		return errorValue
	}
	for stepIndex, step := range result.Steps {
		eventsPath := filepath.Join(directoryPath, "diagnostics", "events", fmt.Sprintf("step-%02d.json", stepIndex+1))
		if errorValue := writeExpensiveJSONArtifact(eventsPath, step.TaskEvents); errorValue != nil {
			return errorValue
		}
	}
	resultPath := filepath.Join(directoryPath, "diagnostics", "result.json")
	if errorValue := writeExpensiveJSONArtifact(resultPath, sanitizeMattermostScenarioResult(result)); errorValue != nil {
		return errorValue
	}
	return writeExpensiveMattermostEvidenceManifest(directoryPath, result)
}

func writeExpensiveMattermostEvidenceManifest(directoryPath string, result mattermostScenarioResult) error {
	manifest := expensiveMattermostArtifactManifest{
		SchemaVersion:        2,
		ScenarioName:         result.ScenarioName,
		TurnCount:            result.TurnCount,
		DiagnosticResultPath: "diagnostics/result.json",
		Steps:                make([]expensiveMattermostArtifactStepRecord, 0, len(result.Steps)),
	}
	for stepIndex, step := range result.Steps {
		record := expensiveMattermostArtifactStepRecord{
			StepIndex:           stepIndex,
			TaskRunID:           step.TaskRunID,
			TaskStatus:          step.TaskStatus,
			DiagnosticEventPath: filepath.ToSlash(filepath.Join("diagnostics", "events", fmt.Sprintf("step-%02d.json", stepIndex+1))),
			LLMCallCount:        step.LLMCallCount,
			AgentStepCount:      step.AgentStepCount,
			ToolCallCount:       step.ToolCallCount,
			ProcessingMS:        step.ProcessingMS,
		}
		evidenceDirectoryPath := filepath.Join(directoryPath, "evidence", fmt.Sprintf("step-%02d", stepIndex+1))
		evidencePaths, errorValue := expensiveMattermostEvidenceFiles(evidenceDirectoryPath)
		if errorValue != nil {
			return errorValue
		}
		record.EvidencePaths = evidencePaths
		manifest.Steps = append(manifest.Steps, record)
	}
	return writeExpensiveJSONArtifact(filepath.Join(directoryPath, "manifest.json"), manifest)
}

func expensiveMattermostEvidenceFiles(directoryPath string) ([]string, error) {
	if _, errorValue := os.Stat(directoryPath); errors.Is(errorValue, os.ErrNotExist) {
		return nil, nil
	} else if errorValue != nil {
		return nil, errorValue
	}
	paths := []string{}
	errorValue := filepath.Walk(directoryPath, func(filePath string, information os.FileInfo, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if information.IsDir() {
			return nil
		}
		relativePath, errorValue := filepath.Rel(filepath.Dir(filepath.Dir(directoryPath)), filePath)
		if errorValue != nil {
			return errorValue
		}
		paths = append(paths, filepath.ToSlash(relativePath))
		return nil
	})
	if errorValue != nil {
		return nil, errorValue
	}
	sort.Strings(paths)
	return paths, nil
}

func sanitizeMattermostScenarioResult(result mattermostScenarioResult) mattermostScenarioResult {
	sanitizedResult := result
	sanitizedResult.Steps = append([]mattermostScenarioStepResult(nil), result.Steps...)
	for stepIndex := range sanitizedResult.Steps {
		sanitizedResult.Steps[stepIndex].Attachments = sanitizeDownloadedMattermostFiles(result.Steps[stepIndex].Attachments)
	}
	return sanitizedResult
}

func sanitizeDownloadedMattermostFiles(files []downloadedMattermostFile) []downloadedMattermostFile {
	sanitizedFiles := append([]downloadedMattermostFile(nil), files...)
	for fileIndex := range sanitizedFiles {
		sanitizedFiles[fileIndex].ContentBase64 = ""
	}
	return sanitizedFiles
}

func writeExpensiveJSONArtifact(filePath string, value any) error {
	if errorValue := os.MkdirAll(filepath.Dir(filePath), 0o755); errorValue != nil {
		return errorValue
	}
	document, errorValue := json.MarshalIndent(value, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	return os.WriteFile(filePath, append(document, '\n'), 0o600)
}

type expensiveMattermostBrowserRequest struct {
	RepositoryRootPath     string
	ArtifactDirectoryPath  string
	MattermostURL          string
	SiteProxyURL           string
	Execution              mattermostScenarioExecution
	StepIndex              int
	BotReplyPostID         string
	ScreenshotName         string
	PlaywrightRunName      string
	ApprovalAction         mattermostScenarioApprovalAction
	ExpectedAttachments    []string
	ExpectedPublicURL      string
	ExpectedPublicText     []string
	ExpectedPublicControls []string
}

func verifyExpensiveMattermostStep(contextValue context.Context, repositoryRootPath string, artifactDirectoryPath string, siteProxyURL string, mattermostURL string, scenario mattermostScenario, execution mattermostScenarioExecution, stepIndex int, shouldAutoConfirm bool) error {
	step := scenario.Steps[stepIndex]
	result := execution.Result.Steps[stepIndex]
	botReply, hasBotReply := latestMattermostScenarioPost(execution.Result.Posts)
	if !hasBotReply {
		return fmt.Errorf("verify Mattermost UI for step %s: bot reply is missing", strconv.Itoa(stepIndex+1))
	}
	approvalAction := mattermostScenarioApprovalAction("")
	if shouldAutoConfirm {
		approvalAction = step.ApprovalAction
	}
	return runExpensiveMattermostBrowserVerification(contextValue, expensiveMattermostBrowserRequest{
		RepositoryRootPath:     repositoryRootPath,
		ArtifactDirectoryPath:  artifactDirectoryPath,
		MattermostURL:          mattermostURL,
		SiteProxyURL:           siteProxyURL,
		Execution:              execution,
		StepIndex:              stepIndex,
		BotReplyPostID:         botReply.ID,
		ScreenshotName:         "mattermost-dm.png",
		PlaywrightRunName:      fmt.Sprintf("step-%02d", stepIndex+1),
		ApprovalAction:         approvalAction,
		ExpectedAttachments:    step.ExpectedAttachments,
		ExpectedPublicURL:      result.PublicURL,
		ExpectedPublicText:     step.ExpectedPublicText,
		ExpectedPublicControls: step.ExpectedPublicControls,
	})
}

func verifyExpensiveMattermostApprovalCompletions(contextValue context.Context, repositoryRootPath string, artifactDirectoryPath string, mattermostURL string, scenario mattermostScenario, execution mattermostScenarioExecution) error {
	for stepIndex, step := range scenario.Steps {
		if step.ApprovalAction == "" {
			continue
		}
		if errorValue := verifyExpensiveMattermostApprovalCompletion(contextValue, repositoryRootPath, artifactDirectoryPath, mattermostURL, execution, stepIndex); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func verifyExpensiveMattermostApprovalCompletion(contextValue context.Context, repositoryRootPath string, artifactDirectoryPath string, mattermostURL string, execution mattermostScenarioExecution, stepIndex int) error {
	stepResult := execution.Result.Steps[stepIndex]
	if stepResult.BotPostID == "" {
		return fmt.Errorf("verify Mattermost completed UI for step %s: bot reply is missing", strconv.Itoa(stepIndex+1))
	}
	return runExpensiveMattermostBrowserVerification(contextValue, expensiveMattermostBrowserRequest{
		RepositoryRootPath:    repositoryRootPath,
		ArtifactDirectoryPath: artifactDirectoryPath,
		MattermostURL:         mattermostURL,
		Execution:             execution,
		StepIndex:             stepIndex,
		BotReplyPostID:        stepResult.BotPostID,
		ScreenshotName:        "mattermost-completed.png",
		PlaywrightRunName:     fmt.Sprintf("step-%02d-completed", stepIndex+1),
	})
}

func runExpensiveMattermostBrowserVerification(contextValue context.Context, request expensiveMattermostBrowserRequest) error {
	stepArtifactDirectoryPath := filepath.Join(request.ArtifactDirectoryPath, "evidence", fmt.Sprintf("step-%02d", request.StepIndex+1))
	if errorValue := os.MkdirAll(stepArtifactDirectoryPath, 0o755); errorValue != nil {
		return errorValue
	}
	environment := append(os.Environ(),
		"INTERNKIM_MATTERMOST_URL="+request.MattermostURL,
		"INTERNKIM_MATTERMOST_PROBE_USERNAME="+request.Execution.Username,
		"INTERNKIM_MATTERMOST_PROBE_PASSWORD="+request.Execution.Password,
		"INTERNKIM_MATTERMOST_DM_CHANNEL_ID="+request.Execution.Result.ChannelID,
		"INTERNKIM_MATTERMOST_ROOT_POST_ID="+request.Execution.RootPostID,
		"INTERNKIM_MATTERMOST_BOT_USERNAME="+request.Execution.BotUsername,
		"INTERNKIM_MATTERMOST_BOT_REPLY_POST_ID="+request.BotReplyPostID,
		"INTERNKIM_MATTERMOST_SCREENSHOT_NAME="+request.ScreenshotName,
		"INTERNKIM_MATTERMOST_ARTIFACT_DIR="+stepArtifactDirectoryPath,
		"INTERNKIM_MATTERMOST_APPROVAL_ACTION="+string(request.ApprovalAction),
		"INTERNKIM_MATTERMOST_EXPECT_ATTACHMENTS="+marshalEnvironmentStringArray(request.ExpectedAttachments),
		"INTERNKIM_MATTERMOST_EXPECT_PUBLIC_URL="+request.ExpectedPublicURL,
		"INTERNKIM_SITE_PROXY_URL="+request.SiteProxyURL,
		"INTERNKIM_MATTERMOST_EXPECT_PUBLIC_TEXT="+marshalEnvironmentStringArray(request.ExpectedPublicText),
		"INTERNKIM_MATTERMOST_EXPECT_PUBLIC_CONTROLS="+marshalEnvironmentStringArray(request.ExpectedPublicControls),
	)
	playwrightOutputPath := filepath.Join(request.ArtifactDirectoryPath, "diagnostics", "playwright", request.PlaywrightRunName)
	command := exec.CommandContext(contextValue, "bun", "run", "test:e2e:mattermost-expensive", "--output="+playwrightOutputPath)
	command.Dir = filepath.Join(request.RepositoryRootPath, "web")
	command.Env = environment
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if errorValue := command.Run(); errorValue != nil {
		return fmt.Errorf("verify Mattermost UI for step %s: %w", strconv.Itoa(request.StepIndex+1), errorValue)
	}
	return nil
}

type expensiveMattermostFailureContext struct {
	StepIndex       int    `json:"stepIndex"`
	FailureReason   string `json:"failureReason"`
	LastBotPostText string `json:"lastBotPostText"`
}

func captureExpensiveMattermostFailureEvidence(contextValue context.Context, repositoryRootPath string, artifactDirectoryPath string, mattermostURL string, execution mattermostScenarioExecution, stepIndex int, failureReason string) {
	if stepIndex < 0 {
		return
	}
	failureContextPath := filepath.Join(artifactDirectoryPath, "evidence", fmt.Sprintf("step-%02d", stepIndex+1), "failure-context.json")
	failureContext := expensiveMattermostFailureContext{
		StepIndex:       stepIndex,
		FailureReason:   failureReason,
		LastBotPostText: mattermostScenarioLastBotPostText(execution.Result),
	}
	if errorValue := writeExpensiveJSONArtifact(failureContextPath, failureContext); errorValue != nil {
		fmt.Println("warning: failed to write failure context for step " + strconv.Itoa(stepIndex+1) + ": " + errorValue.Error())
	}
	botReplyPostID := ""
	if stepIndex < len(execution.Result.Steps) {
		botReplyPostID = execution.Result.Steps[stepIndex].BotPostID
	}
	screenshotError := runExpensiveMattermostBrowserVerification(contextValue, expensiveMattermostBrowserRequest{
		RepositoryRootPath:    repositoryRootPath,
		ArtifactDirectoryPath: artifactDirectoryPath,
		MattermostURL:         mattermostURL,
		Execution:             execution,
		StepIndex:             stepIndex,
		BotReplyPostID:        botReplyPostID,
		ScreenshotName:        "failure-dm.png",
		PlaywrightRunName:     fmt.Sprintf("step-%02d-failure", stepIndex+1),
	})
	if screenshotError != nil {
		fmt.Println("warning: failed to capture failure screenshot for step " + strconv.Itoa(stepIndex+1) + ": " + screenshotError.Error())
	}
}

func latestMattermostScenarioPost(posts []mattermostScenarioPost) (mattermostScenarioPost, bool) {
	if len(posts) == 0 {
		return mattermostScenarioPost{}, false
	}
	return posts[len(posts)-1], true
}

func marshalEnvironmentStringArray(values []string) string {
	document, errorValue := json.Marshal(append([]string{}, values...))
	if errorValue != nil {
		return "[]"
	}
	return string(document)
}
