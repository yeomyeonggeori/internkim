package setup

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
)

var blueclawServiceHealthAttempts = 180
var blueclawServiceHealthRetryDelay = 2 * time.Second
var errBlueclawHealthCheckFailed = errors.New("blueclaw health check failed after service restart")

var StepServices = Step{
	Name: "services",
	Deps: []string{"skills", "blueclaw-config", "blueclaw-payload", "openrouter", "local-llm", "mattermost"},
	Title: func(context *Context) string {
		return context.T("서비스 시작 중...", "Starting services...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Backend != BackendSSH {
			return false
		}
		runtimeCheck := trimmedRun(context, blueclawRuntimeContractCheckCommand())
		rootfsBaseCheck := trimmedRun(context, blueclawRootfsBaseContractCheckCommand())
		return trimmedRun(context, "systemctl is-active "+blueclaw.BlueclawServiceName) == "active" &&
			trimmedRun(context, "systemctl is-active "+blueclaw.CapabilitydServiceName) == "active" &&
			trimmedRun(context, "systemctl is-active "+blueclaw.AdmindServiceName) == "active" &&
			strings.Contains(trimmedRun(context, "systemctl cat "+locallm.LlamaCppServiceName+" 2>/dev/null"), locallm.LlamaCppBinaryPath) &&
			strings.Contains(trimmedRun(context, "systemctl cat "+locallm.LlamaCppEmbeddingServiceName+" 2>/dev/null"), locallm.LlamaCppEmbeddingModelPath) &&
			trimmedRun(context, blueclaw.BlueclawHealthCheckCommand()) == "ok" &&
			trimmedRun(context, blueclaw.CapabilitydHealthCheckCommand()) == "ok" &&
			trimmedRun(context, "systemctl is-active mattermost") == "active" &&
			runtimeCheck == "ok" &&
			rootfsBaseCheck == "ok"
	},
	Run: func(context *Context) error {
		connection := context.SSH

		if runtimeCheck := trimmedRun(context, blueclawRuntimeContractCheckCommand()); runtimeCheck != "ok" {
			return fmt.Errorf("blueclaw runtime configuration contract drift: %s", runtimeCheck)
		}
		if baseCheck := trimmedRun(context, blueclawRootfsBaseContractCheckCommand()); baseCheck != "ok" {
			return fmt.Errorf("blueclaw rootfs base contract drift: %s", baseCheck)
		}

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

		connection.Run(`mkdir -p /root/.blueclaw/workspace/.blueclaw/postgres /root/.blueclaw/workspace/.blueclaw/graphiti /root/.blueclaw/workspace/.blueclaw/logs /root/.blueclaw/workspace/.blueclaw/blobs
chown -R blueclaw:blueclaw /root/.blueclaw/workspace/.blueclaw
chmod -R u=rwX,g=rwX,o= /root/.blueclaw/workspace/.blueclaw`)

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
cat > %s <<'LLAMACPP_EOF'
%sLLAMACPP_EOF
cat > %s <<'LLAMACPP_EMBEDDING_EOF'
%sLLAMACPP_EMBEDDING_EOF
systemctl daemon-reload
systemctl disable %s 2>/dev/null || true
systemctl disable %s 2>/dev/null || true
systemctl enable %s
systemctl restart %s
systemctl enable %s
systemctl restart %s
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
			locallm.LlamaCppServicePath,
			blueclaw.LlamaCppServiceUnit(),
			locallm.LlamaCppEmbeddingServicePath,
			blueclaw.LlamaCppEmbeddingServiceUnit(),
			locallm.LlamaCppServiceName,
			locallm.LlamaCppEmbeddingServiceName,
			locallm.LlamaCppServiceName,
			locallm.LlamaCppServiceName,
			locallm.LlamaCppEmbeddingServiceName,
			locallm.LlamaCppEmbeddingServiceName,
			blueclaw.CapabilitydServiceName,
			blueclaw.CapabilitydServiceName,
			blueclaw.AdmindServiceName,
			blueclaw.AdmindServiceName,
			blueclaw.BlueclawServiceName,
			blueclaw.BlueclawServiceName,
		))

		isBlueclawHealthy := false
		for attempt := 0; attempt < blueclawServiceHealthAttempts; attempt++ {
			isBlueclawHealthy = trimmedRun(context, "systemctl is-active "+blueclaw.BlueclawServiceName) == "active" &&
				trimmedRun(context, "systemctl is-active "+blueclaw.CapabilitydServiceName) == "active" &&
				trimmedRun(context, "systemctl is-active "+blueclaw.AdmindServiceName) == "active" &&
				trimmedRun(context, "systemctl is-active "+locallm.LlamaCppEmbeddingServiceName) == "active" &&
				trimmedRun(context, blueclaw.BlueclawHealthCheckCommand()) == "ok" &&
				trimmedRun(context, blueclaw.CapabilitydHealthCheckCommand()) == "ok"
			if isBlueclawHealthy {
				break
			}
			time.Sleep(blueclawServiceHealthRetryDelay)
		}

		if isBlueclawHealthy {
			fmt.Println("  " + context.T("blueclaw 실행 중", "blueclaw running"))
			connection.Run("systemctl stop " + blueclaw.GraphitiMemorydServiceName + " 2>/dev/null || true; systemctl disable " + blueclaw.GraphitiMemorydServiceName + " 2>/dev/null || true")
		} else {
			fmt.Println("  " + context.T("gateway 시작 실패", "Gateway failed"))
			return errBlueclawHealthCheckFailed
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

func blueclawRuntimeContractCheckCommand() string {
	return `python3 - <<'PY'
import json

runtime_path = "/root/.blueclaw/config/runtime.json"
workspace_runtime_path = "/root/.blueclaw/workspace/.blueclaw/config/runtime.json"

try:
    with open(runtime_path) as file:
        runtime_configuration = json.load(file)
except Exception:
    print("runtime-config-missing")
    raise SystemExit

try:
    with open(workspace_runtime_path) as file:
        workspace_runtime_configuration = json.load(file)
except Exception:
    print("workspace-runtime-config-missing")
    raise SystemExit

if runtime_configuration != workspace_runtime_configuration:
    print("runtime-config-mirror-drift")
    raise SystemExit

document = json.dumps(runtime_configuration)
for forbidden in ("apiKeyPath", "botTokenPath", "signingSecretPath", "OPENROUTER_API_KEY", "wrapperPath", "modelPath", "backend", "defaultBudgetClass"):
    if forbidden in document:
        print("legacy-runtime-config")
        raise SystemExit

agent = runtime_configuration.get("agent", {})
if agent.get("defaultEffortLevel") != "standard":
    print("runtime-effort-level")
    raise SystemExit

capabilities = runtime_configuration.get("capabilities", {})
if capabilities.get("transport") != "vsock":
    print("runtime-capability-transport")
    raise SystemExit
if capabilities.get("endpoint") != "http://internkim-capability":
    print("runtime-capability-endpoint")
    raise SystemExit

for tool_name in capabilities.get("toolNames", []):
    if str(tool_name).startswith("google."):
        print("runtime-capability-google-tool")
        raise SystemExit

database = runtime_configuration.get("database", {})
if database.get("connectionString") != "postgres://blueclaw@/blueclaw?host=/workspace/.blueclaw/postgres&sslmode=disable":
    print("runtime-database")
    raise SystemExit

memory = runtime_configuration.get("memory", {})
if memory.get("graphitiEndpoint") != "http://127.0.0.1:7791":
    print("runtime-graphiti-endpoint")
    raise SystemExit
if memory.get("graphitiKuzuPath") != "/workspace/.blueclaw/graphiti/kuzu":
    print("runtime-graphiti-path")
    raise SystemExit

terminal = runtime_configuration.get("terminal", {})
if terminal.get("mode") != "firecrackerGuest":
    print("runtime-terminal-mode")
    raise SystemExit

profile_tool_names = []
for profile in runtime_configuration.get("agentProfiles", []):
    if profile.get("name") == "default":
        profile_tool_names = [str(tool_name) for tool_name in profile.get("allowedToolNames", [])]
        break

required_tools = {"terminal.run", "terminal.session", "browser_handoff.openURL", "approval.request", "file.write", "file.attach"}
missing_tools = sorted(required_tools - set(profile_tool_names))
if missing_tools:
    print("runtime-profile-missing-tools:" + ",".join(missing_tools))
    raise SystemExit

for tool_name in profile_tool_names:
    if tool_name.startswith("google."):
        print("runtime-profile-google-tool")
        raise SystemExit

print("ok")
PY`
}

func blueclawRootfsBaseContractCheckCommand() string {
	return `set -eu
rootfs_path="/opt/internkim/blueclaw-runtime/rootfs.ext4"
init_dump_path="/tmp/internkim-blueclaw-rootfs-init-check"
passwd_dump_path="/tmp/internkim-blueclaw-rootfs-passwd-check"
group_dump_path="/tmp/internkim-blueclaw-rootfs-group-check"
if [ ! -s "$rootfs_path" ]; then
  echo rootfs-missing
  exit 0
fi
if ! command -v debugfs >/dev/null 2>&1; then
  echo debugfs-missing
  exit 0
fi
rm -f "$init_dump_path"
if ! debugfs -R "dump /sbin/init $init_dump_path" "$rootfs_path" >/tmp/internkim-blueclaw-rootfs-init-check.log 2>&1; then
  echo rootfs-init-dump-failed
  exit 0
fi
if ! debugfs -R "stat /usr/local/bin/marp" "$rootfs_path" >/tmp/internkim-blueclaw-rootfs-marp-check.log 2>&1; then
  echo rootfs-marp-missing
  exit 0
fi
if ! debugfs -R "stat /usr/local/bin/bun" "$rootfs_path" >/tmp/internkim-blueclaw-rootfs-bun-check.log 2>&1; then
  echo rootfs-bun-missing
  exit 0
fi
if ! debugfs -R "stat /usr/local/bin/bunx" "$rootfs_path" >/tmp/internkim-blueclaw-rootfs-bunx-check.log 2>&1; then
  echo rootfs-bunx-missing
  exit 0
fi
if ! debugfs -R "stat /usr/bin/chromium" "$rootfs_path" >/tmp/internkim-blueclaw-rootfs-chromium-check.log 2>&1; then
  echo rootfs-chromium-missing
  exit 0
fi
rm -f "$passwd_dump_path" "$group_dump_path"
if ! debugfs -R "dump /etc/passwd $passwd_dump_path" "$rootfs_path" >/tmp/internkim-blueclaw-rootfs-passwd-check.log 2>&1; then
  echo rootfs-passwd-dump-failed
  exit 0
fi
if ! debugfs -R "dump /etc/group $group_dump_path" "$rootfs_path" >/tmp/internkim-blueclaw-rootfs-group-check.log 2>&1; then
  echo rootfs-group-dump-failed
  exit 0
fi
python3 - "$init_dump_path" "$passwd_dump_path" "$group_dump_path" <<'PY'
from pathlib import Path
import sys

guest_init = Path(sys.argv[1]).read_text()
passwd = Path(sys.argv[2]).read_text()
group = Path(sys.argv[3]).read_text()

for name, marker in {
    "blueclaw-workspace-owner": "chown blueclaw:blueclaw /workspace /workspace/.blueclaw",
    "blueclaw-payload-launch": "/workspace/.blueclaw/runtime/current/bin/blueclaw",
    "blueclaw-runtime-directory": "/workspace/.blueclaw/runtime",
}.items():
    if marker not in guest_init:
        print("rootfs-init-missing-marker:" + name)
        raise SystemExit

if "/usr/local/bin/blueclaw -runtime " in guest_init:
    print("rootfs-blueclaw-init-root-launch")
    raise SystemExit

if "blueclaw:x:998:971:Blueclaw:/workspace:/bin/bash" not in passwd:
    print("rootfs-passwd-missing-blueclaw-user")
    raise SystemExit

if "blueclaw:x:971:" not in group:
    print("rootfs-group-missing-blueclaw-group")
    raise SystemExit

print("ok")
PY`
}

func BlueclawRootfsBaseContractCheckCommand() string {
	return blueclawRootfsBaseContractCheckCommand()
}
