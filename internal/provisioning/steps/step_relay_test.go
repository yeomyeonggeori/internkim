package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestADeviceWithNoRelaySettingsIsLeftAlone(t *testing.T) {
	t.Setenv(relaySettingsEnvironmentName, "")
	context := &Context{Backend: BackendSSH, Language: "en"}

	if !StepRelay.IsSatisfied(context) {
		t.Fatal("a device whose company has not moved would be asked to place settings it does not have")
	}
	if errorValue := StepRelay.Run(context); errorValue != nil {
		t.Fatalf("the step failed instead of skipping: %v", errorValue)
	}
}

func TestMissingRelaySettingsAreReportedRatherThanIgnored(t *testing.T) {
	t.Setenv(relaySettingsEnvironmentName, filepath.Join(t.TempDir(), "not-here.env"))
	context := &Context{Backend: BackendSSH, Language: "en"}

	errorValue := StepRelay.Run(context)
	if errorValue == nil {
		t.Fatal("a settings path that names nothing was accepted")
	}
	if !strings.Contains(errorValue.Error(), "not-here.env") {
		t.Fatalf("the failure does not name the file: %v", errorValue)
	}
}

func TestTheSettingsLandWhereTheUnitReadsThem(t *testing.T) {
	command := placeForTheRelay(blueclaw.RelayEnvironmentFilePath, "MESSENGER_PLATFORM=mattermost\n")

	for _, expected := range []string{
		"install -d -m 755 " + filepath.Dir(blueclaw.RelayEnvironmentFilePath),
		"cat > " + blueclaw.RelayEnvironmentFilePath,
		"MESSENGER_PLATFORM=mattermost",
		"chown internkim:internkim " + blueclaw.RelayEnvironmentFilePath,
		"chmod 600 " + blueclaw.RelayEnvironmentFilePath,
	} {
		if !strings.Contains(command, expected) {
			t.Errorf("the placement command is missing %q:\n%s", expected, command)
		}
	}
}

func TestTheAgentKeyIsPlacedWhereOnlyTheRelayCanReadIt(t *testing.T) {
	command := placeForTheRelay(blueclaw.RelayAgentKeyPath, "a-key")

	if !strings.Contains(command, "chmod 600 "+blueclaw.RelayAgentKeyPath) {
		t.Fatalf("the agent key would be readable by others:\n%s", command)
	}
	if !strings.Contains(command, "chown internkim:internkim "+blueclaw.RelayAgentKeyPath) {
		t.Fatalf("the relay's own user could not read its key:\n%s", command)
	}
}

func TestALatchedUnitIsClearedBeforeItIsRestarted(t *testing.T) {
	command := restartAfterClearingTheFailure(blueclaw.RelayServiceName)

	clearedAt := strings.Index(command, "reset-failed "+blueclaw.RelayServiceName)
	restartedAt := strings.Index(command, "restart "+blueclaw.RelayServiceName)
	if clearedAt < 0 {
		t.Fatalf("settings placed on a unit systemd has latched would never take:\n%s", command)
	}
	if restartedAt < clearedAt {
		t.Fatalf("the restart runs before the failure is cleared:\n%s", command)
	}
}

func TestSettingsAreCarriedAcrossExactly(t *testing.T) {
	directory := t.TempDir()
	settingsPath := filepath.Join(directory, "relay.env")
	settings := "SUPABASE_URL=https://example.supabase.co\nMESSENGER_PLATFORM=mattermost\n"
	if errorValue := os.WriteFile(settingsPath, []byte(settings), 0o600); errorValue != nil {
		t.Fatalf("write the settings: %v", errorValue)
	}
	t.Setenv(relaySettingsEnvironmentName, settingsPath)

	command := placeForTheRelay(blueclaw.RelayEnvironmentFilePath, settings)
	for _, line := range strings.Split(strings.TrimSpace(settings), "\n") {
		if !strings.Contains(command, line) {
			t.Errorf("%q never reached the device", line)
		}
	}
}
