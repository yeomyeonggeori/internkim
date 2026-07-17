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

type expensiveMattermostEvidenceManifest struct {
	SchemaVersion int                                     `json:"schemaVersion"`
	ScenarioName  string                                  `json:"scenarioName"`
	TurnCount     int                                     `json:"turnCount"`
	ResultPath    string                                  `json:"resultPath"`
	Steps         []expensiveMattermostEvidenceStepRecord `json:"steps"`
}

type expensiveMattermostEvidenceStepRecord struct {
	StepIndex          int      `json:"stepIndex"`
	TaskRunID          string   `json:"taskRunID"`
	TaskStatus         string   `json:"taskStatus"`
	EventPath          string   `json:"eventPath"`
	AttachmentPaths    []string `json:"attachmentPaths,omitempty"`
	WorkspaceFilePaths []string `json:"workspaceFilePaths,omitempty"`
	UIArtifactPaths    []string `json:"uiArtifactPaths,omitempty"`
	LLMCallCount       int      `json:"llmCallCount"`
	AgentStepCount     int      `json:"agentStepCount"`
	ToolCallCount      int      `json:"toolCallCount"`
	ProcessingMS       int64    `json:"processingMs"`
}

func writeExpensiveMattermostEvidence(directoryPath string, result mattermostScenarioResult) error {
	if errorValue := os.MkdirAll(directoryPath, 0o755); errorValue != nil {
		return errorValue
	}
	for stepIndex, step := range result.Steps {
		stepDirectoryPath := filepath.Join(directoryPath, "files", fmt.Sprintf("step-%02d", stepIndex+1))
		for _, file := range step.Attachments {
			filePath := filepath.Join(stepDirectoryPath, filepath.Base(file.Filename))
			if _, errorValue := writeDownloadedMattermostFileToPath(file, filePath); errorValue != nil {
				return errorValue
			}
		}
		eventsPath := filepath.Join(directoryPath, "events", fmt.Sprintf("step-%02d.json", stepIndex+1))
		if errorValue := writeExpensiveJSONArtifact(eventsPath, step.TaskEvents); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := writeExpensiveJSONArtifact(filepath.Join(directoryPath, "result.json"), sanitizeMattermostScenarioResult(result)); errorValue != nil {
		return errorValue
	}
	return writeExpensiveMattermostEvidenceManifest(directoryPath, result)
}

func writeExpensiveMattermostEvidenceManifest(directoryPath string, result mattermostScenarioResult) error {
	manifest := expensiveMattermostEvidenceManifest{
		SchemaVersion: 1,
		ScenarioName:  result.ScenarioName,
		TurnCount:     result.TurnCount,
		ResultPath:    "result.json",
		Steps:         make([]expensiveMattermostEvidenceStepRecord, 0, len(result.Steps)),
	}
	for stepIndex, step := range result.Steps {
		record := expensiveMattermostEvidenceStepRecord{
			StepIndex:      stepIndex,
			TaskRunID:      step.TaskRunID,
			TaskStatus:     step.TaskStatus,
			EventPath:      filepath.ToSlash(filepath.Join("events", fmt.Sprintf("step-%02d.json", stepIndex+1))),
			LLMCallCount:   step.LLMCallCount,
			AgentStepCount: step.AgentStepCount,
			ToolCallCount:  step.ToolCallCount,
			ProcessingMS:   step.ProcessingMS,
		}
		for _, file := range step.Attachments {
			record.AttachmentPaths = append(record.AttachmentPaths, filepath.ToSlash(filepath.Join("files", fmt.Sprintf("step-%02d", stepIndex+1), filepath.Base(file.Filename))))
		}
		for _, file := range step.WorkspaceFiles {
			record.WorkspaceFilePaths = append(record.WorkspaceFilePaths, file.Path)
		}
		uiDirectoryPath := filepath.Join(directoryPath, "ui", fmt.Sprintf("step-%02d", stepIndex+1))
		uiPaths, errorValue := expensiveMattermostEvidenceFiles(uiDirectoryPath)
		if errorValue != nil {
			return errorValue
		}
		record.UIArtifactPaths = uiPaths
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

func verifyExpensiveMattermostStep(contextValue context.Context, repositoryRootPath string, artifactDirectoryPath string, siteProxyURL string, mattermostURL string, scenario mattermostScenario, execution mattermostScenarioExecution, stepIndex int, shouldAutoConfirm bool) error {
	step := scenario.Steps[stepIndex]
	result := execution.Result.Steps[stepIndex]
	isFinalStep := stepIndex == len(scenario.Steps)-1
	if !isFinalStep && len(step.ExpectedAttachments) == 0 && result.PublicURL == "" && step.ApprovalAction == "" {
		return nil
	}
	stepArtifactDirectoryPath := filepath.Join(artifactDirectoryPath, "ui", fmt.Sprintf("step-%02d", stepIndex+1))
	if errorValue := os.MkdirAll(stepArtifactDirectoryPath, 0o755); errorValue != nil {
		return errorValue
	}
	botReply, hasBotReply := latestMattermostScenarioPost(execution.Result.Posts)
	if !hasBotReply {
		return fmt.Errorf("verify Mattermost UI for step %s: bot reply is missing", strconv.Itoa(stepIndex+1))
	}
	environment := append(os.Environ(),
		"INTERNKIM_MATTERMOST_URL="+mattermostURL,
		"INTERNKIM_MATTERMOST_PROBE_USERNAME="+execution.Username,
		"INTERNKIM_MATTERMOST_PROBE_PASSWORD="+execution.Password,
		"INTERNKIM_MATTERMOST_DM_CHANNEL_ID="+execution.Result.ChannelID,
		"INTERNKIM_MATTERMOST_ROOT_POST_ID="+execution.RootPostID,
		"INTERNKIM_MATTERMOST_BOT_USERNAME="+execution.BotUsername,
		"INTERNKIM_MATTERMOST_BOT_REPLY_POST_ID="+botReply.ID,
		"INTERNKIM_MATTERMOST_ARTIFACT_DIR="+stepArtifactDirectoryPath,
		"INTERNKIM_MATTERMOST_EXPECT_PUBLIC_URL="+result.PublicURL,
		"INTERNKIM_SITE_PROXY_URL="+siteProxyURL,
	)
	if shouldAutoConfirm {
		environment = append(environment, "INTERNKIM_MATTERMOST_APPROVAL_ACTION="+string(step.ApprovalAction))
	}
	if len(step.ExpectedAttachments) > 0 {
		environment = append(environment, "INTERNKIM_MATTERMOST_EXPECT_ATTACHMENTS="+marshalEnvironmentStringArray(step.ExpectedAttachments))
	}
	environment = append(environment,
		"INTERNKIM_MATTERMOST_EXPECT_PUBLIC_TEXT="+marshalEnvironmentStringArray(step.ExpectedPublicText),
		"INTERNKIM_MATTERMOST_EXPECT_PUBLIC_CONTROLS="+marshalEnvironmentStringArray(step.ExpectedPublicControls),
	)
	command := exec.CommandContext(contextValue, "bun", "run", "test:e2e:mattermost-expensive", "--output="+filepath.Join(stepArtifactDirectoryPath, "playwright"))
	command.Dir = filepath.Join(repositoryRootPath, "web")
	command.Env = environment
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if errorValue := command.Run(); errorValue != nil {
		return fmt.Errorf("verify Mattermost UI for step %s: %w", strconv.Itoa(stepIndex+1), errorValue)
	}
	return nil
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
