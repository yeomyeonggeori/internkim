package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestDeliveryShareIsBoundReadOnlyBeforeBlueclawStarts(t *testing.T) {
	mountCommand := buildBlueclawDeliveryMountCommand()

	for _, fragment := range []string{
		"mount --bind " + blueclaw.BlueclawDeliveryPath + " " + blueclaw.BlueclawDeliveryReadOnlyPath,
		"mount -o remount,bind,ro " + blueclaw.BlueclawDeliveryReadOnlyPath,
		"Before=blueclaw.service",
	} {
		if !strings.Contains(mountCommand, fragment) {
			t.Fatalf("virtiofsd has no read-only mode, so the bind must carry it: missing %q", fragment)
		}
	}

	if strings.Index(mountCommand, "mount --bind") > strings.Index(mountCommand, "remount,bind,ro") {
		t.Fatal("a bind takes its flags on the remount, so the bind has to come first")
	}
	if !strings.Contains(mountCommand, "findmnt -no OPTIONS") {
		t.Fatal("the unit must prove the share came up read-only rather than report success for starting")
	}
	if strings.Contains(blueclaw.BlueclawDeliveryServiceName, `\`) {
		t.Fatal("a unit named after a path needs systemd's escaping, which is a class of bug worth not having")
	}
}

func TestDeliveredConfigurationIsWrittenWhereTheShareServesIt(t *testing.T) {
	if !strings.Contains(buildBlueclawConfigurationDirectoryCommand(), blueclaw.BlueclawDeliveryConfigPath) {
		t.Fatal("expected the delivered configuration directory to be created")
	}
}

func TestDeliveredConfigurationIsReadableByModeNotByGroup(t *testing.T) {
	permissionCommand := buildBlueclawDeliveryPermissionCommand()

	for _, fragment := range []string{
		"chmod 0644 " + blueclaw.BlueclawDeliveryConfigPath + "/runtime.json",
		"chmod 0755 " + blueclaw.BlueclawDeliveryPath,
		"chown -R root:root " + blueclaw.BlueclawDeliveryPath,
	} {
		if !strings.Contains(permissionCommand, fragment) {
			t.Fatalf("configuration written over SSH lands at 0600 owned by that user, which the guest cannot open: missing %q", fragment)
		}
	}
}

func TestRootfsContractHoldsWhereverThePayloadIs(t *testing.T) {
	repositoryRootPath := deliveryRepositoryRoot(t)
	guestInit, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "assets", "blueclaw-runtime", "guest-init"))
	if errorValue != nil {
		t.Fatalf("expected guest init: %v", errorValue)
	}
	contractCommand := BlueclawRootfsBaseContractCheckCommand()

	for _, marker := range []string{
		"$(blueclaw_runtime_directory)/bin/blueclaw",
		"/delivery/runtime/current/bin/blueclaw",
		"/workspace/.blueclaw/runtime/current",
	} {
		if !strings.Contains(contractCommand, marker) {
			t.Fatalf("the contract has to name where the payload can be, missing %q", marker)
		}
		if !strings.Contains(string(guestInit), marker) {
			t.Fatalf("guest init no longer carries %q, so the contract would refuse the image it just built", marker)
		}
	}
}

func deliveryRepositoryRoot(t *testing.T) string {
	t.Helper()
	workingDirectory, errorValue := os.Getwd()
	if errorValue != nil {
		t.Fatalf("expected working directory: %v", errorValue)
	}
	for directory := workingDirectory; directory != "/"; directory = filepath.Dir(directory) {
		if _, errorValue := os.Stat(filepath.Join(directory, "go.mod")); errorValue == nil {
			return directory
		}
	}
	t.Fatal("expected the repository root")
	return ""
}

func TestTheConfigurationNeverPromisesARuntimeTheShareLacks(t *testing.T) {
	runtimeConfiguration := blueclaw.BlueclawGuestDeliveryRuntimePath
	if !strings.Contains(deliveredRuntimeCheckCommand(), blueclaw.BlueclawDeliveryRuntimePath+"/migrations") {
		t.Fatalf("the guest opens %s/migrations on every boot, so that is what has to be checked", runtimeConfiguration)
	}
	if !strings.Contains(deliveredRuntimeCheckCommand(), blueclaw.BlueclawDeliveryRuntimePath+"/bin/blueclaw") {
		t.Fatal("a share carrying migrations and no binary is the same outage in the other order")
	}
}

func TestTheStepThatWritesTheConfigurationAlsoFillsTheShare(t *testing.T) {
	installCommand := buildBlueclawConfigurationInstallCommand()

	for _, fragment := range []string{
		blueclaw.BlueclawDeliveryRuntimePath,
		blueclaw.BlueclawDeliverySkillsPath,
		"rsync -a --delete",
	} {
		if !strings.Contains(installCommand, fragment) {
			t.Fatalf("the configuration names the share's runtime, so the same step has to put one there: missing %q", fragment)
		}
	}
	if strings.Contains(blueclaw.BlueclawDeliveryRefreshCommand(), "set -e") {
		t.Fatal("the refresh is joined onto a command that already set -e; its own would mask the caller's")
	}
}

func TestAFreshDeviceIsNotBehindOnAPayloadItHasNeverHad(t *testing.T) {
	checkCommand := deliveredRuntimeCheckCommand()
	workspaceRuntimePath := blueclaw.BlueclawWorkspacePath + "/.blueclaw/runtime/current"

	if !strings.Contains(checkCommand, "[ ! -d "+shellQuote(workspaceRuntimePath)+" ]") {
		t.Fatal("blueclaw-config runs before blueclaw-payload, so a device with no payload yet must not be reported as a share that fell behind")
	}
	if strings.Index(checkCommand, "echo ok; exit 0") > strings.Index(checkCommand, "migrations") {
		t.Fatal("the fresh-device answer has to come before the check it would fail")
	}
	if !strings.Contains(blueclaw.BlueclawDeliveryRefreshCommand(), "if [ -d "+blueclaw.BlueclawWorkspacePath+"/.blueclaw/runtime/current ]") {
		t.Fatal("the refresh runs under set -e on a device that may have nothing to deliver yet")
	}
}
