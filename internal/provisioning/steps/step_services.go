package setup

import (
	"fmt"
	"time"

	"github.com/anthropic-lab/internkim/internal/runtime/blueclaw"
)

var StepServices = Step{
	Name: "services",
	Deps: []string{"binaries", "openrouter", "litert", "mattermost"},
	Title: func(context *Context) string {
		return context.T("서비스 시작 중...", "Starting services...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Backend != BackendSSH {
			return false
		}
		runtimeCheck := trimmedRun(context, `python3 - <<'PY'
import json
try:
    with open("/root/.blueclaw/config/runtime.json") as file:
        document = json.dumps(json.load(file))
except Exception:
    print("missing")
    raise SystemExit
for forbidden in ("apiKeyPath", "botTokenPath", "signingSecretPath", "OPENROUTER_API_KEY", "wrapperPath", "modelPath", "backend"):
    if forbidden in document:
        print("legacy")
        raise SystemExit
if '"endpoint": "http://internkim"' not in document or '"unixSocketPath": "/run/internkim/capability.sock"' not in document:
    print("stale")
    raise SystemExit
print("ok")
PY`)
		databaseCheck := trimmedRun(context, `test -d /root/.blueclaw/migrations && su - postgres -c "psql -tAc \"SELECT 1 FROM pg_database WHERE datname='blueclaw'\"" 2>/dev/null | grep -q 1 && echo ok || echo missing`)
		return trimmedRun(context, "systemctl is-active "+blueclaw.BlueclawServiceName) == "active" &&
			trimmedRun(context, "systemctl is-active "+blueclaw.CapabilitydServiceName) == "active" &&
			trimmedRun(context, "systemctl is-active "+blueclaw.AdmindServiceName) == "active" &&
			trimmedRun(context, blueclaw.BlueclawHealthCheckCommand()) == "ok" &&
			trimmedRun(context, "systemctl is-active mattermost") == "active" &&
			runtimeCheck == "ok" &&
			databaseCheck == "ok"
	},
	Run: func(context *Context) error {
		connection := context.SSH

		runtimeConfiguration, err := blueclaw.BlueclawRuntimeConfigDocument(blueclaw.BlueclawDefaultModelName)
		if err != nil {
			return err
		}
		policyConfiguration, err := blueclaw.BlueclawPolicyDocument(context.Callbacks.LoadState("google_email"))
		if err != nil {
			return err
		}
		connection.Run(fmt.Sprintf(`mkdir -p %s
printf '%%s' %s > %s
printf '%%s' %s > %s
chown -R root:blueclaw %s
chmod 770 %s
chmod 640 %s %s`,
			blueclaw.BlueclawConfigPath,
			shellQuote(runtimeConfiguration),
			blueclaw.BlueclawRuntimeConfigPath,
			shellQuote(policyConfiguration),
			blueclaw.BlueclawPolicyConfigPath,
			blueclaw.BlueclawConfigPath,
			blueclaw.BlueclawConfigPath,
			blueclaw.BlueclawRuntimeConfigPath,
			blueclaw.BlueclawPolicyConfigPath,
		))

		connection.Run("rm -f /etc/init.d/S97httpd; killall board-bridge 2>/dev/null; " +
			"kill $(ps | grep 'python3 -m http.server' | grep -v grep | awk '{print $1}') 2>/dev/null || true")

		connection.Run(`mkdir -p /root/.internkim/secrets
if [ ! -s /root/.internkim/secrets/mattermost-bot-token ] && [ -s /root/.internkim/env/bot-token ]; then
  cp /root/.internkim/env/bot-token /root/.internkim/secrets/mattermost-bot-token
fi
chown root:root /root/.internkim/secrets /root/.internkim/secrets/mattermost-bot-token 2>/dev/null || true
chmod 700 /root/.internkim/secrets 2>/dev/null || true
chmod 600 /root/.internkim/secrets/mattermost-bot-token 2>/dev/null || true
rm -f /root/.internkim/env/bot-token`)

		connection.Run(`cd /root/.blueclaw/workspace/skills 2>/dev/null && \
rm -rf agent-browser github summarize skill-creator 2>/dev/null; \
echo "Cleaned unavailable skills"`)

		connection.Run(`systemctl start postgresql 2>/dev/null || service postgresql start 2>/dev/null || true
su - postgres -c "psql -tAc \"SELECT 1 FROM pg_roles WHERE rolname='blueclaw'\" | grep -q 1 || createuser blueclaw" 2>/dev/null || true
su - postgres -c "psql -tAc \"SELECT 1 FROM pg_database WHERE datname='blueclaw'\" | grep -q 1 || createdb -O blueclaw blueclaw" 2>/dev/null || true
su - postgres -c "psql -c \"ALTER DATABASE blueclaw OWNER TO blueclaw\"" 2>/dev/null || true`)

		connection.Run(fmt.Sprintf(`systemctl stop zeroclaw 2>/dev/null || true
systemctl disable zeroclaw 2>/dev/null || true
rm -f /etc/systemd/system/zeroclaw.service
rm -rf /etc/systemd/system/zeroclaw.service.d
systemctl enable systemd-time-wait-sync.service 2>/dev/null
cat > %s <<'SVCEOF'
%sSVCEOF
cat > %s <<'CAPABILITYEOF'
%sCAPABILITYEOF
cat > %s <<'ADMINDEOF'
%sADMINDEOF
systemctl daemon-reload
systemctl enable %s
systemctl restart %s
systemctl enable %s
systemctl restart %s
systemctl enable %s
systemctl restart %s
sleep 2`,
			blueclaw.BlueclawServicePath,
			blueclaw.BlueclawServiceUnit(),
			blueclaw.CapabilitydServicePath,
			blueclaw.CapabilitydServiceUnit(),
			blueclaw.AdmindServicePath,
			blueclaw.AdmindServiceUnit(),
			blueclaw.CapabilitydServiceName,
			blueclaw.CapabilitydServiceName,
			blueclaw.AdmindServiceName,
			blueclaw.AdmindServiceName,
			blueclaw.BlueclawServiceName,
			blueclaw.BlueclawServiceName,
		))

		isBlueclawHealthy := false
		for attempt := 0; attempt < 15; attempt++ {
			isBlueclawHealthy = trimmedRun(context, "systemctl is-active "+blueclaw.BlueclawServiceName) == "active" &&
				trimmedRun(context, "systemctl is-active "+blueclaw.CapabilitydServiceName) == "active" &&
				trimmedRun(context, "systemctl is-active "+blueclaw.AdmindServiceName) == "active" &&
				trimmedRun(context, blueclaw.BlueclawHealthCheckCommand()) == "ok"
			if isBlueclawHealthy {
				break
			}
			time.Sleep(2 * time.Second)
		}

		if isBlueclawHealthy {
			fmt.Println("  " + context.T("blueclaw 실행 중", "blueclaw running"))
		} else {
			fmt.Println("  " + context.T("gateway 시작 실패", "Gateway failed"))
		}

		connection.Run("systemctl stop lightpanda 2>/dev/null; systemctl disable lightpanda 2>/dev/null; " +
			"rm -f /etc/systemd/system/lightpanda.service; systemctl daemon-reload")

		connection.Run(`if ! grep -q '/swapfile' /proc/swaps 2>/dev/null; then
  if [ ! -f /swapfile ]; then
    dd if=/dev/zero of=/swapfile bs=1M count=256 2>/dev/null
    chmod 600 /swapfile; mkswap /swapfile >/dev/null 2>&1
  fi
  swapon /swapfile 2>/dev/null || true
  grep -q '/swapfile' /etc/fstab 2>/dev/null || echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi`)

		if deviceURL := context.Callbacks.LoadState("device_url"); deviceURL != "" {
			connection.Run(fmt.Sprintf(`
if [ -f /opt/mattermost/config/config.json ]; then
  sed -i 's|"SiteURL": "[^"]*"|"SiteURL": "%s"|' /opt/mattermost/config/config.json
  systemctl restart mattermost 2>/dev/null || true
fi`, deviceURL))
			fmt.Printf("  %s: %s\n", context.T("Mattermost URL 설정", "Mattermost URL"), deviceURL)
			if trimmedRun(context, `curl -sf http://localhost:8065/api/v4/system/ping 2>/dev/null | grep -o '"status":"OK"'`) != "" {
				fmt.Println("  " + context.T("Mattermost 응답 확인", "Mattermost responded"))
			} else {
				fmt.Println("  WARN: " + context.T("Mattermost 로컬 응답 확인 실패", "Mattermost local ping failed"))
			}
		}
		return nil
	},
}
