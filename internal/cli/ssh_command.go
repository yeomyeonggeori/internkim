package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func runDeviceSSH() {
	configuration := loadConfig()
	arguments := commandControlArguments(os.Args[2:])
	shouldElevate := hasControlFlag(arguments, "--sudo")
	target := resolveCommandTarget(withoutControlFlag(arguments, "--sudo"))
	connection, isRemote, errorValue := resolveDeviceSSHConnection(configuration, target)
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	target.host = connection.host
	target.useRemoteSSH = isRemote
	printCommandTargetEvidence(target)
	if isRemote {
		fmt.Printf("Backend: remote ssh\n")
	} else {
		fmt.Printf("Backend: local ssh\n")
	}
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

func resolveDeviceSSHConnection(configuration config, target commandTarget) (*sshClient, bool, error) {
	if target.useRemoteSSH {
		return resolveRemoteSSHConnection(configuration, target, true)
	}
	if connection := resolveLocalSSHConnection(target); connection != nil {
		return connection, false, nil
	}
	return resolveRemoteSSHConnection(configuration, target, false)
}

func resolveLocalSSHConnection(target commandTarget) *sshClient {
	if strings.TrimSpace(target.host) != "" {
		connection := newSSH(target.sshUser, target.sshPassword, target.host)
		if _, errorValue := connection.runResult("true"); errorValue == nil {
			return connection
		}
		return nil
	}
	host := findBoardIPForCredentials(target.stateDir, target.sshUser, target.sshPassword)
	if host == "" {
		return nil
	}
	return newSSH(target.sshUser, target.sshPassword, host)
}

func resolveRemoteSSHConnection(configuration config, target commandTarget, isRequired bool) (*sshClient, bool, error) {
	target.sshHostname = savedRemoteSSHHostname(target)
	if target.sshHostname == "" {
		return nil, false, errors.New("device is not reachable locally and no ssh hostname is known; run setup once on the device network first")
	}
	connection := newSSH(target.sshUser, target.sshPassword, target.sshHostname)
	if output, errorValue := connection.runResult("true"); errorValue != nil {
		return nil, false, remoteSSHError(target.sshHostname, output, errorValue)
	}
	return connection, true, nil
}

func remoteSSHError(hostname string, output string, errorValue error) error {
	detail := strings.TrimSpace(output)
	if detail == "" {
		return fmt.Errorf("ssh to %s failed: %w", hostname, errorValue)
	}
	return fmt.Errorf("ssh to %s failed: %s: %w", hostname, detail, errorValue)
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
