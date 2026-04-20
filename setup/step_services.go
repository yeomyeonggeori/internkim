package setup

import "fmt"

var StepServices = Step{
	Name: "services",
	Deps: []string{"binaries", "openrouter", "mattermost"},
	Title: func(context *Context) string {
		return context.T("서비스 시작 중...", "Starting services...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Backend != BackendSSH {
			return false
		}
		return trimmedRun(context, "systemctl is-active zeroclaw") == "active" &&
			trimmedRun(context, "systemctl is-active mattermost") == "active"
	},
	Run: func(context *Context) error {
		connection := context.SSH

		connection.Run("rm -f /etc/init.d/S97httpd; killall board-bridge 2>/dev/null; " +
			"kill $(ps | grep 'python3 -m http.server' | grep -v grep | awk '{print $1}') 2>/dev/null || true")

		connection.Run(`cd /root/.zeroclaw/workspace/skills 2>/dev/null && \
rm -rf agent-browser github summarize skill-creator 2>/dev/null; \
echo "Cleaned unavailable skills"`)

		connection.Run(`systemctl enable systemd-time-wait-sync.service 2>/dev/null
cat > /etc/systemd/system/zeroclaw.service <<'SVCEOF'
[Unit]
Description=ZeroClaw AI Gateway
After=network-online.target time-sync.target
Wants=network-online.target time-sync.target

[Service]
User=zeroclaw
EnvironmentFile=/root/.internkim/secrets/openrouter-api-key
Environment=HOME=/home/zeroclaw
Environment=INTERNKIM_GAS_WEBHOOK_URL_FILE=/root/.internkim/secrets/gas-webhook-url
ExecStart=/usr/local/bin/zeroclaw daemon
Restart=on-failure

[Install]
WantedBy=multi-user.target
SVCEOF
systemctl daemon-reload
systemctl enable zeroclaw
systemctl restart zeroclaw
sleep 2`)

		if trimmedRun(context, "systemctl is-active zeroclaw") == "active" {
			fmt.Println("  " + context.T("zeroclaw gateway 실행 중", "zeroclaw gateway running"))
		} else {
			fmt.Println("  " + context.T("gateway 시작 실패", "Gateway failed"))
		}

		connection.Run("apt-get install -y -qq chromium 2>/dev/null")
		chromiumPath := trimmedRun(context, "which chromium 2>/dev/null")
		if chromiumPath != "" {
			connection.Run(fmt.Sprintf(`mkdir -p /etc/systemd/system/zeroclaw.service.d
cat > /etc/systemd/system/zeroclaw.service.d/browser.conf <<EOF
[Service]
ExecStartPre=/bin/bash -c 'mkdir -p /run/user/993 && chown zeroclaw:zeroclaw /run/user/993 && chmod 700 /run/user/993'
Environment=AGENT_BROWSER_EXECUTABLE_PATH=%s
Environment=XDG_RUNTIME_DIR=/run/user/993
EOF
systemctl daemon-reload`, chromiumPath))
			fmt.Println("  " + context.T("agent-browser + Chromium 설치 완료", "agent-browser + Chromium installed"))
		} else {
			fmt.Println("  " + context.T("Chromium 설치 실패 (건너뜀)", "Chromium install failed (skipped)"))
		}

		connection.Run("systemctl stop lightpanda 2>/dev/null; systemctl disable lightpanda 2>/dev/null; " +
			"rm -f /etc/systemd/system/lightpanda.service; systemctl daemon-reload")

		connection.Run(`if command -v rtk >/dev/null 2>&1 && command -v zeroclaw >/dev/null 2>&1; then
  mkdir -p /root/.zeroclaw/plugins
  cat > /root/.zeroclaw/plugins/rtk-rewrite.ts <<'PLUGEOF'
import { Plugin, PluginHookBeforeToolCallResult } from "zeroclaw";
import { execSync } from "child_process";

export default {
  name: "rtk-rewrite",
  hooks: {
    before_tool_call: (tool: string, input: Record<string, unknown>): PluginHookBeforeToolCallResult => {
      if (tool !== "exec" || typeof input.command !== "string") return {};
      try {
        const rewritten = execSync("rtk rewrite " + JSON.stringify(input.command), { encoding: "utf8" }).trim();
        if (rewritten && rewritten !== input.command) {
          return { updated_input: { ...input, command: rewritten } };
        }
      } catch (_) {}
      return {};
    },
  },
} satisfies Plugin;
PLUGEOF
  echo "rtk plugin installed"
fi`)
		fmt.Println("  " + context.T("rtk hook 설치 완료", "rtk hook installed"))

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
		}
		return nil
	},
}
