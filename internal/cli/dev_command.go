package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type devVirtualSessionArguments struct {
	ScenarioName          string
	ArtifactDirectoryPath string
	CassettePath          string
	RecordCassettePath    string
	SkillDirectoryPath    string
	LanguageModelEndpoint string
	LanguageModelSocket   string
	LanguageModelName     string
	ExecutionMode         string
	TargetName            string
	Seed                  string
	Temperature           string
	RequiredExecutables   []string
	IsLiveLanguageModel   bool
	ShouldSkipPreflight   bool
}

type devCommandInvocation struct {
	WorkingDirectoryPath string
	Arguments            []string
}

var runDevLocalVirtualSession = runLocalDevVirtualSession
var runDevTartVirtualSession = runTartDevVirtualSession

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
	case "help":
		printDevUsage()
		return nil
	default:
		return fmt.Errorf("unknown dev subcommand: %s", subcommand)
	}
}

func runDevSimulateArguments(arguments []string) error {
	sessionArguments, errorValue := parseDevVirtualSessionArguments("simulate", arguments)
	if errorValue != nil {
		return errorValue
	}
	if sessionArguments.TargetName != "local" {
		return errors.New("dev simulate only supports --target local; use dev replay --target tart for Linux permission checks")
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
	case "tart":
		return runDevTartVirtualSession(sessionArguments)
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
	languageModelName := flagSet.String("llm-model", "", "Live LLM model name")
	executionMode := flagSet.String("llm-execution-mode", "", "Live LLM execution mode")
	targetName := flagSet.String("target", "local", "Replay target: local or tart")
	liveLanguageModel := flagSet.Bool("live-llm", false, "Allow live LLM calls")
	skipPreflight := flagSet.Bool("skip-preflight", false, "Skip executable dependency checks")
	requiredExecutables := repeatedDevStringFlag{}
	flagSet.Var(&requiredExecutables, "require-executable", "Require an executable before running; repeat for multiple executables")
	seedValue := flagSet.Int64("seed", 0, "Generation seed for live LLM calls")
	temperatureValue := flagSet.Float64("temperature", 0, "Generation temperature for live LLM calls")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return devVirtualSessionArguments{}, errorValue
	}

	return devVirtualSessionArguments{
		ScenarioName:          strings.TrimSpace(*scenarioName),
		ArtifactDirectoryPath: strings.TrimSpace(*artifactDirectoryPath),
		CassettePath:          strings.TrimSpace(*cassettePath),
		RecordCassettePath:    strings.TrimSpace(*recordCassettePath),
		SkillDirectoryPath:    strings.TrimSpace(*skillDirectoryPath),
		LanguageModelEndpoint: strings.TrimSpace(*languageModelEndpoint),
		LanguageModelSocket:   strings.TrimSpace(*languageModelSocket),
		LanguageModelName:     strings.TrimSpace(*languageModelName),
		ExecutionMode:         strings.TrimSpace(*executionMode),
		TargetName:            strings.TrimSpace(*targetName),
		Seed:                  optionalIntegerArgument(flagSet, "seed", *seedValue),
		Temperature:           optionalFloatArgument(flagSet, "temperature", *temperatureValue),
		RequiredExecutables:   devRequiredExecutables(strings.TrimSpace(*scenarioName), requiredExecutables.Values()),
		IsLiveLanguageModel:   *liveLanguageModel,
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
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func runTartDevVirtualSession(sessionArguments devVirtualSessionArguments) error {
	invocation, errorValue := tartDevVirtualSessionInvocation(sessionArguments)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := runLabArguments([]string{"vm-up"}); errorValue != nil {
		return errorValue
	}
	if errorValue := checkTartDevDependencies(sessionArguments); errorValue != nil {
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

func tartDevVirtualSessionInvocation(sessionArguments devVirtualSessionArguments) (devCommandInvocation, error) {
	return devCommandInvocation{
		WorkingDirectoryPath: "/mnt/shared/.dependency/blueclaw",
		Arguments:            devVirtualSessionCommandArguments(sessionArguments),
	}, nil
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
	commandArguments = appendOptionalDevFlag(commandArguments, "--llm-model", sessionArguments.LanguageModelName)
	commandArguments = appendOptionalDevFlag(commandArguments, "--llm-execution-mode", sessionArguments.ExecutionMode)
	commandArguments = appendOptionalDevFlag(commandArguments, "--seed", sessionArguments.Seed)
	commandArguments = appendOptionalDevFlag(commandArguments, "--temperature", sessionArguments.Temperature)
	if sessionArguments.IsLiveLanguageModel {
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

func checkTartDevDependencies(sessionArguments devVirtualSessionArguments) error {
	if sessionArguments.ShouldSkipPreflight {
		return nil
	}
	command := "missing=''; for executable in " + quoteDevShellArguments(sessionArguments.RequiredExecutables) + "; do command -v \"$executable\" >/dev/null 2>&1 || missing=\"$missing $executable\"; done; if [ -n \"$missing\" ]; then echo \"missing executable(s):$missing\" >&2; exit 127; fi"
	if errorValue := runLabArguments([]string{"vm-ssh", command}); errorValue != nil {
		return fmt.Errorf("dev tart preflight failed: %w", errorValue)
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
	case "site", "site_prototype_acceptance":
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

func optionalIntegerArgument(flagSet *flag.FlagSet, name string, value int64) string {
	if !flagWasPassed(flagSet, name) {
		return ""
	}
	return strconv.FormatInt(value, 10)
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
	fmt.Println("Usage: internkim dev <simulate|replay> [--scenario name] [--cassette path] [--target local|tart]")
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
