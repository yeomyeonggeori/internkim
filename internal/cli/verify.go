package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type verifyTarget struct {
	host       string
	user       string
	password   string
	nodeID     string
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
	case "browser":
		return runVerifyBrowser(arguments)
	case "install-addresses":
		return runVerifyInstallAddresses()
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

func hasExplicitVerifyTargetArgument(arguments []string) bool {
	for _, flagName := range []string{"--host", "--node", "--board", "--cloudflare-ssh", "--sim"} {
		if hasCommandArgument(arguments, flagName) {
			return true
		}
	}
	return false
}

func verifyTargetArguments(host string, user string, password string, node string, cloudflareSSH bool, board string, simulation bool) []string {
	targetArguments := []string{}
	if strings.TrimSpace(host) != "" {
		targetArguments = append(targetArguments, "--host", strings.TrimSpace(host))
	}
	if strings.TrimSpace(user) != "" {
		targetArguments = append(targetArguments, "--user", strings.TrimSpace(user))
	}
	if strings.TrimSpace(password) != "" {
		targetArguments = append(targetArguments, "--password", password)
	}
	if strings.TrimSpace(node) != "" {
		targetArguments = append(targetArguments, "--node", strings.TrimSpace(node))
	}
	if cloudflareSSH {
		targetArguments = append(targetArguments, "--cloudflare-ssh")
	}
	if strings.TrimSpace(board) != "" {
		targetArguments = append(targetArguments, "--board", strings.TrimSpace(board))
	}
	if simulation {
		targetArguments = append(targetArguments, "--sim")
	}
	return targetArguments
}

type repeatedStringFlag struct {
	values []string
}

func (flagValue *repeatedStringFlag) String() string {
	return strings.Join(flagValue.values, ",")
}

func (flagValue *repeatedStringFlag) Set(value string) error {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue != "" {
		flagValue.values = append(flagValue.values, trimmedValue)
	}
	return nil
}

func (flagValue repeatedStringFlag) Values() []string {
	return append([]string{}, flagValue.values...)
}

func trimmedNonEmptyValues(values []string) []string {
	trimmedValues := []string{}
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			trimmedValues = append(trimmedValues, trimmedValue)
		}
	}
	return trimmedValues
}

func runVerifyBrowser(arguments []string) error {
	flagSet := flag.NewFlagSet("verify browser", flag.ContinueOnError)
	publicMode := flagSet.Bool("public", true, "Run public URL browser smoke test")
	target := registerTargetFlags(flagSet)
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}

	verifyTarget, errorValue := target.resolveVerifyTarget()
	if errorValue != nil {
		return errorValue
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
	user := flagSet.String("user", "", "SSH user")
	password := flagSet.String("password", "", "SSH password")
	flagSet.String("node", "", "Fleet node target")
	flagSet.Bool("remote-ssh", false, "Reach the device by its ssh hostname instead of probing the local network")
	flagSet.String("board", "", "Board target")
	flagSet.Bool("sim", false, "Use simulation target")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return verifyTarget{}, errorValue
	}

	sshpassBin := filepath.Join(repositoryRootPath, "bin", "sshpass")
	target := resolveCommandTarget(arguments)
	if strings.TrimSpace(*host) != "" {
		target.host = strings.TrimSpace(*host)
		if strings.TrimSpace(*user) != "" {
			target.sshUser = strings.TrimSpace(*user)
		}
		if *password != "" {
			target.sshPassword = *password
		}
	}
	target = resolveLabHostForCommandTarget(target, repositoryRootPath)
	target = simulationHostTarget(repositoryRootPath, target)
	configuration := loadConfig()
	if target.useRemoteSSH && strings.TrimSpace(target.host) == "" {
		target.host = savedRemoteSSHHostname(target)
	}
	if !target.useRemoteSSH && strings.TrimSpace(target.host) == "" && target.mode != commandTargetModeSimulation {
		target.host = findBoardIPForCredentials(sshpassBin, target.stateDir, target.sshUser, target.sshPassword)
	}
	sshClient := (*sshClient)(nil)
	if strings.TrimSpace(target.host) != "" {
		sshClient = newVerifySSHClient(sshpassBin, target)
	} else {
		connection, isRemote, connectionError := resolveDeviceSSHConnection(configuration, sshpassBin, target)
		if connectionError != nil || connection == nil {
			return verifyTarget{}, errors.New("verify target not found; pass --host <ip>")
		}
		sshClient = connection
		target.host = connection.host
		target.useRemoteSSH = isRemote
	}

	printCommandTargetEvidence(target)
	return verifyTarget{
		host:       target.host,
		user:       target.sshUser,
		password:   target.sshPassword,
		nodeID:     target.nodeID,
		scriptDir:  repositoryRootPath,
		stateDir:   target.stateDir,
		sshpassBin: sshpassBin,
		sshClient:  sshClient,
	}, nil
}

func newVerifySSHClient(sshpassBin string, target commandTarget) *sshClient {
	return newSSH(sshpassBin, target.sshUser, target.sshPassword, target.host)
}

func (target verifyTarget) runRemoteVerification(script string) error {
	return target.runRemoteVerificationWithTimeout(script, 60*time.Second)
}

func (target verifyTarget) runRemoteVerificationWithTimeout(script string, timeout time.Duration) error {
	output, errorValue := target.sshClient.runResultWithTimeout(script, timeout)
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

func redactBase64Attachments(payload map[string]any, fieldName string) {
	files, isArray := payload[fieldName].([]any)
	if !isArray {
		return
	}
	for _, value := range files {
		fileDocument, isDocument := value.(map[string]any)
		if !isDocument {
			continue
		}
		contentBase64, isString := fileDocument["contentBase64"].(string)
		if isString && contentBase64 != "" {
			fileDocument["contentBase64"] = fmt.Sprintf("<redacted %d base64 chars>", len(contentBase64))
		}
	}
}

func parseLastJSONDocument(output string) ([]byte, bool) {
	trimmedOutput := strings.TrimSpace(output)
	for index := len(trimmedOutput) - 1; index >= 0; index-- {
		if trimmedOutput[index] != '{' {
			continue
		}
		if index > 0 && trimmedOutput[index-1] != '\n' && trimmedOutput[index-1] != '\r' {
			continue
		}
		candidate := trimmedOutput[index:]
		var document map[string]any
		decoder := json.NewDecoder(strings.NewReader(candidate))
		if decoder.Decode(&document) == nil {
			return []byte(candidate[:int(decoder.InputOffset())]), true
		}
	}
	return nil, false
}

func replaceLastJSONDocument(output string, replacement string) string {
	trimmedOutput := strings.TrimSpace(output)
	document, found := parseLastJSONDocument(trimmedOutput)
	if !found {
		return output
	}
	index := strings.LastIndex(trimmedOutput, string(document))
	if index < 0 {
		return output
	}
	prefix := trimmedOutput[:index]
	if prefix == "" {
		return replacement + "\n"
	}
	return prefix + replacement + "\n"
}

func runPublicBrowserVerification(target verifyTarget) error {
	publicURL := strings.TrimSpace(target.sshClient.run("cat /root/.internkim/env/device-url 2>/dev/null"))
	if publicURL == "" {
		return errors.New("public device URL is empty")
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
	script := `set -euo pipefail

echo "checking services"
systemctl is-active blueclaw | grep -q '^active$'
systemctl is-active internkim-admind | grep -q '^active$'
if systemctl cat cloudflared >/dev/null 2>&1; then
  systemctl is-active cloudflared | grep -q '^active$'
fi
grep -q 'blueclaw-supervisor' /etc/systemd/system/blueclaw.service
! grep -q 'ExecStart=/usr/local/bin/blueclaw ' /etc/systemd/system/blueclaw.service
test -x /usr/local/bin/cloud-hypervisor
test -x /usr/local/bin/virtiofsd
test -s /opt/internkim/blueclaw-runtime/manifest.json
test -s /opt/internkim/blueclaw-runtime/payload-manifest.json
test -s /opt/internkim/blueclaw-runtime/vmlinux.bin
test -s /opt/internkim/blueclaw-runtime/rootfs.ext4
blkid -o value -s TYPE /var/lib/blueclaw/workspace.ext4 | grep -q '^ext4$'
` + browserruntime.DeviceReadinessShellScript() + `

echo "checking admin gateway"
curl --silent --show-error --fail http://127.0.0.1:18080/admin/api/health | jq -e '.status == "ok"' >/dev/null
curl --silent --show-error --fail http://127.0.0.1:18080/admin/ | grep -q '<script'
curl --silent --show-error --fail http://127.0.0.1:18080/admin/_app/version.json | jq -e '.version | length > 0' >/dev/null

echo "checking the paths the company web reaches through the relay"
` + verifyRelayActorsScript(blueclaw.InternKimCentralPlaneAgentKeyPath, blueclaw.InternKimCentralPlaneAppURLPath) + `
` + verifySkillInventoryScript() + `
` + verifyAdminOnlyRunsScript() + `
for relay_path in /memory/api/facts /memory/api/schedules /files/api/roots /files/api/list /runs/api /runs/api/detail /agent/api/buzz-claim /agent/api/buzz-relay-config /persona/api/user /persona/api/soul /persona/api/identity /agent-learning/api/skills /agent-learning/api/skills?includeRetired=true /agent-learning/api/settings /agent-learning/api/soul /agent-learning/api/soul/history /companion/api/mine; do
  relay_status="$(curl --silent --output /dev/null --write-out '%{http_code}' --unix-socket ` + blueclaw.AdmindSocketPath + ` -H "X-INTERNKIM-REQUESTER-EMAIL: $relay_requester" "http://internkim$relay_path")"
  case "$relay_status" in
    401|403|404)
      echo "the company web asks for $relay_path and this device answered $relay_status for $relay_requester"
      exit 1
      ;;
  esac
done

echo "checking capabilityd health"
curl --silent --show-error --fail --unix-socket /run/internkim/capability.sock http://internkim/health | jq -e '.status == "ok"' >/dev/null

echo "checking llm capability"
model="$(jq -r '.languageModel.capability.model // "__DEFAULT_OPENROUTER_MODEL__"' /root/.blueclaw/config/runtime.json)"
llm_text_body="$(jq -cn --arg model "$model" '{
  model: $model,
  executionMode: "remote",
  messages: [{role:"user", content:"Reply with ok."}],
  requireParameters: true,
  enableResponseHealing: true
}')"
llm_text_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$llm_text_body" http://internkim/v1/llm/text)"
printf '%s' "$llm_text_response" | jq -e '.content | type == "string" and length > 0' >/dev/null
schema='{"type":"object","properties":{"reply":{"type":"string"}},"required":["reply"],"additionalProperties":false}'
llm_structured_body="$(jq -cn --arg model "$model" --argjson schema "$schema" '{
  model: $model,
  executionMode: "remote",
  messages: [{role:"user", content:"Return JSON only with reply set to ok."}],
  structuredOutputSchema: {name:"smoke_reply", document:$schema, isStrictlyEnforced:true},
  requireParameters: true,
  enableResponseHealing: true
}')"
llm_structured_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$llm_structured_body" http://internkim/v1/llm/structured)"
printf '%s' "$llm_structured_response" | jq -e '.content | fromjson | .reply | type == "string"' >/dev/null
llm_auto_structured_body="$(jq -cn --arg model "$model" --argjson schema "$schema" '{
  model: $model,
  executionMode: "auto",
  messages: [{role:"user", content:"Return JSON only with reply set to ok."}],
  structuredOutputSchema: {name:"smoke_reply", document:$schema, isStrictlyEnforced:true},
  requireParameters: true,
  enableResponseHealing: true
}')"
llm_auto_structured_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$llm_auto_structured_body" http://internkim/v1/llm/structured)"
printf '%s' "$llm_auto_structured_response" | jq -e '.provider == "openrouter" and .selectedBackend == "remote" and (.content | fromjson | .reply | type == "string")' >/dev/null
action_schema='{"oneOf":[{"type":"object","properties":{"action":{"type":"string","enum":["finish"]},"message":{"type":"string"},"goalStatus":{"type":"string","enum":["satisfied"]},"goalSatisfied":{"type":"boolean"},"completionEvidence":{"type":"array","items":{"type":"object"}},"qualityReview":{"type":"array","items":{"type":"object"}}},"required":["action","message","goalStatus","goalSatisfied","completionEvidence","qualityReview"],"additionalProperties":false}]}'
llm_action_body="$(jq -cn --arg model "$model" --argjson schema "$action_schema" '{
  model: $model,
  executionMode: "auto",
  messages: [
    {role:"system", content:"You must finish this smoke test now. Use the finish action with message ok, goalStatus satisfied, goalSatisfied true, and empty evidence/review arrays."},
    {role:"user", content:"Finish now."}
  ],
  structuredOutputSchema: {name:"bluecollar_agent_turn_action", document:$schema, isStrictlyEnforced:true},
  requireParameters: true,
  enableResponseHealing: true
}')"
llm_action_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$llm_action_body" http://internkim/v1/llm/structured)"
printf '%s' "$llm_action_response" | jq -e '.provider == "openrouter" and .selectedBackend == "remote" and (.constraintMode == "openai_json_schema" or .constraintMode == "native_tool_call") and (.content | fromjson | .action == "finish")' >/dev/null

echo "checking litert capability"
if command -v litert-lm >/dev/null 2>&1 && [ -s /root/.internkim/models/gemma-4-E4B-it.litertlm ]; then
  litert_body="$(jq -cn '{
    model: "local/gemma-4-E4B-it-litert-lm",
    accelerator: "cpu",
    executionMode: "device",
    messages: [{role:"user", content:"Reply with ok."}],
    requireParameters: true,
    enableResponseHealing: true
  }')"
  litert_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$litert_body" http://internkim/v1/llm/text 2>/tmp/internkim-verify-litert-error || true)"
  if ! printf '%s' "$litert_response" | python3 -c 'import json, sys; document=json.load(sys.stdin); backend=document.get("selectedBackend"); content=document.get("content"); raise SystemExit(0 if backend in ("gpu", "cpu") and isinstance(content, str) and len(content) > 0 else 1)' >/dev/null 2>&1; then
    if [ -n "$litert_response" ]; then
      printf 'litert capability: %s\n' "$(printf '%s' "$litert_response" | tr '\n' ' ' | cut -c1-180)"
    else
      printf 'litert capability: %s\n' "$(tr '\n' ' ' </tmp/internkim-verify-litert-error | cut -c1-180)"
    fi
    echo "litert capability: optional local check failed"
  fi
else
  echo "litert capability: skipped"
fi

echo "checking secret isolation"
! su -s /bin/sh blueclaw -c '` + blueclaw.SecretIsolationShellTest() + `' 2>/dev/null

echo "checking blueclaw health"
curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/health | jq -e '.status == "ok"' >/dev/null

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
policy_response_path="$(mktemp)"
trap 'rm -f "$policy_response_path"' EXIT
curl --silent --show-error --fail http://127.0.0.1:8080/admin/api/policy > "$policy_response_path"
python3 - "$policy_response_path" <<'PY'
import json
import sys

with open("/root/.internkim/state/users-sync.json") as file:
    expected = {email.lower() for email in json.load(file).get("users", [])}
with open(sys.argv[1]) as file:
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
	return strings.ReplaceAll(script, "__DEFAULT_OPENROUTER_MODEL__", blueclaw.BlueclawDefaultModelName)
}

func verifyRelayActorsScript(centralPlaneAgentKeyPath string, centralPlaneAppURLPath string) string {
	return `central_plane_agent_key_path="` + centralPlaneAgentKeyPath + `"
if [ ! -s "$central_plane_agent_key_path" ]; then
  echo "this host holds no company key, so there is no directory to verify relay actors against"
  exit 1
fi
central_plane_app_url="$(cat ` + centralPlaneAppURLPath + `)"
central_plane_agent_key="$(cat "$central_plane_agent_key_path")"
test -n "$central_plane_app_url"
test -n "$central_plane_agent_key"
directory_response="$(curl --silent --show-error --fail -H "Authorization: Bearer $central_plane_agent_key" "$central_plane_app_url/api/agent/member")"
active_records="$(printf '%s' "$directory_response" | jq -c -e '.members // [] | map(select((.status // "") == "" or (.status | ascii_downcase) == "active"))')"
relay_requester="$(printf '%s' "$active_records" | jq -r '[.[] | select((.role // "" | ascii_downcase) != "admin") | .email | select(length > 0)] | first // empty')"
relay_admin="$(printf '%s' "$active_records" | jq -r '[.[] | select((.role // "" | ascii_downcase) == "admin") | .email | select(length > 0)] | first // empty')"
if [ -z "$relay_requester" ]; then
  echo "the directory has no active nonadministrator to use for relay verification"
  exit 1
fi
if [ -z "$relay_admin" ]; then
  echo "the directory has no active administrator to use for relay verification"
  exit 1
fi`
}

func verifySkillInventoryScript() string {
	return `skill_member_status="$(curl --silent --output /dev/null --write-out '%{http_code}' --unix-socket ` + blueclaw.AdmindSocketPath + ` -H "X-INTERNKIM-REQUESTER-EMAIL: $relay_requester" "http://internkim/skills/api")"
if [ "$skill_member_status" != "403" ]; then
  echo "the skill inventory answered $skill_member_status for nonadministrator $relay_requester"
  exit 1
fi
skill_admin_status="$(curl --silent --output /dev/null --write-out '%{http_code}' --unix-socket ` + blueclaw.AdmindSocketPath + ` -H "X-INTERNKIM-REQUESTER-EMAIL: $relay_admin" "http://internkim/skills/api")"
if [ "$skill_admin_status" != "200" ]; then
  echo "the skill inventory answered $skill_admin_status for administrator $relay_admin"
  exit 1
fi`
}

func verifyAdminOnlyRunsScript() string {
	return `for relay_path in /runs/api/llm-call /runs/api/turn-input /runs/api/inbound; do
  runs_member_status="$(curl --silent --output /dev/null --write-out '%{http_code}' --unix-socket ` + blueclaw.AdmindSocketPath + ` -H "X-INTERNKIM-REQUESTER-EMAIL: $relay_requester" "http://internkim$relay_path")"
  if [ "$runs_member_status" != "403" ]; then
    echo "$relay_path answered $runs_member_status for nonadministrator $relay_requester"
    exit 1
  fi
  runs_admin_status="$(curl --silent --output /dev/null --write-out '%{http_code}' --unix-socket ` + blueclaw.AdmindSocketPath + ` -H "X-INTERNKIM-REQUESTER-EMAIL: $relay_admin" "http://internkim$relay_path")"
  case "$runs_admin_status" in
    401|403|404)
      echo "$relay_path answered $runs_admin_status for administrator $relay_admin"
      exit 1
      ;;
  esac
done`
}

// Site tools are named exactly; matching a name prefix silently reclassifies any
// future tool that happens to start the same way.
var siteToolNames = map[string]bool{"site_serve": true, "site_list": true, "site_unserve": true}

func isSiteToolName(toolName string) bool {
	return siteToolNames[strings.TrimSpace(toolName)]
}
