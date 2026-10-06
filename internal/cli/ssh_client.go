package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultHostSSHUser = "internkim"

const sshPasswordPromptVariable = "INTERNKIM_SSH_ASKPASS"

type hostSSH struct {
	hostname     string
	user         string
	password     string
	proxyCommand string
}

func hostSSHFromEnvironment(repositoryRootPath string) (hostSSH, error) {
	hostname := strings.TrimSpace(os.Getenv("INTERNKIM_SSH_HOSTNAME"))
	if hostname == "" {
		return hostSSH{}, errors.New("INTERNKIM_SSH_HOSTNAME names no host; run it as `internkim @host ssh`")
	}
	accessHelperPath := filepath.Join(repositoryRootPath, "tools", "cloudflared-access-ssh")
	return hostSSH{
		hostname:     hostname,
		user:         firstNonEmptyString(strings.TrimSpace(os.Getenv("INTERNKIM_SSH_USER")), defaultHostSSHUser),
		password:     os.Getenv("INTERNKIM_CONSOLE_PASSWORD"),
		proxyCommand: quoteShellValue(accessHelperPath) + " %h",
	}, nil
}

func (connection hostSSH) sshArguments(remoteArguments []string) []string {
	arguments := []string{"-o", "ProxyCommand=" + connection.proxyCommand, "-o", "StrictHostKeyChecking=accept-new"}
	if connection.password != "" {
		arguments = append(arguments, "-o", "PreferredAuthentications=password", "-o", "PubkeyAuthentication=no", "-o", "NumberOfPasswordPrompts=1")
	}
	arguments = append(arguments, connection.user+"@"+connection.hostname)
	return append(arguments, remoteArguments...)
}

func (connection hostSSH) command(remoteArguments []string) (*exec.Cmd, error) {
	command := exec.Command("ssh", connection.sshArguments(remoteArguments)...)
	if connection.password == "" {
		return command, nil
	}
	executablePath, errorValue := currentExecutablePath()
	if errorValue != nil {
		return nil, errorValue
	}
	command.Env = append(os.Environ(), "SSH_ASKPASS="+executablePath, "SSH_ASKPASS_REQUIRE=force", sshPasswordPromptVariable+"=1")
	return command, nil
}

func isAnsweringSSHPasswordPrompt() bool {
	return os.Getenv("INTERNKIM_SSH_ASKPASS") != ""
}

func answerSSHPasswordPrompt() {
	fmt.Println(os.Getenv("INTERNKIM_CONSOLE_PASSWORD"))
}

func (connection hostSSH) privilegedCommand(command string) string {
	if connection.password == "" {
		return "sudo -p '' bash -lc " + quoteShellValue(command)
	}
	passwordThenInput := "{ printf '%s\n' " + quoteShellValue(connection.password) + "; cat; }"
	return passwordThenInput + " | sudo -k -S -p '' bash -lc " + quoteShellValue(command)
}

func quoteShellValue(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
