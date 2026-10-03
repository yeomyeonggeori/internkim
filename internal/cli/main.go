package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func currentExecutablePath() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolvedPath, err := filepath.EvalSymlinks(executablePath); err == nil {
		executablePath = resolvedPath
	}
	return executablePath, nil
}

func resolveRepositoryRootPath() (string, error) {
	workingDirectoryPath, errorValue := os.Getwd()
	if errorValue != nil {
		return "", errorValue
	}

	searchPath := workingDirectoryPath
	for {
		if _, errorValue := os.Stat(filepath.Join(searchPath, "go.mod")); errorValue == nil {
			return searchPath, nil
		}
		parentPath := filepath.Dir(searchPath)
		if parentPath == searchPath {
			return "", errors.New("could not find repository root")
		}
		searchPath = parentPath
	}
}

func Main() {
	if isAnsweringSSHPasswordPrompt() {
		answerSSHPasswordPrompt()
		return
	}
	requestedProfile, arguments := splitVaultProfileArgument(os.Args[1:])
	os.Args = append(os.Args[:1], arguments...)
	reExecuteWithVaultEnvironment(requestedProfile)
	if len(os.Args) < 2 {
		printUsage()
		return
	}
	runNamedCommand(os.Args[1])
}

func runNamedCommand(name string) {
	switch name {
	case "--help", "-h":
		printUsage()
	case "ssh":
		runHostSSH()
	case "release":
		runRelease()
	case "doctor":
		runDoctor()
	case "verify":
		runVerify()
	case "test":
		runTest()
	case "dev":
		runDev()
	case "lab":
		runLab()
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: internkim <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  ssh      Open SSH to the company host through Cloudflare Access")
	fmt.Println("  release  Publish and inspect release sets")
	fmt.Println("  doctor   Check host dependencies")
	fmt.Println("  verify   Run API and browser verification")
	fmt.Println("  test     Run a prompt through disposable Local Fleet; use -o <file> for one returned attachment")
	fmt.Println("  lab      Run container-based Blueclaw-aligned lab workflows")
}
