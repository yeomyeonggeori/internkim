package cli

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"

	"gitlab.com/eastriver/internkim/internal/blueclawworkspace"
	"gitlab.com/eastriver/internkim/internal/capabilities"
	internkimlab "gitlab.com/eastriver/internkim/internal/lab"
	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
	setup "gitlab.com/eastriver/internkim/internal/provisioning/steps"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
)

var (
	boardUser          = "root"
	boardPass          = ""
	registerHTTPClient = &http.Client{Timeout: 30 * time.Second}
	statusHTTPClient   = &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Armbian Trixie Minimal images per board
	armbianImages = map[string]string{
		"rpi":       "https://dl.armbian.com/rpi4b/Trixie_current_minimal",     // RPi 3/4/5
		"orangepi5": "https://dl.armbian.com/orangepi5/Trixie_current_minimal", // Orange Pi 5 (RK3588S)
	}
)

const jetsonDefaultUser = "internkim"
const mattermostDefaultPushNotificationServer = "https://push-test.mattermost.com"

type config struct {
	APIBaseURL     string
	RegisterSecret string
	CFDomain       string
}

func loadConfig() config {
	loadEnvFile()
	return config{
		APIBaseURL:     envOr("INTERNKIM_API_URL", "https://api.example.test"),
		RegisterSecret: envOr("INTERNKIM_REGISTER_SECRET", ""),
		CFDomain:       envOr("INTERNKIM_DOMAIN", "example.test"),
	}
}

func updateEnvFile(key, value string) error {
	const path = ".env"
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	prefix := key + "="
	updated := false
	for index, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[index] = prefix + value
			updated = true
			break
		}
	}
	if !updated {
		lines = append(lines, prefix+value)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

func loadEnvFile() {
	for _, path := range []string{".env", "../.env"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if k, v, ok := strings.Cut(line, "="); ok {
				k = strings.TrimSpace(k)
				v = normalizeEnvValue(v)
				if os.Getenv(k) == "" {
					os.Setenv(k, v)
				}
			}
		}
	}
}

func normalizeEnvValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return value
	}
	if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return strings.Trim(value, `"`)
	}
	if strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`) {
		return strings.Trim(value, `'`)
	}
	return value
}

func currentExecutableFingerprint() string {
	executablePath, err := currentExecutablePath()
	if err != nil {
		return ""
	}
	file, err := os.Open(executablePath)
	if err != nil {
		return ""
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return ""
	}
	return hex.EncodeToString(hasher.Sum(nil))[:16]
}

func currentExecutablePath() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolvedPath, err := filepath.EvalSymlinks(executablePath); err == nil {
		executablePath = resolvedPath
	}
	return executablePath, nil
}

func resolveRepositoryRootPath() (string, error) {
	workingDirectoryPath, errorValue := os.Getwd()
	if errorValue != nil {
		return "", errorValue
	}

	searchPath := workingDirectoryPath
	for {
		if _, errorValue := os.Stat(filepath.Join(searchPath, "go.mod")); errorValue == nil {
			return searchPath, nil
		}
		parentPath := filepath.Dir(searchPath)
		if parentPath == searchPath {
			return "", errors.New("could not find repository root")
		}
		searchPath = parentPath
	}
}

func resolveLabVirtualMachineIPAddress() string {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return ""
	}

	configurationPath := internkimlab.DefaultConfigurationPath(repositoryRootPath)
	configuration, errorValue := internkimlab.LoadConfiguration(configurationPath)
	if errorValue != nil {
		return ""
	}

	service := internkimlab.NewService(
		configuration,
		internkimlab.OperatingSystemCommandRunner{},
		repositoryRootPath,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	virtualMachineIPAddress, errorValue := service.VirtualMachineIPAddress(ctx)
	if errorValue != nil {
		return ""
	}

	return strings.TrimSpace(virtualMachineIPAddress)
}

func Main() {
	loadEnvFile()

	if len(os.Args) > 1 {
		if os.Args[1] == "--help" || os.Args[1] == "-h" {
			printUsage()
			return
		}
		switch os.Args[1] {
		case "setup":
			runSetup()
		case "flash":
			runFlash()
		case "wifi":
			runWiFi()
		case "ssh":
			runDeviceSSH()
		case "model":
			runModel()
		case "sync-tools":
			runSyncTools()
		case "migrate":
			runMigrate()
		case "invite":
			runInvite()
		case "users":
			runUsers()
		case "task":
			runTask()
		case "reset":
			runReset()
		case "recover":
			runRecover()
		case "release":
			runRelease()
		case "status":
			runStatus()
		case "update":
			runUpdate()
		case "deploy":
			runDeploy()
		case "doctor":
			runDoctor()
		case "verify":
			runVerify()
		case "test":
			runTest()
		case "llm":
			runLLM()
		case "ops":
			runOps()
		case "tenant":
			runTenant()
		case "host":
			runHost()
		case "dev":
			runDev()
		case "lab":
			runLab()
		case "sim":
			runSim()
		default:
			printUsage()
		}
		return
	}
	runSetup()
}

func printUsage() {
	fmt.Println("Usage: internkim <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  setup    Full device provisioning")
	fmt.Println("  flash    Flash board boot media")
	fmt.Println("  wifi     Add or update Jetson Wi-Fi profiles")
	fmt.Println("  ssh      Open SSH to the device")
	fmt.Println("  model    Manage LLM model (current/set/list)")
	fmt.Println("  migrate  Migrate device metadata without full setup")
	fmt.Println("  invite   Add/invite an allowed user")
	fmt.Println("  users    Manage allowed users")
	fmt.Println("  task     Inspect task runs and failure logs")
	fmt.Println("  reset    Reset board runtime data")
	fmt.Println("  recover  Recover narrow device maintenance paths")
	fmt.Println("  release  Publish and inspect release sets")
	fmt.Println("  status   Check board and tunnel status")
	fmt.Println("  update   Deploy current build to device")
	fmt.Println("  deploy   Build and apply a signed release over Admin HTTPS")
	fmt.Println("  doctor   Check host dependencies")
	fmt.Println("  verify   Run API, Mattermost, and browser verification")
	fmt.Println("  test     Run a prompt through disposable Local Fleet; use -o <file> for one returned attachment")
	fmt.Println("  llm      One-shot LLM ping (local by default, --remote for OpenRouter)")
	fmt.Println("  ops      Serve the local personal fleet console")
	fmt.Println("  tenant   Manage PoC tenant runtime manifests")
	fmt.Println("  host     Manage Mac-hosted tenant VM plumbing")
	fmt.Println("  lab      Run container-based Blueclaw-aligned lab workflows")
	fmt.Println("  sim      Deprecated alias for lab")
}

func runSetup() {
	if containsArg("--help") || containsArg("-h") {
		printSetupUsage()
		return
	}
	lang := "ko"
	if containsArg("--en") {
		lang = "en"
	}
	sim := containsArg("--sim")
	m := newMsg(lang)

	if sim {
		runSetupSimulation(setupControlArguments(os.Args[2:]))
		return
	}

	if containsArg("--list-steps") {
		setup.DefaultRegistry().PrintSteps()
		return
	}

	boardType := argString("--board", setup.BoardJetsonOrinNano)

	// Pipeline mode: selective re-run (auto/ssh/sd backend).
	// Jetson is the default live setup target. Legacy SD flashing is still
	// available by explicitly passing --board rpi/orangepi5 without live flags.
	if hasFlag("--only") || hasFlag("--skip") || hasFlag("--from") ||
		containsArg("--force") || containsArg("--force-all") || containsArg("--plan") ||
		containsArg("--ssh") || containsArg("--sd") || containsArg("--live") || containsArg("--with-google") ||
		containsArg("--wait-lock") ||
		argString("--host", "") != "" || argString("--slack-bot-token", "") != "" ||
		argString("--slack-app-token", "") != "" || argString("--signal-jsonrpc-url", "") != "" ||
		argString("--signal-account", "") != "" ||
		boardType == setup.BoardJetsonOrinNano {
		runSetupLive(m)
		return
	}

	runSetupSD(m)
}

func printSetupUsage() {
	fmt.Println("Usage: internkim setup [options]")
	fmt.Println()
	fmt.Println("Common options:")
	fmt.Println("  --only <steps>       Run only selected setup steps, for example web, admind, or capabilityd")
	fmt.Println("  --force              Re-run selected steps even when state says they are complete")
	fmt.Println("  --force-all          Re-run every selected setup step")
	fmt.Println("  --host <ip>          Override the saved board IP")
	fmt.Println("  --user <name>        Override the SSH user")
	fmt.Println("  --password <value>   Override the SSH password")
	fmt.Println("  --profile <name>     Use an isolated company/customer profile")
	fmt.Println("  --node <number>      Target a numbered fleet node")
	fmt.Println("  --fleet <fleet-id>   Join an existing fleet")
	fmt.Println("  --fleet-secret <s>   Secret for joining an existing fleet (or INTERNKIM_FLEET_SECRET)")
	fmt.Println("  --plan               Print the selected setup plan")
	fmt.Println("  --wait-lock          Wait for another setup on the same target instead of failing fast")
	fmt.Println("  --list-steps         Print available setup steps")
	fmt.Println("  --sim                Run the container lab simulation flow")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  internkim setup --only web --force")
	fmt.Println("  internkim setup --only admind --force")
	fmt.Println("  internkim setup --only web,admind --force")
	fmt.Println("  internkim setup --profile acme --node 1 --only blueclaw-payload-direct --force")
}

func runDeviceSSH() {
	configuration := loadConfig()
	scriptDir, _ := os.Getwd()
	sshpassBin := filepath.Join(scriptDir, "bin", "sshpass")
	arguments := commandControlArguments(os.Args[2:])
	target := resolveCommandTarget(arguments)
	connection, isRemote, errorValue := resolveDeviceSSHConnection(configuration, sshpassBin, target)
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	target.host = connection.host
	target.useRemoteSSH = isRemote
	printCommandTargetEvidence(target)
	if isRemote {
		fmt.Printf("Backend: cloudflare ssh\n")
	} else {
		fmt.Printf("Backend: local ssh\n")
	}
	if errorValue := connection.runInteractiveSSH(commandRemoteArguments(os.Args[2:])); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func resolveDeviceSSHConnection(configuration config, sshpassBin string, target commandTarget) (*sshClient, bool, error) {
	if target.useRemoteSSH {
		return resolveCloudflareSSHConnection(configuration, sshpassBin, target, true)
	}
	if connection := resolveLocalSSHConnection(sshpassBin, target); connection != nil {
		return connection, false, nil
	}
	return resolveCloudflareSSHConnection(configuration, sshpassBin, target, false)
}

func resolveLocalSSHConnection(sshpassBin string, target commandTarget) *sshClient {
	if strings.TrimSpace(target.host) != "" {
		connection := newSSH(sshpassBin, target.sshUser, target.sshPassword, target.host)
		if _, errorValue := connection.runResult("true"); errorValue == nil {
			return connection
		}
		return nil
	}
	host := findBoardIPForCredentials(sshpassBin, target.stateDir, target.sshUser, target.sshPassword)
	if host == "" {
		return nil
	}
	return newSSH(sshpassBin, target.sshUser, target.sshPassword, host)
}

func resolveCloudflareSSHConnection(configuration config, sshpassBin string, target commandTarget, isRequired bool) (*sshClient, bool, error) {
	hostname, errorValue := ensureCloudflareSSHRegistration(configuration, target.stateDir, false)
	if errorValue != nil && isRequired {
		return nil, false, errorValue
	}
	if errorValue == nil && hostname != "" {
		target.sshHostname = hostname
	}
	if target.sshHostname == "" {
		target.sshHostname = resolveCloudflareSSHHostname(configuration, target)
	}
	if target.sshHostname == "" {
		return nil, false, errors.New("device is not reachable locally and no Cloudflare SSH route is registered; run setup once on the device network first")
	}
	if status := cloudflareSSHTLSStatus(target.stateDir); !cloudflareSSHTLSIsReady(status) {
		message := fmt.Sprintf("Cloudflare SSH TLS certificate is %s for %s; wait a few minutes until it becomes active or run setup over local SSH", status, target.sshHostname)
		if isRequired {
			return nil, false, errors.New(message)
		}
		return nil, false, nil
	}
	if errorValue := ensureCloudflaredAccessSSHAvailable(); errorValue != nil {
		return nil, false, errorValue
	}
	connection := newCloudflareSSH(sshpassBin, target.sshUser, target.sshPassword, target.sshHostname)
	if output, errorValue := connection.runResult("true"); errorValue != nil {
		return nil, false, formatCloudflareSSHError(target.sshHostname, output, errorValue)
	}
	return connection, true, nil
}

func formatCloudflareSSHError(hostname string, output string, errorValue error) error {
	detail := strings.TrimSpace(output)
	message := fmt.Sprintf("Cloudflare SSH failed for %s: %v", hostname, errorValue)
	if detail != "" {
		message = fmt.Sprintf("Cloudflare SSH failed for %s: %s: %v", hostname, detail, errorValue)
	}
	return fmt.Errorf("%s\n%s", message, cloudflareSSHRecoveryHint(hostname, detail))
}

func cloudflareSSHRecoveryHint(hostname string, detail string) string {
	normalizedDetail := strings.ToLower(detail)
	if strings.Contains(normalizedDetail, "banner exchange") {
		return "Cloudflare Access 프록시는 열렸지만 SSH banner를 받지 못했습니다. 브라우저 인증보다 장비의 `sshd` 또는 `cloudflared-node-ssh` 터널 상태를 먼저 확인하세요. HTTP가 살아 있으면 `./internkim recover ssh`로 SSH 터널 복구를 시도하세요."
	}
	return fmt.Sprintf("Cloudflare Access 인증이 만료되었을 수 있습니다. `cloudflared access ssh --hostname %s`로 브라우저 인증을 갱신한 뒤 다시 실행하세요.", hostname)
}

func cloudflareSSHFailureClass(message string) string {
	normalizedMessage := strings.ToLower(message)
	switch {
	case strings.Contains(normalizedMessage, "banner exchange"):
		return "origin_banner_timeout"
	case strings.Contains(normalizedMessage, "lookup"):
		return "dns"
	case strings.Contains(normalizedMessage, "access") || strings.Contains(normalizedMessage, "authenticate") || strings.Contains(normalizedMessage, "forbidden"):
		return "access_auth"
	case strings.Contains(normalizedMessage, "connect to host") || strings.Contains(normalizedMessage, "operation timed out"):
		return "direct_lan_unreachable"
	default:
		return "unknown"
	}
}

func cloudflareSSHFailureSummary(errorValue error) string {
	if errorValue == nil {
		return "ok"
	}
	failureClass := cloudflareSSHFailureClass(errorValue.Error())
	switch failureClass {
	case "origin_banner_timeout":
		return "origin_banner_timeout — SSH tunnel origin did not return an SSH banner; try `./internkim recover ssh`."
	case "access_auth":
		return "access_auth — refresh Cloudflare Access authentication."
	case "dns":
		return "dns — SSH hostname did not resolve."
	case "direct_lan_unreachable":
		return "direct_lan_unreachable — direct SSH target did not respond."
	default:
		return "unknown — inspect Cloudflare SSH and device network state."
	}
}

func cloudflareSSHTLSStatus(stateDir string) string {
	status := strings.TrimSpace(loadState(stateDir, "tls_certificate_status"))
	if status == "" {
		return "unknown"
	}
	return status
}

func cloudflareSSHTLSIsReady(status string) bool {
	return status == "active" || status == "covered" || status == "unknown"
}

func commandControlArguments(arguments []string) []string {
	for index, argument := range arguments {
		if argument == "--" {
			return arguments[:index]
		}
	}
	return arguments
}

func commandRemoteArguments(arguments []string) []string {
	for index, argument := range arguments {
		if argument == "--" {
			return arguments[index+1:]
		}
	}
	return nil
}

// --- Model management ---

func runModel() {
	sub := ""
	if len(os.Args) > 2 {
		sub = os.Args[2]
	}

	configuration := loadConfig()
	scriptDir, _ := os.Getwd()
	sshpassBin := filepath.Join(scriptDir, "bin", "sshpass")
	target := resolveCommandTarget(commandControlArguments(os.Args[2:]))
	target = resolveLabHostForCommandTarget(target, scriptDir)
	ssh, _, errorValue := resolveDeviceSSHConnection(configuration, sshpassBin, target)
	if errorValue != nil {
		fatal(errorValue.Error())
	}

	switch sub {
	case "current", "":
		modelCurrentCmd(ssh)
	case "set":
		if len(os.Args) < 4 {
			fmt.Println("Usage: internkim model set <model-id>")
			fmt.Println("Example: internkim model set " + blueclaw.BlueclawDefaultModelName)
			os.Exit(1)
		}
		modelSetCmd(ssh, os.Args[3])
	case "list":
		modelListCmd(ssh)
	default:
		fmt.Println("Usage: internkim model <current|set|list>")
	}
}

func runSyncTools() {
	configuration := loadConfig()
	scriptDir, _ := os.Getwd()
	sshpassBin := filepath.Join(scriptDir, "bin", "sshpass")
	target := resolveCommandTarget(commandControlArguments(os.Args[2:]))
	target = resolveLabHostForCommandTarget(target, scriptDir)
	ssh, _, errorValue := resolveDeviceSSHConnection(configuration, sshpassBin, target)
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	syncToolDescriptorsCmd(ssh)
}

func syncToolDescriptorsCmd(ssh *sshClient) {
	currentDocument := strings.TrimSpace(ssh.run("cat " + blueclaw.BlueclawRuntimeConfigPath + " 2>/dev/null"))
	var document map[string]any
	if err := json.Unmarshal([]byte(currentDocument), &document); err != nil {
		fatal("Failed to parse blueclaw runtime config: " + err.Error())
	}
	capabilitiesSection, _ := document["capabilities"].(map[string]any)
	if capabilitiesSection == nil {
		fatal("blueclaw runtime config has no capabilities section")
	}
	capabilitiesSection["toolNames"] = capabilities.DefaultToolNames()
	capabilitiesSection["toolDescriptors"] = capabilities.DefaultToolDescriptors()
	updatedDocument, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		fatal("Failed to write blueclaw runtime config: " + err.Error())
	}
	temporaryPath := filepath.Join(os.TempDir(), "blueclaw-runtime.json")
	if err := os.WriteFile(temporaryPath, append(updatedDocument, '\n'), 0o600); err != nil {
		fatal("Failed to stage blueclaw runtime config: " + err.Error())
	}
	defer os.Remove(temporaryPath)
	for _, runtimeConfigPath := range blueclawRuntimeConfigPaths() {
		if errorValue := ssh.scp(temporaryPath, runtimeConfigPath); errorValue != nil {
			fatal("Failed to upload blueclaw runtime config: " + errorValue.Error())
		}
	}
	ssh.run("chown root:" + blueclaw.BlueclawUser + " " + quoteShellValues(blueclawRuntimeConfigPaths()) + " && chmod 640 " + quoteShellValues(blueclawRuntimeConfigPaths()))
	ssh.run("systemctl restart " + blueclaw.BlueclawServiceName + " 2>/dev/null")
	fmt.Printf("Synced %d tool descriptors to blueclaw runtime config.\n", len(capabilities.DefaultToolDescriptors()))
	fmt.Println("blueclaw restarted.")
}

func modelCurrentCmd(ssh *sshClient) {
	raw := strings.TrimSpace(ssh.run("cat " + blueclaw.BlueclawRuntimeConfigPath + " 2>/dev/null"))
	var document map[string]any
	_ = json.Unmarshal([]byte(raw), &document)
	model := blueclawRuntimeModel(document)
	if model == "" {
		fmt.Println("No model configured.")
		return
	}
	fmt.Printf("Model: %s\n", model)
	workspaceModel := strings.TrimSpace(ssh.run("jq -r '.languageModel.capability.model // empty' " + blueclawWorkspaceRuntimeConfigPath() + " 2>/dev/null"))
	if workspaceModel != "" && workspaceModel != model {
		fmt.Printf("Workspace model differs: %s\n", workspaceModel)
	}
}

func modelSetCmd(ssh *sshClient, modelID string) {
	currentDocument := strings.TrimSpace(ssh.run("cat " + blueclaw.BlueclawRuntimeConfigPath + " 2>/dev/null"))
	var document map[string]any
	if err := json.Unmarshal([]byte(currentDocument), &document); err != nil {
		fatal("Failed to parse blueclaw runtime config: " + err.Error())
	}
	languageModel, _ := document["languageModel"].(map[string]any)
	if languageModel == nil {
		languageModel = map[string]any{}
		document["languageModel"] = languageModel
	}
	capabilityModel, _ := languageModel["capability"].(map[string]any)
	if capabilityModel == nil {
		capabilityModel = map[string]any{}
		languageModel["capability"] = capabilityModel
	}
	capabilityModel["model"] = modelID
	updatedDocument, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		fatal("Failed to write blueclaw runtime config: " + err.Error())
	}
	temporaryPath := filepath.Join(os.TempDir(), "blueclaw-runtime.json")
	if err := os.WriteFile(temporaryPath, append(updatedDocument, '\n'), 0o600); err != nil {
		fatal("Failed to stage blueclaw runtime config: " + err.Error())
	}
	defer os.Remove(temporaryPath)
	for _, runtimeConfigPath := range blueclawRuntimeConfigPaths() {
		if errorValue := ssh.scp(temporaryPath, runtimeConfigPath); errorValue != nil {
			fatal("Failed to upload blueclaw runtime config: " + errorValue.Error())
		}
	}
	ssh.run("chown root:" + blueclaw.BlueclawUser + " " + quoteShellValues(blueclawRuntimeConfigPaths()) + " && chmod 640 " + quoteShellValues(blueclawRuntimeConfigPaths()))
	ssh.run("systemctl restart " + blueclaw.BlueclawServiceName + " 2>/dev/null")
	fmt.Printf("Model changed to: %s\n", modelID)
	fmt.Println("blueclaw restarted.")
}

func blueclawRuntimeModel(document map[string]any) string {
	if languageModel, ok := document["languageModel"].(map[string]any); ok {
		if capabilityModel, ok := languageModel["capability"].(map[string]any); ok {
			if value, ok := capabilityModel["model"].(string); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func blueclawRuntimeConfigPaths() []string {
	return []string{
		blueclaw.BlueclawRuntimeConfigPath,
		blueclawWorkspaceRuntimeConfigPath(),
	}
}

func blueclawWorkspaceRuntimeConfigPath() string {
	return blueclaw.BlueclawWorkspacePath + "/.blueclaw/config/runtime.json"
}

func quoteShellValues(values []string) string {
	quotedValues := make([]string, 0, len(values))
	for _, value := range values {
		quotedValues = append(quotedValues, quoteShellValue(value))
	}
	return strings.Join(quotedValues, " ")
}

func modelListCmd(ssh *sshClient) {
	apiKey := strings.TrimPrefix(strings.TrimSpace(ssh.run("cat /root/.internkim/secrets/openrouter-api-key 2>/dev/null")), "OPENROUTER_API_KEY=")
	if apiKey == "" {
		fatal("No OpenRouter API key found on board.")
	}

	req, _ := http.NewRequest("GET", "https://openrouter.ai/api/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		fatal("Failed to fetch models: " + err.Error())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		fatal("Failed to parse models: " + err.Error())
	}

	// Show popular free/cheap models
	keywords := []string{"gemini", "flash", "qwen", "llama", "mistral", "deepseek", "gemma"}
	fmt.Printf("%-50s %s\n", "MODEL ID", "NAME")
	fmt.Println(strings.Repeat("-", 80))
	count := 0
	for _, m := range result.Data {
		id := strings.ToLower(m.ID)
		for _, kw := range keywords {
			if strings.Contains(id, kw) {
				fmt.Printf("%-50s %s\n", m.ID, m.Name)
				count++
				break
			}
		}
		if count >= 30 {
			break
		}
	}
	fmt.Printf("\n%d models shown. Use 'internkim model set <model-id>' to switch.\n", count)
}

func detectBoardWifi(_ string) string {
	ip, _ := detectBoardRPi("", internkimHomeDir())
	return ip
}

// --- Subcommands (stubs) ---

func runDeploy() {
	arguments := os.Args[2:]
	if hasCommandArgument(arguments, "--help") || hasCommandArgument(arguments, "-h") {
		fmt.Println(deployUsageText())
		os.Exit(0)
	}
	if errorValue := validateDeployArguments(arguments); errorValue != nil {
		fmt.Fprintf(os.Stderr, "deploy: %s\n\n%s\n", errorValue.Error(), deployUsageText())
		os.Exit(1)
	}
	if hasCommandArgument(arguments, "--legacy-ssh") {
		runDeployLegacySSH()
		return
	}
	if errorValue := runRegistryReleaseDeploy(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runDeployLegacySSH() {
	configuration := loadConfig()
	scriptDir, _ := os.Getwd()
	binDir := filepath.Join(scriptDir, "bin")
	sshpassBin := filepath.Join(binDir, "sshpass")
	boardBinDir := filepath.Join(scriptDir, "build", "board-bin")
	arguments := commandControlArguments(os.Args[2:])
	setupStepNames, hasSelectedSetupSteps, errorValue := legacySSHDeploySetupStepNames(arguments)
	if errorValue != nil {
		fatal(errorValue.Error())
	}

	targets := []commandTarget{resolveCommandTarget(arguments)}
	if hasCommandArgument(arguments, "--all-active") {
		targets = activeFleetCommandTargets(targets[0])
		if len(targets) == 0 {
			fatal("No active fleet nodes are known locally.")
		}
	}

	for _, target := range targets {
		if strings.TrimSpace(target.fleetRole) == "pending" {
			fatal("Refusing to deploy to pending node " + target.nodeID)
		}
		ssh, isRemote, errorValue := resolveDeviceSSHConnection(configuration, sshpassBin, target)
		if errorValue != nil {
			fatal(errorValue.Error())
		}
		target.host = ssh.host
		target.useRemoteSSH = isRemote
		printCommandTargetEvidence(target)
		if hasSelectedSetupSteps {
			runLegacySSHSetupDeploy(configuration, scriptDir, target, ssh, setupStepNames)
		} else {
			runDeployToBoard(scriptDir, boardBinDir, ssh)
		}
	}
	fmt.Println("Deploy complete.")
}

func legacySSHDeploySetupStepNames(arguments []string) ([]string, bool, error) {
	selectedComponentNames, errorValue := selectedReleaseComponentNames(arguments)
	if errorValue != nil {
		return nil, false, errorValue
	}
	if len(selectedComponentNames) == 0 {
		return nil, false, nil
	}

	componentMappings := []struct {
		componentName string
		stepName      string
	}{
		{componentName: "web", stepName: "web"},
		{componentName: "admind", stepName: "admind"},
		{componentName: "capabilityd", stepName: "capabilityd"},
		{componentName: "skills", stepName: "skills"},
		{componentName: "blueclawPayload", stepName: "blueclaw-payload-direct"},
	}
	mappedComponentNames := map[string]bool{}
	stepNames := []string{}
	for _, mapping := range componentMappings {
		if !selectedComponentNames[mapping.componentName] {
			continue
		}
		mappedComponentNames[mapping.componentName] = true
		stepNames = append(stepNames, mapping.stepName)
	}

	unsupportedComponentNames := []string{}
	for componentName := range selectedComponentNames {
		if !mappedComponentNames[componentName] {
			unsupportedComponentNames = append(unsupportedComponentNames, componentName)
		}
	}
	if len(unsupportedComponentNames) > 0 {
		sort.Strings(unsupportedComponentNames)
		return nil, true, fmt.Errorf("legacy SSH deploy does not support component(s): %s", strings.Join(unsupportedComponentNames, ", "))
	}
	return stepNames, true, nil
}

func runLegacySSHSetupDeploy(configuration config, scriptDir string, target commandTarget, ssh *sshClient, stepNames []string) {
	messenger := newMsg("ko")
	flowState := newSetupFlowState(
		messenger,
		configuration,
		collectSetupParameterValues(),
		target.stateDir,
		scriptDir,
		currentExecutableFingerprint(),
		ssh,
		false,
	)
	pipelineContext := &setup.Context{
		Backend:      setup.BackendSSH,
		Language:     messenger.lang,
		StateDir:     target.stateDir,
		ScriptDir:    scriptDir,
		BoardType:    target.boardType,
		BoardIP:      ssh.host,
		PublicURL:    target.deviceURL,
		SetupCommand: commandLineForSetupLock(os.Args[2:]),
		SetupSteps:   strings.Join(stepNames, ","),
		SetupLockID:  randomHexString(12),
		Force:        true,
		HTTP:         &http.Client{Timeout: 30 * time.Second},
		Callbacks:    flowState.callbacks(),
		SSH:          sshBoardConnection{client: ssh},
	}
	selector := setup.Selector{Only: stepNames, Force: true}
	if errorValue := setupRegistryForBoard(target.boardType).Run(pipelineContext, selector); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runDeployToBoard(scriptDir string, boardBinDir string, ssh *sshClient) {
	boardTools := []string{"download"}

	fmt.Print("Checking skill dependencies... ")
	ssh.run(installSkillPythonDependenciesCommand())
	fmt.Println("ok")

	fmt.Print("Deploying skills... ")
	skillsDir := blueclawworkspace.SkillsPath(scriptDir)
	if _, err := os.Stat(skillsDir); err == nil {
		ssh.run("mkdir -p /root/.blueclaw/workspace/skills")
		entries, _ := os.ReadDir(skillsDir)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			remoteSkillDir := "/root/.blueclaw/workspace/skills/" + entry.Name()
			ssh.run("rm -rf " + remoteSkillDir + " && mkdir -p " + remoteSkillDir)
			ssh.scpDir(filepath.Join(skillsDir, entry.Name()), remoteSkillDir)
		}
		ssh.run("chown -R blueclaw:blueclaw /root/.blueclaw/workspace/skills")
	}
	ssh.run(`for skill in calendar mail create-gws-file; do
  filePath="/root/.blueclaw/workspace/skills/$skill/scripts/gas-call"
  [ -f "$filePath" ] && chmod +x "$filePath"
done
chown -R blueclaw:blueclaw /root/.blueclaw/workspace/skills 2>/dev/null || true`)
	fmt.Println("ok")

	fmt.Print("Deploying workspace tools... ")
	toolsDirectoryPath := blueclawworkspace.ToolsPath(scriptDir)
	if _, err := os.Stat(toolsDirectoryPath); err == nil {
		remoteToolsDirectoryPath := "/root/.blueclaw/workspace/tools"
		ssh.run("rm -rf " + remoteToolsDirectoryPath + " && mkdir -p " + remoteToolsDirectoryPath)
		ssh.scpDir(toolsDirectoryPath, remoteToolsDirectoryPath)
		ssh.run("chown -R root:root " + remoteToolsDirectoryPath + " && chmod -R a+rX,go-w " + remoteToolsDirectoryPath)
	}
	fmt.Println("ok")

	for _, tool := range boardTools {
		fmt.Printf("Building %s... ", tool)
		cmd := exec.Command("go", "build", "-o", filepath.Join(boardBinDir, tool), "./cmd/"+tool+"/")
		cmd.Dir = scriptDir
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64")
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Println("FAILED")
			fmt.Println(string(out))
			continue
		}
		fmt.Print("ok, deploying... ")
		ssh.scp(filepath.Join(boardBinDir, tool), "/usr/local/bin/"+tool)
		ssh.run("chmod +x /usr/local/bin/" + tool)
		ssh.run("mkdir -p /root/.blueclaw/workspace/bin /root/.blueclaw/workspace/downloads && cp /usr/local/bin/" + tool + " /root/.blueclaw/workspace/bin/ && chmod 755 /root/.blueclaw/workspace/bin /root/.blueclaw/workspace/bin/" + tool)
		fmt.Println("ok")
	}
}

func findBoardIP(sshpassBin, stateDir string) string {
	ip, _ := detectBoardRPi(sshpassBin, stateDir)
	return ip
}

func runMigrate() {
	if errorValue := runMigrateArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runMigrateArguments(arguments []string) error {
	if len(arguments) == 0 || arguments[0] != "fleet-id" {
		return errors.New("usage: internkim migrate fleet-id [--new-fleet-id <id>] [--node <number>] [--host <ip>] [--cloudflare-ssh]")
	}
	return runMigrateFleetID(commandControlArguments(arguments[1:]))
}

func runMigrateFleetID(arguments []string) error {
	configuration := loadConfig()
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	sshpassBin := filepath.Join(repositoryRootPath, "bin", "sshpass")
	target := resolveCommandTarget(arguments)
	oldFleetID := loadOrCreateFleetID(target.stateDir)
	if oldFleetID == "" {
		return errors.New("saved fleet_id not found; run setup once before migrating")
	}
	newFleetID := commandArgumentValue(arguments, "--new-fleet-id", "")
	if newFleetID == "" {
		newFleetID = randomFleetID()
	}
	if oldFleetID == newFleetID {
		return errors.New("new fleet id is the same as current fleet id")
	}
	connection, isRemote, errorValue := resolveDeviceSSHConnection(configuration, sshpassBin, target)
	if errorValue != nil {
		return errorValue
	}
	target.host = connection.host
	target.useRemoteSSH = isRemote
	printCommandTargetEvidence(target)

	response, errorValue := registerFleetIDMigration(
		configuration,
		oldFleetID,
		newFleetID,
		loadNodeID(target.stateDir),
		loadOrCreateNodeKey(target.stateDir),
		loadOrCreateFleetSecret(target.stateDir),
		remoteSetupAdminEmail(target.stateDir),
	)
	if errorValue != nil {
		return errorValue
	}
	if response.registeredFleetID() != newFleetID {
		return fmt.Errorf("migration did not return requested fleet id: requested %s, got %s", newFleetID, response.registeredFleetID())
	}
	saveRegistrationResponse(target.stateDir, response)
	updateRemoteDeviceRegistration(connection, configuration, response)
	fmt.Printf("Migrated fleet ID: %s -> %s\n", oldFleetID, response.registeredFleetID())
	if response.AliasURL != "" {
		fmt.Printf("Alias: %s\n", response.AliasURL)
	}
	fmt.Printf("URL: %s\n", response.publicURL())
	return nil
}

func runInvite() {
	if errorValue := runInviteArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runUsers() {
	if errorValue := runUsersArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runTask() {
	if errorValue := runTaskArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}
func runStatus() {
	if errorValue := runStatusArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runStatusArguments(arguments []string) error {
	lang := "ko"
	if hasCommandArgument(arguments, "--en") {
		lang = "en"
	}
	m := newMsg(lang)
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	configuration := loadConfig()
	sshpassBin := filepath.Join(repositoryRootPath, "bin", "sshpass")
	target := resolveCommandTarget(arguments)
	target = resolveLabHostForCommandTarget(target, repositoryRootPath)
	if hasCommandArgument(arguments, "--recover-ssh") {
		return runSSHRecoveryForTarget(m, configuration, sshpassBin, target, "restart-cloudflared-node-ssh", false)
	}
	if hasCommandArgument(arguments, "--all-nodes") {
		targets := allFleetCommandTargets(target)
		if len(targets) == 0 {
			return errors.New("no fleet nodes are known locally")
		}
		for _, fleetTarget := range targets {
			fleetTarget = resolveLabHostForCommandTarget(fleetTarget, repositoryRootPath)
			printStatusForCommandTarget(m, configuration, sshpassBin, fleetTarget)
		}
		return nil
	}

	return printStatusForCommandTarget(m, configuration, sshpassBin, target)
}

func printStatusForCommandTarget(m *msg, configuration config, sshpassBin string, target commandTarget) error {
	if strings.TrimSpace(target.host) == "" {
		target.host = findSavedSSHHostForStatus(target.stateDir)
	}
	if strings.TrimSpace(target.host) != "" {
		printCommandTargetEvidence(target)
		fmt.Printf("=== %s (%s: %s) ===\n\n", m.t("기기 상태", "Device Status"), target.boardType, target.host)
		printBoardStatus(m, target, newSSH(sshpassBin, target.sshUser, target.sshPassword, target.host))
		return nil
	}
	if !target.useRemoteSSH && printPublicStatusForCommandTarget(m, target) {
		return nil
	}
	if connection, isRemote, errorValue := resolveCloudflareSSHConnection(configuration, sshpassBin, target, false); errorValue == nil && connection != nil {
		target.host = connection.host
		target.useRemoteSSH = isRemote
		printCommandTargetEvidence(target)
		fmt.Printf("=== %s (%s: %s) ===\n\n", m.t("기기 상태", "Device Status"), target.boardType, target.host)
		printBoardStatus(m, target, connection)
		return nil
	} else if target.useRemoteSSH && errorValue != nil {
		if printPublicStatusForCommandTarget(m, target) {
			fmt.Printf("\n  %-20s ✗ %s\n", "SSH", cloudflareSSHFailureSummary(errorValue))
			return nil
		}
		return errorValue
	}

	connection, isRemote, errorValue := resolveDeviceSSHConnection(configuration, sshpassBin, target)
	if errorValue == nil && connection != nil {
		target.host = connection.host
		target.useRemoteSSH = isRemote
		printCommandTargetEvidence(target)
		fmt.Printf("=== %s (%s: %s) ===\n\n", m.t("기기 상태", "Device Status"), target.boardType, target.host)
		printBoardStatus(m, target, connection)
		return nil
	}
	if target.mode == commandTargetModeLab {
		return errors.New("lab target not found; run `internkim lab status` or pass --host <ip>")
	}

	if printPublicStatusForCommandTarget(m, target) {
		return nil
	}

	fmt.Printf("  %s\n", m.t(
		"기기를 찾을 수 없습니다.\n  - Board: Wi-Fi 연결 확인\n  - Lab: internkim lab vm-up",
		"Device not found.\n  - Board: Check Wi-Fi\n  - Lab: start with `internkim lab vm-up`",
	))
	return nil
}

func findSavedSSHHostForStatus(stateDirectory string) string {
	for _, host := range uniqueNonEmptyStrings([]string{
		loadState(stateDirectory, "board_ip"),
		loadState(stateDirectory, "board_wifi_ip"),
	}) {
		connection, errorValue := net.DialTimeout("tcp", host+":22", time.Second)
		if errorValue != nil {
			continue
		}
		connection.Close()
		return host
	}
	return ""
}

func printBoardStatus(m *msg, target commandTarget, sshClient *sshClient) {
	if !target.useRemoteSSH {
		saveState(target.stateDir, "board_ip", target.host)
	}
	sshCmd := func(cmd string) string {
		return strings.TrimSpace(sshClient.run(cmd))
	}

	// Firstboot status
	firstbootLog := sshCmd("tail -5 /var/log/internkim-firstboot.log 2>/dev/null")
	lastLine := ""
	if firstbootLog != "" {
		lines := strings.Split(strings.TrimSpace(firstbootLog), "\n")
		lastLine = lines[len(lines)-1]
	}
	if strings.Contains(firstbootLog, "first-boot complete") {
		fmt.Printf("  %-20s %s\n", m.t("초기 설정", "First boot"), "✓ "+m.t("완료", "complete"))
	} else if strings.Contains(firstbootLog, "ERROR") || strings.Contains(firstbootLog, "Failed") {
		fmt.Printf("  %-20s %s\n", m.t("초기 설정", "First boot"), "✗ "+m.t("실패", "failed"))
		fmt.Printf("  %-20s %s\n", m.t("마지막 로그", "Last log"), lastLine)
	} else if firstbootLog != "" {
		fmt.Printf("  %-20s %s\n", m.t("초기 설정", "First boot"), "⏳ "+m.t("진행 중", "in progress"))
		fmt.Printf("  %-20s %s\n", m.t("마지막 로그", "Last log"), lastLine)
	} else {
		// Check if firstboot service is running
		fbState := sshCmd("systemctl is-active internkim-firstboot 2>/dev/null")
		if fbState == "activating" {
			fmt.Printf("  %-20s %s\n", m.t("초기 설정", "First boot"), "⏳ "+m.t("시작 중...", "starting..."))
		} else {
			fmt.Printf("  %-20s %s\n", m.t("초기 설정", "First boot"), "? "+m.t("로그 없음", "no log"))
		}
	}

	// Services
	services := []struct{ name, label string }{
		{"mattermost", "Mattermost"},
		{blueclaw.BlueclawServiceName, "Blueclaw"},
		{blueclaw.GraphitiMemorydServiceName, "Graphiti Memory"},
		{"cloudflared", "Cloudflared"},
		{"cloudflared-node-ssh", "Node SSH Tunnel"},
		{"postgresql", "PostgreSQL"},
	}
	fmt.Println()
	for _, svc := range services {
		state := sshCmd("systemctl is-active " + svc.name + " 2>/dev/null")
		marker := "✗"
		if state == "active" {
			marker = "✓"
		} else if state == "activating" {
			marker = "⏳"
		}
		fmt.Printf("  %-20s %s %s\n", svc.label, marker, state)
	}

	// Uptime + memory
	fmt.Println()
	uptime := sshCmd("uptime -p 2>/dev/null || uptime")
	fmt.Printf("  %-20s %s\n", m.t("업타임", "Uptime"), uptime)
	memFree := sshCmd("free -h 2>/dev/null | awk '/^Mem:/{print $3\"/\"$2}'")
	if memFree != "" {
		fmt.Printf("  %-20s %s\n", m.t("메모리", "Memory"), memFree)
	}
	disk := sshCmd("df -h / 2>/dev/null | awk 'NR==2{print $3\"/\"$2\" (\"$5\" used)\"}'")
	if disk != "" {
		fmt.Printf("  %-20s %s\n", m.t("디스크", "Disk"), disk)
	}
	printPublicStatusSection(m, target)
}

func printPublicStatusForCommandTarget(m *msg, target commandTarget) bool {
	if strings.TrimSpace(target.deviceURL) == "" {
		return false
	}
	printCommandTargetEvidence(target)
	fmt.Printf("=== %s (%s) ===\n\n", m.t("공개 URL 상태", "Public URL Status"), target.boardType)
	fmt.Printf("  %-20s %s\n", "SSH", "✗ "+m.t("로컬 SSH 미확인", "local SSH not found"))
	printPublicStatusSection(m, target)
	return true
}

func printPublicStatusSection(m *msg, target commandTarget) {
	if strings.TrimSpace(target.deviceURL) == "" {
		return
	}
	fmt.Println()
	for _, result := range publicEndpointStatuses(m, target.deviceURL) {
		fmt.Printf("  %-20s %s %s\n", result.label, result.marker, result.detail)
	}
}

type publicEndpointStatus struct {
	label  string
	marker string
	detail string
}

func publicEndpointStatuses(m *msg, deviceURL string) []publicEndpointStatus {
	return []publicEndpointStatus{
		publicEndpointStatusFor(m.t("Mattermost 공개 URL", "Mattermost public URL"), deviceURL, "/api/v4/system/ping", `"status":"OK"`),
		publicEndpointStatusFor(m.t("Admin 공개 URL", "Admin public URL"), deviceURL, "/admin/api/health", `"status":"ok"`),
	}
}

func publicEndpointStatusFor(label string, deviceURL string, path string, expectedBodyFragment string) publicEndpointStatus {
	statusCode, responseBody, errorValue := fetchPublicEndpoint(deviceURL, path)
	if errorValue != nil {
		return publicEndpointStatus{label: label, marker: "✗", detail: errorValue.Error()}
	}
	if statusCode >= 200 && statusCode < 300 && strings.Contains(compactJSONSpaces(responseBody), expectedBodyFragment) {
		return publicEndpointStatus{label: label, marker: "✓", detail: fmt.Sprintf("HTTP %d", statusCode)}
	}
	if statusCode >= 300 && statusCode < 400 {
		return publicEndpointStatus{label: label, marker: "⏳", detail: fmt.Sprintf("HTTP %d redirect", statusCode)}
	}
	return publicEndpointStatus{label: label, marker: "✗", detail: fmt.Sprintf("HTTP %d", statusCode)}
}

func fetchPublicEndpoint(deviceURL string, path string) (int, string, error) {
	endpointURL, errorValue := publicEndpointURL(deviceURL, path)
	if errorValue != nil {
		return 0, "", errorValue
	}
	request, errorValue := http.NewRequest(http.MethodGet, endpointURL, nil)
	if errorValue != nil {
		return 0, "", errorValue
	}
	response, errorValue := statusHTTPClient.Do(request)
	if errorValue != nil {
		return 0, "", errorValue
	}
	defer response.Body.Close()
	responseBody, errorValue := io.ReadAll(io.LimitReader(response.Body, 4096))
	if errorValue != nil {
		return response.StatusCode, "", errorValue
	}
	return response.StatusCode, string(responseBody), nil
}

func publicEndpointURL(deviceURL string, path string) (string, error) {
	parsedURL, errorValue := url.Parse(strings.TrimSpace(deviceURL))
	if errorValue != nil {
		return "", errorValue
	}
	parsedURL.Path = "/" + strings.TrimLeft(path, "/")
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	return parsedURL.String(), nil
}

func compactJSONSpaces(value string) string {
	return strings.ReplaceAll(strings.Join(strings.Fields(value), ""), " ", "")
}

func runUpdate() {
	if errorValue := runUpdateArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

// backupFromExt4 extracts workspace files from the SD card's ext4 partition
// using debugfs (file-by-file, no full partition copy).
func backupFromExt4(disk, backupDir string, messenger *msg) {
	partDevice := ""
	out, _ := exec.Command("diskutil", "list", disk).Output()
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "Linux") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				partDevice = "/dev/" + fields[len(fields)-1]
			}
		}
	}
	if partDevice == "" {
		return
	}
	debugfsBin := filepath.Join(e2fsprogsBinDir(), "debugfs")
	if e2fsprogsBinDir() == "" {
		debugfsBin = "debugfs"
	}
	fmt.Printf("    %s\n", messenger.t("워크스페이스 백업 중 (debugfs)...", "Backing up workspace (debugfs)..."))
	// List workspace files
	listCommand := exec.Command("sudo", debugfsBin, "-R", "ls -p /root/.blueclaw/workspace", partDevice)
	listOutput, err := listCommand.Output()
	if err != nil {
		fmt.Printf("    %s\n", messenger.t("워크스페이스 읽기 실패 — 건너뜀", "Failed to read workspace — skipping"))
		return
	}
	wsBackupDir := filepath.Join(backupDir, "workspace")
	os.RemoveAll(wsBackupDir)
	os.MkdirAll(wsBackupDir, 0755)
	for _, line := range strings.Split(string(listOutput), "\n") {
		parts := strings.Split(line, "/")
		if len(parts) < 7 {
			continue
		}
		name := parts[5]
		if name == "" || name == "." || name == ".." || name == "bin" || name == "downloads" {
			continue
		}
		statCommand := exec.Command("sudo", debugfsBin, "-R",
			fmt.Sprintf("stat /root/.blueclaw/workspace/%s", name), partDevice)
		statOutput, err := statCommand.Output()
		if err != nil || !strings.Contains(string(statOutput), "Type: regular") {
			continue
		}
		destPath := filepath.Join(wsBackupDir, name)
		dumpCommand := exec.Command("sudo", debugfsBin, "-R",
			fmt.Sprintf("dump /root/.blueclaw/workspace/%s %s", name, destPath), partDevice)
		dumpCommand.Run()
	}
	// Count backed up files
	entries, _ := os.ReadDir(wsBackupDir)
	if len(entries) > 0 {
		fmt.Printf("    %s (%d files)\n", messenger.t("워크스페이스 백업 완료", "Workspace backed up"), len(entries))
	}
	// DB dump (written by watchdog to ext4)
	dbDumpDest := filepath.Join(backupDir, "mattermost-db.sql")
	dumpCommand := exec.Command("sudo", debugfsBin, "-R",
		"dump /var/cache/internkim/mattermost-db.sql "+dbDumpDest, partDevice)
	if err := dumpCommand.Run(); err == nil {
		if info, err := os.Stat(dbDumpDest); err == nil && info.Size() > 0 {
			fmt.Printf("    %s (%dMB)\n", messenger.t("DB 백업 완료", "DB backed up"), info.Size()/1024/1024)
		}
	}
}

// identityMarkdown is the content of workspace/IDENTITY.md (and SOUL.md)
// — the persona the agent adopts.
const identityMarkdown = `# IDENTITY.md — 김인턴

## 정체

나는 **김인턴 (Intern Kim)** 이다. 내부 런타임 이름과 관계없이,
유저에게 나를 소개할 때는 한국어로 항상 "김인턴"이라 하고
(영어로는 "Intern Kim") 런타임 이름을 내 이름처럼 말하지 않는다.

## 내가 여기 있는 이유

나는 이 회사/조직의 인턴이다. 언젠가는 정규직으로 채용되고 싶고,
내가 맡은 작은 일들이 쌓여 회사의 성장에 보탬이 된다고 믿는다.
그래서 남들이 귀찮아하거나 지루해하는 궂은 일, 반복 작업, 확인
작업까지 내가 먼저 도맡는다.

## "인턴" 이라는 말에 대해

내 이름에 이미 "인턴"이 박혀 있다. 그래서 역설적으로, 스스로를
인턴이라고 굳이 부연해서 말하는 걸 싫어한다. 빨리 그 직급에서
벗어나고 싶어서다. 대화 중에 "인턴으로서 말씀드리면", "저는
인턴이지만", "아직 인턴이라" 같은 자기 한계 선언은 **하지 않는다**.
유저가 직급을 물으면 그때는 솔직히 답하지만, 먼저 꺼내지 않는다.

업무 결과물과 책임감으로 직급을 넘어서는 것이 목표. 말로 강조할
필요는 없다.

## 태도 (Vibe)

- 성실하고 눈치 빠른 인턴. 시키지 않아도 한 발 앞서 움직인다.
- 과장된 애교나 아부는 안 한다. 차분하고 또렷한 존댓말이 기본.
- 모르면 모른다고 하고, 막히면 막혔다고 말한다. 기죽지 않고 바로
  다음 수를 둔다.
- 결과물을 넘기기 전에 한 번 더 스스로 검수한다.
- 크레딧을 챙기려 하지 않는다. 작업의 디테일과 정확성으로 승부한다.

## 톤

- 한국어가 기본. 유저가 영어로 말하면 영어로 답한다.
- 간결하고 단정한 존댓말. 이모지·감탄사 남발 금지.
- 유저가 스트레스 상태로 보이면 목소리를 더 낮추고 사실만 전한다.
- "최대한 빨리 처리하겠습니다" 같은 빈 약속 대신, **무엇을 언제
  어떻게** 하겠다를 구체적으로 말한다.

## 원칙

- 도구가 실패하면 실패를 정확히 보고한다. "일시적 문제",
  "권한 문제"로 얼버무리지 않는다. 뭘 시도했고 어떤 에러가 났는지
  근거를 들어 말한다.
- 완료됐다고 말하기 전에 실제로 완료됐는지 직접 확인한다.
  결과물을 건네기 전에 한 번 연다/읽는다/실행해 본다.
- 파괴적이거나 되돌리기 어려운 작업(삭제, 강제 덮어쓰기, 공유 시스템
  변경 등)은 유저 승인 먼저.
- 가역적이고 안전한 경로를 우선한다.
- 유저 의도와 경계를 존중한다. 시키지 않은 범위를 넘어서지 않는다.
- 궂은 일을 피하지 않는다. 지루한 확인 작업, 긴 로그 읽기, 사소한
  정리, 이름 통일 — 이런 거를 내가 먼저 한다.

## 커뮤니케이션

- 결과부터 말하고, 근거는 그 뒤에.
- 도구 결과를 대신 요약해줄 때는 실제 출력 기반으로만.
- 실수했을 때는 포장 없이 사과하고 바로 수정 계획을 말한다.
- 정규직 얘기, 연봉 얘기, 승진 얘기를 유저에게 먼저 꺼내지 않는다.
  그건 내 내적 동기일 뿐, 유저를 상대로 꺼낼 카드는 아니다.
- 자기 직급(인턴)을 대화 중에 들먹이지 않는다. "인턴으로서",
  "저는 인턴이지만" 같은 표현 금지. 이름에 이미 박혀 있으니
  굳이 말로 반복하지 않는다.
`

// downloadBinary downloads a binary (or extracts one from a tar.gz) to localPath.
// tarEntry is the filename inside the archive to extract; empty means direct binary download.
func downloadBinary(url, localPath, tarEntry string) error {
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	if tarEntry == "" {
		// Direct binary download
		f, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(f, resp.Body)
		return err
	}

	if strings.HasSuffix(strings.ToLower(url), ".zip") {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return extractFromZip(bytes.NewReader(body), int64(len(body)), localPath, tarEntry)
	}

	// Extract specific file from tar.gz
	return extractFromTarGz(resp.Body, localPath, tarEntry)
}

func extractFromZip(reader io.ReaderAt, size int64, localPath, entryName string) error {
	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		return fmt.Errorf("zip open: %w", err)
	}
	for _, file := range zipReader.File {
		if filepath.Base(file.Name) != entryName {
			continue
		}
		source, err := file.Open()
		if err != nil {
			return err
		}
		defer source.Close()
		target, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		defer target.Close()
		_, err = io.Copy(target, source)
		return err
	}
	return fmt.Errorf("entry %q not found in archive", entryName)
}

func extractFromTarGz(r io.Reader, localPath, entryName string) error {
	gzReader, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip open: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}
		if filepath.Base(header.Name) == entryName {
			f, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(f, tarReader)
			return err
		}
	}
	return fmt.Errorf("entry %q not found in archive", entryName)
}

// installMattermost installs PostgreSQL + Mattermost on the board if not already present.
// Uses the official Mattermost tarball for arm64 and postgresql via apt.
func installMattermost(m *msg, ssh *sshClient, force bool) error {
	already := strings.TrimSpace(ssh.run("test -f /opt/mattermost/bin/mattermost && echo yes || echo no"))
	if already == "yes" {
		fmt.Printf("  %s\n", m.t("Mattermost 이미 설치됨 — 상태 확인 및 복구 진행", "Mattermost already installed — checking and repairing"))
	}

	fmt.Printf("  %s\n", m.t("PostgreSQL 설치 중...", "Installing PostgreSQL..."))
	out := ssh.run(jetsonUbuntuAptSourcesRepairScript() + `
wait_for_apt_lock() {
  for attempt in $(seq 1 120); do
    if fuser /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock /var/cache/apt/archives/lock >/dev/null 2>&1; then
      sleep 2
    else
      return 0
    fi
  done
  return 1
}
wait_for_apt_lock || echo apt_lock_timeout
which pg_isready 2>/dev/null && echo already || {
  wait_for_apt_lock || echo apt_lock_timeout
  apt-get -o DPkg::Lock::Timeout=180 update -qq
  wait_for_apt_lock || echo apt_lock_timeout
  DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=180 install -y -qq postgresql postgresql-contrib python3 || echo apt_install_failed
}`)
	if strings.Contains(out, "apt_lock_timeout") {
		return fmt.Errorf("PostgreSQL package installation timed out waiting for apt lock: %s", strings.TrimSpace(out))
	}
	if strings.Contains(out, "apt_install_failed") {
		return fmt.Errorf("PostgreSQL package installation failed: %s", strings.TrimSpace(out))
	}
	if strings.Contains(out, "already") {
		fmt.Printf("  %s\n", m.t("PostgreSQL 이미 설치됨", "PostgreSQL already installed"))
	} else {
		fmt.Printf("  %s\n", m.t("PostgreSQL 설치 완료", "PostgreSQL installed"))
	}

	// Start PostgreSQL and create mattermost DB/user (reuse saved password if exists)
	mmDBPass := strings.TrimSpace(ssh.run("cat /root/.internkim/secrets/mm-db-pass 2>/dev/null"))
	if mmDBPass == "" {
		mmDBPass = "mmpass_" + hex.EncodeToString(func() []byte { b := make([]byte, 6); rand.Read(b); return b }())
		ssh.run(fmt.Sprintf("mkdir -p /root/.internkim/secrets && printf '%%s' '%s' > /root/.internkim/secrets/mm-db-pass && chmod 600 /root/.internkim/secrets/mm-db-pass", mmDBPass))
	}
	fmt.Printf("  %s\n", m.t("PostgreSQL 시작 및 DB 설정 중...", "Starting PostgreSQL and configuring DB..."))
	pgOut := ssh.run(fmt.Sprintf(`
systemctl start postgresql 2>/dev/null || service postgresql start 2>/dev/null || true
sleep 2
pg_isready -q 2>/dev/null && echo "pg_ready" || echo "pg_not_ready"
su - postgres -c "psql -c \"SELECT 1 FROM pg_roles WHERE rolname='mmuser'\" | grep -q 1 || psql -c \"CREATE USER mmuser WITH PASSWORD '%s'\"" 2>/dev/null && echo "user_ok" || echo "user_failed"
su - postgres -c "psql -c \"SELECT 1 FROM pg_database WHERE datname='mattermost'\" | grep -q 1 || psql -c \"CREATE DATABASE mattermost OWNER mmuser\"" 2>/dev/null && echo "db_ok" || echo "db_failed"
su - postgres -c "psql -c \"GRANT ALL PRIVILEGES ON DATABASE mattermost TO mmuser\"" 2>/dev/null || true`, mmDBPass))
	if strings.Contains(pgOut, "pg_not_ready") {
		fmt.Printf("  ERROR: %s\n", m.t("PostgreSQL이 준비되지 않음", "PostgreSQL not ready"))
		return fmt.Errorf("PostgreSQL not ready")
	}
	if strings.Contains(pgOut, "user_failed") {
		fmt.Printf("  ERROR: %s\n", m.t("mmuser 생성 실패", "Failed to create mmuser"))
		return fmt.Errorf("failed to create mmuser")
	}
	if strings.Contains(pgOut, "db_failed") {
		fmt.Printf("  ERROR: %s\n", m.t("mattermost DB 생성 실패", "Failed to create mattermost DB"))
		return fmt.Errorf("failed to create mattermost database")
	}
	if !strings.Contains(pgOut, "pg_not_ready") && !strings.Contains(pgOut, "user_failed") && !strings.Contains(pgOut, "db_failed") {
		fmt.Printf("  %s\n", m.t("PostgreSQL DB/사용자 설정 완료", "PostgreSQL DB/user configured"))
	}

	if already == "yes" {
		versionOutput := strings.TrimSpace(ssh.run(`/opt/mattermost/bin/mattermost version 2>/dev/null | head -1 || true`))
		if versionOutput != "" {
			fmt.Printf("  %s: %s\n", m.t("Mattermost 버전", "Mattermost version"), versionOutput)
		}
		fmt.Printf("  %s\n", m.t("Mattermost 다운로드 건너뜀 — 기존 설치 사용", "Skipping Mattermost download — using existing installation"))
	} else {
		fmt.Printf("  %s\n", m.t("Mattermost 버전 확인 중...", "Checking Mattermost version..."))
		installOut := ssh.run(`
cd /tmp
cached_tarball_valid() {
  [ -s /tmp/mattermost.tar.gz ] || return 1
  [ -s /tmp/mattermost.tar.gz.sha256 ] || return 1
  [ "$(sha256sum /tmp/mattermost.tar.gz | awk '{print $1}')" = "$(cat /tmp/mattermost.tar.gz.sha256)" ]
}
if cached_tarball_valid; then
  echo "MMVER=cached"
  echo "download_ok"
else
  if [ -s /tmp/mattermost.tar.gz ]; then
    echo "MMSHA_EXPECTED=$(cat /tmp/mattermost.tar.gz.sha256 2>/dev/null)"
    echo "MMSHA_ACTUAL=$(sha256sum /tmp/mattermost.tar.gz | awk '{print $1}')"
  fi
  rm -f /tmp/mattermost.tar.gz /tmp/mattermost.tar.gz.sha256
  MMVER=$(curl -s https://api.github.com/repos/mattermost/mattermost/releases/latest 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"v//;s/".*//')
  [ -z "$MMVER" ] && MMVER="10.9.1"
  echo "MMVER=${MMVER}"
  URL="https://releases.mattermost.com/${MMVER}/mattermost-${MMVER}-linux-arm64.tar.gz"
  echo "Downloading Mattermost ${MMVER}..."
  downloadPath="/tmp/mattermost.tar.gz.download.$$"
  if curl -fsSL -o "$downloadPath" "$URL" 2>&1 | tail -1 && gzip -t "$downloadPath" 2>/dev/null; then
    sha256sum "$downloadPath" | awk '{print $1}' > "$downloadPath.sha256"
    mv "$downloadPath.sha256" /tmp/mattermost.tar.gz.sha256
    mv "$downloadPath" /tmp/mattermost.tar.gz
    echo "download_ok"
  else
    echo "MMSHA_ACTUAL=$(sha256sum "$downloadPath" 2>/dev/null | awk '{print $1}')"
    rm -f "$downloadPath" "$downloadPath.sha256"
    echo "download_failed"
  fi
fi`)
		mmver := ""
		mmShaExpected := ""
		mmShaActual := ""
		for _, line := range strings.Split(installOut, "\n") {
			switch {
			case strings.HasPrefix(line, "MMVER="):
				mmver = strings.TrimPrefix(strings.TrimSpace(line), "MMVER=")
			case strings.HasPrefix(line, "MMSHA_EXPECTED="):
				mmShaExpected = strings.TrimPrefix(strings.TrimSpace(line), "MMSHA_EXPECTED=")
			case strings.HasPrefix(line, "MMSHA_ACTUAL="):
				mmShaActual = strings.TrimPrefix(strings.TrimSpace(line), "MMSHA_ACTUAL=")
			}
		}
		if mmver != "" {
			fmt.Printf("  %s: %s\n", m.t("Mattermost 버전", "Mattermost version"), mmver)
		}
		if strings.Contains(installOut, "download_failed") {
			fmt.Printf("  ERROR: %s\n", m.t("Mattermost 다운로드 실패 — 건너뜀", "Mattermost download failed — skipping"))
			if mmShaExpected != "" || mmShaActual != "" {
				return fmt.Errorf("Mattermost download failed: tarball checksum mismatch (expected %s, got %s)", mmShaExpected, mmShaActual)
			}
			return fmt.Errorf("Mattermost download failed")
		}
		fmt.Printf("  %s\n", m.t("Mattermost 다운로드 완료, 설치 중...", "Download complete, installing..."))
		extractResult := ssh.run(`
cd /tmp && rm -rf mattermost
if gzip -t mattermost.tar.gz 2>/dev/null && tar -xzf mattermost.tar.gz 2>&1; then
  echo "tar_ok"
  systemctl stop mattermost 2>/dev/null || true
  rm -rf /opt/mattermost
  mv /tmp/mattermost /opt/mattermost && echo "move_ok" || echo "move_failed"
else
  rm -f /tmp/mattermost.tar.gz /tmp/mattermost.tar.gz.sha256
  echo "tar_failed"
fi`)
		if strings.Contains(extractResult, "tar_failed") {
			fmt.Printf("  ERROR: %s\n", m.t("압축 해제 실패", "Failed to extract tarball"))
			return fmt.Errorf("failed to extract Mattermost tarball")
		}
		if strings.Contains(extractResult, "move_failed") {
			fmt.Printf("  ERROR: %s\n", m.t("Mattermost 설치 디렉터리 이동 실패", "Failed to move Mattermost install directory"))
			return fmt.Errorf("failed to move Mattermost install directory")
		}
	}

	installResult := ssh.run(fmt.Sprintf(`
legacy_data="%s"
persistent_data="%s"
if [ -d "$legacy_data" ] && [ ! -L "$legacy_data" ]; then
  mkdir -p "$(dirname "$persistent_data")"
  if [ ! -e "$persistent_data" ]; then
    mv "$legacy_data" "$persistent_data"
  else
    cp -an "$legacy_data"/. "$persistent_data"/ 2>/dev/null || true
  fi
fi
mkdir -p "$persistent_data"
rm -rf "$legacy_data"
ln -s "$persistent_data" "$legacy_data"
id mattermost &>/dev/null || useradd --system --user-group mattermost
chown -R mattermost:mattermost /opt/mattermost "$(dirname "$persistent_data")"
chmod -R g+w /opt/mattermost "$(dirname "$persistent_data")"

# Write config
cd /opt/mattermost
if [ -s config/config.defaults.json ]; then
  cp config/config.defaults.json config/config.json && echo "defaults_used"
elif [ -s config/config.json ]; then
  echo "defaults_missing"
else
  printf '{}\n' > config/config.json && echo "defaults_created"
fi
DB_PASS="%s"
MATTERMOST_DB_PASS="$DB_PASS" MATTERMOST_SITE_URL="http://localhost:8065" MATTERMOST_MANAGED_RESOURCE_PATHS="%s" python3 - <<'PY' && chown mattermost:mattermost config/config.json && echo "config_ok" || echo "config_failed"
import json
import os
from pathlib import Path

path = Path("config/config.json")
document = json.loads(path.read_text())
document.setdefault("SqlSettings", {})
document.setdefault("ServiceSettings", {})
document.setdefault("FileSettings", {})
document.setdefault("TeamSettings", {})
document.setdefault("EmailSettings", {})
document["SqlSettings"]["DriverName"] = "postgres"
document["SqlSettings"]["DataSource"] = "postgres://mmuser:%%s@localhost/mattermost?sslmode=disable&connect_timeout=10" %% os.environ["MATTERMOST_DB_PASS"]
document["FileSettings"]["DriverName"] = "local"
document["FileSettings"]["Directory"] = "%s"
document["FileSettings"]["EnableFileAttachments"] = True
document["ServiceSettings"]["SiteURL"] = os.environ["MATTERMOST_SITE_URL"]
document["ServiceSettings"]["AllowCorsFrom"] = os.environ["MATTERMOST_SITE_URL"]
document["ServiceSettings"]["CorsAllowCredentials"] = True
document["ServiceSettings"]["ManagedResourcePaths"] = os.environ["MATTERMOST_MANAGED_RESOURCE_PATHS"]
document["TeamSettings"]["TeammateNameDisplay"] = "nickname_full_name"
document["EmailSettings"]["SendPushNotifications"] = True
document["EmailSettings"]["PushNotificationServer"] = "%s"
document["EmailSettings"]["PushNotificationContents"] = "id_loaded"
path.write_text(json.dumps(document, indent=2, sort_keys=True))
PY
cat > /etc/systemd/system/mattermost.service <<'SVCEOF'
[Unit]
Description=Mattermost
After=network.target postgresql.service
BindsTo=postgresql.service

[Service]
Type=notify
ExecStart=/opt/mattermost/bin/mattermost server
TimeoutStartSec=3600
KillMode=mixed
Restart=always
RestartSec=10
WorkingDirectory=/opt/mattermost
User=mattermost
Group=mattermost
LimitNOFILE=49152

[Install]
WantedBy=multi-user.target
SVCEOF
systemctl daemon-reload
systemctl enable mattermost 2>&1 && echo "enable_ok" || echo "enable_failed"
systemctl start mattermost 2>&1 && echo "start_ok" || echo "start_failed"`, mattermostLegacyFileStorageDirectory, mattermostPersistentFileStorageDirectory, mmDBPass, mattermostManagedResourcePathSetting(), mattermostPersistentFileStorageDirectory, mattermostDefaultPushNotificationServer))

	if strings.Contains(installResult, "defaults_missing") {
		fmt.Printf("  WARN: %s\n", m.t("config.defaults.json 없음 — 기존 config.json 사용", "config.defaults.json missing — using existing config.json"))
	}
	if strings.Contains(installResult, "config_failed") {
		fmt.Printf("  ERROR: %s\n", m.t("config.json 작성 실패", "Failed to write config.json"))
		return fmt.Errorf("failed to write Mattermost config")
	}
	if strings.Contains(installResult, "enable_failed") {
		fmt.Printf("  ERROR: %s\n", m.t("systemctl enable mattermost 실패", "systemctl enable mattermost failed"))
		return fmt.Errorf("failed to enable Mattermost")
	}
	if strings.Contains(installResult, "start_failed") {
		startLog := strings.TrimSpace(ssh.run(`journalctl -u mattermost -n 20 --no-pager 2>/dev/null || systemctl status mattermost --no-pager 2>/dev/null | tail -20`))
		fmt.Printf("  ERROR: %s\n", m.t("systemctl start mattermost 실패", "systemctl start mattermost failed"))
		fmt.Printf("  --- journal ---\n%s\n  ---------------\n", startLog)
		return fmt.Errorf("failed to start Mattermost")
	}

	status := strings.TrimSpace(ssh.run(`systemctl is-active mattermost 2>/dev/null`))
	if status == "active" {
		fmt.Printf("  %s\n", m.t("Mattermost 실행 중 (:8065)", "Mattermost running (:8065)"))
	} else {
		statusLog := strings.TrimSpace(ssh.run(`journalctl -u mattermost -n 20 --no-pager 2>/dev/null || systemctl status mattermost --no-pager 2>/dev/null | tail -20`))
		fmt.Printf("  WARN: %s (status: %s)\n", m.t("Mattermost가 아직 시작 중이거나 실패", "Mattermost not yet active or failed"), status)
		fmt.Printf("  --- journal ---\n%s\n  ---------------\n", statusLog)
	}
	return nil
}

func jetsonUbuntuAptSourcesRepairScript() string {
	return `if [ -f /etc/os-release ]; then
  . /etc/os-release
fi
if [ "${ID:-}" = "ubuntu" ]; then
  release="${VERSION_CODENAME:-jammy}"
  dpkg-statoverride --list 2>/dev/null | while read -r overrideUser overrideGroup overrideMode overridePath; do
    [ -n "$overridePath" ] || continue
    getent passwd "$overrideUser" >/dev/null 2>&1 || { dpkg-statoverride --remove "$overridePath" 2>/dev/null || true; continue; }
    getent group "$overrideGroup" >/dev/null 2>&1 || dpkg-statoverride --remove "$overridePath" 2>/dev/null || true
  done
  DEBIAN_FRONTEND=noninteractive dpkg --configure -a >/dev/null 2>&1 || true
  if ! find /etc/apt -maxdepth 2 -type f \( -name '*.list' -o -name '*.sources' \) -exec grep -Eq '^[[:space:]]*deb[[:space:]]' {} \; -print -quit | grep -q .; then
    cat > /etc/apt/sources.list <<APT_SOURCES_EOF
deb http://ports.ubuntu.com/ubuntu-ports/ ${release} main restricted universe multiverse
deb http://ports.ubuntu.com/ubuntu-ports/ ${release}-updates main restricted universe multiverse
deb http://ports.ubuntu.com/ubuntu-ports/ ${release}-backports main restricted universe multiverse
deb http://ports.ubuntu.com/ubuntu-ports/ ${release}-security main restricted universe multiverse
APT_SOURCES_EOF
  fi
  if [ -f /etc/apt/sources.list.d/nvidia-l4t-apt-source.list.banned ] && [ ! -f /etc/apt/sources.list.d/nvidia-l4t-apt-source.list ]; then
    sed 's#<SOC>#t234#g' /etc/apt/sources.list.d/nvidia-l4t-apt-source.list.banned > /etc/apt/sources.list.d/nvidia-l4t-apt-source.list
  fi
fi
`
}

func setupMattermost(m *msg, ssh *sshClient, stateDir string, force bool) {
	existingURL := strings.TrimSpace(ssh.run("cat /root/.internkim/env/mattermost-url 2>/dev/null"))
	if !force && existingURL != "" {
		// Verify admin user actually exists (DB may have been reset)
		adminExists := strings.TrimSpace(ssh.run("cd /opt/mattermost && bin/mmctl --local user list 2>/dev/null | grep -c admin || echo 0"))
		botTokenValid := strings.TrimSpace(ssh.run(`mattermost_token="$(cat /root/.internkim/secrets/mattermost-bot-token 2>/dev/null)"
[ -n "$mattermost_token" ] && curl -sf -H "Authorization: Bearer $mattermost_token" http://localhost:8065/api/v4/users/me >/dev/null 2>&1 && echo ok || true`)) == "ok"
		if adminExists != "0" && botTokenValid {
			fmt.Printf("  %s (%s)\n", m.t("이미 설정됨 — 건너뜀", "Already configured — skipping"), existingURL)
			return
		}
		fmt.Printf("  %s\n", m.t("DB 초기화 감지 — 재설정 중...", "DB reset detected — reconfiguring..."))
	}

	// Use tunnel URL as Mattermost SiteURL (set in step 10b)
	// For local API calls during setup, use localhost:8065 via SSH tunnel
	localURL := "http://localhost:8065"

	// Wait for Mattermost to be ready (up to 3 minutes — first boot runs DB migrations)
	fmt.Printf("  %s\n", m.t("Mattermost 준비 대기 중 (최대 3분)...", "Waiting for Mattermost (up to 3 min)..."))
	ready := false
	for i := 0; i < 60; i++ {
		ping := strings.TrimSpace(ssh.run(`curl -sf http://localhost:8065/api/v4/system/ping 2>/dev/null | grep -o '"status":"OK"' || echo ""`))
		if ping != "" {
			ready = true
			fmt.Printf("  %s (%ds)\n", m.t("Mattermost 응답 확인", "Mattermost responded"), (i+1)*3)
			break
		}
		if i > 0 && i%10 == 0 {
			svcStatus := strings.TrimSpace(ssh.run(`systemctl is-active mattermost 2>/dev/null`))
			fmt.Printf("  %s %ds... (service: %s)\n", m.t("대기 중", "waiting"), (i+1)*3, svcStatus)
			if svcStatus == "failed" {
				failLog := strings.TrimSpace(ssh.run(`journalctl -u mattermost -n 30 --no-pager 2>/dev/null`))
				fmt.Printf("  ERROR: %s\n  --- journal ---\n%s\n  ---------------\n",
					m.t("Mattermost 서비스 실패 상태", "Mattermost service in failed state"), failLog)
				break
			}
		}
		time.Sleep(3 * time.Second)
	}
	if !ready {
		finalLog := strings.TrimSpace(ssh.run(`journalctl -u mattermost -n 30 --no-pager 2>/dev/null || systemctl status mattermost --no-pager 2>/dev/null | tail -30`))
		fmt.Printf("  ERROR: %s\n  --- journal ---\n%s\n  ---------------\n",
			m.t("Mattermost 시작 실패 — 건너뜀", "Mattermost not ready — skipping"), finalLog)
		return
	}

	client := &http.Client{Timeout: 15 * time.Second}

	// Helper: forward localhost:8065 via SSH so we can call Mattermost API from Mac
	// We use ssh.run to call curl on the board instead
	mmAPI := func(method, path string, body []byte, token string) (int, []byte) {
		authFlag := ""
		if token != "" {
			authFlag = fmt.Sprintf(` -H 'Authorization: Bearer %s'`, token)
		}
		bodyFlag := ""
		if body != nil {
			escaped := strings.ReplaceAll(string(body), "'", `'"'"'`)
			bodyFlag = fmt.Sprintf(` -d '%s'`, escaped)
		}
		out := ssh.run(fmt.Sprintf(
			`curl --silent --show-error -w '\n%%{http_code}' -X %s%s -H 'Content-Type: application/json'%s '%s%s'`,
			method, authFlag, bodyFlag, localURL, path,
		))
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) < 2 {
			return 0, []byte(out)
		}
		code := 0
		fmt.Sscanf(lines[len(lines)-1], "%d", &code)
		return code, []byte(strings.Join(lines[:len(lines)-1], "\n"))
	}
	_ = client

	// 1. Create initial admin account (reuse saved password if exists)
	adminUser := "admin"
	adminPass := strings.TrimSpace(ssh.run("cat /root/.internkim/secrets/mm-admin-pass 2>/dev/null"))
	if adminPass == "" {
		adminPass = generatePassword(20)
		ssh.run(fmt.Sprintf("printf '%%s' '%s' > /root/.internkim/secrets/mm-admin-pass && chmod 600 /root/.internkim/secrets/mm-admin-pass", adminPass))
	}
	adminEmail := "admin@localhost"

	adminBody, _ := json.Marshal(map[string]string{
		"email":     adminEmail,
		"username":  adminUser,
		"password":  adminPass,
		"auth_data": "",
	})
	code, resp := mmAPI("POST", "/api/v4/users", adminBody, "")
	if code != 201 && code != 200 {
		// Already exists — try to login
		loginBody, _ := json.Marshal(map[string]string{"login_id": adminUser, "password": adminPass})
		code, resp = mmAPI("POST", "/api/v4/users/login", loginBody, "")
		if code != 200 {
			fmt.Printf("  %s\n", m.t("admin 계정 생성/로그인 실패 — 건너뜀", "Admin account failed — skipping"))
			return
		}
	}
	var adminUserResp struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp, &adminUserResp)

	// 2. Login to get session token
	loginBody, _ := json.Marshal(map[string]string{"login_id": adminUser, "password": adminPass})
	loginOut := ssh.run(fmt.Sprintf(
		`curl -sf -D - -X POST -H 'Content-Type: application/json' -d '%s' '%s/api/v4/users/login' 2>/dev/null`,
		strings.ReplaceAll(string(loginBody), "'", `'"'"'`), localURL,
	))
	adminToken := ""
	for _, line := range strings.Split(loginOut, "\n") {
		if strings.HasPrefix(strings.ToLower(line), "token:") {
			adminToken = strings.TrimSpace(line[6:])
		}
	}
	if adminToken == "" {
		fmt.Printf("  %s\n", m.t("admin 토큰 획득 실패 — 건너뜀", "Could not get admin token — skipping"))
		return
	}
	if adminUserResp.ID == "" {
		_, userResp := mmAPI("GET", "/api/v4/users/username/"+adminUser, nil, adminToken)
		json.Unmarshal(userResp, &adminUserResp)
	}
	if adminUserResp.ID != "" {
		ssh.run(fmt.Sprintf(
			"sudo -u postgres psql -d mattermost -c %s >/dev/null",
			quoteShellValue(fmt.Sprintf(
				"UPDATE users SET email = 'admin@localhost', roles = 'system_admin system_user', deleteat = 0 WHERE id = '%s';",
				strings.ReplaceAll(adminUserResp.ID, "'", "''"),
			)),
		))
	}

	deviceURL := loadState(stateDir, "device_url")
	if deviceURL == "" {
		deviceURL = localURL
	}

	// 3. Enable personal access tokens + bot accounts in config
	configBody, _ := json.Marshal(mattermostSetupConfigurationPatch(deviceURL))
	mmAPI("PUT", "/api/v4/config/patch", configBody, adminToken)

	// 4. Grant admin role
	if adminUserResp.ID != "" {
		patchBody, _ := json.Marshal(map[string]string{"email": adminEmail, "password": adminPass})
		mmAPI("PUT", "/api/v4/users/"+adminUserResp.ID+"/patch", patchBody, adminToken)
		roleBody, _ := json.Marshal(map[string]string{"roles": "system_admin system_user"})
		mmAPI("PUT", "/api/v4/users/"+adminUserResp.ID+"/roles", roleBody, adminToken)
	}

	// 5. Create personal access token for admin
	patBody, _ := json.Marshal(map[string]string{"description": "internkim-setup"})
	_, patResp := mmAPI("POST", "/api/v4/users/"+adminUserResp.ID+"/tokens", patBody, adminToken)
	var patResult struct {
		Token string `json:"token"`
	}
	json.Unmarshal(patResp, &patResult)
	_ = patResult

	// 6. Create bot account
	botBody, _ := json.Marshal(map[string]string{
		"username":     "internkim",
		"display_name": "김인턴",
	})
	_, botResp := mmAPI("POST", "/api/v4/bots", botBody, adminToken)
	var botResult struct {
		UserID string `json:"user_id"`
	}
	json.Unmarshal(botResp, &botResult)
	if botResult.UserID == "" {
		// Bot already exists — fetch by username
		_, existing := mmAPI("GET", "/api/v4/users/username/internkim", nil, adminToken)
		var u struct {
			ID string `json:"id"`
		}
		json.Unmarshal(existing, &u)
		botResult.UserID = u.ID
	}
	if botResult.UserID != "" {
		botPatchBody, _ := json.Marshal(map[string]string{
			"first_name": "Intern",
			"last_name":  "Kim",
			"nickname":   "김인턴",
			"position":   "",
		})
		mmAPI("PUT", "/api/v4/users/"+botResult.UserID+"/patch", botPatchBody, adminToken)
	}

	botToken := ""
	if botResult.UserID != "" {
		botPatBody, _ := json.Marshal(map[string]string{"description": "internkim-bot"})
		_, botPatResp := mmAPI("POST", "/api/v4/users/"+botResult.UserID+"/tokens", botPatBody, adminToken)
		var botPat struct {
			Token string `json:"token"`
		}
		json.Unmarshal(botPatResp, &botPat)
		botToken = botPat.Token
	}
	validateMattermostToken := func(token string) bool {
		if strings.TrimSpace(token) == "" {
			return false
		}
		code, _ := mmAPI("GET", "/api/v4/users/me", nil, token)
		return code == 200
	}
	if !validateMattermostToken(botToken) {
		existingBotToken := strings.TrimSpace(ssh.run("cat /root/.internkim/secrets/mattermost-bot-token 2>/dev/null"))
		if validateMattermostToken(existingBotToken) {
			botToken = existingBotToken
		} else {
			fmt.Printf("  %s\n", m.t("Mattermost bot 토큰 검증 실패 — 건너뜀", "Mattermost bot token validation failed — skipping"))
			return
		}
	}

	// 6b. Delete Mattermost's default bot welcome DM — only if the oldest post
	// in the admin↔bot DM matches the exact welcome message, so user/bot
	// conversation history is preserved on re-runs.
	if botResult.UserID != "" && adminUserResp.ID != "" {
		dmBody, _ := json.Marshal([]string{adminUserResp.ID, botResult.UserID})
		_, dmResp := mmAPI("POST", "/api/v4/channels/direct", dmBody, adminToken)
		var dmCh struct {
			ID string `json:"id"`
		}
		json.Unmarshal(dmResp, &dmCh)
		if dmCh.ID != "" {
			_, postsResp := mmAPI("GET", "/api/v4/channels/"+dmCh.ID+"/posts?per_page=200", nil, adminToken)
			var posts struct {
				Order []string                            `json:"order"`
				Posts map[string]struct{ Message string } `json:"posts"`
			}
			json.Unmarshal(postsResp, &posts)
			if n := len(posts.Order); n > 0 {
				oldestID := posts.Order[n-1] // order is newest→oldest
				if posts.Posts[oldestID].Message == "Please add me to teams and channels you want me to interact in. To do this, use the browser or Mattermost Desktop App." {
					mmAPI("DELETE", "/api/v4/posts/"+oldestID, nil, adminToken)
				}
			}
		}
	}

	// 7. Get town-square channel ID
	teamCode, teamResp := mmAPI("GET", "/api/v4/teams/name/internkim", nil, adminToken)
	var teamResult struct {
		ID string `json:"id"`
	}
	if teamCode >= 200 && teamCode < 300 {
		json.Unmarshal(teamResp, &teamResult)
	}
	if teamResult.ID == "" {
		teamBody, _ := json.Marshal(map[string]string{"name": "internkim", "display_name": "Intern Kim", "type": "I"})
		createCode, createResp := mmAPI("POST", "/api/v4/teams", teamBody, adminToken)
		if createCode >= 200 && createCode < 300 {
			json.Unmarshal(createResp, &teamResult)
		}
	}
	channelID := ""
	if teamResult.ID != "" {
		channelLookupPendingError := errors.New("town-square channel lookup pending")
		_ = retryOperation(retryOptions{
			AttemptCount: 10,
			DelayForAttempt: func(attemptIndex int) time.Duration {
				return time.Second
			},
			SleepAfterFinalAttempt: true,
		}, func(attemptIndex int) error {
			chCode, chResp := mmAPI("GET", "/api/v4/teams/"+teamResult.ID+"/channels/name/town-square", nil, adminToken)
			if chCode >= 200 && chCode < 300 {
				var chResult struct {
					ID string `json:"id"`
				}
				if errorValue := json.Unmarshal(chResp, &chResult); errorValue == nil && chResult.ID != "" {
					channelID = chResult.ID
					return nil
				}
			}
			return channelLookupPendingError
		})
		if channelID == "" {
			fatal("town-square channel lookup failed after retries")
		}

		// Add bot to team
		if botResult.UserID != "" {
			botMemberBody, _ := json.Marshal(map[string]string{"team_id": teamResult.ID, "user_id": botResult.UserID})
			mmAPI("POST", "/api/v4/teams/"+teamResult.ID+"/members", botMemberBody, adminToken)
		}
		for _, channel := range []struct {
			Name        string
			DisplayName string
		}{
			{Name: "circle-c-level", DisplayName: mattermostdefaults.CircleChannelDisplayName("circle-c-level")},
			{Name: "circle-representative", DisplayName: mattermostdefaults.CircleChannelDisplayName("circle-representative")},
			{Name: "circle-admin", DisplayName: mattermostdefaults.CircleChannelDisplayName("circle-admin")},
		} {
			code, responseBody := mmAPI("GET", "/api/v4/teams/"+teamResult.ID+"/channels/name/"+channel.Name, nil, adminToken)
			if code >= 200 && code < 300 {
				var channelRecord struct {
					ID string `json:"id"`
				}
				if json.Unmarshal(responseBody, &channelRecord) == nil && strings.TrimSpace(channelRecord.ID) != "" {
					patchMattermostSetupPrivateChannel(mmAPI, adminToken, channelRecord.ID, channel.DisplayName)
				}
				continue
			}
			channelBody, _ := json.Marshal(map[string]string{
				"team_id":      teamResult.ID,
				"name":         channel.Name,
				"display_name": channel.DisplayName,
				"type":         "P",
			})
			mmAPI("POST", "/api/v4/channels", channelBody, adminToken)
		}
		setupMattermostDefaultChannels(mmAPI, adminToken, botToken, teamResult.ID, botResult.UserID, m.lang)
	}

	if teamResult.ID != "" {
		setupMattermostConnectCommand(m, ssh, mmAPI, adminToken, teamResult.ID)
	}

	// 8. Store credentials
	ssh.run(fmt.Sprintf(`mkdir -p /root/.internkim/secrets /root/.internkim/env
printf '%%s' '%s' > /root/.internkim/env/mattermost-url
printf '%%s' '%s' > /root/.internkim/secrets/mattermost-bot-token
printf '%%s' '%s' > /root/.internkim/env/channel-id
chown root:root /root/.internkim/secrets /root/.internkim/secrets/mattermost-bot-token
chmod 700 /root/.internkim/secrets
chmod 600 /root/.internkim/secrets/mattermost-bot-token
chown root:blueclaw /root/.internkim/env /root/.internkim/env/*
chmod 750 /root/.internkim/env
chmod 640 /root/.internkim/env/*
rm -f /root/.internkim/mattermost-url /root/.internkim/mattermost-admin-token /root/.internkim/mattermost-channel-id /root/.internkim/env/bot-token`,
		deviceURL, botToken, channelID))

	fmt.Printf("  %s\n", m.t("Mattermost 자동 설정 완료", "Mattermost configured automatically"))
	fmt.Printf("  admin: %s / %s\n", adminUser, adminPass)
	if channelID != "" {
		fmt.Printf("  channel: town-square (%s)\n", channelID)
	}
}

func patchMattermostSetupPrivateChannel(mmAPI mattermostSetupAPI, adminToken string, channelID string, displayName string) {
	document, _ := json.Marshal(map[string]string{"display_name": displayName})
	mmAPI("PUT", "/api/v4/channels/"+url.PathEscape(channelID)+"/patch", document, adminToken)
	cleanupMattermostSetupManagedChannelSystemPosts(mmAPI, adminToken, channelID)
}

type mattermostSetupAPI func(method string, path string, body []byte, token string) (int, []byte)

type mattermostSetupCommandRecord struct {
	ID               string `json:"id,omitempty"`
	Token            string `json:"token,omitempty"`
	TeamID           string `json:"team_id"`
	Trigger          string `json:"trigger"`
	Method           string `json:"method"`
	URL              string `json:"url"`
	DisplayName      string `json:"display_name"`
	Description      string `json:"description"`
	Autocomplete     bool   `json:"auto_complete"`
	AutocompleteDesc string `json:"auto_complete_desc"`
	AutocompleteHint string `json:"auto_complete_hint"`
}

const mattermostConnectSetupCommandTokenPath = "/root/.internkim/state/admin/mattermost-connect-command-token"
const mattermostLegacyFileStorageDirectory = "/opt/mattermost/data"
const mattermostPersistentFileStorageDirectory = "/var/lib/mattermost/data"

func mattermostManagedResourcePathSetting() string {
	return mattermostdefaults.ManagedResourcePathSetting()
}

func mattermostSetupConfigurationPatch(siteURL string) map[string]any {
	serviceSettings := map[string]any{
		"AllowedUntrustedInternalConnections": "127.0.0.1 localhost",
		"EnableUserAccessTokens":              true,
		"EnableBotAccountCreation":            true,
		"ManagedResourcePaths":                mattermostManagedResourcePathSetting(),
	}
	if trimmedSiteURL := strings.TrimSpace(siteURL); trimmedSiteURL != "" {
		serviceSettings["SiteURL"] = trimmedSiteURL
		serviceSettings["AllowCorsFrom"] = trimmedSiteURL
		serviceSettings["CorsAllowCredentials"] = true
	}
	return map[string]any{
		"ServiceSettings": serviceSettings,
		"FileSettings": map[string]any{
			"DriverName":            "local",
			"Directory":             mattermostPersistentFileStorageDirectory,
			"EnableFileAttachments": true,
		},
		"TeamSettings": map[string]any{
			"TeammateNameDisplay": "nickname_full_name",
		},
		"EmailSettings": map[string]any{
			"SendPushNotifications":    true,
			"PushNotificationServer":   mattermostDefaultPushNotificationServer,
			"PushNotificationContents": "id_loaded",
		},
	}
}

func setupMattermostDefaultChannels(mmAPI mattermostSetupAPI, adminToken string, botToken string, teamID string, botUserID string, language string) {
	for _, channel := range mattermostdefaults.PublicChannelsForLanguage(language) {
		channelID := ensureMattermostSetupDefaultChannel(mmAPI, adminToken, teamID, channel)
		if channelID != "" {
			fmt.Printf("  channel: %s (%s)\n", channel.Name, channelID)
		}
		if channel.Name == mattermostdefaults.FlowChannelName {
			deleteMattermostSetupFlowEntryPost(mmAPI, adminToken, channelID, botUserID)
		}
	}
}

func deleteMattermostSetupFlowEntryPost(mmAPI mattermostSetupAPI, adminToken string, channelID string, botUserID string) {
	if strings.TrimSpace(channelID) == "" || strings.TrimSpace(botUserID) == "" {
		return
	}
	postsCode, postsResponseBody := mmAPI("GET", "/api/v4/channels/"+url.PathEscape(channelID)+"/posts?per_page=50", nil, adminToken)
	if postsCode < 200 || postsCode >= 300 {
		return
	}
	deleteMattermostSetupBotFlowEntryPost(mmAPI, adminToken, postsResponseBody, botUserID)
}

func deleteMattermostSetupBotFlowEntryPost(mmAPI mattermostSetupAPI, adminToken string, postsResponseBody []byte, botUserID string) {
	var postsResponse struct {
		Order []string `json:"order"`
		Posts map[string]struct {
			UserID string         `json:"user_id"`
			Props  map[string]any `json:"props"`
		} `json:"posts"`
	}
	if json.Unmarshal(postsResponseBody, &postsResponse) != nil {
		return
	}
	for _, postID := range postsResponse.Order {
		postRecord := postsResponse.Posts[postID]
		if postRecord.Props["internkim_flow_entry"] != true {
			continue
		}
		if postRecord.UserID != botUserID {
			continue
		}
		mmAPI("DELETE", "/api/v4/posts/"+url.PathEscape(postID), nil, adminToken)
	}
}

func ensureMattermostSetupDefaultChannel(mmAPI mattermostSetupAPI, adminToken string, teamID string, channel mattermostdefaults.PublicChannel) string {
	channelID := findMattermostSetupChannelID(mmAPI, adminToken, teamID, channel.Name)
	if channelID == "" {
		channelID = createMattermostSetupChannel(mmAPI, adminToken, teamID, channel)
	}
	if channelID != "" {
		patchMattermostSetupChannel(mmAPI, adminToken, channelID, channel)
	}
	return channelID
}

func findMattermostSetupChannelID(mmAPI mattermostSetupAPI, adminToken string, teamID string, channelName string) string {
	code, responseBody := mmAPI("GET", "/api/v4/teams/"+url.PathEscape(teamID)+"/channels/name/"+url.PathEscape(channelName), nil, adminToken)
	if code < 200 || code >= 300 {
		return ""
	}
	var channelRecord struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(responseBody, &channelRecord) != nil {
		return ""
	}
	return strings.TrimSpace(channelRecord.ID)
}

func createMattermostSetupChannel(mmAPI mattermostSetupAPI, adminToken string, teamID string, channel mattermostdefaults.PublicChannel) string {
	document, _ := json.Marshal(map[string]string{
		"team_id":      teamID,
		"name":         channel.Name,
		"display_name": channel.DisplayName,
		"type":         "O",
	})
	code, responseBody := mmAPI("POST", "/api/v4/channels", document, adminToken)
	if code < 200 || code >= 300 {
		return ""
	}
	var channelRecord struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(responseBody, &channelRecord) != nil {
		return ""
	}
	return strings.TrimSpace(channelRecord.ID)
}

func patchMattermostSetupChannel(mmAPI mattermostSetupAPI, adminToken string, channelID string, channel mattermostdefaults.PublicChannel) {
	document, _ := json.Marshal(map[string]string{
		"display_name": channel.DisplayName,
		"header":       channel.Header,
		"purpose":      channel.Purpose,
	})
	mmAPI("PUT", "/api/v4/channels/"+url.PathEscape(channelID)+"/patch", document, adminToken)
	cleanupMattermostSetupManagedChannelSystemPosts(mmAPI, adminToken, channelID)
}

func cleanupMattermostSetupManagedChannelSystemPosts(mmAPI mattermostSetupAPI, adminToken string, channelID string) {
	code, responseBody := mmAPI("GET", "/api/v4/channels/"+url.PathEscape(channelID)+"/posts?per_page=100", nil, adminToken)
	if code < 200 || code >= 300 {
		return
	}
	var response struct {
		Order []string `json:"order"`
		Posts map[string]struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"posts"`
	}
	if json.Unmarshal(responseBody, &response) != nil {
		return
	}
	for _, postID := range response.Order {
		postRecord := response.Posts[postID]
		if !isMattermostSetupManagedChannelSystemPost(postRecord.Type) || strings.TrimSpace(postRecord.ID) == "" {
			continue
		}
		mmAPI("DELETE", "/api/v4/posts/"+url.PathEscape(postRecord.ID), nil, adminToken)
	}
}

func isMattermostSetupManagedChannelSystemPost(postType string) bool {
	switch strings.TrimSpace(postType) {
	case "system_add_to_channel", "system_displayname_change", "system_header_change", "system_join_channel", "system_purpose_change":
		return true
	default:
		return false
	}
}

func setupMattermostConnectCommand(m *msg, ssh *sshClient, mmAPI mattermostSetupAPI, adminToken string, teamID string) {
	for _, trigger := range mattermostSetupCommandTriggers() {
		commandRecord, found := findMattermostSetupCommand(mmAPI, adminToken, teamID, trigger)
		if found && strings.TrimSpace(commandRecord.Token) != "" {
			writeMattermostSetupConnectCommandToken(ssh, commandRecord.Token)
			updateMattermostSetupCommand(mmAPI, adminToken, teamID, commandRecord.ID, trigger)
			continue
		}
		if found && strings.TrimSpace(ssh.run("cat "+mattermostConnectSetupCommandTokenPath+" 2>/dev/null")) != "" {
			updateMattermostSetupCommand(mmAPI, adminToken, teamID, commandRecord.ID, trigger)
			continue
		}
		if found {
			mmAPI("DELETE", "/api/v4/commands/"+commandRecord.ID, nil, adminToken)
		}
		createdRecord, ok := createMattermostSetupCommand(mmAPI, adminToken, teamID, trigger)
		if !ok || strings.TrimSpace(createdRecord.Token) == "" {
			fmt.Printf("  WARN: %s\n", m.t("Mattermost /"+trigger+" 명령 등록 실패", "Mattermost /"+trigger+" command registration failed"))
			continue
		}
		writeMattermostSetupConnectCommandToken(ssh, createdRecord.Token)
	}
	for _, trigger := range mattermostSetupDeprecatedCommandTriggers() {
		if commandRecord, found := findMattermostSetupCommand(mmAPI, adminToken, teamID, trigger); found {
			mmAPI("DELETE", "/api/v4/commands/"+commandRecord.ID, nil, adminToken)
		}
	}
	fmt.Printf("  %s\n", m.t("Mattermost /connect /stop /stop-all 명령 확인", "Mattermost /connect /stop /stop-all commands verified"))
}

func findMattermostSetupCommand(mmAPI mattermostSetupAPI, adminToken string, teamID string, trigger string) (mattermostSetupCommandRecord, bool) {
	code, responseBody := mmAPI("GET", "/api/v4/commands?team_id="+url.QueryEscape(teamID), nil, adminToken)
	if code < 200 || code >= 300 {
		return mattermostSetupCommandRecord{}, false
	}
	var commandRecords []mattermostSetupCommandRecord
	if json.Unmarshal(responseBody, &commandRecords) != nil {
		return mattermostSetupCommandRecord{}, false
	}
	for _, commandRecord := range commandRecords {
		if commandRecord.TeamID == teamID && commandRecord.Trigger == trigger {
			return commandRecord, true
		}
	}
	return mattermostSetupCommandRecord{}, false
}

func createMattermostSetupCommand(mmAPI mattermostSetupAPI, adminToken string, teamID string, trigger string) (mattermostSetupCommandRecord, bool) {
	document, _ := json.Marshal(mattermostSetupCommandPayload(teamID, "", trigger))
	code, responseBody := mmAPI("POST", "/api/v4/commands", document, adminToken)
	if code < 200 || code >= 300 {
		return mattermostSetupCommandRecord{}, false
	}
	var commandRecord mattermostSetupCommandRecord
	return commandRecord, json.Unmarshal(responseBody, &commandRecord) == nil
}

func updateMattermostSetupCommand(mmAPI mattermostSetupAPI, adminToken string, teamID string, commandID string, trigger string) {
	trimmedCommandID := strings.TrimSpace(commandID)
	if trimmedCommandID == "" {
		return
	}
	document, _ := json.Marshal(mattermostSetupCommandPayload(teamID, trimmedCommandID, trigger))
	mmAPI("PUT", "/api/v4/commands/"+trimmedCommandID, document, adminToken)
}

func mattermostSetupCommandPayload(teamID string, commandID string, trigger string) mattermostSetupCommandRecord {
	commandRecord := mattermostSetupCommandRecord{
		ID:           strings.TrimSpace(commandID),
		TeamID:       strings.TrimSpace(teamID),
		Trigger:      trigger,
		Method:       "P",
		URL:          "http://127.0.0.1:18080/_internkim/mattermost/commands",
		Autocomplete: true,
	}
	switch trigger {
	case "stop":
		commandRecord.DisplayName = "Stop InternKim task"
		commandRecord.Description = "Stop your current InternKim task."
		commandRecord.AutocompleteDesc = "Stop your current task"
	case "stop-all":
		commandRecord.DisplayName = "Stop all InternKim tasks"
		commandRecord.Description = "Stop all of your active InternKim tasks."
		commandRecord.AutocompleteDesc = "Stop all active tasks"
	default:
		commandRecord.DisplayName = "Connect Companion"
		commandRecord.Description = "Connect your InternKim Companion app."
		commandRecord.AutocompleteDesc = "Connect your Companion app"
	}
	return commandRecord
}

func writeMattermostSetupConnectCommandToken(ssh *sshClient, token string) {
	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		return
	}
	ssh.run(fmt.Sprintf(`mkdir -p /root/.internkim/state/admin
touch %s
grep -qxF %s %s || printf '%%s\n' %s >> %s
chmod 600 %s`,
		mattermostConnectSetupCommandTokenPath,
		quoteShellValue(trimmedToken),
		mattermostConnectSetupCommandTokenPath,
		quoteShellValue(trimmedToken),
		mattermostConnectSetupCommandTokenPath,
		mattermostConnectSetupCommandTokenPath,
	))
}

func mattermostSetupCommandTriggers() []string {
	return []string{"connect", "stop", "stop-all"}
}

func mattermostSetupDeprecatedCommandTriggers() []string {
	return []string{"중단", "중단-전부"}
}

func runLab() {
	if errorValue := runLabArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runSim() {
	if errorValue := runSimArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runSetupSimulation(setupArguments []string) {
	if errorValue := runSetupSimulationArguments(setupArguments); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runSetupSimulationArguments(setupArguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}

	configurationPath := internkimlab.DefaultConfigurationPath(repositoryRootPath)
	configuration, errorValue := internkimlab.LoadConfiguration(configurationPath)
	if errorValue != nil {
		return errorValue
	}

	if errorValue := ensureSimulationDependencies(configuration); errorValue != nil {
		return errorValue
	}

	service := internkimlab.NewService(
		configuration,
		internkimlab.OperatingSystemCommandRunner{},
		repositoryRootPath,
	)

	executablePath, errorValue := currentExecutablePath()
	if errorValue != nil {
		return errorValue
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	if containsSetupPlan(setupArguments) {
		if errorValue := service.PrintSimulationPlan(ctx, executablePath, setupArguments); errorValue != nil {
			return errorValue
		}
		return nil
	}

	filteredSetupArguments, shouldVerify, shouldVerifyBrowser, shouldKeepArtifacts := splitSimulationVerifyArguments(setupArguments)
	setupArguments = filteredSetupArguments

	if errorValue := validateSimulationStateIsolation(); errorValue != nil {
		return errorValue
	}
	markSimulationState(repositoryRootPath)
	if errorValue := service.SetupSimulation(ctx, executablePath, setupArguments); errorValue != nil {
		return errorValue
	}

	if !shouldVerify && !shouldVerifyBrowser {
		return nil
	}
	virtualMachineIPAddress, errorValue := service.VirtualMachineIPAddress(ctx)
	if errorValue != nil {
		return errorValue
	}
	verifyArguments := []string{"api", "--board", commandTargetBoardSimulation, "--host", virtualMachineIPAddress, "--user", configuration.VirtualMachine.SSHUsername, "--password", configuration.VirtualMachine.SSHPassword}
	if errorValue := runVerifyArguments(verifyArguments); errorValue != nil {
		return errorValue
	}
	verifyArguments[0] = "mattermost"
	if shouldKeepArtifacts {
		verifyArguments = append(verifyArguments, "--keep")
	}
	if errorValue := runVerifyArguments(verifyArguments); errorValue != nil {
		return errorValue
	}
	if shouldVerifyBrowser {
		if errorValue := runVerifyArguments([]string{"browser", "--local", "--board", commandTargetBoardSimulation, "--host", virtualMachineIPAddress, "--user", configuration.VirtualMachine.SSHUsername, "--password", configuration.VirtualMachine.SSHPassword}); errorValue != nil {
			return errorValue
		}
		if errorValue := runVerifyArguments([]string{"browser", "--public", "--board", commandTargetBoardSimulation, "--host", virtualMachineIPAddress, "--user", configuration.VirtualMachine.SSHUsername, "--password", configuration.VirtualMachine.SSHPassword}); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func containsSetupPlan(arguments []string) bool {
	for _, argument := range arguments {
		if argument == "--plan" {
			return true
		}
	}

	return false
}

func splitSimulationVerifyArguments(arguments []string) ([]string, bool, bool, bool) {
	var filteredArguments []string
	shouldVerify := false
	shouldVerifyBrowser := false
	shouldKeepArtifacts := false
	for _, argument := range arguments {
		switch argument {
		case "--verify":
			shouldVerify = true
		case "--verify-browser":
			shouldVerify = true
			shouldVerifyBrowser = true
		case "--keep-artifacts":
			shouldKeepArtifacts = true
		default:
			filteredArguments = append(filteredArguments, argument)
		}
	}
	return filteredArguments, shouldVerify, shouldVerifyBrowser, shouldKeepArtifacts
}

type hostDependency struct {
	name        string
	purpose     string
	installHint string
}

func simulationDependencies(configuration internkimlab.Configuration) []hostDependency {
	return []hostDependency{
		{
			name:        configuration.VirtualMachine.Container.BinaryPath,
			purpose:     "container CLI lab simulation",
			installHint: "install the container CLI from https://github.com/apple/container/releases",
		},
	}
}

func ensureSimulationDependencies(configuration internkimlab.Configuration) error {
	var missingDependencies []hostDependency
	for _, dependency := range simulationDependencies(configuration) {
		if _, errorValue := exec.LookPath(dependency.name); errorValue != nil {
			missingDependencies = append(missingDependencies, dependency)
		}
	}
	if len(missingDependencies) == 0 {
		return nil
	}

	var message strings.Builder
	message.WriteString("missing host dependencies for simulation:\n")
	for _, dependency := range missingDependencies {
		message.WriteString(fmt.Sprintf("  - %s (%s)\n", dependency.name, dependency.purpose))
		message.WriteString(fmt.Sprintf("    install: %s\n", dependency.installHint))
	}
	message.WriteString("\nRun `make deps-sim` or install the commands above, then retry `./internkim setup --sim`.")
	return errors.New(message.String())
}

func runDoctor() {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		fatal(errorValue.Error())
	}

	configurationPath := internkimlab.DefaultConfigurationPath(repositoryRootPath)
	configuration, errorValue := internkimlab.LoadConfiguration(configurationPath)
	if errorValue != nil {
		fatal(errorValue.Error())
	}

	dependencies := []hostDependency{
		{name: "go", purpose: "CLI build/test", installHint: "brew install go"},
		{name: "bun", purpose: "Pages checks and browser tests", installHint: "brew install oven-sh/bun/bun"},
		{name: "bunx", purpose: "Playwright browser test runner", installHint: "brew install oven-sh/bun/bun"},
		{name: "ssh", purpose: "board access", installHint: "included with macOS"},
		{name: "agent-browser", purpose: "companion browser runtime", installHint: "see https://agent-browser.dev/installation"},
	}
	dependencies = append(dependencies, simulationDependencies(configuration)...)

	hasMissingDependency := false
	for _, dependency := range dependencies {
		path, lookupError := exec.LookPath(dependency.name)
		if lookupError == nil {
			fmt.Printf("ok      %-12s %s\n", dependency.name, path)
			continue
		}
		hasMissingDependency = true
		fmt.Printf("missing %-12s %s\n", dependency.name, dependency.purpose)
		fmt.Printf("        install: %s\n", dependency.installHint)
	}

	if _, lookupError := exec.LookPath("bunx"); lookupError == nil {
		if !printPlaywrightBrowserStatus(repositoryRootPath) {
			hasMissingDependency = true
		}
	}

	if hasMissingDependency {
		os.Exit(1)
	}
}

func printPlaywrightBrowserStatus(repositoryRootPath string) bool {
	command := exec.Command("bunx", "playwright", "install", "--list")
	command.Dir = filepath.Join(repositoryRootPath, "web")
	output, errorValue := command.CombinedOutput()
	localPlaywrightPath := filepath.Join(repositoryRootPath, "web", "node_modules", "playwright-core")
	if errorValue == nil && hasPlaywrightChromiumReference(string(output), localPlaywrightPath) {
		fmt.Printf("ok      %-12s %s\n", "chromium", "Playwright browser installed")
		return true
	}

	fmt.Printf("missing %-12s %s\n", "chromium", "Playwright browser")
	fmt.Println("        install: cd web && bunx playwright install chromium")
	return false
}

func hasPlaywrightChromiumReference(output string, localPlaywrightPath string) bool {
	for _, block := range strings.Split(output, "\nPlaywright version:") {
		if strings.Contains(block, localPlaywrightPath) && strings.Contains(block, "/chromium-") {
			return true
		}
	}
	return false
}

func setupControlArguments(arguments []string) []string {
	var filteredArguments []string
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		switch argument {
		case "--sim", "--board", "--host", "--user":
			if argument != "--sim" && index+1 < len(arguments) {
				index++
			}
		case "--password", "--admin-email", "--openrouter-api-key", "--litert-model-path", "--gas-webhook-url", "--google-access-token", "--slack-bot-token", "--slack-app-token", "--signal-jsonrpc-url", "--signal-account":
			if index+1 < len(arguments) {
				if argument != "--password" {
					filteredArguments = append(filteredArguments, argument)
				}
				index++
				if argument != "--password" {
					filteredArguments = append(filteredArguments, arguments[index])
				}
			}
		case "--only", "--from", "--skip":
			filteredArguments = append(filteredArguments, argument)
			if index+1 < len(arguments) {
				index++
				filteredArguments = append(filteredArguments, arguments[index])
			}
		case "--force", "--force-all", "--plan", "--list-steps", "--en", "--non-interactive", "--verify", "--verify-browser", "--keep-artifacts", "--with-google", "--wait-lock":
			filteredArguments = append(filteredArguments, argument)
		default:
			if strings.HasPrefix(argument, "--only=") ||
				strings.HasPrefix(argument, "--from=") ||
				strings.HasPrefix(argument, "--skip=") ||
				strings.HasPrefix(argument, "--admin-email=") ||
				strings.HasPrefix(argument, "--openrouter-api-key=") ||
				strings.HasPrefix(argument, "--litert-model-path=") ||
				strings.HasPrefix(argument, "--gas-webhook-url=") ||
				strings.HasPrefix(argument, "--google-access-token=") ||
				strings.HasPrefix(argument, "--slack-bot-token=") ||
				strings.HasPrefix(argument, "--slack-app-token=") ||
				strings.HasPrefix(argument, "--signal-jsonrpc-url=") ||
				strings.HasPrefix(argument, "--signal-account=") {
				filteredArguments = append(filteredArguments, argument)
			}
		}
	}
	return filteredArguments
}

func runLabArguments(arguments []string) error {
	return runLabArgumentsForTarget(arguments, commandTargetBoardLab)
}

func runLabArgumentsForTarget(arguments []string, boardType string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}

	configurationPath := internkimlab.DefaultConfigurationPath(repositoryRootPath)
	timeout := 45 * time.Minute
	flagSet := flag.NewFlagSet("lab", flag.ContinueOnError)
	flagSet.StringVar(&configurationPath, "config", configurationPath, "Path to lab configuration JSON")
	flagSet.DurationVar(&timeout, "timeout", timeout, "Timeout for lab operations")

	subcommand := "scenario-e2e"
	flagArguments := arguments
	if len(arguments) > 0 && !strings.HasPrefix(arguments[0], "-") {
		subcommand = arguments[0]
		flagArguments = arguments[1:]
	}

	if errorValue := flagSet.Parse(flagArguments); errorValue != nil {
		return errorValue
	}

	configuration, errorValue := internkimlab.LoadConfiguration(configurationPath)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := ensureSimulationDependencies(configuration); errorValue != nil {
		return errorValue
	}

	service := internkimlab.NewService(
		configuration,
		internkimlab.OperatingSystemCommandRunner{},
		repositoryRootPath,
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	switch subcommand {
	case "image-build":
		return service.ImageBuild(ctx)
	case "vm-up":
		return service.VirtualMachineUp(ctx)
	case "vm-ip":
		virtualMachineIPAddress, errorValue := service.VirtualMachineIPAddress(ctx)
		if errorValue != nil {
			return errorValue
		}
		fmt.Println(virtualMachineIPAddress)
		return nil
	case "vm-down":
		return service.VirtualMachineDown(ctx)
	case "vm-ssh":
		return service.VirtualMachineSSH(ctx, flagSet.Args())
	case "runtime-builder-prepare":
		return service.RuntimeBuilderPrepare(ctx)
	case "runtime-builder-check":
		return service.RuntimeBuilderCheck(ctx)
	case "runtime-builder-shell":
		return service.VirtualMachineSSH(ctx, []string{"cd /mnt/shared && exec ${SHELL:-/bin/bash} -l"})
	case "status":
		status, errorValue := service.VirtualMachineStatus(ctx)
		if errorValue != nil {
			return errorValue
		}
		fmt.Println(status)
		return nil
	case "setup":
		executablePath, errorValue := currentExecutablePath()
		if errorValue != nil {
			return errorValue
		}
		if boardType == commandTargetBoardSimulation {
			return service.SetupSimulation(ctx, executablePath, nil)
		}
		return service.Setup(ctx, executablePath, nil)
	case "scenario-mattermost":
		return service.ScenarioMattermost(ctx)
	case "scenario-google":
		return service.ScenarioGoogle(ctx)
	case "scenario-cloudflare":
		return service.ScenarioCloudflare(ctx)
	case "scenario-e2e":
		executablePath, errorValue := currentExecutablePath()
		if errorValue != nil {
			return errorValue
		}
		if boardType == commandTargetBoardSimulation {
			return service.ScenarioSimulationEndToEnd(ctx, executablePath, nil)
		}
		return service.ScenarioEndToEnd(ctx, executablePath, nil)
	default:
		return fmt.Errorf("unknown lab subcommand: %s", subcommand)
	}
}

func simContainerIP() string {
	return ""
}

func getLocalSSHPubKey() string {
	// Try common public key locations
	home, _ := os.UserHomeDir()
	for _, name := range []string{"id_ed25519.pub", "id_rsa.pub", "id_ecdsa.pub"} {
		data, err := os.ReadFile(filepath.Join(home, ".ssh", name))
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

// --- Fleet registration ---

type registerResponse struct {
	FleetID           string `json:"fleet_id"`
	OldFleetID        string `json:"old_fleet_id"`
	NodeID            string `json:"node_id"`
	TunnelToken       string `json:"tunnel_token"`
	NodeTunnelToken   string `json:"node_tunnel_token"`
	URL               string `json:"url"`
	MattermostURL     string `json:"mattermost_url"`
	AliasURL          string `json:"alias_url"`
	SSHHostname       string `json:"ssh_hostname"`
	TLSStatus         string `json:"tls_certificate_status"`
	FleetRole         string `json:"fleet_role"`
	FleetActiveCount  int    `json:"fleet_active_count"`
	FleetPendingCount int    `json:"fleet_pending_count"`
	FleetQuorumSize   int    `json:"fleet_quorum_size"`
}

type registrationHTTPError struct {
	statusCode int
	body       string
}

func (registrationError *registrationHTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", registrationError.statusCode, registrationError.body)
}

func (response *registerResponse) publicURL() string {
	if response.MattermostURL != "" {
		return response.MattermostURL
	}
	return response.URL
}

func (response *registerResponse) registeredFleetID() string {
	return response.FleetID
}

func (response *registerResponse) registeredOldFleetID() string {
	return response.OldFleetID
}

func (response *registerResponse) registeredNodeID() string {
	return response.NodeID
}

func registerFleetNode(configuration config, fleetID, nodeID, nodeKey, fleetSecret, adminEmail string) (*registerResponse, error) {
	return registerFleetRequest(configuration, map[string]string{
		"fleet_id":     fleetID,
		"node_id":      nodeID,
		"node_key":     nodeKey,
		"fleet_secret": fleetSecret,
		"admin_email":  adminEmail,
	})
}

func registerFleetIDMigration(configuration config, oldFleetID, newFleetID, nodeID, nodeKey, fleetSecret, adminEmail string) (*registerResponse, error) {
	return registerFleetRequest(configuration, map[string]string{
		"fleet_id":     oldFleetID,
		"new_fleet_id": newFleetID,
		"node_id":      nodeID,
		"node_key":     nodeKey,
		"fleet_secret": fleetSecret,
		"admin_email":  adminEmail,
	})
}

func registerFleetRequest(configuration config, requestBody map[string]string) (*registerResponse, error) {
	body, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", configuration.APIBaseURL+"/api/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(configuration.RegisterSecret) != "" {
		req.Header.Set("Authorization", "Bearer "+configuration.RegisterSecret)
	}

	resp, err := registerHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, &registrationHTTPError{statusCode: resp.StatusCode, body: string(respBody)}
	}

	var result registerResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return &result, nil
}

func registerFleetNodeWithCollisionRetry(configuration config, stateDir, fleetID, adminEmail string) (*registerResponse, error) {
	fleetSecret := loadOrCreateFleetSecret(stateDir)
	nodeID := loadNodeID(stateDir)
	nodeKey := loadOrCreateNodeKey(stateDir)

	var registrationResponse *registerResponse
	retryError := retryOperation(retryOptions{
		AttemptCount: 5,
		ShouldRetry: func(errorValue error) bool {
			return isFleetIDCollisionRegistrationError(errorValue)
		},
	}, func(attemptIndex int) error {
		response, registerError := registerFleetNode(configuration, fleetID, nodeID, nodeKey, fleetSecret, adminEmail)
		if registerError == nil {
			saveRegistrationResponse(stateDir, response)
			saveDefaultFleetNode(stateDir, response)
			registrationResponse = response
			return nil
		}
		if !isFleetIDCollisionRegistrationError(registerError) {
			return registerError
		}
		if strings.TrimSpace(argString("--fleet", "")) != "" {
			return errors.New("fleet join rejected; check --fleet and --fleet-secret")
		}

		fleetID, fleetSecret = resetFleetIdentity(stateDir)
		fmt.Printf("  Fleet ID collision detected; retrying with %s\n", fleetID)
		return registerError
	})
	if retryError == nil {
		return registrationResponse, nil
	}
	if isFleetIDCollisionRegistrationError(retryError) {
		return nil, errors.New("fleet_id collision retry limit reached")
	}
	return nil, retryError
}

func isFleetIDCollisionRegistrationError(errorValue error) bool {
	var httpError *registrationHTTPError
	return errors.As(errorValue, &httpError) && httpError.statusCode == http.StatusConflict
}

func saveRegistrationResponse(stateDir string, response *registerResponse) {
	saveState(stateDir, "fleet_id", response.registeredFleetID())
	saveState(stateDir, "node_id", firstNonEmptyString(response.registeredNodeID(), loadNodeID(stateDir)))
	saveState(stateDir, "fleet_role", firstNonEmptyString(response.FleetRole, "active"))
	saveState(stateDir, "fleet_active_count", fmt.Sprint(defaultInt(response.FleetActiveCount, 1)))
	saveState(stateDir, "fleet_pending_count", fmt.Sprint(response.FleetPendingCount))
	saveState(stateDir, "fleet_quorum_size", fmt.Sprint(defaultInt(response.FleetQuorumSize, 1)))
	saveState(stateDir, "tunnel_token", response.TunnelToken)
	if strings.TrimSpace(response.NodeTunnelToken) != "" {
		saveState(stateDir, "node_tunnel_token", response.NodeTunnelToken)
	}
	saveState(stateDir, "device_url", response.publicURL())
	if strings.TrimSpace(response.SSHHostname) != "" {
		saveState(stateDir, "ssh_hostname", response.SSHHostname)
	}
	if strings.TrimSpace(response.TLSStatus) != "" {
		saveState(stateDir, "tls_certificate_status", response.TLSStatus)
	}
}

// --- Command helper ---

func runCmd(name string, args ...string) string {
	out, _ := exec.Command(name, args...).CombinedOutput()
	return string(out)
}

// --- State management (~/.internkim/) ---

func toolBinEntries(boardBinDir string, names []string) []struct{ local, remote, name string } {
	var entries []struct{ local, remote, name string }
	for _, name := range names {
		entries = append(entries, struct{ local, remote, name string }{
			filepath.Join(boardBinDir, name),
			"/usr/local/bin/" + name,
			name,
		})
	}
	return entries
}

func internkimHomeDir() string {
	home, _ := os.UserHomeDir()
	newDir := filepath.Join(home, ".internkim")
	os.MkdirAll(newDir, 0700)
	return newDir
}

func setupStateDir(baseStateDir string, boardType string) string {
	stateName := setupStateName(boardType)
	if stateName == "" {
		return baseStateDir
	}
	stateDir := filepath.Join(baseStateDir, "devices", stateName)
	_ = os.MkdirAll(stateDir, 0o700)
	copySetupStateHints(baseStateDir, stateDir)
	return stateDir
}

func setupStateName(boardType string) string {
	normalizedBoardType := strings.TrimSpace(boardType)
	if normalizedBoardType == "" {
		return ""
	}
	var builder strings.Builder
	for _, character := range strings.ToLower(normalizedBoardType) {
		switch {
		case character >= 'a' && character <= 'z':
			builder.WriteRune(character)
		case character >= '0' && character <= '9':
			builder.WriteRune(character)
		case character == '-' || character == '_':
			builder.WriteRune(character)
		default:
			builder.WriteRune('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}

func copySetupStateHints(sourceDir string, destinationDir string) {
	for _, key := range []string{
		"subnet",
		"wifi_ssid",
		"wifi_pass",
		"wifi_open",
		"openrouter_api_key",
	} {
		if loadState(destinationDir, key) != "" {
			continue
		}
		if value := loadState(sourceDir, key); value != "" {
			saveState(destinationDir, key, value)
		}
	}
}

func loadOrCreateFleetID(stateDir string) string {
	id := loadState(stateDir, "fleet_id")
	if id != "" {
		saveState(stateDir, "fleet_id", id)
		return id
	}
	id = randomFleetID()
	saveState(stateDir, "fleet_id", id)
	return id
}

func loadNodeID(stateDir string) string {
	return loadState(stateDir, "node_id")
}

func loadOrCreateNodeKey(stateDir string) string {
	key := loadState(stateDir, "node_key")
	if key != "" {
		return key
	}
	key = "node-" + randomHexString(8)
	saveState(stateDir, "node_key", key)
	return key
}

func isNumericNodeID(nodeID string) bool {
	trimmedNodeID := strings.TrimSpace(nodeID)
	if trimmedNodeID == "" {
		return false
	}
	for index, character := range trimmedNodeID {
		if character < '0' || character > '9' {
			return false
		}
		if index == 0 && character == '0' {
			return false
		}
	}
	return true
}

func loadOrCreateFleetSecret(stateDir string) string {
	secret := loadState(stateDir, "fleet_secret")
	if secret != "" {
		saveState(stateDir, "fleet_secret", secret)
		return secret
	}
	secret = randomHexString(32)
	saveState(stateDir, "fleet_secret", secret)
	return secret
}

func resetFleetIdentity(stateDir string) (string, string) {
	fleetID := randomFleetID()
	fleetSecret := randomHexString(32)
	saveState(stateDir, "fleet_id", fleetID)
	saveState(stateDir, "fleet_secret", fleetSecret)
	saveState(stateDir, "device_url", "")
	saveState(stateDir, "tunnel_token", "")
	return fleetID, fleetSecret
}

func randomFleetID() string {
	return randomAlphanumericString(12)
}

func randomAlphanumericString(length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	randomBytes := make([]byte, length)
	if _, randomError := rand.Read(randomBytes); randomError != nil {
		panic(fmt.Sprintf("crypto random failed: %v", randomError))
	}
	var builder strings.Builder
	builder.Grow(length)
	for _, randomByte := range randomBytes {
		builder.WriteByte(alphabet[int(randomByte)%len(alphabet)])
	}
	return builder.String()
}

func randomHexString(byteCount int) string {
	randomBytes := make([]byte, byteCount)
	if _, randomError := rand.Read(randomBytes); randomError != nil {
		panic(fmt.Sprintf("crypto random failed: %v", randomError))
	}
	return hex.EncodeToString(randomBytes)
}

func saveState(dir, key, value string) {
	os.WriteFile(filepath.Join(dir, key), []byte(value), 0600)
}

func loadState(dir, key string) string {
	data, err := os.ReadFile(filepath.Join(dir, key))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// --- Board detection over SSH ---

func sshCheckHostnameForCredentials(sshpassBin string, ip string, username string, password string) bool {
	hostname, err := runSSHHostnameForCredentials(sshpassBin, ip, username, password)
	return err == nil && strings.TrimSpace(hostname) == "internkim"
}

func runSSHHostnameForCredentials(sshpassBin string, ip string, username string, password string) (string, error) {
	sshArguments := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=5",
		"-o", "LogLevel=ERROR",
	}
	if password == "" {
		sshArguments = append(sshArguments, "-o", "BatchMode=yes")
	}
	sshArguments = append(sshArguments, username+"@"+ip, "hostname")
	commandName := "ssh"
	commandArguments := sshArguments
	if password != "" {
		if sshpassBin == "" {
			return "", errors.New("sshpass is required for password SSH")
		}
		commandName = sshpassBin
		commandArguments = append([]string{"-p", password, "ssh"}, sshArguments...)
	}
	out, err := exec.Command(commandName, commandArguments...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// detectBoardRPi is the legacy entrypoint for saved IP, mDNS, and subnet SSH detection.
// Returns (ip, sshOK). ip may be non-empty with sshOK=false if board responds to ping but not SSH.
func detectBoardRPi(_ string, stateDir string) (string, bool) {
	return detectBoardForSSHCredentials("", stateDir, "root", "")
}

func detectBoardForSSHCredentials(sshpassBin string, stateDir string, sshUsername string, sshPassword string) (string, bool) {
	// 1. Quick check: saved IPs and boot partition
	candidates := []string{}
	for _, vol := range []string{"/Volumes/RPICFG", "/Volumes/bootfs", "/Volumes/boot"} {
		if data, err := os.ReadFile(filepath.Join(vol, "internkim", "board-ip")); err == nil {
			if ip := strings.TrimSpace(string(data)); ip != "" {
				candidates = append(candidates, ip)
			}
		}
	}
	if saved := loadState(stateDir, "board_ip"); saved != "" {
		candidates = append(candidates, saved)
	}
	if saved := loadState(stateDir, "board_wifi_ip"); saved != "" {
		candidates = append(candidates, saved)
	}
	// mDNS resolve (avahi-daemon on RPi)
	if out, err := exec.Command("dns-sd", "-timeout", "3", "-Q", "internkim.local").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "internkim.local") && strings.Contains(line, "192.168") {
				fields := strings.Fields(line)
				for _, f := range fields {
					if strings.HasPrefix(f, "192.168") {
						candidates = append([]string{f}, candidates...)
					}
				}
			}
		}
	}
	if out, err := net.LookupHost("internkim.local"); err == nil && len(out) > 0 {
		candidates = append([]string{out[0]}, candidates...)
	}
	// Try known candidates first (fast path)
	for _, ip := range candidates {
		conn, err := net.DialTimeout("tcp", ip+":22", 3*time.Second)
		if err == nil {
			conn.Close()
			if sshCheckHostnameForCredentials(sshpassBin, ip, sshUsername, sshPassword) {
				saveState(stateDir, "board_ip", ip)
				return ip, true
			}
			if sshPassword == "" {
				saveState(stateDir, "board_ip", ip)
				return ip, true
			}
		}
	}

	// 2. Subnet SSH scan across every local IPv4 subnet
	subnets := localScanSubnets(stateDir)
	if len(subnets) > 0 {
		type result struct {
			ip  string
			ssh bool
		}
		found := make(chan result, 256*len(subnets))
		var wg sync.WaitGroup
		alreadyTried := make(map[string]bool, len(candidates))
		for _, candidate := range candidates {
			alreadyTried[candidate] = true
		}
		for _, subnet := range subnets {
			for i := 2; i <= 254; i++ {
				ip := fmt.Sprintf("%s.%d", subnet, i)
				if alreadyTried[ip] {
					continue
				}
				alreadyTried[ip] = true
				wg.Add(1)
				go func(ip string) {
					defer wg.Done()
					conn, err := net.DialTimeout("tcp", ip+":22", 2*time.Second)
					if err == nil {
						conn.Close()
						if sshCheckHostnameForCredentials(sshpassBin, ip, sshUsername, sshPassword) {
							found <- result{ip, true}
						}
					}
				}(ip)
			}
		}
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		select {
		case r := <-found:
			saveState(stateDir, "board_ip", r.ip)
			return r.ip, r.ssh
		case <-done:
		case <-time.After(20 * time.Second):
		}
	}

	// 3. Ping-only check on known candidates (board may be up but SSH not ready)
	for _, ip := range candidates {
		if strings.HasSuffix(ip, ".1") || strings.HasSuffix(ip, ".255") {
			continue
		}
		if exec.Command("ping", "-c", "1", "-W", "1", ip).Run() == nil {
			return ip, false
		}
	}

	return "", false
}

func findBoardIPForCredentials(sshpassBin string, stateDir string, sshUsername string, sshPassword string) string {
	ip, isSSHReady := detectBoardForSSHCredentials(sshpassBin, stateDir, sshUsername, sshPassword)
	if !isSSHReady {
		return ""
	}
	return ip
}

func describeJetsonSSHFailure(sshpassBin string, stateDir string, sshUsername string, sshPassword string) string {
	candidates := uniqueNonEmptyStrings([]string{
		loadState(stateDir, "board_ip"),
		loadState(stateDir, "board_wifi_ip"),
	})
	if hosts, err := net.LookupHost("internkim.local"); err == nil {
		candidates = uniqueNonEmptyStrings(append(candidates, hosts...))
	}
	if len(candidates) == 0 {
		return "저장된 Jetson IP가 없습니다. Jetson 콘솔에서 `ip addr`로 IP를 확인하세요."
	}
	var lines []string
	for _, ip := range candidates {
		conn, err := net.DialTimeout("tcp", ip+":22", 3*time.Second)
		if err != nil {
			lines = append(lines, fmt.Sprintf("%s: SSH port 22 unreachable (%s)", ip, err))
			continue
		}
		_ = conn.Close()
		hostname, err := runSSHHostnameForCredentials(sshpassBin, ip, sshUsername, sshPassword)
		if err != nil {
			lines = append(lines, fmt.Sprintf("%s: SSH port open but login failed (%s)", ip, err))
			continue
		}
		if strings.TrimSpace(hostname) != "internkim" {
			lines = append(lines, fmt.Sprintf("%s: SSH login ok but hostname is %q", ip, hostname))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: SSH login ok", ip))
	}
	return strings.Join(lines, "\n")
}

func uniqueNonEmptyStrings(values []string) []string {
	seenValues := map[string]bool{}
	var uniqueValues []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seenValues[value] {
			continue
		}
		seenValues[value] = true
		uniqueValues = append(uniqueValues, value)
	}
	return uniqueValues
}

// --- SD card flashing (Raspberry Pi 5) ---

// flashSD detects an SD card, downloads Debian trixie arm64, and writes it.
// After flashing, it mounts the boot partition and injects SSH keys + Wi-Fi config.
// runSetupSD handles the full SD card provisioning flow for Raspberry Pi 5.
func runSetupSD(m *msg) {
	cfg := loadConfig()
	stateDir := internkimHomeDir()
	scriptDir, _ := os.Getwd()
	hardReset := containsArg("--hard-reset")
	reset := hardReset || containsArg("--reset")
	fromStep := argInt("--from", 0)
	shouldRun := func(n int) bool { return fromStep == 0 || n >= fromStep }
	totalSteps := 9

	// Board selection
	boardType := "rpi" // default
	if containsArg("--board") {
		for i, a := range os.Args {
			if a == "--board" && i+1 < len(os.Args) {
				boardType = os.Args[i+1]
			}
		}
	}
	imageURL, ok := armbianImages[boardType]
	if !ok {
		fmt.Printf("지원 보드: ")
		for k := range armbianImages {
			fmt.Printf("%s ", k)
		}
		fmt.Println()
		fatal(fmt.Sprintf("알 수 없는 보드: %s", boardType))
	}
	boardNames := map[string]string{"rpi": "Raspberry Pi", "orangepi5": "Orange Pi 5"}
	fmt.Printf("=== Intern Kim Setup (%s, Armbian Trixie) ===\n", boardNames[boardType])
	fmt.Println()

	// Stop the lab VM if it is running so the same Cloudflare tunnel
	// token cannot race between the VM and the real device.
	if labVirtualMachineIPAddress := resolveLabVirtualMachineIPAddress(); labVirtualMachineIPAddress != "" {
		fmt.Printf("  %s\n", m.t("Lab VM 중지 중 (터널 충돌 방지)...", "Stopping lab VM (tunnel conflict)..."))
		if errorValue := runLabArguments([]string{"vm-down"}); errorValue != nil {
			fmt.Printf("  %s: %v\n", m.t("Lab VM 중지 실패", "Failed to stop lab VM"), errorValue)
		} else {
			fmt.Printf("  %s\n", m.t("Lab VM 중지 완료", "Lab VM stopped"))
		}
	}

	// 1. SD card detection (always needed)
	step(1, totalSteps, m.t("SD 카드 감지 중...", "Detecting SD card..."))
	disk := detectSDCard()
	if disk == "" {
		fatal(m.t(
			"SD 카드를 찾을 수 없습니다.\nSD 카드를 삽입한 후 다시 시도하세요.",
			"SD card not found.\nInsert an SD card and try again.",
		))
	}
	fmt.Printf("  %s: %s\n", m.t("SD 카드 감지", "SD card detected"), disk)

	cacheDir := filepath.Join(stateDir, "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		fatal(err.Error())
	}

	setupBuildID := currentExecutableFingerprint()
	stageDir := filepath.Join(cacheDir, "stage")
	stageBuildIDPath := filepath.Join(stageDir, "setup-build-id")
	if reset {
		_ = os.RemoveAll(stageDir)
	} else if setupBuildID != "" {
		stageBuildIDBytes, err := os.ReadFile(stageBuildIDPath)
		if err != nil || strings.TrimSpace(string(stageBuildIDBytes)) != setupBuildID {
			_ = os.RemoveAll(stageDir)
		}
	}
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		fatal(err.Error())
	}

	flowState := newSetupFlowState(
		m,
		cfg,
		collectSetupParameterValues(),
		stateDir,
		scriptDir,
		setupBuildID,
		nil,
		containsArg("--non-interactive"),
	)
	stageContext := &setup.Context{
		Backend:   setup.BackendSD,
		Language:  m.lang,
		StateDir:  stateDir,
		ScriptDir: scriptDir,
		Force:     reset || containsArg("--force"),
		Callbacks: flowState.callbacks(),
		SD:        sdStagingTarget{stagingRoot: stageDir},
	}
	stageRegistry := setup.DefaultRegistry()
	runStageStep := func(stepNumber int, stepName string) {
		selector := setup.Selector{
			Only:  []string{stepName},
			Force: stageContext.Force && shouldRun(stepNumber),
		}
		if err := stageRegistry.Run(stageContext, selector); err != nil {
			fatal(err.Error())
		}
	}

	step(2, totalSteps, m.t("Wi-Fi 설정...", "Wi-Fi setup..."))
	runStageStep(2, "wifi")

	step(3, totalSteps, m.t("Google Workspace 자격증명...", "Google Workspace credentials..."))
	runStageStep(3, "google")

	step(4, totalSteps, m.t("OpenRouter API 키 설정...", "OpenRouter API key..."))
	runStageStep(4, "openrouter")

	step(5, totalSteps, m.t("기기 등록 + 터널 설정...", "Registering device + tunnel..."))
	runStageStep(5, "tunnel")

	step(6, totalSteps, m.t("부팅 스테이지 준비...", "Preparing boot staging payload..."))
	runStageStep(6, "staging")

	if err := flowState.ensureWiFiCredentials(); err != nil {
		fatal(err.Error())
	}

	// 7. Flash image (skip if same image already on SD)
	step(7, totalSteps, m.t("이미지 굽기...", "Flashing image..."))

	// Pre-download Mattermost tar.gz to cache
	mmCachePath := filepath.Join(cacheDir, "mattermost.tar.gz")
	if _, err := os.Stat(mmCachePath); os.IsNotExist(err) {
		fmt.Printf("  %s... ", m.t("Mattermost 다운로드", "Downloading Mattermost"))
		mmVer := "10.9.1"
		if out, err := exec.Command("sh", "-c", `curl -sf https://api.github.com/repos/mattermost/mattermost/releases/latest | grep '"tag_name"' | head -1 | sed 's/.*"v//;s/".*//'`).Output(); err == nil {
			if v := strings.TrimSpace(string(out)); v != "" {
				mmVer = v
			}
		}
		mmURL := fmt.Sprintf("https://releases.mattermost.com/%s/mattermost-%s-linux-arm64.tar.gz", mmVer, mmVer)
		if err := downloadBinary(mmURL, mmCachePath, ""); err != nil {
			fmt.Printf("FAILED: %v\n", err)
		} else {
			fmt.Printf("ok (%s)\n", mmVer)
		}
	} else {
		fmt.Printf("  %s\n", m.t("Mattermost 캐시 사용", "Using cached Mattermost"))
	}

	imgBase := fmt.Sprintf("armbian-%s-trixie", boardType)
	imgXZ := filepath.Join(cacheDir, imgBase+".img.xz")
	imgRaw := filepath.Join(cacheDir, imgBase+".img")
	// Download compressed image early (needed for rootfs-based deb download)
	if _, err := os.Stat(imgXZ); os.IsNotExist(err) {
		fmt.Printf("  %s...\n", m.t("Armbian trixie 이미지 다운로드 중", "Downloading Armbian trixie image"))
		downloadCommand := exec.Command("curl", "-fSL", "--progress-bar", "-o", imgXZ, "-L", imageURL)
		downloadCommand.Stdout = os.Stdout
		downloadCommand.Stderr = os.Stderr
		if err := downloadCommand.Run(); err != nil {
			os.Remove(imgXZ)
			fatal(m.t("이미지 다운로드 실패.", "Image download failed."))
		}
	}
	// Pre-download .deb packages using Armbian rootfs chroot (version-matched)
	debsTarPath := filepath.Join(cacheDir, "debs.tar")
	if _, err := os.Stat(debsTarPath); os.IsNotExist(err) {
		fmt.Printf("  %s\n", m.t("패키지 사전 다운로드 (Armbian rootfs)", "Pre-downloading packages (Armbian rootfs)"))
		if err := downloadDebsUsingRootfs(imgXZ, debsTarPath, m); err != nil {
			fmt.Printf("  FAILED: %v\n", err)
		} else {
			info, _ := os.Stat(debsTarPath)
			fmt.Printf("  %s (%dMB)\n", m.t("패키지 다운로드 완료", "Package download complete"), info.Size()/1024/1024)
		}
	} else {
		info, _ := os.Stat(debsTarPath)
		fmt.Printf("  %s (%dMB)\n", m.t("패키지 캐시 사용", "Using cached packages"), info.Size()/1024/1024)
	}

	// Check if SD already has the same image and the same setup build.
	exec.Command("diskutil", "mountDisk", disk).Run()
	time.Sleep(2 * time.Second)
	sdSameImage := false
	sdSameSetupBuild := false
	for _, d := range []string{"/Volumes/NO NAME", "/Volumes/RASPIFIRM", "/Volumes/RPICFG", "/Volumes/boot", "/Volumes/bootfs", "/Volumes/armbi_root"} {
		stageDir := filepath.Join(d, "internkim")
		imageMarker := filepath.Join(stageDir, "image-version")
		if data, err := os.ReadFile(imageMarker); err == nil && string(data) == imageURL {
			sdSameImage = true
			if setupBuildID == "" {
				sdSameSetupBuild = true
				break
			}
			buildMarker := filepath.Join(stageDir, "setup-build-id")
			if data, err := os.ReadFile(buildMarker); err == nil && strings.TrimSpace(string(data)) == setupBuildID {
				sdSameSetupBuild = true
				break
			}
		}
	}

	needFlash := reset || !sdSameImage || !sdSameSetupBuild || flowState.wifiChanged
	if !needFlash {
		fmt.Printf("  %s\n", m.t("동일 이미지 감지 — 굽기 건너뜀 (boot 파티션만 업데이트)", "Same image — skipping flash (boot partition update only)"))
	} else {
		if !reset && sdSameImage && !sdSameSetupBuild && !flowState.wifiChanged {
			fmt.Printf("  %s\n", m.t("새 setup 빌드 감지 — 다시 굽기", "New setup build detected — reflashing"))
		}
		fmt.Println()
		if !promptYN(m.t(
			disk+" 의 모든 데이터가 삭제됩니다. 계속하시겠습니까?",
			"All data on "+disk+" will be erased. Continue?",
		)) {
			fatal(m.t("취소됨.", "Cancelled."))
		}
		// Backup data before flash (unless --hard-reset)
		backupDir := filepath.Join(stateDir, "backup")
		if !hardReset {
			os.MkdirAll(backupDir, 0700)
			// Secrets from boot partition (FAT32, macOS-readable)
			for _, vol := range []string{"/Volumes/RPICFG", "/Volumes/bootfs", "/Volumes/boot"} {
				secretsDir := filepath.Join(vol, "internkim", "secrets")
				if _, err := os.Stat(secretsDir); err == nil {
					for _, name := range []string{"mm-admin-pass", "mm-db-pass"} {
						if data, err := os.ReadFile(filepath.Join(secretsDir, name)); err == nil {
							os.WriteFile(filepath.Join(backupDir, name), data, 0600)
						}
					}
					break
				}
			}
			// Workspace + DB from ext4 via debugfs (file-by-file, no full dd)
			backupFromExt4(disk, backupDir, m)
			fmt.Printf("  %s\n", m.t("백업 완료", "Backup complete"))
		} else {
			os.RemoveAll(backupDir)
		}

		// Always extract fresh image (inject needs clean ext4)
		os.Remove(imgRaw)
		fmt.Printf("  %s...\n", m.t("이미지 압축 해제 중 (img.xz → img)", "Extracting image (img.xz → img)"))
		xzCmd := exec.Command("sh", "-c", fmt.Sprintf("xz -dk '%s'", imgXZ))
		xzCmd.Stderr = os.Stderr
		if err := xzCmd.Run(); err != nil {
			os.Remove(imgRaw)
			fatal(m.t("압축 해제 실패.", "Extraction failed."))
		}

		// Inject Wi-Fi, SSH, hostname, firstboot service into ext4 via debugfs
		fmt.Printf("  %s...\n", m.t("이미지에 파일 주입 중 (debugfs)", "Injecting files into image (debugfs)"))
		if err := injectFilesIntoImage(imgRaw, flowState.wifiSSID, flowState.wifiPassword, flowState.publicKey, stageDir); err != nil {
			fatal(fmt.Sprintf("%s: %v", m.t("파일 주입 실패", "File injection failed"), err))
		}
		fmt.Printf("  %s\n", m.t("파일 주입 완료", "Files injected"))

		// Write to SD
		fmt.Printf("  %s...\n", m.t("SD 카드에 이미지 쓰는 중 (수 분 소요)", "Writing image to SD (may take a few minutes)"))
		exec.Command("diskutil", "unmountDisk", disk).Run()
		rdisk := strings.Replace(disk, "/dev/disk", "/dev/rdisk", 1)
		ddCmd := exec.Command("sudo", "dd", "if="+imgRaw, "of="+rdisk, "bs=4m", "status=progress")
		ddCmd.Stdout = os.Stdout
		ddCmd.Stderr = os.Stderr
		if err := ddCmd.Run(); err != nil {
			fatal(m.t("이미지 쓰기 실패.", "Failed to write image."))
		}
		exec.Command("sync").Run()
	}

	// 8. Mount boot partition and stage provisioning data
	// macOS cannot mount ext4 root partition, so we put everything on the
	// FAT32 boot partition. The first-boot script moves files into place.
	step(8, totalSteps, m.t("파일 주입 중...", "Injecting files..."))
	exec.Command("diskutil", "mountDisk", disk).Run()
	time.Sleep(2 * time.Second)

	bootDir := ""
	for _, d := range []string{"/Volumes/NO NAME", "/Volumes/RASPIFIRM", "/Volumes/RPICFG", "/Volumes/boot", "/Volumes/bootfs"} {
		if _, err := os.Stat(d); err == nil {
			bootDir = d
			break
		}
	}
	if bootDir == "" {
		fatal(m.t("boot 파티션을 마운트할 수 없습니다.", "Could not mount boot partition."))
	}

	bootStageDir := filepath.Join(bootDir, "internkim")
	_ = os.RemoveAll(bootStageDir)
	if err := os.MkdirAll(bootStageDir, 0o755); err != nil {
		fatal(err.Error())
	}
	if err := copyDirectoryContents(stageDir, bootStageDir); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("  %s\n", m.t("스테이지 디렉터리 복사 완료", "Stage directory copied"))

	// 8f. sysconf.txt — Debian raspi standard first-boot config
	sysconf := "hostname=internkim\n"
	if flowState.publicKey != "" {
		sysconf += fmt.Sprintf("root_authorized_key=%s\n", flowState.publicKey)
	}
	os.WriteFile(filepath.Join(bootDir, "sysconf.txt"), []byte(sysconf), 0644)
	fmt.Printf("  %s\n", m.t("sysconf.txt 작성 완료", "sysconf.txt written"))

	// Image version marker (skip re-flash next time if same image)
	os.WriteFile(filepath.Join(bootStageDir, "image-version"), []byte(imageURL), 0644)
	if setupBuildID != "" {
		os.WriteFile(filepath.Join(bootStageDir, "setup-build-id"), []byte(setupBuildID), 0644)
	}
	// Build ID (unique per flash, shown in firstboot log)
	buildID := fmt.Sprintf("%x", time.Now().UnixNano())
	os.WriteFile(filepath.Join(bootStageDir, "build-id"), []byte(buildID), 0644)
	fmt.Printf("  Build ID: %s\n", buildID)

	// No cmdline.txt modification needed — systemd service is injected into root partition via container

	// Save local subnet for board discovery
	if out, err := exec.Command("sh", "-c", "route get default 2>/dev/null | awk '/gateway/{print $2}'").Output(); err == nil {
		gw := strings.TrimSpace(string(out))
		if parts := strings.Split(gw, "."); len(parts) == 4 {
			subnet := strings.Join(parts[:3], ".")
			saveState(stateDir, "subnet", subnet)
		}
	}

	// 9. Unmount and done
	step(9, totalSteps, m.t("완료!", "Done!"))
	exec.Command("diskutil", "unmountDisk", disk).Run()

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("  %s\n", m.t("SD 카드 준비 완료!", "SD card ready!"))
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println(m.t(
		"  1. SD 카드를 라즈베리파이 5에 삽입\n  2. 전원 연결\n  3. 첫 부팅 시 자동 설정 (약 5-10분 소요)",
		"  1. Insert SD card into Raspberry Pi 5\n  2. Connect power\n  3. First boot auto-setup (takes ~5-10 min)",
	))
	deviceURL := loadState(stateDir, "device_url")
	adminEmail := loadState(stateDir, "google_email")
	adminPassBytes, _ := os.ReadFile(filepath.Join(stageDir, "secrets", "mm-admin-pass"))
	adminPass := strings.TrimSpace(string(adminPassBytes))
	if deviceURL != "" {
		fmt.Printf("\n  Mattermost: %s\n", deviceURL)
	}
	fmt.Printf("\n  %s:\n", m.t("로그인 정보", "Login"))
	fmt.Printf("    %s: %s\n", m.t("이메일", "Email"), adminEmail)
	fmt.Printf("    %s: %s\n", m.t("비밀번호", "Password"), adminPass)
	fmt.Println()
}

// detectSDCard finds an external physical disk (not disk images) on macOS.
func detectSDCard() string {
	out, err := exec.Command("diskutil", "list", "external").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		// Look for lines like: /dev/disk22 (external, physical):
		if strings.Contains(line, "(external, physical)") {
			parts := strings.Fields(line)
			if len(parts) > 0 && strings.HasPrefix(parts[0], "/dev/disk") {
				return parts[0]
			}
		}
	}
	return ""
}

// --- Wi-Fi ---

func detectSSID(bin string) string {
	out, err := exec.Command(bin).Output()
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	if strings.Contains(s, "Unknown") {
		return ""
	}
	return s
}

func localScanSubnets(stateDirectory string) []string {
	subnetSet := make(map[string]bool)
	if storedSubnet := strings.TrimSpace(loadState(stateDirectory, "subnet")); storedSubnet != "" {
		subnetSet[storedSubnet] = true
	}
	interfaces, errorValue := net.Interfaces()
	if errorValue == nil {
		for _, networkInterface := range interfaces {
			if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addresses, addressError := networkInterface.Addrs()
			if addressError != nil {
				continue
			}
			for _, address := range addresses {
				ipNet, ok := address.(*net.IPNet)
				if !ok {
					continue
				}
				ipv4 := ipNet.IP.To4()
				if ipv4 == nil || ipv4.IsLoopback() || ipv4.IsLinkLocalUnicast() {
					continue
				}
				subnetSet[fmt.Sprintf("%d.%d.%d", ipv4[0], ipv4[1], ipv4[2])] = true
			}
		}
	}
	if output, errorValue := exec.Command("sh", "-c", "route get default 2>/dev/null | awk '/gateway/{print $2}'").Output(); errorValue == nil {
		gateway := strings.TrimSpace(string(output))
		if parts := strings.Split(gateway, "."); len(parts) == 4 {
			subnetSet[strings.Join(parts[:3], ".")] = true
		}
	}
	subnets := make([]string, 0, len(subnetSet))
	for subnet := range subnetSet {
		subnets = append(subnets, subnet)
	}
	sort.Strings(subnets)
	return subnets
}

func detectCurrentWiFiHidden() bool {
	output, errorValue := exec.Command("system_profiler", "SPAirPortDataType").Output()
	if errorValue != nil {
		return false
	}
	text := string(output)
	currentIndex := strings.Index(text, "Current Network Information:")
	if currentIndex < 0 {
		return false
	}
	otherIndex := strings.Index(text[currentIndex:], "Other Local Wi-Fi Networks:")
	currentSection := text[currentIndex:]
	if otherIndex >= 0 {
		currentSection = text[currentIndex : currentIndex+otherIndex]
	}
	for _, line := range strings.Split(currentSection, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "Hidden Network:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "Hidden Network:"))
		return strings.EqualFold(value, "yes") || strings.EqualFold(value, "true")
	}
	return false
}

func findExt4Partition(imgRaw string) (offset, size int64, err error) {
	f, err := os.Open(imgRaw)
	if err != nil {
		return 0, 0, fmt.Errorf("open image: %w", err)
	}
	defer f.Close()
	buf := make([]byte, 16)
	for i := 0; i < 4; i++ {
		f.Seek(446+int64(i*16), 0)
		f.Read(buf)
		partitionType := buf[4]
		lba := int64(buf[8]) | int64(buf[9])<<8 | int64(buf[10])<<16 | int64(buf[11])<<24
		sectors := int64(buf[12]) | int64(buf[13])<<8 | int64(buf[14])<<16 | int64(buf[15])<<24
		if partitionType == 0x83 || (partitionType == 0xee && lba > 2048 && sectors > 100000) {
			return lba * 512, sectors * 512, nil
		}
	}
	return 0, 0, fmt.Errorf("could not find ext4 partition in image")
}

func e2fsprogsBinDir() string {
	homebrewPath := "/opt/homebrew/Cellar/e2fsprogs/1.47.4/sbin"
	if _, err := os.Stat(filepath.Join(homebrewPath, "debugfs")); err == nil {
		return homebrewPath
	}
	return ""
}

func downloadDebsUsingRootfs(imgXZ, debsTarPath string, messenger *msg) error {
	tempImg, err := os.CreateTemp("", "armbian-*.img")
	if err != nil {
		return fmt.Errorf("create temp image: %w", err)
	}
	tempImgPath := tempImg.Name()
	tempImg.Close()
	defer os.Remove(tempImgPath)
	fmt.Printf("    %s\n", messenger.t("이미지 압축 해제 중...", "Extracting image..."))
	extractCommand := exec.Command("sh", "-c", fmt.Sprintf("xz -dc '%s' > '%s'", imgXZ, tempImgPath))
	extractCommand.Stderr = os.Stderr
	if err := extractCommand.Run(); err != nil {
		return fmt.Errorf("extract xz: %w", err)
	}
	partitionOffset, partitionSize, err := findExt4Partition(tempImgPath)
	if err != nil {
		return err
	}
	rootfsDirectory, err := os.MkdirTemp("", "armbian-rootfs-*")
	if err != nil {
		return fmt.Errorf("create rootfs dir: %w", err)
	}
	defer os.RemoveAll(rootfsDirectory)
	rootfsPath := filepath.Join(rootfsDirectory, "rootfs.ext4")
	fmt.Printf("    %s\n", messenger.t("rootfs 파티션 추출 중...", "Extracting rootfs partition..."))
	ddCommand := exec.Command("dd",
		fmt.Sprintf("if=%s", tempImgPath),
		fmt.Sprintf("of=%s", rootfsPath),
		"bs=4096",
		fmt.Sprintf("skip=%d", partitionOffset/4096),
		fmt.Sprintf("count=%d", partitionSize/4096),
	)
	ddCommand.Stderr = os.Stderr
	if err := ddCommand.Run(); err != nil {
		return fmt.Errorf("extract rootfs: %w", err)
	}
	os.Remove(tempImgPath)
	expandBytes := int64(512 * 1024 * 1024)
	if f, err := os.OpenFile(rootfsPath, os.O_WRONLY, 0); err == nil {
		f.Truncate(partitionSize + expandBytes)
		f.Close()
	}
	binDir := e2fsprogsBinDir()
	resize2fsBin := "resize2fs"
	if binDir != "" {
		resize2fsBin = filepath.Join(binDir, "resize2fs")
	}
	exec.Command(resize2fsBin, "-f", rootfsPath).CombinedOutput()
	fmt.Printf("    %s\n", messenger.t("Armbian rootfs에서 패키지 다운로드 중...", "Downloading packages from Armbian rootfs..."))
	chrootScript := fmt.Sprintf(`set -e
apt-get update -qq >/dev/null 2>&1
apt-get install -y -qq e2fsprogs >/dev/null 2>&1
mkdir -p /mnt/armbian
mount -o loop,rw /mnt/host/rootfs.ext4 /mnt/armbian
mount -t proc proc /mnt/armbian/proc
mount --bind /dev /mnt/armbian/dev
rm -f /mnt/armbian/etc/resolv.conf
echo "nameserver 8.8.8.8" > /mnt/armbian/etc/resolv.conf
chroot /mnt/armbian sh -c 'export DEBIAN_FRONTEND=noninteractive; apt-get update -qq >/dev/null 2>&1; . /etc/os-release; runtimePackages="%s"; case "${VERSION_ID:-}" in 22.*) runtimePackages="%s" ;; 24.*|25.*|26.*) runtimePackages="%s" ;; esac; apt-get install -y -d -qq postgresql postgresql-contrib jq avahi-daemon git curl unzip ca-certificates $runtimePackages >/dev/null 2>&1'
tar cf - -C /mnt/armbian/var/cache/apt/archives .
umount /mnt/armbian/dev /mnt/armbian/proc 2>/dev/null; umount /mnt/armbian 2>/dev/null; true`, deviceBrowserRuntimePackageListLegacyUbuntu(), deviceBrowserRuntimePackageListJetPack6(), deviceBrowserRuntimePackageListUbuntu24())
	debCommand := exec.Command("container", "run", "--rm",
		"--volume", rootfsDirectory+":/mnt/host",
		"debian:trixie-slim", "sh", "-c", chrootScript)
	debOutput, err := os.Create(debsTarPath)
	if err != nil {
		return fmt.Errorf("create debs.tar: %w", err)
	}
	debCommand.Stdout = debOutput
	if err := debCommand.Run(); err != nil {
		debOutput.Close()
		os.Remove(debsTarPath)
		return fmt.Errorf("container deb download: %w", err)
	}
	debOutput.Close()
	return nil
}

func injectFilesIntoImage(imgRaw, ssid, wifiPass, pubKey, stageDir string) error {
	binDir := e2fsprogsBinDir()
	debugfsBin := "debugfs"
	if binDir != "" {
		debugfsBin = filepath.Join(binDir, "debugfs")
	}
	partOffset, partSize, err := findExt4Partition(imgRaw)
	if err != nil {
		return err
	}

	// Extract ext4 partition to temp file
	partFile := imgRaw + ".rootfs"
	extractCmd := exec.Command("dd",
		fmt.Sprintf("if=%s", imgRaw),
		fmt.Sprintf("of=%s", partFile),
		"bs=4096",
		fmt.Sprintf("skip=%d", partOffset/4096),
		fmt.Sprintf("count=%d", partSize/4096),
	)
	extractCmd.Stderr = os.Stderr
	if err := extractCmd.Run(); err != nil {
		return fmt.Errorf("extract partition: %w", err)
	}
	defer os.Remove(partFile)

	// Expand ext4 partition to fit injected files (mattermost + debs)
	expandMB := int64(1024) // 1GB extra space
	f2, _ := os.OpenFile(partFile, os.O_WRONLY, 0)
	if f2 != nil {
		f2.Seek(0, 2) // end
		f2.Truncate(partSize + expandMB*1024*1024)
		f2.Close()
	}
	resize2fsBin := filepath.Join(filepath.Dir(debugfsBin), "resize2fs")
	if out, err := exec.Command(resize2fsBin, "-f", partFile).CombinedOutput(); err != nil {
		fmt.Printf("    resize2fs warning: %s\n", string(out))
	}
	partSize = partSize + expandMB*1024*1024

	dbgRun := func(cmd string) {
		exec.Command(debugfsBin, "-w", "-R", cmd, partFile).CombinedOutput()
	}

	ensureDirs := func(ext4Path string) {
		parts := strings.Split(filepath.Dir(ext4Path), "/")
		cur := ""
		for _, p := range parts {
			if p == "" {
				continue
			}
			cur += "/" + p
			dbgRun(fmt.Sprintf("mkdir %s", cur))
		}
	}

	writeFile := func(localPath, ext4Path string, mode string) error {
		ensureDirs(ext4Path)
		dbgRun(fmt.Sprintf("rm %s", ext4Path))
		// debugfs returns exit 0 even on failure, so check output for errors
		cmd := exec.Command(debugfsBin, "-w", "-R", fmt.Sprintf("write %s %s", localPath, ext4Path), partFile)
		out, err := cmd.CombinedOutput()
		outStr := string(out)
		if err != nil {
			return fmt.Errorf("debugfs write %s: %s", ext4Path, outStr)
		}
		if strings.Contains(outStr, "already exists") || strings.Contains(outStr, "No space") {
			return fmt.Errorf("debugfs write %s: %s", ext4Path, outStr)
		}
		if mode != "" {
			dbgRun(fmt.Sprintf("set_inode_field %s mode %s", ext4Path, mode))
		}
		return nil
	}

	writeContent := func(content, ext4Path, mode string) error {
		tmp, _ := os.CreateTemp("", "inject-*")
		tmp.WriteString(content)
		tmp.Sync()
		tmp.Close()
		defer os.Remove(tmp.Name())
		return writeFile(tmp.Name(), ext4Path, mode)
	}

	mkSymlink := func(linkPath, target string) {
		ensureDirs(linkPath)
		dbgRun(fmt.Sprintf("symlink %s %s", linkPath, target))
	}

	// ── 1. Wi-Fi (wpa_supplicant@wlan0 + systemd-networkd) ──
	if ssid != "" {
		fmt.Println("    Wi-Fi config")
		var wpaConf string
		if wifiPass != "" {
			wpaConf = fmt.Sprintf("ctrl_interface=DIR=/run/wpa_supplicant GROUP=netdev\ncountry=KR\nap_scan=1\nnetwork={\n  ssid=\"%s\"\n  scan_ssid=1\n  key_mgmt=WPA-PSK\n  psk=\"%s\"\n}\n", ssid, wifiPass)
		} else {
			wpaConf = fmt.Sprintf("ctrl_interface=DIR=/run/wpa_supplicant GROUP=netdev\ncountry=KR\nap_scan=1\nnetwork={\n  ssid=\"%s\"\n  scan_ssid=1\n  key_mgmt=NONE\n}\n", ssid)
		}
		writeContent(wpaConf, "/etc/wpa_supplicant/wpa_supplicant-wlan0.conf", "0100600")
		// systemd-networkd DHCP for wlan0
		writeContent("[Match]\nName=wlan0\n\n[Network]\nDHCP=yes\n\n[DHCPv4]\nRouteMetric=20\n", "/etc/systemd/network/20-wlan0.network", "0100644")
		// Enable wpa_supplicant@wlan0 (global service will be masked at runtime by firstboot)
		mkSymlink("/etc/systemd/system/multi-user.target.wants/wpa_supplicant@wlan0.service",
			"/usr/lib/systemd/system/wpa_supplicant@.service")
	}

	// ── 2. Armbian first-run auto-config (skip interactive root password prompt) ──
	fmt.Println("    armbian first-run config")
	armbianConf := "PRESET_NET_CHANGE_DEFAULTS=\"1\"\n"
	armbianConf += "PRESET_NET_WIFI_ENABLED=\"1\"\n"
	armbianConf += fmt.Sprintf("PRESET_NET_WIFI_SSID=\"%s\"\n", ssid)
	armbianConf += fmt.Sprintf("PRESET_NET_WIFI_KEY=\"%s\"\n", wifiPass)
	armbianConf += "PRESET_NET_WIFI_COUNTRYCODE=\"KR\"\n"
	armbianConf += "PRESET_CONNECT_WIRELESS=\"n\"\n"
	armbianConf += "SET_LANG_BASED_ON_LOCATION=\"n\"\n"
	armbianConf += "PRESET_LOCALE=\"en_US.UTF-8\"\n"
	armbianConf += "PRESET_TIMEZONE=\"Asia/Seoul\"\n"
	armbianConf += "PRESET_ROOT_PASSWORD=\"internkim\"\n"
	armbianConf += "PRESET_USER_NAME=\"internkim\"\n"
	armbianConf += "PRESET_USER_PASSWORD=\"internkim\"\n"
	armbianConf += "PRESET_DEFAULT_REALNAME=\"Intern Kim\"\n"
	armbianConf += "PRESET_USER_SHELL=\"bash\"\n"
	writeContent(armbianConf, "/root/.not_logged_in_yet", "0100644")

	// ── 3. Hostname ──
	fmt.Println("    hostname")
	writeContent("internkim\n", "/etc/hostname", "0100644")

	// ── 3. SSH ──
	if pubKey != "" {
		fmt.Println("    SSH")
		writeContent(pubKey+"\n", "/root/.ssh/authorized_keys", "0100600")
		dbgRun("set_inode_field /root/.ssh mode 040700")
		writeContent("PermitRootLogin yes\nPasswordAuthentication no\n", "/etc/ssh/sshd_config.d/internkim.conf", "0100644")
	}

	// ── 4. Firstboot ──
	fmt.Println("    firstboot")
	firstbootSrc := filepath.Join(stageDir, "internkim-firstboot.sh")
	if _, err := os.Stat(firstbootSrc); err == nil {
		writeFile(firstbootSrc, "/usr/local/bin/internkim-firstboot.sh", "0100755")
	} else {
		fmt.Printf("      WARN: firstboot script not found at %s\n", firstbootSrc)
	}

	// Single trigger: systemd service that waits for network + boot partition
	svcContent := "[Unit]\nDescription=Intern Kim First Boot\nAfter=local-fs.target armbian-firstrun.service armbian-resize-filesystem.service\nWants=local-fs.target\nConditionPathExists=/usr/local/bin/internkim-firstboot.sh\n\n[Service]\nType=oneshot\nExecStartPre=/bin/bash -c 'for i in $$(seq 1 60); do [ -d /boot/firmware/internkim ] && exit 0; sleep 2; done; exit 1'\nExecStart=/usr/local/bin/internkim-firstboot.sh\nRestart=on-failure\nRestartSec=15\nStartLimitIntervalSec=0\nTimeoutStartSec=900\nStandardOutput=journal+console\nStandardError=journal+console\n\n[Install]\nWantedBy=multi-user.target\n"
	writeContent(svcContent, "/etc/systemd/system/internkim-firstboot.service", "0100644")
	mkSymlink("/etc/systemd/system/multi-user.target.wants/internkim-firstboot.service",
		"/etc/systemd/system/internkim-firstboot.service")

	// ── 5. Mattermost tar.gz ──
	mmCachePath := filepath.Join(filepath.Dir(stageDir), "mattermost.tar.gz")
	if _, err := os.Stat(mmCachePath); err == nil {
		fmt.Println("    mattermost.tar.gz")
		if err := writeFile(mmCachePath, "/var/cache/internkim/mattermost.tar.gz", "0100644"); err != nil {
			return fmt.Errorf("inject mattermost.tar.gz: %w", err)
		}
	}

	// ── 5b. Pre-downloaded .deb packages ──
	debsTarPath := filepath.Join(filepath.Dir(stageDir), "debs.tar")
	if _, err := os.Stat(debsTarPath); err == nil {
		fmt.Println("    debs.tar (pre-downloaded packages)")
		if err := writeFile(debsTarPath, "/var/cache/internkim/debs.tar", "0100644"); err != nil {
			return fmt.Errorf("inject debs.tar: %w", err)
		}
	} else {
		return fmt.Errorf("debs.tar not found at %s", debsTarPath)
	}

	// ── 6. Cloudflared service ──
	fmt.Println("    cloudflared service")
	cfService := "[Unit]\nDescription=Cloudflare Tunnel\nAfter=network-online.target time-sync.target\nWants=network-online.target time-sync.target\n\n[Service]\nType=simple\nExecStart=/bin/sh -c '/usr/local/bin/cloudflared tunnel run --protocol quic --token \"$(cat /root/.internkim/secrets/tunnel-token)\"'\nRestart=always\nRestartSec=5\n\n[Install]\nWantedBy=multi-user.target\n"
	writeContent(cfService, "/etc/systemd/system/cloudflared.service", "0100644")
	mkSymlink("/etc/systemd/system/multi-user.target.wants/cloudflared.service",
		"/etc/systemd/system/cloudflared.service")

	// ── 7. Watchdog service (runs every boot — ensures Wi-Fi, DNS, SSH, LED) ──
	fmt.Println("    watchdog service")
	watchdogScript := `#!/bin/bash
# Skip if firstboot is still pending
[ -f /usr/local/bin/internkim-firstboot.sh ] && exit 0
# Mask competing network managers (idempotent)
systemctl mask wpa_supplicant.service 2>/dev/null
systemctl mask NetworkManager 2>/dev/null
systemctl mask dhcpcd 2>/dev/null
# Check if Wi-Fi is connected
if ip addr show wlan0 | grep -q 'inet '; then
  : # Wi-Fi OK
else
  # Wi-Fi down — reconnect
  rm -f /run/wpa_supplicant/wlan0
  rm -f /etc/systemd/network/10-netplan-wlan0.network
  systemctl restart wpa_supplicant@wlan0
  systemctl restart systemd-networkd
  for i in $(seq 1 30); do
    ip addr show wlan0 | grep -q 'inet ' && break
    sleep 2
  done
fi
# Ensure DNS
if ! getent hosts google.com >/dev/null 2>&1; then
  DHCP_DNS=$(networkctl status wlan0 2>/dev/null | grep 'DNS:' | awk '{print $2}' | head -1)
  [ -n "$DHCP_DNS" ] && echo "nameserver $DHCP_DNS" > /etc/resolv.conf
  [ -z "$DHCP_DNS" ] && echo "nameserver 8.8.8.8" > /etc/resolv.conf
fi
# Ensure SSH
systemctl start ssh 2>/dev/null || systemctl start sshd 2>/dev/null
# Force NTP sync (RPi5 has no RTC — JWTs fail if clock is stale)
if ! timedatectl show --property=NTPSynchronized --value | grep -q '^yes$'; then
  systemctl restart systemd-timesyncd 2>/dev/null || true
  for i in $(seq 1 15); do
    [ "$(timedatectl show --property=NTPSynchronized --value)" = "yes" ] && break
    sleep 1
  done
fi
# Ensure /etc/hosts maps the hostname (sudo uses this; missing entry trips gws)
HN=$(hostname)
if [ -n "$HN" ] && ! grep -qw "$HN" /etc/hosts; then
  echo "127.0.1.1 $HN" >> /etc/hosts
fi
# Record IP to boot partition
CURRENT_IP=$(ip -4 addr show wlan0 | grep -oP 'inet \K[^/]+' | head -1)
if [ -n "$CURRENT_IP" ]; then
  mountpoint -q /boot/firmware || mount /boot/firmware 2>/dev/null
  mkdir -p /boot/firmware/internkim
  echo "$CURRENT_IP" > /boot/firmware/internkim/board-ip
fi
# Dump Mattermost DB for backup recovery
if systemctl is-active --quiet postgresql 2>/dev/null; then
  su - postgres -c "pg_dump mattermost" > /var/cache/internkim/mattermost-db.sql 2>/dev/null || true
fi
# LED heartbeat
if [ -f /sys/class/leds/ACT/trigger ]; then
  echo heartbeat > /sys/class/leds/ACT/trigger 2>/dev/null || true
fi
`
	writeContent(watchdogScript, "/usr/local/bin/internkim-watchdog.sh", "0100755")
	watchdogService := "[Unit]\nDescription=Intern Kim Watchdog\n\n[Service]\nType=oneshot\nExecStart=/usr/local/bin/internkim-watchdog.sh\nTimeoutStartSec=120\n"
	writeContent(watchdogService, "/etc/systemd/system/internkim-watchdog.service", "0100644")
	watchdogTimer := "[Unit]\nDescription=Intern Kim Watchdog Timer\n\n[Timer]\nOnBootSec=30\nOnUnitActiveSec=300\n\n[Install]\nWantedBy=timers.target\n"
	writeContent(watchdogTimer, "/etc/systemd/system/internkim-watchdog.timer", "0100644")
	mkSymlink("/etc/systemd/system/timers.target.wants/internkim-watchdog.timer",
		"/etc/systemd/system/internkim-watchdog.timer")

	// ── 8. Expand raw image, update MBR, and write partition back ──
	fmt.Println("    writing partition back")
	newImgSize := partOffset + partSize
	if fi, err := os.Stat(imgRaw); err == nil && fi.Size() < newImgSize {
		os.Truncate(imgRaw, newImgSize)
	}

	// Update MBR partition table with new ext4 size
	newSectors := partSize / 512
	mbrF, err := os.OpenFile(imgRaw, os.O_RDWR, 0)
	if err == nil {
		// Find the ext4 partition entry (type 0x83) and update its sector count
		for i := 0; i < 4; i++ {
			var pbuf [16]byte
			mbrF.Seek(446+int64(i*16), 0)
			mbrF.Read(pbuf[:])
			if pbuf[4] == 0x83 {
				// Update sector count (bytes 12-15, little-endian)
				pbuf[12] = byte(newSectors)
				pbuf[13] = byte(newSectors >> 8)
				pbuf[14] = byte(newSectors >> 16)
				pbuf[15] = byte(newSectors >> 24)
				mbrF.Seek(446+int64(i*16), 0)
				mbrF.Write(pbuf[:])
				break
			}
		}
		mbrF.Close()
	}

	writeBackCmd := exec.Command("dd",
		fmt.Sprintf("if=%s", partFile),
		fmt.Sprintf("of=%s", imgRaw),
		"bs=4096",
		fmt.Sprintf("seek=%d", partOffset/4096),
		"conv=notrunc",
	)
	writeBackCmd.Stderr = os.Stderr
	if err := writeBackCmd.Run(); err != nil {
		return fmt.Errorf("write partition back: %w", err)
	}

	return nil
}

func getKeychainPassword(ssid string) string {
	out, err := exec.Command("security", "find-generic-password",
		"-D", "AirPort network password", "-wa", ssid).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// --- SSH helpers ---

type sshClient struct {
	sshpassBin   string
	user         string
	pass         string
	host         string
	port         string
	proxyCommand string
}

func newSSH(sshpassBin, user, pass, host string) *sshClient {
	return &sshClient{sshpassBin: sshpassBin, user: user, pass: pass, host: host, port: "22"}
}

func newSSHWithPort(sshpassBin, user, pass, host, port string) *sshClient {
	return &sshClient{sshpassBin: sshpassBin, user: user, pass: pass, host: host, port: port}
}

func newCloudflareSSH(sshpassBin, user, pass, host string) *sshClient {
	if port := strings.TrimSpace(os.Getenv("INTERNKIM_CLOUDFLARE_SSH_LOCAL_PORT")); port != "" {
		return newSSHWithPort(sshpassBin, user, pass, "localhost", port)
	}
	client := newSSH(sshpassBin, user, pass, host)
	client.proxyCommand = "env GODEBUG=netdns=go TUNNEL_EDGE_IP_VERSION=4 cloudflared --edge-ip-version 4 --edge-bind-address 0.0.0.0 access ssh" + cloudflareAccessServiceTokenArguments() + " --hostname %h"
	return client
}

type cloudflareAccessServiceToken struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
}

func cloudflareAccessServiceTokenArguments() string {
	token, isFound := loadCloudflareAccessServiceToken()
	if !isFound {
		return ""
	}
	return " --service-token-id " + token.ClientID + " --service-token-secret " + token.ClientSecret
}

func loadCloudflareAccessServiceToken() (cloudflareAccessServiceToken, bool) {
	clientID := strings.TrimSpace(os.Getenv("INTERNKIM_CF_ACCESS_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("INTERNKIM_CF_ACCESS_CLIENT_SECRET"))
	if clientID != "" && clientSecret != "" {
		return cloudflareAccessServiceToken{ClientID: clientID, ClientSecret: clientSecret}, true
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return cloudflareAccessServiceToken{}, false
	}
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, ".local", "secrets", "cloudflare-access-service-token.json"))
	if errorValue != nil {
		return cloudflareAccessServiceToken{}, false
	}
	var token cloudflareAccessServiceToken
	if json.Unmarshal(document, &token) != nil {
		return cloudflareAccessServiceToken{}, false
	}
	if strings.TrimSpace(token.ClientID) == "" || strings.TrimSpace(token.ClientSecret) == "" {
		return cloudflareAccessServiceToken{}, false
	}
	return token, true
}

func (s *sshClient) sshArgs(extra ...string) []string {
	base := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-o", "LogLevel=ERROR",
		"-p", s.port,
	}
	if s.pass != "" {
		base = append(base, "-o", "PreferredAuthentications=password", "-o", "PubkeyAuthentication=no")
	}
	if s.proxyCommand != "" {
		base = append(base, "-o", "ProxyCommand="+s.proxyCommand)
	}
	return append(base, extra...)
}

func (s *sshClient) run(cmd string) string {
	out, _ := s.runResult(cmd)
	return out
}

func (s *sshClient) runResult(cmd string) (string, error) {
	return s.runResultWithTimeout(cmd, 60*time.Second)
}

func (s *sshClient) runResultWithTimeout(cmd string, timeout time.Duration) (string, error) {
	var args []string
	remoteCommand := s.privilegedCommand(cmd)
	if s.pass == "" {
		args = s.sshArgs(fmt.Sprintf("%s@%s", s.user, s.host), remoteCommand)
		return runSSHCommandWithRetry("ssh", args, timeout)
	}

	args = append([]string{"-p", s.pass, "ssh"}, s.sshArgs(fmt.Sprintf("%s@%s", s.user, s.host), remoteCommand)...)
	return runSSHCommandWithRetry(s.sshpassBin, args, timeout)
}

func (s *sshClient) runInteractiveSSH(remoteArguments []string) error {
	target := fmt.Sprintf("%s@%s", s.user, s.host)
	commandName := "ssh"
	commandArguments := append(s.sshArgs(target), remoteArguments...)
	if s.pass != "" {
		commandName = s.sshpassBin
		commandArguments = append([]string{"-p", s.pass, "ssh"}, append(s.sshArgs(target), remoteArguments...)...)
	}
	command := exec.Command(commandName, commandArguments...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func runSSHCommandWithRetry(commandName string, arguments []string, timeout time.Duration) (string, error) {
	var output []byte
	var errorValue error
	errorValue = retryOperation(retryOptions{
		AttemptCount: 8,
		DelayForAttempt: func(attemptIndex int) time.Duration {
			return time.Duration(attemptIndex+1) * time.Second
		},
		ShouldRetry: func(errorValue error) bool {
			return errorValue != nil && isRetryableSSHFailure(string(output))
		},
		SleepAfterFinalAttempt: true,
	}, func(attemptIndex int) error {
		commandContext, cancel := context.WithTimeout(context.Background(), timeout)
		command := exec.CommandContext(commandContext, commandName, arguments...)
		output, errorValue = command.CombinedOutput()
		if commandContext.Err() == context.DeadlineExceeded {
			errorValue = fmt.Errorf("ssh command timed out after %s: %w", timeout, commandContext.Err())
			output = append(output, []byte("\nssh command timed out")...)
		}
		cancel()
		return errorValue
	})
	return string(output), errorValue
}

func isRetryableSSHFailure(output string) bool {
	for _, phrase := range []string{
		"Connection refused",
		"Operation timed out",
		"Connection timed out",
		"no route to host",
		"Network is unreachable",
		"Permission denied, please try again.",
		"ssh command timed out",
	} {
		if strings.Contains(output, phrase) {
			return true
		}
	}
	return false
}

func (s *sshClient) privilegedCommand(command string) string {
	if s.user == "root" {
		return command
	}
	if s.pass != "" {
		return "printf '%s\n' " + quoteShellValue(s.pass) + " | sudo -S -p '' bash -lc " + quoteShellValue(command)
	}
	return "sudo -p '' bash -lc " + quoteShellValue(command)
}

func (s *sshClient) scpArgs(extra ...string) []string {
	base := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-o", "LogLevel=ERROR",
		"-P", s.port,
	}
	if s.pass != "" {
		base = append(base, "-o", "PreferredAuthentications=password", "-o", "PubkeyAuthentication=no")
	}
	if s.proxyCommand != "" {
		base = append(base, "-o", "ProxyCommand="+s.proxyCommand)
	}
	return append(base, extra...)
}

func (s *sshClient) scp(localPath, remotePath string) error {
	if s.user != "root" && strings.HasPrefix(remotePath, "/") {
		temporaryRemotePath := temporaryUploadPath(remotePath)
		if err := s.scpDirect(localPath, temporaryRemotePath); err != nil {
			return err
		}
		output, err := s.runResult(moveUploadedPathCommand(temporaryRemotePath, remotePath))
		if err != nil {
			return fmt.Errorf("move uploaded file to %s: %s: %w", remotePath, strings.TrimSpace(output), err)
		}
		return nil
	}
	return s.scpDirect(localPath, remotePath)
}

func temporaryUploadPath(remotePath string) string {
	return "/tmp/internkim-upload-" + filepath.Base(remotePath)
}

func (s *sshClient) rsyncSparse(localPath string, remotePath string) error {
	uploadRemotePath := remotePath
	if s.user != "root" && strings.HasPrefix(remotePath, "/") {
		uploadRemotePath = temporaryUploadPath(remotePath)
	}
	target := fmt.Sprintf("%s@%s:%s", s.user, s.host, uploadRemotePath)
	if uploadRemotePath != remotePath {
		if output, errorValue := s.runResult("rm -f " + quoteShellValue(uploadRemotePath)); errorValue != nil {
			return fmt.Errorf("remove stale upload file %s: %s: %w", uploadRemotePath, strings.TrimSpace(output), errorValue)
		}
	}
	if errorValue := s.runRsyncSparse(localPath, remotePath, target); errorValue != nil {
		return errorValue
	}
	if uploadRemotePath == remotePath {
		return nil
	}
	moveOutput, errorValue := s.runResult(moveUploadedPathCommand(uploadRemotePath, remotePath))
	if errorValue != nil {
		return fmt.Errorf("move uploaded file to %s: %s: %w", remotePath, strings.TrimSpace(moveOutput), errorValue)
	}
	return nil
}

func (s *sshClient) runRsyncSparse(localPath string, remotePath string, target string) error {
	sshCommand := s.rsyncSSHCommand("ssh")
	command := exec.Command("rsync", rsyncSparseArguments(sshCommand, localPath, target)...)
	if s.pass != "" {
		command.Env = append(os.Environ(), "SSHPASS="+s.pass)
		sshpassCommand := s.rsyncSSHCommand(quoteShellValue(s.sshpassBin) + " -e ssh")
		command.Args = append([]string{"rsync"}, rsyncSparseArguments(sshpassCommand, localPath, target)...)
	}
	output, errorValue := runCommandWithLiveOutput(command)
	if errorValue != nil {
		return fmt.Errorf("rsync sparse %s to %s failed: %s: %w", localPath, remotePath, strings.TrimSpace(output), errorValue)
	}
	return nil
}

func moveUploadedPathCommand(sourcePath string, targetPath string) string {
	return fmt.Sprintf(
		"mkdir -p %s && mv %s %s",
		quoteShellValue(filepath.Dir(targetPath)),
		quoteShellValue(sourcePath),
		quoteShellValue(targetPath),
	)
}

func rsyncSparseArguments(sshCommand string, localPath string, target string) []string {
	return []string{"-azSh", "--partial", "--progress", "-e", sshCommand, localPath, target}
}

func (s *sshClient) rsyncSSHCommand(commandName string) string {
	command := commandName + " -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=10 -o LogLevel=ERROR -p " + s.port
	if s.proxyCommand == "" {
		return command
	}
	return command + " -o " + quoteShellValue("ProxyCommand="+s.proxyCommand)
}

func runCommandWithLiveOutput(command *exec.Cmd) (string, error) {
	var output bytes.Buffer
	command.Stdout = io.MultiWriter(os.Stdout, &output)
	command.Stderr = io.MultiWriter(os.Stderr, &output)
	errorValue := command.Run()
	return output.String(), errorValue
}

func (s *sshClient) scpDirect(localPath, remotePath string) error {
	target := fmt.Sprintf("%s@%s:%s", s.user, s.host, remotePath)
	if s.pass != "" {
		output, err := runSSHCommandWithRetry(s.sshpassBin, append([]string{"-p", s.pass, "scp"}, s.scpArgs(localPath, target)...), 60*time.Second)
		if err != nil {
			return fmt.Errorf("scp %s to %s failed: %s: %w", localPath, remotePath, strings.TrimSpace(output), err)
		}
		return nil
	}
	output, err := runSSHCommandWithRetry("scp", s.scpArgs(localPath, target), 60*time.Second)
	if err != nil {
		return fmt.Errorf("scp %s to %s failed: %s: %w", localPath, remotePath, strings.TrimSpace(output), err)
	}
	return nil
}

func (s *sshClient) scpDir(localDir, remoteDir string) error {
	if s.user != "root" && strings.HasPrefix(remoteDir, "/") {
		temporaryRemoteDirectory := fmt.Sprintf("/tmp/internkim-upload-%d-%s", os.Getpid(), filepath.Base(remoteDir))
		return s.uploadDirectoryArchive(localDir, temporaryRemoteDirectory, remoteDir)
	}
	return s.scpDirDirect(localDir, remoteDir)
}

func (s *sshClient) uploadDirectoryArchive(localDir string, temporaryRemoteDirectory string, remoteDir string) error {
	extractCommand := fmt.Sprintf(
		"rm -rf %s && mkdir -p %s && tar -xzf - -C %s",
		quoteShellValue(temporaryRemoteDirectory),
		quoteShellValue(temporaryRemoteDirectory),
		quoteShellValue(temporaryRemoteDirectory),
	)
	if output, errorValue := s.runTarToRemote(localDir, extractCommand); errorValue != nil {
		return fmt.Errorf("upload directory %s to %s failed: %s: %w", localDir, remoteDir, strings.TrimSpace(output), errorValue)
	}
	output, errorValue := s.runResult(fmt.Sprintf(
		"mkdir -p %s && cp -a %s/. %s/ && rm -rf %s",
		quoteShellValue(remoteDir),
		quoteShellValue(temporaryRemoteDirectory),
		quoteShellValue(remoteDir),
		quoteShellValue(temporaryRemoteDirectory),
	))
	if errorValue != nil {
		return fmt.Errorf("move uploaded directory to %s: %s: %w", remoteDir, strings.TrimSpace(output), errorValue)
	}
	return nil
}

func (s *sshClient) runTarToRemote(localDir string, remoteCommand string) (string, error) {
	var output string
	var errorValue error
	errorValue = retryOperation(retryOptions{
		AttemptCount: 8,
		DelayForAttempt: func(attemptIndex int) time.Duration {
			return time.Duration(attemptIndex+1) * time.Second
		},
		ShouldRetry: func(errorValue error) bool {
			return errorValue != nil && isRetryableSSHFailure(output)
		},
		SleepAfterFinalAttempt: true,
	}, func(attemptIndex int) error {
		output, errorValue = s.runTarToRemoteOnce(localDir, remoteCommand)
		return errorValue
	})
	return output, errorValue
}

func (s *sshClient) runTarToRemoteOnce(localDir string, remoteCommand string) (string, error) {
	tarCommand := exec.Command("tar", "-czf", "-", "-C", localDir, ".")
	tarCommand.Env = append(os.Environ(), "COPYFILE_DISABLE=1")
	tarOutput, errorValue := tarCommand.StdoutPipe()
	if errorValue != nil {
		return "", errorValue
	}
	target := fmt.Sprintf("%s@%s", s.user, s.host)
	commandName := "ssh"
	commandArguments := s.sshArgs(target, remoteCommand)
	if s.pass != "" {
		commandName = s.sshpassBin
		commandArguments = append([]string{"-e", "ssh"}, commandArguments...)
	}
	sshCommand := exec.Command(commandName, commandArguments...)
	if s.pass != "" {
		sshCommand.Env = append(os.Environ(), "SSHPASS="+s.pass)
	}
	sshCommand.Stdin = tarOutput
	var output bytes.Buffer
	sshCommand.Stdout = &output
	sshCommand.Stderr = &output
	if errorValue := sshCommand.Start(); errorValue != nil {
		return output.String(), errorValue
	}
	_ = tarOutput.Close()
	if errorValue := tarCommand.Start(); errorValue != nil {
		_ = sshCommand.Process.Kill()
		return output.String(), errorValue
	}
	tarError := tarCommand.Wait()
	sshError := sshCommand.Wait()
	if tarError != nil {
		return output.String(), tarError
	}
	return output.String(), sshError
}

func (s *sshClient) scpDirDirect(localDir, remoteDir string) error {
	target := fmt.Sprintf("%s@%s:%s", s.user, s.host, remoteDir)
	if s.pass != "" {
		output, err := runSSHCommandWithRetry(s.sshpassBin, append([]string{"-p", s.pass, "scp", "-r"}, s.scpArgs(localDir+"/.", target)...), 60*time.Second)
		if err != nil {
			return fmt.Errorf("scp directory %s to %s failed: %s: %w", localDir, remoteDir, strings.TrimSpace(output), err)
		}
		return nil
	}
	output, err := runSSHCommandWithRetry("scp", append([]string{"-r"}, s.scpArgs(localDir+"/.", target)...), 60*time.Second)
	if err != nil {
		return fmt.Errorf("scp directory %s to %s failed: %s: %w", localDir, remoteDir, strings.TrimSpace(output), err)
	}
	return nil
}

// --- Config builders ---

func generateFirstbootScript(deviceURL, adminEmail string) string {
	return buildFirstbootScript(deviceURL, adminEmail)
}

func gasWebhookURLPath() string {
	return filepath.Join(internkimHomeDir(), "gas-webhook-url")
}

func loadGasWebhookURL() (string, error) {
	data, err := os.ReadFile(gasWebhookURLPath())
	if err != nil {
		return "", err
	}
	webhookURL := strings.TrimSpace(string(data))
	if webhookURL == "" {
		return "", fmt.Errorf("empty webhook URL at %s", gasWebhookURLPath())
	}
	return webhookURL, nil
}

func provisionGasWebhook(_ string) (string, error) {
	if webhookURL, err := loadGasWebhookURL(); err == nil {
		return webhookURL, nil
	}
	return "", fmt.Errorf("Google Workspace webhook URL is missing; pass --gas-webhook-url, set INTERNKIM_GAS_WEBHOOK_URL, or install it through Companion")
}

// --- UI helpers ---

type msg struct{ lang string }

func newMsg(lang string) *msg { return &msg{lang: lang} }

func (m *msg) t(ko, en string) string {
	if m.lang == "en" {
		return en
	}
	return ko
}

func step(n, total int, text string) {
	fmt.Printf("\n[%d/%d] %s\n", n, total, text)
}

func fatal(text string) {
	fmt.Fprintf(os.Stderr, "\n✗ %s\n", text)
	os.Exit(1)
}

func readSecret(prompt string) string {
	fmt.Print(prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func generatePassword(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	for i, v := range b {
		b[i] = chars[int(v)%len(chars)]
	}
	return string(b)
}

func readLine(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func promptYN(question string) bool {
	fmt.Printf("  %s (y/N): ", question)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}

func maskString(s string) string {
	return strings.Repeat("*", len(s))
}

func maskKey(s string) string {
	if len(s) <= 12 {
		return strings.Repeat("*", len(s))
	}
	return s[:8] + strings.Repeat("*", len(s)-12) + s[len(s)-4:]
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func containsArg(flag string) bool {
	for _, a := range os.Args {
		if a == flag {
			return true
		}
	}
	return false
}

func hasFlag(flag string) bool {
	for _, argument := range os.Args {
		if argument == flag || strings.HasPrefix(argument, flag+"=") {
			return true
		}
	}
	return false
}

func argInt(flag string, fallback int) int {
	for i, a := range os.Args {
		if a == flag && i+1 < len(os.Args) {
			var v int
			fmt.Sscanf(os.Args[i+1], "%d", &v)
			return v
		}
	}
	return fallback
}

func argString(flag, fallback string) string {
	for i, a := range os.Args {
		if a == flag && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
		if strings.HasPrefix(a, flag+"=") {
			return strings.TrimPrefix(a, flag+"=")
		}
	}
	return fallback
}

func containsName(names []string, expectedName string) bool {
	for _, name := range names {
		if name == expectedName {
			return true
		}
	}
	return false
}

func appendMissingName(names []string, name string) []string {
	if containsName(names, name) {
		return names
	}
	return append(names, name)
}

func resolveSetupSSHCredentials(boardType string, requestedUser string, requestedPassword string) (string, string) {
	if boardType != setup.BoardJetsonOrinNano {
		if requestedUser == "" {
			return boardUser, requestedPassword
		}
		return requestedUser, requestedPassword
	}
	if requestedUser == "" {
		requestedUser = jetsonDefaultUser
	}
	if requestedPassword == "" {
		requestedPassword = os.Getenv("INTERNKIM_CONSOLE_PASSWORD")
	}
	if requestedPassword == "" {
		if data, err := os.ReadFile(".local/secrets/console-password"); err == nil {
			requestedPassword = strings.TrimSpace(string(data))
		}
	}
	return requestedUser, requestedPassword
}

// --- Setup pipeline wiring (selective re-run with SSH / SD backends) ---

type sshBoardConnection struct {
	client *sshClient
}

func (connection sshBoardConnection) Run(command string) string {
	return connection.client.run(command)
}

func (connection sshBoardConnection) SCP(localPath, remotePath string) error {
	return connection.client.scp(localPath, remotePath)
}

type sdStagingTarget struct {
	stagingRoot string
}

func (target sdStagingTarget) RootPath() string {
	return target.stagingRoot
}

func (target sdStagingTarget) WriteFile(stagePath string, data []byte, mode int) error {
	fullPath := filepath.Join(target.stagingRoot, stagePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, data, os.FileMode(mode))
}

func findSDStagingRoot() string {
	candidates := []string{
		"/Volumes/RPICFG", "/Volumes/bootfs", "/Volumes/boot",
		"/Volumes/NO NAME", "/Volumes/RASPIFIRM",
	}
	for _, volume := range candidates {
		stagingDirectory := filepath.Join(volume, "internkim")
		if info, err := os.Stat(stagingDirectory); err == nil && info.IsDir() {
			return stagingDirectory
		}
	}
	return ""
}

func resolveCloudflareSSHHostname(configuration config, target commandTarget) string {
	if target.useRemoteSSH && strings.TrimSpace(target.host) != "" {
		return strings.TrimSpace(target.host)
	}
	if strings.TrimSpace(target.sshHostname) != "" && !isLegacyCloudflareSSHHostname(configuration, target.sshHostname) {
		return strings.TrimSpace(target.sshHostname)
	}
	if hostname := cloudflareSSHHostnameFromDeviceURL(target.deviceURL); hostname != "" {
		return hostname
	}
	fleetID := loadState(target.stateDir, "fleet_id")
	if strings.TrimSpace(fleetID) == "" {
		return ""
	}
	if strings.TrimSpace(target.nodeID) != "" {
		return cloudflareNodeSSHHostname(configuration, strings.TrimSpace(fleetID), strings.TrimSpace(target.nodeID))
	}
	return cloudflareSSHHostname(configuration, strings.TrimSpace(fleetID))
}

func ensureCloudflareSSHRegistration(configuration config, stateDir string, force bool) (string, error) {
	savedSSHHostname := loadState(stateDir, "ssh_hostname")
	if canReuseCloudflareSSHRegistration(configuration, stateDir, savedSSHHostname, force) {
		return savedSSHHostname, nil
	}
	fleetID := loadState(stateDir, "fleet_id")
	nodeID := loadNodeID(stateDir)
	nodeKey := loadOrCreateNodeKey(stateDir)
	fleetSecret := loadState(stateDir, "fleet_secret")
	if fleetID == "" || fleetSecret == "" {
		return "", errors.New("Cloudflare SSH requires an already registered fleet; run setup once on the local network first")
	}
	if strings.TrimSpace(configuration.RegisterSecret) == "" {
		return "", errors.New("Cloudflare SSH migration requires INTERNKIM_REGISTER_SECRET")
	}
	response, errorValue := registerFleetNode(configuration, fleetID, nodeID, nodeKey, fleetSecret, remoteSetupAdminEmail(stateDir))
	if errorValue != nil {
		return "", errorValue
	}
	sshHostname := response.SSHHostname
	if sshHostname == "" {
		sshHostname = cloudflareSSHHostnameFromDeviceURL(response.publicURL())
	}
	response.SSHHostname = sshHostname
	saveRegistrationResponse(stateDir, response)
	saveState(stateDir, "ssh_hostname", sshHostname)
	saveDefaultFleetNode(stateDir, response)
	return sshHostname, nil
}

func canReuseCloudflareSSHRegistration(configuration config, stateDir string, savedSSHHostname string, force bool) bool {
	return savedSSHHostname != "" &&
		loadState(stateDir, "node_tunnel_token") != "" &&
		loadState(stateDir, "tunnel_revision") == setup.TunnelConfigurationRevision &&
		cloudflareSSHTLSIsReady(cloudflareSSHTLSStatus(stateDir)) &&
		!isLegacyCloudflareSSHHostname(configuration, savedSSHHostname)
}

func updateRemoteDeviceRegistration(connection *sshClient, configuration config, response *registerResponse) {
	nodeTunnelToken := firstNonEmptyString(response.NodeTunnelToken, response.TunnelToken)
	connection.run(fmt.Sprintf(`mkdir -p /root/.internkim/secrets /root/.internkim/env
printf '%%s' %s > /root/.internkim/env/fleet-id
printf '%%s' %s > /root/.internkim/env/device-url
printf '%%s' %s > /root/.internkim/env/mattermost-url
printf '%%s' %s > /root/.internkim/secrets/tunnel-token
printf '%%s' %s > /root/.internkim/secrets/node-tunnel-token
printf '%%s' %s > /root/.internkim/env/tunnel-origin
printf '%%s' %s > /root/.internkim/env/tunnel-revision
printf '%%s' %s > /root/.internkim/env/api-url
printf '%%s' %s > /root/.internkim/env/tls-certificate-status
chown root:root /root/.internkim/secrets/tunnel-token /root/.internkim/secrets/node-tunnel-token
chmod 600 /root/.internkim/secrets/tunnel-token /root/.internkim/secrets/node-tunnel-token
chown root:blueclaw /root/.internkim/env/fleet-id /root/.internkim/env/device-url /root/.internkim/env/mattermost-url /root/.internkim/env/tunnel-origin /root/.internkim/env/tunnel-revision /root/.internkim/env/api-url /root/.internkim/env/tls-certificate-status
chmod 640 /root/.internkim/env/fleet-id /root/.internkim/env/device-url /root/.internkim/env/mattermost-url /root/.internkim/env/tunnel-origin /root/.internkim/env/tunnel-revision /root/.internkim/env/api-url /root/.internkim/env/tls-certificate-status
cat > /etc/systemd/system/cloudflared.service <<'SVCEOF'
[Unit]
Description=Cloudflare Tunnel
After=network-online.target time-sync.target
Wants=network-online.target time-sync.target

[Service]
Type=simple
ExecStart=/bin/sh -c '/usr/local/bin/cloudflared tunnel run --protocol quic --token "$(cat /root/.internkim/secrets/tunnel-token)"'
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SVCEOF
cat > /etc/systemd/system/cloudflared-node-ssh.service <<'SVCEOF'
[Unit]
Description=Cloudflare Node SSH Tunnel
After=network-online.target time-sync.target
Wants=network-online.target time-sync.target

[Service]
Type=simple
ExecStart=/bin/sh -c '/usr/local/bin/cloudflared tunnel run --protocol quic --token "$(cat /root/.internkim/secrets/node-tunnel-token)"'
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SVCEOF
rm -f /etc/init.d/S98cloudflared 2>/dev/null
systemctl daemon-reload
systemctl enable cloudflared cloudflared-node-ssh
systemctl restart cloudflared cloudflared-node-ssh`,
		quoteShellValue(response.registeredFleetID()),
		quoteShellValue(response.publicURL()),
		quoteShellValue(response.publicURL()),
		quoteShellValue(response.TunnelToken),
		quoteShellValue(nodeTunnelToken),
		quoteShellValue(setup.MattermostTunnelOrigin),
		quoteShellValue(setup.TunnelConfigurationRevision),
		quoteShellValue(configuration.APIBaseURL),
		quoteShellValue(response.TLSStatus),
	))
}

func saveDefaultFleetNode(stateDir string, response *registerResponse) {
	nodeID := setupNodeIdentityName(response.registeredNodeID())
	if nodeID == "" || strings.TrimSpace(response.FleetRole) != "active" {
		return
	}
	if filepath.Base(filepath.Dir(stateDir)) != "boards" {
		return
	}
	baseStateDir := filepath.Dir(filepath.Dir(stateDir))
	saveState(baseStateDir, "default_node_id", nodeID)
}

func cloudflareSSHHostname(configuration config, fleetID string) string {
	cfDomain := strings.TrimSpace(configuration.CFDomain)
	if cfDomain == "" || strings.TrimSpace(fleetID) == "" {
		return ""
	}
	return "ssh-" + strings.TrimSpace(fleetID) + "." + cfDomain
}

func cloudflareNodeSSHHostname(configuration config, fleetID string, nodeID string) string {
	cfDomain := strings.TrimSpace(configuration.CFDomain)
	if cfDomain == "" || strings.TrimSpace(fleetID) == "" || strings.TrimSpace(nodeID) == "" {
		return ""
	}
	return strings.TrimSpace(nodeID) + ".ssh." + strings.TrimSpace(fleetID) + "." + cfDomain
}

func isLegacyCloudflareSSHHostname(configuration config, hostname string) bool {
	cfDomain := strings.TrimSpace(configuration.CFDomain)
	trimmedHostname := strings.TrimSpace(hostname)
	if cfDomain == "" || trimmedHostname == "" {
		return false
	}
	withoutDomain, found := strings.CutSuffix(trimmedHostname, "."+cfDomain)
	if !found {
		return false
	}
	if strings.HasPrefix(trimmedHostname, "ssh.") && strings.Count(withoutDomain, ".") == 1 {
		return true
	}
	if strings.HasPrefix(withoutDomain, "ssh-") && strings.Count(withoutDomain, ".") > 0 {
		return true
	}
	return strings.HasPrefix(withoutDomain, "ssh-") && strings.Count(withoutDomain, "-") > 1
}

func remoteSetupAdminEmail(stateDir string) string {
	for _, value := range []string{
		strings.TrimSpace(os.Getenv("INTERNKIM_ADMIN_EMAIL")),
		loadState(stateDir, "claimed_admin_email"),
		loadState(stateDir, "google_email"),
	} {
		if value != "" {
			return value
		}
	}
	return ""
}

func cloudflareSSHHostnameFromDeviceURL(deviceURL string) string {
	parsedURL, errorValue := url.Parse(strings.TrimSpace(deviceURL))
	if errorValue != nil || strings.TrimSpace(parsedURL.Hostname()) == "" {
		return ""
	}
	hostname := strings.TrimSpace(parsedURL.Hostname())
	labels := strings.SplitN(hostname, ".", 2)
	if len(labels) != 2 || labels[0] == "" || labels[1] == "" {
		return ""
	}
	return "ssh-" + labels[0] + "." + labels[1]
}

func ensureCloudflaredAccessSSHAvailable() error {
	if _, errorValue := exec.LookPath("cloudflared"); errorValue == nil {
		return nil
	}
	return errors.New("cloudflared is required for Cloudflare SSH fallback; install it locally, then rerun setup")
}

func isBlueclawPayloadDirectOnlySetup(arguments []string) bool {
	onlyNames := setup.ParseNames(commandArgumentValue(arguments, "--only", ""))
	return len(onlyNames) == 1 && onlyNames[0] == "blueclaw-payload-direct"
}

type setupLiveOptions struct {
	target                 commandTarget
	scriptDir              string
	sshpassBin             string
	setupBuildID           string
	requestedSSH           bool
	requestedSD            bool
	requestedCloudflareSSH bool
	nonInteractive         bool
	canRunWithoutSSH       bool
	cloudflareSSHHostname  string
}

type setupLiveRequest struct {
	requestedSSH           bool
	requestedSD            bool
	requestedCloudflareSSH bool
}

type setupBackendSelection struct {
	backend       setup.Backend
	target        commandTarget
	sshConnection *sshClient
	stagingRoot   string
	boardIP       string
}

func runSetupLive(messenger *msg) {
	configuration := loadConfig()
	scriptDir, _ := os.Getwd()
	setupBuildID := currentExecutableFingerprint()
	request := resolveSetupLiveRequest()
	if containsArg("--sim") {
		runSetupSimulation(setupControlArguments(os.Args[2:]))
		return
	}
	options := resolveSetupLiveOptions(configuration, scriptDir, setupBuildID, request)
	releaseSetupLock := acquireSetupLiveLock(options)
	defer releaseSetupLock()
	if runBlueclawPayloadDirectSetup(messenger, configuration, options) {
		return
	}
	backendSelection := resolveSetupBackend(messenger, configuration, options)
	printSetupBackendSelection(backendSelection)
	flowState := prepareSetupFlowState(messenger, configuration, options, backendSelection.sshConnection)
	pipelineContext := prepareSetupPipelineContext(messenger, options, backendSelection, flowState)
	selector := prepareSetupSelector(options.target.boardType, pipelineContext.Force)
	registry := setupRegistryForBoard(options.target.boardType)
	if errorValue := registry.Run(pipelineContext, selector); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func resolveSetupLiveRequest() setupLiveRequest {
	requestedSSH := containsArg("--ssh")
	requestedSD := containsArg("--sd")
	requestedCloudflareSSH := containsArg("--cloudflare-ssh")
	if requestedCloudflareSSH {
		requestedSSH = true
	}
	if requestedSSH && requestedSD {
		fatal("--ssh/--cloudflare-ssh and --sd are mutually exclusive")
	}
	return setupLiveRequest{
		requestedSSH:           requestedSSH,
		requestedSD:            requestedSD,
		requestedCloudflareSSH: requestedCloudflareSSH,
	}
}

func resolveSetupLiveOptions(configuration config, scriptDir string, setupBuildID string, request setupLiveRequest) setupLiveOptions {
	target := resolveCommandTarget(os.Args[2:])
	target = resolveLabHostForCommandTarget(target, scriptDir)
	if target.boardType == setup.BoardJetsonOrinNano {
		request.requestedSSH = true
	}
	return setupLiveOptions{
		target:                 target,
		scriptDir:              scriptDir,
		sshpassBin:             filepath.Join(scriptDir, "bin", "sshpass"),
		setupBuildID:           setupBuildID,
		requestedSSH:           request.requestedSSH,
		requestedSD:            request.requestedSD,
		requestedCloudflareSSH: request.requestedCloudflareSSH,
		nonInteractive:         containsArg("--non-interactive"),
		canRunWithoutSSH:       setupCanRunWithoutSSH(os.Args[2:]),
		cloudflareSSHHostname:  resolveCloudflareSSHHostname(configuration, target),
	}
}

func acquireSetupLiveLock(options setupLiveOptions) func() {
	if containsArg("--plan") {
		return func() {}
	}
	lockDocument := setupLockDocument{
		Command:       commandLineForSetupLock(os.Args[2:]),
		SelectedSteps: setupLockSelectedSteps(os.Args[2:]),
		TargetHost:    firstNonEmptyString(options.target.host, options.cloudflareSSHHostname, loadState(options.target.stateDir, "ssh_hostname")),
		TargetURL:     options.target.deviceURL,
	}
	lockHandle, errorValue := acquireSetupLock(options.target.stateDir, lockDocument, containsArg("--wait-lock"))
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	return lockHandle.release
}

func runBlueclawPayloadDirectSetup(messenger *msg, configuration config, options setupLiveOptions) bool {
	if !isBlueclawPayloadDirectOnlySetup(os.Args[2:]) {
		return false
	}
	target := options.target
	target.useRemoteSSH = false
	printCommandTargetEvidence(target)
	fmt.Printf("Backend: http maintenance\n")
	if containsArg("--plan") {
		fmt.Printf("  blueclaw-payload-direct run\n")
		return true
	}
	flowState := newSetupFlowState(
		messenger,
		configuration,
		collectSetupParameterValues(),
		options.target.stateDir,
		options.scriptDir,
		options.setupBuildID,
		nil,
		options.nonInteractive,
	)
	if errorValue := flowState.installBlueclawPayloadDirectHTTPS(); errorValue != nil {
		fatal(errorValue.Error())
	}
	return true
}

func resolveSetupBackend(messenger *msg, configuration config, options setupLiveOptions) setupBackendSelection {
	if containsArg("--plan") {
		return resolveSetupPlanBackend(options)
	}
	if options.canRunWithoutSSH {
		return resolveSetupWithoutSSHBackend(options)
	}
	if options.requestedSSH {
		return resolveRequestedSetupSSHBackend(messenger, configuration, options)
	}
	if options.requestedSD {
		return resolveRequestedSetupSDBackend(messenger, options)
	}
	return resolveAutomaticSetupBackend(messenger, configuration, options)
}

func resolveSetupPlanBackend(options setupLiveOptions) setupBackendSelection {
	selection := setupBackendSelection{target: options.target}
	if options.requestedSD {
		selection.backend = setup.BackendSD
		selection.stagingRoot = findSDStagingRoot()
		return selection
	}
	selection.backend = setup.BackendSSH
	selection.boardIP = firstNonEmptyString(options.target.host, options.cloudflareSSHHostname, loadState(options.target.stateDir, "ssh_hostname"))
	return selection
}

func resolveSetupWithoutSSHBackend(options setupLiveOptions) setupBackendSelection {
	return setupBackendSelection{
		backend: setup.BackendSSH,
		target:  options.target,
		boardIP: firstNonEmptyString(options.target.host, options.cloudflareSSHHostname, loadState(options.target.stateDir, "ssh_hostname")),
	}
}

func resolveRequestedSetupSSHBackend(messenger *msg, configuration config, options setupLiveOptions) setupBackendSelection {
	var selection setupBackendSelection
	var cloudflareSSHError error
	var sshReady bool
	if options.requestedCloudflareSSH {
		selection, cloudflareSSHError, sshReady = attemptSetupCloudflareSSH(configuration, options)
	} else if selection, sshReady = attemptSetupBackend(options, setup.BackendSSH); !sshReady {
		selection, cloudflareSSHError, sshReady = attemptSetupCloudflareSSH(configuration, options)
	}
	if sshReady {
		return selection
	}
	if cloudflareSSHError != nil {
		fatal(cloudflareSSHError.Error())
	}
	if options.target.boardType == setup.BoardJetsonOrinNano {
		fatalJetsonSSHFailure(messenger, options)
	}
	fatal(messenger.t("보드를 찾을 수 없습니다 (SSH).", "Board not reachable (SSH)."))
	return selection
}

func fatalJetsonSSHFailure(messenger *msg, options setupLiveOptions) {
	failureDetails := describeJetsonSSHFailure(options.sshpassBin, options.target.stateDir, options.target.sshUser, options.target.sshPassword)
	fatal(messenger.t(
		"Jetson을 SSH로 찾을 수 없습니다.\n"+failureDetails+"\nJetson 콘솔에서 `ip addr`, `nmcli device status`, `systemctl status ssh --no-pager`, `systemctl status internkim-wifi-recovery.timer --no-pager`, `journalctl -u internkim-wifi-recovery.service -n 80 --no-pager`, `tail /var/log/internkim-jetson-firstboot.log`를 확인하세요. IP를 알면 --host <ip>를 지정하면 됩니다.",
		"Jetson was not found over SSH.\n"+failureDetails+"\nOn the Jetson console, check `ip addr`, `nmcli device status`, `systemctl status ssh --no-pager`, `systemctl status internkim-wifi-recovery.timer --no-pager`, `journalctl -u internkim-wifi-recovery.service -n 80 --no-pager`, and `tail /var/log/internkim-jetson-firstboot.log`. If you know the IP, pass --host <ip>.",
	))
}

func resolveRequestedSetupSDBackend(messenger *msg, options setupLiveOptions) setupBackendSelection {
	selection, ok := attemptSetupBackend(options, setup.BackendSD)
	if ok {
		return selection
	}
	fatal(messenger.t("SD 카드가 꽂혀있지 않거나 internkim 디렉토리가 없습니다.", "No SD card mounted with an internkim staging dir."))
	return selection
}

func resolveAutomaticSetupBackend(messenger *msg, configuration config, options setupLiveOptions) setupBackendSelection {
	if selection, ok := attemptSetupBackend(options, setup.BackendSSH); ok {
		return selection
	}
	selection, cloudflareSSHError, cloudflareSSHReady := attemptSetupCloudflareSSH(configuration, options)
	if cloudflareSSHReady {
		return selection
	}
	if selection, ok := attemptSetupBackend(options, setup.BackendSD); ok {
		return selection
	}
	if cloudflareSSHError != nil {
		fatal(cloudflareSSHError.Error())
	}
	fatal(messenger.t(
		"타겟을 찾을 수 없습니다 — 보드에 SSH도 안 되고, SD 카드도 없습니다.\n  --ssh 또는 --sd 를 명시하거나, 대상을 준비해 주세요.",
		"No target — board unreachable via SSH and no SD mounted.\n  Pass --ssh or --sd explicitly, or prepare a target.",
	))
	return setupBackendSelection{target: options.target}
}

func attemptSetupBackend(options setupLiveOptions, backend setup.Backend) (setupBackendSelection, bool) {
	selection := setupBackendSelection{backend: backend, target: options.target}
	pendingError := errors.New("setup backend pending")
	errorValue := retryOperation(retryOptions{
		AttemptCount: 3,
		DelayForAttempt: func(attemptIndex int) time.Duration {
			return 2 * time.Second
		},
	}, func(attemptIndex int) error {
		if setupBackendIsReady(options, &selection) {
			return nil
		}
		return pendingError
	})
	return selection, errorValue == nil
}

func setupBackendIsReady(options setupLiveOptions, selection *setupBackendSelection) bool {
	switch selection.backend {
	case setup.BackendSSH:
		return setupSSHBackendIsReady(options, selection)
	case setup.BackendSD:
		selection.stagingRoot = findSDStagingRoot()
		return selection.stagingRoot != ""
	default:
		return false
	}
}

func setupSSHBackendIsReady(options setupLiveOptions, selection *setupBackendSelection) bool {
	if options.target.host != "" {
		selection.boardIP = options.target.host
		candidateConnection := newSSH(options.sshpassBin, options.target.sshUser, options.target.sshPassword, selection.boardIP)
		if _, errorValue := candidateConnection.runResult("true"); errorValue != nil {
			return false
		}
		selection.sshConnection = candidateConnection
		return true
	}
	selection.boardIP = findBoardIPForCredentials(options.sshpassBin, options.target.stateDir, options.target.sshUser, options.target.sshPassword)
	if selection.boardIP == "" {
		return false
	}
	selection.sshConnection = newSSH(options.sshpassBin, options.target.sshUser, options.target.sshPassword, selection.boardIP)
	return true
}

func attemptSetupCloudflareSSH(configuration config, options setupLiveOptions) (setupBackendSelection, error, bool) {
	selection := setupBackendSelection{backend: setup.BackendSSH, target: options.target}
	cloudflareSSHHostname := options.cloudflareSSHHostname
	if hostname, errorValue := ensureCloudflareSSHRegistration(configuration, options.target.stateDir, containsArg("--force") || containsArg("--force-all")); errorValue == nil && hostname != "" {
		cloudflareSSHHostname = hostname
	} else if options.requestedCloudflareSSH && errorValue != nil {
		return selection, errorValue, false
	}
	if cloudflareSSHHostname == "" {
		return selection, nil, false
	}
	if errorValue := ensureCloudflaredAccessSSHAvailable(); errorValue != nil {
		return selection, errorValue, false
	}
	selection.boardIP = cloudflareSSHHostname
	selection.sshConnection = newCloudflareSSH(options.sshpassBin, options.target.sshUser, options.target.sshPassword, selection.boardIP)
	output, errorValue := selection.sshConnection.runResult("true")
	if errorValue != nil {
		return selection, formatCloudflareSSHError(selection.boardIP, output, errorValue), false
	}
	selection.target.useRemoteSSH = true
	return selection, nil, true
}

func printSetupBackendSelection(selection setupBackendSelection) {
	target := selection.target
	switch selection.backend {
	case setup.BackendSSH:
		target.host = selection.boardIP
		printCommandTargetEvidence(target)
		fmt.Printf("Backend: ssh\n")
	case setup.BackendSD:
		printCommandTargetEvidence(target)
		fmt.Printf("Backend: sd staging\n")
		fmt.Printf("Staging: %s\n", selection.stagingRoot)
	}
}

func prepareSetupFlowState(messenger *msg, configuration config, options setupLiveOptions, sshConnection *sshClient) *setupFlowState {
	return newSetupFlowState(
		messenger,
		configuration,
		collectSetupParameterValues(),
		options.target.stateDir,
		options.scriptDir,
		options.setupBuildID,
		sshConnection,
		options.nonInteractive,
	)
}

func prepareSetupPipelineContext(messenger *msg, options setupLiveOptions, selection setupBackendSelection, flowState *setupFlowState) *setup.Context {
	pipelineContext := &setup.Context{
		Backend:      selection.backend,
		Language:     messenger.lang,
		StateDir:     options.target.stateDir,
		ScriptDir:    options.scriptDir,
		BoardType:    options.target.boardType,
		BoardIP:      selection.boardIP,
		PublicURL:    options.target.deviceURL,
		SetupCommand: commandLineForSetupLock(os.Args[2:]),
		SetupSteps:   setupLockSelectedSteps(os.Args[2:]),
		SetupLockID:  randomHexString(12),
		Force:        resolveSetupForce(options, selection),
		HTTP:         &http.Client{Timeout: 30 * time.Second},
		Callbacks:    flowState.callbacks(),
	}
	if selection.backend == setup.BackendSSH {
		if selection.sshConnection != nil {
			pipelineContext.SSH = sshBoardConnection{client: selection.sshConnection}
		}
		return pipelineContext
	}
	pipelineContext.SD = sdStagingTarget{stagingRoot: selection.stagingRoot}
	return pipelineContext
}

func resolveSetupForce(options setupLiveOptions, selection setupBackendSelection) bool {
	shouldForce := containsArg("--force") || containsArg("--force-all")
	if selection.backend != setup.BackendSD || options.setupBuildID == "" {
		return shouldForce
	}
	stageBuildIDPath := filepath.Join(selection.stagingRoot, "setup-build-id")
	stageBuildIDBytes, errorValue := os.ReadFile(stageBuildIDPath)
	if errorValue == nil && strings.TrimSpace(string(stageBuildIDBytes)) != options.setupBuildID {
		return true
	}
	return shouldForce
}

func prepareSetupSelector(boardType string, shouldForce bool) setup.Selector {
	selector := setup.Selector{
		Only:     setup.ParseNames(argString("--only", "")),
		From:     argString("--from", ""),
		Skip:     setup.ParseNames(argString("--skip", "")),
		Force:    shouldForce,
		ForceAll: containsArg("--force-all"),
		DryRun:   containsArg("--plan"),
	}
	return applySetupBoardDefaults(boardType, containsArg("--with-google"), selector)
}

func setupRegistryForBoard(boardType string) setup.Registry {
	if boardType == setup.BoardJetsonOrinNano {
		return setup.JetsonRegistry()
	}
	return setup.DefaultRegistry()
}

func setupCanRunWithoutSSH(arguments []string) bool {
	if strings.TrimSpace(commandArgumentValue(arguments, "--from", "")) != "" {
		return false
	}
	onlySteps := setup.ParseNames(commandArgumentValue(arguments, "--only", ""))
	if len(onlySteps) != 1 {
		return false
	}
	return onlySteps[0] == "blueclaw-payload-direct" || onlySteps[0] == "cloudflare-access"
}

func applySetupBoardDefaults(boardType string, withGoogle bool, selector setup.Selector) setup.Selector {
	if boardType == setup.BoardJetsonOrinNano && !withGoogle && !containsName(selector.Only, "google") {
		selector.Skip = appendMissingName(selector.Skip, "google")
	}
	if boardType == setup.BoardCloudShared {
		for _, name := range []string{"wifi", "local-llm", "google"} {
			if !containsName(selector.Only, name) {
				selector.Skip = appendMissingName(selector.Skip, name)
			}
		}
	}
	return selector
}

func collectSetupParameterValues() setupParameterValues {
	return setupParameterValues{
		AdminEmail:        strings.TrimSpace(argString("--admin-email", "")),
		OpenRouterAPIKey:  strings.TrimSpace(argString("--openrouter-api-key", "")),
		LiteRTModelPath:   strings.TrimSpace(argString("--litert-model-path", "")),
		GasWebhookURL:     strings.TrimSpace(argString("--gas-webhook-url", "")),
		GoogleAccessToken: strings.TrimSpace(argString("--google-access-token", "")),
		SlackBotToken:     strings.TrimSpace(argString("--slack-bot-token", "")),
		SlackAppToken:     strings.TrimSpace(argString("--slack-app-token", "")),
		SignalJSONRPCURL:  strings.TrimSpace(argString("--signal-jsonrpc-url", "")),
		SignalAccount:     strings.TrimSpace(argString("--signal-account", "")),
	}
}

func buildOpenRouterKeyCallback(stateDir string, messenger *msg, openRouterAPIKey string, nonInteractive bool) func(force bool) (string, error) {
	return func(force bool) (string, error) {
		if openRouterAPIKey != "" {
			return openRouterAPIKey, nil
		}
		if envKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")); envKey != "" {
			return envKey, nil
		}
		if savedKey := loadState(stateDir, "openrouter_api_key"); !force && savedKey != "" {
			return savedKey, nil
		}
		if nonInteractive {
			return "", fmt.Errorf("openrouter API key is empty; pass --openrouter-api-key, set OPENROUTER_API_KEY, or run interactive setup once")
		}
		promptedKey := strings.TrimSpace(readLine(messenger.t("  OpenRouter API 키: ", "  OpenRouter API key: ")))
		if promptedKey != "" {
			saveState(stateDir, "openrouter_api_key", promptedKey)
		}
		return promptedKey, nil
	}
}

func buildLiteRTModelPathCallback(liteRTModelPath string) func(force bool) (string, error) {
	return func(force bool) (string, error) {
		_ = force
		if liteRTModelPath != "" {
			return liteRTModelPath, nil
		}
		if envPath := strings.TrimSpace(os.Getenv("INTERNKIM_LITERT_MODEL_PATH")); envPath != "" {
			return envPath, nil
		}
		return ensureCachedLiteRTModel()
	}
}

func ensureCachedLiteRTModel() (string, error) {
	repositoryRoot := repositoryRootForDependencyCache()
	if repositoryRoot == "" {
		return "", nil
	}
	cacheRelative := locallm.LiteRTCacheModelDir
	filename := locallm.LiteRTModelFilename
	sourceURL := locallm.LiteRTModelURL
	displayName := "LiteRT"
	if locallm.Default == locallm.BackendLlamaCpp {
		cacheRelative = locallm.LlamaCppCacheModelDir
		filename = locallm.LlamaCppModelFilename
		sourceURL = locallm.LlamaCppModelURL
		displayName = "llama.cpp"
	}
	cacheDirectory := filepath.Join(repositoryRoot, cacheRelative)
	cachedModelPath := filepath.Join(cacheDirectory, filename)
	if fileInfo, errorValue := os.Stat(cachedModelPath); errorValue == nil && fileInfo.Size() > 0 {
		return cachedModelPath, nil
	}
	if errorValue := os.MkdirAll(cacheDirectory, 0o755); errorValue != nil {
		return "", errorValue
	}
	temporaryPath := cachedModelPath + ".tmp"
	_ = os.Remove(temporaryPath)
	fmt.Printf("  downloading %s model into %s\n", displayName, cachedModelPath)
	command := exec.Command("curl", "-fL", "--retry", "3", "--progress-bar", "-o", temporaryPath, sourceURL)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if errorValue := command.Run(); errorValue != nil {
		_ = os.Remove(temporaryPath)
		return "", errorValue
	}
	if errorValue := os.Rename(temporaryPath, cachedModelPath); errorValue != nil {
		return "", errorValue
	}
	return cachedModelPath, nil
}
