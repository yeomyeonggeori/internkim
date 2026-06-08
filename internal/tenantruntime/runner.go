package tenantruntime

import (
	"context"
	"os"
	"os/exec"
)

type CommandRunner interface {
	Run(context.Context, ExecutableCommand) (string, error)
}

type ExecutableCommand struct {
	ExecutableName string
	Arguments      []string
}

type OperatingSystemCommandRunner struct{}

func (operatingSystemCommandRunner OperatingSystemCommandRunner) Run(
	ctx context.Context,
	executableCommand ExecutableCommand,
) (string, error) {
	command := exec.CommandContext(ctx, executableCommand.ExecutableName, executableCommand.Arguments...)
	command.Stdin = os.Stdin
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return string(output), errorValue
	}
	return string(output), nil
}
