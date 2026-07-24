package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type updateCommandRunner func(directoryPath string, name string, arguments ...string) error

var runUpdateCommand updateCommandRunner = runStreamingUpdateCommand
var resolveUpdateExecutablePath = currentExecutablePath
var runUpdateSimulationGate = runSimGateArguments
var stopUpdateSimulation = stopSimulationBeforePhysicalDeploy

func runUpdateArguments(arguments []string) error {
	if hasCommandArgument(arguments, "--help") || hasCommandArgument(arguments, "-h") {
		printUpdateUsage()
		return nil
	}
	if len(arguments) > 0 {
		switch arguments[0] {
		case "check":
			return runReleaseUpdateCheck(arguments[1:])
		case "apply":
			return runReleaseUpdateApply(arguments[1:])
		case "blueclaw-payload":
			return runBlueclawPayloadHTTPUpdate(arguments[1:])
		}
	}
	setupSlice, errorValue := updateSetupSlice(arguments)
	if errorValue != nil {
		return errorValue
	}
	isPlan := hasCommandArgument(arguments, "--plan")
	if hasCommandArgument(arguments, "--sim-first") {
		if errorValue := runUpdateSimulationGate(updateSimulationGateArguments(arguments)); errorValue != nil {
			return errorValue
		}
		if !isPlan {
			if errorValue := stopUpdateSimulation(); errorValue != nil {
				return errorValue
			}
		}
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	if !isPlan && !hasCommandArgument(arguments, "--sim-first") {
		if errorValue := runUpdateCommand(repositoryRootPath, "make", "build"); errorValue != nil {
			return errorValue
		}
	}
	executablePath, errorValue := resolveUpdateExecutablePath()
	if errorValue != nil {
		return errorValue
	}
	setupArguments := updateSetupArguments(setupSlice, arguments)
	return runUpdateCommand(repositoryRootPath, executablePath, setupArguments...)
}

func updateSetupSlice(arguments []string) (string, error) {
	selectedModes := 0
	for _, flag := range []string{"--all", "--web", "--binaries"} {
		if hasCommandArgument(arguments, flag) {
			selectedModes++
		}
	}
	if selectedModes > 1 {
		return "", errors.New("choose only one of --all, --web, or --binaries")
	}
	if hasCommandArgument(arguments, "--web") {
		return "web", nil
	}
	if hasCommandArgument(arguments, "--binaries") {
		return "binaries,services", nil
	}
	return "web,binaries,services", nil
}

func updateSetupArguments(setupSlice string, arguments []string) []string {
	setupArguments := []string{"setup", "--only", setupSlice, "--force"}
	for _, argument := range arguments {
		if updateSpecificArgument(argument) {
			continue
		}
		setupArguments = append(setupArguments, argument)
	}
	return setupArguments
}

func updateSpecificArgument(argument string) bool {
	switch argument {
	case "--all", "--web", "--binaries", "--sim-first", "--help", "-h":
		return true
	default:
		return strings.HasPrefix(argument, "--all=") || strings.HasPrefix(argument, "--web=") || strings.HasPrefix(argument, "--binaries=") || strings.HasPrefix(argument, "--sim-first=")
	}
}

func updateSimulationGateArguments(arguments []string) []string {
	gateArguments := []string{}
	if hasCommandArgument(arguments, "--plan") {
		gateArguments = append(gateArguments, "--plan")
	}
	return gateArguments
}

func runStreamingUpdateCommand(directoryPath string, name string, arguments ...string) error {
	command := exec.Command(name, arguments...)
	command.Dir = directoryPath
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	if errorValue := command.Run(); errorValue != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(arguments, " "), errorValue)
	}
	return nil
}

func printUpdateUsage() {
	fmt.Println("Usage: internkim update [check|apply] [--all|--web|--binaries] [--sim-first] [--plan] [target options]")
	fmt.Println("Examples:")
	fmt.Println("  internkim update")
	fmt.Println("  internkim update check --node 1")
	fmt.Println("  internkim update apply --node 1")
	fmt.Println("  internkim update blueclaw-payload")
	fmt.Println("  internkim update --web")
	fmt.Println("  internkim update --binaries --host 192.168.1.50")
	fmt.Println("  internkim update --profile acme --node 1")
	fmt.Println("  internkim update --sim-first")
	fmt.Println("  internkim update --plan")
}

func runBlueclawPayloadHTTPUpdate(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	target := resolveCommandTarget(arguments)
	target = resolveLabHostForCommandTarget(target, repositoryRootPath)
	state := newSetupFlowState(
		newMsg("ko"),
		loadConfig(),
		collectSetupParameterValues(),
		target.stateDir,
		repositoryRootPath,
		currentExecutableFingerprint(),
		nil,
		true,
	)
	artifactDirectoryPath, errorValue := state.ensureBlueclawPayloadArtifact()
	if errorValue != nil {
		return errorValue
	}
	manifest, errorValue := blueclaw.ValidatePayloadArtifactDirectory(artifactDirectoryPath)
	if errorValue != nil {
		return errorValue
	}
	printCommandTargetEvidence(target)
	fmt.Print("  blueclaw runtime payload via HTTPS... ")
	outcome, errorValue := state.installBlueclawPayloadHTTPS(artifactDirectoryPath, manifest)
	if errorValue != nil {
		fmt.Println("failed")
		return errorValue
	}
	return state.reportBlueclawPayloadHTTPSOutcome(outcome)
}
