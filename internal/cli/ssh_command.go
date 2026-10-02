package cli

import (
	"fmt"
	"os"
	"strings"
)

func runDeviceSSH() {
	arguments := commandControlArguments(os.Args[2:])
	shouldElevate := hasControlFlag(arguments, "--sudo")
	target := resolveCommandTarget(withoutControlFlag(arguments, "--sudo"))
	connection := target.sshConnection()
	fmt.Printf("SSH: %s@%s\n", connection.user, connection.host)
	remoteArguments := commandRemoteArguments(os.Args[2:])
	if shouldElevate {
		if len(remoteArguments) == 0 {
			fatal("--sudo needs a command after --")
		}
		remoteArguments = []string{connection.privilegedCommand(strings.Join(remoteArguments, " "))}
	}
	if errorValue := connection.runInteractiveSSH(remoteArguments); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func withoutControlFlag(arguments []string, name string) []string {
	remaining := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		if argument == name {
			continue
		}
		remaining = append(remaining, argument)
	}
	return remaining
}

func hasControlFlag(arguments []string, name string) bool {
	for _, argument := range arguments {
		if argument == name {
			return true
		}
	}
	return false
}

func commandControlArguments(arguments []string) []string {
	for index, argument := range arguments {
		if argument == "--" {
			return arguments[:index]
		}
	}
	return arguments
}

func commandRemoteArguments(arguments []string) []string {
	for index, argument := range arguments {
		if argument == "--" {
			return arguments[index+1:]
		}
	}
	return nil
}
