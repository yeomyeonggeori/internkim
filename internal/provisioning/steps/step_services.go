package setup

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
)

var blueclawServiceHealthAttempts = 360
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
			sdkdServiceIsReady(context) &&
			trimmedRun(context, "systemctl is-active "+blueclaw.AdmindServiceName) == "active" &&
			trimmedRun(context, "systemctl is-active "+blueclaw.GraphitiMemorydServiceName) == "active" &&
			localLLMServiceUnitsAreReady(context) &&
			trimmedRun(context, blueclaw.BlueclawHealthCheckCommand()) == "ok" &&
			trimmedRun(context, blueclaw.CapabilitydHealthCheckCommand()) == "ok" &&
			trimmedRun(context, blueclaw.GraphitiMemorydHealthCheckCommand()) == "ok" &&
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

		connection.Run(blueclawHostNetworkDependencyInstallCommand())

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

		connection.Run(`mkdir -p /root/.internkim/secrets
if [ ! -s /root/.internkim/secrets/sdkd-auth-key ]; then
  umask 077
  head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' > /root/.internkim/secrets/sdkd-auth-key
fi
chown root:root /root/.internkim/secrets/sdkd-auth-key
chmod 600 /root/.internkim/secrets/sdkd-auth-key
install -d -o root -g root -m 700 ` + blueclaw.SDKDServiceCredentialDirectoryPath + `
install -o root -g root -m 600 /root/.internkim/secrets/sdkd-auth-key ` + blueclaw.SDKDServiceAuthKeyPath + `
if [ -s /root/.internkim/secrets/openrouter-api-key ]; then
  install -o root -g root -m 600 /root/.internkim/secrets/openrouter-api-key ` + blueclaw.SDKDServiceOpenRouterKeyPath + `
else
  rm -f ` + blueclaw.SDKDServiceOpenRouterKeyPath + `
fi`)

		connection.Run(`cd /root/.blueclaw/workspace/skills 2>/dev/null && \
rm -rf agent-browser github summarize skill-creator 2>/dev/null; \
echo "Cleaned unavailable skills"`)

		connection.Run(`mkdir -p /root/.blueclaw/workspace/.blueclaw/postgres /root/.blueclaw/workspace/.blueclaw/graphiti /root/.blueclaw/workspace/.blueclaw/logs /root/.blueclaw/workspace/.blueclaw/blobs
chown -R blueclaw:blueclaw /root/.blueclaw/workspace/.blueclaw
chmod -R u=rwX,g=rwX,o= /root/.blueclaw/workspace/.blueclaw`)

		connection.Run(serviceUnitInstallCommand(context))

		isBlueclawHealthy := false
		for attempt := 0; attempt < blueclawServiceHealthAttempts; attempt++ {
			isBlueclawHealthy = blueclawServicesAreHealthy(context)
			if isBlueclawHealthy {
				break
			}
			time.Sleep(blueclawServiceHealthRetryDelay)
		}

		if isBlueclawHealthy {
			fmt.Println("  " + context.T("blueclaw 실행 중", "blueclaw running"))
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

		if deviceURL := context.Callbacks.LoadState("device_url"); deviceURL != "" && shouldReconcileMattermostSiteURL(context) {
			connection.Run(mattermostSiteURLReconcileCommand(deviceURL))
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

func blueclawHostNetworkDependencyInstallCommand() string {
	return `set -euo pipefail
if ! command -v ip >/dev/null 2>&1 || ! command -v iptables >/dev/null 2>&1 || ! command -v sysctl >/dev/null 2>&1 || ! command -v jq >/dev/null 2>&1 || ! command -v git >/dev/null 2>&1; then
  apt-get update -qq >/dev/null 2>&1 || true
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq iproute2 iptables procps jq git >/dev/null
fi
command -v ip >/dev/null
command -v iptables >/dev/null
command -v sysctl >/dev/null
command -v jq >/dev/null
command -v git >/dev/null`
}

func shouldReconcileMattermostSiteURL(context *Context) bool {
	return context != nil && context.PlannedSteps["mattermost"]
}

func mattermostSiteURLReconcileCommand(deviceURL string) string {
	quotedDeviceURL := shellQuote(deviceURL)
	return `set -e
configuration_path=/opt/mattermost/config/config.json
if [ -f "$configuration_path" ]; then
  current_url="$(jq -r '.ServiceSettings.SiteURL // .SiteURL // ""' "$configuration_path" 2>/dev/null || true)"
  if [ "$current_url" != ` + quotedDeviceURL + ` ]; then
    temporary_path="$(mktemp)"
    jq --arg siteURL ` + quotedDeviceURL + ` '.ServiceSettings = ((.ServiceSettings // {}) + {"SiteURL": $siteURL})' "$configuration_path" > "$temporary_path"
    cat "$temporary_path" > "$configuration_path"
    rm -f "$temporary_path"
    systemctl restart mattermost 2>/dev/null || true
  fi
fi`
}

func sdkdServiceIsReady(context *Context) bool {
	if context.BoardType == BoardSimulation {
		return true
	}
	return trimmedRun(context, "systemctl is-active "+blueclaw.SDKDServiceName) == "active" &&
		trimmedRun(context, blueclaw.SDKDHealthCheckCommand()) == "ok"
}

func localLLMServiceUnitsAreSatisfied(context *Context) bool {
	if context.BoardType == BoardSimulation {
		return true
	}
	return strings.Contains(trimmedRun(context, "systemctl cat "+locallm.LlamaCppServiceName+" 2>/dev/null"), locallm.LlamaCppBinaryPath) &&
		strings.Contains(trimmedRun(context, "systemctl cat "+locallm.LlamaCppEmbeddingServiceName+" 2>/dev/null"), locallm.LlamaCppEmbeddingModelPath)
}

func localLLMServiceUnitsAreReady(context *Context) bool {
	if !shouldManageLocalLLMServices(context) {
		return true
	}
	return localLLMServiceUnitsAreSatisfied(context)
}

func serviceUnitInstallCommand(context *Context) string {
	services := serviceUnitDocuments(context)
	serviceNames := enabledServiceNames(context)
	var command strings.Builder
	command.WriteString(`systemctl stop zeroclaw 2>/dev/null || true
systemctl disable zeroclaw 2>/dev/null || true
rm -f /etc/systemd/system/zeroclaw.service
rm -rf /etc/systemd/system/zeroclaw.service.d
systemctl enable systemd-time-wait-sync.service 2>/dev/null
`)
	for _, service := range services {
		fmt.Fprintf(&command, "cat > %s <<'SERVICEEOF'\n%sSERVICEEOF\n", service.path, service.document)
	}
	command.WriteString("systemctl daemon-reload\n")
	for _, serviceName := range disabledServiceNames(context) {
		fmt.Fprintf(&command, "systemctl disable %s 2>/dev/null || true\n", serviceName)
	}
	for _, serviceName := range serviceNames {
		fmt.Fprintf(&command, "systemctl enable %s\nsystemctl restart %s\n", serviceName, serviceName)
	}
	command.WriteString("sleep 2")
	return command.String()
}

func serviceUnitDocuments(context *Context) []serviceUnitDocument {
	services := []serviceUnitDocument{
		{path: blueclaw.BlueclawServicePath, document: blueclaw.BlueclawServiceUnit()},
		{path: blueclaw.CapabilitydServicePath, document: capabilitydServiceUnitForContext(context)},
		{path: blueclaw.AdmindServicePath, document: blueclaw.AdmindServiceUnit()},
		{path: blueclaw.SDKDServicePath, document: blueclaw.SDKDServiceUnit()},
	}
	if context.BoardType == BoardSimulation {
		return services[:3]
	}
	if !shouldManageLocalLLMServices(context) {
		return services
	}
	services = append(services, serviceUnitDocument{path: blueclaw.GraphitiMemorydServicePath, document: blueclaw.GraphitiMemorydServiceUnit()})
	return append(services,
		serviceUnitDocument{path: locallm.LlamaCppServicePath, document: blueclaw.LlamaCppServiceUnit()},
		serviceUnitDocument{path: locallm.LlamaCppEmbeddingServicePath, document: blueclaw.LlamaCppEmbeddingServiceUnit()},
	)
}

func capabilitydServiceUnitForContext(context *Context) string {
	if context.BoardType == BoardCloudShared {
		return blueclaw.CapabilitydServiceUnitForLocalInferenceMode("remote")
	}
	return blueclaw.CapabilitydServiceUnit()
}

func enabledServiceNames(context *Context) []string {
	serviceNames := []string{
		blueclaw.SDKDServiceName,
		blueclaw.CapabilitydServiceName,
		blueclaw.AdmindServiceName,
		blueclaw.BlueclawServiceName,
	}
	if context.BoardType == BoardSimulation {
		return serviceNames[1:]
	}
	if !shouldManageLocalLLMServices(context) {
		return serviceNames
	}
	return append([]string{locallm.LlamaCppServiceName, locallm.LlamaCppEmbeddingServiceName, blueclaw.GraphitiMemorydServiceName}, serviceNames...)
}

func disabledServiceNames(context *Context) []string {
	return nil
}

func shouldManageLocalLLMServices(context *Context) bool {
	return context.BoardType != BoardSimulation && context.PlannedSteps["local-llm"]
}

func blueclawServicesAreHealthy(context *Context) bool {
	report := readBlueclawServiceHealthReport(context)
	if report["blueclaw"] != "active" {
		return false
	}
	if report["capabilityd"] != "active" {
		return false
	}
	if context.BoardType != BoardSimulation && report["sdkd"] != "active" {
		return false
	}
	if report["admind"] != "active" {
		return false
	}
	if report["blueclawHealth"] != "ok" {
		return false
	}
	if report["capabilitydHealth"] != "ok" {
		return false
	}
	if context.BoardType != BoardSimulation && report["sdkdHealth"] != "ok" {
		return false
	}
	if context.BoardType == BoardSimulation {
		return true
	}
	if !shouldManageLocalLLMServices(context) {
		return true
	}
	if report["graphiti"] != "active" {
		return false
	}
	if report["graphitiHealth"] != "ok" {
		return false
	}
	return report["embedding"] == "active"
}

type serviceUnitDocument struct {
	path     string
	document string
}

func readBlueclawServiceHealthReport(context *Context) map[string]string {
	output := trimmedRun(context, blueclawServiceHealthReportCommand(context))
	report := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if found {
			report[key] = value
		}
	}
	return report
}

func blueclawServiceHealthReportCommand(context *Context) string {
	checks := []serviceHealthCheck{
		{name: "blueclaw", command: "systemctl is-active " + blueclaw.BlueclawServiceName + " 2>/dev/null"},
		{name: "capabilityd", command: "systemctl is-active " + blueclaw.CapabilitydServiceName + " 2>/dev/null"},
		{name: "admind", command: "systemctl is-active " + blueclaw.AdmindServiceName + " 2>/dev/null"},
		{name: "blueclawHealth", command: blueclaw.BlueclawHealthCheckCommand()},
		{name: "capabilitydHealth", command: blueclaw.CapabilitydHealthCheckCommand()},
	}
	if context.BoardType != BoardSimulation {
		checks = append(checks,
			serviceHealthCheck{name: "sdkd", command: "systemctl is-active " + blueclaw.SDKDServiceName + " 2>/dev/null"},
			serviceHealthCheck{name: "sdkdHealth", command: blueclaw.SDKDHealthCheckCommand()},
		)
	}
	if shouldManageLocalLLMServices(context) {
		checks = append(checks,
			serviceHealthCheck{name: "graphiti", command: "systemctl is-active " + blueclaw.GraphitiMemorydServiceName + " 2>/dev/null"},
			serviceHealthCheck{name: "graphitiHealth", command: blueclaw.GraphitiMemorydHealthCheckCommand()},
		)
		checks = append(checks, serviceHealthCheck{name: "embedding", command: "systemctl is-active " + locallm.LlamaCppEmbeddingServiceName + " 2>/dev/null"})
	}

	var command strings.Builder
	for _, check := range checks {
		fmt.Fprintf(&command, "printf '%%s=' %s; (%s) 2>/dev/null || true\n", shellQuote(check.name), check.command)
	}
	return command.String()
}

type serviceHealthCheck struct {
	name    string
	command string
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

forbidden_keys = {"apiKeyPath", "botTokenPath", "signingSecretPath", "OPENROUTER_API_KEY", "wrapperPath", "modelPath", "backend", "defaultBudgetClass"}

def contains_forbidden_key(value):
    if isinstance(value, dict):
        for key, nested in value.items():
            if key in forbidden_keys or contains_forbidden_key(nested):
                return True
        return False
    if isinstance(value, list):
        for item in value:
            if contains_forbidden_key(item):
                return True
        return False
    return False

if contains_forbidden_key(runtime_configuration):
    print("legacy-runtime-config")
    raise SystemExit

agent = runtime_configuration.get("agent", {})
if agent.get("defaultTaskLevel") != "low":
    print("runtime-task-level")
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
if database.get("connectionString") != "user=blueclaw dbname=blueclaw host=/workspace/.blueclaw/postgres sslmode=disable":
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

firecracker = runtime_configuration.get("firecracker", {})
outbound_network = firecracker.get("outboundNetwork", {})
if outbound_network.get("enabled") is not True:
    print("runtime-outbound-network-disabled")
    raise SystemExit
if outbound_network.get("hostDeviceName") != "bctap0":
    print("runtime-outbound-network-device")
    raise SystemExit
if outbound_network.get("networkCIDR") != "172.31.0.0/30":
    print("runtime-outbound-network-cidr")
    raise SystemExit
if outbound_network.get("guestGateway") != "172.31.0.1":
    print("runtime-outbound-network-gateway")
    raise SystemExit

profile_tool_names = []
for profile in runtime_configuration.get("agentProfiles", []):
    if profile.get("name") == "default":
        profile_tool_names = [str(tool_name) for tool_name in profile.get("allowedToolNames", [])]
        break

mandatory_profile_tools = {"terminal.run", "ask.input", "ask.confirm", "file.deliver", "skill.search", "file.read", "file.write", "file.edit", "file.patch", "file.preview", "image.read"}
missing_tools = sorted(mandatory_profile_tools - set(profile_tool_names))
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
mount_path="$(mktemp -d /tmp/internkim-blueclaw-rootfs-check.XXXXXX)"
run_contract_check_command() {
  timeout_seconds="$1"
  shift
  if command -v timeout >/dev/null 2>&1; then
    timeout "$timeout_seconds" "$@"
  else
    "$@"
  fi
}
cleanup_rootfs_check() {
  if mountpoint -q "$mount_path"; then
    umount "$mount_path" 2>/dev/null || true
  fi
  rmdir "$mount_path" 2>/dev/null || true
}
trap cleanup_rootfs_check EXIT
if [ ! -s "$rootfs_path" ]; then
  echo rootfs-missing
  exit 0
fi
if ! run_contract_check_command 20s mount -o loop,ro,noload "$rootfs_path" "$mount_path" >/tmp/internkim-blueclaw-rootfs-mount-check.log 2>&1; then
  echo rootfs-mount-failed
  exit 0
fi
if [ ! -x "$mount_path/usr/local/bin/marp" ]; then
  echo rootfs-marp-missing
  exit 0
fi
if [ ! -x "$mount_path/usr/local/bin/bun" ]; then
  echo rootfs-bun-missing
  exit 0
fi
if [ ! -x "$mount_path/usr/local/bin/bunx" ]; then
  echo rootfs-bunx-missing
  exit 0
fi
if [ ! -x "$mount_path/usr/local/bin/uv" ]; then
  echo rootfs-uv-missing
  exit 0
fi
for managed_executable in marp bun bunx uv; do
  managed_path="$mount_path/usr/local/bin/$managed_executable"
  managed_stat_path="$managed_path"
  if [ -L "$managed_path" ]; then
    managed_target="$(readlink "$managed_path" 2>/dev/null || true)"
    case "$managed_target" in
      /*) managed_stat_path="$mount_path$managed_target" ;;
      *) managed_stat_path="$(dirname "$managed_path")/$managed_target" ;;
    esac
  fi
  managed_owner="$(stat -c '%u' "$managed_stat_path" 2>/dev/null || echo missing)"
  managed_mode="$(stat -c '%a' "$managed_stat_path" 2>/dev/null || echo missing)"
  case "$managed_owner" in
    0|998) ;;
    *) echo "rootfs-$managed_executable-owner-drift"; exit 0 ;;
  esac
  case "$managed_mode" in
    555|755) ;;
    *) echo "rootfs-$managed_executable-mode-drift"; exit 0 ;;
  esac
done
if [ ! -x "$mount_path/opt/blueclaw/builtin-skills-venv/bin/python" ]; then
  echo rootfs-builtin-skills-python-missing
  exit 0
fi
if [ ! -x "$mount_path/usr/bin/bc" ]; then
  echo rootfs-bc-missing
  exit 0
fi
if [ ! -x "$mount_path/usr/bin/chromium" ]; then
  echo rootfs-chromium-missing
  exit 0
fi
if [ ! -u "$mount_path/usr/local/bin/blueclaw-posix-helper" ]; then
  echo rootfs-posix-helper-missing
  exit 0
fi
if [ "$(stat -c '%u:%g:%a' "$mount_path/usr/local/bin/blueclaw-posix-helper")" != "0:0:4755" ]; then
  echo rootfs-posix-helper-mode-drift
  exit 0
fi
if ! run_contract_check_command 5s "$mount_path/usr/local/bin/blueclaw-posix-helper" capabilities | python3 -c 'import json, sys; document=json.load(sys.stdin); sys.exit(0 if document.get("version", 0) >= 2 and "fs" in document.get("capabilities", []) else 1)'; then
  echo rootfs-posix-helper-fs-capability-missing
  exit 0
fi
python3 - "$mount_path/sbin/init" "$mount_path/etc/passwd" "$mount_path/etc/group" <<'PY'
from pathlib import Path
import sys

guest_init = Path(sys.argv[1]).read_text()
passwd = Path(sys.argv[2]).read_text()
group = Path(sys.argv[3]).read_text()

for name, marker in {
    "blueclaw-workspace-owner": "chown blueclaw:blueclaw /workspace /workspace/.blueclaw",
    "blueclaw-payload-launch": "/workspace/.blueclaw/runtime/current/bin/blueclaw",
    "blueclaw-runtime-directory": "/workspace/.blueclaw/runtime",
    "blueclaw-posix-sync": "blueclaw-posix-helper sync",
    "blueclaw-posix-helper-preflight": "posix helper is not executable by blueclaw",
    "blueclaw-outbound-network": "configure_outbound_network",
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
