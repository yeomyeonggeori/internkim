package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

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
	return writeExpensiveJSONArtifact(filepath.Join(directoryPath, "result.json"), sanitizeMattermostScenarioResult(result))
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
	environment := append(os.Environ(),
		"INTERNKIM_MATTERMOST_URL="+mattermostURL,
		"INTERNKIM_MATTERMOST_PROBE_USERNAME="+execution.Username,
		"INTERNKIM_MATTERMOST_PROBE_PASSWORD="+execution.Password,
		"INTERNKIM_MATTERMOST_DM_CHANNEL_ID="+execution.Result.ChannelID,
		"INTERNKIM_MATTERMOST_ROOT_POST_ID="+execution.RootPostID,
		"INTERNKIM_MATTERMOST_BOT_USERNAME="+execution.BotUsername,
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

func marshalEnvironmentStringArray(values []string) string {
	document, errorValue := json.Marshal(append([]string{}, values...))
	if errorValue != nil {
		return "[]"
	}
	return string(document)
}
