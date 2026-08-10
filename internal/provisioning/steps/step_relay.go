package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// The relay reaches a company's messenger on behalf of people who are not on
// this network, so its settings name that company and cannot be derived from
// the device. An operator hands them over; without them this step does nothing
// and the unit stays stopped, which is the right answer for a device whose
// company has not moved yet.
const (
	relaySettingsEnvironmentName = "INTERNKIM_RELAY_ENV"
	relayAgentKeyEnvironmentName = "INTERNKIM_RELAY_AGENT_KEY"
)

var StepRelay = Step{
	Name: "relay",
	Deps: []string{"services"},
	Title: func(context *Context) string {
		return context.T("릴레이 설정 배치 중...", "Placing the relay's settings...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Backend != BackendSSH || namedFile(relaySettingsEnvironmentName) == "" {
			return true
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
		context.SSH.Run(placeForTheRelay(blueclaw.RelayEnvironmentFilePath, string(settings)))

		if agentKeyPath := namedFile(relayAgentKeyEnvironmentName); agentKeyPath != "" {
			agentKey, errorValue := os.ReadFile(agentKeyPath)
			if errorValue != nil {
				return fmt.Errorf("read the relay's agent key at %s: %w", agentKeyPath, errorValue)
			}
			context.SSH.Run(placeForTheRelay(blueclaw.RelayAgentKeyPath, string(agentKey)))
		}

		context.SSH.Run("systemctl restart " + blueclaw.RelayServiceName)
		fmt.Println("  " + context.T("릴레이 설정 배치 완료", "relay settings placed"))
		return nil
	},
	RunSD: func(context *Context) error {
		return nil
	},
}

func namedFile(environmentName string) string {
	return strings.TrimSpace(os.Getenv(environmentName))
}

func placeForTheRelay(path string, content string) string {
	return `install -d -m 755 ` + filepath.Dir(path) + `
cat > ` + path + ` <<'RELAYFILEEOF'
` + strings.TrimRight(content, "\n") + `
RELAYFILEEOF
chown ` + blueclaw.RelayUserName + `:` + blueclaw.RelayUserName + ` ` + path + `
chmod 600 ` + path
}
