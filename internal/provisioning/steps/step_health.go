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
		checkBlueclaw(context, &failedChecks)
		checkMattermostPing(context, &failedChecks)
		checkMattermostURL(context, &failedChecks)
		checkMattermostPublic(context, &failedChecks)
		checkBlueclawUsersPolicy(context, &failedChecks)
		checkMattermostProfileLookup(context, &failedChecks)
		checkSlackProfileLookup(context, &failedChecks)

		if len(failedChecks) > 0 {
			return fmt.Errorf("health check failed: %s", strings.Join(failedChecks, ", "))
		}

		fmt.Println("  " + context.T("최종 상태 정상", "Final health checks passed"))
		return nil
	},
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
	check := strings.TrimSpace(context.SSH.Run(`admin_email="$(cat /root/.internkim/admin-email 2>/dev/null || true)"
bot_token="$(cat /root/.internkim/env/bot-token 2>/dev/null || true)"
if [ -z "$admin_email" ] || [ -z "$bot_token" ]; then
  echo missing
  exit 0
fi
encoded_email="$(printf '%s' "$admin_email" | jq -sRr @uri)"
curl -fsS -H "Authorization: Bearer $bot_token" "http://localhost:8065/api/v4/users/email/$encoded_email" 2>/dev/null | jq -e '.email != null' >/dev/null && echo ok || echo failed`))
	if check == "ok" {
		fmt.Println("  mattermost profile lookup: ok")
		return
	}
	*failedChecks = append(*failedChecks, "mattermost-profile-lookup")
	fmt.Printf("  mattermost profile lookup: %s\n", check)
}

func checkSlackProfileLookup(context *Context, failedChecks *[]string) {
	check := strings.TrimSpace(context.SSH.Run(`token="$(cat /root/.internkim/secrets/slack-bot-token 2>/dev/null || true)"
if [ -z "$token" ]; then
  echo skipped
  exit 0
fi
auth="$(curl -fsS -H "Authorization: Bearer $token" https://slack.com/api/auth.test 2>/dev/null || true)"
user_id="$(printf '%s' "$auth" | jq -r 'select(.ok == true) | .user_id // empty' 2>/dev/null)"
if [ -z "$user_id" ]; then
  echo failed
  exit 0
fi
curl -fsS -H "Authorization: Bearer $token" "https://slack.com/api/users.info?user=$user_id" 2>/dev/null | jq -e '.ok == true and .user.id != null' >/dev/null && echo ok || echo failed`))
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
