package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/blueclawworkspace"
	"github.com/yeomyeonggeori/internkim/internal/localfleet"
)

type devVirtualSessionArguments struct {
	ScenarioName          string
	ScenarioFilePath      string
	ArtifactDirectoryPath string
	SkillDirectoryPath    string
	LanguageModelEndpoint string
	LanguageModelSocket   string
	LanguageModelProvider string
	LanguageModelName     string
	ExecutionMode         string
	Seed                  string
	Temperature           string
	MaximumModelTier      string
	RequiredExecutables   []string
	IsLiveLanguageModel   bool
	HasStrictAssertions   bool
	ShouldSkipPreflight   bool
}

type devCommandInvocation struct {
	WorkingDirectoryPath string
	Arguments            []string
	EnvironmentVariables []string
}

// Declared by blueclaw, which is a separate module this one cannot import.
const blueclawScenarioSkillRootsVariable = "BLUECLAW_SCENARIO_SKILL_ROOTS"

// Declared by blueclaw, which is a separate module this one cannot import.
const blueclawScenarioCapabilityCatalogVariable = "BLUECLAW_SCENARIO_CAPABILITY_CATALOG"

func scenarioCapabilityCatalogVariable(repositoryRootPath string) string {
	catalogPath := filepath.Join(repositoryRootPath, "pkg", "capabilityprotocol", "generated", "capability-tools.json")
	return blueclawScenarioCapabilityCatalogVariable + "=" + catalogPath
}

func scenarioSkillRootsVariable(repositoryRootPath string) (string, error) {
	skillRootPaths, errorValue := blueclawworkspace.SkillRootPaths(repositoryRootPath)
	if errorValue != nil {
		return "", errorValue
	}
	return blueclawScenarioSkillRootsVariable + "=" + strings.Join(skillRootPaths, string(os.PathListSeparator)), nil
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
	case "plane":
		return runDevPlaneArguments(commandArguments)
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
	case "run":
		if os.Getenv("LOCAL_PLANE_LOCK_HOLDER") == "" {
			return runHoldingTheLocalPlane()
		}
		return runDevFleetRunArguments(commandArguments)
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

func runHoldingTheLocalPlane() error {
	repositoryRootPath, errorValue := os.Getwd()
	if errorValue != nil {
		return errorValue
	}
	executablePath, errorValue := os.Executable()
	if errorValue != nil {
		return errorValue
	}
	command := holdingTheLocalPlane(repositoryRootPath, executablePath, os.Args[1:])
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
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
	scenario := flagSet.String("scenario", "", "Local fleet scenario to run")
	ephemeral := flagSet.Bool("ephemeral", false, "Deprecated; disposable local fleet runs are now the default")
	reuseFleet := flagSet.Bool("reuse", false, "Reuse the shared local fleet instead of creating a disposable run")
	keepArtifacts := flagSet.Bool("keep", false, "Keep disposable VM test artifacts; run evidence is kept by default")
	virtualSession := flagSet.Bool("virtual-session", false, "Run the scenario as a scripted Linux virtual session instead of the full local fleet")
	runID := flagSet.String("run-id", "", "Optional disposable run identifier")
	adminHostPort := flagSet.Int("admin-port", 0, "Host port for the local admind tunnel")
	useRealModels := flagSet.Bool("real", false, "Use production model configuration instead of the Local Fleet test model")
	flagSet.Usage = func() {
		fmt.Fprintln(flagSet.Output(), "Usage: internkim dev fleet run [flags]")
		flagSet.PrintDefaults()
		fmt.Fprintln(flagSet.Output(), "\nScenarios:")
		for _, name := range localfleet.ScenarioNames() {
			fmt.Fprintln(flagSet.Output(), "  "+name)
		}
	}
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return devFleetRunConfiguration{}, errorValue
	}
	trimmedScenario := strings.TrimSpace(*scenario)
	if len(flagSet.Args()) > 0 {
		return devFleetRunConfiguration{}, fmt.Errorf("unexpected arguments: %s", strings.Join(flagSet.Args(), " "))
	}
	if trimmedScenario == "" {
		return devFleetRunConfiguration{}, errors.New("dev fleet run requires --scenario")
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
		ShouldUseRealModels: *useRealModels,
	}
	request := localfleet.JobRequest{
		Action:         localfleet.ActionRunScenario,
		Scenario:       trimmedScenario,
		KeepArtifacts:  *keepArtifacts,
		VirtualSession: *virtualSession,
	}
	return devFleetRunConfiguration{ServiceOptions: serviceOptions, Request: request}, nil
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
	return localfleet.NewService(options)
}

func printLocalFleetStatus(service localfleet.Service) error {
	status := service.Status(context.Background())
	fmt.Printf("VM: %s %s\n", status.VirtualMachine.State, status.VirtualMachine.Message)
	fmt.Printf("SSH: %s %s\n", status.SSH.State, status.SSH.Message)
	fmt.Printf("Admin: %s %s\n", status.Admin.State, status.Admin.Message)
	if status.AdminURL != "" {
		fmt.Println("Admin URL: " + status.AdminURL)
	}
	if status.LastResult != "" {
		fmt.Println("Last result: " + status.LastResult)
	}
	return nil
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
	languageModelProvider := flagSet.String("llm-provider", "", "Live LLM provider: endpoint or capability")
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
	flagSet.Usage = func() {
		fmt.Fprintln(flagSet.Output(), "Usage: internkim dev simulate [flags]")
		flagSet.PrintDefaults()
		fmt.Fprintln(flagSet.Output(), "\nScenarios:")
		printBlueclawScenarioNames(flagSet.Output())
	}
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return devVirtualSessionArguments{}, errorValue
	}

	return devVirtualSessionArguments{
		ScenarioName:          strings.TrimSpace(*scenarioName),
		ScenarioFilePath:      strings.TrimSpace(*scenarioFilePath),
		ArtifactDirectoryPath: strings.TrimSpace(*artifactDirectoryPath),
		SkillDirectoryPath:    strings.TrimSpace(*skillDirectoryPath),
		LanguageModelEndpoint: strings.TrimSpace(*languageModelEndpoint),
		LanguageModelSocket:   strings.TrimSpace(*languageModelSocket),
		LanguageModelProvider: strings.TrimSpace(*languageModelProvider),
		LanguageModelName:     strings.TrimSpace(*languageModelName),
		ExecutionMode:         strings.TrimSpace(*executionMode),
		Seed:                  strconv.FormatInt(*seedValue, 10),
		Temperature:           optionalFloatArgument(flagSet, "temperature", *temperatureValue),
		MaximumModelTier:      strings.TrimSpace(*maximumModelTier),
		RequiredExecutables:   devRequiredExecutables(strings.TrimSpace(*scenarioName), requiredExecutables.Values()),
		IsLiveLanguageModel:   *liveLanguageModel,
		HasStrictAssertions:   *strictAssertions,
		ShouldSkipPreflight:   *skipPreflight,
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
	command.Env = append(os.Environ(), invocation.EnvironmentVariables...)
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
	skillRootsVariable, errorValue := scenarioSkillRootsVariable(repositoryRootPath)
	if errorValue != nil {
		return devCommandInvocation{}, errorValue
	}
	return devCommandInvocation{
		WorkingDirectoryPath: filepath.Join(repositoryRootPath, ".dependency", "blueclaw"),
		Arguments:            devVirtualSessionCommandArguments(sessionArguments),
		EnvironmentVariables: []string{skillRootsVariable, scenarioCapabilityCatalogVariable(repositoryRootPath)},
	}, nil
}

func printBlueclawScenarioNames(output io.Writer) {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		fmt.Fprintln(output, "  (scenario names unavailable: "+errorValue.Error()+")")
		return
	}
	command := exec.Command("go", "run", "./cmd/blueclaw-lab", "virtual-session", "--list-scenarios")
	command.Dir = filepath.Join(repositoryRootPath, ".dependency", "blueclaw")
	scenarioNamesOutput, errorValue := command.Output()
	if errorValue != nil {
		fmt.Fprintln(output, "  (scenario names unavailable: "+errorValue.Error()+")")
		return
	}
	for _, scenarioName := range strings.Fields(string(scenarioNamesOutput)) {
		fmt.Fprintln(output, "  "+scenarioName)
	}
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
	case "presentation", "presentation_local_multiturn_success":
		return []string{"bun", "python3"}
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

func holdingTheLocalPlane(repositoryRootPath string, commandPath string, arguments []string) *exec.Cmd {
	command := exec.Command(
		filepath.Join(repositoryRootPath, "tools", "with-local-plane"),
		append([]string{commandPath}, arguments...)...,
	)
	command.Dir = repositoryRootPath
	return command
}

func devPlaneServiceOptions(arguments []string) localfleet.Options {
	testArguments := arguments
	if len(testArguments) > 0 && testArguments[0] == "--" {
		testArguments = testArguments[1:]
	}
	return localfleet.Options{IsEphemeral: true, ScenarioArguments: testArguments}
}

func runDevPlaneArguments(arguments []string) error {
	if os.Getenv("LOCAL_PLANE_LOCK_HOLDER") == "" {
		return runHoldingTheLocalPlane()
	}
	service, errorValue := newLocalFleetServiceWithOptions(devPlaneServiceOptions(arguments))
	if errorValue != nil {
		return errorValue
	}
	contextValue, stop := interruptContext()
	defer stop()
	return service.Run(contextValue, standardLocalFleetLogger{}, localfleet.JobRequest{Action: localfleet.ActionRunCompanyPlane})
}

func printDevUsage() {
	fmt.Println("Usage: internkim dev <simulate|fleet|plane> [options]")
	fmt.Println("  internkim dev simulate --scenario dm_send_confirm_acceptance")
	fmt.Println("  internkim dev plane")
	fmt.Println("  internkim dev plane -t \"leaves on the messenger\"")
	fmt.Println("  internkim dev fleet run --scenario workspace-ownership")
}

func printDevFleetUsage() {
	fmt.Println("Usage: internkim dev fleet <down|status|run>")
	fmt.Println("  internkim dev fleet run --scenario workspace-ownership")
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
