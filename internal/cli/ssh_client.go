package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultHostSSHUser = "internkim"

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
		arguments = append(arguments, "-o", "PreferredAuthentications=password", "-o", "PubkeyAuthentication=no")
	}
	arguments = append(arguments, connection.user+"@"+connection.hostname)
	return append(arguments, remoteArguments...)
}

func (connection hostSSH) command(remoteArguments []string) (*exec.Cmd, error) {
	if connection.password == "" {
		return exec.Command("ssh", connection.sshArguments(remoteArguments)...), nil
	}
	if _, errorValue := exec.LookPath("sshpass"); errorValue != nil {
		return nil, errors.New("password SSH needs sshpass on PATH: brew install sshpass, or apt install sshpass")
	}
	command := exec.Command("sshpass", append([]string{"-e", "ssh"}, connection.sshArguments(remoteArguments)...)...)
	command.Env = append(os.Environ(), "SSHPASS="+connection.password)
	return command, nil
}

func (connection hostSSH) privilegedCommand(command string) string {
	if connection.password == "" {
		return "sudo -p '' bash -lc " + quoteShellValue(command)
	}
	return "printf '%s\n' " + quoteShellValue(connection.password) + " | sudo -S -p '' bash -lc " + quoteShellValue(command)
}

func quoteShellValue(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
