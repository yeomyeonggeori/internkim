package blueclaw

import (
	"strings"
	"testing"
)

func TestDeliveryRefreshMakesThePayloadRunnableByTheGuest(t *testing.T) {
	command := BlueclawDeliveryRefreshCommand()

	for _, fragment := range []string{
		BlueclawWorkspacePath + "/.blueclaw/runtime/current/",
		BlueclawDeliveryRuntimePath,
		"chown -R root:root " + BlueclawDeliveryPath,
		"-type d -exec chmod 0755",
		"-type f -exec chmod 0644",
	} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("the guest reads the share by mode, and the host's ownership passes through: missing %q", fragment)
		}
	}

	if !strings.Contains(command, BlueclawDeliveryRuntimePath+"/bin -type f -exec chmod 0755") {
		t.Fatal("the payload has to stay executable by the guest's blueclaw, which the blanket 0644 would take away")
	}
	if strings.Index(command, "-type f -exec chmod 0644") > strings.Index(command, "/bin -type f -exec chmod 0755") {
		t.Fatal("the blanket file mode has to come first, or it clears the executable bit it just set")
	}
}

func TestMigrationsFollowThePayload(t *testing.T) {
	if migrationPath := guestMigrationDirectoryPath(FirecrackerMonitorName); migrationPath != BlueclawGuestMigrationPath {
		t.Fatalf("Firecracker offers no share, so the migrations stay in the image, got %q", migrationPath)
	}
	for _, virtualMachineMonitor := range []string{CloudHypervisorMonitorName, VfkitMonitorName} {
		migrationPath := guestMigrationDirectoryPath(virtualMachineMonitor)
		if !strings.HasPrefix(migrationPath, BlueclawGuestDeliveryRuntimePath) {
			t.Fatalf("the migrations travel with the payload under %s, got %q", virtualMachineMonitor, migrationPath)
		}
	}
}

func TestOnlyAHostThatCanBindReadOnlyIsGivenTheReadOnlyPath(t *testing.T) {
	if deliveryPath := deliveryDirectoryPathForMonitor(CloudHypervisorMonitorName); deliveryPath != BlueclawDeliveryReadOnlyPath {
		t.Fatalf("a Linux host binds the share read-only and serves that, got %q", deliveryPath)
	}
	if deliveryPath := deliveryDirectoryPathForMonitor(VfkitMonitorName); deliveryPath != BlueclawDeliveryPath {
		t.Fatalf("macOS has no bind mount, so delivery-ro never exists there and naming it would serve nothing, got %q", deliveryPath)
	}
	if deliveryPath := deliveryDirectoryPathForMonitor(FirecrackerMonitorName); deliveryPath != "" {
		t.Fatalf("Firecracker has no virtio-fs at all, got %q", deliveryPath)
	}
}

func TestDeliveryRefreshDoesNotSwallowAFailedSync(t *testing.T) {
	command := BlueclawDeliveryRefreshCommand()

	if strings.HasPrefix(strings.TrimSpace(command), "set -e") {
		t.Fatal("set -e belongs to the caller, which joins this to a sync whose failure must abort the pair")
	}
	if !strings.HasPrefix(command, "\n") {
		t.Fatal("expected the refresh to begin on its own line, since it is appended to a command")
	}
}

func TestDeliveryRefreshMirrorsTheBundledSkills(t *testing.T) {
	command := BlueclawDeliveryRefreshCommand()

	if !strings.Contains(command, BlueclawWorkspacePath+"/skills/ "+BlueclawDeliverySkillsPath+"/") {
		t.Fatal("the bundled skills are the host's, so they belong on the share with the payload")
	}
	if strings.Contains(command, ".agents/skills") {
		t.Fatal("the skills the agent writes are its own and stay in the workspace, where it can write them")
	}
}
