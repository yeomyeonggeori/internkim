package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	deviceID             string
	adminEmail           string
	deviceURL            string
	tunnelToken          string
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

const deviceBrowserRuntimeArtifactName = "internkim-device-browser-linux-arm64.tar.zst"

func deviceBrowserRuntimePackageListUbuntu24() string {
	return strings.Join([]string{
		"zstd",
		"fonts-noto-color-emoji",
		"libfontconfig1",
		"libfreetype6",
		"libasound2t64",
		"libatk-bridge2.0-0t64",
		"libatk1.0-0t64",
		"libatspi2.0-0t64",
		"libcairo2",
		"libcups2t64",
		"libdbus-1-3",
		"libdrm2",
		"libgbm1",
		"libglib2.0-0t64",
		"libnspr4",
		"libnss3",
		"libpango-1.0-0",
		"libx11-6",
		"libxcb1",
		"libxcomposite1",
		"libxdamage1",
		"libxext6",
		"libxfixes3",
		"libxkbcommon0",
		"libxrandr2",
	}, " ")
}

func deviceBrowserRuntimePackageListLegacyUbuntu() string {
	return strings.Join([]string{
		"zstd",
		"fonts-noto-color-emoji",
		"libfontconfig1",
		"libfreetype6",
		"libasound2",
		"libatk-bridge2.0-0",
		"libatk1.0-0",
		"libatspi2.0-0",
		"libcairo2",
		"libcups2",
		"libdbus-1-3",
		"libdrm2",
		"libgbm1",
		"libglib2.0-0",
		"libnspr4",
		"libnss3",
		"libpango-1.0-0",
		"libwayland-client0",
		"libx11-6",
		"libxcb1",
		"libxcomposite1",
		"libxdamage1",
		"libxext6",
		"libxfixes3",
		"libxkbcommon0",
		"libxrandr2",
	}, " ")
}

func deviceBrowserRuntimePackageListJetPack6() string {
	return deviceBrowserRuntimePackageListLegacyUbuntu()
}

func deviceBrowserRuntimeOptionalPackageListUbuntu24() string {
	return strings.Join([]string{
		"fonts-liberation",
		"fonts-freefont-ttf",
		"fonts-noto-cjk",
	}, " ")
}

func deviceBrowserRuntimeOptionalPackageListLegacyUbuntu() string {
	return strings.Join([]string{
		"fonts-liberation",
		"fonts-freefont-ttf",
		"fonts-noto-cjk",
	}, " ")
}

func deviceBrowserRuntimeOptionalPackageListJetPack6() string {
	return deviceBrowserRuntimeOptionalPackageListLegacyUbuntu()
}

func deviceBrowserRuntimePackageSelectionScript() string {
	return fmt.Sprintf(`runtimePackages=%s
optionalRuntimePackages=%s
if [ -f /etc/os-release ]; then
  . /etc/os-release
  case "${VERSION_ID:-}" in
    22.*) runtimePackages=%s; optionalRuntimePackages=%s ;;
    24.*|25.*|26.*) runtimePackages=%s; optionalRuntimePackages=%s ;;
    *) runtimePackages=%s; optionalRuntimePackages=%s ;;
  esac
fi`,
		quoteShellValue(deviceBrowserRuntimePackageListLegacyUbuntu()),
		quoteShellValue(deviceBrowserRuntimeOptionalPackageListLegacyUbuntu()),
		quoteShellValue(deviceBrowserRuntimePackageListJetPack6()),
		quoteShellValue(deviceBrowserRuntimeOptionalPackageListJetPack6()),
		quoteShellValue(deviceBrowserRuntimePackageListUbuntu24()),
		quoteShellValue(deviceBrowserRuntimeOptionalPackageListUbuntu24()),
		quoteShellValue(deviceBrowserRuntimePackageListLegacyUbuntu()),
		quoteShellValue(deviceBrowserRuntimeOptionalPackageListLegacyUbuntu()),
	)
}

func deviceBrowserRuntimeDependencyInstallScript() string {
	return strings.TrimSpace(`apt-get update -qq >/dev/null 2>&1 || true
` + deviceBrowserRuntimePackageSelectionScript() + `
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq $runtimePackages >/dev/null
installableOptionalPackages=""
for packageName in $optionalRuntimePackages; do
  if apt-cache show "$packageName" >/dev/null 2>&1; then
    installableOptionalPackages="$installableOptionalPackages $packageName"
  fi
done
if [ -n "$installableOptionalPackages" ]; then
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq $installableOptionalPackages >/dev/null || true
fi`)
}

func usersSyncDependencyInstallScript() string {
	return strings.TrimSpace(`if ! command -v jq >/dev/null 2>&1 || ! command -v curl >/dev/null 2>&1; then
  apt-get update -qq >/dev/null 2>&1 || true
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq jq curl ca-certificates >/dev/null
fi
command -v jq >/dev/null 2>&1
command -v curl >/dev/null 2>&1`)
}

func deviceBrowserRuntimeExtractScript() string {
	return strings.TrimSpace(`install -d -m 755 /opt/internkim
temporaryDirectory="$(mktemp -d /opt/internkim/device-browser.next.XXXXXX)"
cleanup() {
  rm -rf "$temporaryDirectory"
}
trap cleanup EXIT
tar --use-compress-program=zstd -xf "$archivePath" -C "$temporaryDirectory"
manifestPath="$temporaryDirectory/manifest.json"
test -f "$manifestPath"
executableRelativePath="$(python3 -c 'import json, sys; print(json.load(open(sys.argv[1])).get("executablePath", ""))' "$manifestPath")"
test -n "$executableRelativePath"
test -x "$temporaryDirectory/$executableRelativePath"
ln -sfn "$executableRelativePath" "$temporaryDirectory/chromium"
"$temporaryDirectory/chromium" --version >/tmp/internkim-device-browser-version.log 2>&1
chmod -R a+rX "$temporaryDirectory"
rm -rf /opt/internkim/device-browser.previous
if [ -d /opt/internkim/device-browser ]; then
  mv /opt/internkim/device-browser /opt/internkim/device-browser.previous
fi
mv "$temporaryDirectory" /opt/internkim/device-browser
trap - EXIT
rm -rf /opt/internkim/device-browser.previous`)
}

func deviceBrowserRuntimeInstallScript(remoteArchivePath string) string {
	return fmt.Sprintf(`set -eu
archivePath=%s
%s
%s
rm -f "$archivePath"`,
		quoteShellValue(remoteArchivePath),
		deviceBrowserRuntimeDependencyInstallScript(),
		deviceBrowserRuntimeExtractScript(),
	)
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
		deviceID:       loadOrCreateDeviceID(stateDir),
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
		StageBinariesSD:           state.stageBinariesSD,
		InstallBlueclawRuntimeSSH: state.installBlueclawRuntimeSSH,
		AdminWebVersion:           state.adminWebVersion,
		DeployAdminWeb:            state.deployAdminWeb,
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

	setupCommand := fmt.Sprintf(`set -eu
nmcli radio wifi on
%s
cat > /usr/local/bin/internkim-wifi-select <<'WIFIEOF'
%s
WIFIEOF
chmod 755 /usr/local/bin/internkim-wifi-select
/usr/local/bin/internkim-wifi-select >/dev/null 2>&1 || true
for attempt in $(seq 1 12); do
  address="$(ip -4 addr show 2>/dev/null | awk '/inet / && $0 !~ / lo / {print $2; exit}' | cut -d/ -f1)"
  if [ -n "$address" ]; then
    echo "$address"
    exit 0
  fi
  sleep 5
done
exit 1`, buildJetsonWiFiUpsertScript(state.wifiProfiles), buildJetsonWiFiSelectorScript())

	output, errorValue := state.sshClient.runResult(setupCommand)
	if errorValue != nil {
		return fmt.Errorf("Jetson NetworkManager Wi-Fi connection failed: %s", strings.TrimSpace(output))
	}
	if address := strings.TrimSpace(output); address != "" {
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
	buildCommand := exec.Command("go", "build", "-o", asset.localPath, "./cmd/"+asset.name+"/")
	buildCommand.Dir = state.scriptDir
	buildCommand.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64")
	if output, buildError := buildCommand.CombinedOutput(); buildError != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("build %s: %s", asset.name, strings.TrimSpace(string(output)))
	}
	fmt.Println("ok")
	return nil
}

func (state *setupFlowState) binariesVersion() string {
	hash := sha256.New()
	for _, path := range []string{
		filepath.Join(state.scriptDir, "cmd", blueclaw.CapabilitydName),
		filepath.Join(state.scriptDir, "cmd", blueclaw.AdmindName),
		filepath.Join(state.scriptDir, "cmd", blueclaw.LocalLLMRunnerName),
		filepath.Join(state.scriptDir, "internal", "admind"),
		filepath.Join(state.scriptDir, "internal", "capabilityd"),
		filepath.Join(state.scriptDir, "internal", "runtime", "blueclaw"),
		filepath.Join(blueclaw.BlueclawSubmoduleRoot(state.scriptDir), "tools", "graphiti_memoryd"),
		filepath.Join(blueclaw.BlueclawSubmoduleRoot(state.scriptDir), "tools", "graphiti-memoryd"),
	} {
		state.writeDirectoryHash(hash, path)
	}
	blueclawRevision := strings.TrimSpace(runCmd("git", "-C", blueclaw.BlueclawSubmoduleRoot(state.scriptDir), "rev-parse", "HEAD"))
	_, _ = hash.Write([]byte("blueclaw:" + blueclawRevision + "\n"))
	return hex.EncodeToString(hash.Sum(nil))
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
		if errorValue := state.runAdminWebCommand(webRoot, "bunx", "wrangler", "pages", "deploy", ".svelte-kit/cloudflare", "--project-name", "internkim"); errorValue != nil {
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
			temporaryAdminUIPath := "/tmp/internkim-admin-ui"
			state.sshClient.run("rm -rf " + quoteShellValue(temporaryAdminUIPath) + " && mkdir -p " + quoteShellValue(temporaryAdminUIPath) + " && chmod 777 " + quoteShellValue(temporaryAdminUIPath))
			if errorValue := state.sshClient.scpDirDirect(boardUIPath, temporaryAdminUIPath); errorValue != nil {
				return errorValue
			}
			if output, errorValue := state.sshClient.runResult("rm -rf /opt/internkim/admin-ui && mkdir -p /opt/internkim/admin-ui && cp -a " + quoteShellValue(temporaryAdminUIPath) + "/. /opt/internkim/admin-ui/ && chmod -R a+rX /opt/internkim/admin-ui"); errorValue != nil {
				return fmt.Errorf("deploy web UI: %s: %w", strings.TrimSpace(output), errorValue)
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

	if err := state.installLocalLLMSSH(); err != nil {
		return err
	}

	if err := state.ensureAgentBrowserRuntimeSSH(); err != nil {
		return err
	}

	state.sshClient.run(`
NOLOGIN_BIN=$(command -v nologin || echo /usr/sbin/nologin)
getent group blueclaw >/dev/null 2>&1 || groupadd --system blueclaw
id blueclaw &>/dev/null || useradd -r -g blueclaw -m -d /home/blueclaw -s "$NOLOGIN_BIN" blueclaw
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
chmod 755 /root/.blueclaw/workspace/bin /root/.blueclaw/workspace/downloads`)

	if err := state.installBlueclawMigrationsSSH(); err != nil {
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

	skillsDir := blueclawworkspace.SkillsPath(state.scriptDir)
	if info, err := os.Stat(skillsDir); err == nil && info.IsDir() {
		state.sshClient.run("mkdir -p " + quoteShellValue(filepath.Join(blueclaw.BlueclawWorkspacePath, "skills")))
		entries, err := os.ReadDir(skillsDir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			remoteSkillDir := blueclaw.BlueclawWorkspaceSkillPath(entry.Name())
			state.sshClient.run("rm -rf " + quoteShellValue(remoteSkillDir))
			state.sshClient.run("mkdir -p " + quoteShellValue(remoteSkillDir))
			if err := state.sshClient.scpDir(filepath.Join(skillsDir, entry.Name()), remoteSkillDir); err != nil {
				return err
			}
		}
		state.sshClient.run(`chown -R blueclaw:blueclaw /root/.blueclaw/workspace/skills 2>/dev/null || true`)
	}

	return nil
}

func (state *setupFlowState) installBlueclawRuntimeSSH(context *setup.Context) error {
	artifactDirectoryPath := filepath.Join(state.scriptDir, blueclaw.BlueclawRuntimeArtifactPath)
	manifest, errorValue := blueclaw.ValidateRuntimeArtifactDirectory(artifactDirectoryPath)
	if errorValue != nil {
		return fmt.Errorf("blueclaw Firecracker runtime artifact invalid: %w; run `make prepare-blueclaw-runtime` before setup", errorValue)
	}

	requiredArtifacts := []struct {
		name       string
		remotePath string
		mode       string
	}{
		{name: "firecracker", remotePath: blueclaw.BlueclawFirecrackerPath, mode: "0755"},
		{name: "jailer", remotePath: blueclaw.BlueclawJailerPath, mode: "0755"},
		{name: "vmlinux.bin", remotePath: blueclaw.BlueclawKernelImagePath, mode: "0644"},
		{name: "rootfs.ext4", remotePath: blueclaw.BlueclawRootFilesystemImagePath, mode: "0644"},
	}

	fmt.Print("  blueclaw Firecracker runtime... ")
	state.sshClient.run("rm -rf /tmp/internkim-blueclaw-runtime && mkdir -p /tmp/internkim-blueclaw-runtime/runtime " + blueclaw.BlueclawRuntimeInstallPath + " /var/lib/blueclaw /var/log/blueclaw-supervisor")
	for _, artifact := range requiredArtifacts {
		localArtifactPath, errorValue := blueclaw.RuntimeArtifactFilePath(artifactDirectoryPath, manifest, artifact.name)
		if errorValue != nil {
			fmt.Println("failed")
			return errorValue
		}
		temporaryRemotePath := "/tmp/internkim-blueclaw-runtime/runtime/" + artifact.name
		if errorValue := state.sshClient.scp(localArtifactPath, temporaryRemotePath); errorValue != nil {
			fmt.Println("failed")
			return errorValue
		}
		installCommand := "install -m " + artifact.mode + " " + quoteShellValue(temporaryRemotePath) + " " + quoteShellValue(artifact.remotePath)
		if artifact.name == "rootfs.ext4" {
			installCommand = "cp --sparse=always " + quoteShellValue(temporaryRemotePath) + " " + quoteShellValue(artifact.remotePath) + " && chmod " + artifact.mode + " " + quoteShellValue(artifact.remotePath)
		}
		output, errorValue := state.sshClient.runResult(installCommand)
		if errorValue != nil {
			fmt.Println("failed")
			return fmt.Errorf("install %s: %s: %w", artifact.name, strings.TrimSpace(output), errorValue)
		}
	}
	if errorValue := state.sshClient.scp(filepath.Join(artifactDirectoryPath, "manifest.json"), "/tmp/internkim-blueclaw-runtime/runtime/manifest.json"); errorValue != nil {
		fmt.Println("failed")
		return errorValue
	}
	output, errorValue := state.sshClient.runResult(`set -eu
test -x /usr/local/bin/blueclaw-supervisor
test -x /usr/local/bin/firecracker
test -x /usr/local/bin/jailer
test -s /opt/internkim/blueclaw-runtime/vmlinux.bin
test -s /opt/internkim/blueclaw-runtime/rootfs.ext4
install -m 0644 /tmp/internkim-blueclaw-runtime/runtime/manifest.json /opt/internkim/blueclaw-runtime/manifest.json
if [ ! -e /var/lib/blueclaw/workspace.ext4 ]; then
  truncate -s 16G /var/lib/blueclaw/workspace.ext4
fi
if ! blkid -o value -s TYPE /var/lib/blueclaw/workspace.ext4 2>/dev/null | grep -qx ext4; then
  mkfs.ext4 -F -L blueclaw-workspace /var/lib/blueclaw/workspace.ext4 >/dev/null
fi
chmod 0600 /var/lib/blueclaw/workspace.ext4
mkdir -p /var/log/blueclaw-supervisor
`)
	if errorValue != nil {
		fmt.Println("failed")
		return fmt.Errorf("verify blueclaw Firecracker runtime: %s: %w", strings.TrimSpace(output), errorValue)
	}
	fmt.Println("installed")
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

func (state *setupFlowState) deviceBrowserRuntimeArtifactPath() string {
	return filepath.Join(state.scriptDir, ".dependency", "device-browser", deviceBrowserRuntimeArtifactName)
}

func (state *setupFlowState) ensureDeviceBrowserRuntimeArtifact() (string, error) {
	artifactPath := state.deviceBrowserRuntimeArtifactPath()
	fileInfo, errorValue := os.Stat(artifactPath)
	if errorValue == nil && !fileInfo.IsDir() && fileInfo.Size() > 0 {
		return artifactPath, nil
	}
	command := exec.Command("make", "prepare-device-browser")
	command.Dir = state.scriptDir
	output, prepareError := command.CombinedOutput()
	if prepareError != nil {
		return "", fmt.Errorf("device browser runtime artifact missing at %s and automatic preparation failed: %s; run `make prepare-device-browser` before setup", artifactPath, strings.TrimSpace(string(output)))
	}
	fileInfo, errorValue = os.Stat(artifactPath)
	if errorValue == nil && !fileInfo.IsDir() && fileInfo.Size() > 0 {
		return artifactPath, nil
	}
	return "", fmt.Errorf("device browser runtime preparation did not create %s; run `make prepare-device-browser` before setup", artifactPath)
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
	artifactPath, err := state.ensureDeviceBrowserRuntimeArtifact()
	if err != nil {
		return err
	}
	fmt.Print("  device browser runtime... ")
	remoteArchivePath := "/tmp/" + deviceBrowserRuntimeArtifactName
	if err := state.sshClient.scp(artifactPath, remoteArchivePath); err != nil {
		return err
	}
	output, errorValue := state.sshClient.runResult(deviceBrowserRuntimeInstallScript(remoteArchivePath))
	if errorValue != nil {
		fmt.Println("failed")
		diagnostic := strings.TrimSpace(output)
		if diagnostic == "" {
			diagnostic = errorValue.Error()
		}
		return fmt.Errorf("device browser runtime install failed: %s", diagnostic)
	}
	fmt.Println("installed")
	return nil
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
  echo "agent-browser chrome open log:"
  tail -80 /tmp/internkim-agent-browser-chrome-open.log 2>/dev/null || true
  echo "agent-browser chrome snapshot log:"
  tail -80 /tmp/internkim-agent-browser-chrome-snapshot.log 2>/dev/null || true
  echo "agent-browser chrome screenshot log:"
  tail -80 /tmp/internkim-agent-browser-chrome-screenshot.log 2>/dev/null || true
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
chown -R blueclaw:blueclaw /root/.blueclaw/workspace/.agents 2>/dev/null || true`
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
	if err := state.stageDeviceBrowserRuntimeSD(context); err != nil {
		return err
	}

	fmt.Printf("  %s\n", state.messenger.t("바이너리 준비 완료", "Binaries staged"))
	return nil
}

func (state *setupFlowState) stageDeviceBrowserRuntimeSD(context *setup.Context) error {
	artifactPath, err := state.ensureDeviceBrowserRuntimeArtifact()
	if err != nil {
		return err
	}
	document, errorValue := os.ReadFile(artifactPath)
	if errorValue != nil {
		return errorValue
	}
	return context.SD.WriteFile(filepath.Join("device-browser", deviceBrowserRuntimeArtifactName), document, 0o644)
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

func (state *setupFlowState) ensureDeviceRegistration(force bool) error {
	if state.registrationResolved && !force {
		return nil
	}

	state.deviceID = loadOrCreateDeviceID(state.stateDir)
	state.adminEmail = ""
	if state.parameters.AdminEmail != "" {
		state.adminEmail = state.parameters.AdminEmail
	}
	if state.adminEmail == "" {
		state.adminEmail = strings.TrimSpace(os.Getenv("INTERNKIM_ADMIN_EMAIL"))
	}
	state.tunnelToken = loadState(state.stateDir, "tunnel_token")
	state.deviceURL = loadState(state.stateDir, "device_url")
	tunnelOrigin := loadState(state.stateDir, "tunnel_origin")
	tunnelRevision := loadState(state.stateDir, "tunnel_revision")

	fmt.Printf("  %s: %s\n", state.messenger.t("기기 ID", "Device ID"), state.deviceID)

	if force || state.tunnelToken == "" || tunnelOrigin != setup.MattermostTunnelOrigin || tunnelRevision != setup.TunnelConfigurationRevision {
		registrationResponse, err := registerDeviceWithCollisionRetry(
			state.configuration,
			state.stateDir,
			state.deviceID,
			state.adminEmail,
		)
		if err != nil {
			return err
		}

		state.deviceID = registrationResponse.DeviceID
		state.tunnelToken = registrationResponse.TunnelToken
		state.deviceURL = registrationResponse.publicURL()

		saveState(state.stateDir, "tunnel_token", state.tunnelToken)
		saveState(state.stateDir, "device_url", state.deviceURL)
		saveState(state.stateDir, "tunnel_origin", setup.MattermostTunnelOrigin)
		saveState(state.stateDir, "tunnel_revision", setup.TunnelConfigurationRevision)
		if state.adminEmail != "" {
			saveState(state.stateDir, "google_email", state.adminEmail)
		}
	} else {
		fmt.Printf("  %s\n", state.messenger.t("이미 등록됨", "Already registered"))
	}

	if state.adminEmail == "" {
		state.adminEmail = loadState(state.stateDir, "claimed_admin_email")
	}

	if state.deviceURL != "" {
		fmt.Printf("  URL: %s\n", state.deviceURL)
	}

	state.registrationResolved = true
	return nil
}

func (state *setupFlowState) provisionTunnelSSH(context *setup.Context) error {
	if err := state.ensureDeviceRegistration(context.Force); err != nil {
		return err
	}

	state.writeDeviceAuthFilesSSH()

	state.sshClient.run(fmt.Sprintf(`mkdir -p /root/.internkim/secrets /root/.internkim/env
printf '%%s' %s > /root/.internkim/secrets/tunnel-token
chmod 600 /root/.internkim/secrets/tunnel-token
printf '%%s' %s > /root/.internkim/env/device-url
printf '%%s' %s > /root/.internkim/env/mattermost-url
printf '%%s' %s > /root/.internkim/env/tunnel-origin
printf '%%s' %s > /root/.internkim/env/tunnel-revision
printf '%%s' %s > /root/.internkim/admin-email
chmod 644 /root/.internkim/admin-email
chown root:blueclaw /root/.internkim/env/device-url /root/.internkim/env/mattermost-url /root/.internkim/env/tunnel-origin /root/.internkim/env/tunnel-revision
chmod 640 /root/.internkim/env/device-url /root/.internkim/env/mattermost-url /root/.internkim/env/tunnel-origin /root/.internkim/env/tunnel-revision`,
		quoteShellValue(state.tunnelToken),
		quoteShellValue(state.deviceURL),
		quoteShellValue(state.deviceURL),
		quoteShellValue(setup.MattermostTunnelOrigin),
		quoteShellValue(setup.TunnelConfigurationRevision),
		quoteShellValue(state.adminEmail),
	))

	state.sshClient.run(`systemctl enable systemd-time-wait-sync.service 2>/dev/null
cat > /etc/systemd/system/cloudflared.service <<'SVCEOF'
[Unit]
Description=Cloudflare Tunnel
After=network-online.target time-sync.target
Wants=network-online.target time-sync.target

[Service]
Type=simple
ExecStart=/bin/sh -c '/usr/local/bin/cloudflared tunnel run --protocol http2 --token "$(cat /root/.internkim/secrets/tunnel-token)"'
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SVCEOF
rm -f /etc/init.d/S98cloudflared 2>/dev/null
systemctl stop cloudflared 2>/dev/null || true
killall cloudflared 2>/dev/null || true
systemctl daemon-reload
systemctl enable --now cloudflared
for i in $(seq 1 45); do
  [ "$(systemctl is-active cloudflared 2>/dev/null)" = "active" ] && break
  sleep 2
done`)

	if strings.TrimSpace(state.sshClient.run("for i in $(seq 1 15); do [ \"$(systemctl is-active cloudflared 2>/dev/null)\" = active ] && echo active && exit 0; sleep 1; done; systemctl is-active cloudflared 2>/dev/null || true")) == "active" {
		fmt.Printf("  %s\n", state.messenger.t("cloudflared 실행 중", "cloudflared running"))
		return nil
	}

	return fmt.Errorf("cloudflared failed to start")
}

func (state *setupFlowState) writeDeviceAuthFilesSSH() {
	deviceSecret := loadOrCreateDeviceSecret(state.stateDir)
	state.sshClient.run(fmt.Sprintf(`mkdir -p /root/.internkim/secrets /root/.internkim/env
printf '%%s' %s > %s
printf '%%s' %s > %s
printf '%%s' %s > %s
chown root:root %s
chmod 600 %s
chown root:blueclaw %s %s
chmod 640 %s %s`,
		quoteShellValue(state.deviceID),
		quoteShellValue(blueclaw.InternKimDeviceIDPath),
		quoteShellValue(deviceSecret),
		quoteShellValue(blueclaw.InternKimDeviceSecretPath),
		quoteShellValue(state.configuration.APIBaseURL),
		quoteShellValue(blueclaw.InternKimAPIURLPath),
		quoteShellValue(blueclaw.InternKimDeviceSecretPath),
		quoteShellValue(blueclaw.InternKimDeviceSecretPath),
		quoteShellValue(blueclaw.InternKimDeviceIDPath),
		quoteShellValue(blueclaw.InternKimAPIURLPath),
		quoteShellValue(blueclaw.InternKimDeviceIDPath),
		quoteShellValue(blueclaw.InternKimAPIURLPath),
	))
}

func (state *setupFlowState) stageTunnelSD(context *setup.Context) error {
	if err := state.ensureDeviceRegistration(context.Force); err != nil {
		return err
	}

	if err := context.SD.WriteFile("secrets/tunnel-token", []byte(state.tunnelToken), 0o644); err != nil {
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
	if err := context.SD.WriteFile("device-id", []byte(state.deviceID), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("api-url", []byte(state.configuration.APIBaseURL), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("secrets/device-secret", []byte(loadOrCreateDeviceSecret(state.stateDir)), 0o644); err != nil {
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
	if err := state.ensureDeviceRegistration(false); err != nil {
		return err
	}
	state.writeDeviceAuthFilesSSH()
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
chown root:blueclaw /root/.blueclaw/config 2>/dev/null || true
chmod 770 /root/.blueclaw/config 2>/dev/null || true
systemctl daemon-reload
systemctl enable --now internkim-users-sync.timer
systemctl start internkim-users-sync.service || journalctl -u internkim-users-sync -n 40 --no-pager`,
		quoteShellValue(blueclaw.InternKimUsersSyncScriptPath),
		blueclaw.InternKimUsersSyncScript(),
		quoteShellValue(blueclaw.InternKimUsersSyncScriptPath),
		quoteShellValue(blueclaw.InternKimUsersSyncServicePath),
		blueclaw.InternKimUsersSyncServiceUnit(),
		quoteShellValue(blueclaw.InternKimUsersSyncTimerPath),
		blueclaw.InternKimUsersSyncTimerUnit(),
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
	if err := state.ensureDeviceRegistration(false); err != nil {
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

	runtimeConfiguration, err := blueclaw.BlueclawRuntimeConfigDocument("")
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
