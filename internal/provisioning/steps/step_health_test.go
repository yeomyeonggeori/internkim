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

func TestBlueclawUsersPolicyHealthReadsRuntimePolicyAPI(t *testing.T) {
	connection := &blueclawUsersPolicyHealthBoardConnection{}
	context := &Context{SSH: connection}
	failedChecks := []string{}

	checkBlueclawUsersPolicy(context, &failedChecks)

	if len(failedChecks) != 0 {
		t.Fatalf("expected runtime policy API health to pass, got failed checks %+v", failedChecks)
	}
	if !strings.Contains(connection.command, "http://127.0.0.1:8080/admin/api/policy") {
		t.Fatalf("expected runtime policy API lookup, got %q", connection.command)
	}
	if strings.Contains(connection.command, `policy_path = "/root/.blueclaw/config/policy.json"`) {
		t.Fatal("expected health check to avoid the pre-launch host policy copy")
	}
}

type blueclawUsersPolicyHealthBoardConnection struct {
	command string
}

func (connection *blueclawUsersPolicyHealthBoardConnection) Run(command string) string {
	connection.command = command
	return "ok"
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
