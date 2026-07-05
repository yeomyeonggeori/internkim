package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/localfleet"
)

type testCommandConfiguration struct {
	Prompt                string
	DownloadDirectoryPath string
	OutputFilePath        string
	ResultJSONPath        string
	RunID                 string
	TimeoutSeconds        int
	GenerationSeed        int64
	GenerationTemperature float64
	ExpectedTools         []string
	ShouldExpectPublicURL bool
	ShouldReuseFleet      bool
	ShouldKeepArtifacts   bool
	ShouldOpenFiles       bool
	ShouldUseRealModels   bool
	ShouldAutoConfirm     bool
}

func runTest() {
	if errorValue := runTestArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runTestArguments(arguments []string) error {
	configuration, errorValue := parseTestArguments(arguments, time.Now())
	if errorValue != nil {
		return errorValue
	}
	contextValue, stop := interruptContext()
	defer stop()
	return runTestConfiguration(contextValue, configuration)
}

func parseTestArguments(arguments []string, now time.Time) (testCommandConfiguration, error) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	reuseFleet := flagSet.Bool("reuse", false, "Reuse the shared Local Fleet instead of creating a disposable one")
	keepArtifacts := flagSet.Bool("keep", false, "Keep the Local Fleet VM, messages, and users after the test")
	noOpen := flagSet.Bool("no-open", false, "Download files without opening them")
	useRealModels := flagSet.Bool("real", false, "Use production model configuration instead of the Local Fleet test model")
	autoConfirm := flagSet.Bool("auto-confirm", false, "Automatically approve Mattermost confirmation prompts during the test")
	outputFilePath := flagSet.String("o", "", "Local output file path for one Mattermost attachment")
	resultJSONPath := flagSet.String("result-json", "", "Write the parsed Mattermost test result JSON to this local path")
	runID := flagSet.String("run-id", "", "Optional disposable Local Fleet run identifier")
	timeoutSeconds := flagSet.Int("timeout", 900, "Seconds to wait for the task to complete")
	generationSeed := flagSet.Int64("seed", 41, "Generation seed to apply before the Mattermost prompt")
	generationTemperature := flagSet.Float64("temperature", 0, "Generation temperature to apply before the Mattermost prompt")
	expectPublicURL := flagSet.Bool("expect-public-url", false, "Require a public URL and remote desktop/mobile screenshot verification")
	expectedTools := repeatedStringFlag{}
	flagSet.Var(&expectedTools, "expect-tool", "Require a requested tool event; repeat for multiple tools")
	flagArguments, positionalArguments := splitFlagsAndPositionals(arguments, map[string]bool{
		"reuse":             true,
		"keep":              true,
		"no-open":           true,
		"real":              true,
		"auto-confirm":      true,
		"expect-public-url": true,
	}, map[string]bool{
		"o":           true,
		"result-json": true,
		"run-id":      true,
		"timeout":     true,
		"seed":        true,
		"temperature": true,
		"expect-tool": true,
	})
	if errorValue := flagSet.Parse(flagArguments); errorValue != nil {
		return testCommandConfiguration{}, errorValue
	}
	prompt := strings.TrimSpace(strings.Join(positionalArguments, " "))
	if prompt == "" {
		return testCommandConfiguration{}, errors.New("usage: internkim test \"<prompt>\" [--reuse] [--keep] [-o /tmp/result.docx] [--no-open] [--seed 41] [--temperature 0]")
	}
	if *timeoutSeconds <= 0 {
		return testCommandConfiguration{}, errors.New("--timeout must be greater than 0")
	}
	if math.IsNaN(*generationTemperature) || math.IsInf(*generationTemperature, 0) || *generationTemperature < 0 {
		return testCommandConfiguration{}, errors.New("--temperature must be 0 or greater")
	}
	if *reuseFleet && strings.TrimSpace(*runID) != "" {
		return testCommandConfiguration{}, errors.New("--run-id requires a disposable Local Fleet run; remove --reuse")
	}
	defaultDownloadDirectoryPath := filepath.Join("/tmp", "internkim-test-"+now.UTC().Format("20060102T150405"))
	return testCommandConfiguration{
		Prompt:                prompt,
		DownloadDirectoryPath: defaultDownloadDirectoryPath,
		OutputFilePath:        strings.TrimSpace(*outputFilePath),
		ResultJSONPath:        strings.TrimSpace(*resultJSONPath),
		RunID:                 strings.TrimSpace(*runID),
		TimeoutSeconds:        *timeoutSeconds,
		GenerationSeed:        *generationSeed,
		GenerationTemperature: *generationTemperature,
		ExpectedTools:         expectedTools.Values(),
		ShouldExpectPublicURL: *expectPublicURL,
		ShouldReuseFleet:      *reuseFleet,
		ShouldKeepArtifacts:   *keepArtifacts,
		ShouldOpenFiles:       !*noOpen,
		ShouldUseRealModels:   *useRealModels,
		ShouldAutoConfirm:     *autoConfirm,
	}, nil
}

func runTestConfiguration(contextValue context.Context, configuration testCommandConfiguration) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	executablePath, errorValue := currentExecutablePath()
	if errorValue != nil {
		return errorValue
	}
	service, errorValue := localfleet.NewService(localfleet.Options{
		RepositoryRootPath:    repositoryRootPath,
		ExecutablePath:        executablePath,
		RunID:                 configuration.RunID,
		GenerationSeed:        strconv.FormatInt(configuration.GenerationSeed, 10),
		GenerationTemperature: formatTestFloat(configuration.GenerationTemperature),
		ShouldUseRealModels:   configuration.ShouldUseRealModels,
		IsEphemeral:           !configuration.ShouldReuseFleet,
	})
	if errorValue != nil {
		return errorValue
	}
	logger := standardLocalFleetLogger{}
	shouldCleanup := !configuration.ShouldKeepArtifacts && !configuration.ShouldReuseFleet
	runError := service.Run(contextValue, logger, localfleet.JobRequest{
		Action:        localfleet.ActionUp,
		KeepArtifacts: true,
		SkipWeb:       !configuration.ShouldExpectPublicURL,
	})
	if runError == nil {
		runError = runTestPrompt(contextValue, service, repositoryRootPath, executablePath, configuration)
	}
	if shouldCleanup {
		cleanupError := service.CleanupEphemeral(contextValue, logger)
		if runError != nil && cleanupError != nil {
			return fmt.Errorf("%w; cleanup failed: %v", runError, cleanupError)
		}
		if cleanupError != nil {
			return cleanupError
		}
	}
	return runError
}

func runTestPrompt(contextValue context.Context, service localfleet.Service, repositoryRootPath string, executablePath string, configuration testCommandConfiguration) error {
	target, errorValue := resolveLocalFleetTestTarget(contextValue, service, repositoryRootPath, executablePath)
	if errorValue != nil {
		return errorValue
	}
	fmt.Println("Generation options: seed=" + strconv.FormatInt(configuration.GenerationSeed, 10) + " temperature=" + formatTestFloat(configuration.GenerationTemperature))
	fmt.Println("Mattermost prompt: " + configuration.Prompt)
	script := verifyMattermostPromptScript(configuration.Prompt, configuration.ShouldKeepArtifacts, configuration.TimeoutSeconds, false, configuration.ShouldExpectPublicURL, configuration.ExpectedTools, nil, true, true, configuration.ShouldAutoConfirm)
	output, errorValue := target.sshClient.runResultWithTimeout(script, mattermostPromptScriptSSHTimeout(configuration.TimeoutSeconds, configuration.ShouldExpectPublicURL))
	if errorValue != nil {
		if strings.TrimSpace(output) != "" {
			fmt.Print(redactDownloadedMattermostFiles(output))
			if !strings.HasSuffix(output, "\n") {
				fmt.Println()
			}
		}
		writeTestResultJSONBestEffort(target, configuration, output)
		return fmt.Errorf("remote Mattermost test failed: %w", errorValue)
	}
	verificationOutput, errorValue := parseMattermostVerificationOutput(output)
	if errorValue != nil {
		if strings.TrimSpace(output) != "" {
			fmt.Print(redactDownloadedMattermostFiles(output))
			if !strings.HasSuffix(output, "\n") {
				fmt.Println()
			}
		}
		return errorValue
	}
	downloadedFilePaths, errorValue := writeTestDownloadedMattermostFiles(output, configuration.OutputFilePath, configuration.DownloadDirectoryPath)
	if errorValue != nil {
		return errorValue
	}
	taskDetail, taskDetailError := fetchTestTaskDetailJSON(target, verificationOutput.TaskRunID)
	if errorValue := writeTestResultJSON(configuration.ResultJSONPath, verificationOutput, downloadedFilePaths, taskDetail, taskDetailError); errorValue != nil {
		return errorValue
	}
	printTestResult(verificationOutput, downloadedFilePaths)
	return openDownloadedTestFiles(downloadedFilePaths, configuration.ShouldOpenFiles)
}

func fetchTestTaskDetailJSON(target verifyTarget, taskRunID string) (json.RawMessage, string) {
	trimmedTaskRunID := strings.TrimSpace(taskRunID)
	if trimmedTaskRunID == "" {
		return nil, "no taskRunID was returned by the Mattermost verification"
	}
	command := "curl -s --max-time 20 'http://127.0.0.1:8080/admin/api/task/detail?taskRunID=" + trimmedTaskRunID + "'"
	output, errorValue := target.sshClient.runResultWithTimeout(command, 30*time.Second)
	if errorValue != nil {
		return nil, "fetch task detail over SSH: " + errorValue.Error()
	}
	trimmedOutput := strings.TrimSpace(output)
	if trimmedOutput == "" {
		return nil, "task detail admin endpoint returned an empty response"
	}
	if !json.Valid([]byte(trimmedOutput)) {
		return nil, "task detail admin endpoint did not return valid JSON"
	}
	return json.RawMessage(trimmedOutput), ""
}

func formatTestFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func writeTestDownloadedMattermostFiles(output string, outputFilePath string, downloadDirectoryPath string) ([]string, error) {
	normalizedOutputFilePath := strings.TrimSpace(outputFilePath)
	if normalizedOutputFilePath == "" {
		return writeDownloadedMattermostFilesAllowEmpty(output, downloadDirectoryPath)
	}
	verificationOutput, errorValue := parseMattermostVerificationOutput(output)
	if errorValue != nil {
		return nil, errorValue
	}
	if len(verificationOutput.DownloadedFiles) == 0 {
		return nil, nil
	}
	if len(verificationOutput.DownloadedFiles) != 1 {
		return nil, fmt.Errorf("-o can only write one Mattermost attachment; got %d", len(verificationOutput.DownloadedFiles))
	}
	downloadedFilePath, errorValue := writeDownloadedMattermostFileToPath(verificationOutput.DownloadedFiles[0], normalizedOutputFilePath)
	if errorValue != nil {
		return nil, errorValue
	}
	return []string{downloadedFilePath}, nil
}

func resolveLocalFleetTestTarget(contextValue context.Context, service localfleet.Service, repositoryRootPath string, executablePath string) (verifyTarget, error) {
	command := exec.CommandContext(contextValue, executablePath, "lab", "vm-ip", "--config", service.ConfigurationPath())
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return verifyTarget{}, fmt.Errorf("resolve Local Fleet VM IP: %w: %s", errorValue, strings.TrimSpace(string(output)))
	}
	host := strings.TrimSpace(string(output))
	if host == "" {
		return verifyTarget{}, errors.New("Local Fleet VM IP is empty")
	}
	sshpassBin := filepath.Join(repositoryRootPath, "bin", "sshpass")
	return verifyTarget{
		host:       host,
		user:       "admin",
		password:   "admin",
		scriptDir:  repositoryRootPath,
		sshpassBin: sshpassBin,
		sshClient:  newSSH(sshpassBin, "admin", "admin", host),
	}, nil
}

func printTestResult(verificationOutput mattermostVerificationOutput, downloadedFilePaths []string) {
	fmt.Println()
	fmt.Println("김인턴 응답:")
	if strings.TrimSpace(verificationOutput.BotMessage) == "" {
		fmt.Println("(빈 Mattermost 메시지)")
	} else {
		fmt.Println(strings.TrimSpace(verificationOutput.BotMessage))
	}
	if strings.TrimSpace(verificationOutput.TaskRunID) != "" {
		status := "unknown"
		if verificationOutput.TaskStatus != nil && strings.TrimSpace(*verificationOutput.TaskStatus) != "" {
			status = strings.TrimSpace(*verificationOutput.TaskStatus)
		}
		fmt.Println()
		fmt.Println("Task: " + verificationOutput.TaskRunID + " (" + status + ")")
	}
	if len(downloadedFilePaths) == 0 {
		fmt.Println()
		fmt.Println("첨부 파일: 없음")
		return
	}
	fmt.Println()
	fmt.Println("첨부 파일:")
	for _, downloadedFilePath := range downloadedFilePaths {
		fmt.Println("- " + downloadedFilePath)
	}
}

func openDownloadedTestFiles(downloadedFilePaths []string, shouldOpenFiles bool) error {
	if !shouldOpenFiles || len(downloadedFilePaths) == 0 {
		return nil
	}
	if runtime.GOOS != "darwin" {
		fmt.Println("open skipped: this host is not macOS")
		return nil
	}
	for _, downloadedFilePath := range downloadedFilePaths {
		if errorValue := exec.Command("open", downloadedFilePath).Run(); errorValue != nil {
			return fmt.Errorf("open %s: %w", downloadedFilePath, errorValue)
		}
		fmt.Println("opened: " + downloadedFilePath)
	}
	return nil
}

func writeTestResultJSONBestEffort(target verifyTarget, configuration testCommandConfiguration, output string) {
	if strings.TrimSpace(configuration.ResultJSONPath) == "" {
		return
	}
	verificationOutput, errorValue := parseMattermostVerificationOutput(output)
	if errorValue != nil {
		return
	}
	downloadedFilePaths, _ := writeDownloadedMattermostFilesAllowEmpty(output, configuration.DownloadDirectoryPath)
	taskDetail, taskDetailError := fetchTestTaskDetailJSON(target, verificationOutput.TaskRunID)
	if errorValue := writeTestResultJSON(configuration.ResultJSONPath, verificationOutput, downloadedFilePaths, taskDetail, taskDetailError); errorValue != nil {
		fmt.Println("warning: failed to write best-effort test result JSON: " + errorValue.Error())
	}
}

func writeTestResultJSON(resultJSONPath string, verificationOutput mattermostVerificationOutput, downloadedFilePaths []string, taskDetail json.RawMessage, taskDetailError string) error {
	normalizedPath := strings.TrimSpace(resultJSONPath)
	if normalizedPath == "" {
		return nil
	}
	parentPath := filepath.Dir(normalizedPath)
	if parentPath != "." {
		if errorValue := os.MkdirAll(parentPath, 0o755); errorValue != nil {
			return fmt.Errorf("create test result parent directory: %w", errorValue)
		}
	}
	document := struct {
		mattermostVerificationOutput
		DownloadedFilePaths []string        `json:"downloadedFilePaths"`
		TaskDetail          json.RawMessage `json:"taskDetail,omitempty"`
		TaskDetailError     string          `json:"taskDetailError,omitempty"`
	}{
		mattermostVerificationOutput: verificationOutput,
		DownloadedFilePaths:          downloadedFilePaths,
		TaskDetail:                   taskDetail,
		TaskDetailError:              taskDetailError,
	}
	content, errorValue := json.MarshalIndent(document, "", "  ")
	if errorValue != nil {
		return fmt.Errorf("marshal test result JSON: %w", errorValue)
	}
	if errorValue := os.WriteFile(normalizedPath, append(content, '\n'), 0o644); errorValue != nil {
		return fmt.Errorf("write test result JSON: %w", errorValue)
	}
	return nil
}
