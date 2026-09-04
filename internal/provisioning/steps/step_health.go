package setup

import (
	"errors"
	"fmt"
	"strings"
	"time"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const (
	blueclawRosterArrivalWait  = 3 * time.Minute
	blueclawRosterPollInterval = 5 * time.Second
)

var StepHealth = Step{
	Name: "health",
	Deps: []string{"users-sync"},
	Title: func(context *Context) string {
		return context.T("최종 상태 확인...", "Running final health checks...")
	},
	IsSatisfied: func(context *Context) bool {
		return false
	},
	Run: func(context *Context) error {
		if context.SSH == nil {
			return errors.New("health SSH callback missing")
		}

		var failedChecks []string
		checkService(context, blueclaw.CapabilitydServiceName, &failedChecks)
		checkCapabilitydHealth(context, &failedChecks)
		checkService(context, blueclaw.AdmindServiceName, &failedChecks)
		checkBlueclawGuestRuntime(context, &failedChecks)
		checkBlueclaw(context, &failedChecks)
		checkSecretIsolation(context, &failedChecks)
		checkAdminHealth(context, &failedChecks)
		checkFirstAdminBootstrap(context, &failedChecks)
		checkAgentBrowser(context, &failedChecks)
		checkBlueclawBackupManifest(context, &failedChecks)
		checkBlueclawUsersPolicy(context, &failedChecks)
		checkLLMCapability(context, &failedChecks)
		checkLiteRTCapability(context, &failedChecks)

		if len(failedChecks) > 0 {
			return fmt.Errorf("health check failed: %s", strings.Join(failedChecks, ", "))
		}

		fmt.Println("  " + context.T("최종 상태 정상", "Final health checks passed"))
		return nil
	},
}

func checkFirstAdminBootstrap(context *Context, failedChecks *[]string) {
	check := strings.TrimSpace(context.SSH.Run(`python3 - <<'PY'
import json
from pathlib import Path

path = Path("/root/.internkim/state/admin/first-admin-bootstrap.json")
if not path.exists():
    print("pending")
    raise SystemExit
try:
    document = json.loads(path.read_text())
except Exception:
    print("invalid")
    raise SystemExit
status = document.get("status") or "pending"
if status == "failed":
    print("failed: " + (document.get("error") or "unknown"))
else:
    print(status)
PY`))
	if check == "claimed" {
		fmt.Println("  first admin bootstrap: claimed")
		return
	}
	if strings.HasPrefix(check, "failed: ") || check == "invalid" {
		*failedChecks = append(*failedChecks, "first-admin-bootstrap")
		fmt.Printf("  first admin bootstrap: %s\n", check)
		return
	}
	if check == "" {
		check = "pending"
	}
	fmt.Printf("  first admin bootstrap: %s\n", check)
}

func checkAgentBrowser(context *Context, failedChecks *[]string) {
	check := strings.TrimSpace(context.SSH.Run("(\n" + browserruntime.DeviceReadinessShellScript() + "\n) >/dev/null 2>&1 && echo ok || true"))
	if check == "ok" {
		fmt.Println("  agent-browser: ready")
		return
	}
	fmt.Println("  agent-browser: unavailable")
}

func checkAdminHealth(context *Context, failedChecks *[]string) {
	check := strings.TrimSpace(context.SSH.Run(`curl --silent --show-error --fail http://127.0.0.1:18080/admin/api/health 2>/dev/null | python3 -c 'import json, sys; print(json.load(sys.stdin).get("status", ""))' 2>/dev/null || true`))
	if check == "ok" {
		if !isPlannedStep(context, "web") {
			fmt.Println("  admind: ok")
			return
		}
		adminPage := strings.TrimSpace(context.SSH.Run(`curl --silent --show-error --fail http://127.0.0.1:18080/admin/ 2>/dev/null | grep -o '<script' | head -1 || true`))
		adminAssets := strings.TrimSpace(context.SSH.Run(`curl --silent --show-error --fail http://127.0.0.1:18080/admin/_app/version.json 2>/dev/null | python3 -c 'import json, sys; print(json.load(sys.stdin).get("version", ""))' 2>/dev/null || true`))
		if adminPage == "<script" && adminAssets != "" {
			fmt.Println("  admind: ok")
			return
		}
		*failedChecks = append(*failedChecks, "admin-ui")
		fmt.Println("  admind: admin-ui failed")
		return
	}
	*failedChecks = append(*failedChecks, "admind")
	if check == "" {
		check = "failed"
	}
	fmt.Printf("  admind: %s\n", check)
}

func checkSecretIsolation(context *Context, failedChecks *[]string) {
	runtimeCheck := strings.TrimSpace(context.SSH.Run(blueclawRuntimeContractCheckCommand()))
	if runtimeCheck != "ok" {
		*failedChecks = append(*failedChecks, "runtime-configuration")
		fmt.Printf("  runtime configuration: %s\n", runtimeCheck)
		return
	}
	check := strings.TrimSpace(context.SSH.Run(`if su -s /bin/sh blueclaw -c '` + blueclaw.SecretIsolationShellTest() + `' 2>/dev/null; then
  echo readable
else
  echo ok
fi`))
	if check == "ok" {
		fmt.Println("  secret isolation: ok")
		return
	}
	*failedChecks = append(*failedChecks, "secret-isolation")
	fmt.Printf("  secret isolation: %s\n", check)
}

func checkBlueclawUsersPolicy(context *Context, failedChecks *[]string) {
	checkBlueclawUsersPolicyWithin(context, blueclawRosterArrivalWait, blueclawRosterPollInterval, failedChecks)
}

func checkBlueclawUsersPolicyWithin(context *Context, arrivalWait time.Duration, pollInterval time.Duration, failedChecks *[]string) {
	check := waitForBlueclawUsersPolicy(context, arrivalWait, pollInterval)
	if check == "ok" {
		fmt.Println("  blueclaw users policy: ok")
		return
	}
	*failedChecks = append(*failedChecks, "blueclaw-users-policy")
	fmt.Printf("  blueclaw users policy: failed (%s)\n", check)
}

func waitForBlueclawUsersPolicy(context *Context, arrivalWait time.Duration, pollInterval time.Duration) string {
	deadline := time.Now().Add(arrivalWait)
	for {
		check := strings.TrimSpace(context.SSH.Run(blueclawUsersPolicyCheckCommand()))
		if check == "ok" {
			return check
		}
		if time.Now().Add(pollInterval).After(deadline) {
			return check
		}
		fmt.Printf("  blueclaw users policy: %s, waiting for the roster\n", check)
		time.Sleep(pollInterval)
	}
}

func blueclawUsersPolicyCheckCommand() string {
	return fmt.Sprintf(`policy_response_path="$(mktemp)"
trap 'rm -f "$policy_response_path"' EXIT
if ! curl --silent --show-error --fail %s/admin/api/policy > "$policy_response_path" 2>/dev/null; then
  echo policy-api-unavailable
  exit 0
fi
python3 - "$policy_response_path" <<'PY'
import json
import sys
import urllib.request

state_path = "/root/.internkim/state/users-sync.json"
try:
    with open(state_path) as file:
        desired = set(json.load(file).get("users", []))
except Exception:
    print("ok")
    raise SystemExit

if not desired:
    print("ok")
    raise SystemExit

try:
    with open(sys.argv[1]) as file:
        policy = json.load(file)
except Exception:
    print("policy-api-invalid")
    raise SystemExit(1)

emails = set()
for person in policy.get("people", []):
    for email in person.get("emails", []):
        emails.add(str(email).lower())

missing = sorted(email for email in desired if email.lower() not in emails)
if missing:
    print(",".join(missing))
    sys.exit(1)
print("ok")
PY`, blueclaw.BlueclawBaseURL)
}

func checkBlueclawBackupManifest(context *Context, failedChecks *[]string) {
	check := strings.TrimSpace(context.SSH.Run(`manifest_path="/tmp/internkim-blueclaw-backup-manifest.json"
curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/backup/manifest > "$manifest_path" 2>/dev/null || exit 0
python3 - "$manifest_path" <<'PY'
import json
import sys

try:
    with open(sys.argv[1]) as file:
        manifest = json.load(file)
except Exception:
    print("invalid-json")
    raise SystemExit

document = json.dumps(manifest)
for forbidden in ("token", "Authorization", "passphrase", "OpenRouter", "apiKey", "signingSecret"):
    if forbidden in document:
        print("secret-reference")
        raise SystemExit
if manifest.get("contractVersion") != 1:
    print("contract-version")
    raise SystemExit
if "blueclaw-postgres-dump" not in manifest.get("requiredBackupArtifacts", []):
    print("postgres-artifact")
    raise SystemExit
print("ok")
PY`))
	if check == "ok" {
		fmt.Println("  blueclaw backup manifest: ok")
		return
	}
	*failedChecks = append(*failedChecks, "blueclaw-backup-manifest")
	if check == "" {
		check = "failed"
	}
	fmt.Printf("  blueclaw backup manifest: %s\n", check)
}

func checkLLMCapability(context *Context, failedChecks *[]string) {
	command := strings.ReplaceAll(`model="$(python3 - <<'PY'
import json
try:
    with open("/root/.blueclaw/config/runtime.json") as file:
        document = json.load(file)
    print(document.get("languageModel", {}).get("capability", {}).get("model") or "__DEFAULT_OPENROUTER_MODEL__")
except Exception:
    print("__DEFAULT_OPENROUTER_MODEL__")
PY
)"
text_body="$(MODEL="$model" python3 - <<'PY'
import json
import os
print(json.dumps({
    "model": os.environ["MODEL"],
    "executionMode": "remote",
    "messages": [{"role": "user", "content": "Reply with ok."}],
}))
PY
)"
text_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$text_body" http://internkim/v1/llm/text 2>/tmp/internkim-llm-smoke-error || true)"
if ! printf '%s' "$text_response" | python3 -c 'import json, sys; document=json.load(sys.stdin); raise SystemExit(0 if isinstance(document.get("content"), str) and len(document.get("content")) > 0 else 1)' >/dev/null 2>&1; then
  if [ -n "$text_response" ]; then
    printf '%s' "$text_response" | tr '\n' ' ' | cut -c1-180
  else
    tr '\n' ' ' </tmp/internkim-llm-smoke-error | cut -c1-180
  fi
  exit 0
fi
schema='{"type":"object","properties":{"reply":{"type":"string"}},"required":["reply"],"additionalProperties":false}'
structured_body="$(MODEL="$model" SCHEMA="$schema" python3 - <<'PY'
import json
import os
print(json.dumps({
    "model": os.environ["MODEL"],
    "executionMode": "remote",
    "messages": [{"role": "user", "content": "Return JSON only with reply set to ok."}],
    "structuredOutputSchema": {"name": "smoke_reply", "document": os.environ["SCHEMA"], "isStrictlyEnforced": True},
}))
PY
)"
structured_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$structured_body" http://internkim/v1/llm/structured 2>/tmp/internkim-llm-smoke-error || true)"
if printf '%s' "$structured_response" | python3 -c 'import json, sys; document=json.load(sys.stdin); content=json.loads(document.get("content", "{}")); raise SystemExit(0 if isinstance(content.get("reply"), str) else 1)' >/dev/null 2>&1; then
  echo ok
else
  printf '%s' "$structured_response" | tr '\n' ' ' | cut -c1-180
fi`, "__DEFAULT_OPENROUTER_MODEL__", blueclaw.BlueclawDefaultModelName)
	check := strings.TrimSpace(context.SSH.Run(command))
	if check == "ok" {
		fmt.Println("  llm capability: ok")
		return
	}
	*failedChecks = append(*failedChecks, "llm-capability")
	if check == "" {
		check = "failed"
	}
	fmt.Printf("  llm capability: failed (%s)\n", check)
}

func checkLiteRTCapability(context *Context, failedChecks *[]string) {
	if context.BoardType == BoardSimulation {
		fmt.Println("  local ai capability: not applicable")
		return
	}
	if !isPlannedStep(context, "local-llm") {
		fmt.Println("  local ai capability: skipped")
		return
	}
	check := strings.TrimSpace(context.SSH.Run(`body="$(python3 - <<'PY'
import json
print(json.dumps({
    "model": "local/gemma-4-E4B-it-litert-lm",
    "accelerator": "cpu",
    "executionMode": "device",
    "messages": [{"role": "user", "content": "Reply with ok."}],
    "requireParameters": True,
    "enableResponseHealing": True,
}))
PY
)"
response="$(curl --max-time 210 --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$body" http://internkim/v1/llm/text 2>/tmp/internkim-litert-smoke-error || true)"
if printf '%s' "$response" | python3 -c 'import json, sys; document=json.load(sys.stdin); backend=document.get("selectedBackend"); content=document.get("content"); raise SystemExit(0 if backend in ("gpu", "cpu") and isinstance(content, str) and len(content) > 0 else 1)' >/dev/null 2>&1; then
  echo ok
  exit 0
fi
if [ -n "$response" ]; then
  printf '%s' "$response" | tr '\n' ' ' | cut -c1-180
else
  tr '\n' ' ' </tmp/internkim-litert-smoke-error | cut -c1-180
fi`))
	if check == "ok" {
		fmt.Println("  local ai capability: ok")
		return
	}
	*failedChecks = append(*failedChecks, "local-ai-capability")
	if check == "" {
		check = "failed"
	}
	fmt.Printf("  local ai capability: failed (%s)\n", check)
}

func isPlannedStep(context *Context, stepName string) bool {
	if context.PlannedSteps == nil {
		return true
	}
	return context.PlannedSteps[stepName]
}

func checkService(context *Context, serviceName string, failedChecks *[]string) {
	command := "for attempt in $(seq 1 20); do if [ \"$(systemctl is-active " + serviceName + " 2>/dev/null)\" = active ]; then echo active; exit 0; fi; sleep 1; done; systemctl is-active " + serviceName + " 2>/dev/null || true"
	if strings.TrimSpace(context.SSH.Run(command)) == "active" {
		fmt.Printf("  %s: active\n", serviceName)
		return
	}
	*failedChecks = append(*failedChecks, serviceName)
	fmt.Printf("  %s: failed\n", serviceName)
}

func checkCapabilitydHealth(context *Context, failedChecks *[]string) {
	check := strings.TrimSpace(context.SSH.Run(blueclaw.CapabilitydHealthCheckCommand()))
	if check == "ok" {
		fmt.Println("  capabilityd health: ok")
		return
	}
	*failedChecks = append(*failedChecks, "capabilityd-health")
	fmt.Printf("  capabilityd health: %s\n", check)
}

func checkBlueclaw(context *Context, failedChecks *[]string) {
	if strings.TrimSpace(context.SSH.Run(blueclaw.BlueclawHealthCheckCommand())) == "ok" {
		fmt.Println("  blueclaw: ok")
		return
	}
	*failedChecks = append(*failedChecks, "blueclaw")
	fmt.Println("  blueclaw: failed")
}

func checkBlueclawGuestRuntime(context *Context, failedChecks *[]string) {
	check := strings.TrimSpace(context.SSH.Run(`python3 - <<'PY'
from pathlib import Path

unit = Path("/etc/systemd/system/blueclaw.service")
if not unit.exists():
    print("unit-missing")
    raise SystemExit
text = unit.read_text()
if "blueclaw-supervisor" not in text:
    print("not-supervisor")
    raise SystemExit
if "ExecStart=/usr/local/bin/blueclaw " in text:
    print("direct-blueclaw")
    raise SystemExit
required = [
    "/usr/local/bin/blueclaw-supervisor",
    "/usr/local/bin/cloud-hypervisor",
    "/usr/local/bin/virtiofsd",
    "/opt/internkim/blueclaw-runtime/manifest.json",
    "/opt/internkim/blueclaw-runtime/payload-manifest.json",
    "/opt/internkim/blueclaw-runtime/vmlinux.bin",
    "/opt/internkim/blueclaw-runtime/rootfs.ext4",
    "/var/lib/blueclaw/workspace.ext4",
]
for path in required:
    if not Path(path).exists():
        print("missing:" + path)
        raise SystemExit
print("ok")
PY`))
	if check == "ok" {
		workspaceFilesystem := strings.TrimSpace(context.SSH.Run("blkid -o value -s TYPE /var/lib/blueclaw/workspace.ext4 2>/dev/null || true"))
		if workspaceFilesystem != "ext4" {
			check = "workspace-not-ext4"
		}
	}
	if check == "ok" {
		workspaceSizeCheck := strings.TrimSpace(context.SSH.Run("bytes=$(stat -c '%s' /var/lib/blueclaw/workspace.ext4 2>/dev/null || echo 0); [ \"$bytes\" -ge 34359738368 ] && echo ok || echo workspace-too-small"))
		if workspaceSizeCheck != "ok" {
			check = "workspace-too-small"
		}
	}
	if check == "ok" {
		if !blueclawServiceIsActive(context) {
			check = strings.TrimSpace(context.SSH.Run(blueclawRootfsBaseContractCheckCommand()))
		}
	}
	if check == "ok" {
		fmt.Println("  blueclaw guest runtime: ok")
		return
	}
	*failedChecks = append(*failedChecks, "blueclaw-guest-runtime")
	if check == "" {
		check = "failed"
	}
	fmt.Printf("  blueclaw guest runtime: %s\n", check)
}

func blueclawServiceIsActive(context *Context) bool {
	return strings.TrimSpace(context.SSH.Run("systemctl is-active "+blueclaw.BlueclawServiceName+" 2>/dev/null || true")) == "active"
}
