package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
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
	flagSet := flag.NewFlagSet("verify mattermost", flag.ContinueOnError)
	prompt := flagSet.String("prompt", "", "Post this prompt through the real Mattermost ingress path")
	expectBrowserOpen := flagSet.Bool("expect-browser-open", false, "Require a successful browser.open tool result for prompt verification")
	expectedTools := repeatedStringFlag{}
	expectedEvents := repeatedStringFlag{}
	flagSet.Var(&expectedTools, "expect-tool", "Require a requested tool event for prompt verification; repeat for multiple tools")
	flagSet.Var(&expectedEvents, "expect-event", "Require a task event name for prompt verification; repeat for multiple events")
	browserOpenE2E := flagSet.Bool("browser-open-e2e", false, "Pair a local probe companion and require a successful browser.open result")
	keep := flagSet.Bool("keep", false, "Keep probe messages and users for inspection")
	keepBrowser := flagSet.Bool("keep-browser", false, "Keep the local browser window open after browser-open E2E")
	timeoutSeconds := flagSet.Int("timeout", 240, "Seconds to wait for the bot reply")
	companionPath := flagSet.String("companion-path", "/Applications/Intern Kim Companion.app/Contents/MacOS/internkim-companion", "Local internkim-companion executable for browser-open E2E")
	agentBrowserPath := flagSet.String("agent-browser-path", "/Applications/Intern Kim Companion.app/Contents/MacOS/agent-browser", "Local agent-browser executable for browser-open E2E")
	host := flagSet.String("host", "", "Board host")
	user := flagSet.String("user", "", "SSH user")
	password := flagSet.String("password", "", "SSH password")
	board := flagSet.String("board", "", "Board target")
	simulation := flagSet.Bool("sim", false, "Use simulation target")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}

	targetArguments := []string{}
	if strings.TrimSpace(*host) != "" {
		targetArguments = append(targetArguments, "--host", strings.TrimSpace(*host))
	}
	if strings.TrimSpace(*user) != "" {
		targetArguments = append(targetArguments, "--user", strings.TrimSpace(*user))
	}
	if strings.TrimSpace(*password) != "" {
		targetArguments = append(targetArguments, "--password", *password)
	}
	if strings.TrimSpace(*board) != "" {
		targetArguments = append(targetArguments, "--board", strings.TrimSpace(*board))
	}
	if *simulation {
		targetArguments = append(targetArguments, "--sim")
	}

	verifyTarget, errorValue := resolveVerifyTarget(targetArguments)
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("verify mattermost: %s@%s\n", verifyTarget.user, verifyTarget.host)
	if *browserOpenE2E {
		promptText := strings.TrimSpace(*prompt)
		if promptText == "" {
			promptText = "브라우저 열어줘."
		}
		return runMattermostBrowserOpenE2E(verifyTarget, promptText, *keep, *keepBrowser, *timeoutSeconds, *companionPath, *agentBrowserPath)
	}
	if strings.TrimSpace(*prompt) != "" {
		return verifyTarget.runRemoteVerification(verifyMattermostPromptScript(*prompt, *keep, *timeoutSeconds, *expectBrowserOpen, expectedTools.Values(), expectedEvents.Values()))
	}
	if *expectBrowserOpen || len(expectedTools.Values()) > 0 || len(expectedEvents.Values()) > 0 {
		return fmt.Errorf("--expect-browser-open, --expect-tool, and --expect-event require --prompt")
	}
	return verifyTarget.runRemoteVerification(verifyMattermostScript())
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

type mattermostBrowserOpenE2EPreparation struct {
	DeviceURL string `json:"deviceURL"`
	Code      string `json:"code"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	UserID    string `json:"userID"`
	ChannelID string `json:"channelID"`
}

func runMattermostBrowserOpenE2E(target verifyTarget, prompt string, keep bool, keepBrowser bool, timeoutSeconds int, companionPath string, agentBrowserPath string) error {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 240
	}
	preparationOutput, errorValue := target.sshClient.runResult(prepareMattermostBrowserOpenE2EScript())
	if strings.TrimSpace(preparationOutput) != "" {
		fmt.Print(preparationOutput)
		if !strings.HasSuffix(preparationOutput, "\n") {
			fmt.Println()
		}
	}
	if errorValue != nil {
		return fmt.Errorf("prepare Mattermost browser E2E: %w", errorValue)
	}
	preparation, errorValue := parseMattermostBrowserOpenE2EPreparation(preparationOutput)
	if errorValue != nil {
		return fmt.Errorf("parse Mattermost browser E2E preparation: %w", errorValue)
	}
	if !keep {
		defer func() {
			output, cleanupError := target.sshClient.runResult(cleanupMattermostBrowserOpenE2EScript(preparation))
			if strings.TrimSpace(output) != "" {
				fmt.Print(output)
				if !strings.HasSuffix(output, "\n") {
					fmt.Println()
				}
			}
			if cleanupError != nil {
				fmt.Fprintf(os.Stderr, "cleanup warning: %v\n", cleanupError)
			}
		}()
	}
	temporaryDirectory, errorValue := os.MkdirTemp("", "internkim-companion-browser-e2e-")
	if errorValue != nil {
		return errorValue
	}
	statePath := filepath.Join(temporaryDirectory, "state.json")
	browserProfilePath := filepath.Join(temporaryDirectory, "browser-profile")
	if keepBrowser {
		fmt.Println("local browser will remain open; temporary directory: " + temporaryDirectory)
	} else {
		defer os.RemoveAll(temporaryDirectory)
	}
	if errorValue := pairCompanionForMattermostBrowserOpenE2E(companionPath, preparation, statePath); errorValue != nil {
		return errorValue
	}
	closeMattermostBrowserOpenE2ESession(agentBrowserPath)
	if !keepBrowser {
		defer closeMattermostBrowserOpenE2ESession(agentBrowserPath)
	}
	companionCommand, companionLog, errorValue := startMattermostBrowserOpenE2ECompanion(companionPath, agentBrowserPath, statePath, browserProfilePath)
	if errorValue != nil {
		return errorValue
	}
	defer stopMattermostBrowserOpenE2ECompanion(companionCommand, companionLog)
	if errorValue := verifyMattermostBrowserOpenE2ECompanion(companionPath, agentBrowserPath, statePath); errorValue != nil {
		return errorValue
	}
	if errorValue := waitMattermostBrowserOpenE2ECompanionOnline(target, preparation, timeoutSeconds); errorValue != nil {
		return errorValue
	}
	if errorValue := target.runRemoteVerification(runMattermostBrowserOpenE2EScript(prompt, keep, timeoutSeconds, preparation)); errorValue != nil {
		return errorValue
	}
	return confirmMattermostBrowserOpenE2ELocalBrowser(agentBrowserPath)
}

func parseMattermostBrowserOpenE2EPreparation(output string) (mattermostBrowserOpenE2EPreparation, error) {
	lines := strings.Split(output, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var preparation mattermostBrowserOpenE2EPreparation
		if errorValue := json.Unmarshal([]byte(line), &preparation); errorValue == nil && preparation.Code != "" {
			return preparation, nil
		}
	}
	return mattermostBrowserOpenE2EPreparation{}, errors.New("preparation JSON was not found")
}

func closeMattermostBrowserOpenE2ESession(agentBrowserPath string) {
	command := exec.Command(agentBrowserPath, "--session", "internkim", "--session-name", "internkim", "close", "--all")
	_ = command.Run()
}

func pairCompanionForMattermostBrowserOpenE2E(companionPath string, preparation mattermostBrowserOpenE2EPreparation, statePath string) error {
	command := exec.Command(companionPath, "pair", "--device-url", preparation.DeviceURL, "--code", preparation.Code, "--state", statePath)
	output, errorValue := command.CombinedOutput()
	if strings.TrimSpace(string(output)) != "" {
		fmt.Print(string(output))
		if !strings.HasSuffix(string(output), "\n") {
			fmt.Println()
		}
	}
	if errorValue != nil {
		return fmt.Errorf("pair local companion: %w", errorValue)
	}
	return nil
}

func startMattermostBrowserOpenE2ECompanion(companionPath string, agentBrowserPath string, statePath string, browserProfilePath string) (*exec.Cmd, *bytes.Buffer, error) {
	command := exec.Command(
		companionPath,
		"run",
		"--state", statePath,
		"--agent-browser-path", agentBrowserPath,
		"--browser-profile", browserProfilePath,
		"--prefer-companion-browser",
		"--development-auto-approve-browser",
	)
	var logBuffer bytes.Buffer
	command.Stdout = &logBuffer
	command.Stderr = &logBuffer
	if errorValue := command.Start(); errorValue != nil {
		return nil, nil, fmt.Errorf("start local companion: %w", errorValue)
	}
	time.Sleep(3 * time.Second)
	if command.ProcessState != nil && command.ProcessState.Exited() {
		return nil, nil, fmt.Errorf("local companion exited early: %s", strings.TrimSpace(logBuffer.String()))
	}
	return command, &logBuffer, nil
}

func stopMattermostBrowserOpenE2ECompanion(command *exec.Cmd, logBuffer *bytes.Buffer) {
	if command == nil || command.Process == nil {
		return
	}
	_ = command.Process.Kill()
	_, _ = command.Process.Wait()
	if strings.TrimSpace(logBuffer.String()) != "" {
		fmt.Print(logBuffer.String())
		if !strings.HasSuffix(logBuffer.String(), "\n") {
			fmt.Println()
		}
	}
}

func verifyMattermostBrowserOpenE2ECompanion(companionPath string, agentBrowserPath string, statePath string) error {
	command := exec.Command(companionPath, "status", "--state", statePath, "--json", "--verify-auth")
	command.Env = append(os.Environ(), "INTERNKIM_AGENT_BROWSER_PATH="+agentBrowserPath)
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("verify local companion status: %w: %s", errorValue, strings.TrimSpace(string(output)))
	}
	var statusDocument struct {
		AuthStatus           string `json:"authStatus"`
		BrowserRuntimeStatus string `json:"browserRuntimeStatus"`
	}
	if errorValue := json.Unmarshal(output, &statusDocument); errorValue != nil {
		return fmt.Errorf("parse local companion status: %w", errorValue)
	}
	if statusDocument.AuthStatus != "verified" {
		return fmt.Errorf("local companion auth status is %s", statusDocument.AuthStatus)
	}
	if statusDocument.BrowserRuntimeStatus != "ready" {
		return fmt.Errorf("local companion browser runtime status is %s", statusDocument.BrowserRuntimeStatus)
	}
	fmt.Println("local companion: verified, browser runtime ready")
	return nil
}

func waitMattermostBrowserOpenE2ECompanionOnline(target verifyTarget, preparation mattermostBrowserOpenE2EPreparation, timeoutSeconds int) error {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 240
	}
	output, errorValue := target.sshClient.runResult(waitMattermostBrowserOpenE2ECompanionOnlineScript(preparation, timeoutSeconds))
	if strings.TrimSpace(output) != "" {
		fmt.Print(output)
		if !strings.HasSuffix(output, "\n") {
			fmt.Println()
		}
	}
	if errorValue != nil {
		return fmt.Errorf("wait for remote companion heartbeat: %w", errorValue)
	}
	return nil
}

func confirmMattermostBrowserOpenE2ELocalBrowser(agentBrowserPath string) error {
	command := exec.Command(agentBrowserPath, "--session", "internkim", "--session-name", "internkim", "get", "url")
	output, errorValue := command.CombinedOutput()
	localURL := strings.TrimSpace(string(output))
	if errorValue != nil {
		return fmt.Errorf("confirm local browser URL: %w: %s", errorValue, localURL)
	}
	if !strings.Contains(localURL, "google.") {
		return fmt.Errorf("local browser did not navigate to Google: %s", localURL)
	}
	fmt.Println("local browser URL: " + localURL)
	return nil
}

func runVerifyBrowser(arguments []string) error {
	flagSet := flag.NewFlagSet("verify browser", flag.ContinueOnError)
	localMode := flagSet.Bool("local", false, "Run local Mattermost browser smoke test")
	publicMode := flagSet.Bool("public", false, "Run public URL browser smoke test")
	host := flagSet.String("host", "", "Board host")
	user := flagSet.String("user", "", "SSH user")
	password := flagSet.String("password", "", "SSH password")
	board := flagSet.String("board", "", "Board target")
	if errorValue := flagSet.Parse(arguments); errorValue != nil {
		return errorValue
	}
	if !*localMode && !*publicMode {
		*localMode = true
	}

	targetArguments := []string{}
	if strings.TrimSpace(*host) != "" {
		targetArguments = append(targetArguments, "--host", *host)
	}
	if strings.TrimSpace(*user) != "" {
		targetArguments = append(targetArguments, "--user", *user)
	}
	if strings.TrimSpace(*password) != "" {
		targetArguments = append(targetArguments, "--password", *password)
	}
	if strings.TrimSpace(*board) != "" {
		targetArguments = append(targetArguments, "--board", *board)
	}
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
	user := flagSet.String("user", "", "SSH user")
	password := flagSet.String("password", "", "SSH password")
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
	if strings.TrimSpace(target.host) == "" && target.mode != commandTargetModeSimulation {
		target.host = findBoardIPForCredentials(sshpassBin, target.stateDir, target.sshUser, target.sshPassword)
	}
	if strings.TrimSpace(target.host) == "" {
		return verifyTarget{}, errors.New("verify target not found; pass --host <ip>")
	}

	printCommandTargetEvidence(target)
	sshClient := newSSH(sshpassBin, target.sshUser, target.sshPassword, target.host)
	return verifyTarget{
		host:       target.host,
		user:       target.sshUser,
		password:   target.sshPassword,
		scriptDir:  repositoryRootPath,
		stateDir:   target.stateDir,
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

	adminEmail := strings.TrimSpace(target.sshClient.run("cat /root/.internkim/config/admin-email 2>/dev/null || cat /root/.internkim/admin-email 2>/dev/null"))
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
systemctl is-active cloudflared | grep -q '^active$'
grep -q 'blueclaw-supervisor' /etc/systemd/system/blueclaw.service
! grep -q 'ExecStart=/usr/local/bin/blueclaw ' /etc/systemd/system/blueclaw.service
test -x /usr/local/bin/firecracker
test -x /usr/local/bin/jailer
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

echo "checking mattermost ping"
curl --silent --show-error --fail http://localhost:8065/api/v4/system/ping | jq -e '.status == "OK"' >/dev/null

echo "checking capabilityd health"
curl --silent --show-error --fail --unix-socket /run/internkim/capability.sock http://internkim/health | jq -e '.status == "ok"' >/dev/null

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
model="$(jq -r '.languageModel.capability.model // "google/gemini-3.1-flash-lite-preview"' /root/.blueclaw/config/runtime.json)"
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
llm_structured_body="$(jq -cn --arg model "$model" --arg schema "$schema" '{
  model: $model,
  executionMode: "remote",
  messages: [{role:"user", content:"Return JSON only with reply set to ok."}],
  structuredOutputSchema: {name:"smoke_reply", document:$schema, isStrictlyEnforced:true},
  requireParameters: true,
  enableResponseHealing: true
}')"
llm_structured_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$llm_structured_body" http://internkim/v1/llm/structured)"
printf '%s' "$llm_structured_response" | jq -e '.content | fromjson | .reply | type == "string"' >/dev/null

echo "checking litert capability"
if command -v litert-lm >/dev/null 2>&1 && [ -s /root/.internkim/models/gemma-4-E4B-it.litertlm ]; then
  litert_body="$(jq -cn '{
    model: "local/gemma-4-E4B-it-litert-lm",
    executionMode: "device",
    messages: [{role:"user", content:"Reply with ok."}],
    requireParameters: true,
    enableResponseHealing: true
  }')"
  litert_response="$(curl --silent --show-error --unix-socket /run/internkim/capability.sock -H "Content-Type: application/json" -d "$litert_body" http://internkim/v1/llm/text)"
  printf '%s' "$litert_response" | jq -e '.selectedBackend as $backend | ($backend == "gpu" or $backend == "cpu") and (.content | type == "string" and length > 0)' >/dev/null
else
  echo "litert capability: skipped"
fi

echo "checking secret isolation"
! su -s /bin/sh blueclaw -c 'test -r /root/.internkim/secrets/openrouter-api-key || test -r /root/.internkim/secrets/mattermost-bot-token || test -r /root/.internkim/secrets/slack-bot-token || test -r /root/.internkim/secrets/slack-app-token || test -r /root/.internkim/secrets/device-secret || test -r /root/.internkim/config/signal-jsonrpc-url || test -r /root/.internkim/config/signal-account || test -r /root/.internkim/models/gemma-4-E4B-it.litertlm' 2>/dev/null

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
test_started_at="$(date +%s%3N)"

phase() {
  echo "$1"
}

api_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local token="${4:-}"
  local body="${5:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    if [ -n "$token" ]; then
      status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
        -X "$method" -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
        -d "$body" "$url")" || curl_status="$?"
    else
      status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
        -X "$method" -H "Content-Type: application/json" \
        -d "$body" "$url")" || curl_status="$?"
    fi
  else
    if [ -n "$token" ]; then
      status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
        -X "$method" -H "Authorization: Bearer $token" "$url")" || curl_status="$?"
    else
      status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
        -X "$method" "$url")" || curl_status="$?"
    fi
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Mattermost API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Mattermost API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

blueclaw_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local body="${4:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
      -X "$method" -H "Content-Type: application/json" -d "$body" "$url")" || curl_status="$?"
  else
    status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
      -X "$method" "$url")" || curl_status="$?"
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Blueclaw API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Blueclaw API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

resolve_team_id() {
  api_request "resolve channel team" GET "http://localhost:8065/api/v4/channels/$channel_id" "$admin_token" | jq -r '.team_id // empty'
}

phase "admin login"
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
  api_request "create user $username" POST http://localhost:8065/api/v4/users "$admin_token" "$body" | jq -r '.id'
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
  local team_member_body
  local channel_member_body
  team_id="$(resolve_team_id)"
  if [ -z "$team_id" ]; then
    echo "Mattermost channel $channel_id does not belong to a team" >&2
    return 1
  fi
  team_member_body="$(jq -cn --arg team_id "$team_id" --arg user_id "$user_id" '{team_id:$team_id,user_id:$user_id}')"
  api_request "join team $user_id" POST "http://localhost:8065/api/v4/teams/$team_id/members" "$admin_token" "$team_member_body" >/dev/null
  channel_member_body="$(jq -cn --arg user_id "$user_id" '{user_id:$user_id}')"
  api_request "join channel $user_id" POST "http://localhost:8065/api/v4/channels/$channel_id/members" "$admin_token" "$channel_member_body" >/dev/null
  api_request "verify channel membership $user_id" GET "http://localhost:8065/api/v4/channels/$channel_id/members/$user_id" "$admin_token" >/dev/null
}

post_message() {
  local user_token="$1"
  local message="$2"
  api_request "post message" POST http://localhost:8065/api/v4/posts "$user_token" \
    "$(jq -cn --arg channel_id "$channel_id" --arg message "$message" '{channel_id:$channel_id,message:$message}')"
}

task_count() {
  blueclaw_request "task count" GET http://127.0.0.1:8080/admin/api/task | jq 'length'
}

wait_for_task_count() {
  local expected_count="$1"
  for _ in $(seq 1 120); do
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

print_recent_bot_replies() {
  local posted_after="$1"
  api_request "print recent bot replies" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=60" "$admin_token" |
    jq -r --arg bot_user_id "$bot_user_id" --argjson posted_after "$posted_after" \
      '.posts[] | select(.user_id == $bot_user_id and .create_at >= $posted_after) | "\(.create_at)\t\(.message)"' >&2 || true
}

wait_for_bot_reply() {
  local expected_text="$1"
  local posted_after="$2"
  for _ in $(seq 1 120); do
    if api_request "wait for bot reply" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=30" "$admin_token" |
      jq -e --arg bot_user_id "$bot_user_id" --arg expected_text "$expected_text" --argjson posted_after "$posted_after" \
        '.posts[] | select(.user_id == $bot_user_id and .create_at >= $posted_after and (.message | contains($expected_text)))' >/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "expected bot reply containing: $expected_text" >&2
  print_recent_bot_replies "$posted_after"
  return 1
}

wait_for_model_reply() {
  local posted_after="$1"
  for _ in $(seq 1 120); do
    if api_request "wait for model reply" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=60" "$admin_token" |
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
  print_recent_bot_replies "$posted_after"
  return 1
}

delete_post() {
  local token="$1"
  local post_id="$2"
  if [ -z "$post_id" ] || [ "$post_id" = "null" ]; then
    return 0
  fi
  if [ -z "$token" ]; then
    echo "cleanup warning: missing token for Mattermost post $post_id" >&2
    return 0
  fi
  curl --silent --show-error -X DELETE -H "Authorization: Bearer $token" \
    "http://localhost:8065/api/v4/posts/$post_id" >/dev/null || echo "cleanup warning: failed to delete Mattermost post $post_id" >&2
}

delete_user() {
  local user_id="$1"
  local team_id
  if [ -z "$user_id" ] || [ "$user_id" = "null" ]; then
    return 0
  fi
  team_id="$(resolve_team_id 2>/dev/null || true)"
  curl --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
    "http://localhost:8065/api/v4/channels/$channel_id/members/$user_id" >/dev/null || true
  if [ -n "$team_id" ]; then
    curl --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/teams/$team_id/members/$user_id" >/dev/null || true
  fi
  curl --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
    "http://localhost:8065/api/v4/users/$user_id?permanent=true" >/dev/null || \
    curl --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/users/$user_id" >/dev/null || \
    echo "cleanup warning: failed to delete Mattermost user $user_id" >&2
}

delete_stale_verify_users() {
  for username_prefix in verifyinvited verifyuninvited; do
    api_request "find stale users $username_prefix" POST http://localhost:8065/api/v4/users/search "$admin_token" \
      "$(jq -cn --arg term "$username_prefix" '{term:$term}')" |
      jq -r --arg username_prefix "$username_prefix" '.[] | select(.username | startswith($username_prefix)) | .id' |
      while read -r user_id; do
        delete_user "$user_id"
      done
  done
}

delete_verify_system_posts() {
  sudo -u postgres psql -d mattermost <<'SQL' >/dev/null || echo "cleanup warning: failed to delete verify Mattermost system posts" >&2
UPDATE posts
SET deleteat = (extract(epoch from now()) * 1000)::bigint
WHERE type LIKE 'system_%'
  AND deleteat = 0
  AND message ~ '(verifyinvited|verifyuninvited|labmattermost)';
SQL
}

delete_verify_replies() {
  local token="$1"
  api_request "cleanup enumerate bot replies" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=100" "$admin_token" |
    jq -r --arg bot_user_id "$bot_user_id" --argjson test_started_at "$test_started_at" \
      '.posts[] | select(.user_id == $bot_user_id and .create_at >= $test_started_at) | .id' |
    while read -r post_id; do
      delete_post "$token" "$post_id"
    done || echo "cleanup warning: failed to enumerate Mattermost bot replies" >&2
}

cleanup() {
  delete_post "${invited_token:-}" "${invited_post_id:-}"
  delete_post "${uninvited_token:-}" "${uninvited_post_id:-}"
  delete_verify_replies "${mattermost_token:-}"
  delete_verify_system_posts
  delete_user "${invited_user_id:-}"
  delete_user "${uninvited_user_id:-}"
  curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$invited_email" >/dev/null || true
}
trap cleanup EXIT

phase "cleanup stale verify users"
delete_verify_system_posts
delete_stale_verify_users
phase "bot lookup"
mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
bot_user_id="$(api_request "bot lookup" GET http://localhost:8065/api/v4/users/me "$mattermost_token" | jq -r '.id // empty')"
test -n "$bot_user_id"
phase "create users"
invited_user_id="$(create_user "$invited_email" "$invited_username")"
uninvited_user_id="$(create_user "$uninvited_email" "$uninvited_username")"
phase "join users"
join_channel "$invited_user_id"
join_channel "$uninvited_user_id"
phase "login users"
invited_token="$(login_user "$invited_username")"
uninvited_token="$(login_user "$uninvited_username")"
test -n "$invited_token"
test -n "$uninvited_token"

phase "invite policy"
blueclaw_request "invite policy" POST http://127.0.0.1:8080/admin/api/people/invite \
  "$(jq -cn --arg email "$invited_email" '{email:$email}')" >/dev/null

phase "invited post"
before_count="$(task_count)"
invited_message="verify invited $timestamp"
invited_post="$(post_message "$invited_token" "$invited_message")"
invited_post_id="$(printf '%s' "$invited_post" | jq -r '.id')"
invited_post_create_at="$(printf '%s' "$invited_post" | jq -r '.create_at')"
test -n "$invited_post_id"
test -n "$invited_post_create_at"
phase "reply wait"
wait_for_task_count "$((before_count + 1))"
wait_for_model_reply "$invited_post_create_at"
after_count="$(task_count)"

phase "uninvited post"
uninvited_message="verify uninvited $timestamp"
uninvited_post="$(post_message "$uninvited_token" "$uninvited_message")"
uninvited_post_id="$(printf '%s' "$uninvited_post" | jq -r '.id')"
uninvited_post_create_at="$(printf '%s' "$uninvited_post" | jq -r '.create_at')"
test -n "$uninvited_post_id"
test -n "$uninvited_post_create_at"
phase "rejection wait"
wait_for_bot_reply "has not invited" "$uninvited_post_create_at"
final_count="$(task_count)"
test "$final_count" -eq "$after_count"

echo "verify mattermost: ok"
`
}

func verifyMattermostPromptScript(prompt string, keep bool, timeoutSeconds int, expectBrowserOpen bool, expectedTools []string, expectedEvents []string) string {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 240
	}
	encodedPrompt := base64.StdEncoding.EncodeToString([]byte(prompt))
	expectedToolsJSON, _ := json.Marshal(trimmedNonEmptyValues(expectedTools))
	expectedEventsJSON, _ := json.Marshal(trimmedNonEmptyValues(expectedEvents))
	encodedExpectedTools := base64.StdEncoding.EncodeToString(expectedToolsJSON)
	encodedExpectedEvents := base64.StdEncoding.EncodeToString(expectedEventsJSON)
	keepValue := "false"
	if keep {
		keepValue = "true"
	}
	expectBrowserOpenValue := "false"
	if expectBrowserOpen {
		expectBrowserOpenValue = "true"
	}
	return fmt.Sprintf(`set -euo pipefail

timestamp="$(date +%%s)"
email="probe-mattermost-$timestamp@internkim.test"
username="probemm$timestamp"
password="ProbePass!$timestamp"
prompt="$(printf '%%s' %s | base64 -d)"
expected_tools_json="$(printf '%%s' %s | base64 -d)"
expected_events_json="$(printf '%%s' %s | base64 -d)"
keep_artifacts=%s
timeout_seconds=%d
expect_browser_open=%s
test_started_at="$(date +%%s%%3N)"

api_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local token="${4:-}"
  local body="${5:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    if [ -n "$token" ]; then
      status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
        -X "$method" -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
        -d "$body" "$url")" || curl_status="$?"
    else
      status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
        -X "$method" -H "Content-Type: application/json" \
        -d "$body" "$url")" || curl_status="$?"
    fi
  else
    if [ -n "$token" ]; then
      status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
        -X "$method" -H "Authorization: Bearer $token" "$url")" || curl_status="$?"
    else
      status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
        -X "$method" "$url")" || curl_status="$?"
    fi
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Mattermost API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Mattermost API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

blueclaw_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local body="${4:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" -H "Content-Type: application/json" -d "$body" "$url")" || curl_status="$?"
  else
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" "$url")" || curl_status="$?"
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Blueclaw API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Blueclaw API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

login_headers="$(mktemp)"
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
login_body="$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')"
curl --silent --show-error --fail -D "$login_headers" -o /tmp/internkim-admin-login.json \
  -H "Content-Type: application/json" \
  -d "$login_body" \
  http://localhost:8065/api/v4/users/login >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
test -n "$admin_token"

cleanup() {
  if [ "$keep_artifacts" = "true" ]; then
    return 0
  fi
  if [ -n "${bot_post_id:-}" ]; then
    curl --silent --show-error -X DELETE -H "Authorization: Bearer $mattermost_token" \
      "http://localhost:8065/api/v4/posts/$bot_post_id" >/dev/null || true
  fi
  if [ -n "${user_post_id:-}" ]; then
    curl --silent --show-error -X DELETE -H "Authorization: Bearer $user_token" \
      "http://localhost:8065/api/v4/posts/$user_post_id" >/dev/null || true
  fi
  if [ -n "${user_id:-}" ]; then
    curl --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/users/$user_id?permanent=true" >/dev/null || true
  fi
  curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$email" >/dev/null || true
}
trap cleanup EXIT

mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
bot_user="$(api_request "bot lookup" GET http://localhost:8065/api/v4/users/me "$mattermost_token")"
bot_user_id="$(printf '%%s' "$bot_user" | jq -r '.id // empty')"
bot_username="$(printf '%%s' "$bot_user" | jq -r '.username // empty')"
test -n "$bot_user_id"

user_body="$(jq -cn --arg email "$email" --arg username "$username" --arg password "$password" '{email:$email,username:$username,password:$password}')"
user_id="$(api_request "create probe user" POST http://localhost:8065/api/v4/users "$admin_token" "$user_body" | jq -r '.id')"
test -n "$user_id"

user_login_headers="$(mktemp)"
user_login_body="$(jq -cn --arg login_id "$username" --arg password "$password" '{login_id:$login_id,password:$password}')"
curl --silent --show-error --fail -D "$user_login_headers" -o /tmp/internkim-probe-user-login.json \
  -H "Content-Type: application/json" \
  -d "$user_login_body" \
  http://localhost:8065/api/v4/users/login >/dev/null
user_token="$(awk 'tolower($1) == "token:" {print $2}' "$user_login_headers" | tr -d '\r')"
test -n "$user_token"

blueclaw_request "invite probe user" POST http://127.0.0.1:8080/admin/api/people/invite \
  "$(jq -cn --arg email "$email" '{email:$email}')" >/dev/null

channel_id="$(api_request "create probe dm" POST http://localhost:8065/api/v4/channels/direct "$user_token" \
  "$(jq -cn --arg user_id "$user_id" --arg bot_user_id "$bot_user_id" '[$user_id,$bot_user_id]')" | jq -r '.id')"
test -n "$channel_id"

post_body="$(jq -cn --arg channel_id "$channel_id" --arg message "$prompt" '{channel_id:$channel_id,message:$message}')"
user_post="$(api_request "post probe message" POST http://localhost:8065/api/v4/posts "$user_token" "$post_body")"
user_post_id="$(printf '%%s' "$user_post" | jq -r '.id')"
user_post_create_at="$(printf '%%s' "$user_post" | jq -r '.create_at')"
test -n "$user_post_id"

bot_post_id=""
for _ in $(seq 1 "$timeout_seconds"); do
  bot_post_id="$(api_request "wait for probe reply" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=60" "$admin_token" |
    jq -r --arg bot_user_id "$bot_user_id" --argjson posted_after "$user_post_create_at" \
      '.posts[] | select(.user_id == $bot_user_id and .create_at >= $posted_after) | .id' | head -1)"
  if [ -n "$bot_post_id" ]; then
    break
  fi
  sleep 1
done
if [ -z "$bot_post_id" ]; then
  echo "expected Mattermost bot reply for probe post $user_post_id" >&2
  exit 1
fi

bot_post_file="$(mktemp)"
api_request "fetch probe reply" GET "http://localhost:8065/api/v4/posts/$bot_post_id" "$admin_token" > "$bot_post_file"
task_run_id="$(blueclaw_request "find probe task" GET http://127.0.0.1:8080/admin/api/task |
  jq -r --arg prompt "$prompt" '[.[] | select(.prompt == $prompt)] | sort_by(.createdAt) | last | .taskRunID // empty')"
task_detail_file="$(mktemp)"
printf '{}' > "$task_detail_file"
if [ -n "$task_run_id" ]; then
  blueclaw_request "probe task detail" GET "http://127.0.0.1:8080/admin/api/task/detail?taskRunID=$task_run_id" > "$task_detail_file"
fi
browser_open_verified=false
if [ "$expect_browser_open" = "true" ]; then
  if [ -z "$task_run_id" ]; then
    echo "expected successful browser.open result, but no task was created for probe prompt" >&2
    exit 1
  fi
  if jq -e 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; .name == "tool.browser.open.result" and (((.body // "{}") | fromjson? // {}) | .isError != true))' "$task_detail_file" >/dev/null; then
    browser_open_verified=true
  else
    echo "expected successful tool.browser.open.result for probe task $task_run_id" >&2
    jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
    exit 1
  fi
fi

for expected_tool in $(printf '%%s' "$expected_tools_json" | jq -r '.[]'); do
  if [ -z "$task_run_id" ]; then
    echo "expected tool.$expected_tool.requested, but no task was created for probe prompt" >&2
    exit 1
  fi
  if ! jq -e --arg name "tool.$expected_tool.requested" --arg fragment "$expected_tool" 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; .name == $name and ((.body // "") | tostring | contains($fragment)))' "$task_detail_file" >/dev/null; then
    echo "expected requested tool event for $expected_tool in task $task_run_id" >&2
    jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
    exit 1
  fi
done

for expected_event in $(printf '%%s' "$expected_events_json" | jq -r '.[]'); do
  if [ -z "$task_run_id" ]; then
    echo "expected event $expected_event, but no task was created for probe prompt" >&2
    exit 1
  fi
  if ! jq -e --arg name "$expected_event" 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; .name == $name)' "$task_detail_file" >/dev/null; then
    echo "expected task event $expected_event in task $task_run_id" >&2
    jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
    exit 1
  fi
done

jq -cn \
  --arg channel_id "$channel_id" \
  --arg user_post_id "$user_post_id" \
  --arg bot_post_id "$bot_post_id" \
  --arg task_run_id "$task_run_id" \
  --argjson keep "$keep_artifacts" \
  --argjson browser_open_verified "$browser_open_verified" \
  --slurpfile bot_post "$bot_post_file" \
  --slurpfile task_detail "$task_detail_file" \
  '{
    ok: true,
    kept: $keep,
    channelID: $channel_id,
    userPostID: $user_post_id,
    botPostID: $bot_post_id,
    taskRunID: $task_run_id,
    browserOpenVerified: $browser_open_verified,
    botMessage: $bot_post[0].message,
    fileIDs: ($bot_post[0].file_ids // []),
    taskStatus: ((if ($task_detail[0] | type) == "array" then $task_detail[0][0] else $task_detail[0] end).taskRun.status // null),
    taskEvents: (((if ($task_detail[0] | type) == "array" then $task_detail[0][0] else $task_detail[0] end).taskEvents // []) | map({name, body: ((.body // "") | tostring | .[0:1200])}))
  }'
`, strconv.Quote(encodedPrompt), strconv.Quote(encodedExpectedTools), strconv.Quote(encodedExpectedEvents), keepValue, timeoutSeconds, expectBrowserOpenValue)
}

func prepareMattermostBrowserOpenE2EScript() string {
	return `set -euo pipefail

timestamp="$(date +%s)"
email="probe-browser-open-$timestamp@internkim.test"
username="probebrowser$timestamp"
password="ProbePass!$timestamp"

api_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local token="${4:-}"
  local body="${5:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
      -X "$method" -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
      -d "$body" "$url")" || curl_status="$?"
  else
    status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
      -X "$method" -H "Authorization: Bearer $token" "$url")" || curl_status="$?"
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Mattermost API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Mattermost API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

admind_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local body="${4:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
    -X "$method" -H "Content-Type: application/json" -d "$body" "$url")" || curl_status="$?"
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Admind API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Admind API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

login_headers="$(mktemp)"
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
curl --silent --show-error --fail -D "$login_headers" -o /tmp/internkim-admin-browser-e2e-login.json \
  -H "Content-Type: application/json" \
  -d "$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')" \
  http://localhost:8065/api/v4/users/login >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
test -n "$admin_token"

stale_users_file="$(mktemp)"
api_request "search stale probe browser users" POST http://localhost:8065/api/v4/users/search "$admin_token" \
  "$(jq -cn --arg term probe-browser-open '{term:$term}')" > "$stale_users_file"
jq -r '.[] | select((.email // "") | startswith("probe-browser-open-")) | [.id, .email] | @tsv' "$stale_users_file" |
while IFS="$(printf '\t')" read -r stale_user_id stale_email; do
  if [ -n "$stale_user_id" ]; then
    curl --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
      "http://localhost:8065/api/v4/users/$stale_user_id?permanent=true" >/dev/null || true
  fi
  if [ -n "$stale_email" ]; then
    curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$stale_email" >/dev/null || true
  fi
done

mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
bot_user_id="$(api_request "bot lookup" GET http://localhost:8065/api/v4/users/me "$mattermost_token" | jq -r '.id // empty')"
test -n "$bot_user_id"

user_body="$(jq -cn --arg email "$email" --arg username "$username" --arg password "$password" '{email:$email,username:$username,password:$password}')"
user_id="$(api_request "create probe browser user" POST http://localhost:8065/api/v4/users "$admin_token" "$user_body" | jq -r '.id')"
test -n "$user_id"

admind_request "invite probe browser user" POST http://127.0.0.1:8080/admin/api/people/invite \
  "$(jq -cn --arg email "$email" '{email:$email}')" >/dev/null

user_login_headers="$(mktemp)"
curl --silent --show-error --fail -D "$user_login_headers" -o /tmp/internkim-probe-browser-e2e-login.json \
  -H "Content-Type: application/json" \
  -d "$(jq -cn --arg login_id "$username" --arg password "$password" '{login_id:$login_id,password:$password}')" \
  http://localhost:8065/api/v4/users/login >/dev/null
user_token="$(awk 'tolower($1) == "token:" {print $2}' "$user_login_headers" | tr -d '\r')"
test -n "$user_token"

channel_id="$(api_request "create probe browser dm" POST http://localhost:8065/api/v4/channels/direct "$user_token" \
  "$(jq -cn --arg user_id "$user_id" --arg bot_user_id "$bot_user_id" '[$user_id,$bot_user_id]')" | jq -r '.id')"
test -n "$channel_id"

device_url="$(cat /root/.internkim/env/device-url)"
pairing_body="$(jq -cn \
  --arg owner_platform mattermost \
  --arg owner_platform_user_id "$user_id" \
  --arg owner_email "$email" \
  --arg owner_name "$username" \
  --arg device_url "$device_url" \
  '{ownerPlatform:$owner_platform,ownerPlatformUserID:$owner_platform_user_id,ownerEmail:$owner_email,ownerName:$owner_name,deviceURL:$device_url}')"
pairing_document="$(admind_request "create probe companion pairing" POST http://127.0.0.1:18080/_internkim/companion/pairing-codes "$pairing_body")"
pairing_code="$(printf '%s' "$pairing_document" | jq -r '.code // empty')"
test -n "$pairing_code"

jq -cn \
  --arg device_url "$device_url" \
  --arg code "$pairing_code" \
  --arg email "$email" \
  --arg username "$username" \
  --arg password "$password" \
  --arg user_id "$user_id" \
  --arg channel_id "$channel_id" \
  '{deviceURL:$device_url,code:$code,email:$email,username:$username,password:$password,userID:$user_id,channelID:$channel_id}'
`
}

func cleanupMattermostBrowserOpenE2EScript(preparation mattermostBrowserOpenE2EPreparation) string {
	return fmt.Sprintf(`set -euo pipefail

email="$(printf '%%s' %s | base64 -d)"
user_id="$(printf '%%s' %s | base64 -d)"
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
login_headers="$(mktemp)"
curl --silent --show-error --fail -D "$login_headers" -o /tmp/internkim-admin-browser-e2e-cleanup-login.json \
  -H "Content-Type: application/json" \
  -d "$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')" \
  http://localhost:8065/api/v4/users/login >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
if [ -n "$user_id" ]; then
  curl --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
    "http://localhost:8065/api/v4/users/$user_id?permanent=true" >/dev/null || true
fi
curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$email" >/dev/null || true
`, strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.Email))), strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.UserID))))
}

func waitMattermostBrowserOpenE2ECompanionOnlineScript(preparation mattermostBrowserOpenE2EPreparation, timeoutSeconds int) string {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 240
	}
	return fmt.Sprintf(`set -euo pipefail

owner_platform_user_id="$(printf '%%s' %s | base64 -d)"
owner_email="$(printf '%%s' %s | base64 -d)"
timeout_seconds=%d
deadline=$((SECONDS + timeout_seconds))

while [ "$SECONDS" -lt "$deadline" ]; do
  status_document="$(curl --silent --show-error http://127.0.0.1:18080/admin/api/companion/status)"
  if printf '%%s' "$status_document" | jq -e --arg owner_platform_user_id "$owner_platform_user_id" --arg owner_email "$owner_email" '
    any(.companions[]?;
      .isOnline == true
      and (.ownerPlatformUserID == $owner_platform_user_id or (.ownerEmail | ascii_downcase) == ($owner_email | ascii_downcase))
      and any(.capabilities[]?; .name == "browser.open")
    )
  ' >/dev/null; then
    echo "remote companion: online with browser.open"
    exit 0
  fi
  sleep 1
done

echo "expected remote companion heartbeat with browser.open for $owner_email" >&2
curl --silent --show-error http://127.0.0.1:18080/admin/api/companion/status |
  jq --arg owner_platform_user_id "$owner_platform_user_id" --arg owner_email "$owner_email" '
    .companions
    | map(select(.ownerPlatformUserID == $owner_platform_user_id or (.ownerEmail | ascii_downcase) == ($owner_email | ascii_downcase)))
    | map({companionID, ownerPlatformUserID, ownerEmail, isOnline, lastSeenAt, capabilityNames: (.capabilities | map(.name))})
  ' >&2
exit 1
`,
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.UserID))),
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.Email))),
		timeoutSeconds,
	)
}

func runMattermostBrowserOpenE2EScript(prompt string, keep bool, timeoutSeconds int, preparation mattermostBrowserOpenE2EPreparation) string {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 240
	}
	keepValue := "false"
	if keep {
		keepValue = "true"
	}
	return fmt.Sprintf(`set -euo pipefail

prompt="$(printf '%%s' %s | base64 -d)"
email="$(printf '%%s' %s | base64 -d)"
username="$(printf '%%s' %s | base64 -d)"
password="$(printf '%%s' %s | base64 -d)"
user_id="$(printf '%%s' %s | base64 -d)"
channel_id="$(printf '%%s' %s | base64 -d)"
keep_artifacts=%s
timeout_seconds=%d

api_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local token="${4:-}"
  local body="${5:-}"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  if [ -n "$body" ]; then
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" -H "Authorization: Bearer $token" -H "Content-Type: application/json" \
      -d "$body" "$url")" || curl_status="$?"
  else
    status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" \
      -X "$method" -H "Authorization: Bearer $token" "$url")" || curl_status="$?"
  fi
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Mattermost API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Mattermost API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

blueclaw_request() {
  local phase_name="$1"
  local method="$2"
  local url="$3"
  local response_file
  local status
  local curl_status
  response_file="$(mktemp)"
  status="$(curl --silent --show-error --output "$response_file" --write-out "%%{http_code}" -X "$method" "$url")" || curl_status="$?"
  if [ "${curl_status:-0}" != "0" ]; then
    echo "Blueclaw API curl failure during $phase_name: $method $url (curl exit ${curl_status:-0})" >&2
    cat "$response_file" >&2 || true
    rm -f "$response_file"
    return "${curl_status:-1}"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "Blueclaw API failure during $phase_name: $method $url returned HTTP $status" >&2
    cat "$response_file" >&2 || true
    echo >&2
    rm -f "$response_file"
    return 22
  fi
  cat "$response_file"
  rm -f "$response_file"
}

login_headers="$(mktemp)"
admin_password="$(cat /root/.internkim/secrets/mm-admin-pass)"
curl --silent --show-error --fail -D "$login_headers" -o /tmp/internkim-admin-browser-e2e-run-login.json \
  -H "Content-Type: application/json" \
  -d "$(jq -cn --arg login_id admin --arg password "$admin_password" '{login_id:$login_id,password:$password}')" \
  http://localhost:8065/api/v4/users/login >/dev/null
admin_token="$(awk 'tolower($1) == "token:" {print $2}' "$login_headers" | tr -d '\r')"
test -n "$admin_token"

cleanup() {
  if [ "$keep_artifacts" = "true" ]; then
    return 0
  fi
  mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
  if [ -n "${bot_post_id:-}" ]; then
    curl --silent --show-error -X DELETE -H "Authorization: Bearer $mattermost_token" \
      "http://localhost:8065/api/v4/posts/$bot_post_id" >/dev/null || true
  fi
  if [ -n "${user_post_id:-}" ]; then
    curl --silent --show-error -X DELETE -H "Authorization: Bearer $user_token" \
      "http://localhost:8065/api/v4/posts/$user_post_id" >/dev/null || true
  fi
  curl --silent --show-error -X DELETE -H "Authorization: Bearer $admin_token" \
    "http://localhost:8065/api/v4/users/$user_id?permanent=true" >/dev/null || true
  curl --silent --show-error -X DELETE "http://127.0.0.1:8080/admin/api/people?email=$email" >/dev/null || true
}
trap cleanup EXIT

user_login_headers="$(mktemp)"
curl --silent --show-error --fail -D "$user_login_headers" -o /tmp/internkim-probe-browser-e2e-run-user-login.json \
  -H "Content-Type: application/json" \
  -d "$(jq -cn --arg login_id "$username" --arg password "$password" '{login_id:$login_id,password:$password}')" \
  http://localhost:8065/api/v4/users/login >/dev/null
user_token="$(awk 'tolower($1) == "token:" {print $2}' "$user_login_headers" | tr -d '\r')"
test -n "$user_token"

mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token)"
bot_user_id="$(api_request "bot lookup" GET http://localhost:8065/api/v4/users/me "$mattermost_token" | jq -r '.id // empty')"
test -n "$bot_user_id"

post_body="$(jq -cn --arg channel_id "$channel_id" --arg message "$prompt" '{channel_id:$channel_id,message:$message}')"
user_post="$(api_request "post probe browser message" POST http://localhost:8065/api/v4/posts "$user_token" "$post_body")"
user_post_id="$(printf '%%s' "$user_post" | jq -r '.id')"
user_post_create_at="$(printf '%%s' "$user_post" | jq -r '.create_at')"
test -n "$user_post_id"

bot_post_id=""
for _ in $(seq 1 "$timeout_seconds"); do
  bot_post_id="$(api_request "wait for probe browser reply" GET "http://localhost:8065/api/v4/channels/$channel_id/posts?per_page=60" "$admin_token" |
    jq -r --arg bot_user_id "$bot_user_id" --argjson posted_after "$user_post_create_at" \
      '.posts[] | select(.user_id == $bot_user_id and .create_at >= $posted_after) | .id' | head -1)"
  if [ -n "$bot_post_id" ]; then
    break
  fi
  sleep 1
done
if [ -z "$bot_post_id" ]; then
  echo "expected Mattermost bot reply for probe browser post $user_post_id" >&2
  exit 1
fi

bot_post_file="$(mktemp)"
api_request "fetch probe browser reply" GET "http://localhost:8065/api/v4/posts/$bot_post_id" "$admin_token" > "$bot_post_file"
task_run_id="$(blueclaw_request "find probe browser task" GET http://127.0.0.1:8080/admin/api/task |
  jq -r --arg prompt "$prompt" '[.[] | select(.prompt == $prompt)] | sort_by(.createdAt) | last | .taskRunID // empty')"
if [ -z "$task_run_id" ]; then
  echo "expected task for probe browser prompt" >&2
  exit 1
fi
task_detail_file="$(mktemp)"
blueclaw_request "probe browser task detail" GET "http://127.0.0.1:8080/admin/api/task/detail?taskRunID=$task_run_id" > "$task_detail_file"
if ! jq -e 'def detail: if type == "array" then .[0] else . end; any((detail.taskEvents // [])[]; .name == "tool.browser.open.result" and (((.body // "{}") | fromjson? // {}) | .isError != true))' "$task_detail_file" >/dev/null; then
  echo "expected successful tool.browser.open.result for probe browser task $task_run_id" >&2
  jq 'def detail: if type == "array" then .[0] else . end; detail.taskEvents // [] | map({name, body})' "$task_detail_file" >&2 || true
  exit 1
fi

jq -cn \
  --arg channel_id "$channel_id" \
  --arg user_post_id "$user_post_id" \
  --arg bot_post_id "$bot_post_id" \
  --arg task_run_id "$task_run_id" \
  --argjson keep "$keep_artifacts" \
  --slurpfile bot_post "$bot_post_file" \
  --slurpfile task_detail "$task_detail_file" \
  '{
    ok: true,
    kept: $keep,
    browserOpenVerified: true,
    channelID: $channel_id,
    userPostID: $user_post_id,
    botPostID: $bot_post_id,
    taskRunID: $task_run_id,
    botMessage: $bot_post[0].message,
    taskStatus: ((if ($task_detail[0] | type) == "array" then $task_detail[0][0] else $task_detail[0] end).taskRun.status // null),
    taskEvents: (((if ($task_detail[0] | type) == "array" then $task_detail[0][0] else $task_detail[0] end).taskEvents // []) | map({name, body: ((.body // "") | tostring | .[0:1200])}))
  }'
`,
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(prompt))),
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.Email))),
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.Username))),
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.Password))),
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.UserID))),
		strconv.Quote(base64.StdEncoding.EncodeToString([]byte(preparation.ChannelID))),
		keepValue,
		timeoutSeconds,
	)
}
