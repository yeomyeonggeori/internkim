package cli

import (
	"strings"
	"testing"
)

func TestSSHReachesTheHostThroughTheAccessHelper(t *testing.T) {
	t.Setenv("INTERNKIM_SSH_HOSTNAME", "ssh.example.test")
	t.Setenv("INTERNKIM_SSH_USER", "")
	t.Setenv("INTERNKIM_CONSOLE_PASSWORD", "")
	connection, errorValue := hostSSHFromEnvironment("/checkout")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	arguments := strings.Join(connection.sshArguments([]string{"uptime"}), " ")
	if !strings.Contains(arguments, "ProxyCommand='/checkout/tools/cloudflared-access-ssh' %h") {
		t.Errorf("ssh does not go through the Access helper: %s", arguments)
	}
	if !strings.HasSuffix(arguments, "internkim@ssh.example.test uptime") {
		t.Errorf("ssh does not log in as the default user on the named host: %s", arguments)
	}
}

func TestSSHRefusesWithoutAHostname(t *testing.T) {
	t.Setenv("INTERNKIM_SSH_HOSTNAME", "")
	if _, errorValue := hostSSHFromEnvironment("/checkout"); errorValue == nil {
		t.Fatal("ssh started without a host to reach")
	}
}

func TestSSHPinsPasswordAuthenticationWhenGivenAPassword(t *testing.T) {
	connection := hostSSH{user: "admin", hostname: "ssh.example.test", password: "secret"}
	arguments := strings.Join(connection.sshArguments(nil), " ")
	if !strings.Contains(arguments, "PreferredAuthentications=password") ||
		!strings.Contains(arguments, "PubkeyAuthentication=no") {
		t.Errorf("ssh offers keys before the password it was given: %s", arguments)
	}
	if strings.Contains(arguments, "secret") {
		t.Errorf("the password is on the ssh command line: %s", arguments)
	}
}

func TestSSHAsksForThePasswordThroughAskpassInsteadOfTypingIt(t *testing.T) {
	connection := hostSSH{user: "admin", hostname: "ssh.example.test", password: "secret"}
	command, errorValue := connection.command([]string{"true"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.HasSuffix(command.Path, "ssh") || strings.Contains(command.Path, "sshpass") {
		t.Errorf("the password is typed by %s instead of asked for by ssh", command.Path)
	}
	environment := strings.Join(command.Env, "\n")
	for _, setting := range []string{"SSH_ASKPASS_REQUIRE=force", sshPasswordPromptVariable + "=1", "SSH_ASKPASS="} {
		if !strings.Contains(environment, setting) {
			t.Errorf("ssh is started without %s", setting)
		}
	}
	if !strings.Contains(strings.Join(command.Args, " "), "NumberOfPasswordPrompts=1") {
		t.Errorf("ssh retries a refused password: %v", command.Args)
	}
}

func TestTheAskpassAnswerIsTheConsolePassword(t *testing.T) {
	t.Setenv(sshPasswordPromptVariable, "1")
	if !isAnsweringSSHPasswordPrompt() {
		t.Fatal("the binary does not recognize ssh asking it for the password")
	}
	t.Setenv(sshPasswordPromptVariable, "")
	if isAnsweringSSHPasswordPrompt() {
		t.Fatal("the binary answers a password prompt it was not asked")
	}
}

func TestSSHLeavesKeyAuthenticationAloneWithoutAPassword(t *testing.T) {
	connection := hostSSH{user: "admin", hostname: "ssh.example.test"}
	if strings.Contains(strings.Join(connection.sshArguments(nil), " "), "PubkeyAuthentication=no") {
		t.Error("ssh refused keys although no password was given")
	}
}
