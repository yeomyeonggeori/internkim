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

func TestSSHLeavesKeyAuthenticationAloneWithoutAPassword(t *testing.T) {
	connection := hostSSH{user: "admin", hostname: "ssh.example.test"}
	if strings.Contains(strings.Join(connection.sshArguments(nil), " "), "PubkeyAuthentication=no") {
		t.Error("ssh refused keys although no password was given")
	}
}
