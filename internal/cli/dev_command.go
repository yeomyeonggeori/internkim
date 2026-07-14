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
)

type devVirtualSessionArguments struct {
	ScenarioName             string
	ArtifactDirectoryPath    string
	CassettePath             string
	RecordCassettePath       string
	SkillDirectoryPath       string
	LanguageModelEndpoint    string
	LanguageModelSocket      string
	LanguageModelProvider    string
	LanguageModelAuthKeyPath string
	LanguageModelName        string
	ExecutionMode            string
	TargetName               string
	Seed                     string
	Temperature              string
	RequiredExecutables      []string
	IsLiveLanguageModel      bool
	ShouldSkipPreflight      bool
}

type devCommandInvocation struct {
	WorkingDirectoryPath string
	Arguments            []string
}

var runDevLocalVirtualSession = runLocalDevVirtualSession
var runDevContainerVirtualSession = runContainerDevVirtualSession

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
	case "replay":
		return runDevReplayArguments(commandArguments)
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
		return runDevFleetReprovision()
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
func runDevFleetReprovision() error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	executablePath, errorValue := currentExecutablePath()
	if errorValue != nil {
		return errorValue
	}
	configurationPath, errorValue := latestLocalFleetConfigurationPath(repositoryRootPath)
	if errorValue != nil {
		return errorValue
	}
	vmInternetProtocolAddress, errorValue := localFleetVMInternetProtocolAddress(executablePath, configurationPath)
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("reprovisioning local fleet at %s from the working tree\n", vmInternetProtocolAddress)

	command := exec.Command(executablePath, "setup", "--board", "lab", "--ssh", "--host", vmInternetProtocolAddress,
		"--user", "admin", "--password", "admin",
		"--admin-email", "local-fleet-admin@internkim.test",
		"--force", "--skip", "wifi,local-llm,cloudflare-access,tunnel,google,slack,mattermost")
	command.Env = append(os.Environ(),
		"INTERNKIM_BLUECLAW_USE_LOCAL=1",
		"INTERNKIM_SKIP_PAGES_DEPLOY_FOR_LAB=1",
		"INTERNKIM_TEST_MODEL_TIER=xlow",
		"INTERNKIM_BLUECLAW_VCPU_COUNT=4")
	if moduleCachePath := goModuleCachePath(); moduleCachePath != "" {
		command.Env = append(command.Env, "GO_MOD_CACHE="+moduleCachePath)
	}
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func latestLocalFleetConfigurationPath(repositoryRootPath string) (string, error) {
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
	sessionArguments, errorValue := parseDevVirtualSessionArguments("simulate", arguments)
	if errorValue != nil {
		return errorValue
	}
	if sessionArguments.TargetName != "local" {
		return errors.New("dev simulate only supports --target local; use dev fleet run --without-mattermost --scenario <name> for Linux permission checks")
	}
	return runDevLocalVirtualSession(sessionArguments)
}

func runDevReplayArguments(arguments []string) error {
	sessionArguments, errorValue := parseDevVirtualSessionArguments("replay", arguments)
	if errorValue != nil {
		return errorValue
	}
	switch sessionArguments.TargetName {
	case "local":
		return runDevLocalVirtualSession(sessionArguments)
	case "container":
		return runDevContainerVirtualSession(sessionArguments)
	case "tart":
		return errors.New("the tart target was removed; use dev fleet run --without-mattermost --scenario <name>")
	default:
		return fmt.Errorf("unsupported dev replay target: %s", sessionArguments.TargetName)
	}
}

func parseDevVirtualSessionArguments(commandName string, arguments []string) (devVirtualSessionArguments, error) {
	flagSet := flag.NewFlagSet("dev "+commandName, flag.ContinueOnError)
	scenarioName := flagSet.String("scenario", "schedule_create_acceptance", "Blueclaw virtual-session scenario")
	artifactDirectoryPath := flagSet.String("artifact-dir", ".artifacts/blueclaw-dev", "Artifact directory for virtual-session output")
	cassettePath := flagSet.String("cassette", "", "Replay model responses from a cassette JSON file")
	recordCassettePath := flagSet.String("record-cassette", "", "Record model responses to a cassette JSON file")
	skillDirectoryPath := flagSet.String("skill-dir", "", "Skill directory to load into the virtual workspace")
	languageModelEndpoint := flagSet.String("llm-endpoint", "", "Live LLM capability endpoint")
	languageModelSocket := flagSet.String("llm-unix-socket", "", "Live LLM capability unix socket path")
	languageModelProvider := flagSet.String("llm-provider", "", "Live LLM provider: openrouter, capability, or sdkd")
	languageModelAuthKeyPath := flagSet.String("llm-auth-key-path", "", "SDKD installation auth key path")
	languageModelName := flagSet.String("llm-model", "", "Live LLM model override")
	executionMode := flagSet.String("llm-execution-mode", "", "Live LLM execution mode")
	targetName := flagSet.String("target", "local", "Replay target: local or container")
	liveLanguageModel := flagSet.Bool("live-llm", false, "Allow live LLM calls")
	skipPreflight := flagSet.Bool("skip-preflight", false, "Skip executable dependency checks")
	requiredExecutables := repeatedDevStringFlag{}
	flagSet.Var(&requiredExecutables, "require-executable", "Require an executable before running; repeat for multiple executables")
	seedValue := flagSet.Int64("seed", 41, "Generation seed for live LLM calls")
	temperatureValue := flagSet.Float64("temperature", 0, "Generation temperature for live LLM calls")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return devVirtualSessionArguments{}, errorValue
	}

	return devVirtualSessionArguments{
		ScenarioName:             strings.TrimSpace(*scenarioName),
		ArtifactDirectoryPath:    strings.TrimSpace(*artifactDirectoryPath),
		CassettePath:             strings.TrimSpace(*cassettePath),
		RecordCassettePath:       strings.TrimSpace(*recordCassettePath),
		SkillDirectoryPath:       strings.TrimSpace(*skillDirectoryPath),
		LanguageModelEndpoint:    strings.TrimSpace(*languageModelEndpoint),
		LanguageModelSocket:      strings.TrimSpace(*languageModelSocket),
		LanguageModelProvider:    strings.TrimSpace(*languageModelProvider),
		LanguageModelAuthKeyPath: strings.TrimSpace(*languageModelAuthKeyPath),
		LanguageModelName:        strings.TrimSpace(*languageModelName),
		ExecutionMode:            strings.TrimSpace(*executionMode),
		TargetName:               strings.TrimSpace(*targetName),
		Seed:                     strconv.FormatInt(*seedValue, 10),
		Temperature:              optionalFloatArgument(flagSet, "temperature", *temperatureValue),
		RequiredExecutables:      devRequiredExecutables(strings.TrimSpace(*scenarioName), requiredExecutables.Values()),
		IsLiveLanguageModel:      *liveLanguageModel,
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

func runContainerDevVirtualSession(sessionArguments devVirtualSessionArguments) error {
	invocation, errorValue := containerDevVirtualSessionInvocation(sessionArguments)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := runLabArguments([]string{"vm-up"}); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureContainerDevSharedWorkspace(invocation); errorValue != nil {
		return errorValue
	}
	if errorValue := checkContainerDevDependencies(sessionArguments); errorValue != nil {
		return errorValue
	}
	return runLabArguments([]string{"vm-ssh", devRemoteShellCommand(invocation)})
}

func localDevVirtualSessionInvocation(sessionArguments devVirtualSessionArguments) (devCommandInvocation, error) {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return devCommandInvocation{}, errorValue
	}
	return devCommandInvocation{
		WorkingDirectoryPath: filepath.Join(repositoryRootPath, ".dependency", "blueclaw"),
		Arguments:            devVirtualSessionCommandArguments(sessionArguments),
	}, nil
}

func containerDevVirtualSessionInvocation(sessionArguments devVirtualSessionArguments) (devCommandInvocation, error) {
	return devCommandInvocation{
		WorkingDirectoryPath: filepath.Join("/mnt/shared", "workspace", ".dependency", "blueclaw"),
		Arguments:            devVirtualSessionCommandArguments(sessionArguments),
	}, nil
}

func ensureContainerDevSharedWorkspace(invocation devCommandInvocation) error {
	command := containerDevSharedWorkspaceCommand(invocation.WorkingDirectoryPath)
	if errorValue := runLabArguments([]string{"vm-ssh", command}); errorValue != nil {
		return fmt.Errorf("dev container shared workspace check failed: %w", errorValue)
	}
	return nil
}

func containerDevSharedWorkspaceCommand(workingDirectoryPath string) string {
	return strings.Join([]string{
		"set -eu",
		"if [ ! -d " + quoteDevShellArgument(workingDirectoryPath) + " ]; then echo " + quoteDevShellArgument("missing shared workspace at "+workingDirectoryPath) + " >&2; exit 1; fi",
	}, "; ")
}

func devVirtualSessionCommandArguments(sessionArguments devVirtualSessionArguments) []string {
	commandArguments := []string{"run", "./cmd/blueclaw-lab", "virtual-session"}
	commandArguments = append(commandArguments, "--scenario", sessionArguments.ScenarioName)
	commandArguments = append(commandArguments, "--artifact-dir", sessionArguments.ArtifactDirectoryPath)
	commandArguments = appendOptionalDevFlag(commandArguments, "--cassette", sessionArguments.CassettePath)
	commandArguments = appendOptionalDevFlag(commandArguments, "--record-cassette", sessionArguments.RecordCassettePath)
	commandArguments = appendOptionalDevFlag(commandArguments, "--skill-dir", sessionArguments.SkillDirectoryPath)
	commandArguments = appendOptionalDevFlag(commandArguments, "--llm-endpoint", sessionArguments.LanguageModelEndpoint)
	commandArguments = appendOptionalDevFlag(commandArguments, "--llm-unix-socket", sessionArguments.LanguageModelSocket)
	commandArguments = appendOptionalDevFlag(commandArguments, "--llm-provider", sessionArguments.LanguageModelProvider)
	commandArguments = appendOptionalDevFlag(commandArguments, "--llm-auth-key-path", sessionArguments.LanguageModelAuthKeyPath)
	commandArguments = appendOptionalDevFlag(commandArguments, "--llm-model", sessionArguments.LanguageModelName)
	commandArguments = appendOptionalDevFlag(commandArguments, "--llm-execution-mode", sessionArguments.ExecutionMode)
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
	return fmt.Errorf("dev %s preflight failed; missing executable(s): %s", sessionArguments.TargetName, strings.Join(missingExecutables, ", "))
}

func checkContainerDevDependencies(sessionArguments devVirtualSessionArguments) error {
	if sessionArguments.ShouldSkipPreflight {
		return nil
	}
	command := "missing=''; for executable in " + quoteDevShellArguments(sessionArguments.RequiredExecutables) + "; do command -v \"$executable\" >/dev/null 2>&1 || missing=\"$missing $executable\"; done; if [ -n \"$missing\" ]; then echo \"missing executable(s):$missing\" >&2; exit 127; fi"
	if errorValue := runLabArguments([]string{"vm-ssh", command}); errorValue != nil {
		return fmt.Errorf("dev container preflight failed: %w", errorValue)
	}
	return nil
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

func devRemoteShellCommand(invocation devCommandInvocation) string {
	arguments := append([]string{"go"}, invocation.Arguments...)
	return "cd " + quoteDevShellArgument(invocation.WorkingDirectoryPath) + " && " + quoteDevShellArguments(arguments)
}

func quoteDevShellArguments(arguments []string) string {
	quotedArguments := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		quotedArguments = append(quotedArguments, quoteDevShellArgument(argument))
	}
	return strings.Join(quotedArguments, " ")
}

func quoteDevShellArgument(argument string) string {
	return "'" + strings.ReplaceAll(argument, "'", "'\"'\"'") + "'"
}

func printDevUsage() {
	fmt.Println("Usage: internkim dev <simulate|fleet> [options]")
	fmt.Println("  internkim dev simulate --scenario dm_send_confirm_acceptance")
	fmt.Println("  internkim dev fleet run")
	fmt.Println("  internkim dev fleet run --scenario mattermost-direct-message-send")
	fmt.Println("  internkim dev fleet run --without-mattermost --scenario dm_send_confirm_acceptance")
	fmt.Println("  internkim dev fleet run --reuse --recipe predeploy-gate")
	fmt.Println("  internkim dev fleet run --scenario mattermost-bot-invited")
	fmt.Println("  internkim dev fleet verify-regression --base main --scenario regression-proof")
}

func printDevFleetUsage() {
	fmt.Println("Usage: internkim dev fleet <up|down|status|reset|run|reprovision|verify-regression>")
	fmt.Println("  internkim dev fleet reprovision")
	fmt.Println("  internkim dev fleet run")
	fmt.Println("  internkim dev fleet run --scenario mattermost-direct-message-send")
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
