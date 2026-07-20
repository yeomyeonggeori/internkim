package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/localfleet"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type devVirtualSessionArguments struct {
	ScenarioName             string
	ScenarioFilePath         string
	ArtifactDirectoryPath    string
	SkillDirectoryPath       string
	LanguageModelEndpoint    string
	LanguageModelSocket      string
	LanguageModelProvider    string
	LanguageModelAuthKeyPath string
	LanguageModelName        string
	ExecutionMode            string
	Seed                     string
	Temperature              string
	MaximumModelTier         string
	RequiredExecutables      []string
	IsLiveLanguageModel      bool
	HasStrictAssertions      bool
	ShouldSkipPreflight      bool
}

type devCommandInvocation struct {
	WorkingDirectoryPath string
	Arguments            []string
}

var runDevLocalVirtualSession = runLocalDevVirtualSession

func runDev() {
	if errorValue := runDevArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runDevArguments(arguments []string) error {
	subcommand := "simulate"
	commandArguments := arguments
	if len(arguments) > 0 && !strings.HasPrefix(arguments[0], "-") {
		subcommand = arguments[0]
		commandArguments = arguments[1:]
	}

	switch subcommand {
	case "simulate":
		return runDevSimulateArguments(commandArguments)
	case "fleet":
		return runDevFleetArguments(commandArguments)
	case "help":
		printDevUsage()
		return nil
	default:
		return fmt.Errorf("unknown dev subcommand: %s", subcommand)
	}
}

type standardLocalFleetLogger struct{}

func (logger standardLocalFleetLogger) Info(message string) {
	fmt.Println(message)
}

func runDevFleetArguments(arguments []string) error {
	if len(arguments) == 0 {
		service, errorValue := newLocalFleetService()
		if errorValue != nil {
			return errorValue
		}
		return printLocalFleetStatus(service)
	}
	subcommand := arguments[0]
	commandArguments := arguments[1:]
	switch subcommand {
	case "up":
		service, errorValue := newLocalFleetService()
		if errorValue != nil {
			return errorValue
		}
		contextValue, stop := interruptContext()
		defer stop()
		return service.Run(contextValue, standardLocalFleetLogger{}, localfleet.JobRequest{Action: localfleet.ActionUp})
	case "down":
		service, errorValue := newLocalFleetService()
		if errorValue != nil {
			return errorValue
		}
		contextValue, stop := interruptContext()
		defer stop()
		return service.Run(contextValue, standardLocalFleetLogger{}, localfleet.JobRequest{Action: localfleet.ActionDown})
	case "status":
		service, errorValue := newLocalFleetService()
		if errorValue != nil {
			return errorValue
		}
		return printLocalFleetStatus(service)
	case "reset":
		service, errorValue := newLocalFleetService()
		if errorValue != nil {
			return errorValue
		}
		contextValue, stop := interruptContext()
		defer stop()
		return service.Run(contextValue, standardLocalFleetLogger{}, localfleet.JobRequest{Action: localfleet.ActionReset})
	case "run":
		return runDevFleetRunArguments(commandArguments)
	case "reprovision":
		return runDevFleetReprovision(commandArguments)
	case "verify-regression":
		service, errorValue := newLocalFleetService()
		if errorValue != nil {
			return errorValue
		}
		return runDevFleetVerifyRegressionArguments(service, commandArguments)
	case "help":
		printDevFleetUsage()
		return nil
	default:
		return fmt.Errorf("unknown dev fleet subcommand: %s", subcommand)
	}
}

type devFleetRunConfiguration struct {
	ServiceOptions localfleet.Options
	Request        localfleet.JobRequest
}

// Reprovision the running local fleet VM in place from the current working tree.
// The Firecracker guest runs from a baked rootfs, so Blueclaw, skill, prompt, and
// runtime changes only reach it through a reprovision; copying files onto the host
// and restarting the service does not update the guest. GO_MOD_CACHE must point at
// the real module cache or the payload build fails on the empty isolated cache.
func runDevFleetReprovision(arguments []string) error {
	flagSet := flag.NewFlagSet("dev fleet reprovision", flag.ContinueOnError)
	configurationPathArgument := flagSet.String("config", "", "Local Fleet configuration path")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	executablePath, errorValue := currentExecutablePath()
	if errorValue != nil {
		return errorValue
	}
	configurationPath := strings.TrimSpace(*configurationPathArgument)
	if configurationPath == "" {
		configurationPath, errorValue = latestLocalFleetConfigurationPath(repositoryRootPath)
		if errorValue != nil {
			return errorValue
		}
	} else if !filepath.IsAbs(configurationPath) {
		configurationPath = filepath.Join(repositoryRootPath, configurationPath)
	}
	vmInternetProtocolAddress, errorValue := localFleetVMInternetProtocolAddress(executablePath, configurationPath)
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("reprovisioning local fleet at %s from the working tree\n", vmInternetProtocolAddress)

	command := exec.Command(executablePath, "setup", "--board", "lab", "--ssh", "--host", vmInternetProtocolAddress,
		"--user", "admin", "--password", "admin",
		"--admin-email", "local-fleet-admin@internkim.test",
		"--wait-lock", "--force", "--skip", "wifi,local-llm,cloudflare-access,tunnel,google,slack,mattermost,web,blueclaw-runtime-base")
	command.Env = devFleetReprovisionEnvironment(os.Environ(), goModuleCachePath())
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func devFleetReprovisionEnvironment(environment []string, moduleCachePath string) []string {
	environment = append(environment,
		"INTERNKIM_BLUECLAW_USE_LOCAL=1",
		"INTERNKIM_SKIP_PAGES_DEPLOY_FOR_LAB=1",
		"INTERNKIM_TEST_MODEL_TIER=low",
		blueclaw.BlueclawTestMaximumModelTierEnvironment+"=low",
		blueclaw.BlueclawLLMDModeEnvironment+"=authoritative",
		"INTERNKIM_BLUECLAW_VCPU_COUNT=4")
	if moduleCachePath == "" {
		return environment
	}
	return append(environment, "GO_MOD_CACHE="+moduleCachePath)
}

func latestLocalFleetConfigurationPath(repositoryRootPath string) (string, error) {
	canonicalPath := filepath.Join(repositoryRootPath, ".local", "local-fleet", "config.json")
	fileInfo, errorValue := os.Stat(canonicalPath)
	if errorValue == nil && fileInfo.Mode().IsRegular() {
		return canonicalPath, nil
	}
	if errorValue != nil && !errors.Is(errorValue, os.ErrNotExist) {
		return "", errorValue
	}

	matches, errorValue := filepath.Glob(filepath.Join(repositoryRootPath, ".local", "local-fleet", "runs", "*", "config.json"))
	if errorValue != nil {
		return "", errorValue
	}
	latestPath := ""
	var latestModificationTime int64
	for _, match := range matches {
		fileInfo, statError := os.Stat(match)
		if statError != nil {
			continue
		}
		if latestPath == "" || fileInfo.ModTime().UnixNano() > latestModificationTime {
			latestPath = match
			latestModificationTime = fileInfo.ModTime().UnixNano()
		}
	}
	if latestPath == "" {
		return "", errors.New("no local fleet run config found; bring up a fleet first")
	}
	return latestPath, nil
}

func localFleetVMInternetProtocolAddress(executablePath string, configurationPath string) (string, error) {
	command := exec.Command(executablePath, "lab", "vm-ip", "--config", configurationPath)
	output, errorValue := command.Output()
	if errorValue != nil {
		return "", fmt.Errorf("could not resolve fleet VM IP: %w", errorValue)
	}
	lines := strings.Fields(strings.TrimSpace(string(output)))
	if len(lines) == 0 {
		return "", errors.New("fleet VM IP lookup returned no address")
	}
	return lines[len(lines)-1], nil
}

func goModuleCachePath() string {
	output, errorValue := exec.Command("go", "env", "GOMODCACHE").Output()
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func runDevFleetRunArguments(arguments []string) error {
	configuration, errorValue := parseDevFleetRunArguments(arguments)
	if errorValue != nil {
		return errorValue
	}
	service, errorValue := newLocalFleetServiceWithOptions(configuration.ServiceOptions)
	if errorValue != nil {
		return errorValue
	}
	contextValue, stop := interruptContext()
	defer stop()
	return service.Run(contextValue, standardLocalFleetLogger{}, configuration.Request)
}

func parseDevFleetRunArguments(arguments []string) (devFleetRunConfiguration, error) {
	flagSet := flag.NewFlagSet("dev fleet run", flag.ContinueOnError)
	recipe := flagSet.String("recipe", "", "Local fleet recipe to run")
	scenario := flagSet.String("scenario", "", "Local fleet scenario to run")
	ephemeral := flagSet.Bool("ephemeral", false, "Deprecated; disposable local fleet runs are now the default")
	reuseFleet := flagSet.Bool("reuse", false, "Reuse the shared local fleet instead of creating a disposable run")
	keepArtifacts := flagSet.Bool("keep", false, "Keep disposable VM and Mattermost test artifacts; run evidence is kept by default")
	withoutMattermost := flagSet.Bool("without-mattermost", false, "Run the scenario inside Linux without starting or using Mattermost")
	runID := flagSet.String("run-id", "", "Optional disposable run identifier")
	adminHostPort := flagSet.Int("admin-port", 0, "Host port for the local admind tunnel")
	mattermostHostPort := flagSet.Int("mattermost-port", 0, "Host port for the local Mattermost tunnel")
	useRealModels := flagSet.Bool("real", false, "Use production model configuration instead of the Local Fleet test model")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return devFleetRunConfiguration{}, errorValue
	}
	trimmedScenario := strings.TrimSpace(*scenario)
	if *withoutMattermost && trimmedScenario == "" {
		return devFleetRunConfiguration{}, errors.New("without-mattermost mode requires --scenario")
	}
	if *ephemeral && *reuseFleet {
		return devFleetRunConfiguration{}, errors.New("use either --ephemeral or --reuse, not both")
	}
	if *reuseFleet && strings.TrimSpace(*runID) != "" {
		return devFleetRunConfiguration{}, errors.New("--run-id requires a disposable run; remove --reuse")
	}
	serviceOptions := localfleet.Options{
		IsEphemeral:         !*reuseFleet,
		RunID:               strings.TrimSpace(*runID),
		AdminHostPort:       *adminHostPort,
		MattermostHostPort:  *mattermostHostPort,
		ShouldUseRealModels: *useRealModels,
	}
	request := localfleet.JobRequest{
		KeepArtifacts:     *keepArtifacts,
		WithoutMattermost: *withoutMattermost,
	}
	if trimmedScenario != "" {
		request.Action = localfleet.ActionRunScenario
		request.Scenario = trimmedScenario
	} else {
		request.Action = localfleet.ActionRunRecipe
		request.Recipe = firstNonEmptyLocalFleetValue(*recipe, localfleet.DefaultRecipe)
	}
	return devFleetRunConfiguration{ServiceOptions: serviceOptions, Request: request}, nil
}

func runDevFleetVerifyRegressionArguments(service localfleet.Service, arguments []string) error {
	flagSet := flag.NewFlagSet("dev fleet verify-regression", flag.ContinueOnError)
	base := flagSet.String("base", "main", "Base branch or revision that should fail the scenario")
	scenario := flagSet.String("scenario", "", "Scenario that should fail on base and pass on current checkout")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}
	contextValue, stop := interruptContext()
	defer stop()
	return service.Run(contextValue, standardLocalFleetLogger{}, localfleet.JobRequest{Action: localfleet.ActionVerifyRegression, Base: *base, Scenario: *scenario})
}

func newLocalFleetService() (localfleet.Service, error) {
	return newLocalFleetServiceWithOptions(localfleet.Options{})
}

func newLocalFleetServiceWithOptions(options localfleet.Options) (localfleet.Service, error) {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return localfleet.Service{}, errorValue
	}
	executablePath, errorValue := currentExecutablePath()
	if errorValue != nil {
		return localfleet.Service{}, errorValue
	}
	options.RepositoryRootPath = repositoryRootPath
	options.ExecutablePath = executablePath
	return localfleet.NewService(localfleet.Options{
		RepositoryRootPath:    options.RepositoryRootPath,
		ExecutablePath:        options.ExecutablePath,
		StateRootPath:         options.StateRootPath,
		VirtualMachineName:    options.VirtualMachineName,
		RunID:                 options.RunID,
		AdminHostPort:         options.AdminHostPort,
		MattermostHostPort:    options.MattermostHostPort,
		GenerationSeed:        options.GenerationSeed,
		GenerationTemperature: options.GenerationTemperature,
		MaximumModelTier:      options.MaximumModelTier,
		ShouldUseRealModels:   options.ShouldUseRealModels,
		IsEphemeral:           options.IsEphemeral,
	})
}

func printLocalFleetStatus(service localfleet.Service) error {
	status := service.Status(context.Background())
	fmt.Printf("VM: %s %s\n", status.VirtualMachine.State, status.VirtualMachine.Message)
	fmt.Printf("SSH: %s %s\n", status.SSH.State, status.SSH.Message)
	fmt.Printf("Admin: %s %s\n", status.Admin.State, status.Admin.Message)
	fmt.Printf("Mattermost: %s %s\n", status.Mattermost.State, status.Mattermost.Message)
	if status.AdminURL != "" {
		fmt.Println("Admin URL: " + status.AdminURL)
	}
	if status.MattermostURL != "" {
		fmt.Println("Mattermost URL: " + status.MattermostURL)
	}
	if status.LastResult != "" {
		fmt.Println("Last result: " + status.LastResult)
	}
	return nil
}

func firstNonEmptyLocalFleetValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func runDevSimulateArguments(arguments []string) error {
	sessionArguments, errorValue := parseDevVirtualSessionArguments(arguments)
	if errorValue != nil {
		return errorValue
	}
	return runDevLocalVirtualSession(sessionArguments)
}

func parseDevVirtualSessionArguments(arguments []string) (devVirtualSessionArguments, error) {
	flagSet := flag.NewFlagSet("dev simulate", flag.ContinueOnError)
	scenarioName := flagSet.String("scenario", "schedule_create_acceptance", "Blueclaw virtual-session scenario")
	scenarioFilePath := flagSet.String("scenario-file", "", "File-backed sequential virtual-session scenario")
	artifactDirectoryPath := flagSet.String("artifact-dir", ".artifacts/blueclaw-dev", "Artifact directory for virtual-session output")
	skillDirectoryPath := flagSet.String("skill-dir", "", "Skill directory to load into the virtual workspace")
	languageModelEndpoint := flagSet.String("llm-endpoint", "", "Live LLM capability endpoint")
	languageModelSocket := flagSet.String("llm-unix-socket", "", "Live LLM capability unix socket path")
	languageModelProvider := flagSet.String("llm-provider", "", "Live LLM provider: openrouter, capability, or llmd")
	languageModelAuthKeyPath := flagSet.String("llm-auth-key-path", "", "LLMD installation auth key path")
	languageModelName := flagSet.String("llm-model", "", "Live LLM model override")
	executionMode := flagSet.String("llm-execution-mode", "", "Live LLM execution mode")
	liveLanguageModel := flagSet.Bool("live-llm", false, "Allow live LLM calls")
	skipPreflight := flagSet.Bool("skip-preflight", false, "Skip executable dependency checks")
	requiredExecutables := repeatedDevStringFlag{}
	flagSet.Var(&requiredExecutables, "require-executable", "Require an executable before running; repeat for multiple executables")
	seedValue := flagSet.Int64("seed", 41, "Generation seed for live LLM calls")
	temperatureValue := flagSet.Float64("temperature", 0, "Generation temperature for live LLM calls")
	maximumModelTier := flagSet.String("maximum-model-tier", "", "Maximum live model tier")
	strictAssertions := flagSet.Bool("strict-assertions", false, "Fail when a declared expectation is not satisfied")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return devVirtualSessionArguments{}, errorValue
	}

	return devVirtualSessionArguments{
		ScenarioName:             strings.TrimSpace(*scenarioName),
		ScenarioFilePath:         strings.TrimSpace(*scenarioFilePath),
		ArtifactDirectoryPath:    strings.TrimSpace(*artifactDirectoryPath),
		SkillDirectoryPath:       strings.TrimSpace(*skillDirectoryPath),
		LanguageModelEndpoint:    strings.TrimSpace(*languageModelEndpoint),
		LanguageModelSocket:      strings.TrimSpace(*languageModelSocket),
		LanguageModelProvider:    strings.TrimSpace(*languageModelProvider),
		LanguageModelAuthKeyPath: strings.TrimSpace(*languageModelAuthKeyPath),
		LanguageModelName:        strings.TrimSpace(*languageModelName),
		ExecutionMode:            strings.TrimSpace(*executionMode),
		Seed:                     strconv.FormatInt(*seedValue, 10),
		Temperature:              optionalFloatArgument(flagSet, "temperature", *temperatureValue),
		MaximumModelTier:         strings.TrimSpace(*maximumModelTier),
		RequiredExecutables:      devRequiredExecutables(strings.TrimSpace(*scenarioName), requiredExecutables.Values()),
		IsLiveLanguageModel:      *liveLanguageModel,
		HasStrictAssertions:      *strictAssertions,
		ShouldSkipPreflight:      *skipPreflight,
	}, nil
}

func runLocalDevVirtualSession(sessionArguments devVirtualSessionArguments) error {
	if errorValue := checkLocalDevDependencies(sessionArguments); errorValue != nil {
		return errorValue
	}
	invocation, errorValue := localDevVirtualSessionInvocation(sessionArguments)
	if errorValue != nil {
		return errorValue
	}
	command := exec.Command("go", invocation.Arguments...)
	command.Dir = invocation.WorkingDirectoryPath
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func localDevVirtualSessionInvocation(sessionArguments devVirtualSessionArguments) (devCommandInvocation, error) {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return devCommandInvocation{}, errorValue
	}
	sessionArguments.ArtifactDirectoryPath = resolveDevPath(repositoryRootPath, sessionArguments.ArtifactDirectoryPath)
	sessionArguments.ScenarioFilePath = resolveDevPath(repositoryRootPath, sessionArguments.ScenarioFilePath)
	sessionArguments.SkillDirectoryPath = resolveDevPath(repositoryRootPath, sessionArguments.SkillDirectoryPath)
	return devCommandInvocation{
		WorkingDirectoryPath: filepath.Join(repositoryRootPath, ".dependency", "blueclaw"),
		Arguments:            devVirtualSessionCommandArguments(sessionArguments),
	}, nil
}

func resolveDevPath(repositoryRootPath string, value string) string {
	normalizedValue := strings.TrimSpace(value)
	if normalizedValue == "" || filepath.IsAbs(normalizedValue) {
		return normalizedValue
	}
	return filepath.Join(repositoryRootPath, normalizedValue)
}

func devVirtualSessionCommandArguments(sessionArguments devVirtualSessionArguments) []string {
	commandArguments := []string{"run", "./cmd/blueclaw-lab", "virtual-session"}
	commandArguments = append(commandArguments, "--scenario", sessionArguments.ScenarioName)
	commandArguments = appendOptionalDevFlag(commandArguments, "--scenario-file", sessionArguments.ScenarioFilePath)
	commandArguments = append(commandArguments, "--artifact-dir", sessionArguments.ArtifactDirectoryPath)
	commandArguments = appendOptionalDevFlag(commandArguments, "--skill-dir", sessionArguments.SkillDirectoryPath)
	commandArguments = appendOptionalDevFlag(commandArguments, "--llm-endpoint", sessionArguments.LanguageModelEndpoint)
	commandArguments = appendOptionalDevFlag(commandArguments, "--llm-unix-socket", sessionArguments.LanguageModelSocket)
	commandArguments = appendOptionalDevFlag(commandArguments, "--llm-provider", sessionArguments.LanguageModelProvider)
	commandArguments = appendOptionalDevFlag(commandArguments, "--llm-auth-key-path", sessionArguments.LanguageModelAuthKeyPath)
	commandArguments = appendOptionalDevFlag(commandArguments, "--llm-model", sessionArguments.LanguageModelName)
	commandArguments = appendOptionalDevFlag(commandArguments, "--llm-execution-mode", sessionArguments.ExecutionMode)
	commandArguments = appendOptionalDevFlag(commandArguments, "--maximum-model-tier", sessionArguments.MaximumModelTier)
	if sessionArguments.HasStrictAssertions {
		commandArguments = append(commandArguments, "--strict-assertions")
	}
	if sessionArguments.IsLiveLanguageModel {
		commandArguments = appendOptionalDevFlag(commandArguments, "--seed", sessionArguments.Seed)
		commandArguments = appendOptionalDevFlag(commandArguments, "--temperature", sessionArguments.Temperature)
		commandArguments = append(commandArguments, "--live-llm")
	}
	return commandArguments
}

func appendOptionalDevFlag(arguments []string, name string, value string) []string {
	if strings.TrimSpace(value) == "" {
		return arguments
	}
	return append(arguments, name, value)
}

func checkLocalDevDependencies(sessionArguments devVirtualSessionArguments) error {
	if sessionArguments.ShouldSkipPreflight {
		return nil
	}
	missingExecutables := missingLocalExecutables(sessionArguments.RequiredExecutables)
	if len(missingExecutables) == 0 {
		return nil
	}
	return fmt.Errorf("dev simulate preflight failed; missing executable(s): %s", strings.Join(missingExecutables, ", "))
}

func missingLocalExecutables(executableNames []string) []string {
	missingExecutables := []string{}
	for _, executableName := range executableNames {
		if _, errorValue := exec.LookPath(executableName); errorValue != nil {
			missingExecutables = append(missingExecutables, executableName)
		}
	}
	return missingExecutables
}

func devRequiredExecutables(scenarioName string, explicitExecutables []string) []string {
	executableSet := map[string]bool{"go": true}
	for _, executableName := range scenarioExecutableDependencies(scenarioName) {
		executableSet[executableName] = true
	}
	for _, executableName := range explicitExecutables {
		trimmedExecutableName := strings.TrimSpace(executableName)
		if trimmedExecutableName != "" {
			executableSet[trimmedExecutableName] = true
		}
	}
	return sortedDevExecutableNames(executableSet)
}

func scenarioExecutableDependencies(scenarioName string) []string {
	normalizedScenarioName := strings.ToLower(strings.TrimSpace(scenarioName))
	switch normalizedScenarioName {
	case "slides", "slides_local_multiturn_success":
		return []string{"bun", "python3"}
	case "site", "site_artifact_acceptance", "site_prototype_acceptance", "site_edit_redeploy_acceptance", "site_lifecycle_acceptance":
		return []string{"bun"}
	default:
		return nil
	}
}

func sortedDevExecutableNames(executableSet map[string]bool) []string {
	executableNames := make([]string, 0, len(executableSet))
	for executableName := range executableSet {
		executableNames = append(executableNames, executableName)
	}
	sort.Strings(executableNames)
	return executableNames
}

func optionalFloatArgument(flagSet *flag.FlagSet, name string, value float64) string {
	if !flagWasPassed(flagSet, name) {
		return ""
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func flagWasPassed(flagSet *flag.FlagSet, name string) bool {
	isFound := false
	flagSet.Visit(func(flagValue *flag.Flag) {
		if flagValue.Name == name {
			isFound = true
		}
	})
	return isFound
}

func printDevUsage() {
	fmt.Println("Usage: internkim dev <simulate|fleet> [options]")
	fmt.Println("  internkim dev simulate --scenario dm_send_confirm_acceptance")
	fmt.Println("  internkim dev fleet run")
	fmt.Println("  internkim dev fleet run --scenario mattermost-direct-message-send")
	fmt.Println("  internkim dev fleet run --keep --scenario mattermost-manual")
	fmt.Println("  internkim dev fleet run --without-mattermost --scenario dm_send_confirm_acceptance")
	fmt.Println("  internkim dev fleet run --reuse --recipe predeploy-gate")
	fmt.Println("  internkim dev fleet run --scenario mattermost-bot-invited")
	fmt.Println("  internkim dev fleet verify-regression --base main --scenario regression-proof")
}

func printDevFleetUsage() {
	fmt.Println("Usage: internkim dev fleet <up|down|status|reset|run|reprovision|verify-regression>")
	fmt.Println("  internkim dev fleet reprovision [--config path]")
	fmt.Println("  internkim dev fleet run")
	fmt.Println("  internkim dev fleet run --scenario mattermost-direct-message-send")
	fmt.Println("  internkim dev fleet run --keep --scenario mattermost-manual")
	fmt.Println("  internkim dev fleet run --without-mattermost --scenario dm_send_confirm_acceptance")
	fmt.Println("  internkim dev fleet run --reuse --recipe predeploy-gate")
	fmt.Println("  internkim dev fleet run --scenario mattermost-bot-invited")
}

type repeatedDevStringFlag struct {
	values []string
}

func (flagValue *repeatedDevStringFlag) String() string {
	return strings.Join(flagValue.values, ",")
}

func (flagValue *repeatedDevStringFlag) Set(value string) error {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return nil
	}
	flagValue.values = append(flagValue.values, trimmedValue)
	return nil
}

func (flagValue repeatedDevStringFlag) Values() []string {
	return append([]string{}, flagValue.values...)
}
