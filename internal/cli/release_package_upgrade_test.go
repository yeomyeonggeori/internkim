package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const relayFileTheFirstReleaseWrote = `SUPABASE_URL=https://example.supabase.test
SUPABASE_PUBLISHABLE_KEY=publishable
INTERNKIM_APP_URL=https://example.test
GATEWAY_URL=wss://gateway.example.test
MESSENGER_PLATFORM=buzz
AGENT_API_KEY_PATH=/etc/internkim/agent-key
CHATD_BASE_URL=http://127.0.0.1:18090
ADMIND_BASE_URL=http://127.0.0.1:18080
ADMIND_SOCKET_PATH=/run/internkim/admind.sock
BLUECLAW_ACP_SOCKET_PATH=/run/internkim/blueclaw-acp.sock
WORKSPACE_ROOT_PATH=/workspace
`

// systemd.exec(5): "Settings from these files override settings made with
// Environment=", whatever order the two are written in.
func systemdEnvironment(t *testing.T, unit string, files map[string]string) map[string]string {
	t.Helper()
	environment := map[string]string{}
	filePaths := []string{}
	for _, line := range strings.Split(unit, "\n") {
		if assignment, isSetting := strings.CutPrefix(line, "Environment="); isSetting {
			name, value, _ := strings.Cut(assignment, "=")
			environment[name] = value
		}
		if filePath, isFile := strings.CutPrefix(line, "EnvironmentFile="); isFile {
			filePaths = append(filePaths, strings.TrimPrefix(filePath, "-"))
		}
	}
	for _, filePath := range filePaths {
		for _, line := range strings.Split(files[filePath], "\n") {
			if name, value, isAssignment := strings.Cut(line, "="); isAssignment {
				environment[name] = value
			}
		}
	}
	return environment
}

func packagedUnitNamed(t *testing.T, name string) string {
	t.Helper()
	for _, unit := range blueclaw.CompanyPackageUnits() {
		if unit.Name == name {
			return unit.Contents
		}
	}
	t.Fatalf("the package installs no %s", name)
	return ""
}

func startArgument(t *testing.T, serviceName string, flag string) string {
	t.Helper()
	service, _ := blueclaw.CompanyHostServiceNamed(blueclaw.LinuxCompanyHostLayout(), serviceName)
	index := slices.Index(service.Command, flag)
	if index < 0 || index+1 >= len(service.Command) {
		t.Fatalf("%s is started without %s", serviceName, flag)
	}
	return service.Command[index+1]
}

func relayFileAfterThePostInstall(t *testing.T, contents string) string {
	t.Helper()
	filePath := filepath.Join(t.TempDir(), "relay.env")
	if errorValue := os.WriteFile(filePath, []byte(contents), 0o640); errorValue != nil {
		t.Fatal(errorValue)
	}
	output, errorValue := exec.Command("sh", "-c", forgetThePackageSettingsInTheRelayFile(filePath)).CombinedOutput()
	if errorValue != nil {
		t.Fatalf("the post-install's relay file step fails: %v: %s", errorValue, output)
	}
	cleaned, errorValue := os.ReadFile(filePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(cleaned)
}

func TestAnUpgradedHostsRelayIsPointedAtTheSocketsThePackageShips(t *testing.T) {
	relayFile := relayFileAfterThePostInstall(t, relayFileTheFirstReleaseWrote)
	environment := systemdEnvironment(t, packagedUnitNamed(t, blueclaw.RelayServiceName), map[string]string{
		blueclaw.RelayEnvironmentFilePath: relayFile,
	})
	sockets := map[string]string{
		"ADMIND_SOCKET_PATH":       startArgument(t, blueclaw.AdmindServiceName, "-listen-socket"),
		"BLUECLAW_ACP_SOCKET_PATH": startArgument(t, blueclaw.BlueclawServiceName, "-acp-socket"),
	}
	for name, daemonPath := range sockets {
		if environment[name] != daemonPath {
			t.Errorf("after the upgrade the relay reads %s=%s and the daemon listens at %s, so no message reaches the agent", name, environment[name], daemonPath)
		}
	}
	if environment["SUPABASE_URL"] != "https://example.supabase.test" || environment["GATEWAY_URL"] != "wss://gateway.example.test" {
		t.Errorf("the upgrade lost the company's own settings from %s:\n%s", blueclaw.RelayEnvironmentFilePath, relayFile)
	}
}

func TestThePostInstallCleansTheRelayFileBeforeItRestartsTheRelay(t *testing.T) {
	script := maintainerScript(debianPackageFormat, postInstallScript)
	cleanedAt := strings.Index(script, forgetThePackageSettingsInTheRelayFile(blueclaw.RelayEnvironmentFilePath))
	if cleanedAt < 0 {
		t.Fatalf("the post-install leaves the package's settings in %s, where they outrank the unit", blueclaw.RelayEnvironmentFilePath)
	}
	if cleanedAt > strings.Index(script, "systemctl restart ") {
		t.Fatal("the post-install restarts the relay before it cleans its settings file, so the relay keeps the old values until its next restart")
	}
}

func TestThePostInstallLeavesACurrentRelayFileAlone(t *testing.T) {
	current := "SUPABASE_URL=https://example.supabase.test\nGATEWAY_URL=wss://gateway.example.test\n"
	if cleaned := relayFileAfterThePostInstall(t, current); cleaned != current {
		t.Fatalf("the post-install rewrote a relay file that carried nothing of the package's:\n%q", cleaned)
	}
}
