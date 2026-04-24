package lab

import "strings"

type ExecutableCommand struct {
	ExecutableName       string
	Arguments            []string
	WorkingDirectoryPath string
	EnvironmentVariables map[string]string
	StandardInputPath    string
}

func (executableCommand ExecutableCommand) String() string {
	return strings.TrimSpace(executableCommand.ExecutableName + " " + strings.Join(executableCommand.Arguments, " "))
}
