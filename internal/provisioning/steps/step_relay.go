package setup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// The relay reaches a company's messenger on behalf of people who are not on
// this network, so its settings name that company and cannot be derived from
// the device. An operator hands them over; without them this step does nothing
// and the unit stays stopped, which is the right answer for a device whose
// company has not moved yet.
const (
	relaySettingsEnvironmentName = "INTERNKIM_RELAY_ENV"
	relayAgentKeyEnvironmentName = "INTERNKIM_RELAY_AGENT_KEY"
	admindSocketSettingName      = "ADMIND_SOCKET_PATH"
)

var StepRelay = Step{
	Name: "relay",
	Deps: nil,
	Title: func(context *Context) string {
		return context.T("릴레이 설정 배치 중...", "Placing the relay's settings...")
	},
	IsSatisfied: func(context *Context) bool {
		settingsPath := namedFile(relaySettingsEnvironmentName)
		if context.Backend != BackendSSH || settingsPath == "" {
			return true
		}
		settings, errorValue := os.ReadFile(settingsPath)
		if errorValue != nil {
			return false
		}
		if deviceFileDigest(context, blueclaw.RelayEnvironmentFilePath) != asPlacedDigest(relaySettingsNamingTheAdmindSocket(string(settings))) {
			return false
		}
		return trimmedRun(context, "systemctl is-active "+blueclaw.RelayServiceName) == "active"
	},
	Run: func(context *Context) error {
		if context.Backend != BackendSSH {
			return nil
		}
		settingsPath := namedFile(relaySettingsEnvironmentName)
		if settingsPath == "" {
			fmt.Println("  " + context.T(
				"릴레이 설정 없음 ("+relaySettingsEnvironmentName+" 미지정), 건너뜀",
				"no relay settings ("+relaySettingsEnvironmentName+" is not set), skipped",
			))
			return nil
		}
		settings, errorValue := os.ReadFile(settingsPath)
		if errorValue != nil {
			return fmt.Errorf("read the relay settings at %s: %w", settingsPath, errorValue)
		}
		context.SSH.Run(makeTheDirectoryTheAdmindSocketLivesIn())
		context.SSH.Run(placeForTheRelay(blueclaw.RelayEnvironmentFilePath, relaySettingsNamingTheAdmindSocket(string(settings))))

		if agentKeyPath := namedFile(relayAgentKeyEnvironmentName); agentKeyPath != "" {
			agentKey, errorValue := os.ReadFile(agentKeyPath)
			if errorValue != nil {
				return fmt.Errorf("read the relay's agent key at %s: %w", agentKeyPath, errorValue)
			}
			context.SSH.Run(placeForTheRelay(blueclaw.RelayAgentKeyPath, string(agentKey)))
		}

		context.SSH.Run(restartAfterClearingTheFailure(blueclaw.RelayServiceName))
		fmt.Println("  " + context.T("릴레이 설정 배치 완료", "relay settings placed"))
		return nil
	},
	RunSD: func(context *Context) error {
		return nil
	},
}

func relaySettingsNamingTheAdmindSocket(settings string) string {
	if settingsAlreadyName(settings, admindSocketSettingName) {
		return settings
	}
	return asPlaced(settings) + admindSocketSettingName + "=" + blueclaw.AdmindSocketPath
}

func settingsAlreadyName(settings string, settingName string) bool {
	for _, line := range strings.Split(settings, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), settingName+"=") {
			return true
		}
	}
	return false
}

func makeTheDirectoryTheAdmindSocketLivesIn() string {
	return "install -d -m 755 -o root -g root " + filepath.Dir(blueclaw.AdmindSocketPath)
}

func namedFile(environmentName string) string {
	return strings.TrimSpace(os.Getenv(environmentName))
}

// A running relay is not the same as a relay running on the settings an operator
// just handed over — changing one is the whole reason to run this step. What the
// device holds is compared to what would be placed, so a settings change is
// never mistaken for a device that is already done.
func asPlacedDigest(settings string) string {
	sum := sha256.Sum256([]byte(asPlaced(settings)))
	return hex.EncodeToString(sum[:])
}

func deviceFileDigest(context *Context, path string) string {
	answer := trimmedRun(context, "sha256sum "+path+" 2>/dev/null")
	digest, _, _ := strings.Cut(answer, " ")
	return digest
}

// Settings that stop the relay from starting are the ordinary reason to place
// new ones, and by then systemd has usually latched the unit: five failures
// inside the start-limit window and every later restart is refused for the rest
// of it. Clearing that first is what makes placing settings a fix.
func restartAfterClearingTheFailure(serviceName string) string {
	return `systemctl reset-failed ` + serviceName + ` 2>/dev/null || true
systemctl restart ` + serviceName
}

func asPlaced(content string) string {
	return strings.TrimRight(content, "\n") + "\n"
}

func placeForTheRelay(path string, content string) string {
	return `install -d -m 755 ` + filepath.Dir(path) + `
cat > ` + path + ` <<'RELAYFILEEOF'
` + strings.TrimRight(content, "\n") + `
RELAYFILEEOF
chown ` + blueclaw.RelayUserName + `:` + blueclaw.RelayUserName + ` ` + path + `
chmod 600 ` + path
}
