package lab

import (
	"context"
	"os"
	"os/exec"
)

type CommandRunner interface {
	Run(context.Context, ExecutableCommand) error
	Start(context.Context, ExecutableCommand) error
	Output(context.Context, ExecutableCommand) (string, error)
}

type OperatingSystemCommandRunner struct{}

func (operatingSystemCommandRunner OperatingSystemCommandRunner) Run(
	ctx context.Context,
	executableCommand ExecutableCommand,
) error {
	command, standardInputFile, errorValue := createCommand(ctx, executableCommand)
	if errorValue != nil {
		return errorValue
	}
	if standardInputFile != nil {
		defer standardInputFile.Close()
	}

	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func (operatingSystemCommandRunner OperatingSystemCommandRunner) Start(
	ctx context.Context,
	executableCommand ExecutableCommand,
) error {
	if errorValue := ctx.Err(); errorValue != nil {
		return errorValue
	}

	command, standardInputFile, errorValue := createCommand(context.Background(), executableCommand)
	if errorValue != nil {
		return errorValue
	}
	if standardInputFile != nil {
		defer standardInputFile.Close()
	}

	logFile, errorValue := openDetachedLogFile(executableCommand.DetachedLogPath)
	if errorValue != nil {
		return errorValue
	}
	defer logFile.Close()

	nullInput, errorValue := os.Open(os.DevNull)
	if errorValue != nil {
		return errorValue
	}
	defer nullInput.Close()

	command.Stdin = nullInput
	command.Stdout = logFile
	command.Stderr = logFile
	detachCommand(command)
	errorValue = command.Start()
	if errorValue != nil {
		return errorValue
	}

	return command.Process.Release()
}

func openDetachedLogFile(path string) (*os.File, error) {
	if path == "" {
		return os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

func (operatingSystemCommandRunner OperatingSystemCommandRunner) Output(
	ctx context.Context,
	executableCommand ExecutableCommand,
) (string, error) {
	command, standardInputFile, errorValue := createCommand(ctx, executableCommand)
	if errorValue != nil {
		return "", errorValue
	}
	if standardInputFile != nil {
		defer standardInputFile.Close()
	}

	output, errorValue := command.Output()
	if errorValue != nil {
		return "", errorValue
	}

	return string(output), nil
}

func createCommand(
	ctx context.Context,
	executableCommand ExecutableCommand,
) (*exec.Cmd, *os.File, error) {
	command := exec.CommandContext(ctx, executableCommand.ExecutableName, executableCommand.Arguments...)
	command.Dir = executableCommand.WorkingDirectoryPath
	command.Env = mergeEnvironmentVariables(os.Environ(), executableCommand.EnvironmentVariables)
	command.Stdin = os.Stdin

	if executableCommand.StandardInputPath == "" {
		return command, nil, nil
	}

	standardInputFile, errorValue := os.Open(executableCommand.StandardInputPath)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	command.Stdin = standardInputFile

	return command, standardInputFile, nil
}

func mergeEnvironmentVariables(baseEnvironmentVariables []string, extraEnvironmentVariables map[string]string) []string {
	mergedEnvironmentVariables := append([]string{}, baseEnvironmentVariables...)
	for key, value := range extraEnvironmentVariables {
		mergedEnvironmentVariables = append(mergedEnvironmentVariables, key+"="+value)
	}

	return mergedEnvironmentVariables
}
