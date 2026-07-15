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
	"sort"
	"strconv"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/localfleet"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const (
	testSuiteCheap     = "cheap"
	testSuiteExpensive = "expensive"
	testSuiteFull      = "full"
)

type testCommandConfiguration struct {
	Suite                 string
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
	MaximumModelTier      string
	ScenarioNames         []string
}

func runTest() {
	if errorValue := runTestArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runTestArguments(arguments []string) error {
	configuration, errorValue := parseTestArguments(arguments, time.Now())
	if errors.Is(errorValue, flag.ErrHelp) {
		return nil
	}
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
	timeoutSeconds := flagSet.Int("timeout", 900, "Maximum seconds for the whole task or scenario; individual provider requests are not limited")
	generationSeed := flagSet.Int64("seed", 41, "Generation seed to apply before the Mattermost prompt")
	generationTemperature := flagSet.Float64("temperature", 0, "Generation temperature to apply before the Mattermost prompt")
	expectPublicURL := flagSet.Bool("expect-public-url", false, "Require a public URL and remote desktop/mobile screenshot verification")
	maximumModelTier := flagSet.String("maximum-model-tier", "", "Maximum model tier for costed tests: xlow, low, medium, high, xhigh, or max")
	expectedTools := repeatedStringFlag{}
	scenarioNames := repeatedStringFlag{}
	flagSet.Var(&expectedTools, "expect-tool", "Require a requested tool event; repeat for multiple tools")
	flagSet.Var(&scenarioNames, "scenario", "Run one expensive scenario by name; repeat for multiple scenarios")
	flagArguments, positionalArguments := splitFlagsAndPositionals(arguments, map[string]bool{
		"reuse":             true,
		"keep":              true,
		"no-open":           true,
		"real":              true,
		"auto-confirm":      true,
		"expect-public-url": true,
		"help":              true,
	}, map[string]bool{
		"o":                  true,
		"result-json":        true,
		"run-id":             true,
		"timeout":            true,
		"seed":               true,
		"temperature":        true,
		"expect-tool":        true,
		"maximum-model-tier": true,
		"scenario":           true,
	})
	if errorValue := flagSet.Parse(flagArguments); errorValue != nil {
		return testCommandConfiguration{}, errorValue
	}
	suite, prompt, errorValue := parseTestSuiteAndPrompt(positionalArguments)
	if errorValue != nil {
		return testCommandConfiguration{}, errorValue
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
	normalizedMaximumModelTier, errorValue := blueclaw.NormalizeMaximumModelTier(*maximumModelTier)
	if errorValue != nil {
		return testCommandConfiguration{}, errorValue
	}
	if *useRealModels && normalizedMaximumModelTier != "" {
		return testCommandConfiguration{}, errors.New("--real cannot be combined with --maximum-model-tier")
	}
	if !*useRealModels && normalizedMaximumModelTier == "" {
		normalizedMaximumModelTier = "xlow"
	}
	defaultDownloadDirectoryPath := filepath.Join("/tmp", "internkim-test-"+now.UTC().Format("20060102T150405"))
	return testCommandConfiguration{
		Suite:                 suite,
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
		MaximumModelTier:      normalizedMaximumModelTier,
		ScenarioNames:         scenarioNames.Values(),
	}, nil
}

func parseTestSuiteAndPrompt(positionalArguments []string) (string, string, error) {
	if len(positionalArguments) == 1 {
		candidateSuite := strings.ToLower(strings.TrimSpace(positionalArguments[0]))
		if candidateSuite == testSuiteCheap || candidateSuite == testSuiteExpensive || candidateSuite == testSuiteFull {
			return candidateSuite, "", nil
		}
	}
	prompt := strings.TrimSpace(strings.Join(positionalArguments, " "))
	if prompt == "" {
		return "", "", errors.New("usage: internkim test <cheap|expensive|full> [--scenario name] [--maximum-model-tier xlow] [--real] or internkim test \"<prompt>\"")
	}
	return "", prompt, nil
}

func runTestConfiguration(contextValue context.Context, configuration testCommandConfiguration) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	if configuration.Suite != "" {
		return runTestSuite(contextValue, repositoryRootPath, configuration)
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
		MaximumModelTier:      configuration.MaximumModelTier,
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

func runTestSuite(contextValue context.Context, repositoryRootPath string, configuration testCommandConfiguration) error {
	if configuration.Suite == testSuiteCheap || configuration.Suite == testSuiteFull {
		if errorValue := runCheapTestSuite(contextValue, repositoryRootPath); errorValue != nil {
			return errorValue
		}
	}
	if configuration.Suite == testSuiteExpensive || configuration.Suite == testSuiteFull {
		return runExpensiveTestSuite(contextValue, repositoryRootPath, configuration)
	}
	return fmt.Errorf("unsupported test suite: %s", configuration.Suite)
}

func runCheapTestSuite(contextValue context.Context, repositoryRootPath string) error {
	fmt.Println("Test suite: cheap")
	command := exec.CommandContext(contextValue, "make", "check")
	command.Dir = repositoryRootPath
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if errorValue := command.Run(); errorValue != nil {
		return fmt.Errorf("cheap test suite failed: %w", errorValue)
	}
	return nil
}

type expensiveScenarioReference struct {
	Name string `json:"name"`
	Path string `json:"-"`
}

func runExpensiveTestSuite(contextValue context.Context, repositoryRootPath string, configuration testCommandConfiguration) error {
	scenarios, errorValue := loadExpensiveScenarioReferences(repositoryRootPath, configuration.ScenarioNames)
	if errorValue != nil {
		return errorValue
	}
	embeddingService, errorValue := startLocalTestEmbeddingService(contextValue, repositoryRootPath)
	if errorValue != nil {
		return errorValue
	}
	defer embeddingService.stop()
	fmt.Println("Test suite: expensive")
	fmt.Println("Embedding model: baai/bge-m3 (local llama.cpp)")
	fmt.Println("Generation options: seed=" + strconv.FormatInt(configuration.GenerationSeed, 10) + " temperature=" + formatTestFloat(configuration.GenerationTemperature))
	if configuration.ShouldUseRealModels {
		fmt.Println("Model tiers: production (--real)")
	} else {
		fmt.Println("Maximum model tier: " + configuration.MaximumModelTier)
	}
	return runSequentialExpensiveScenarios(scenarios, func(scenario expensiveScenarioReference) error {
		return runExpensiveScenario(contextValue, repositoryRootPath, configuration, scenario, embeddingService.endpoint)
	})
}

func runSequentialExpensiveScenarios(scenarios []expensiveScenarioReference, runScenario func(expensiveScenarioReference) error) error {
	for scenarioIndex, scenario := range scenarios {
		fmt.Printf("\n[%d/%d] %s\n", scenarioIndex+1, len(scenarios), scenario.Name)
		if errorValue := runScenario(scenario); errorValue != nil {
			return fmt.Errorf("expensive scenario %s failed: %w", scenario.Name, errorValue)
		}
	}
	return nil
}

func loadExpensiveScenarioReferences(repositoryRootPath string, selectedScenarioNames []string) ([]expensiveScenarioReference, error) {
	scenarioPaths, errorValue := filepath.Glob(filepath.Join(repositoryRootPath, "tests", "expensive", "*.json"))
	if errorValue != nil {
		return nil, errorValue
	}
	sort.Strings(scenarioPaths)
	selectedNames := testStringSet(trimmedNonEmptyValues(selectedScenarioNames))
	scenarios := make([]expensiveScenarioReference, 0, len(scenarioPaths))
	matchedNames := map[string]bool{}
	for _, scenarioPath := range scenarioPaths {
		document, errorValue := os.ReadFile(scenarioPath)
		if errorValue != nil {
			return nil, errorValue
		}
		var scenario expensiveScenarioReference
		if errorValue := json.Unmarshal(document, &scenario); errorValue != nil {
			return nil, fmt.Errorf("read expensive scenario %s: %w", scenarioPath, errorValue)
		}
		scenario.Name = strings.TrimSpace(scenario.Name)
		scenario.Path = scenarioPath
		baseName := strings.TrimSuffix(filepath.Base(scenarioPath), filepath.Ext(scenarioPath))
		if len(selectedNames) > 0 && !selectedNames[scenario.Name] && !selectedNames[baseName] {
			continue
		}
		matchedNames[scenario.Name] = true
		matchedNames[baseName] = true
		scenarios = append(scenarios, scenario)
	}
	if len(scenarios) == 0 {
		return nil, errors.New("no expensive scenarios matched")
	}
	for selectedName := range selectedNames {
		if !matchedNames[selectedName] {
			return nil, fmt.Errorf("unknown expensive scenario: %s", selectedName)
		}
	}
	return scenarios, nil
}

func testStringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func runExpensiveScenario(contextValue context.Context, repositoryRootPath string, configuration testCommandConfiguration, scenario expensiveScenarioReference, embeddingEndpoint string) error {
	timeout := time.Duration(configuration.TimeoutSeconds) * time.Second
	scenarioContext, cancel := context.WithTimeout(contextValue, timeout)
	defer cancel()
	artifactDirectoryPath := filepath.Join(repositoryRootPath, ".artifacts", "expensive", safeTestScenarioName(scenario.Name))
	arguments := []string{
		"run", "./cmd/blueclaw-lab", "virtual-session",
		"--scenario-file", scenario.Path,
		"--artifact-dir", artifactDirectoryPath,
		"--live-llm", "--strict-assertions",
		"--seed", strconv.FormatInt(configuration.GenerationSeed, 10),
		"--temperature", formatTestFloat(configuration.GenerationTemperature),
		"--embedding-endpoint", embeddingEndpoint,
	}
	if configuration.ShouldUseRealModels {
		arguments = append(arguments, "--real-model-tiers")
	} else {
		arguments = append(arguments, "--maximum-model-tier", configuration.MaximumModelTier)
	}
	command := exec.CommandContext(scenarioContext, "go", arguments...)
	command.Dir = filepath.Join(repositoryRootPath, ".dependency", "blueclaw")
	command.Env = append(os.Environ(), "GOCACHE=/tmp/internkim-expensive-go-cache")
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	errorValue := command.Run()
	if errors.Is(scenarioContext.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("timed out after %s", timeout)
	}
	return errorValue
}

func safeTestScenarioName(name string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(strings.TrimSpace(name)) {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' || character == '_' {
			builder.WriteRune(character)
		} else {
			builder.WriteRune('-')
		}
	}
	return strings.Trim(builder.String(), "-")
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
