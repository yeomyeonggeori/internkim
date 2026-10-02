package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type sshClient struct {
	user string
	pass string
	host string
	port string
}

func newSSH(user, pass, host string) *sshClient {
	return &sshClient{user: user, pass: pass, host: host, port: "22"}
}

func requireSSHPass() error {
	if _, errorValue := exec.LookPath("sshpass"); errorValue != nil {
		return errors.New("password SSH needs sshpass on PATH: brew install sshpass, or apt install sshpass")
	}
	return nil
}

func (s *sshClient) sshArgs(extra ...string) []string {
	base := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-o", "LogLevel=ERROR",
		"-p", s.port,
	}
	if s.pass != "" {
		base = append(base, "-o", "PreferredAuthentications=password", "-o", "PubkeyAuthentication=no")
	}
	return append(base, extra...)
}

func (s *sshClient) runInteractiveSSH(remoteArguments []string) error {
	target := fmt.Sprintf("%s@%s", s.user, s.host)
	commandName := "ssh"
	commandArguments := append(s.sshArgs(target), remoteArguments...)
	if s.pass != "" {
		if errorValue := requireSSHPass(); errorValue != nil {
			return errorValue
		}
		commandName = "sshpass"
		commandArguments = append([]string{"-p", s.pass, "ssh"}, append(s.sshArgs(target), remoteArguments...)...)
	}
	command := exec.Command(commandName, commandArguments...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func (s *sshClient) privilegedCommand(command string) string {
	if s.user == "root" {
		return command
	}
	if s.pass != "" {
		return "printf '%s\n' " + quoteShellValue(s.pass) + " | sudo -S -p '' bash -lc " + quoteShellValue(command)
	}
	return "sudo -p '' bash -lc " + quoteShellValue(command)
}

func quoteShellValue(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
