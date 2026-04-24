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
systemctl is-active cloudflared | grep -q '^active$'

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

echo "checking bot token profile lookup"
bot_token="$(cat /root/.internkim/env/bot-token)"
bot_user_id="$(curl --silent --show-error --fail -H "Authorization: Bearer $bot_token" http://localhost:8065/api/v4/users/me | jq -r '.id // empty')"
test -n "$bot_user_id"
curl --silent --show-error --fail -H "Authorization: Bearer $bot_token" "http://localhost:8065/api/v4/users/$bot_user_id" | jq -e '.email != null' >/dev/null

echo "checking blueclaw health"
curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/policy >/dev/null

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

submit_event() {
  local user_id="$1"
  local post_id="$2"
  local message="$3"
  curl --silent --show-error --fail -H "Content-Type: application/json" \
    -d "$(jq -cn --arg user_id "$user_id" --arg channel_id "$channel_id" --arg post_id "$post_id" --arg message "$message" '{event:"posted",user_id:$user_id,channel_id:$channel_id,post_id:$post_id,message:$message}')" \
    http://127.0.0.1:8080/connectors/mattermost/events
}

task_count() {
  curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/task | jq 'length'
}

cleanup() {
  curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$invited_email" >/dev/null || true
}
trap cleanup EXIT

echo "preparing verify users"
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
invited_result="$(submit_event "$invited_user_id" "$invited_post_id" "$invited_message")"
printf '%s' "$invited_result" | jq -e '.isAllowed == true and (.reply | startswith("Working on it: ")) and .taskRunID != ""' >/dev/null
after_count="$(task_count)"
test "$after_count" -eq "$((before_count + 1))"

duplicate_result="$(submit_event "$invited_user_id" "$invited_post_id" "$invited_message")"
printf '%s' "$duplicate_result" | jq -e '.isDuplicate == true and .taskRunID != ""' >/dev/null
duplicate_count="$(task_count)"
test "$duplicate_count" -eq "$after_count"

uninvited_message="verify uninvited $timestamp"
uninvited_post="$(post_message "$uninvited_token" "$uninvited_message")"
uninvited_post_id="$(printf '%s' "$uninvited_post" | jq -r '.id')"
uninvited_result="$(submit_event "$uninvited_user_id" "$uninvited_post_id" "$uninvited_message")"
printf '%s' "$uninvited_result" | jq -e '.isAllowed == false and .reason == "not_invited" and (.reply | length > 0)' >/dev/null
final_count="$(task_count)"
test "$final_count" -eq "$after_count"

echo "verify mattermost: ok"
`
}
