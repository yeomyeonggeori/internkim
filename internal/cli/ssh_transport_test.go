package cli

import (
	"strings"
	"testing"
)

func TestSSHPinsPasswordAuthenticationWhenGivenAPassword(t *testing.T) {
	client := &sshClient{user: "admin", host: "127.0.0.1", port: "22", pass: "secret"}
	arguments := strings.Join(client.sshArgs(), " ")
	if !strings.Contains(arguments, "PreferredAuthentications=password") ||
		!strings.Contains(arguments, "PubkeyAuthentication=no") {
		t.Errorf("ssh offers keys before the password it was given: %s", arguments)
	}
}

func TestSSHLeavesKeyAuthenticationAloneWithoutAPassword(t *testing.T) {
	client := &sshClient{user: "admin", host: "127.0.0.1", port: "22"}
	if strings.Contains(strings.Join(client.sshArgs(), " "), "PubkeyAuthentication=no") {
		t.Error("ssh refused keys although no password was given")
	}
}
