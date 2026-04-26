package cli

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	internkimlab "github.com/anthropic-lab/internkim/internal/lab"
)

type verifyTarget struct {
	host       string
	user       string
	password   string
	scriptDir  string
	stateDir   string
	sshpassBin string
	sshClient  *sshClient
}

func runVerify() {
	if errorValue := runVerifyArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runVerifyArguments(arguments []string) error {
	subcommand := "api"
	if len(arguments) > 0 && !strings.HasPrefix(arguments[0], "-") {
		subcommand = arguments[0]
		arguments = arguments[1:]
	}

	switch subcommand {
	case "api":
		return runVerifyAPI(arguments)
	case "mattermost":
		return runVerifyMattermost(arguments)
	case "browser":
		return runVerifyBrowser(arguments)
	default:
		return fmt.Errorf("unknown verify subcommand: %s", subcommand)
	}
}

func runVerifyAPI(arguments []string) error {
	verifyTarget, errorValue := resolveVerifyTarget(arguments)
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("verify api: %s@%s\n", verifyTarget.user, verifyTarget.host)
	return verifyTarget.runRemoteVerification(verifyAPIScript())
}

func runVerifyMattermost(arguments []string) error {
	verifyTarget, errorValue := resolveVerifyTarget(arguments)
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("verify mattermost: %s@%s\n", verifyTarget.user, verifyTarget.host)
	return verifyTarget.runRemoteVerification(verifyMattermostScript())
}

func runVerifyBrowser(arguments []string) error {
	flagSet := flag.NewFlagSet("verify browser", flag.ContinueOnError)
	localMode := flagSet.Bool("local", false, "Run local Mattermost browser smoke test")
	publicMode := flagSet.Bool("public", false, "Run public URL browser smoke test")
	host := flagSet.String("host", "", "Board host")
	user := flagSet.String("user", boardUser, "SSH user")
	password := flagSet.String("password", "", "SSH password")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}
	if !*localMode && !*publicMode {
		*localMode = true
	}

	targetArguments := []string{"--host", *host, "--user", *user, "--password", *password}
	verifyTarget, errorValue := resolveVerifyTarget(targetArguments)
	if errorValue != nil {
		return errorValue
	}
	if *localMode {
		if errorValue := runLocalBrowserVerification(verifyTarget); errorValue != nil {
			return errorValue
		}
	}
	if *publicMode {
		if errorValue := runPublicBrowserVerification(verifyTarget); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func resolveVerifyTarget(arguments []string) (verifyTarget, error) {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return verifyTarget{}, errorValue
	}

	flagSet := flag.NewFlagSet("verify", flag.ContinueOnError)
	host := flagSet.String("host", "", "Board host")
	user := flagSet.String("user", boardUser, "SSH user")
	password := flagSet.String("password", "", "SSH password")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return verifyTarget{}, errorValue
	}

	stateDir := internkimHomeDir()
	sshpassBin := filepath.Join(repositoryRootPath, "bin", "sshpass")
	if strings.TrimSpace(*host) == "" {
		labHost := resolveLabVirtualMachineIPAddress()
		if labHost != "" {
			configurationPath := internkimlab.DefaultConfigurationPath(repositoryRootPath)
			configuration, loadError := internkimlab.LoadConfiguration(configurationPath)
			if loadError == nil {
				*host = labHost
				*user = configuration.VirtualMachine.SSHUsername
				*password = configuration.VirtualMachine.SSHPassword
			}
		}
	}
	if strings.TrimSpace(*host) == "" {
		*host = findBoardIP(sshpassBin, stateDir)
	}
	if strings.TrimSpace(*host) == "" {
		return verifyTarget{}, errors.New("verify target not found; pass --host <ip>")
	}

	sshClient := newSSH(sshpassBin, *user, *password, *host)
	return verifyTarget{
		host:       *host,
		user:       *user,
		password:   *password,
		scriptDir:  repositoryRootPath,
		stateDir:   stateDir,
		sshpassBin: sshpassBin,
		sshClient:  sshClient,
	}, nil
}

func (target verifyTarget) runRemoteVerification(script string) error {
	output, errorValue := target.sshClient.runResult(script)
	if strings.TrimSpace(output) != "" {
		fmt.Print(output)
		if !strings.HasSuffix(output, "\n") {
			fmt.Println()
		}
	}
	if errorValue != nil {
		return fmt.Errorf("remote verification failed: %w", errorValue)
	}
	return nil
}

func runLocalBrowserVerification(target verifyTarget) error {
	port, errorValue := reserveLocalPort()
	if errorValue != nil {
		return errorValue
	}

	tunnelCommand := buildMattermostTunnelCommand(target, port)
	tunnelCommand.Stdout = os.Stdout
	tunnelCommand.Stderr = os.Stderr
	if errorValue := tunnelCommand.Start(); errorValue != nil {
		return errorValue
	}
	defer func() {
		_ = tunnelCommand.Process.Kill()
		_, _ = tunnelCommand.Process.Wait()
	}()
	time.Sleep(1500 * time.Millisecond)

	adminEmail := strings.TrimSpace(target.sshClient.run("cat /root/.internkim/admin-email 2>/dev/null"))
	adminPassword := strings.TrimSpace(target.sshClient.run("cat /root/.internkim/secrets/mm-admin-pass 2>/dev/null"))
	return runPlaywright("tests/e2e/mattermost.spec.ts", map[string]string{
		"INTERNKIM_MATTERMOST_URL": fmt.Sprintf("http://127.0.0.1:%d", port),
		"INTERNKIM_ADMIN_EMAIL":    adminEmail,
		"INTERNKIM_ADMIN_PASSWORD": adminPassword,
	})
}

func runPublicBrowserVerification(target verifyTarget) error {
	publicURL := strings.TrimSpace(target.sshClient.run("cat /root/.internkim/env/mattermost-url 2>/dev/null"))
	if publicURL == "" {
		return errors.New("public Mattermost URL is empty")
	}
	return runPlaywright("tests/e2e/public-url.spec.ts", map[string]string{
		"INTERNKIM_PUBLIC_URL": publicURL,
	})
}

func reserveLocalPort() (int, error) {
	listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
	if errorValue != nil {
		return 0, errorValue
	}
	defer listener.Close()
	address, isTCPAddress := listener.Addr().(*net.TCPAddr)
	if !isTCPAddress {
		return 0, errors.New("failed to reserve local tcp port")
	}
	return address.Port, nil
}

func buildMattermostTunnelCommand(target verifyTarget, port int) *exec.Cmd {
	sshArguments := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ExitOnForwardFailure=yes",
		"-N",
		"-L", fmt.Sprintf("%d:127.0.0.1:8065", port),
		fmt.Sprintf("%s@%s", target.user, target.host),
	}
	if target.password != "" {
		return exec.Command(target.sshpassBin, append([]string{"-p", target.password, "ssh"}, sshArguments...)...)
	}
	return exec.Command("ssh", sshArguments...)
}

func runPlaywright(specPath string, environmentVariables map[string]string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}

	command := exec.Command("bunx", "playwright", "test", specPath)
	command.Dir = filepath.Join(repositoryRootPath, "web")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Env = os.Environ()
	for key, value := range environmentVariables {
		command.Env = append(command.Env, key+"="+value)
	}
	return command.Run()
}

func verifyAPIScript() string {
	return `set -euo pipefail

echo "checking services"
systemctl is-active mattermost | grep -q '^active$'
systemctl is-active blueclaw | grep -q '^active$'
systemctl is-active internkim-admind | grep -q '^active$'
systemctl is-active graphiti-memoryd | grep -q '^active$'
systemctl is-active cloudflared | grep -q '^active$'

echo "checking admin gateway"
curl --silent --show-error --fail http://127.0.0.1:18080/_internkim/admin/health | jq -e '.status == "ok"' >/dev/null

echo "checking mattermost ping"
curl --silent --show-error --fail http://localhost:8065/api/v4/system/ping | jq -e '.status == "OK"' >/dev/null

echo "checking admin login"
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
login_headers="$(mktemp)"
login_body="$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')"
curl --silent --show-error --fail -D "$login_headers" -o /tmp/internkim-admin-login.json \
  -H "Content-Type: application/json" \
  -d "$login_body" \
  http://localhost:8065/api/v4/users/login >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
test -n "$admin_token"

echo "checking capability profile lookup"
mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
bot_user_id="$(curl --silent --show-error --fail -H "Authorization: Bearer $mattermost_token" http://localhost:8065/api/v4/users/me | jq -r '.id // empty')"
test -n "$bot_user_id"
lookup_body="$(jq -cn --arg senderID "$bot_user_id" '{senderID:$senderID}')"
curl --silent --show-error --fail --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$lookup_body" http://internkim/v1/platform/mattermost/identity.resolve | jq -e '.email != null' >/dev/null

echo "checking llm capability"
model="$(jq -r '.languageModel.capability.model // "google/gemini-3-flash-preview"' /root/.blueclaw/config/runtime.json)"
schema='{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}'
llm_body="$(jq -cn --arg model "$model" --arg schema "$schema" '{
  model: $model,
  executionMode: "remote",
  messages: [{role:"user", content:"Return JSON only with content set to ok."}],
  structuredOutputSchema: {name:"plain_text_response", document:$schema, isStrictlyEnforced:true},
  requireParameters: true,
  enableResponseHealing: true
}')"
llm_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$llm_body" http://internkim/v1/llm/structured)"
printf '%s' "$llm_response" | jq -e '.content | fromjson | .content | type == "string"' >/dev/null

echo "checking litert capability"
if command -v litert-lm >/dev/null 2>&1 && [ -s /root/.internkim/models/gemma-4-E4B-it.litertlm ]; then
  litert_body="$(jq -cn --arg schema "$schema" '{
    model: "local/gemma-4-E4B-it-litert-lm",
    executionMode: "local",
    messages: [{role:"user", content:"Return JSON only with content set to ok."}],
    structuredOutputSchema: {name:"plain_text_response", document:$schema, isStrictlyEnforced:true},
    requireParameters: true,
    enableResponseHealing: true
  }')"
  litert_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$litert_body" http://internkim/v1/llm/structured)"
  printf '%s' "$litert_response" | jq -e '.selectedBackend as $backend | ($backend == "gpu" or $backend == "cpu") and (.content | fromjson | .content | type == "string")' >/dev/null
else
  echo "litert capability: skipped"
fi

echo "checking secret isolation"
! su -s /bin/sh blueclaw -c 'test -r /root/.internkim/secrets/openrouter-api-key || test -r /root/.internkim/secrets/mattermost-bot-token || test -r /root/.internkim/secrets/slack-bot-token || test -r /root/.internkim/secrets/slack-app-token || test -r /root/.internkim/secrets/device-secret || test -r /root/.internkim/config/signal-jsonrpc-url || test -r /root/.internkim/config/signal-account || test -r /root/.internkim/models/gemma-4-E4B-it.litertlm' 2>/dev/null

echo "checking blueclaw health"
curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/policy >/dev/null

echo "checking graphiti memory health"
curl --silent --show-error --fail http://127.0.0.1:7791/health | jq -e '.status == "ok"' >/dev/null

echo "checking blueclaw backup manifest"
manifest_path="$(mktemp)"
curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/backup/manifest > "$manifest_path"
python3 - "$manifest_path" <<'PY'
import json
import sys

with open(sys.argv[1]) as file:
    manifest = json.load(file)

document = json.dumps(manifest)
for forbidden in ("token", "Authorization", "passphrase", "OpenRouter", "apiKey", "signingSecret"):
    if forbidden in document:
        print("backup manifest contains secret reference: " + forbidden, file=sys.stderr)
        sys.exit(1)
if manifest.get("contractVersion") != 1:
    print("unexpected backup contract version", file=sys.stderr)
    sys.exit(1)
if "blueclaw-postgres-dump" not in manifest.get("requiredBackupArtifacts", []):
    print("missing blueclaw postgres backup artifact", file=sys.stderr)
    sys.exit(1)
PY

echo "checking users sync"
systemctl start internkim-users-sync.service || journalctl -u internkim-users-sync -n 40 --no-pager
test -f /root/.internkim/state/users-sync.json
python3 - <<'PY'
import json
import sys

with open("/root/.internkim/state/users-sync.json") as file:
    expected = {email.lower() for email in json.load(file).get("users", [])}
with open("/root/.blueclaw/config/policy.json") as file:
    policy = json.load(file)
actual = set()
for person in policy.get("people", []):
    for email in person.get("emails", []):
        actual.add(str(email).lower())
missing = sorted(expected - actual)
if missing:
    print("missing policy emails: " + ", ".join(missing), file=sys.stderr)
    sys.exit(1)
PY

echo "verify api: ok"
`
}

func verifyMattermostScript() string {
	return `set -euo pipefail

timestamp="$(date +%s)"
invited_email="verify-invited-$timestamp@internkim.test"
uninvited_email="verify-uninvited-$timestamp@internkim.test"
invited_username="verifyinvited$timestamp"
uninvited_username="verifyuninvited$timestamp"
password="VerifyPass!$timestamp"
channel_id="$(cat /root/.internkim/env/channel-id)"
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"

login_headers="$(mktemp)"
login_body="$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')"
curl --silent --show-error --fail -D "$login_headers" -o /tmp/internkim-admin-login.json \
  -H "Content-Type: application/json" \
  -d "$login_body" \
  http://localhost:8065/api/v4/users/login >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
test -n "$admin_token"

create_user() {
  local email="$1"
  local username="$2"
  local body
  body="$(jq -cn --arg email "$email" --arg username "$username" --arg password "$password" '{email:$email,username:$username,password:$password}')"
  curl --silent --show-error --fail -H "Authorization: Bearer $admin_token" -H "Content-Type: application/json" \
    -d "$body" http://localhost:8065/api/v4/users | jq -r '.id'
}

login_user() {
  local username="$1"
  local headers
  headers="$(mktemp)"
  local body
  body="$(jq -cn --arg login_id "$username" --arg password "$password" '{login_id:$login_id,password:$password}')"
  curl --silent --show-error --fail -D "$headers" -o /tmp/internkim-user-login.json \
    -H "Content-Type: application/json" \
    -d "$body" \
    http://localhost:8065/api/v4/users/login >/dev/null
  awk 'tolower($1) == "token:" {print $2}' "$headers" | tr -d '\r'
}

join_channel() {
  local user_id="$1"
  local team_id
  team_id="$(curl --silent --show-error --fail -H "Authorization: Bearer $admin_token" "http://localhost:8065/api/v4/channels/$channel_id" | jq -r '.team_id // empty')"
  if [ -n "$team_id" ]; then
    curl --silent --show-error -H "Authorization: Bearer $admin_token" -H "Content-Type: application/json" \
      -d "$(jq -cn --arg team_id "$team_id" --arg user_id "$user_id" '{team_id:$team_id,user_id:$user_id}')" \
      "http://localhost:8065/api/v4/teams/$team_id/members" >/dev/null || true
  fi
  curl --silent --show-error -H "Authorization: Bearer $admin_token" -H "Content-Type: application/json" \
    -d "$(jq -cn --arg user_id "$user_id" '{user_id:$user_id}')" \
    "http://localhost:8065/api/v4/channels/$channel_id/members" >/dev/null || true
}

post_message() {
  local user_token="$1"
  local message="$2"
  curl --silent --show-error --fail -H "Authorization: Bearer $user_token" -H "Content-Type: application/json" \
    -d "$(jq -cn --arg channel_id "$channel_id" --arg message "$message" '{channel_id:$channel_id,message:$message}')" \
    http://localhost:8065/api/v4/posts
}

task_count() {
  curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/task | jq 'length'
}

wait_for_task_count() {
  local expected_count="$1"
  for _ in $(seq 1 30); do
    local current_count
    current_count="$(task_count)"
    if [ "$current_count" -ge "$expected_count" ]; then
      return 0
    fi
    sleep 1
  done
  echo "expected task count >= $expected_count" >&2
  return 1
}

wait_for_bot_reply() {
  local expected_text="$1"
  for _ in $(seq 1 30); do
    if curl --silent --show-error --fail -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=30" |
      jq -e --arg bot_user_id "$bot_user_id" --arg expected_text "$expected_text" \
        '.posts[] | select(.user_id == $bot_user_id and (.message | contains($expected_text)))' >/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "expected bot reply containing: $expected_text" >&2
  return 1
}

wait_for_model_reply() {
  local posted_after="$1"
  for _ in $(seq 1 45); do
    if curl --silent --show-error --fail -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=60" |
      jq -e --arg bot_user_id "$bot_user_id" --argjson posted_after "$posted_after" \
        '.posts[] | select(
          .user_id == $bot_user_id and
          .create_at >= $posted_after and
          (.message | contains("I am having trouble reaching the language model") | not) and
          (.message | contains("has not invited") | not)
        )' >/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "expected model-generated bot reply after post timestamp: $posted_after" >&2
  return 1
}

cleanup() {
  curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$invited_email" >/dev/null || true
}
trap cleanup EXIT

echo "preparing verify users"
mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
bot_user_id="$(curl --silent --show-error --fail -H "Authorization: Bearer $mattermost_token" http://localhost:8065/api/v4/users/me | jq -r '.id // empty')"
test -n "$bot_user_id"
invited_user_id="$(create_user "$invited_email" "$invited_username")"
uninvited_user_id="$(create_user "$uninvited_email" "$uninvited_username")"
join_channel "$invited_user_id"
join_channel "$uninvited_user_id"
invited_token="$(login_user "$invited_username")"
uninvited_token="$(login_user "$uninvited_username")"
test -n "$invited_token"
test -n "$uninvited_token"

curl --silent --show-error --fail -H "Content-Type: application/json" \
  -d "$(jq -cn --arg email "$invited_email" '{email:$email}')" \
  http://127.0.0.1:8080/admin/api/people/invite >/dev/null

before_count="$(task_count)"
invited_message="verify invited $timestamp"
invited_post="$(post_message "$invited_token" "$invited_message")"
invited_post_id="$(printf '%s' "$invited_post" | jq -r '.id')"
invited_post_create_at="$(printf '%s' "$invited_post" | jq -r '.create_at')"
test -n "$invited_post_id"
test -n "$invited_post_create_at"
wait_for_task_count "$((before_count + 1))"
wait_for_model_reply "$invited_post_create_at"
after_count="$(task_count)"

uninvited_message="verify uninvited $timestamp"
uninvited_post="$(post_message "$uninvited_token" "$uninvited_message")"
uninvited_post_id="$(printf '%s' "$uninvited_post" | jq -r '.id')"
test -n "$uninvited_post_id"
wait_for_bot_reply "has not invited"
final_count="$(task_count)"
test "$final_count" -eq "$after_count"

echo "verify mattermost: ok"
`
}
