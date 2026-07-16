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
	testSuiteCheap              = "cheap"
	testSuiteExpensive          = "expensive"
	testSuiteFull               = "full"
	defaultPromptTimeoutSeconds = 900
)

type testCommandConfiguration struct {
	Suite                      string
	Prompt                     string
	DownloadDirectoryPath      string
	OutputFilePath             string
	ResultJSONPath             string
	RunID                      string
	TimeoutSeconds             int
	GenerationSeed             int64
	GenerationTemperature      float64
	ExpectedTools              []string
	ShouldExpectPublicURL      bool
	ShouldReuseFleet           bool
	ShouldSkipProvisioning     bool
	ShouldKeepArtifacts        bool
	ShouldOpenFiles            bool
	ShouldUseRealModels        bool
	ShouldAutoConfirm          bool
	MaximumModelTier           string
	ScenarioNames              []string
	LanguageModelProvider      string
	LanguageModelEndpoint      string
	LanguageModelSocket        string
	LanguageModelAuthKeyPath   string
	LanguageModelExecutionMode string
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
	skipProvisioning := flagSet.Bool("skip-provisioning", false, "Run against an already prepared Local Fleet")
	keepArtifacts := flagSet.Bool("keep", false, "Keep the Local Fleet VM and evidence after the test")
	noOpen := flagSet.Bool("no-open", false, "Download files without opening them")
	useRealModels := flagSet.Bool("real", false, "Use production model configuration instead of the Local Fleet test model")
	autoConfirm := flagSet.Bool("auto-confirm", false, "Automatically approve Mattermost confirmation prompts during the test")
	outputFilePath := flagSet.String("o", "", "Local output file path for one Mattermost attachment")
	resultJSONPath := flagSet.String("result-json", "", "Write the parsed Mattermost test result JSON to this local path")
	runID := flagSet.String("run-id", "", "Optional disposable Local Fleet run identifier")
	timeoutSeconds := flagSet.Int("timeout", 0, "Maximum seconds for the whole expensive scenario; 0 disables the deadline")
	generationSeed := flagSet.Int64("seed", 41, "Generation seed to apply before the Mattermost prompt")
	generationTemperature := flagSet.Float64("temperature", 0, "Generation temperature to apply before the Mattermost prompt")
	expectPublicURL := flagSet.Bool("expect-public-url", false, "Require a public URL and remote desktop/mobile screenshot verification")
	maximumModelTier := flagSet.String("maximum-model-tier", "", "Maximum model tier for costed tests: xlow, low, medium, high, xhigh, or max")
	languageModelProviderDefault := strings.TrimSpace(os.Getenv("BLUECLAW_E2E_LLM_PROVIDER"))
	if languageModelProviderDefault == "" {
		languageModelProviderDefault = "sdkd"
	}
	languageModelProvider := flagSet.String("llm-provider", languageModelProviderDefault, "Live LLM provider: openrouter, capability, or sdkd")
	languageModelEndpoint := flagSet.String("llm-endpoint", "", "Live LLM endpoint; defaults to BLUECLAW_E2E_LLM_ENDPOINT")
	languageModelSocket := flagSet.String("llm-unix-socket", "", "Live LLM Unix socket; defaults to BLUECLAW_E2E_LLM_UNIX_SOCKET")
	languageModelAuthKeyPath := flagSet.String("llm-auth-key-path", "", "SDKD installation auth key path; defaults to BLUECLAW_E2E_LLM_AUTH_KEY_PATH")
	languageModelExecutionMode := flagSet.String("llm-execution-mode", "", "Live LLM execution mode; defaults to BLUECLAW_E2E_LLM_EXECUTION_MODE")
	expectedTools := repeatedStringFlag{}
	scenarioNames := repeatedStringFlag{}
	flagSet.Var(&expectedTools, "expect-tool", "Require a requested tool event; repeat for multiple tools")
	flagSet.Var(&scenarioNames, "scenario", "Run one expensive scenario by name; repeat for multiple scenarios")
	flagArguments, positionalArguments := splitFlagsAndPositionals(arguments, map[string]bool{
		"reuse":             true,
		"skip-provisioning": true,
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
		"llm-provider":       true,
		"llm-endpoint":       true,
		"llm-unix-socket":    true,
		"llm-auth-key-path":  true,
		"llm-execution-mode": true,
	})
	if errorValue := flagSet.Parse(flagArguments); errorValue != nil {
		return testCommandConfiguration{}, errorValue
	}
	suite, prompt, errorValue := parseTestSuiteAndPrompt(positionalArguments)
	if errorValue != nil {
		return testCommandConfiguration{}, errorValue
	}
	if *timeoutSeconds < 0 {
		return testCommandConfiguration{}, errors.New("--timeout must be 0 or greater")
	}
	if math.IsNaN(*generationTemperature) || math.IsInf(*generationTemperature, 0) || *generationTemperature < 0 {
		return testCommandConfiguration{}, errors.New("--temperature must be 0 or greater")
	}
	if *reuseFleet && strings.TrimSpace(*runID) != "" {
		return testCommandConfiguration{}, errors.New("--run-id requires a disposable Local Fleet run; remove --reuse")
	}
	if *skipProvisioning && suite != testSuiteExpensive {
		return testCommandConfiguration{}, errors.New("--skip-provisioning requires the expensive suite")
	}
	if *skipProvisioning && !*reuseFleet && strings.TrimSpace(*runID) == "" {
		return testCommandConfiguration{}, errors.New("--skip-provisioning requires --run-id or --reuse")
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
	normalizedLanguageModelProvider, errorValue := normalizeTestLanguageModelProvider(*languageModelProvider)
	if errorValue != nil {
		return testCommandConfiguration{}, errorValue
	}
	defaultDownloadDirectoryPath := filepath.Join("/tmp", "internkim-test-"+now.UTC().Format("20060102T150405"))
	return testCommandConfiguration{
		Suite:                      suite,
		Prompt:                     prompt,
		DownloadDirectoryPath:      defaultDownloadDirectoryPath,
		OutputFilePath:             strings.TrimSpace(*outputFilePath),
		ResultJSONPath:             strings.TrimSpace(*resultJSONPath),
		RunID:                      strings.TrimSpace(*runID),
		TimeoutSeconds:             *timeoutSeconds,
		GenerationSeed:             *generationSeed,
		GenerationTemperature:      *generationTemperature,
		ExpectedTools:              expectedTools.Values(),
		ShouldExpectPublicURL:      *expectPublicURL,
		ShouldReuseFleet:           *reuseFleet,
		ShouldSkipProvisioning:     *skipProvisioning,
		ShouldKeepArtifacts:        *keepArtifacts,
		ShouldOpenFiles:            !*noOpen,
		ShouldUseRealModels:        *useRealModels,
		ShouldAutoConfirm:          *autoConfirm,
		MaximumModelTier:           normalizedMaximumModelTier,
		ScenarioNames:              scenarioNames.Values(),
		LanguageModelProvider:      normalizedLanguageModelProvider,
		LanguageModelEndpoint:      strings.TrimSpace(*languageModelEndpoint),
		LanguageModelSocket:        strings.TrimSpace(*languageModelSocket),
		LanguageModelAuthKeyPath:   strings.TrimSpace(*languageModelAuthKeyPath),
		LanguageModelExecutionMode: strings.TrimSpace(*languageModelExecutionMode),
	}, nil
}

func normalizeTestLanguageModelProvider(provider string) (string, error) {
	normalizedProvider := strings.ToLower(strings.TrimSpace(provider))
	switch normalizedProvider {
	case "openrouter", "capability", "sdkd":
		return normalizedProvider, nil
	default:
		return "", fmt.Errorf("llm provider must be openrouter, capability, or sdkd: %s", provider)
	}
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
		SDKDMode:              testSDKDMode(configuration.LanguageModelProvider),
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

func testSDKDMode(provider string) localfleet.SDKDMode {
	if provider == "sdkd" {
		return localfleet.SDKDModeAuthoritative
	}
	return ""
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

var realMattermostScenarioNames = map[string]bool{
	"task-lifecycle":     true,
	"calendar-lifecycle": true,
	"website-lifecycle":  true,
	"document-lifecycle": true,
}

func runExpensiveTestSuite(contextValue context.Context, repositoryRootPath string, configuration testCommandConfiguration) error {
	scenarios, errorValue := loadExpensiveScenarioReferences(repositoryRootPath, configuration.ScenarioNames)
	if errorValue != nil {
		return errorValue
	}
	executablePath, errorValue := currentExecutablePath()
	if errorValue != nil {
		return errorValue
	}
	runID := firstNonEmptyString(configuration.RunID, "expensive-"+time.Now().UTC().Format("20060102T150405")+"-"+randomHexString(4))
	service, errorValue := localfleet.NewService(localfleet.Options{
		RepositoryRootPath:    repositoryRootPath,
		ExecutablePath:        executablePath,
		RunID:                 runID,
		GenerationSeed:        strconv.FormatInt(configuration.GenerationSeed, 10),
		GenerationTemperature: formatTestFloat(configuration.GenerationTemperature),
		MaximumModelTier:      configuration.MaximumModelTier,
		SDKDMode:              localfleet.SDKDModeAuthoritative,
		IsEphemeral:           !configuration.ShouldReuseFleet,
		ShouldUseRealModels:   configuration.ShouldUseRealModels,
	})
	if errorValue != nil {
		return errorValue
	}
	logger := standardLocalFleetLogger{}
	shouldCleanupFleet := !configuration.ShouldKeepArtifacts && !configuration.ShouldReuseFleet
	fmt.Println("Test suite: expensive")
	fmt.Println("Environment: Local Fleet Mattermost DM")
	fmt.Println("LLM runtime: SDKD authoritative")
	fmt.Println("Generation options: seed=" + strconv.FormatInt(configuration.GenerationSeed, 10) + " temperature=" + formatTestFloat(configuration.GenerationTemperature))
	if configuration.ShouldUseRealModels {
		fmt.Println("Model tiers: production (--real)")
	} else {
		fmt.Println("Maximum model tier: " + configuration.MaximumModelTier)
	}
	var runError error
	if configuration.ShouldSkipProvisioning {
		fmt.Println("Provisioning: skipped (restoring prepared Local Fleet connectivity)")
		runError = service.ConnectPreparedFleet(contextValue, logger)
	} else {
		runError = service.Run(contextValue, logger, localfleet.JobRequest{Action: localfleet.ActionUp, KeepArtifacts: true, SkipWeb: false})
	}
	if runError == nil {
		runError = runExpensiveMattermostScenarios(contextValue, repositoryRootPath, executablePath, runID, service, configuration, scenarios)
	}
	if shouldCleanupFleet {
		cleanupError := service.CleanupEphemeral(contextValue, logger)
		return errors.Join(runError, cleanupError)
	}
	return runError
}

func runExpensiveMattermostScenarios(contextValue context.Context, repositoryRootPath string, executablePath string, runID string, service localfleet.Service, configuration testCommandConfiguration, scenarios []expensiveScenarioReference) error {
	status := service.Status(contextValue)
	if errorValue := validateExpensiveFleetStatus(status); errorValue != nil {
		return errorValue
	}
	target, errorValue := resolveLocalFleetTestTarget(contextValue, service, repositoryRootPath, executablePath)
	if errorValue != nil {
		return errorValue
	}
	return runSequentialExpensiveScenarios(scenarios, func(scenario expensiveScenarioReference) error {
		return runExpensiveMattermostScenario(contextValue, repositoryRootPath, runID, target, status.AdminURL, status.MattermostURL, configuration, scenario)
	})
}

func validateExpensiveFleetStatus(status localfleet.Status) error {
	endpoints := map[string]localfleet.EndpointStatus{
		"virtual machine": status.VirtualMachine,
		"SSH":             status.SSH,
		"admin":           status.Admin,
		"Mattermost":      status.Mattermost,
	}
	for name, endpoint := range endpoints {
		if endpoint.State != "ok" {
			return fmt.Errorf("Local Fleet %s is %s: %s", name, endpoint.State, endpoint.Message)
		}
	}
	if strings.TrimSpace(status.MattermostURL) == "" {
		return errors.New("Local Fleet Mattermost URL is empty")
	}
	if strings.TrimSpace(status.AdminURL) == "" {
		return errors.New("Local Fleet admin URL is empty")
	}
	return nil
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
		if len(selectedNames) == 0 && !realMattermostScenarioNames[scenario.Name] {
			continue
		}
		if len(selectedNames) > 0 && !selectedNames[scenario.Name] && !selectedNames[baseName] {
			continue
		}
		if !realMattermostScenarioNames[scenario.Name] {
			return nil, fmt.Errorf("expensive scenario %s does not yet have a real Mattermost topology", scenario.Name)
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

func runExpensiveMattermostScenario(contextValue context.Context, repositoryRootPath string, runID string, target verifyTarget, siteProxyURL string, mattermostURL string, configuration testCommandConfiguration, scenarioReference expensiveScenarioReference) error {
	scenarioContext, cancel := expensiveScenarioContext(contextValue, configuration.TimeoutSeconds)
	defer cancel()
	scenario, errorValue := loadMattermostScenario(scenarioReference.Path)
	if errorValue != nil {
		return errorValue
	}
	scenario.MaximumModelTier = maximumMattermostScenarioModelTier(configuration)
	artifactDirectoryPath := filepath.Join(repositoryRootPath, ".artifacts", "expensive", safeTestScenarioName(runID), safeTestScenarioName(scenario.Name))
	if errorValue := os.MkdirAll(artifactDirectoryPath, 0o755); errorValue != nil {
		return errorValue
	}
	session, errorValue := startMattermostScenarioSession(scenarioContext, target, mattermostURL, scenario)
	if errorValue != nil {
		return errorValue
	}
	session.shouldAutoConfirm = configuration.ShouldAutoConfirm
	runError := session.run(scenarioContext, func(hookContext context.Context, execution mattermostScenarioExecution, stepIndex int) error {
		if errorValue := writeExpensiveMattermostEvidence(artifactDirectoryPath, execution.Result); errorValue != nil {
			return errorValue
		}
		return verifyExpensiveMattermostStep(hookContext, repositoryRootPath, artifactDirectoryPath, siteProxyURL, mattermostURL, scenario, execution, stepIndex, configuration.ShouldAutoConfirm)
	})
	writeError := writeExpensiveMattermostEvidence(artifactDirectoryPath, session.result)
	cleanupContext, cancelCleanup := context.WithTimeout(context.Background(), 5*time.Minute)
	cleanupError := session.cleanup(cleanupContext)
	cancelCleanup()
	if configuration.TimeoutSeconds > 0 && errors.Is(scenarioContext.Err(), context.DeadlineExceeded) {
		runError = fmt.Errorf("timed out after %s", time.Duration(configuration.TimeoutSeconds)*time.Second)
	}
	return errors.Join(runError, writeError, cleanupError)
}

func maximumMattermostScenarioModelTier(configuration testCommandConfiguration) string {
	if configuration.ShouldUseRealModels {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(configuration.MaximumModelTier), "low") {
		return "low"
	}
	return ""
}

func expensiveScenarioContext(parent context.Context, timeoutSeconds int) (context.Context, context.CancelFunc) {
	if timeoutSeconds <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, time.Duration(timeoutSeconds)*time.Second)
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
	timeoutSeconds := promptTimeoutSeconds(configuration.TimeoutSeconds)
	script := verifyMattermostPromptScript(configuration.Prompt, configuration.ShouldKeepArtifacts, timeoutSeconds, false, configuration.ShouldExpectPublicURL, configuration.ExpectedTools, nil, true, true, configuration.ShouldAutoConfirm)
	output, errorValue := target.sshClient.runResultWithTimeout(script, mattermostPromptScriptSSHTimeout(timeoutSeconds, configuration.ShouldExpectPublicURL))
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

func promptTimeoutSeconds(timeoutSeconds int) int {
	if timeoutSeconds <= 0 {
		return defaultPromptTimeoutSeconds
	}
	return timeoutSeconds
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
	return newLocalFleetTestTarget(repositoryRootPath, executablePath, service.ConfigurationPath(), host), nil
}

func newLocalFleetTestTarget(repositoryRootPath string, executablePath string, configurationPath string, host string) verifyTarget {
	sshpassBin := filepath.Join(repositoryRootPath, "bin", "sshpass")
	return verifyTarget{
		host:       host,
		user:       "admin",
		password:   "admin",
		scriptDir:  repositoryRootPath,
		sshpassBin: sshpassBin,
		sshClient:  newSSH(sshpassBin, "admin", "admin", host),
		scenarioRemote: mattermostScenarioLocalFleetRemote{
			executablePath:    executablePath,
			configurationPath: configurationPath,
		},
	}
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
