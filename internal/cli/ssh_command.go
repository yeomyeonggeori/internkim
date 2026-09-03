package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runDeviceSSH() {
	configuration := loadConfig()
	scriptDir, _ := os.Getwd()
	sshpassBin := filepath.Join(scriptDir, "bin", "sshpass")
	arguments := commandControlArguments(os.Args[2:])
	shouldElevate := hasControlFlag(arguments, "--sudo")
	target := resolveCommandTarget(withoutControlFlag(arguments, "--sudo"))
	connection, isRemote, errorValue := resolveDeviceSSHConnection(configuration, sshpassBin, target)
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

func resolveDeviceSSHConnection(configuration config, sshpassBin string, target commandTarget) (*sshClient, bool, error) {
	if target.useRemoteSSH {
		return resolveRemoteSSHConnection(configuration, sshpassBin, target, true)
	}
	if connection := resolveLocalSSHConnection(sshpassBin, target); connection != nil {
		return connection, false, nil
	}
	return resolveRemoteSSHConnection(configuration, sshpassBin, target, false)
}

func resolveLocalSSHConnection(sshpassBin string, target commandTarget) *sshClient {
	if strings.TrimSpace(target.host) != "" {
		connection := newSSH(sshpassBin, target.sshUser, target.sshPassword, target.host)
		if _, errorValue := connection.runResult("true"); errorValue == nil {
			return connection
		}
		return nil
	}
	host := findBoardIPForCredentials(sshpassBin, target.stateDir, target.sshUser, target.sshPassword)
	if host == "" {
		return nil
	}
	return newSSH(sshpassBin, target.sshUser, target.sshPassword, host)
}

func resolveRemoteSSHConnection(configuration config, sshpassBin string, target commandTarget, isRequired bool) (*sshClient, bool, error) {
	target.sshHostname = savedRemoteSSHHostname(target)
	if target.sshHostname == "" {
		return nil, false, errors.New("device is not reachable locally and no ssh hostname is known; run setup once on the device network first")
	}
	connection := newSSH(sshpassBin, target.sshUser, target.sshPassword, target.sshHostname)
	if output, errorValue := connection.runResult("true"); errorValue != nil {
		return nil, false, remoteSSHError(target.sshHostname, output, errorValue)
	}
	return connection, true, nil
}

func remoteSSHError(hostname string, output string, errorValue error) error {
	detail := strings.TrimSpace(output)
	if detail == "" {
		return fmt.Errorf("ssh to %s failed: %w%s", hostname, errorValue, missingAccessTokenHint(hostname))
	}
	return fmt.Errorf("ssh to %s failed: %s: %w%s", hostname, detail, errorValue, missingAccessTokenHint(hostname))
}

func missingAccessTokenHint(hostname string) string {
	if hasCloudflareAccessToken(hostname) {
		return ""
	}
	return fmt.Sprintf("\n  이 호스트의 Cloudflare Access 토큰이 ~/.cloudflared 에 없습니다."+
		" 토큰이 없으면 cloudflared가 브라우저 인증을 기다리며 멈추고, 스트림이 열리지 않아"+
		" 요청이 장비까지 가지 않습니다 — 장비가 아니라 이 컴퓨터의 문제입니다."+
		"\n  cloudflared access login https://%s", hostname)
}

func hasCloudflareAccessToken(hostname string) bool {
	homeDirectory, errorValue := os.UserHomeDir()
	if errorValue != nil {
		return true
	}
	tokenPaths, errorValue := filepath.Glob(filepath.Join(homeDirectory, ".cloudflared", hostname+"-*-token"))
	if errorValue != nil {
		return true
	}
	return len(tokenPaths) > 0
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
