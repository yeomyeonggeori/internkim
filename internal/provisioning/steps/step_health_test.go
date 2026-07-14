package setup

import (
	"strings"
	"testing"
)

func TestBlueclawFirecrackerHealthSkipsRootfsMountCheckWhenServiceIsActive(t *testing.T) {
	connection := &blueclawFirecrackerHealthBoardConnection{blueclawServiceStatus: "active", rootfsContractOutput: "rootfs-passwd-missing-blueclaw-user"}
	context := &Context{SSH: connection}
	failedChecks := []string{}

	checkBlueclawFirecrackerRuntime(context, &failedChecks)

	if len(failedChecks) != 0 {
		t.Fatalf("expected active service health to skip rootfs mount check, got failed checks %+v", failedChecks)
	}
	if connection.hasCommand("rootfs_path=") {
		t.Fatal("expected active service health not to mount-check the rootfs image")
	}
}

func TestBlueclawFirecrackerHealthRunsRootfsMountCheckWhenServiceIsInactive(t *testing.T) {
	connection := &blueclawFirecrackerHealthBoardConnection{blueclawServiceStatus: "inactive", rootfsContractOutput: "rootfs-passwd-missing-blueclaw-user"}
	context := &Context{SSH: connection}
	failedChecks := []string{}

	checkBlueclawFirecrackerRuntime(context, &failedChecks)

	if len(failedChecks) != 1 || failedChecks[0] != "blueclaw-firecracker-runtime" {
		t.Fatalf("expected inactive service health to fail rootfs drift, got failed checks %+v", failedChecks)
	}
	if !connection.hasCommand("rootfs_path=") {
		t.Fatal("expected inactive service health to mount-check the rootfs image")
	}
}

func TestBlueclawUsersPolicyHealthUsesAuthoritativeAPI(t *testing.T) {
	connection := &blueclawUsersPolicyHealthBoardConnection{output: "ok"}
	context := &Context{SSH: connection}
	failedChecks := []string{}

	checkBlueclawUsersPolicy(context, &failedChecks)

	if len(failedChecks) != 0 {
		t.Fatalf("expected authoritative API policy to pass, got %+v", failedChecks)
	}
	if !strings.Contains(connection.command, "http://127.0.0.1:8080/admin/api/policy") {
		t.Fatalf("expected health to read live policy API, got %s", connection.command)
	}
	if strings.Contains(connection.command, "/root/.blueclaw/config/policy.json") {
		t.Fatalf("expected health to avoid the stale host policy seed, got %s", connection.command)
	}
}

func TestBlueclawUsersPolicyHealthReportsMissingAPIUser(t *testing.T) {
	connection := &blueclawUsersPolicyHealthBoardConnection{output: "admin@localhost"}
	context := &Context{SSH: connection}
	failedChecks := []string{}

	checkBlueclawUsersPolicy(context, &failedChecks)

	if len(failedChecks) != 1 || failedChecks[0] != "blueclaw-users-policy" {
		t.Fatalf("expected missing API user to fail policy health, got %+v", failedChecks)
	}
}

func TestBlueclawUsersPolicyHealthReportsUnavailableAPI(t *testing.T) {
	connection := &blueclawUsersPolicyHealthBoardConnection{output: "policy-api-unavailable"}
	context := &Context{SSH: connection}
	failedChecks := []string{}

	checkBlueclawUsersPolicy(context, &failedChecks)

	if len(failedChecks) != 1 || failedChecks[0] != "blueclaw-users-policy" {
		t.Fatalf("expected unavailable API to fail policy health, got %+v", failedChecks)
	}
}

type blueclawUsersPolicyHealthBoardConnection struct {
	output  string
	command string
}

func (connection *blueclawUsersPolicyHealthBoardConnection) Run(command string) string {
	connection.command = command
	return connection.output
}

func (connection *blueclawUsersPolicyHealthBoardConnection) SCP(localPath string, remotePath string) error {
	return nil
}

type blueclawFirecrackerHealthBoardConnection struct {
	blueclawServiceStatus string
	rootfsContractOutput  string
	commands              []string
}

func (connection *blueclawFirecrackerHealthBoardConnection) Run(command string) string {
	connection.commands = append(connection.commands, command)
	switch {
	case strings.Contains(command, "rootfs_path="):
		return connection.rootfsContractOutput
	case strings.Contains(command, "from pathlib import Path"):
		return "ok"
	case strings.Contains(command, "blkid -o value -s TYPE /var/lib/blueclaw/workspace.ext4"):
		return "ext4"
	case strings.Contains(command, "stat -c '%s' /var/lib/blueclaw/workspace.ext4"):
		return "ok"
	case strings.Contains(command, "systemctl is-active blueclaw"):
		return connection.blueclawServiceStatus
	default:
		return ""
	}
}

func (connection *blueclawFirecrackerHealthBoardConnection) SCP(localPath string, remotePath string) error {
	return nil
}

func (connection *blueclawFirecrackerHealthBoardConnection) hasCommand(fragment string) bool {
	for _, command := range connection.commands {
		if strings.Contains(command, fragment) {
			return true
		}
	}
	return false
}
