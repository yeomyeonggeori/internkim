package setup

import (
	"errors"
	"fmt"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

var StepBuzzChatd = Step{
	Name: "buzz-chatd",
	Deps: []string{"buzz-relay", "buzz-public-host"},
	Title: func(context *Context) string {
		return context.T("Buzz chatd 브리지 설치 중...", "Installing Buzz chatd bridge...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Backend != BackendSSH {
			return true
		}
		return trimmedRun(context, "systemctl is-active "+blueclaw.ChatdServiceName) == "active" &&
			trimmedRun(context, blueclaw.ChatdHealthCheckCommand()) == "ok"
	},
	Run: func(context *Context) error {
		if context.Backend != BackendSSH {
			return nil
		}
		if context.Callbacks.InstallBuzzRelayBinariesSSH == nil || context.Callbacks.GetBuzzBootstrapSecret == nil {
			return errors.New("buzz chatd callbacks missing")
		}
		bootstrapSecret, errorValue := context.Callbacks.GetBuzzBootstrapSecret()
		if errorValue != nil {
			return errorValue
		}
		if errorValue := context.Callbacks.InstallBuzzRelayBinariesSSH(context); errorValue != nil {
			return errorValue
		}

		connection := context.SSH
		connection.Run(chatdEnvironmentCommand(bootstrapSecret))
		connection.Run(chatdUnitInstallCommand(blueclaw.DeriveRelayPublicURL(context.PublicURL)))

		fmt.Println("  " + context.T("Buzz chatd 설치 완료", "Buzz chatd installed"))
		return nil
	},
	RunSD: func(context *Context) error {
		return nil
	},
}

func chatdEnvironmentCommand(bootstrapSecret string) string {
	return `mkdir -p /root/.internkim/secrets
printf 'CHATD_BUZZ_PRIVATE_KEY=%s\n' '` + bootstrapSecret + `' > ` + blueclaw.ChatdEnvironmentFilePath + `
chmod 600 ` + blueclaw.ChatdEnvironmentFilePath
}

func chatdUnitInstallCommand(relayPublicURL string) string {
	return `cat > ` + blueclaw.ChatdServicePath + ` <<'CHATDUNITEOF'
` + blueclaw.ChatdServiceUnit(relayPublicURL) + `CHATDUNITEOF
systemctl daemon-reload
systemctl enable ` + blueclaw.ChatdServiceName + `
systemctl restart ` + blueclaw.ChatdServiceName + `
for attempt in $(seq 1 30); do
  curl -fsS --max-time 3 ` + blueclaw.ChatdEndpoint + `/health >/dev/null 2>&1 && break
  sleep 1
done`
}
