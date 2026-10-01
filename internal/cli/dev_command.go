package cli

import (
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
	"github.com/yeomyeonggeori/internkim/internal/devplane"
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
	case "plane":
		return runDevPlaneArguments(commandArguments)
	case "help":
		printDevUsage()
		return nil
	default:
		return fmt.Errorf("unknown dev subcommand: %s", subcommand)
	}
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
	case "site_artifact_acceptance", "site_edit_redeploy_acceptance", "site_custom_structure_acceptance", "site_lifecycle_acceptance":
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

type standardDevPlaneLogger struct{}

func (logger standardDevPlaneLogger) Info(message string) {
	fmt.Println(message)
}

func holdingTheLocalPlane(repositoryRootPath string, commandPath string, arguments []string) *exec.Cmd {
	command := exec.Command(
		filepath.Join(repositoryRootPath, "tools", "with-local-plane"),
		append([]string{commandPath}, arguments...)...,
	)
	command.Dir = repositoryRootPath
	return command
}

func runDevPlaneArguments(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	executablePath, errorValue := currentExecutablePath()
	if errorValue != nil {
		return errorValue
	}
	if os.Getenv("LOCAL_PLANE_LOCK_HOLDER") == "" {
		command := holdingTheLocalPlane(repositoryRootPath, executablePath, append([]string{"dev", "plane"}, arguments...))
		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		return command.Run()
	}
	options := devplane.Options{RepositoryRootPath: repositoryRootPath, ExecutablePath: executablePath, TestArguments: arguments}
	service, errorValue := devplane.NewService(options)
	if errorValue != nil {
		return errorValue
	}
	contextValue, stop := interruptContext()
	defer stop()
	return service.Run(contextValue, standardDevPlaneLogger{})
}

func printDevUsage() {
	fmt.Println("Usage: internkim dev <simulate|plane> [options]")
	fmt.Println("  internkim dev simulate --scenario dm_send_confirm_acceptance")
	fmt.Println("  internkim dev plane")
	fmt.Println("  internkim dev plane -t \"leaves on the messenger\"")
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
