package setup

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

var StepBuzzChatd = Step{
	Name: "buzz-chatd",
	Deps: []string{"buzz-seed", "buzz-relay", "buzz-public-host"},
	Title: func(context *Context) string {
		return context.T("Buzz chatd 브리지 설치 중...", "Installing Buzz chatd bridge...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Backend != BackendSSH {
			return true
		}
		// A chatd that answers is not a chatd that is configured the way this
		// release wants it. Whatever is installed keeps running until it says
		// where it binds, or a change to that address never reaches the device.
		if context.Callbacks.GetBuzzAgentSecret == nil {
			return false
		}
		agentSecret, errorValue := context.Callbacks.GetBuzzAgentSecret()
		if errorValue != nil {
			return false
		}
		installedUnit := trimmedRun(context, "cat "+blueclaw.ChatdServicePath)
		return trimmedRun(context, "systemctl is-active "+blueclaw.ChatdServiceName) == "active" &&
			trimmedRun(context, blueclaw.ChatdHealthCheckCommand()) == "ok" &&
			strings.Contains(installedUnit, "CHATD_LISTEN_HOSTNAME="+blueclaw.ChatdListenHostname) &&
			strings.Contains(installedUnit, "CHATD_STATE_DIRECTORY="+blueclaw.ChatdStateDirectoryPath) &&
			!strings.Contains(installedUnit, "CHATD_WORKSPACE_ROOT=") &&
			trimmedRun(context, "cat "+blueclaw.ChatdEnvironmentFilePath) == strings.TrimSpace(chatdEnvironmentFileContents(agentSecret))
	},
	Run: func(context *Context) error {
		if context.Backend != BackendSSH {
			return nil
		}
		if context.Callbacks.InstallBuzzRelayBinariesSSH == nil || context.Callbacks.GetBuzzAgentSecret == nil {
			return errors.New("buzz chatd callbacks missing")
		}
		agentSecret, errorValue := context.Callbacks.GetBuzzAgentSecret()
		if errorValue != nil {
			return errorValue
		}
		if errorValue := context.Callbacks.InstallBuzzRelayBinariesSSH(context); errorValue != nil {
			return errorValue
		}

		connection := context.SSH
		connection.Run(chatdEnvironmentCommand(agentSecret))
		connection.Run(chatdUnitInstallCommand(blueclaw.RelayPublicURL(context.RelayDomain)))

		fmt.Println("  " + context.T("Buzz chatd 설치 완료", "Buzz chatd installed"))
		return nil
	},
	RunSD: func(context *Context) error {
		return nil
	},
}

func chatdEnvironmentFileContents(agentSecret string) string {
	return "CHATD_BUZZ_PRIVATE_KEY=" + agentSecret + "\n"
}

func chatdEnvironmentCommand(agentSecret string) string {
	return `mkdir -p ` + path.Dir(blueclaw.ChatdEnvironmentFilePath) + `
cat > ` + blueclaw.ChatdEnvironmentFilePath + ` <<'CHATDENVEOF'
` + chatdEnvironmentFileContents(agentSecret) + `CHATDENVEOF
chmod 600 ` + blueclaw.ChatdEnvironmentFilePath
}

func chatdUnitInstallCommand(relayPublicURL string) string {
	return `cat > ` + blueclaw.ChatdServicePath + ` <<'CHATDUNITEOF'
` + blueclaw.ChatdServiceUnit(relayPublicURL) + `CHATDUNITEOF
systemctl daemon-reload
systemctl enable ` + blueclaw.ChatdServiceName + `
if ! systemctl is-active --quiet ` + blueclaw.BlueclawServiceName + `; then exit 0; fi
systemctl restart ` + blueclaw.ChatdServiceName + `
for attempt in $(seq 1 30); do
  curl -fsS --max-time 3 ` + blueclaw.ChatdEndpoint + blueclaw.ChatdHealthPath + ` >/dev/null 2>&1 && break
  sleep 1
done`
}
