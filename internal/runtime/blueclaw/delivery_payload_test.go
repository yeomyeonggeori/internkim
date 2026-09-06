package blueclaw

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeliveryRefreshMakesThePayloadRunnableByTheGuest(t *testing.T) {
	command := BlueclawDeliveryRefreshCommand()

	for _, fragment := range []string{
		BlueclawWorkspacePath + "/.blueclaw/runtime/current/",
		BlueclawDeliveryRuntimePath,
		"chown -R root:root " + BlueclawDeliveryConfigPath + " " + BlueclawDeliveryRuntimePath + " " + BlueclawDeliverySkillsPath,
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
	if strings.Contains(command, "find "+BlueclawDeliveryPath+" -type") || strings.Contains(command, "chown -R root:root "+BlueclawDeliveryPath+"\n") {
		t.Fatal("public delivery permissions must not traverse the private assertion secret subtree")
	}
	for _, fragment := range []string{
		"install -d -o 998 -g 971 -m 0700 " + BlueclawDeliverySecretsPath,
		"if ! install -o 998 -g 971 -m 0400 " + InternKimCentralPlaneAgentKeyPath + " " + BlueclawDeliverySecretsPath + "/." + BlueclawAdminAssertionKeyName + ".$$; then rm -f ",
		"if ! mv -f " + BlueclawDeliverySecretsPath + "/." + BlueclawAdminAssertionKeyName + ".$$ ",
		"rm -f " + BlueclawDeliverySecretsPath + "/" + BlueclawAdminAssertionKeyName,
	} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("missing private assertion key handling %q", fragment)
		}
	}
}

func TestMigrationsFollowThePayload(t *testing.T) {
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

func TestDeliveryRefreshPreservesThePreviousKeyWhenInstallFails(t *testing.T) {
	command := BlueclawDeliveryRefreshCommand()
	var keyHandling string
	for _, line := range strings.Split(command, "\n") {
		if strings.Contains(line, "if [ -s "+InternKimCentralPlaneAgentKeyPath+" ]") {
			keyHandling = line
			break
		}
	}
	if keyHandling == "" {
		t.Fatal("expected the refresh command to contain the assertion key replacement")
	}

	temporaryDirectory := t.TempDir()
	sourcePath := filepath.Join(temporaryDirectory, "source-key")
	secretDirectory := filepath.Join(temporaryDirectory, "secrets")
	previousKeyPath := filepath.Join(secretDirectory, BlueclawAdminAssertionKeyName)
	if errorValue := os.WriteFile(sourcePath, []byte("new-key\n"), 0600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.Mkdir(secretDirectory, 0700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(previousKeyPath, []byte("old-key\n"), 0400); errorValue != nil {
		t.Fatal(errorValue)
	}
	failingInstallDirectory := filepath.Join(temporaryDirectory, "bin")
	if errorValue := os.Mkdir(failingInstallDirectory, 0700); errorValue != nil {
		t.Fatal(errorValue)
	}
	failingInstallPath := filepath.Join(failingInstallDirectory, "install")
	if errorValue := os.WriteFile(failingInstallPath, []byte("#!/bin/sh\nexit 1\n"), 0700); errorValue != nil {
		t.Fatal(errorValue)
	}

	keyHandling = strings.ReplaceAll(keyHandling, InternKimCentralPlaneAgentKeyPath, sourcePath)
	keyHandling = strings.ReplaceAll(keyHandling, BlueclawDeliverySecretsPath, secretDirectory)
	script := "set -e\n" + keyHandling + "\n"
	process := exec.Command("sh", "-c", script)
	process.Env = append(os.Environ(), "PATH="+failingInstallDirectory+":"+os.Getenv("PATH"))
	if errorValue := process.Run(); errorValue == nil {
		t.Fatal("a failed key install must abort delivery refresh")
	}
	key, errorValue := os.ReadFile(previousKeyPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(key) != "old-key\n" {
		t.Fatalf("failed install replaced the previous key: %q", key)
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
