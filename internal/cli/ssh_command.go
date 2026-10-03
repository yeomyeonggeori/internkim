package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

func runHostSSH() {
	controlArguments, remoteArguments := splitAtDoubleDash(os.Args[2:])
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	connection, errorValue := hostSSHFromEnvironment(repositoryRootPath)
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	if slices.Contains(controlArguments, "--sudo") {
		if len(remoteArguments) == 0 {
			fatal("--sudo needs a command after --")
		}
		remoteArguments = []string{connection.privilegedCommand(strings.Join(remoteArguments, " "))}
	}
	fmt.Printf("SSH: %s@%s\n", connection.user, connection.hostname)
	command, errorValue := connection.command(remoteArguments)
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if errorValue := command.Run(); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func splitAtDoubleDash(arguments []string) ([]string, []string) {
	index := slices.Index(arguments, "--")
	if index < 0 {
		return arguments, nil
	}
	return arguments[:index], arguments[index+1:]
}
