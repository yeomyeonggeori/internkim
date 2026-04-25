package setup

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/anthropic-lab/internkim/internal/runtime/blueclaw"
)

var mattermostPublicURLPattern = regexp.MustCompile(`^https://[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.intern\.kim$`)

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
		checkService(context, "mattermost", &failedChecks)
		checkService(context, "cloudflared", &failedChecks)
		checkService(context, blueclaw.CapabilitydServiceName, &failedChecks)
		checkBlueclaw(context, &failedChecks)
		checkSecretIsolation(context, &failedChecks)
		checkCapabilityHealth(context, &failedChecks)
		checkMattermostPing(context, &failedChecks)
		checkMattermostURL(context, &failedChecks)
		checkMattermostPublic(context, &failedChecks)
		checkBlueclawUsersPolicy(context, &failedChecks)
		checkMattermostProfileLookup(context, &failedChecks)
		checkLLMCapability(context, &failedChecks)
		checkLiteRTCapability(context, &failedChecks)
		checkSlackProfileLookup(context, &failedChecks)

		if len(failedChecks) > 0 {
			return fmt.Errorf("health check failed: %s", strings.Join(failedChecks, ", "))
		}

		fmt.Println("  " + context.T("최종 상태 정상", "Final health checks passed"))
		return nil
	},
}

func checkSecretIsolation(context *Context, failedChecks *[]string) {
	check := strings.TrimSpace(context.SSH.Run(`runtime_check="$(python3 - <<'PY'
import json

try:
    with open("/root/.blueclaw/config/runtime.json") as file:
        runtime_configuration = json.load(file)
except Exception:
    print("runtime-config")
    raise SystemExit

document = json.dumps(runtime_configuration)
for forbidden in ("apiKeyPath", "botTokenPath", "signingSecretPath", "OPENROUTER_API_KEY", "wrapperPath", "modelPath", "backend"):
    if forbidden in document:
        print("runtime-secret-reference")
        raise SystemExit

print("ok")
PY
)"
if [ "$runtime_check" != "ok" ]; then
  echo "$runtime_check"
  exit 0
fi
if su -s /bin/sh blueclaw -c 'test -r /root/.internkim/secrets/openrouter-api-key || test -r /root/.internkim/secrets/mattermost-bot-token || test -r /root/.internkim/secrets/slack-bot-token || test -r /root/.internkim/secrets/device-secret || test -r /root/.internkim/secrets/google-sa.json || test -r /root/.internkim/secrets/gas-webhook-url || test -r /root/.internkim/models/gemma-4-E4B-it.litertlm' 2>/dev/null; then
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

func checkCapabilityHealth(context *Context, failedChecks *[]string) {
	check := strings.TrimSpace(context.SSH.Run(`curl --silent --show-error --fail --unix-socket /run/internkim/capability.sock http://internkim/health 2>/dev/null || true`))
	if check == "ok" {
		fmt.Println("  capabilityd: ok")
		return
	}
	*failedChecks = append(*failedChecks, "capabilityd")
	if check == "" {
		check = "failed"
	}
	fmt.Printf("  capabilityd: %s\n", check)
}

func checkBlueclawUsersPolicy(context *Context, failedChecks *[]string) {
	check := strings.TrimSpace(context.SSH.Run(`python3 - <<'PY'
import json
import sys

state_path = "/root/.internkim/state/users-sync.json"
policy_path = "/root/.blueclaw/config/policy.json"
try:
    with open(state_path) as file:
        desired = set(json.load(file).get("users", []))
    with open(policy_path) as file:
        policy = json.load(file)
except Exception:
    sys.exit(1)

emails = set()
for person in policy.get("people", []):
    for email in person.get("emails", []):
        emails.add(str(email).lower())

missing = sorted(email for email in desired if email.lower() not in emails)
if missing:
    print(",".join(missing))
    sys.exit(1)
print("ok")
PY`))
	if check == "ok" {
		fmt.Println("  blueclaw users policy: ok")
		return
	}
	*failedChecks = append(*failedChecks, "blueclaw-users-policy")
	fmt.Printf("  blueclaw users policy: failed (%s)\n", check)
}

func checkMattermostProfileLookup(context *Context, failedChecks *[]string) {
	check := strings.TrimSpace(context.SSH.Run(`bot_body="$(jq -cn '{}')"
bot_user_id="$(curl -fsS --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$bot_body" http://internkim/v1/platform/mattermost/bot.resolve 2>/dev/null | jq -r '.userID // empty')"
if [ -z "$bot_user_id" ]; then
  echo missing
  exit 0
fi
body="$(jq -cn --arg externalUserID "$bot_user_id" '{externalUserID:$externalUserID}')"
curl -fsS --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$body" http://internkim/v1/platform/mattermost/identity.resolve 2>/dev/null | jq -e '.email != null' >/dev/null && echo ok || echo failed`))
	if check == "ok" {
		fmt.Println("  mattermost profile lookup: ok")
		return
	}
	*failedChecks = append(*failedChecks, "mattermost-profile-lookup")
	fmt.Printf("  mattermost profile lookup: %s\n", check)
}

func checkLLMCapability(context *Context, failedChecks *[]string) {
	check := strings.TrimSpace(context.SSH.Run(`model="$(jq -r '.languageModel.capability.model // "google/gemini-3-flash-preview"' /root/.blueclaw/config/runtime.json 2>/dev/null)"
schema='{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}'
body="$(jq -cn --arg model "$model" --arg schema "$schema" '{
  model: $model,
  executionMode: "remote",
  messages: [{role:"user", content:"Return JSON only with content set to ok."}],
  structuredOutputSchema: {name:"plain_text_response", document:$schema, isStrictlyEnforced:true},
  requireParameters: true,
  enableResponseHealing: true
}')"
response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$body" http://internkim/v1/llm/structured 2>/tmp/internkim-llm-smoke-error || true)"
if printf '%s' "$response" | jq -e '.content | fromjson | .content | type == "string"' >/dev/null 2>&1; then
  echo ok
  exit 0
fi
if [ -n "$response" ]; then
  printf '%s' "$response" | tr '\n' ' ' | cut -c1-180
else
  tr '\n' ' ' </tmp/internkim-llm-smoke-error | cut -c1-180
fi`))
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
	check := strings.TrimSpace(context.SSH.Run(`if ! command -v litert-lm >/dev/null 2>&1 || [ ! -s /root/.internkim/models/gemma-4-E4B-it.litertlm ]; then
  echo skipped
  exit 0
fi
schema='{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}'
body="$(jq -cn --arg schema "$schema" '{
  model: "local/gemma-4-E4B-it-litert-lm",
  executionMode: "local",
  messages: [{role:"user", content:"Return JSON only with content set to ok."}],
  structuredOutputSchema: {name:"plain_text_response", document:$schema, isStrictlyEnforced:true},
  requireParameters: true,
  enableResponseHealing: true
}')"
response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$body" http://internkim/v1/llm/structured 2>/tmp/internkim-litert-smoke-error || true)"
if printf '%s' "$response" | jq -e '.selectedBackend as $backend | ($backend == "gpu" or $backend == "cpu") and (.content | fromjson | .content | type == "string")' >/dev/null 2>&1; then
  echo ok
  exit 0
fi
if [ -n "$response" ]; then
  printf '%s' "$response" | tr '\n' ' ' | cut -c1-180
else
  tr '\n' ' ' </tmp/internkim-litert-smoke-error | cut -c1-180
fi`))
	if check == "skipped" {
		fmt.Println("  litert capability: skipped")
		return
	}
	if check == "ok" {
		fmt.Println("  litert capability: ok")
		return
	}
	*failedChecks = append(*failedChecks, "litert-capability")
	if check == "" {
		check = "failed"
	}
	fmt.Printf("  litert capability: failed (%s)\n", check)
}

func checkSlackProfileLookup(context *Context, failedChecks *[]string) {
	check := strings.TrimSpace(context.SSH.Run(`if [ ! -f /root/.internkim/secrets/slack-bot-token ]; then
  echo skipped
  exit 0
fi
auth_body="$(jq -cn '{}')"
user_id="$(curl -fsS --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$auth_body" http://internkim/v1/platform/slack/bot.resolve 2>/dev/null | jq -r '.userID // empty')"
if [ -z "$user_id" ]; then
  echo failed
  exit 0
fi
lookup_body="$(jq -cn --arg externalUserID "$user_id" '{externalUserID:$externalUserID}')"
curl -fsS --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$lookup_body" http://internkim/v1/platform/slack/identity.resolve 2>/dev/null | jq -e '.externalUserID != null' >/dev/null && echo ok || echo failed`))
	if check == "skipped" {
		fmt.Println("  slack profile lookup: skipped")
		return
	}
	if check == "ok" {
		fmt.Println("  slack profile lookup: ok")
		return
	}
	*failedChecks = append(*failedChecks, "slack-profile-lookup")
	fmt.Printf("  slack profile lookup: %s\n", check)
}

func checkService(context *Context, serviceName string, failedChecks *[]string) {
	if strings.TrimSpace(context.SSH.Run("systemctl is-active "+serviceName+" 2>/dev/null")) == "active" {
		fmt.Printf("  %s: active\n", serviceName)
		return
	}
	*failedChecks = append(*failedChecks, serviceName)
	fmt.Printf("  %s: failed\n", serviceName)
}

func checkBlueclaw(context *Context, failedChecks *[]string) {
	if strings.TrimSpace(context.SSH.Run(blueclaw.BlueclawHealthCheckCommand())) == "ok" {
		fmt.Println("  blueclaw: ok")
		return
	}
	*failedChecks = append(*failedChecks, "blueclaw")
	fmt.Println("  blueclaw: failed")
}

func checkMattermostPing(context *Context, failedChecks *[]string) {
	ping := strings.TrimSpace(context.SSH.Run(`curl -sf http://localhost:8065/api/v4/system/ping 2>/dev/null | grep -o '"status":"OK"' || true`))
	if ping != "" {
		fmt.Println("  mattermost ping: ok")
		return
	}
	*failedChecks = append(*failedChecks, "mattermost-ping")
	fmt.Println("  mattermost ping: failed")
}

func checkMattermostPublic(context *Context, failedChecks *[]string) {
	mattermostURL := strings.TrimSpace(context.SSH.Run("cat /root/.internkim/env/mattermost-url 2>/dev/null"))
	if mattermostURL == "" {
		*failedChecks = append(*failedChecks, "mattermost-public")
		fmt.Println("  mattermost public: missing url")
		return
	}

	statusCode := strings.TrimSpace(context.SSH.Run(
		"curl -sS --max-time 20 --output /dev/null --write-out '%{http_code}' " + shellQuote(mattermostURL) + " 2>/dev/null || true",
	))
	switch statusCode {
	case "200", "302", "401", "403":
		fmt.Printf("  mattermost public: %s\n", statusCode)
	default:
		*failedChecks = append(*failedChecks, "mattermost-public")
		fmt.Printf("  mattermost public: failed (%s)\n", statusCode)
	}
}

func checkMattermostURL(context *Context, failedChecks *[]string) {
	mattermostURL := strings.TrimSpace(context.SSH.Run("cat /root/.internkim/env/mattermost-url 2>/dev/null"))
	if mattermostPublicURLPattern.MatchString(mattermostURL) {
		fmt.Printf("  mattermost url: %s\n", mattermostURL)
		return
	}
	*failedChecks = append(*failedChecks, "mattermost-url")
	fmt.Printf("  mattermost url: invalid (%s)\n", mattermostURL)
}
