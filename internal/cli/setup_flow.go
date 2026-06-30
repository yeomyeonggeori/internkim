package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/blueclawworkspace"
	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	setup "gitlab.com/eastriver/internkim/internal/provisioning/steps"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
)

type setupFlowState struct {
	messenger      *msg
	configuration  config
	parameters     setupParameterValues
	stateDir       string
	scriptDir      string
	boardBinDir    string
	getSSIDPath    string
	setupBuildID   string
	sshClient      *sshClient
	publicKey      string
	nonInteractive bool

	wifiResolved bool
	wifiSSID     string
	wifiPassword string
	wifiProfiles []resolvedWiFiProfile
	wifiChanged  bool

	registrationResolved bool
	fleetID              string
	nodeID               string
	adminEmail           string
	deviceURL            string
	tunnelToken          string
	nodeTunnelToken      string
	tlsCertificateStatus string
	fleetRole            string
	fleetActiveCount     int
	fleetPendingCount    int
	fleetQuorumSize      int
}

type setupParameterValues struct {
	AdminEmail        string
	OpenRouterAPIKey  string
	LiteRTModelPath   string
	GasWebhookURL     string
	GoogleAccessToken string
	SlackBotToken     string
	SlackAppToken     string
	SignalJSONRPCURL  string
	SignalAccount     string
}

type localBinaryAsset struct {
	name         string
	localPath    string
	remotePath   string
	downloadURL  string
	archiveEntry string
}

func installSkillPythonDependenciesCommand() string {
	return `set -euo pipefail
contract_output="$(
` + setup.BlueclawRootfsBaseContractCheckCommand() + `
)"
if [ "$contract_output" != "ok" ]; then
  printf '%s\n' "$contract_output"
  exit 1
fi`
}

func deviceBrowserRuntimePackageListUbuntu24() string {
	return strings.Join(baseDeviceToolPackages(), " ")
}

func deviceBrowserRuntimePackageListLegacyUbuntu() string {
	return strings.Join(baseDeviceToolPackages(), " ")
}

func deviceBrowserRuntimePackageListJetPack6() string {
	return deviceBrowserRuntimePackageListLegacyUbuntu()
}

func deviceBrowserRuntimePackageSelectionScript() string {
	return fmt.Sprintf(`runtimePackages=%s
if [ -f /etc/os-release ]; then
  . /etc/os-release
  case "${VERSION_ID:-}" in
    22.*) runtimePackages=%s ;;
    24.*|25.*|26.*) runtimePackages=%s ;;
    *) runtimePackages=%s ;;
  esac
fi`,
		quoteShellValue(deviceBrowserRuntimePackageListLegacyUbuntu()),
		quoteShellValue(deviceBrowserRuntimePackageListJetPack6()),
		quoteShellValue(deviceBrowserRuntimePackageListUbuntu24()),
		quoteShellValue(deviceBrowserRuntimePackageListLegacyUbuntu()),
	)
}

func deviceBrowserRuntimeDependencyInstallScript() string {
	return strings.TrimSpace(`apt-get update -qq >/dev/null 2>&1 || true
` + deviceBrowserRuntimePackageSelectionScript() + `
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq $runtimePackages >/dev/null 2>&1 || true
if ! command -v git >/dev/null 2>&1; then
  apt-get update -qq >/dev/null 2>&1 || true
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq $runtimePackages >/dev/null 2>&1 || true
fi
command -v git >/dev/null 2>&1`)
}

func baseDeviceToolPackages() []string {
	return []string{"bc", "ca-certificates", "curl", "git", "iproute2", "iptables", "procps", "unzip"}
}

func usersSyncDependencyInstallScript() string {
	return strings.TrimSpace(`if ! command -v jq >/dev/null 2>&1 || ! command -v curl >/dev/null 2>&1; then
  apt-get update -qq >/dev/null 2>&1 || true
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq jq curl ca-certificates >/dev/null
fi
command -v jq >/dev/null 2>&1
command -v curl >/dev/null 2>&1`)
}

func newSetupFlowState(
	messenger *msg,
	configuration config,
	parameters setupParameterValues,
	stateDir string,
	scriptDir string,
	setupBuildID string,
	sshClient *sshClient,
	nonInteractive bool,
) *setupFlowState {
	return &setupFlowState{
		messenger:      messenger,
		configuration:  configuration,
		parameters:     parameters,
		stateDir:       stateDir,
		scriptDir:      scriptDir,
		boardBinDir:    filepath.Join(scriptDir, "build", "board-bin"),
		fleetID:        loadOrCreateFleetID(stateDir),
		nodeID:         loadNodeID(stateDir),
		getSSIDPath:    filepath.Join(scriptDir, "bin", "get-ssid"),
		setupBuildID:   setupBuildID,
		sshClient:      sshClient,
		publicKey:      getLocalSSHPubKey(),
		nonInteractive: nonInteractive,
	}
}

func (state *setupFlowState) callbacks() setup.Callbacks {
	return setup.Callbacks{
		Translate:                 func(korean, english string) string { return state.messenger.t(korean, english) },
		LoadState:                 func(key string) string { return loadState(state.stateDir, key) },
		SaveState:                 func(key, value string) { saveState(state.stateDir, key, value) },
		GetOpenRouterKey:          buildOpenRouterKeyCallback(state.stateDir, state.messenger, state.parameters.OpenRouterAPIKey, state.nonInteractive),
		GetLiteRTModelPath:        buildLiteRTModelPathCallback(state.parameters.LiteRTModelPath),
		GetGasWebhookURL:          state.provisionGasWebhook,
		BinariesVersion:           state.binariesVersion,
		InstallBinariesSSH:        state.installBinariesSSH,
		InstallAdmindSSH:          state.installAdmindSSH,
		InstallCapabilitydSSH:     state.installCapabilitydSSH,
		StageBinariesSD:           state.stageBinariesSD,
		SkillsManifest:            state.skillsManifest,
		InstallSkillsSSH:          state.installSkillsSSH,
		StageSkillsSD:             state.stageSkillsSD,
		BlueclawRuntimeManifest:   state.blueclawRuntimeManifest,
		InstallBlueclawRuntimeSSH: state.installBlueclawRuntimeSSH,
		BlueclawPayloadManifest:   state.blueclawPayloadManifest,
		InstallBlueclawPayloadSSH: state.installBlueclawPayloadSSH,
		AdminWebVersion:           state.adminWebVersion,
		DeployAdminWeb:            state.deployAdminWeb,
		SyncCloudflareAccess:      state.syncCloudflareAccess,
		ConfigureWifiSSH:          state.configureWifiSSH,
		StageWifiSD:               state.stageWifiSD,
		ProvisionTunnelSSH:        state.provisionTunnelSSH,
		StageTunnelSD:             state.stageTunnelSD,
		ConfigureSlackTokenSSH:    state.configureSlackTokenSSH,
		StageSlackTokenSD:         state.stageSlackTokenSD,
		InstallUsersSyncSSH:       state.installUsersSyncSSH,
		StageUsersSyncSD:          state.stageUsersSyncSD,
		StageBootstrapSD:          state.stageBootstrapSD,
		InstallMattermost: func(context *setup.Context) error {
			if state.sshClient == nil {
				return nil
			}
			return installMattermost(state.messenger, state.sshClient, context.Force)
		},
		SetupMattermost: func(context *setup.Context) error {
			if state.sshClient == nil {
				return nil
			}
			setupMattermost(state.messenger, state.sshClient, state.stateDir, context.Force)
			return nil
		},
	}
}

func (state *setupFlowState) provisionGasWebhook(accessToken string) (string, error) {
	if state.parameters.GasWebhookURL != "" {
		return state.parameters.GasWebhookURL, nil
	}
	if webhookURL := strings.TrimSpace(os.Getenv("INTERNKIM_GAS_WEBHOOK_URL")); webhookURL != "" {
		return webhookURL, nil
	}
	if webhookURL := strings.TrimSpace(os.Getenv("GAS_WEBHOOK_URL")); webhookURL != "" {
		return webhookURL, nil
	}
	_ = accessToken
	return provisionGasWebhook("")
}

func (state *setupFlowState) resolveSlackBotToken() string {
	if state.parameters.SlackBotToken != "" {
		return state.parameters.SlackBotToken
	}
	return strings.TrimSpace(os.Getenv("INTERNKIM_SLACK_BOT_TOKEN"))
}

func (state *setupFlowState) resolveSlackAppToken() string {
	if state.parameters.SlackAppToken != "" {
		return state.parameters.SlackAppToken
	}
	return strings.TrimSpace(os.Getenv("INTERNKIM_SLACK_APP_TOKEN"))
}

func (state *setupFlowState) resolveSignalJSONRPCURL() string {
	if state.parameters.SignalJSONRPCURL != "" {
		return state.parameters.SignalJSONRPCURL
	}
	return strings.TrimSpace(os.Getenv("INTERNKIM_SIGNAL_JSONRPC_URL"))
}

func (state *setupFlowState) resolveSignalAccount() string {
	if state.parameters.SignalAccount != "" {
		return state.parameters.SignalAccount
	}
	return strings.TrimSpace(os.Getenv("INTERNKIM_SIGNAL_ACCOUNT"))
}

func (state *setupFlowState) ensureWiFiCredentials() error {
	if state.wifiResolved {
		return nil
	}

	wifiProfiles, errorValue := resolveWiFiProfiles(state.messenger, state.stateDir, state.getSSIDPath)
	if errorValue != nil {
		return errorValue
	}
	if len(wifiProfiles) == 0 {
		return fmt.Errorf("wifi ssid is empty")
	}
	state.wifiProfiles = wifiProfiles
	state.wifiSSID = wifiProfiles[0].SSID
	state.wifiPassword = wifiProfiles[0].Password
	state.wifiResolved = true
	return nil
}

func (state *setupFlowState) configureWifiSSH(context *setup.Context) error {
	if context.BoardType == setup.BoardJetsonOrinNano {
		return state.configureJetsonWiFiSSH()
	}

	if err := state.ensureWiFiCredentials(); err != nil {
		return err
	}

	wpaSupplicant := buildBootWpaSupplicantConfig(state.wifiSSID, state.wifiPassword)
	state.sshClient.run(fmt.Sprintf(`killall wpa_supplicant 2>/dev/null || true
cat > /etc/wpa_supplicant.conf <<'WPAEOF'
%s
WPAEOF
wpa_supplicant -i wlan0 -c /etc/wpa_supplicant.conf -B
sleep 3
udhcpc -i wlan0 -q -n -t 5 -T 3 2>/dev/null || true`, wpaSupplicant))

	var wlanAddress string
	for attemptIndex := 0; attemptIndex < 3; attemptIndex++ {
		wlanAddress = strings.TrimSpace(state.sshClient.run(
			`ip -4 addr show wlan0 2>/dev/null | grep 'inet ' | awk '{print $2}' | cut -d/ -f1`,
		))
		if wlanAddress != "" {
			break
		}
		if attemptIndex < 2 {
			fmt.Printf("  %s (%d/3)\n", state.messenger.t("재시도 중...", "Retrying..."), attemptIndex+2)
			state.sshClient.run("udhcpc -i wlan0 -q -n -t 5 -T 3 2>/dev/null || true")
		}
	}

	if wlanAddress == "" {
		return fmt.Errorf("wifi connection failed")
	}

	fmt.Printf("  %s: %s\n", state.messenger.t("Wi-Fi 연결 성공", "Wi-Fi connected"), wlanAddress)
	return nil
}

func (state *setupFlowState) configureJetsonWiFiSSH() error {
	if err := state.ensureWiFiCredentials(); err != nil {
		return err
	}

	setupCommand := strings.Join([]string{
		"set -eu",
		buildJetsonWiFiInstallScript(state.wifiProfiles),
		strings.TrimSpace(`for attempt in $(seq 1 12); do
	  /usr/local/bin/internkim-wifi-select >/dev/null 2>&1 || true
	  address="$(ip -o -4 addr show scope global 2>/dev/null | awk '$2 ~ /^(wl|wlan)/ {print $4; exit}' | cut -d/ -f1)"
	  if [ -n "$address" ]; then
	    echo "$address"
	    exit 0
	  fi
	  sleep 5
	done
	exit 1`),
	}, "\n")

	output, errorValue := state.sshClient.runResult(setupCommand)
	if errorValue != nil {
		return fmt.Errorf("Jetson NetworkManager Wi-Fi connection failed: %s", strings.TrimSpace(output))
	}
	if address := lastNonEmptyLine(output); address != "" {
		fmt.Printf("  %s: %s\n", state.messenger.t("Wi-Fi 연결 성공", "Wi-Fi connected"), address)
	}
	return nil
}

func (state *setupFlowState) stageWifiSD(context *setup.Context) error {
	if err := state.ensureWiFiCredentials(); err != nil {
		return err
	}
	if err := context.SD.WriteFile(
		"wpa_supplicant.conf",
		[]byte(buildBootWpaSupplicantConfig(state.wifiSSID, state.wifiPassword)),
		0o644,
	); err != nil {
		return err
	}
	fmt.Printf("  %s: %s\n", state.messenger.t("Wi-Fi 설정 준비 완료", "Wi-Fi config staged"), state.wifiSSID)
	return nil
}

func buildBootWpaSupplicantConfig(ssid string, password string) string {
	if password == "" {
		return fmt.Sprintf(
			"ctrl_interface=/var/run/wpa_supplicant\nap_scan=1\nnetwork={\n  ssid=\"%s\"\n  scan_ssid=1\n  key_mgmt=NONE\n}\n",
			ssid,
		)
	}
	return fmt.Sprintf(
		"ctrl_interface=/var/run/wpa_supplicant\nap_scan=1\nnetwork={\n  ssid=\"%s\"\n  scan_ssid=1\n  key_mgmt=WPA-PSK\n  psk=\"%s\"\n}\n",
		ssid,
		password,
	)
}

func (state *setupFlowState) requiredBinaryAssets() []localBinaryAsset {
	return []localBinaryAsset{
		{
			name:       blueclaw.BlueclawName,
			localPath:  filepath.Join(state.boardBinDir, blueclaw.BlueclawName),
			remotePath: blueclaw.BlueclawBinaryPath,
		},
		{
			name:       blueclaw.BlueclawSupervisorName,
			localPath:  filepath.Join(state.boardBinDir, blueclaw.BlueclawSupervisorName),
			remotePath: blueclaw.BlueclawSupervisorBinaryPath,
		},
		{
			name:        "cloudflared",
			localPath:   filepath.Join(state.boardBinDir, "cloudflared"),
			remotePath:  "/usr/local/bin/cloudflared",
			downloadURL: "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64",
		},
		{
			name:         "pocketbase",
			localPath:    filepath.Join(state.boardBinDir, "pocketbase"),
			remotePath:   "/usr/local/bin/pocketbase",
			downloadURL:  "https://github.com/pocketbase/pocketbase/releases/download/v0.37.1/pocketbase_0.37.1_linux_arm64.zip",
			archiveEntry: "pocketbase",
		},
		{
			name:         "rtk",
			localPath:    filepath.Join(state.boardBinDir, "rtk"),
			remotePath:   "/usr/local/bin/rtk",
			downloadURL:  "https://github.com/rtk-ai/rtk/releases/latest/download/rtk-aarch64-unknown-linux-gnu.tar.gz",
			archiveEntry: "rtk",
		},
		{
			name:        "agent-browser",
			localPath:   filepath.Join(state.boardBinDir, "agent-browser"),
			remotePath:  "/usr/local/bin/agent-browser",
			downloadURL: "https://github.com/vercel-labs/agent-browser/releases/latest/download/agent-browser-linux-arm64",
		},
		{
			name:        "lightpanda",
			localPath:   filepath.Join(state.boardBinDir, "lightpanda"),
			remotePath:  browserruntime.DeviceBrowserExecutablePath,
			downloadURL: "https://github.com/lightpanda-io/browser/releases/download/nightly/lightpanda-aarch64-linux",
		},
		{
			name:       "download",
			localPath:  filepath.Join(state.boardBinDir, "download"),
			remotePath: "/usr/local/bin/download",
		},
		{
			name:       blueclaw.CapabilitydName,
			localPath:  filepath.Join(state.boardBinDir, blueclaw.CapabilitydName),
			remotePath: blueclaw.CapabilitydBinaryPath,
		},
		{
			name:       blueclaw.AdmindName,
			localPath:  filepath.Join(state.boardBinDir, blueclaw.AdmindName),
			remotePath: blueclaw.AdmindBinaryPath,
		},
		{
			name:       blueclaw.LocalLLMRunnerName,
			localPath:  filepath.Join(state.boardBinDir, blueclaw.LocalLLMRunnerName),
			remotePath: blueclaw.LocalLLMRunnerBinaryPath,
		},
	}
}

func (state *setupFlowState) ensureLocalBinaryAssets() ([]localBinaryAsset, error) {
	if err := os.MkdirAll(state.boardBinDir, 0o755); err != nil {
		return nil, err
	}

	assets := state.requiredBinaryAssets()

	for _, asset := range assets {
		if asset.name == blueclaw.BlueclawName {
			fmt.Printf("  %s %s... ", state.messenger.t("빌드 중", "Building"), asset.name)
			if err := blueclaw.EnsureBlueclawBinary(asset.localPath, state.scriptDir); err != nil {
				fmt.Println("FAILED")
				return nil, err
			}
			fmt.Println("ok")
			continue
		}
		if asset.name == blueclaw.BlueclawSupervisorName {
			fmt.Printf("  %s %s... ", state.messenger.t("빌드 중", "Building"), asset.name)
			if err := blueclaw.EnsureBlueclawSupervisorBinary(asset.localPath, state.scriptDir); err != nil {
				fmt.Println("FAILED")
				return nil, err
			}
			fmt.Println("ok")
			continue
		}

		if asset.name == blueclaw.CapabilitydName || asset.name == blueclaw.AdmindName || asset.name == blueclaw.LocalLLMRunnerName {
			if err := buildGoBinaryAsset(state, asset); err != nil {
				return nil, err
			}
			continue
		}

		if _, statError := os.Stat(asset.localPath); statError == nil {
			continue
		}

		switch asset.name {
		case "download":
			if err := buildGoBinaryAsset(state, asset); err != nil {
				return nil, err
			}
		default:
			fmt.Printf("  %s %s... ", state.messenger.t("다운로드 중", "Downloading"), asset.name)
			if downloadError := downloadBinary(asset.downloadURL, asset.localPath, asset.archiveEntry); downloadError != nil {
				fmt.Println("FAILED")
				return nil, fmt.Errorf("download %s: %w", asset.name, downloadError)
			}
			fmt.Println("ok")
		}
	}

	return assets, nil
}

func buildGoBinaryAsset(state *setupFlowState, asset localBinaryAsset) error {
	fmt.Printf("  %s %s... ", state.messenger.t("빌드 중", "Building"), asset.name)
	arguments := []string{"build", "-o", asset.localPath}
	if asset.name == blueclaw.AdmindName {
		arguments = append(arguments, "-ldflags", admindBuildFlags(state))
	}
	arguments = append(arguments, "./cmd/"+asset.name+"/")
	buildCommand := exec.Command("go", arguments...)
	buildCommand.Dir = state.scriptDir
	buildCommand.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64")
	if output, buildError := buildCommand.CombinedOutput(); buildError != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("build %s: %s", asset.name, strings.TrimSpace(string(output)))
	}
	fmt.Println("ok")
	return nil
}

func admindBuildFlags(state *setupFlowState) string {
	revision := strings.TrimSpace(runCmd("git", "-C", state.scriptDir, "rev-parse", "--short", "HEAD"))
	buildID := strings.TrimSpace(state.setupBuildID)
	if buildID == "" {
		buildID = "unknown"
	}
	if revision == "" {
		revision = "unknown"
	}
	return strings.Join([]string{
		"-X", "gitlab.com/eastriver/internkim/internal/admind.BuildID=" + buildID,
		"-X", "gitlab.com/eastriver/internkim/internal/admind.GitRevision=" + revision,
	}, " ")
}

func (state *setupFlowState) binariesVersion() string {
	hash := sha256.New()
	for _, path := range state.binaryVersionSourcePaths() {
		state.writePathHash(hash, path)
	}
	blueclawRevision := strings.TrimSpace(runCmd("git", "-C", blueclaw.BlueclawSubmoduleRoot(state.scriptDir), "rev-parse", "HEAD"))
	_, _ = hash.Write([]byte("blueclaw:" + blueclawRevision + "\n"))
	return hex.EncodeToString(hash.Sum(nil))
}

func (state *setupFlowState) binaryVersionSourcePaths() []string {
	paths := state.binaryGoDependencyDirectories(
		"./cmd/"+blueclaw.CapabilitydName,
		"./cmd/"+blueclaw.AdmindName,
		"./cmd/"+blueclaw.LocalLLMRunnerName,
	)
	if len(paths) == 0 {
		paths = fallbackBinaryVersionSourcePaths(state.scriptDir)
	}
	paths = append(paths,
		filepath.Join(blueclaw.BlueclawSubmoduleRoot(state.scriptDir), "tools", "graphiti_memoryd"),
		filepath.Join(blueclaw.BlueclawSubmoduleRoot(state.scriptDir), "tools", "graphiti-memoryd"),
	)
	return uniqueExistingPaths(paths)
}

func fallbackBinaryVersionSourcePaths(scriptDir string) []string {
	return []string{
		filepath.Join(scriptDir, "cmd", blueclaw.CapabilitydName),
		filepath.Join(scriptDir, "cmd", blueclaw.AdmindName),
		filepath.Join(scriptDir, "cmd", blueclaw.LocalLLMRunnerName),
		filepath.Join(scriptDir, "internal", "admind"),
		filepath.Join(scriptDir, "internal", "browser"),
		filepath.Join(scriptDir, "internal", "capabilities"),
		filepath.Join(scriptDir, "internal", "capabilityd"),
		filepath.Join(scriptDir, "internal", "identity"),
		filepath.Join(scriptDir, "internal", "llmbackend"),
		filepath.Join(scriptDir, "internal", "runtime", "blueclaw"),
		filepath.Join(scriptDir, "internal", "runtime", "locallm"),
	}
}

func (state *setupFlowState) binaryGoDependencyDirectories(packageNames ...string) []string {
	arguments := append([]string{"list", "-deps", "-f", "{{if not .Standard}}{{.Dir}}{{end}}"}, packageNames...)
	command := exec.Command("go", arguments...)
	command.Dir = state.scriptDir
	output, errorValue := command.Output()
	if errorValue != nil {
		return nil
	}
	paths := []string{
		filepath.Join(state.scriptDir, "go.mod"),
		filepath.Join(state.scriptDir, "go.sum"),
	}
	for _, line := range strings.Split(string(output), "\n") {
		path := strings.TrimSpace(line)
		if path == "" || !isPathInside(state.scriptDir, path) {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

func uniqueExistingPaths(paths []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, path := range paths {
		cleanPath := filepath.Clean(path)
		if seen[cleanPath] {
			continue
		}
		if _, errorValue := os.Stat(cleanPath); errorValue != nil {
			continue
		}
		seen[cleanPath] = true
		result = append(result, cleanPath)
	}
	sort.Strings(result)
	return result
}

func isPathInside(rootPath string, path string) bool {
	relativePath, errorValue := filepath.Rel(rootPath, path)
	if errorValue != nil {
		return false
	}
	return relativePath == "." || (!strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) && relativePath != "..")
}

func (state *setupFlowState) adminWebVersion() string {
	hash := sha256.New()
	webRoot := filepath.Join(state.scriptDir, "web")
	for _, path := range []string{
		filepath.Join(webRoot, "src"),
		filepath.Join(webRoot, "static"),
		filepath.Join(webRoot, "package.json"),
		filepath.Join(webRoot, "bun.lock"),
		filepath.Join(webRoot, "svelte.config.js"),
		filepath.Join(webRoot, "vite.config.ts"),
		filepath.Join(webRoot, "tsconfig.json"),
		filepath.Join(webRoot, "wrangler.jsonc"),
	} {
		state.writePathHash(hash, path)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (state *setupFlowState) writePathHash(hash io.Writer, path string) {
	fileInfo, errorValue := os.Stat(path)
	if errorValue != nil {
		return
	}
	if fileInfo.IsDir() {
		state.writeDirectoryHash(hash, path)
		return
	}
	document, readError := os.ReadFile(path)
	if readError != nil {
		return
	}
	_, _ = hash.Write([]byte(filepath.Base(path) + "\n"))
	_, _ = hash.Write(document)
	_, _ = hash.Write([]byte("\n"))
}

func (state *setupFlowState) writeDirectoryHash(hash io.Writer, rootPath string) {
	_ = filepath.WalkDir(rootPath, func(path string, entry os.DirEntry, walkError error) error {
		if walkError != nil || entry.IsDir() {
			return nil
		}
		relativePath, relativeError := filepath.Rel(rootPath, path)
		if relativeError != nil {
			return nil
		}
		document, readError := os.ReadFile(path)
		if readError != nil {
			return nil
		}
		_, _ = hash.Write([]byte(relativePath + "\n"))
		_, _ = hash.Write(document)
		_, _ = hash.Write([]byte("\n"))
		return nil
	})
}

func (state *setupFlowState) deployAdminWeb(context *setup.Context) error {
	webRoot := filepath.Join(state.scriptDir, "web")
	if _, errorValue := os.Stat(webRoot); errorValue != nil {
		return fmt.Errorf("web directory missing: %w", errorValue)
	}
	version := state.adminWebVersion()
	fmt.Println("  " + context.T("웹 빌드 중...", "Building web..."))
	if errorValue := state.runAdminWebCommand(webRoot, "bun", "run", "build"); errorValue != nil {
		return errorValue
	}
	if os.Getenv("INTERNKIM_SKIP_PAGES_DEPLOY_FOR_LAB") == "1" {
		fmt.Println("  " + context.T("Cloudflare Pages 배포 건너뜀 (lab)", "Skipping Cloudflare Pages deploy (lab)"))
	} else {
		fmt.Println("  " + context.T("Cloudflare Pages 배포 중...", "Deploying Cloudflare Pages..."))
		if errorValue := state.runAdminWebCommand(webRoot, "bunx", "wrangler", "pages", "deploy", ".svelte-kit/cloudflare", "--project-name", "internkim", "--branch", "main"); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := state.runAdminWebCommand(webRoot, "bun", "run", "build:board"); errorValue != nil {
		return errorValue
	}
	boardUIPath := filepath.Join(state.scriptDir, "build", "board-ui")
	switch context.Backend {
	case setup.BackendSSH:
		if state.sshClient != nil {
			if errorValue := state.deployAdminWebSSH(boardUIPath); errorValue != nil {
				return errorValue
			}
		}
	case setup.BackendSD:
		if context.SD != nil {
			if errorValue := copyDirectoryToStage(boardUIPath, filepath.Join(context.SD.RootPath(), "admin-ui")); errorValue != nil {
				return errorValue
			}
		}
	}
	if version != "" && context.Callbacks.SaveState != nil {
		context.Callbacks.SaveState("web_version", version)
		context.Callbacks.SaveState("admin_web_version", version)
	}
	fmt.Println("  " + context.T("웹 배포 완료", "Web deployed"))
	return nil
}

const (
	remoteAdminUIDirectoryPath    = "/opt/internkim/admin-ui"
	remoteInternKimAssetsPath     = "/opt/internkim/assets"
	remoteBotProfileImagePath     = remoteInternKimAssetsPath + "/internkim.png"
	temporaryAdminUIDirectoryPath = "/tmp/internkim-admin-ui"
	temporaryAdminUIArchivePath   = "/tmp/internkim-admin-ui.tar"
)

func (state *setupFlowState) deployAdminWebSSH(boardUIPath string) error {
	if errorValue := state.uploadAdminUIArchive(boardUIPath); errorValue != nil {
		return errorValue
	}
	if errorValue := state.installAdminUIArchive(); errorValue != nil {
		return errorValue
	}
	return state.verifyAdminUIDeployment(boardUIPath)
}

func (state *setupFlowState) uploadAdminUIArchive(boardUIPath string) error {
	archivePath, errorValue := createAdminUIArchive(boardUIPath)
	if errorValue != nil {
		return errorValue
	}
	defer os.Remove(archivePath)

	if errorValue := state.sshClient.scp(archivePath, temporaryAdminUIArchivePath); errorValue != nil {
		return errorValue
	}
	return nil
}

func createAdminUIArchive(sourceDirectoryPath string) (string, error) {
	archiveFile, errorValue := os.CreateTemp("", "internkim-admin-ui-*.tar")
	if errorValue != nil {
		return "", errorValue
	}
	archivePath := archiveFile.Name()
	_ = archiveFile.Close()

	command := exec.Command("tar", "-C", sourceDirectoryPath, "-cf", archivePath, ".")
	command.Env = append(os.Environ(), "COPYFILE_DISABLE=1")
	if output, archiveError := command.CombinedOutput(); archiveError != nil {
		_ = os.Remove(archivePath)
		return "", fmt.Errorf("archive admin UI: %s: %w", strings.TrimSpace(string(output)), archiveError)
	}
	return archivePath, nil
}

func (state *setupFlowState) installAdminUIArchive() error {
	command := strings.Join([]string{
		"rm -rf " + quoteShellValue(temporaryAdminUIDirectoryPath),
		"mkdir -p " + quoteShellValue(temporaryAdminUIDirectoryPath),
		"tar -xf " + quoteShellValue(temporaryAdminUIArchivePath) + " -C " + quoteShellValue(temporaryAdminUIDirectoryPath),
		"rm -rf " + quoteShellValue(remoteAdminUIDirectoryPath),
		"mkdir -p " + quoteShellValue(remoteAdminUIDirectoryPath),
		"cp -a " + quoteShellValue(temporaryAdminUIDirectoryPath) + "/. " + quoteShellValue(remoteAdminUIDirectoryPath) + "/",
		"chmod -R a+rX " + quoteShellValue(remoteAdminUIDirectoryPath),
		"rm -f " + quoteShellValue(temporaryAdminUIArchivePath),
	}, " && ")
	output, errorValue := state.sshClient.runResult(command)
	if errorValue != nil {
		return fmt.Errorf("deploy web UI: %s: %w", strings.TrimSpace(output), errorValue)
	}
	return nil
}

func (state *setupFlowState) verifyAdminUIDeployment(boardUIPath string) error {
	localVersion, errorValue := readAdminUIVersion(boardUIPath)
	if errorValue != nil {
		return errorValue
	}
	output, errorValue := state.sshClient.runResult("cat " + quoteShellValue(filepath.Join(remoteAdminUIDirectoryPath, "_app", "version.json")))
	if errorValue != nil {
		return fmt.Errorf("read deployed admin UI version: %s: %w", strings.TrimSpace(output), errorValue)
	}
	remoteVersion := strings.TrimSpace(output)
	if remoteVersion != localVersion {
		return fmt.Errorf("admin UI deploy verification failed: remote version %s does not match local version %s", remoteVersion, localVersion)
	}
	return nil
}

func readAdminUIVersion(boardUIPath string) (string, error) {
	document, errorValue := os.ReadFile(filepath.Join(boardUIPath, "_app", "version.json"))
	if errorValue != nil {
		return "", fmt.Errorf("read local admin UI version: %w", errorValue)
	}
	return strings.TrimSpace(string(document)), nil
}

func copyDirectoryToStage(sourceDirectory string, targetDirectory string) error {
	if errorValue := os.RemoveAll(targetDirectory); errorValue != nil {
		return errorValue
	}
	return filepath.WalkDir(sourceDirectory, func(path string, entry os.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relativePath, relativeError := filepath.Rel(sourceDirectory, path)
		if relativeError != nil {
			return relativeError
		}
		targetPath := filepath.Join(targetDirectory, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		document, readError := os.ReadFile(path)
		if readError != nil {
			return readError
		}
		return os.WriteFile(targetPath, document, 0o644)
	})
}

func (state *setupFlowState) runAdminWebCommand(webRoot string, name string, arguments ...string) error {
	command := exec.Command(name, arguments...)
	command.Dir = webRoot
	command.Env = os.Environ()
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("%s %s: %s", name, strings.Join(arguments, " "), strings.TrimSpace(string(output)))
	}
	return nil
}

func (state *setupFlowState) installBinariesSSH(context *setup.Context) error {
	assets, err := state.ensureLocalBinaryAssets()
	if err != nil {
		return err
	}

	for _, asset := range assets {
		existingHash := strings.TrimSpace(state.sshClient.run(
			fmt.Sprintf("md5sum %s 2>/dev/null | awk '{print $1}'", asset.remotePath),
		))
		localHash := strings.TrimSpace(runCmd("md5", "-q", asset.localPath))
		if existingHash != "" && existingHash == localHash {
			fmt.Printf("  %s %s\n", asset.name, state.messenger.t("이미 최신", "up to date"))
			continue
		}
		state.sshClient.run("mkdir -p " + quoteShellValue(filepath.Dir(asset.remotePath)))
		if err := state.sshClient.scp(asset.localPath, asset.remotePath); err != nil {
			return err
		}
		state.sshClient.run("chmod +x " + quoteShellValue(asset.remotePath))
		fmt.Printf("  %s %s\n", asset.name, state.messenger.t("설치 완료", "installed"))
	}

	if err := state.installDeviceBrowserRuntimeSSH(); err != nil {
		return err
	}

	if shouldInstallLocalLLMSSH(context) {
		if err := state.installLocalLLMSSH(); err != nil {
			return err
		}
	}

	if err := state.ensureAgentBrowserRuntimeSSH(); err != nil {
		return err
	}

	if err := state.ensureManagedHostExecutablesSSH(); err != nil {
		return err
	}

	state.sshClient.run(`
NOLOGIN_BIN=$(command -v nologin || echo /usr/sbin/nologin)
getent group blueclaw >/dev/null 2>&1 || groupadd --system blueclaw
id blueclaw &>/dev/null || useradd -r -g blueclaw -m -d /home/blueclaw -s "$NOLOGIN_BIN" blueclaw
getent group internkim-site >/dev/null 2>&1 || groupadd --system internkim-site
id internkim-site &>/dev/null || useradd -r -g internkim-site -d /nonexistent -s "$NOLOGIN_BIN" internkim-site
install -d -o blueclaw -g blueclaw -m 750 /home/blueclaw /home/blueclaw/.cache /home/blueclaw/.config
chmod 711 /root
mkdir -p /root/.internkim/secrets /root/.internkim/env
chown root:root /root/.internkim/secrets
chmod 700 /root/.internkim/secrets
chown root:blueclaw /root/.internkim/env
chmod 750 /root/.internkim/env
mkdir -p /root/.blueclaw/config /root/.blueclaw/workspace/bin /root/.blueclaw/workspace/downloads
chmod 755 /root/.blueclaw
chown -R blueclaw:blueclaw /root/.blueclaw/workspace
chmod 750 /root/.blueclaw/workspace
mkdir -p /root/.internkim/sites /root/.internkim/secrets/sites
chown root:internkim-site /root/.internkim /root/.internkim/sites /root/.internkim/secrets/sites
chmod 755 /root/.internkim
chmod 750 /root/.internkim/sites /root/.internkim/secrets/sites
chmod 755 /root/.blueclaw/workspace/bin /root/.blueclaw/workspace/downloads`)

	if err := state.installReleaseDownloadTokenSSH(); err != nil {
		return err
	}

	if err := state.installBlueclawMigrationsSSH(); err != nil {
		return err
	}
	if err := state.installMattermostEphemeralPluginSSH(); err != nil {
		return err
	}
	if err := state.installGraphitiMemorydSSH(); err != nil {
		return err
	}

	workspaceDocuments, errorValue := loadWorkspaceDocuments(state.scriptDir)
	if errorValue != nil {
		return errorValue
	}
	state.writeWorkspaceDocumentsSSH(workspaceDocuments)
	if err := state.installAgentBrowserSkillSSH(); err != nil {
		return err
	}

	state.sshClient.run(`mkdir -p /etc/sudoers.d
rm -f /usr/local/bin/gws-* /etc/sudoers.d/blueclaw-gws /etc/sudoers.d/blueclaw-mcp /usr/local/bin/role-memory-mcp /usr/local/bin/role-memory`)

	for _, binaryName := range []string{"download"} {
		state.sshClient.run(
			"cp /usr/local/bin/" + binaryName + " " + blueclaw.BlueclawWorkspaceBinaryPath(binaryName) + " && chmod 755 " + blueclaw.BlueclawWorkspaceBinaryPath(binaryName),
		)
	}
	if version := state.binariesVersion(); version != "" {
		state.sshClient.run("mkdir -p /root/.internkim/state && printf '%s' " + quoteShellValue(version) + " > /root/.internkim/state/binaries-version")
	}

	return nil
}

func (state *setupFlowState) installMattermostEphemeralPluginSSH() error {
	localPath := filepath.Join(state.scriptDir, "build", "mattermost-plugins", "com.internkim.ephemeral-0.1.0.tar.gz")
	if _, errorValue := os.Stat(localPath); errorValue != nil {
		return nil
	}
	remotePath := "/opt/internkim/mattermost-plugins/com.internkim.ephemeral-0.1.0.tar.gz"
	state.sshClient.run("mkdir -p " + quoteShellValue(filepath.Dir(remotePath)))
	if errorValue := state.sshClient.scp(localPath, remotePath); errorValue != nil {
		return errorValue
	}
	fmt.Printf("  mattermost-ephemeral-plugin %s\n", state.messenger.t("설치 완료", "installed"))
	return nil
}

func (state *setupFlowState) installReleaseDownloadTokenSSH() error {
	tokenPath, cleanup, errorValue := state.releaseDownloadTokenSourcePath()
	if errorValue != nil {
		return errorValue
	}
	if cleanup != nil {
		defer cleanup()
	}
	if tokenPath == "" {
		return nil
	}
	remotePath := "/root/.internkim/secrets/release-download-token"
	if errorValue := state.sshClient.scp(tokenPath, remotePath); errorValue != nil {
		return errorValue
	}
	state.sshClient.run("chown root:root " + quoteShellValue(remotePath) + " && chmod 600 " + quoteShellValue(remotePath))
	return nil
}

func (state *setupFlowState) releaseDownloadTokenSourcePath() (string, func(), error) {
	if token := strings.TrimSpace(os.Getenv("INTERNKIM_RELEASE_DOWNLOAD_TOKEN")); token != "" {
		file, errorValue := os.CreateTemp("", "internkim-release-token-*")
		if errorValue != nil {
			return "", nil, errorValue
		}
		cleanup := func() { _ = os.Remove(file.Name()) }
		if _, errorValue := file.WriteString(token + "\n"); errorValue != nil {
			_ = file.Close()
			cleanup()
			return "", nil, errorValue
		}
		if errorValue := file.Close(); errorValue != nil {
			cleanup()
			return "", nil, errorValue
		}
		if errorValue := os.Chmod(file.Name(), 0o600); errorValue != nil {
			cleanup()
			return "", nil, errorValue
		}
		return file.Name(), cleanup, nil
	}
	localPath := filepath.Join(state.scriptDir, ".local", "secrets", "release-download-token")
	if information, errorValue := os.Stat(localPath); errorValue == nil && !information.IsDir() {
		return localPath, nil, nil
	}
	return "", nil, nil
}

func (state *setupFlowState) ensureManagedHostExecutablesSSH() error {
	output, errorValue := state.sshClient.runResultWithTimeout(managedHostExecutablesScript(), 4*time.Minute)
	if errorValue != nil {
		return fmt.Errorf("ensure managed host executables: %s: %w", strings.TrimSpace(output), errorValue)
	}
	fmt.Printf("  managed host executables %s\n", state.messenger.t("준비 완료", "ready"))
	return nil
}

func managedHostExecutablesScript() string {
	return `set -eu
install -d -o root -g root -m 755 /opt/internkim/managed-bin /usr/local/bin
if ! command -v unzip >/dev/null 2>&1; then
  if ! command -v apt-get >/dev/null 2>&1; then
    echo "unzip is required to install bun"
    exit 1
  fi
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends unzip
fi
NOLOGIN_BIN=$(command -v nologin || echo /usr/sbin/nologin)
getent group blueclaw >/dev/null 2>&1 || groupadd --system blueclaw
id blueclaw >/dev/null 2>&1 || useradd -r -g blueclaw -m -d /home/blueclaw -s "$NOLOGIN_BIN" blueclaw
install -d -o blueclaw -g blueclaw -m 755 /home/blueclaw /home/blueclaw/.bun
if [ ! -x /opt/internkim/managed-bin/bun ]; then
  if [ ! -x /home/blueclaw/.bun/bin/bun ]; then
    su -s /bin/bash blueclaw -c 'env HOME=/home/blueclaw bash -lc "curl -fsSL https://bun.sh/install | bash"'
  fi
  install -o root -g root -m 755 /home/blueclaw/.bun/bin/bun /opt/internkim/managed-bin/bun
fi
install -o root -g root -m 755 /opt/internkim/managed-bin/bun /usr/local/bin/bun
ln -sfn /usr/local/bin/bun /usr/local/bin/bunx
cat > /usr/local/bin/marp <<'MARPEOF'
#!/bin/sh
exec /usr/local/bin/bun x --bun @marp-team/marp-cli "$@"
MARPEOF
chown root:root /usr/local/bin/bun /usr/local/bin/bunx /usr/local/bin/marp
chmod 755 /usr/local/bin/bun /usr/local/bin/marp
if ! command -v uv >/dev/null 2>&1; then
  curl -LsSf https://astral.sh/uv/0.11.11/install.sh -o /tmp/internkim-uv-install.sh
  UV_UNMANAGED_INSTALL=/usr/local/bin sh /tmp/internkim-uv-install.sh
fi
for managed_executable in bun bunx marp uv; do
  managed_path="/usr/local/bin/$managed_executable"
  test -x "$managed_path"
  managed_stat_path="$managed_path"
  if [ -L "$managed_path" ]; then
    managed_target="$(readlink "$managed_path")"
    case "$managed_target" in
      /*) managed_stat_path="$managed_target" ;;
      *) managed_stat_path="$(dirname "$managed_path")/$managed_target" ;;
    esac
  fi
  managed_owner="$(stat -c '%u' "$managed_stat_path")"
  managed_mode="$(stat -c '%a' "$managed_stat_path")"
  case "$managed_owner" in
    0|998) ;;
    *) echo "host-$managed_executable-owner-drift"; exit 1 ;;
  esac
  case "$managed_mode" in
    555|755) ;;
    *) echo "host-$managed_executable-mode-drift"; exit 1 ;;
  esac
done
`
}

func (state *setupFlowState) installAdmindSSH(context *setup.Context) error {
	if errorValue := state.deployBotProfileImageSSH(); errorValue != nil {
		return errorValue
	}
	if errorValue := state.installGoServiceBinarySSH(localBinaryAsset{
		name:       blueclaw.AdmindName,
		localPath:  filepath.Join(state.boardBinDir, blueclaw.AdmindName),
		remotePath: blueclaw.AdmindBinaryPath,
	}, blueclaw.AdmindServiceName, blueclaw.AdmindServicePath, blueclaw.AdmindServiceUnit()); errorValue != nil {
		return errorValue
	}
	return state.verifyAdmindDeployment(context)
}

func (state *setupFlowState) verifyAdmindDeployment(context *setup.Context) error {
	localHealth, errorValue := state.sshClient.runResult("curl -fsS http://127.0.0.1:18080/admin/api/health")
	if errorValue != nil {
		return fmt.Errorf("admind local health check failed: %s", strings.TrimSpace(localHealth))
	}
	if !strings.Contains(localHealth, `"status":"ok"`) || !strings.Contains(localHealth, `"recoveryAvailable":true`) {
		return fmt.Errorf("admind local health response missing recovery status: %s", strings.TrimSpace(localHealth))
	}
	if strings.TrimSpace(context.PublicURL) == "" || setupContextSkipsStep(context, "tunnel") {
		fmt.Printf("  %s\n", state.messenger.t("admind local 검증 완료", "admind local verification complete"))
		return nil
	}
	statusCode, responseBody, errorValue := fetchPublicEndpoint(context.PublicURL, "/admin/api/health")
	if errorValue != nil {
		return fmt.Errorf("admind public health check failed: %w", errorValue)
	}
	if statusCode < 200 || statusCode >= 300 || !strings.Contains(compactJSONSpaces(responseBody), `"status":"ok"`) {
		return fmt.Errorf("admind public health check failed: HTTP %d %s", statusCode, strings.TrimSpace(responseBody))
	}
	if _, errorValue := performSSHRecoveryRequest(commandTarget{
		stateDir:  state.stateDir,
		deviceURL: context.PublicURL,
	}, "status"); errorValue != nil {
		return fmt.Errorf("admind recovery route check failed: %w", errorValue)
	}
	fmt.Printf("  %s\n", state.messenger.t("admind public/recovery 검증 완료", "admind public/recovery verification complete"))
	return nil
}

func setupContextSkipsStep(context *setup.Context, stepName string) bool {
	for _, part := range strings.Fields(context.SetupSteps) {
		if !strings.HasPrefix(part, "--skip=") {
			continue
		}
		for _, skippedStepName := range setup.ParseNames(strings.TrimPrefix(part, "--skip=")) {
			if skippedStepName == stepName {
				return true
			}
		}
	}
	return false
}

func (state *setupFlowState) deployBotProfileImageSSH() error {
	sourcePath := filepath.Join(state.scriptDir, "assets", "internkim.png")
	if _, errorValue := os.Stat(sourcePath); errorValue != nil {
		return fmt.Errorf("bot profile image missing: %w", errorValue)
	}
	state.sshClient.run("mkdir -p " + quoteShellValue(remoteInternKimAssetsPath))
	if errorValue := state.sshClient.scp(sourcePath, remoteBotProfileImagePath); errorValue != nil {
		return errorValue
	}
	state.sshClient.run("chmod 644 " + quoteShellValue(remoteBotProfileImagePath))
	return nil
}

func (state *setupFlowState) installCapabilitydSSH(context *setup.Context) error {
	return state.installGoServiceBinarySSH(localBinaryAsset{
		name:       blueclaw.CapabilitydName,
		localPath:  filepath.Join(state.boardBinDir, blueclaw.CapabilitydName),
		remotePath: blueclaw.CapabilitydBinaryPath,
	}, blueclaw.CapabilitydServiceName, blueclaw.CapabilitydServicePath, blueclaw.CapabilitydServiceUnit())
}

func (state *setupFlowState) installGoServiceBinarySSH(asset localBinaryAsset, serviceName string, servicePath string, serviceDocument string) error {
	if err := os.MkdirAll(filepath.Dir(asset.localPath), 0o755); err != nil {
		return err
	}
	if err := buildGoBinaryAsset(state, asset); err != nil {
		return err
	}
	existingHash := strings.TrimSpace(state.sshClient.run(
		fmt.Sprintf("md5sum %s 2>/dev/null | awk '{print $1}'", asset.remotePath),
	))
	localHash := strings.TrimSpace(runCmd("md5", "-q", asset.localPath))
	if existingHash != "" && existingHash == localHash {
		fmt.Printf("  %s %s\n", asset.name, state.messenger.t("이미 최신", "up to date"))
	} else {
		state.sshClient.run("mkdir -p " + quoteShellValue(filepath.Dir(asset.remotePath)))
		if err := state.sshClient.scp(asset.localPath, asset.remotePath); err != nil {
			return err
		}
		state.sshClient.run("chmod +x " + quoteShellValue(asset.remotePath))
		fmt.Printf("  %s %s\n", asset.name, state.messenger.t("설치 완료", "installed"))
	}
	state.installGoServiceUnitSSH(servicePath, serviceDocument)
	state.sshClient.run("systemctl restart " + serviceName)
	if strings.TrimSpace(state.sshClient.run("systemctl is-active "+serviceName+" 2>/dev/null")) != "active" {
		return fmt.Errorf("%s restart failed", serviceName)
	}
	fmt.Printf("  %s %s\n", serviceName, state.messenger.t("재시작 완료", "restarted"))
	return nil
}

func (state *setupFlowState) installGoServiceUnitSSH(servicePath string, serviceDocument string) {
	servicePath = strings.TrimSpace(servicePath)
	serviceDocument = strings.TrimSpace(serviceDocument)
	if servicePath == "" || serviceDocument == "" {
		return
	}
	state.sshClient.run(fmt.Sprintf("cat > %s <<'SERVICEEOF'\n%s\nSERVICEEOF\nsystemctl daemon-reload", quoteShellValue(servicePath), serviceDocument))
}

func shouldInstallLocalLLMSSH(context *setup.Context) bool {
	return context.BoardType != setup.BoardSimulation && context.PlannedSteps["local-llm"]
}

func (state *setupFlowState) installSkillsSSH(context *setup.Context) error {
	skillsDirectoryPath := blueclawworkspace.SkillsPath(state.scriptDir)
	if info, errorValue := os.Stat(skillsDirectoryPath); errorValue != nil || !info.IsDir() {
		return fmt.Errorf("skills directory missing: %s", skillsDirectoryPath)
	}
	toolsDirectoryPath := blueclawworkspace.ToolsPath(state.scriptDir)
	if info, errorValue := os.Stat(toolsDirectoryPath); errorValue != nil || !info.IsDir() {
		return fmt.Errorf("tools directory missing: %s", toolsDirectoryPath)
	}
	if errorValue := state.installSkillPythonDependenciesSSH(); errorValue != nil {
		return errorValue
	}

	agentsPath := blueclawworkspace.AgentsPath(state.scriptDir)
	if errorValue := state.sshClient.scp(agentsPath, filepath.Join(blueclaw.BlueclawWorkspacePath, "AGENTS.md")); errorValue != nil {
		return errorValue
	}
	remoteSkillsDirectoryPath := filepath.Join(blueclaw.BlueclawWorkspacePath, "skills")
	state.sshClient.run("rm -rf " + quoteShellValue(remoteSkillsDirectoryPath) + " && mkdir -p " + quoteShellValue(remoteSkillsDirectoryPath))
	if errorValue := state.sshClient.scpDir(skillsDirectoryPath, remoteSkillsDirectoryPath); errorValue != nil {
		return errorValue
	}
	remoteToolsDirectoryPath := filepath.Join(blueclaw.BlueclawWorkspacePath, "tools")
	state.sshClient.run("rm -rf " + quoteShellValue(remoteToolsDirectoryPath) + " && mkdir -p " + quoteShellValue(remoteToolsDirectoryPath))
	if errorValue := state.sshClient.scpDir(toolsDirectoryPath, remoteToolsDirectoryPath); errorValue != nil {
		return errorValue
	}

	manifest := state.skillsManifest()
	if manifest == "" {
		return errors.New("skills manifest is empty")
	}
	temporaryManifestPath, errorValue := os.CreateTemp("", "internkim-skills-manifest-*.json")
	if errorValue != nil {
		return errorValue
	}
	temporaryManifestName := temporaryManifestPath.Name()
	if _, errorValue := temporaryManifestPath.WriteString(manifest); errorValue != nil {
		temporaryManifestPath.Close()
		os.Remove(temporaryManifestName)
		return errorValue
	}
	if errorValue := temporaryManifestPath.Close(); errorValue != nil {
		os.Remove(temporaryManifestName)
		return errorValue
	}
	defer os.Remove(temporaryManifestName)

	if errorValue := state.sshClient.scp(temporaryManifestName, filepath.Join(blueclaw.BlueclawWorkspacePath, "skills", ".internkim-skills-manifest.json")); errorValue != nil {
		return errorValue
	}
	state.sshClient.run(`chown -R root:root /root/.blueclaw/workspace/skills 2>/dev/null || true
chmod -R a+rX,go-w /root/.blueclaw/workspace/skills 2>/dev/null || true
chown -R root:root /root/.blueclaw/workspace/tools 2>/dev/null || true
chmod -R a+rX,go-w /root/.blueclaw/workspace/tools 2>/dev/null || true
chown root:root /root/.blueclaw/workspace/AGENTS.md 2>/dev/null || true
chmod 644 /root/.blueclaw/workspace/AGENTS.md 2>/dev/null || true`)
	return nil
}

func (state *setupFlowState) installSkillPythonDependenciesSSH() error {
	output, errorValue := state.sshClient.runResult(installSkillPythonDependenciesCommand())
	if errorValue != nil {
		return fmt.Errorf("install skill Python dependencies: %w: %s", errorValue, strings.TrimSpace(output))
	}
	return nil
}

func (state *setupFlowState) stageSkillsSD(context *setup.Context) error {
	skillsDirectoryPath := blueclawworkspace.SkillsPath(state.scriptDir)
	if info, errorValue := os.Stat(skillsDirectoryPath); errorValue != nil || !info.IsDir() {
		return fmt.Errorf("skills directory missing: %s", skillsDirectoryPath)
	}
	toolsDirectoryPath := blueclawworkspace.ToolsPath(state.scriptDir)
	if info, errorValue := os.Stat(toolsDirectoryPath); errorValue != nil || !info.IsDir() {
		return fmt.Errorf("tools directory missing: %s", toolsDirectoryPath)
	}
	if errorValue := copyDirectoryToStage(skillsDirectoryPath, filepath.Join(context.SD.RootPath(), "skills")); errorValue != nil {
		return errorValue
	}
	if errorValue := copyDirectoryToStage(toolsDirectoryPath, filepath.Join(context.SD.RootPath(), "tools")); errorValue != nil {
		return errorValue
	}
	agentsDocument, errorValue := os.ReadFile(blueclawworkspace.AgentsPath(state.scriptDir))
	if errorValue != nil {
		return errorValue
	}
	return context.SD.WriteFile("AGENTS.md", agentsDocument, 0o644)
}

func (state *setupFlowState) skillsManifest() string {
	skillsDirectoryPath := blueclawworkspace.SkillsPath(state.scriptDir)
	digest, errorValue := directoryDigest(skillsDirectoryPath)
	if errorValue != nil {
		return ""
	}
	agentsDigest, errorValue := fileSHA256(blueclawworkspace.AgentsPath(state.scriptDir))
	if errorValue != nil {
		return ""
	}
	toolsDigest, errorValue := directoryDigest(blueclawworkspace.ToolsPath(state.scriptDir))
	if errorValue != nil {
		return ""
	}
	digest = sha256String(digest + ":" + agentsDigest + ":" + toolsDigest)
	return fmt.Sprintf("{\n  \"name\": \"internkim-skills\",\n  \"sha256\": \"%s\"\n}\n", digest)
}

func fileSHA256(filePath string) (string, error) {
	document, errorValue := os.ReadFile(filePath)
	if errorValue != nil {
		return "", errorValue
	}
	sum := sha256.Sum256(document)
	return fmt.Sprintf("%x", sum), nil
}

func sha256String(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum)
}

func directoryDigest(directoryPath string) (string, error) {
	filePaths := []string{}
	errorValue := filepath.WalkDir(directoryPath, func(path string, entry os.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if entry.IsDir() {
			return nil
		}
		filePaths = append(filePaths, path)
		return nil
	})
	if errorValue != nil {
		return "", errorValue
	}
	sort.Strings(filePaths)

	hash := sha256.New()
	for _, filePath := range filePaths {
		relativePath, errorValue := filepath.Rel(directoryPath, filePath)
		if errorValue != nil {
			return "", errorValue
		}
		io.WriteString(hash, filepath.ToSlash(relativePath))
		io.WriteString(hash, "\n")
		file, errorValue := os.Open(filePath)
		if errorValue != nil {
			return "", errorValue
		}
		_, copyError := io.Copy(hash, file)
		closeError := file.Close()
		if copyError != nil {
			return "", copyError
		}
		if closeError != nil {
			return "", closeError
		}
		io.WriteString(hash, "\n")
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (state *setupFlowState) installBlueclawRuntimeSSH(context *setup.Context) error {
	artifactDirectoryPath, errorValue := state.ensureBlueclawRuntimeBaseArtifact()
	if errorValue != nil {
		return errorValue
	}
	manifestDocument := state.blueclawRuntimeManifest()
	manifest, errorValue := blueclaw.ValidateRuntimeArtifactDirectory(artifactDirectoryPath)
	if errorValue != nil {
		return fmt.Errorf("blueclaw Firecracker base runtime artifact invalid: %w; run `make prepare-blueclaw-runtime-base` before setup", errorValue)
	}
	if errorValue := blueclaw.ValidateRuntimeArtifactSource(state.scriptDir, manifest); errorValue != nil {
		return fmt.Errorf("blueclaw Firecracker base runtime artifact invalid: %w", errorValue)
	}

	fmt.Print("  blueclaw Firecracker base runtime... ")
	remoteManifestDocument := state.sshClient.run("cat " + blueclaw.BlueclawRuntimeManifestPath + " 2>/dev/null || true")
	installPlan := buildBlueclawRuntimeInstallPlan(
		manifest,
		remoteManifestDocument,
		state.remoteBlueclawRuntimeFilePresence(),
		strings.TrimSpace(state.sshClient.run(setup.BlueclawRootfsBaseContractCheckCommand())) == "ok",
		manifestDocument == remoteManifestDocument,
	)
	if context.Force {
		installPlan = forceBlueclawRuntimeInstallPlan(installPlan)
	}
	printBlueclawRuntimeInstallPlan(installPlan)
	if blueclawRuntimeInstallPlanIsCurrent(installPlan) {
		fmt.Println("already current")
		return nil
	}
	blueclawWasActive := strings.TrimSpace(state.sshClient.run("systemctl is-active blueclaw 2>/dev/null || true")) == "active"
	state.sshClient.run("systemctl stop blueclaw 2>/dev/null || true")
	state.sshClient.run("rm -rf /tmp/internkim-blueclaw-runtime && mkdir -p /tmp/internkim-blueclaw-runtime/runtime " + blueclaw.BlueclawRuntimeInstallPath + " /var/lib/blueclaw /var/log/blueclaw-supervisor")
	for _, artifact := range installPlan.artifacts {
		if !artifact.shouldInstall {
			continue
		}
		localArtifactPath, errorValue := blueclaw.RuntimeArtifactFilePath(artifactDirectoryPath, manifest, artifact.name)
		if errorValue != nil {
			fmt.Println("failed")
			return errorValue
		}
		temporaryRemotePath := "/tmp/internkim-blueclaw-runtime/runtime/" + artifact.name
		printBlueclawRuntimeArtifactInstalling(artifact)
		if errorValue := state.transferBlueclawRuntimeArtifact(localArtifactPath, temporaryRemotePath, artifact.name); errorValue != nil {
			fmt.Println("failed")
			return errorValue
		}
		output, errorValue := state.sshClient.runResultWithTimeout(
			blueclawRuntimeInstallCommand(artifact, temporaryRemotePath),
			blueclawRuntimeInstallTimeout(artifact),
		)
		if errorValue != nil {
			fmt.Println("failed")
			return fmt.Errorf("install %s: %s: %w", artifact.name, strings.TrimSpace(output), errorValue)
		}
	}
	if installPlan.shouldInstallManifest {
		if errorValue := state.sshClient.scp(filepath.Join(artifactDirectoryPath, "manifest.json"), "/tmp/internkim-blueclaw-runtime/runtime/manifest.json"); errorValue != nil {
			fmt.Println("failed")
			return errorValue
		}
		output, errorValue := state.sshClient.runResult("install -m 0644 /tmp/internkim-blueclaw-runtime/runtime/manifest.json " + blueclaw.BlueclawRuntimeManifestPath)
		if errorValue != nil {
			fmt.Println("failed")
			return fmt.Errorf("install blueclaw runtime manifest: %s: %w", strings.TrimSpace(output), errorValue)
		}
	}
	output, errorValue := state.sshClient.runResult(`set -eu
test -x /usr/local/bin/blueclaw-supervisor
test -x /usr/local/bin/firecracker
test -x /usr/local/bin/jailer
test -s /opt/internkim/blueclaw-runtime/vmlinux.bin
test -s /opt/internkim/blueclaw-runtime/rootfs.ext4
minimum_workspace_bytes=34359738368
was_active="$(systemctl is-active blueclaw 2>/dev/null || true)"
if [ ! -e /var/lib/blueclaw/workspace.ext4 ]; then
  truncate -s "$minimum_workspace_bytes" /var/lib/blueclaw/workspace.ext4
fi
if ! blkid -o value -s TYPE /var/lib/blueclaw/workspace.ext4 2>/dev/null | grep -qx ext4; then
  mkfs.ext4 -F -L blueclaw-workspace /var/lib/blueclaw/workspace.ext4 >/dev/null
fi
workspace_bytes="$(stat -c '%s' /var/lib/blueclaw/workspace.ext4)"
if [ "$workspace_bytes" -lt "$minimum_workspace_bytes" ]; then
  systemctl stop blueclaw 2>/dev/null || true
  truncate -s "$minimum_workspace_bytes" /var/lib/blueclaw/workspace.ext4
  e2fsck -fy /var/lib/blueclaw/workspace.ext4 >/dev/null
  resize2fs /var/lib/blueclaw/workspace.ext4 >/dev/null
  if [ "$was_active" = "active" ]; then
    systemctl start blueclaw 2>/dev/null || true
  fi
fi
chmod 0600 /var/lib/blueclaw/workspace.ext4
mkdir -p /var/log/blueclaw-supervisor
`)
	if errorValue != nil {
		fmt.Println("failed")
		return fmt.Errorf("verify blueclaw Firecracker runtime: %s: %w", strings.TrimSpace(output), errorValue)
	}
	if blueclawWasActive {
		if _, startError := state.sshClient.runResult("systemctl start blueclaw"); startError != nil {
			fmt.Println("failed")
			return fmt.Errorf("restart blueclaw after runtime install: %w", startError)
		}
	}
	fmt.Println("installed")
	return nil
}

func (state *setupFlowState) blueclawRuntimeManifest() string {
	manifestPath := filepath.Join(state.scriptDir, blueclaw.BlueclawRuntimeArtifactPath, "manifest.json")
	document, errorValue := os.ReadFile(manifestPath)
	if errorValue != nil {
		return ""
	}
	manifest, errorValue := blueclaw.ParseRuntimeArtifactManifest(document)
	if errorValue != nil {
		return ""
	}
	if errorValue := blueclaw.ValidateRuntimeArtifactSource(state.scriptDir, manifest); errorValue != nil {
		return ""
	}
	return string(document)
}

func (state *setupFlowState) installBlueclawPayloadSSH(context *setup.Context) error {
	artifactDirectoryPath, errorValue := state.ensureBlueclawPayloadArtifact()
	if errorValue != nil {
		return errorValue
	}
	manifestDocument := state.blueclawPayloadManifest()
	manifest, errorValue := blueclaw.ValidatePayloadArtifactDirectory(artifactDirectoryPath)
	if errorValue != nil {
		return fmt.Errorf("blueclaw payload artifact invalid: %w; run `make prepare-blueclaw-payload` before setup", errorValue)
	}
	if errorValue := blueclaw.ValidatePayloadArtifactSource(state.scriptDir, manifest); errorValue != nil {
		return fmt.Errorf("blueclaw payload artifact invalid: %w", errorValue)
	}

	fmt.Print("  blueclaw runtime payload... ")
	if result, errorValue := state.installBlueclawPayloadHTTPS(artifactDirectoryPath, manifest); errorValue == nil {
		fmt.Println(result)
		return nil
	} else {
		fmt.Printf("https self-update unavailable (%s); falling back to SSH... ", strings.TrimSpace(errorValue.Error()))
		if state.sshClient == nil {
			fmt.Println("failed")
			return fmt.Errorf("blueclaw payload deploy requires SSH because https self-update failed: %w", errorValue)
		}
	}
	remoteManifestDocument := state.sshClient.run("cat " + blueclaw.BlueclawPayloadManifestPath + " 2>/dev/null || true")
	remoteWorkspaceManifestDocument := state.sshClient.run(blueclawWorkspaceManifestCommand())
	if manifestDocument == remoteManifestDocument && manifestDocument == remoteWorkspaceManifestDocument {
		fmt.Println("already current")
		return nil
	}

	temporaryPayloadPath := "/tmp/internkim-blueclaw-payload"
	output, errorValue := state.sshClient.runResult(blueclawStopForPayloadSyncCommand())
	if errorValue != nil {
		fmt.Println("failed")
		return fmt.Errorf("stop blueclaw before payload sync: %s: %w", strings.TrimSpace(output), errorValue)
	}
	state.sshClient.run("rm -rf " + temporaryPayloadPath + " && mkdir -p " + temporaryPayloadPath + " " + blueclaw.BlueclawRuntimeInstallPath)
	if errorValue := state.sshClient.scpDir(blueclaw.PayloadWorkspacePath(artifactDirectoryPath), temporaryPayloadPath+"/workspace"); errorValue != nil {
		fmt.Println("failed")
		return errorValue
	}
	if errorValue := state.sshClient.scp(filepath.Join(artifactDirectoryPath, "manifest.json"), temporaryPayloadPath+"/manifest.json"); errorValue != nil {
		fmt.Println("failed")
		return errorValue
	}

	output, errorValue = state.sshClient.runResult(blueclawHostWorkspacePayloadSyncCommand(temporaryPayloadPath))
	if errorValue != nil {
		fmt.Println("failed")
		return fmt.Errorf("sync blueclaw payload host workspace: %s: %w", strings.TrimSpace(output), errorValue)
	}

	syncCommand := strings.Join([]string{
		blueclaw.BlueclawSupervisorBinaryPath,
		"sync-workspace",
		"--workspace-image", quoteShellValue(blueclaw.BlueclawWorkspaceImagePath),
		"--source", quoteShellValue(temporaryPayloadPath + "/workspace"),
	}, " ")
	output, errorValue = state.sshClient.runResult(syncCommand)
	if errorValue != nil {
		fmt.Println("failed")
		return fmt.Errorf("sync blueclaw payload workspace: %s: %w", strings.TrimSpace(output), errorValue)
	}
	remoteWorkspaceManifestDocument = state.sshClient.run(blueclawWorkspaceManifestCommand())
	if manifestDocument != remoteWorkspaceManifestDocument {
		fmt.Println("failed")
		return fmt.Errorf("sync blueclaw payload workspace: workspace manifest mismatch")
	}
	output, errorValue = state.sshClient.runResult("install -m 0644 " + temporaryPayloadPath + "/manifest.json " + blueclaw.BlueclawPayloadManifestPath)
	if errorValue != nil {
		fmt.Println("failed")
		return fmt.Errorf("install blueclaw payload manifest: %s: %w", strings.TrimSpace(output), errorValue)
	}
	output, errorValue = state.sshClient.runResult(blueclawStartAfterPayloadSyncCommand())
	if errorValue != nil {
		fmt.Println("failed")
		return fmt.Errorf("start blueclaw after payload sync: %s: %w", strings.TrimSpace(output), errorValue)
	}

	fmt.Println("installed")
	return nil
}

func (state *setupFlowState) installBlueclawPayloadDirectHTTPS() error {
	artifactDirectoryPath, errorValue := state.ensureBlueclawPayloadArtifact()
	if errorValue != nil {
		return errorValue
	}
	manifest, errorValue := blueclaw.ValidatePayloadArtifactDirectory(artifactDirectoryPath)
	if errorValue != nil {
		return fmt.Errorf("blueclaw payload artifact invalid: %w", errorValue)
	}
	fmt.Print("  blueclaw runtime payload... ")
	result, errorValue := state.installBlueclawPayloadHTTPS(artifactDirectoryPath, manifest)
	if errorValue != nil {
		fmt.Println("failed")
		return errorValue
	}
	fmt.Println(result)
	return nil
}

func blueclawHostWorkspacePayloadSyncCommand(temporaryPayloadPath string) string {
	return blueclaw.HostWorkspacePayloadSyncCommand(temporaryPayloadPath)
}

func blueclawStopForPayloadSyncCommand() string {
	return blueclaw.StopForPayloadSyncCommand()
}

func blueclawStartAfterPayloadSyncCommand() string {
	return blueclaw.StartAfterPayloadSyncCommand()
}

func blueclawWorkspaceManifestCommand() string {
	return "debugfs -R " + quoteShellValue("cat /.blueclaw/runtime/current/manifest.json") + " " + quoteShellValue(blueclaw.BlueclawWorkspaceImagePath) + " 2>/dev/null || true"
}

func (state *setupFlowState) blueclawPayloadManifest() string {
	manifestPath := filepath.Join(state.scriptDir, blueclaw.BlueclawPayloadArtifactPath, "manifest.json")
	document, errorValue := os.ReadFile(manifestPath)
	if errorValue != nil {
		return ""
	}
	manifest, errorValue := blueclaw.ParsePayloadArtifactManifest(document)
	if errorValue != nil {
		return ""
	}
	if errorValue := blueclaw.ValidatePayloadArtifactSource(state.scriptDir, manifest); errorValue != nil {
		return ""
	}
	return string(document)
}

func (state *setupFlowState) ensureBlueclawRuntimeBaseArtifact() (string, error) {
	artifactDirectoryPath := filepath.Join(state.scriptDir, blueclaw.BlueclawRuntimeArtifactPath)
	manifest, errorValue := blueclaw.ValidateRuntimeArtifactDirectory(artifactDirectoryPath)
	if errorValue == nil {
		errorValue = blueclaw.ValidateRuntimeArtifactSource(state.scriptDir, manifest)
	}
	if errorValue == nil {
		return artifactDirectoryPath, nil
	}
	if reuseError := state.reuseBlueclawRuntimeBaseArtifact(artifactDirectoryPath); reuseError == nil {
		manifest, errorValue = blueclaw.ValidateRuntimeArtifactDirectory(artifactDirectoryPath)
		if errorValue == nil {
			errorValue = blueclaw.ValidateRuntimeArtifactSource(state.scriptDir, manifest)
		}
		if errorValue == nil {
			return artifactDirectoryPath, nil
		}
	}
	if makeError := state.runMakeTarget("prepare-blueclaw-runtime-base"); makeError != nil {
		return "", fmt.Errorf("blueclaw Firecracker base runtime artifact invalid: %w; automatic preparation failed: %w", errorValue, makeError)
	}
	manifest, errorValue = blueclaw.ValidateRuntimeArtifactDirectory(artifactDirectoryPath)
	if errorValue != nil {
		return "", fmt.Errorf("blueclaw Firecracker base runtime artifact invalid after automatic preparation: %w", errorValue)
	}
	if errorValue := blueclaw.ValidateRuntimeArtifactSource(state.scriptDir, manifest); errorValue != nil {
		return "", fmt.Errorf("blueclaw Firecracker base runtime artifact source invalid after automatic preparation: %w", errorValue)
	}
	return artifactDirectoryPath, nil
}

func (state *setupFlowState) reuseBlueclawRuntimeBaseArtifact(artifactDirectoryPath string) error {
	currentArtifactDirectoryPath, _ := filepath.Abs(artifactDirectoryPath)
	for _, worktreePath := range gitWorktreePaths(state.scriptDir) {
		candidateArtifactDirectoryPath := filepath.Join(worktreePath, blueclaw.BlueclawRuntimeArtifactPath)
		absoluteCandidateArtifactDirectoryPath, _ := filepath.Abs(candidateArtifactDirectoryPath)
		if absoluteCandidateArtifactDirectoryPath == currentArtifactDirectoryPath {
			continue
		}
		manifest, errorValue := blueclaw.ValidateRuntimeArtifactDirectory(candidateArtifactDirectoryPath)
		if errorValue != nil {
			continue
		}
		if errorValue := blueclaw.ValidateRuntimeArtifactSource(state.scriptDir, manifest); errorValue != nil {
			continue
		}
		if errorValue := os.RemoveAll(artifactDirectoryPath); errorValue != nil {
			return errorValue
		}
		if errorValue := copyDirectoryContents(candidateArtifactDirectoryPath, artifactDirectoryPath); errorValue != nil {
			return errorValue
		}
		fmt.Println("  Blueclaw runtime artifact reused from " + candidateArtifactDirectoryPath)
		return nil
	}
	return errors.New("no reusable Blueclaw runtime artifact found")
}

func gitWorktreePaths(repositoryRootPath string) []string {
	command := exec.Command("git", "-C", repositoryRootPath, "worktree", "list", "--porcelain")
	output, errorValue := command.Output()
	if errorValue != nil {
		return nil
	}
	return parseGitWorktreePaths(string(output))
}

func parseGitWorktreePaths(output string) []string {
	paths := []string{}
	for _, line := range strings.Split(output, "\n") {
		worktreePath, found := strings.CutPrefix(line, "worktree ")
		if found && strings.TrimSpace(worktreePath) != "" {
			paths = append(paths, strings.TrimSpace(worktreePath))
		}
	}
	return paths
}

func (state *setupFlowState) ensureBlueclawPayloadArtifact() (string, error) {
	artifactDirectoryPath := filepath.Join(state.scriptDir, blueclaw.BlueclawPayloadArtifactPath)
	manifest, errorValue := blueclaw.ValidatePayloadArtifactDirectory(artifactDirectoryPath)
	if errorValue == nil {
		errorValue = blueclaw.ValidatePayloadArtifactSource(state.scriptDir, manifest)
	}
	if errorValue == nil {
		return artifactDirectoryPath, nil
	}
	if makeError := state.runMakeTarget("prepare-blueclaw-payload"); makeError != nil {
		return "", fmt.Errorf("blueclaw payload artifact invalid: %w; automatic preparation failed: %w", errorValue, makeError)
	}
	manifest, errorValue = blueclaw.ValidatePayloadArtifactDirectory(artifactDirectoryPath)
	if errorValue != nil {
		return "", fmt.Errorf("blueclaw payload artifact invalid after automatic preparation: %w", errorValue)
	}
	if errorValue := blueclaw.ValidatePayloadArtifactSource(state.scriptDir, manifest); errorValue != nil {
		return "", fmt.Errorf("blueclaw payload artifact source invalid after automatic preparation: %w", errorValue)
	}
	return artifactDirectoryPath, nil
}

func (state *setupFlowState) runMakeTarget(targetName string) error {
	command := exec.Command("make", targetName)
	command.Dir = state.scriptDir
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("make %s failed: %s: %w", targetName, strings.TrimSpace(string(output)), errorValue)
	}
	return nil
}

func (state *setupFlowState) installAgentBrowserSkillSSH() error {
	fallbackContent, err := loadAgentBrowserSkillMarkdown(state.scriptDir)
	if err != nil {
		return err
	}
	state.sshClient.run(agentBrowserSkillInstallScript("/tmp/internkim-agent-browser-skill-fallback.md", fallbackContent))
	return nil
}

func (state *setupFlowState) installLocalLLMSSH() error {
	switch locallm.Default {
	case locallm.BackendLlamaCpp:
		return state.installLocalLLMBinarySSH(
			locallm.LlamaCppDisplayName,
			locallm.LlamaCppBuildTool,
			locallm.LlamaCppCacheRelative,
			locallm.LlamaCppCacheKey,
			locallm.LlamaCppBinaryPath,
			locallm.LlamaCppLibraryDir,
		)
	default:
		return state.installLocalLLMBinarySSH(
			locallm.LiteRTDisplayName,
			locallm.LiteRTBuildTool,
			locallm.LiteRTCacheRelative,
			locallm.LiteRTCacheKey,
			locallm.LiteRTBinaryPath,
			locallm.LiteRTLibraryDir,
		)
	}
}

func (state *setupFlowState) installLocalLLMBinarySSH(displayName, buildTool, cacheRelative, cacheKey, remoteBinaryPath, remoteLibraryDir string) error {
	cacheDirectory := filepath.Join(state.scriptDir, cacheRelative, cacheKey)
	binaryPath := filepath.Join(cacheDirectory, filepath.Base(remoteBinaryPath))
	libraryDirectory := filepath.Join(cacheDirectory, "lib")

	if !cachedArtifactsPresent(binaryPath, libraryDirectory) {
		fmt.Println("  " + state.messenger.t(displayName+" 빌드 캐시 준비 중...", "Preparing "+displayName+" build cache..."))
		command := state.localLLMBuildCommand(buildTool)
		if buildError := command.Run(); buildError != nil {
			return fmt.Errorf("%s build failed: %w", displayName, buildError)
		}
	}
	libraryPaths, errorValue := libraryAssetPathsIn(libraryDirectory)
	if errorValue != nil {
		return fmt.Errorf("%s libraries missing at %s: %w", displayName, libraryDirectory, errorValue)
	}

	fmt.Print("  " + displayName + "... ")
	state.sshClient.run("mkdir -p " + quoteShellValue(filepath.Dir(remoteBinaryPath)) + " " + quoteShellValue(remoteLibraryDir))

	if !state.remoteFileMatchesLocal(remoteBinaryPath, binaryPath) {
		if scpError := state.sshClient.scp(binaryPath, remoteBinaryPath); scpError != nil {
			fmt.Println("failed")
			return scpError
		}
		state.sshClient.run("chmod 0755 " + quoteShellValue(remoteBinaryPath))
	}
	for _, libraryPath := range libraryPaths {
		remoteLibraryPath := filepath.Join(remoteLibraryDir, filepath.Base(libraryPath))
		if state.remoteFileMatchesLocal(remoteLibraryPath, libraryPath) {
			continue
		}
		if scpError := state.sshClient.scp(libraryPath, remoteLibraryPath); scpError != nil {
			fmt.Println("failed")
			return scpError
		}
		state.sshClient.run("chmod 0644 " + quoteShellValue(remoteLibraryPath))
	}
	state.sshClient.run("ldconfig -n " + quoteShellValue(remoteLibraryDir) + " 2>/dev/null || true")
	if pruneError := state.pruneLocalLLMBuildCaches(cacheRelative, cacheKey); pruneError != nil {
		fmt.Println("failed")
		return pruneError
	}
	if pruneError := state.pruneRemoteLocalLLMBuildCaches(); pruneError != nil {
		fmt.Println("failed")
		return pruneError
	}
	fmt.Println(state.messenger.t("설치 완료", "installed"))
	return nil
}

func (state *setupFlowState) localLLMBuildCommand(buildTool string) *exec.Cmd {
	command := exec.Command(filepath.Join(state.scriptDir, "tools", buildTool), localLLMBuildArguments(state.sshClient)...)
	command.Dir = state.scriptDir
	command.Env = localLLMBuildEnvironment(os.Environ(), state.sshClient)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command
}

func localLLMBuildArguments(sshClient *sshClient) []string {
	if sshClient == nil {
		return nil
	}
	arguments := []string{}
	arguments = appendLocalLLMBuildArgument(arguments, "--host", sshClient.host)
	arguments = appendLocalLLMBuildArgument(arguments, "--user", sshClient.user)
	return arguments
}

func localLLMBuildEnvironment(environment []string, sshClient *sshClient) []string {
	if sshClient == nil {
		return environment
	}
	password := strings.TrimSpace(sshClient.pass)
	if password == "" {
		return environment
	}
	return append(
		environment,
		"LLAMA_CPP_BUILD_PASSWORD="+password,
		"LITERT_LM_BUILD_PASSWORD="+password,
	)
}

func appendLocalLLMBuildArgument(arguments []string, name string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return arguments
	}
	return append(arguments, name, value)
}

func (state *setupFlowState) pruneLocalLLMBuildCaches(cacheRelative string, cacheKey string) error {
	cacheRoot := filepath.Join(state.scriptDir, cacheRelative)
	entries, errorValue := os.ReadDir(cacheRoot)
	if errorValue != nil {
		return nil
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == cacheKey || !strings.HasSuffix(entry.Name(), "-aarch64") {
			continue
		}
		if removeError := os.RemoveAll(filepath.Join(cacheRoot, entry.Name())); removeError != nil {
			return fmt.Errorf("remove old local LLM build cache %s: %w", entry.Name(), removeError)
		}
	}
	return nil
}

func cachedArtifactsPresent(binaryPath, libraryDirectory string) bool {
	binaryInfo, binaryError := os.Stat(binaryPath)
	if binaryError != nil || binaryInfo.Size() == 0 {
		return false
	}
	entries, libraryError := os.ReadDir(libraryDirectory)
	return libraryError == nil && len(entries) > 0
}

func libraryAssetPathsIn(libraryDirectory string) ([]string, error) {
	entries, errorValue := os.ReadDir(libraryDirectory)
	if errorValue != nil {
		return nil, errorValue
	}
	libraryPaths := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		libraryPaths = append(libraryPaths, filepath.Join(libraryDirectory, entry.Name()))
	}
	return libraryPaths, nil
}

func (state *setupFlowState) remoteFileMatchesLocal(remotePath, localPath string) bool {
	remoteHash := strings.TrimSpace(state.sshClient.run("md5sum " + quoteShellValue(remotePath) + " 2>/dev/null | awk '{print $1}'"))
	if remoteHash == "" {
		return false
	}
	localHash := strings.TrimSpace(runCmd("md5", "-q", localPath))
	return remoteHash == localHash
}

func (state *setupFlowState) installDeviceBrowserRuntimeSSH() error {
	fmt.Print("  device browser runtime... ")
	output, errorValue := state.sshClient.runResult(deviceBrowserVersionShellCommand(browserruntime.DeviceBrowserExecutablePath))
	if errorValue != nil {
		fmt.Println("failed")
		diagnostic := strings.TrimSpace(output)
		if diagnostic == "" {
			diagnostic = errorValue.Error()
		}
		return fmt.Errorf("device browser runtime unavailable: %s", diagnostic)
	}
	fmt.Println("installed")
	return nil
}

func deviceBrowserVersionShellCommand(executablePath string) string {
	return quoteShellValue(executablePath) + " version >/tmp/internkim-device-browser-version.log 2>&1"
}

func (state *setupFlowState) ensureAgentBrowserRuntimeSSH() error {
	fmt.Print("  agent-browser runtime... ")
	output, err := state.sshClient.runResult(`
set -eu
if command -v pkill >/dev/null 2>&1; then
  pkill -TERM -x agent-browser >/tmp/internkim-agent-browser-device-close.log 2>&1 || true
  sleep 1
  pkill -KILL -x agent-browser >>/tmp/internkim-agent-browser-device-close.log 2>&1 || true
fi
timeout 5s agent-browser close --all >>/tmp/internkim-agent-browser-device-close.log 2>&1 || true
rm -f /root/.agent-browser/internkim-device.pid /root/.agent-browser/internkim-device.stream /root/.agent-browser/internkim-device.engine /root/.agent-browser/internkim-device.version
rm -f /root/.agent-browser/internkim-device-smoke.pid /root/.agent-browser/internkim-device-smoke.stream /root/.agent-browser/internkim-device-smoke.engine /root/.agent-browser/internkim-device-smoke.version
sleep 1
` + browserruntime.DeviceReadinessShellScript() + `
`)
	if err == nil {
		fmt.Println("ready")
		return nil
	}
	fmt.Println("unavailable")
	diagnostic := strings.TrimSpace(state.sshClient.run(`
{
  echo "agent-browser doctor log:"
  tail -80 /tmp/internkim-agent-browser-doctor.log 2>/dev/null || true
  echo "agent-browser close log:"
  tail -80 /tmp/internkim-agent-browser-device-close.log 2>/dev/null || true
  echo "agent-browser lightpanda open log:"
  tail -80 /tmp/internkim-agent-browser-lightpanda-open.log 2>/dev/null || true
  echo "agent-browser lightpanda snapshot log:"
  tail -80 /tmp/internkim-agent-browser-lightpanda-snapshot.log 2>/dev/null || true
} | tail -120
`))
	if diagnostic == "" {
		diagnostic = strings.TrimSpace(output)
	}
	if diagnostic == "" {
		diagnostic = err.Error()
	}
	state.sshClient.run("mkdir -p /root/.internkim/state && printf '%s' " + quoteShellValue(diagnostic) + " > /root/.internkim/state/agent-browser-unavailable")
	return fmt.Errorf("agent-browser runtime unavailable: %s", diagnostic)
}

func (state *setupFlowState) installBlueclawMigrationsSSH() error {
	migrationPath := filepath.Join(blueclaw.BlueclawSubmoduleRoot(state.scriptDir), "migrations")
	if fileInfo, errorValue := os.Stat(migrationPath); errorValue != nil || !fileInfo.IsDir() {
		return fmt.Errorf("blueclaw migrations missing at %s", migrationPath)
	}
	state.sshClient.run("rm -rf " + quoteShellValue(blueclaw.BlueclawMigrationPath) + " && mkdir -p " + quoteShellValue(blueclaw.BlueclawMigrationPath))
	if errorValue := state.sshClient.scpDir(migrationPath, blueclaw.BlueclawMigrationPath); errorValue != nil {
		return errorValue
	}
	state.sshClient.run("chown -R root:blueclaw " + quoteShellValue(blueclaw.BlueclawMigrationPath) + " && chmod -R u=rwX,g=rX,o= " + quoteShellValue(blueclaw.BlueclawMigrationPath) + " && chmod 750 " + quoteShellValue(blueclaw.BlueclawMigrationPath))
	return nil
}

func (state *setupFlowState) installGraphitiMemorydSSH() error {
	packagePath := filepath.Join(blueclaw.BlueclawSubmoduleRoot(state.scriptDir), "tools", "graphiti_memoryd")
	if fileInfo, errorValue := os.Stat(packagePath); errorValue != nil || !fileInfo.IsDir() {
		return fmt.Errorf("graphiti memory daemon package missing at %s", packagePath)
	}

	archiveFile, errorValue := os.CreateTemp("", "internkim-graphiti-memoryd-*.tar.gz")
	if errorValue != nil {
		return errorValue
	}
	archivePath := archiveFile.Name()
	_ = archiveFile.Close()
	defer os.Remove(archivePath)

	archiveCommand := exec.Command("tar", "-C", packagePath, "-czf", archivePath, ".")
	archiveCommand.Env = append(os.Environ(), "COPYFILE_DISABLE=1")
	if output, archiveError := archiveCommand.CombinedOutput(); archiveError != nil {
		return fmt.Errorf("archive graphiti memory daemon: %s", strings.TrimSpace(string(output)))
	}

	remoteArchivePath := "/tmp/internkim-graphiti-memoryd.tar.gz"
	if errorValue := state.sshClient.scpDirect(archivePath, remoteArchivePath); errorValue != nil {
		return errorValue
	}
	installCommand := "rm -rf " + quoteShellValue(blueclaw.GraphitiMemorydPackagePath) + " && mkdir -p " + quoteShellValue(blueclaw.GraphitiMemorydPackagePath) + " && tar -xzf " + quoteShellValue(remoteArchivePath) + " -C " + quoteShellValue(blueclaw.GraphitiMemorydPackagePath) + " && rm -f " + quoteShellValue(remoteArchivePath)
	if output, installError := state.sshClient.runResult(installCommand); installError != nil {
		return fmt.Errorf("deploy graphiti memory daemon: %s: %w", strings.TrimSpace(output), installError)
	}
	if strings.TrimSpace(state.sshClient.run("test -f "+quoteShellValue(filepath.Join(blueclaw.GraphitiMemorydPackagePath, "__main__.py"))+" && echo ok")) != "ok" {
		return fmt.Errorf("graphiti memory daemon package did not deploy correctly")
	}
	state.sshClient.run(fmt.Sprintf(`cat > %s <<'EOF'
#!/bin/sh
PYTHONPATH=/opt/internkim exec /opt/internkim/graphiti-venv/bin/python -m graphiti_memoryd "$@"
EOF
chmod 755 %s
chown -R root:blueclaw /opt/internkim
chmod -R u=rwX,g=rX,o=rX /opt/internkim`,
		quoteShellValue(blueclaw.GraphitiMemorydPath),
		quoteShellValue(blueclaw.GraphitiMemorydPath),
	))
	return nil
}

func (state *setupFlowState) writeWorkspaceDocumentsSSH(workspaceDocuments workspaceDocuments) {
	state.sshClient.run("cat > /root/.blueclaw/workspace/AGENTS.md <<'EOF'\n" +
		workspaceDocuments.Agents +
		"\nEOF\nchown blueclaw:blueclaw /root/.blueclaw/workspace/AGENTS.md")
	state.sshClient.run("cat > /root/.blueclaw/workspace/IDENTITY.md <<'EOF'\n" +
		workspaceDocuments.Identity +
		"\nEOF\nchown blueclaw:blueclaw /root/.blueclaw/workspace/IDENTITY.md")
	state.sshClient.run("cat > /root/.blueclaw/workspace/BOT_PROFILE.yaml <<'EOF'\n" +
		workspaceDocuments.BotProfile +
		"\nEOF\nrm -f /root/.blueclaw/workspace/BOT_PROFILE.md\nchown blueclaw:blueclaw /root/.blueclaw/workspace/BOT_PROFILE.yaml")
	state.sshClient.run("if [ ! -f /root/.blueclaw/workspace/SOUL.md ] || grep -q '^# IDENTITY.md' /root/.blueclaw/workspace/SOUL.md 2>/dev/null; then cat > /root/.blueclaw/workspace/SOUL.md <<'EOF'\n" +
		workspaceDocuments.Soul +
		"\nEOF\nchown blueclaw:blueclaw /root/.blueclaw/workspace/SOUL.md\nfi")
}

type workspaceDocuments struct {
	Agents     string
	Identity   string
	Soul       string
	BotProfile string
}

func loadWorkspaceDocuments(scriptDir string) (workspaceDocuments, error) {
	agentsContent, errorValue := readWorkspaceMarkdown(blueclawworkspace.AgentsPath(scriptDir))
	if errorValue != nil {
		return workspaceDocuments{}, errorValue
	}
	identityContent, errorValue := readWorkspaceMarkdown(blueclawworkspace.IdentityPath(scriptDir))
	if errorValue != nil {
		return workspaceDocuments{}, errorValue
	}
	soulContent, errorValue := readWorkspaceMarkdown(blueclawworkspace.SoulPath(scriptDir))
	if errorValue != nil {
		return workspaceDocuments{}, errorValue
	}
	botProfileContent, errorValue := readWorkspaceMarkdown(blueclawworkspace.BotProfilePath(scriptDir))
	if errorValue != nil {
		return workspaceDocuments{}, errorValue
	}
	return workspaceDocuments{
		Agents:     agentsContent,
		Identity:   identityContent,
		Soul:       soulContent,
		BotProfile: botProfileContent,
	}, nil
}

func loadWorkspaceAgentsMarkdown(scriptDir string) (string, error) {
	return readWorkspaceMarkdown(blueclawworkspace.AgentsPath(scriptDir))
}

func readWorkspaceMarkdown(path string) (string, error) {
	agentsBytes, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return "", errorValue
	}
	return strings.TrimSpace(string(agentsBytes)), nil
}

func loadAgentBrowserSkillMarkdown(scriptDir string) (string, error) {
	skillPath := blueclawworkspace.AgentBrowserSkillPath(scriptDir)
	skillBytes, err := os.ReadFile(skillPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(skillBytes)), nil
}

func agentBrowserSkillInstallScript(fallbackPath string, fallbackContent string) string {
	return `set -eu
skillDir="/root/.blueclaw/workspace/.agents/skills/agent-browser"
mkdir -p "$skillDir"
cat > ` + quoteShellValue(fallbackPath) + ` <<'EOF'
` + fallbackContent + `
EOF
cp ` + quoteShellValue(fallbackPath) + ` "$skillDir/SKILL.md"
chown -R blueclaw:blueclaw /root/.blueclaw/workspace/.agents 2>/dev/null || true
chown -R root:root "$skillDir" 2>/dev/null || true
chmod -R a+rX,go-w "$skillDir" 2>/dev/null || true`
}

func (state *setupFlowState) stageBinariesSD(context *setup.Context) error {
	assets, err := state.ensureLocalBinaryAssets()
	if err != nil {
		return err
	}

	for _, asset := range assets {
		data, readError := os.ReadFile(asset.localPath)
		if readError != nil {
			return readError
		}
		if writeError := context.SD.WriteFile("bin/"+asset.name, data, 0o755); writeError != nil {
			return writeError
		}
	}
	if err := state.stageGraphitiMemorydSD(context); err != nil {
		return err
	}

	fmt.Printf("  %s\n", state.messenger.t("바이너리 준비 완료", "Binaries staged"))
	return nil
}

func (state *setupFlowState) stageGraphitiMemorydSD(context *setup.Context) error {
	packagePath := filepath.Join(blueclaw.BlueclawSubmoduleRoot(state.scriptDir), "tools", "graphiti_memoryd")
	return filepath.WalkDir(packagePath, func(path string, entry os.DirEntry, walkError error) error {
		if walkError != nil || entry.IsDir() {
			return walkError
		}
		relativePath, relativeError := filepath.Rel(packagePath, path)
		if relativeError != nil {
			return relativeError
		}
		document, readError := os.ReadFile(path)
		if readError != nil {
			return readError
		}
		return context.SD.WriteFile(filepath.Join("graphiti_memoryd", relativePath), document, 0o644)
	})
}

func (state *setupFlowState) ensureFleetRegistration(force bool) error {
	if state.registrationResolved && !force {
		return nil
	}

	state.fleetID = loadOrCreateFleetID(state.stateDir)
	state.nodeID = loadNodeID(state.stateDir)
	state.adminEmail = ""
	if state.parameters.AdminEmail != "" {
		state.adminEmail = state.parameters.AdminEmail
	}
	if state.adminEmail == "" {
		state.adminEmail = strings.TrimSpace(os.Getenv("INTERNKIM_ADMIN_EMAIL"))
	}
	state.tunnelToken = loadState(state.stateDir, "tunnel_token")
	state.nodeTunnelToken = loadState(state.stateDir, "node_tunnel_token")
	state.tlsCertificateStatus = loadState(state.stateDir, "tls_certificate_status")
	state.deviceURL = loadState(state.stateDir, "device_url")
	tunnelOrigin := loadState(state.stateDir, "tunnel_origin")
	tunnelRevision := loadState(state.stateDir, "tunnel_revision")

	fmt.Printf("  %s: %s\n", state.messenger.t("플릿 ID", "Fleet ID"), state.fleetID)
	fmt.Printf("  %s: %s\n", state.messenger.t("노드 ID", "Node ID"), firstNonEmptyString(state.nodeID, "auto"))

	if force || state.tunnelToken == "" || state.nodeTunnelToken == "" || tunnelOrigin != setup.MattermostTunnelOrigin || tunnelRevision != setup.TunnelConfigurationRevision {
		registrationResponse, err := registerFleetNodeWithCollisionRetry(
			state.configuration,
			state.stateDir,
			state.fleetID,
			state.adminEmail,
		)
		if err != nil {
			return err
		}

		state.fleetID = registrationResponse.registeredFleetID()
		state.nodeID = firstNonEmptyString(registrationResponse.registeredNodeID(), state.nodeID)
		state.fleetRole = firstNonEmptyString(registrationResponse.FleetRole, "active")
		state.fleetActiveCount = defaultInt(registrationResponse.FleetActiveCount, 1)
		state.fleetPendingCount = registrationResponse.FleetPendingCount
		state.fleetQuorumSize = defaultInt(registrationResponse.FleetQuorumSize, 1)
		state.tunnelToken = registrationResponse.TunnelToken
		state.nodeTunnelToken = firstNonEmptyString(registrationResponse.NodeTunnelToken, registrationResponse.TunnelToken)
		state.tlsCertificateStatus = registrationResponse.TLSStatus
		state.deviceURL = registrationResponse.publicURL()
		sshHostname := registrationResponse.SSHHostname
		if sshHostname == "" {
			sshHostname = cloudflareSSHHostnameFromDeviceURL(state.deviceURL)
		}

		saveState(state.stateDir, "tunnel_token", state.tunnelToken)
		saveState(state.stateDir, "node_tunnel_token", state.nodeTunnelToken)
		saveState(state.stateDir, "device_url", state.deviceURL)
		saveState(state.stateDir, "ssh_hostname", sshHostname)
		saveState(state.stateDir, "tunnel_origin", setup.MattermostTunnelOrigin)
		saveState(state.stateDir, "tunnel_revision", setup.TunnelConfigurationRevision)
		saveDefaultFleetNode(state.stateDir, registrationResponse)
		if state.adminEmail != "" {
			saveState(state.stateDir, "google_email", state.adminEmail)
		}
	} else {
		fmt.Printf("  %s\n", state.messenger.t("이미 등록됨", "Already registered"))
		state.fleetRole = firstNonEmptyString(loadState(state.stateDir, "fleet_role"), "active")
		state.fleetActiveCount = defaultIntString(loadState(state.stateDir, "fleet_active_count"), 1)
		state.fleetPendingCount = defaultIntString(loadState(state.stateDir, "fleet_pending_count"), 0)
		state.fleetQuorumSize = defaultIntString(loadState(state.stateDir, "fleet_quorum_size"), 1)
		state.nodeTunnelToken = firstNonEmptyString(loadState(state.stateDir, "node_tunnel_token"), state.tunnelToken)
	}

	if state.adminEmail == "" {
		state.adminEmail = loadState(state.stateDir, "claimed_admin_email")
	}

	if state.deviceURL != "" {
		fmt.Printf("  URL: %s\n", state.deviceURL)
	}
	if state.fleetRole != "" {
		fmt.Printf("  %s: %s (%d active, %d pending, quorum %d)\n",
			state.messenger.t("Fleet 역할", "Fleet role"),
			state.fleetRole,
			state.fleetActiveCount,
			state.fleetPendingCount,
			state.fleetQuorumSize,
		)
	}

	state.registrationResolved = true
	return nil
}

func (state *setupFlowState) syncCloudflareAccess(context *setup.Context) error {
	fleetID := strings.TrimSpace(loadState(state.stateDir, "fleet_id"))
	fleetSecret := strings.TrimSpace(loadState(state.stateDir, "fleet_secret"))
	nodeID := strings.TrimSpace(loadState(state.stateDir, "node_id"))
	nodeKey := strings.TrimSpace(loadState(state.stateDir, "node_key"))
	adminEmail := state.cloudflareAccessAdminEmail()
	if fleetID == "" || fleetSecret == "" || nodeID == "" || nodeKey == "" {
		return errors.New("cloudflare access sync requires fleet_id, fleet_secret, node_id, and node_key in local state")
	}
	response, errorValue := registerFleetNode(state.configuration, fleetID, nodeID, nodeKey, fleetSecret, adminEmail)
	if errorValue != nil {
		return errorValue
	}
	saveRegistrationResponse(state.stateDir, response)
	saveDefaultFleetNode(state.stateDir, response)
	if context.Callbacks.SaveState != nil {
		context.Callbacks.SaveState("tunnel_origin", setup.MattermostTunnelOrigin)
		context.Callbacks.SaveState("tunnel_revision", setup.TunnelConfigurationRevision)
	}
	fmt.Printf("  %s\n", state.messenger.t("Cloudflare Access 정책 동기화 완료", "Cloudflare Access policies synced"))
	return nil
}

func (state *setupFlowState) cloudflareAccessAdminEmail() string {
	return firstNonEmptyString(
		state.parameters.AdminEmail,
		strings.TrimSpace(os.Getenv("INTERNKIM_ADMIN_EMAIL")),
		loadState(state.stateDir, "claimed_admin_email"),
		loadState(state.stateDir, "google_email"),
	)
}

func (state *setupFlowState) provisionTunnelSSH(context *setup.Context) error {
	if err := state.ensureFleetRegistration(context.Force); err != nil {
		return err
	}

	state.writeFleetAuthFilesSSH()

	state.sshClient.run(fmt.Sprintf(`mkdir -p /root/.internkim/secrets /root/.internkim/env /root/.internkim/config
printf '%%s' %s > /root/.internkim/secrets/tunnel-token
printf '%%s' %s > /root/.internkim/secrets/node-tunnel-token
chmod 600 /root/.internkim/secrets/tunnel-token
chmod 600 /root/.internkim/secrets/node-tunnel-token
printf '%%s' %s > /root/.internkim/env/device-url
printf '%%s' %s > /root/.internkim/env/mattermost-url
printf '%%s' %s > /root/.internkim/env/tunnel-origin
printf '%%s' %s > /root/.internkim/env/tunnel-revision
printf '%%s' %s > /root/.internkim/config/admin-email
chown root:root /root/.internkim/config/admin-email
chmod 600 /root/.internkim/config/admin-email
chown root:blueclaw /root/.internkim/env/device-url /root/.internkim/env/mattermost-url /root/.internkim/env/tunnel-origin /root/.internkim/env/tunnel-revision
chmod 640 /root/.internkim/env/device-url /root/.internkim/env/mattermost-url /root/.internkim/env/tunnel-origin /root/.internkim/env/tunnel-revision`,
		quoteShellValue(state.tunnelToken),
		quoteShellValue(firstNonEmptyString(state.nodeTunnelToken, state.tunnelToken)),
		quoteShellValue(state.deviceURL),
		quoteShellValue(state.deviceURL),
		quoteShellValue(setup.MattermostTunnelOrigin),
		quoteShellValue(setup.TunnelConfigurationRevision),
		quoteShellValue(state.adminEmail),
	))

	state.sshClient.run(`cat > /etc/systemd/system/cloudflared-node-ssh.service <<'SVCEOF'
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
systemctl stop cloudflared cloudflared-node-ssh 2>/dev/null || true
killall cloudflared 2>/dev/null || true
systemctl daemon-reload
systemctl enable --now cloudflared-node-ssh
for i in $(seq 1 45); do
  [ "$(systemctl is-active cloudflared-node-ssh 2>/dev/null)" = "active" ] && break
  sleep 2
done`)

	if strings.TrimSpace(state.sshClient.run("systemctl is-active cloudflared-node-ssh 2>/dev/null || true")) != "active" {
		return fmt.Errorf("cloudflared node ssh tunnel failed to start")
	}

	if state.isPendingFleetMember() {
		state.sshClient.run(`systemctl disable --now cloudflared 2>/dev/null || true
systemctl daemon-reload`)
		fmt.Printf("  %s\n", state.messenger.t("pending 보드 — public 터널은 닫고 node SSH 터널만 유지", "pending board — public tunnel stays closed, node SSH tunnel stays available"))
		return nil
	}

	state.sshClient.run(`systemctl enable systemd-time-wait-sync.service 2>/dev/null
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
systemctl stop cloudflared 2>/dev/null || true
systemctl daemon-reload
systemctl enable --now cloudflared cloudflared-node-ssh
for i in $(seq 1 45); do
  [ "$(systemctl is-active cloudflared 2>/dev/null)" = "active" ] && break
  sleep 2
done`)

	if strings.TrimSpace(state.sshClient.run("for i in $(seq 1 15); do [ \"$(systemctl is-active cloudflared 2>/dev/null)\" = active ] && [ \"$(systemctl is-active cloudflared-node-ssh 2>/dev/null)\" = active ] && echo active && exit 0; sleep 1; done; echo inactive")) == "active" {
		fmt.Printf("  %s\n", state.messenger.t("cloudflared 실행 중", "cloudflared running"))
		return nil
	}

	return fmt.Errorf("cloudflared failed to start")
}

func (state *setupFlowState) writeFleetAuthFilesSSH() {
	fleetSecret := loadOrCreateFleetSecret(state.stateDir)
	state.sshClient.run(fmt.Sprintf(`mkdir -p /root/.internkim/secrets /root/.internkim/env
printf '%%s' %s > %s
printf '%%s' %s > %s
printf '%%s' %s > %s
printf '%%s' %s > %s
printf '%%s' %s > %s
printf '%%s' %s > %s
printf '%%s' %s > %s
printf '%%s' %s > %s
printf '%%s' %s > /root/.internkim/env/tls-certificate-status
chown root:root %s
chmod 600 %s
chown root:blueclaw %s %s %s %s %s %s %s /root/.internkim/env/tls-certificate-status
chmod 640 %s %s %s %s %s %s %s /root/.internkim/env/tls-certificate-status`,
		quoteShellValue(state.fleetID),
		quoteShellValue(blueclaw.InternKimFleetIDPath),
		quoteShellValue(state.nodeID),
		quoteShellValue(blueclaw.InternKimNodeIDPath),
		quoteShellValue(firstNonEmptyString(state.fleetRole, "active")),
		quoteShellValue(blueclaw.InternKimFleetRolePath),
		quoteShellValue(fmt.Sprint(defaultInt(state.fleetActiveCount, 1))),
		quoteShellValue(blueclaw.InternKimFleetActiveCountPath),
		quoteShellValue(fmt.Sprint(state.fleetPendingCount)),
		quoteShellValue(blueclaw.InternKimFleetPendingCountPath),
		quoteShellValue(fmt.Sprint(defaultInt(state.fleetQuorumSize, 1))),
		quoteShellValue(blueclaw.InternKimFleetQuorumSizePath),
		quoteShellValue(fleetSecret),
		quoteShellValue(blueclaw.InternKimFleetSecretPath),
		quoteShellValue(state.configuration.APIBaseURL),
		quoteShellValue(blueclaw.InternKimAPIURLPath),
		quoteShellValue(state.tlsCertificateStatus),
		quoteShellValue(blueclaw.InternKimFleetSecretPath),
		quoteShellValue(blueclaw.InternKimFleetSecretPath),
		quoteShellValue(blueclaw.InternKimFleetIDPath),
		quoteShellValue(blueclaw.InternKimNodeIDPath),
		quoteShellValue(blueclaw.InternKimFleetRolePath),
		quoteShellValue(blueclaw.InternKimFleetActiveCountPath),
		quoteShellValue(blueclaw.InternKimFleetPendingCountPath),
		quoteShellValue(blueclaw.InternKimFleetQuorumSizePath),
		quoteShellValue(blueclaw.InternKimAPIURLPath),
		quoteShellValue(blueclaw.InternKimFleetIDPath),
		quoteShellValue(blueclaw.InternKimNodeIDPath),
		quoteShellValue(blueclaw.InternKimFleetRolePath),
		quoteShellValue(blueclaw.InternKimFleetActiveCountPath),
		quoteShellValue(blueclaw.InternKimFleetPendingCountPath),
		quoteShellValue(blueclaw.InternKimFleetQuorumSizePath),
		quoteShellValue(blueclaw.InternKimAPIURLPath),
	))
}

func (state *setupFlowState) isPendingFleetMember() bool {
	return strings.TrimSpace(state.fleetRole) == "pending"
}

func defaultInt(value int, fallback int) int {
	if value != 0 {
		return value
	}
	return fallback
}

func defaultIntString(value string, fallback int) int {
	parsedValue, errorValue := strconv.Atoi(strings.TrimSpace(value))
	if errorValue != nil || parsedValue == 0 {
		return fallback
	}
	return parsedValue
}

func (state *setupFlowState) stageTunnelSD(context *setup.Context) error {
	if err := state.ensureFleetRegistration(context.Force); err != nil {
		return err
	}

	if err := context.SD.WriteFile("secrets/tunnel-token", []byte(state.tunnelToken), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("secrets/node-tunnel-token", []byte(firstNonEmptyString(state.nodeTunnelToken, state.tunnelToken)), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("device-url", []byte(state.deviceURL), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("mattermost-url", []byte(state.deviceURL), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("tunnel-origin", []byte(setup.MattermostTunnelOrigin), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("tunnel-revision", []byte(setup.TunnelConfigurationRevision), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("fleet-id", []byte(state.fleetID), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("node-id", []byte(state.nodeID), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("fleet-role", []byte(firstNonEmptyString(state.fleetRole, "active")), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("fleet-active-count", []byte(fmt.Sprint(defaultInt(state.fleetActiveCount, 1))), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("fleet-pending-count", []byte(fmt.Sprint(state.fleetPendingCount)), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("fleet-quorum-size", []byte(fmt.Sprint(defaultInt(state.fleetQuorumSize, 1))), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("api-url", []byte(state.configuration.APIBaseURL), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("tls-certificate-status", []byte(state.tlsCertificateStatus), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("secrets/fleet-secret", []byte(loadOrCreateFleetSecret(state.stateDir)), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("admin-email", []byte(state.adminEmail), 0o644); err != nil {
		return err
	}

	fmt.Printf("  %s\n", state.messenger.t("터널 설정 준비 완료", "Tunnel staged"))
	return nil
}

func (state *setupFlowState) configureSlackTokenSSH(context *setup.Context) error {
	slackBotToken := state.resolveSlackBotToken()
	slackAppToken := state.resolveSlackAppToken()
	signalJSONRPCURL := state.resolveSignalJSONRPCURL()
	signalAccount := state.resolveSignalAccount()
	state.sshClient.run("mkdir -p /root/.internkim/secrets /root/.internkim/config")
	if slackBotToken != "" {
		state.sshClient.run(fmt.Sprintf(`mkdir -p /root/.internkim/secrets
printf '%%s' %s > /root/.internkim/secrets/slack-bot-token
chown root:root /root/.internkim/secrets/slack-bot-token
chmod 600 /root/.internkim/secrets/slack-bot-token`,
			quoteShellValue(slackBotToken),
		))
	}
	if slackAppToken != "" {
		state.sshClient.run(fmt.Sprintf(`printf '%%s' %s > /root/.internkim/secrets/slack-app-token
chown root:root /root/.internkim/secrets/slack-app-token
chmod 600 /root/.internkim/secrets/slack-app-token`,
			quoteShellValue(slackAppToken),
		))
	}
	if signalJSONRPCURL != "" && signalAccount != "" {
		state.sshClient.run(fmt.Sprintf(`printf '%%s' %s > /root/.internkim/config/signal-jsonrpc-url
printf '%%s' %s > /root/.internkim/config/signal-account
chown root:root /root/.internkim/config/signal-jsonrpc-url /root/.internkim/config/signal-account
chmod 600 /root/.internkim/config/signal-jsonrpc-url /root/.internkim/config/signal-account`,
			quoteShellValue(signalJSONRPCURL),
			quoteShellValue(signalAccount),
		))
	}
	if slackBotToken == "" && slackAppToken == "" && (signalJSONRPCURL == "" || signalAccount == "") {
		fmt.Printf("  %s\n", state.messenger.t("플랫폼 connector 추가 설정 없음", "No additional platform connector configuration"))
		return nil
	}
	fmt.Printf("  %s\n", state.messenger.t("플랫폼 connector 설정 저장 완료", "Platform connector settings installed"))
	return nil
}

func (state *setupFlowState) stageSlackTokenSD(context *setup.Context) error {
	slackBotToken := state.resolveSlackBotToken()
	slackAppToken := state.resolveSlackAppToken()
	signalJSONRPCURL := state.resolveSignalJSONRPCURL()
	signalAccount := state.resolveSignalAccount()
	hasConfiguration := false
	if slackBotToken != "" {
		if err := context.SD.WriteFile("secrets/slack-bot-token", []byte(slackBotToken), 0o600); err != nil {
			return err
		}
		hasConfiguration = true
	}
	if slackAppToken != "" {
		if err := context.SD.WriteFile("secrets/slack-app-token", []byte(slackAppToken), 0o600); err != nil {
			return err
		}
		hasConfiguration = true
	}
	if signalJSONRPCURL != "" && signalAccount != "" {
		if err := context.SD.WriteFile("config/signal-jsonrpc-url", []byte(signalJSONRPCURL), 0o600); err != nil {
			return err
		}
		if err := context.SD.WriteFile("config/signal-account", []byte(signalAccount), 0o600); err != nil {
			return err
		}
		hasConfiguration = true
	}
	if !hasConfiguration {
		fmt.Printf("  %s\n", state.messenger.t("플랫폼 connector 추가 설정 없음", "No additional platform connector configuration"))
		return nil
	}
	fmt.Printf("  %s\n", state.messenger.t("플랫폼 connector 설정 준비 완료", "Platform connector settings staged"))
	return nil
}

func (state *setupFlowState) installUsersSyncSSH(context *setup.Context) error {
	if err := state.ensureFleetRegistration(false); err != nil {
		return err
	}
	state.writeFleetAuthFilesSSH()
	if output, errorValue := state.sshClient.runResult(usersSyncDependencyInstallScript()); errorValue != nil {
		return fmt.Errorf("install users sync dependencies failed: %s", strings.TrimSpace(output))
	}
	if output, errorValue := state.sshClient.runResult(fmt.Sprintf(`cat > %s <<'SYNCEOF'
%sSYNCEOF
chmod 755 %s
cat > %s <<'SERVICEEOF'
%sSERVICEEOF
cat > %s <<'TIMEREOF'
%sTIMEREOF
cat > %s <<'CLEANEOF'
%sCLEANEOF
chmod 755 %s
cat > %s <<'CLEANSERVICEEOF'
%sCLEANSERVICEEOF
cat > %s <<'CLEANTIMEREOF'
%sCLEANTIMEREOF
chown root:blueclaw /root/.blueclaw/config 2>/dev/null || true
chmod 770 /root/.blueclaw/config 2>/dev/null || true
systemctl daemon-reload
systemctl enable --now internkim-users-sync.timer
systemctl enable --now internkim-blueclaw-tmp-clean.timer
systemctl start internkim-users-sync.service || journalctl -u internkim-users-sync -n 40 --no-pager`,
		quoteShellValue(blueclaw.InternKimUsersSyncScriptPath),
		blueclaw.InternKimUsersSyncScript(),
		quoteShellValue(blueclaw.InternKimUsersSyncScriptPath),
		quoteShellValue(blueclaw.InternKimUsersSyncServicePath),
		blueclaw.InternKimUsersSyncServiceUnit(),
		quoteShellValue(blueclaw.InternKimUsersSyncTimerPath),
		blueclaw.InternKimUsersSyncTimerUnit(),
		quoteShellValue(blueclaw.InternKimBlueclawTemporaryCleanupScriptPath),
		blueclaw.InternKimBlueclawTemporaryCleanupScript(),
		quoteShellValue(blueclaw.InternKimBlueclawTemporaryCleanupScriptPath),
		quoteShellValue(blueclaw.InternKimBlueclawTemporaryCleanupServicePath),
		blueclaw.InternKimBlueclawTemporaryCleanupServiceUnit(),
		quoteShellValue(blueclaw.InternKimBlueclawTemporaryCleanupTimerPath),
		blueclaw.InternKimBlueclawTemporaryCleanupTimerUnit(),
	)); errorValue != nil {
		return fmt.Errorf("install users sync service failed: %s", strings.TrimSpace(output))
	}
	fmt.Printf("  %s\n", state.messenger.t("사용자 동기화 타이머 설치 완료", "Users sync timer installed"))
	return nil
}

func (state *setupFlowState) stageUsersSyncSD(context *setup.Context) error {
	fmt.Printf("  %s\n", state.messenger.t("사용자 동기화는 첫 부팅 서비스 단계에서 설치됩니다", "Users sync is installed during first boot services"))
	return nil
}

func (state *setupFlowState) stageBootstrapSD(context *setup.Context) error {
	if err := state.ensureFleetRegistration(false); err != nil {
		return err
	}
	if err := state.stageSlackTokenSD(context); err != nil {
		return err
	}

	if state.publicKey != "" {
		if err := context.SD.WriteFile("authorized_keys", []byte(state.publicKey+"\n"), 0o644); err != nil {
			return err
		}
	}

	runtimeConfigurationOptions, err := blueclaw.BlueclawRuntimeConfigOptionsFromEnvironment()
	if err != nil {
		return err
	}
	runtimeConfiguration, err := blueclaw.BlueclawRuntimeConfigDocumentWithOptions(runtimeConfigurationOptions)
	if err != nil {
		return err
	}
	if err := context.SD.WriteFile("config/runtime.json", []byte(runtimeConfiguration), 0o644); err != nil {
		return err
	}

	policyConfiguration, err := blueclaw.BlueclawPolicyDocument(state.adminEmail)
	if err != nil {
		return err
	}
	if err := context.SD.WriteFile("config/policy.json", []byte(policyConfiguration), 0o644); err != nil {
		return err
	}

	workspaceDocuments, err := loadWorkspaceDocuments(state.scriptDir)
	if err != nil {
		return err
	}
	if err := context.SD.WriteFile("AGENTS.md", []byte(workspaceDocuments.Agents), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("IDENTITY.md", []byte(workspaceDocuments.Identity), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("SOUL.md", []byte(workspaceDocuments.Soul), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("BOT_PROFILE.yaml", []byte(workspaceDocuments.BotProfile), 0o644); err != nil {
		return err
	}
	botProfileImageDocument, err := os.ReadFile(filepath.Join(state.scriptDir, "assets", "internkim.png"))
	if err != nil {
		return err
	}
	if err := context.SD.WriteFile("assets/internkim.png", botProfileImageDocument, 0o644); err != nil {
		return err
	}
	agentBrowserSkillMarkdown, err := loadAgentBrowserSkillMarkdown(state.scriptDir)
	if err != nil {
		return err
	}
	if err := context.SD.WriteFile(".agents/skills/agent-browser/SKILL.md", []byte(agentBrowserSkillMarkdown), 0o644); err != nil {
		return err
	}
	if err := state.stageBlueclawMigrationsSD(context.SD.RootPath()); err != nil {
		return err
	}

	adminPassword := state.restorePasswordFromBackup("mm-admin-pass")
	if adminPassword == "" {
		adminPassword = generatePassword(20)
	}
	if err := context.SD.WriteFile("secrets/mm-admin-pass", []byte(adminPassword), 0o644); err != nil {
		return err
	}

	databasePassword := state.restorePasswordFromBackup("mm-db-pass")
	if databasePassword == "" {
		databasePassword = generatePassword(20)
	}
	if err := context.SD.WriteFile("secrets/mm-db-pass", []byte(databasePassword), 0o644); err != nil {
		return err
	}

	if err := context.SD.WriteFile(
		"internkim-firstboot.sh",
		[]byte(generateFirstbootScript(state.deviceURL, state.adminEmail)),
		0o755,
	); err != nil {
		return err
	}

	localSkillsPath := blueclawworkspace.SkillsPath(state.scriptDir)
	if info, err := os.Stat(localSkillsPath); err == nil && info.IsDir() {
		if err := copyDirectoryContents(localSkillsPath, filepath.Join(context.SD.RootPath(), "skills")); err != nil {
			return err
		}
	}
	localToolsPath := blueclawworkspace.ToolsPath(state.scriptDir)
	if info, err := os.Stat(localToolsPath); err == nil && info.IsDir() {
		if err := copyDirectoryContents(localToolsPath, filepath.Join(context.SD.RootPath(), "tools")); err != nil {
			return err
		}
	}

	if state.setupBuildID != "" {
		if err := context.SD.WriteFile("setup-build-id", []byte(state.setupBuildID), 0o644); err != nil {
			return err
		}
	}

	if err := state.stageBackupArtifacts(context.SD.RootPath()); err != nil {
		return err
	}

	fmt.Printf("  %s\n", state.messenger.t("부팅 스테이지 준비 완료", "Boot staging prepared"))
	return nil
}

func (state *setupFlowState) restorePasswordFromBackup(name string) string {
	passwordPath := filepath.Join(state.stateDir, "backup", name)
	passwordBytes, err := os.ReadFile(passwordPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(passwordBytes))
}

func (state *setupFlowState) stageBlueclawMigrationsSD(stageRoot string) error {
	sourcePath := filepath.Join(blueclaw.BlueclawSubmoduleRoot(state.scriptDir), "migrations")
	targetPath := filepath.Join(stageRoot, "blueclaw-migrations")
	fileInfo, errorValue := os.Stat(sourcePath)
	if errorValue != nil || !fileInfo.IsDir() {
		return fmt.Errorf("blueclaw migrations missing at %s", sourcePath)
	}
	_ = os.RemoveAll(targetPath)
	return copyDirectoryContents(sourcePath, targetPath)
}

func (state *setupFlowState) stageBackupArtifacts(stageRoot string) error {
	workspaceTarget := filepath.Join(stageRoot, "workspace-restore")
	databaseTarget := filepath.Join(stageRoot, "mattermost-db.sql")
	blueclawDatabaseTarget := filepath.Join(stageRoot, "blueclaw-db.sql")

	_ = os.RemoveAll(workspaceTarget)
	_ = os.Remove(databaseTarget)
	_ = os.Remove(blueclawDatabaseTarget)

	workspaceSource := filepath.Join(state.stateDir, "backup", "workspace")
	if info, err := os.Stat(workspaceSource); err == nil && info.IsDir() {
		if err := copyDirectoryContents(workspaceSource, workspaceTarget); err != nil {
			return err
		}
		fmt.Printf("  %s\n", state.messenger.t("워크스페이스 백업 복원", "Workspace restored from backup"))
	}

	databaseSource := filepath.Join(state.stateDir, "backup", "mattermost-db.sql")
	if info, err := os.Stat(databaseSource); err == nil && info.Mode().IsRegular() {
		if err := copyRegularFile(databaseSource, databaseTarget, 0o644); err != nil {
			return err
		}
		fmt.Printf("  %s\n", state.messenger.t("DB 백업 복원", "DB backup restored"))
	}
	blueclawDatabaseSource := filepath.Join(state.stateDir, "backup", "blueclaw-db.sql")
	if info, err := os.Stat(blueclawDatabaseSource); err == nil && info.Mode().IsRegular() {
		if err := copyRegularFile(blueclawDatabaseSource, blueclawDatabaseTarget, 0o644); err != nil {
			return err
		}
		fmt.Printf("  %s\n", state.messenger.t("Blueclaw DB 백업 복원", "Blueclaw DB backup restored"))
	}

	return nil
}

func (state *setupFlowState) pruneRemoteLocalLLMBuildCaches() error {
	if state.sshClient == nil {
		return nil
	}
	remoteCachePaths := []string{
		"/var/cache/internkim/litert-lm-build",
		"/var/cache/internkim/llama-cpp-build",
	}
	removeCommandParts := make([]string, 0, len(remoteCachePaths)+1)
	removeCommandParts = append(removeCommandParts, "rm -rf")
	for _, remoteCachePath := range remoteCachePaths {
		removeCommandParts = append(removeCommandParts, quoteShellValue(remoteCachePath))
	}
	if _, errorValue := state.sshClient.runResult(strings.Join(removeCommandParts, " ")); errorValue != nil {
		return fmt.Errorf("remove remote local LLM build caches: %w", errorValue)
	}
	return nil
}

func copyDirectoryContents(sourceRoot string, targetRoot string) error {
	return filepath.Walk(sourceRoot, func(sourcePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return err
		}

		targetPath := targetRoot
		if relativePath != "." {
			targetPath = filepath.Join(targetRoot, relativePath)
		}

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		return copyRegularFile(sourcePath, targetPath, info.Mode().Perm())
	})
}

func copyRegularFile(sourcePath string, targetPath string, mode os.FileMode) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		targetFile.Close()
		return err
	}
	if err := targetFile.Close(); err != nil {
		return err
	}

	return os.Chmod(targetPath, mode)
}

func quoteShellValue(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
