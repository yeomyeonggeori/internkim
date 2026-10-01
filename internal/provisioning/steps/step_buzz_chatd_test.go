package setup

import (
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func TestBuzzChatdDefersStartupUntilItsGuestGatewayCanExist(t *testing.T) {
	command := chatdUnitInstallCommand("")
	guard := "if ! systemctl is-active --quiet " + blueclaw.BlueclawServiceName + "; then exit 0; fi"
	restart := "systemctl restart " + blueclaw.ChatdServiceName
	if strings.Index(command, guard) < 0 || strings.Index(command, restart) <= strings.Index(command, guard) {
		t.Fatal("chatd installation must defer startup until the guest service is running")
	}
}

func TestServicesRecoverChatdAfterGuestReadiness(t *testing.T) {
	previousAttempts, previousDelay := blueclawServiceHealthAttempts, blueclawServiceHealthRetryDelay
	blueclawServiceHealthAttempts, blueclawServiceHealthRetryDelay = 4, 0
	t.Cleanup(func() {
		blueclawServiceHealthAttempts, blueclawServiceHealthRetryDelay = previousAttempts, previousDelay
	})
	connection := &chatdBootstrapConnection{}
	context := &Context{Backend: BackendSSH, SSH: connection}
	if errorValue := StepServices.Run(context); errorValue != nil {
		t.Fatal(errorValue)
	}
	if connection.restartCount != 1 || connection.restartProbeCount != 2 || connection.probeCount != 3 {
		t.Fatalf("chatd must restart once after guest readiness and pass a fresh health check: %+v", connection)
	}
}

type chatdBootstrapConnection struct {
	probeCount        int
	restartCount      int
	restartProbeCount int
}

func (connection *chatdBootstrapConnection) Run(command string) string {
	switch command {
	case blueclawRuntimeContractCheckCommand(), blueclawRootfsBaseContractCheckCommand():
		return "ok"
	case restartInstalledChatdCommand():
		connection.restartCount++
		connection.restartProbeCount = connection.probeCount
	case blueclawServiceHealthReportCommand(&Context{Backend: BackendSSH}):
		connection.probeCount++
		guestHealth, capabilityHealth := "no", "unready:chatd"
		if connection.probeCount >= 2 {
			guestHealth = "ok"
		}
		if connection.restartCount == 1 {
			capabilityHealth = "ok"
		}
		return "blueclaw=active\ncapabilityd=active\nadmind=active\nblueclawHealth=" + guestHealth + "\ncapabilitydHealth=" + capabilityHealth
	}
	return ""
}

func (connection *chatdBootstrapConnection) SCP(string, string) error {
	return nil
}

const chatdTestAgentSecret = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

type chatdConnection struct {
	installedEnvironment string
	installedUnit        string
}

func (connection chatdConnection) Run(command string) string {
	switch {
	case strings.HasPrefix(command, "systemctl is-active"):
		return "active"
	case command == blueclaw.ChatdHealthCheckCommand():
		return "ok"
	case command == "cat "+blueclaw.ChatdServicePath:
		return connection.installedUnit
	case command == "cat "+blueclaw.ChatdEnvironmentFilePath:
		return connection.installedEnvironment
	}
	return ""
}

func (connection chatdConnection) SCP(localPath, remotePath string) error {
	return nil
}

func chatdContext(installedEnvironment string) *Context {
	return chatdContextWithUnit(installedEnvironment, blueclaw.ChatdServiceUnit(""))
}

func chatdContextWithUnit(installedEnvironment string, installedUnit string) *Context {
	return &Context{
		Backend: BackendSSH,
		SSH:     chatdConnection{installedEnvironment: installedEnvironment, installedUnit: installedUnit},
		Callbacks: Callbacks{
			GetBuzzAgentSecret: func() (string, error) { return chatdTestAgentSecret, nil },
		},
	}
}

func TestBuzzChatdIsSatisfiedByTheEnvironmentItWrites(t *testing.T) {
	if !StepBuzzChatd.IsSatisfied(chatdContext(chatdEnvironmentFileContents(chatdTestAgentSecret))) {
		t.Fatal("the environment this step writes must count as installed")
	}
}

func TestBuzzChatdReinstallsWhenTheEnvironmentHasDrifted(t *testing.T) {
	stale := chatdEnvironmentFileContents(chatdTestAgentSecret) +
		"CHATD_BUZZ_KEY_SEED_PATH=/root/.internkim/secrets/buzz-key-seed\n"
	if StepBuzzChatd.IsSatisfied(chatdContext(stale)) {
		t.Fatal("a line this step no longer writes must require reinstallation")
	}
}

func TestBuzzChatdReinstallsWhenTheAgentSecretHasChanged(t *testing.T) {
	if StepBuzzChatd.IsSatisfied(chatdContext(chatdEnvironmentFileContents(strings.Repeat("0", 64)))) {
		t.Fatal("an environment naming another key must require reinstallation")
	}
}

func TestBuzzChatdEnvironmentNamesNoIdentitySeed(t *testing.T) {
	command := chatdEnvironmentCommand(chatdTestAgentSecret)
	if strings.Contains(command, "CHATD_BUZZ_KEY_SEED_PATH") {
		t.Fatal("chatd no longer derives identities, so it is handed no identity seed")
	}
	if !strings.Contains(command, "CHATD_BUZZ_PRIVATE_KEY="+chatdTestAgentSecret) {
		t.Fatalf("the agent's own key must still be written, got %s", command)
	}
}

func TestBuzzChatdReinstallsAUnitThatGivesNoStateDirectory(t *testing.T) {
	withoutStateDirectory := strings.ReplaceAll(
		blueclaw.ChatdServiceUnit(""),
		"Environment=CHATD_STATE_DIRECTORY="+blueclaw.ChatdStateDirectoryPath+"\n",
		"",
	)
	if StepBuzzChatd.IsSatisfied(chatdContextWithUnit(chatdEnvironmentFileContents(chatdTestAgentSecret), withoutStateDirectory)) {
		t.Fatal("a unit written before chatd kept a delivery record must be rewritten, or the record never reaches disk")
	}
}

func TestBuzzChatdInstallRemovesTheLegacyTLSDropIns(t *testing.T) {
	command := chatdUnitInstallCommand("")
	for _, path := range blueclaw.ChatdLegacyTLSDropInPaths() {
		if !strings.Contains(command, path) {
			t.Fatalf("a device that still has %s would keep verification off, got:\n%s", path, command)
		}
	}
}
