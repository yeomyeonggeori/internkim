package setup

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBlueclawGuestHealthSkipsRootfsMountCheckWhenServiceIsActive(t *testing.T) {
	connection := &blueclawGuestHealthBoardConnection{blueclawServiceStatus: "active", rootfsContractOutput: "rootfs-passwd-missing-blueclaw-user"}
	context := &Context{SSH: connection}
	failedChecks := []string{}

	checkBlueclawGuestRuntime(context, &failedChecks)

	if len(failedChecks) != 0 {
		t.Fatalf("expected active service health to skip rootfs mount check, got failed checks %+v", failedChecks)
	}
	if connection.hasCommand("rootfs_path=") {
		t.Fatal("expected active service health not to mount-check the rootfs image")
	}
}

func TestBlueclawGuestHealthRunsRootfsMountCheckWhenServiceIsInactive(t *testing.T) {
	connection := &blueclawGuestHealthBoardConnection{blueclawServiceStatus: "inactive", rootfsContractOutput: "rootfs-passwd-missing-blueclaw-user"}
	context := &Context{SSH: connection}
	failedChecks := []string{}

	checkBlueclawGuestRuntime(context, &failedChecks)

	if len(failedChecks) != 1 || failedChecks[0] != "blueclaw-guest-runtime" {
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

	checkBlueclawUsersPolicyWithin(context, time.Millisecond, time.Millisecond, &failedChecks)

	if len(failedChecks) != 1 || failedChecks[0] != "blueclaw-users-policy" {
		t.Fatalf("expected missing API user to fail policy health, got %+v", failedChecks)
	}
}

func TestBlueclawUsersPolicyHealthReportsUnavailableAPI(t *testing.T) {
	connection := &blueclawUsersPolicyHealthBoardConnection{output: "policy-api-unavailable"}
	context := &Context{SSH: connection}
	failedChecks := []string{}

	checkBlueclawUsersPolicyWithin(context, time.Millisecond, time.Millisecond, &failedChecks)

	if len(failedChecks) != 1 || failedChecks[0] != "blueclaw-users-policy" {
		t.Fatalf("expected unavailable API to fail policy health, got %+v", failedChecks)
	}
}

func TestBlueclawUsersPolicyHealthWaitsForTheRosterToArrive(t *testing.T) {
	connection := &blueclawUsersPolicyHealthBoardConnection{outputs: []string{
		"member1@example.com,member2@example.com",
		"member1@example.com,member2@example.com",
		"ok",
	}}
	context := &Context{SSH: connection}
	failedChecks := []string{}

	checkBlueclawUsersPolicyWithin(context, time.Second, time.Millisecond, &failedChecks)

	if len(failedChecks) != 0 {
		t.Fatalf("expected a roster that arrives on the third read to pass, got %+v", failedChecks)
	}
	if connection.runCount != 3 {
		t.Fatalf("expected health to read the policy until the roster arrived, got %d reads", connection.runCount)
	}
}

func TestBlueclawUsersPolicyHealthNamesTheEmailsThatNeverArrived(t *testing.T) {
	connection := &blueclawUsersPolicyHealthBoardConnection{output: "member1@example.com,member2@example.com,member3@example.com"}
	context := &Context{SSH: connection}
	failedChecks := []string{}

	output := captureStandardOutput(t, func() {
		checkBlueclawUsersPolicyWithin(context, 10*time.Millisecond, time.Millisecond, &failedChecks)
	})

	if len(failedChecks) != 1 || failedChecks[0] != "blueclaw-users-policy" {
		t.Fatalf("expected a roster that never arrives to fail policy health, got %+v", failedChecks)
	}
	if connection.runCount < 2 {
		t.Fatalf("expected health to have waited and read again, got %d reads", connection.runCount)
	}
	if !strings.Contains(output, "member3@example.com") {
		t.Fatalf("expected the failure to name the missing emails, got %s", output)
	}
}

func captureStandardOutput(t *testing.T, run func()) string {
	t.Helper()
	readEnd, writeEnd, errorValue := os.Pipe()
	if errorValue != nil {
		t.Fatalf("pipe: %v", errorValue)
	}
	previousStandardOutput := os.Stdout
	os.Stdout = writeEnd
	run()
	os.Stdout = previousStandardOutput
	if errorValue := writeEnd.Close(); errorValue != nil {
		t.Fatalf("close: %v", errorValue)
	}
	captured, errorValue := io.ReadAll(readEnd)
	if errorValue != nil {
		t.Fatalf("read: %v", errorValue)
	}
	return string(captured)
}

type blueclawUsersPolicyHealthBoardConnection struct {
	output   string
	outputs  []string
	command  string
	runCount int
}

func containsString(values []string, expectedValue string) bool {
	for _, value := range values {
		if value == expectedValue {
			return true
		}
	}
	return false
}

func (connection *blueclawUsersPolicyHealthBoardConnection) Run(command string) string {
	connection.command = command
	connection.runCount++
	if len(connection.outputs) == 0 {
		return connection.output
	}
	if connection.runCount > len(connection.outputs) {
		return connection.outputs[len(connection.outputs)-1]
	}
	return connection.outputs[connection.runCount-1]
}

func (connection *blueclawUsersPolicyHealthBoardConnection) SCP(localPath string, remotePath string) error {
	return nil
}

type blueclawGuestHealthBoardConnection struct {
	blueclawServiceStatus string
	rootfsContractOutput  string
	commands              []string
}

func (connection *blueclawGuestHealthBoardConnection) Run(command string) string {
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

func (connection *blueclawGuestHealthBoardConnection) SCP(localPath string, remotePath string) error {
	return nil
}

func (connection *blueclawGuestHealthBoardConnection) hasCommand(fragment string) bool {
	for _, command := range connection.commands {
		if strings.Contains(command, fragment) {
			return true
		}
	}
	return false
}
