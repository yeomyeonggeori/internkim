package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

	"github.com/yeomyeonggeori/internkim/internal/blueclawworkspace"
	browserruntime "github.com/yeomyeonggeori/internkim/internal/browser"
	"github.com/yeomyeonggeori/internkim/internal/deviceassets"
	setup "github.com/yeomyeonggeori/internkim/internal/provisioning/steps"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
	"github.com/yeomyeonggeori/internkim/internal/runtime/locallm"
)

type setupFlowState struct {
	messenger      *msg
	configuration  config
	parameters     setupParameterValues
	stateDir       string
	scriptDir      string
	boardBinDir    string
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
	AdminEmail       string
	OpenRouterAPIKey string
	LiteRTModelPath  string
}

type localBinaryAsset struct {
	name           string
	localPath      string
	remotePath     string
	downloadURL    string
	archiveEntry   string
	expectedSHA256 string
	optional       bool
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
	return strings.TrimSpace(`set -eu
` + deviceBrowserRuntimePackageSelectionScript() + `
apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq $runtimePackages`)
}

func deviceBrowserRuntimeInstallScript() string {
	return deviceBrowserRuntimeDependencyInstallScript() + "\n" + browserruntime.DeviceBrowserServiceInstallShellScript()
}

func baseDeviceToolPackages() []string {
	return []string{"bc", "ca-certificates", "curl", "git", "iproute2", "iptables", "libfontconfig1", "procps", "unzip"}
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
		setupBuildID:   setupBuildID,
		sshClient:      sshClient,
		publicKey:      getLocalSSHPubKey(),
		nonInteractive: nonInteractive,
	}
}

func (state *setupFlowState) callbacks() setup.Callbacks {
	return setup.Callbacks{
		Translate:                   func(korean, english string) string { return state.messenger.t(korean, english) },
		LoadState:                   func(key string) string { return loadState(state.stateDir, key) },
		SaveState:                   func(key, value string) { saveState(state.stateDir, key, value) },
		GetOpenRouterKey:            buildOpenRouterKeyCallback(state.messenger, state.parameters.OpenRouterAPIKey, state.nonInteractive),
		GetLiteRTModelPath:          buildLiteRTModelPathCallback(state.parameters.LiteRTModelPath),
		GetBuzzKeySeed:              buildBuzzKeySeedCallback(),
		GetBuzzRelayOwnerPubkey:     buildBuzzRelayOwnerPubkeyCallback(),
		GetBuzzAgentSecret:          buildBuzzAgentSecretCallback(),
		BinariesVersion:             state.binariesVersion,
		InstallBinariesSSH:          state.installBinariesSSH,
		InstallLocalLLMRuntimeSSH:   state.installLocalLLMRuntimeSSH,
		InstallBuzzRelayBinariesSSH: state.installBuzzRelayBinariesSSH,
		InstallAdmindSSH:            state.installAdmindSSH,
		InstallCapabilitydSSH:       state.installCapabilitydSSH,
		StageBinariesSD:             state.stageBinariesSD,
		SkillsManifest:              state.skillsManifest,
		InstallSkillsSSH:            state.installSkillsSSH,
		StageSkillsSD:               state.stageSkillsSD,
		BlueclawRuntimeManifest:     state.blueclawRuntimeManifest,
		InstallBlueclawRuntimeSSH:   state.installBlueclawRuntimeSSH,
		BlueclawPayloadManifest:     state.blueclawPayloadManifest,
		InstallBlueclawPayloadSSH:   state.installBlueclawPayloadSSH,
		AdminWebVersion:             state.adminWebVersion,
		DeployAdminWeb:              state.deployAdminWeb,
		ConfigureWifiSSH:            state.configureWifiSSH,
		StageWifiSD:                 state.stageWifiSD,
		StageTunnelSD:               state.stageTunnelSD,
		InstallUsersSyncSSH:         state.installUsersSyncSSH,
		StageUsersSyncSD:            state.stageUsersSyncSD,
		StageBootstrapSD:            state.stageBootstrapSD,
	}
}

func (state *setupFlowState) ensureWiFiCredentials() error {
	if state.wifiResolved {
		return nil
	}

	wifiProfiles, errorValue := resolveWiFiProfiles(state.messenger, state.stateDir)
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
			name:           "cloudflared",
			localPath:      filepath.Join(state.boardBinDir, "cloudflared"),
			remotePath:     "/usr/local/bin/cloudflared",
			downloadURL:    "https://github.com/cloudflare/cloudflared/releases/download/2026.7.2/cloudflared-linux-arm64",
			expectedSHA256: "405df476437e027fc6d18729a5a77155c0a33a6082aeee60a799a688f3052e66",
		},
		{
			name:           "agent-browser",
			localPath:      filepath.Join(state.boardBinDir, "agent-browser"),
			remotePath:     "/usr/local/bin/agent-browser",
			downloadURL:    "https://github.com/vercel-labs/agent-browser/releases/download/v0.32.3/agent-browser-linux-arm64",
			expectedSHA256: "87fd2efb67995fc433569f0383260bfee44a785d6d45ca07c77179c45b70de18",
		},
		{
			name:           "moli",
			localPath:      filepath.Join(state.boardBinDir, "moli"),
			remotePath:     browserruntime.DeviceBrowserExecutablePath,
			downloadURL:    "https://github.com/lexmount/moli/releases/download/v1.1.5/moli-aarch64-unknown-linux-gnu.tar.gz",
			archiveEntry:   "moli",
			expectedSHA256: "7546a11f42dd93d7b45865ca572e12e5ce29a62f9be06e962a20d99b1f16f4d6",
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

func (state *setupFlowState) buzzRelayBinaryAssets() []localBinaryAsset {
	return []localBinaryAsset{
		{
			name:       blueclaw.BuzzRelayName,
			localPath:  filepath.Join(state.scriptDir, blueclaw.BuzzRelayArtifactPath, blueclaw.BuzzRelayName),
			remotePath: blueclaw.BuzzRelayBinaryPath,
		},
		{
			name:       blueclaw.BuzzAdminName,
			localPath:  filepath.Join(state.scriptDir, blueclaw.BuzzRelayArtifactPath, blueclaw.BuzzAdminName),
			remotePath: blueclaw.BuzzAdminBinaryPath,
		},
		{
			name:       blueclaw.BuzzMigrateName,
			localPath:  filepath.Join(state.scriptDir, blueclaw.BuzzRelayArtifactPath, blueclaw.BuzzMigrateName),
			remotePath: blueclaw.BuzzMigrateBinaryPath,
			optional:   true,
		},
		{
			name:       blueclaw.ChatdName,
			localPath:  filepath.Join(state.scriptDir, blueclaw.BuzzRelayArtifactPath, blueclaw.ChatdName),
			remotePath: blueclaw.ChatdBinaryPath,
			optional:   true,
		},
	}
}

func (state *setupFlowState) installBuzzRelayBinariesSSH(context *setup.Context) error {
	for _, asset := range state.buzzRelayBinaryAssets() {
		if _, errorValue := os.Stat(asset.localPath); errorValue != nil {
			if !asset.optional {
				return fmt.Errorf("buzz relay artifact missing at %s; run make prepare-buzz-relay: %w", asset.localPath, errorValue)
			}
			fmt.Printf("  %s %s (%s)\n", asset.name, state.messenger.t("건너뜀: 빌드 산출물 없음", "skipped: no build produces it"), asset.localPath)
			continue
		}
		existingHash := strings.TrimSpace(state.sshClient.run(
			fmt.Sprintf("md5sum %s 2>/dev/null | awk '{print $1}'", asset.remotePath),
		))
		localHash := strings.TrimSpace(runCmd("md5", "-q", asset.localPath))
		if existingHash != "" && existingHash == localHash {
			fmt.Printf("  %s %s\n", asset.name, state.messenger.t("이미 최신", "up to date"))
			continue
		}
		state.sshClient.run("mkdir -p " + quoteShellValue(filepath.Dir(asset.remotePath)))
		if errorValue := state.sshClient.scp(asset.localPath, asset.remotePath); errorValue != nil {
			return errorValue
		}
		state.sshClient.run("chmod +x " + quoteShellValue(asset.remotePath))
		fmt.Printf("  %s %s\n", asset.name, state.messenger.t("설치 완료", "installed"))
	}
	return nil
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
			if asset.expectedSHA256 != "" {
				if checksumError := verifyLocalBinaryChecksum(asset.localPath, asset.expectedSHA256); checksumError != nil {
					os.Remove(asset.localPath)
					fmt.Println("FAILED")
					return nil, fmt.Errorf("verify %s: %w", asset.name, checksumError)
				}
			}
			fmt.Println("ok")
		}
	}

	return assets, nil
}

func verifyLocalBinaryChecksum(localPath, expectedSHA256 string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actualSHA256 := hex.EncodeToString(hash.Sum(nil))
	if actualSHA256 != expectedSHA256 {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, actualSHA256)
	}
	return nil
}

func buildGoBinaryAsset(state *setupFlowState, asset localBinaryAsset) error {
	fmt.Printf("  %s %s... ", state.messenger.t("빌드 중", "Building"), asset.name)
	arguments := []string{"build", "-o", asset.localPath}
	if asset.name == blueclaw.AdmindName {
		stampFlags, errorValue := admindBuildFlags(state)
		if errorValue != nil {
			fmt.Println("FAILED")
			return errorValue
		}
		arguments = append(arguments, "-ldflags", stampFlags)
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

func admindBuildFlags(state *setupFlowState) (string, error) {
	revision := strings.TrimSpace(runCmd("git", "-C", state.scriptDir, "rev-parse", "--short", "HEAD"))
	buildID := strings.TrimSpace(state.setupBuildID)
	if buildID == "" {
		buildID = "unknown"
	}
	if revision == "" {
		revision = "unknown"
	}
	return deviceAdmindStampFlags(buildID, revision)
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
		filepath.Join(blueclaw.BlueclawSubmoduleRoot(state.scriptDir), "protocol", "bun.lock"),
		filepath.Join(blueclaw.BlueclawSubmoduleRoot(state.scriptDir), "protocol", "package.json"),
		filepath.Join(blueclaw.BlueclawSubmoduleRoot(state.scriptDir), "protocol", "src"),
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
	remoteVersion, errorValue := adminUIVersionOf([]byte(output))
	if errorValue != nil {
		return fmt.Errorf("deployed admin UI: %w", errorValue)
	}
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
	return adminUIVersionOf(document)
}

func adminUIVersionOf(document []byte) (string, error) {
	var stamp struct {
		Version string `json:"version"`
	}
	if errorValue := json.Unmarshal(document, &stamp); errorValue != nil {
		return "", fmt.Errorf("read admin UI version: %w", errorValue)
	}
	version := strings.TrimSpace(stamp.Version)
	if version == "" {
		return "", errors.New("the admin UI version stamp names no version")
	}
	return version, nil
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

	if err := state.ensureAgentBrowserRuntimeSSH(); err != nil {
		return err
	}

	if err := state.installHostDeviceAssetsSSH(); err != nil {
		return err
	}

	if err := state.ensureManagedHostExecutablesSSH(); err != nil {
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
chmod 755 /root/.internkim
chmod 755 /root/.blueclaw/workspace/bin /root/.blueclaw/workspace/downloads`)

	if err := state.installReleaseDownloadTokenSSH(); err != nil {
		return err
	}

	if err := state.installBlueclawMigrationsSSH(); err != nil {
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

func (state *setupFlowState) installReleaseDownloadTokenSSH() error {
	tokenPath, cleanup, errorValue := releaseDownloadTokenSourcePath()
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

func releaseDownloadTokenSourcePath() (string, func(), error) {
	token := strings.TrimSpace(os.Getenv("INTERNKIM_RELEASE_DOWNLOAD_TOKEN"))
	if token == "" {
		return "", nil, nil
	}
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
    bun_zip_url="https://github.com/oven-sh/bun/releases/download/bun-v1.3.10/bun-linux-aarch64.zip"
    bun_zip_sha256="fa5ecb25cafa8e8f5c87a0f833719d46dd0af0a86c7837d806531212d55636d3"
    bun_tmp_zip="$(mktemp)"
    bun_tmp_dir="$(mktemp -d)"
    curl -fsSL -o "$bun_tmp_zip" "$bun_zip_url"
    echo "$bun_zip_sha256  $bun_tmp_zip" | sha256sum -c -
    unzip -q "$bun_tmp_zip" -d "$bun_tmp_dir"
    install -d -o blueclaw -g blueclaw -m 755 /home/blueclaw/.bun/bin
    install -o blueclaw -g blueclaw -m 755 "$bun_tmp_dir/bun-linux-aarch64/bun" /home/blueclaw/.bun/bin/bun
    rm -rf "$bun_tmp_zip" "$bun_tmp_dir"
  fi
  install -o root -g root -m 755 /home/blueclaw/.bun/bin/bun /opt/internkim/managed-bin/bun
fi
install -o root -g root -m 755 /opt/internkim/managed-bin/bun /usr/local/bin/bun
ln -sfn /usr/local/bin/bun /usr/local/bin/bunx
chown root:root /usr/local/bin/bun /usr/local/bin/bunx
chmod 755 /usr/local/bin/bun
if ! command -v uv >/dev/null 2>&1; then
  curl -LsSf https://astral.sh/uv/0.11.11/install.sh -o /tmp/internkim-uv-install.sh
  UV_UNMANAGED_INSTALL=/usr/local/bin sh /tmp/internkim-uv-install.sh
fi
document_requirements=/opt/internkim/document-conversion/requirements.txt
if [ -f "$document_requirements" ]; then
  if [ ! -x /opt/internkim/document-venv/bin/python ]; then
    uv venv --clear /opt/internkim/document-venv >/dev/null
  fi
  uv pip install --quiet --python /opt/internkim/document-venv/bin/python -r "$document_requirements"
  if ! /opt/internkim/document-venv/bin/python -c 'import anydoc, bs4, markdownify, pypdf, pypdfium2' >/dev/null 2>&1; then
    echo "host-document-venv-incomplete"; exit 1
  fi
else
  echo "host-document-requirements-missing"; exit 1
fi
for managed_executable in bun bunx uv; do
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

// Registration still hands out a hostname and a tunnel token, but the step that
// installed the tunnel was removed in #483, so on anything provisioned since
// then that hostname resolves to nothing and the check below fails on a device
// that is perfectly healthy. A device that runs the tunnel is still held to it.
func (state *setupFlowState) deviceServesItsPublicURL() bool {
	return strings.TrimSpace(state.sshClient.run("systemctl is-active cloudflared 2>/dev/null")) == "active"
}

func (state *setupFlowState) verifyAdmindDeployment(context *setup.Context) error {
	localHealth, errorValue := state.sshClient.runResult("curl -fsS --retry 10 --retry-connrefused --retry-delay 1 http://127.0.0.1:18080/admin/api/health")
	if errorValue != nil {
		return fmt.Errorf("admind local health check failed: %s", strings.TrimSpace(localHealth))
	}
	if !strings.Contains(localHealth, `"status":"ok"`) || !strings.Contains(localHealth, `"recoveryAvailable":true`) {
		return fmt.Errorf("admind local health response missing recovery status: %s", strings.TrimSpace(localHealth))
	}
	if strings.TrimSpace(context.PublicURL) == "" || !state.deviceServesItsPublicURL() {
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
		stateDir:    state.stateDir,
		deviceURL:   context.PublicURL,
		fleetID:     loadState(state.stateDir, "fleet_id"),
		fleetSecret: loadState(state.stateDir, "fleet_secret"),
	}, "status", ""); errorValue != nil {
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
	if !state.waitForActiveService(serviceName) {
		return fmt.Errorf("%s restart failed", serviceName)
	}
	fmt.Printf("  %s %s\n", serviceName, state.messenger.t("재시작 완료", "restarted"))
	return nil
}

// A unit that restarts under systemd passes through activating, and a unit that
// crashed once comes back on its own restart, so sampling once right after the
// restart reports a healthy service as failed.
func (state *setupFlowState) waitForActiveService(serviceName string) bool {
	status := strings.TrimSpace(state.sshClient.run(
		"for attempt in $(seq 1 30); do state=$(systemctl is-active " + serviceName + " 2>/dev/null); " +
			"if [ \"$state\" = active ]; then printf active; exit 0; fi; " +
			"if [ \"$state\" = failed ]; then printf failed; exit 0; fi; sleep 1; done; printf \"$state\"",
	))
	if status != "active" {
		return false
	}
	// A crash-looping unit reads active between restarts, so require it to stay
	// up rather than trusting the first sample.
	restartCount := strings.TrimSpace(state.sshClient.run("systemctl show " + serviceName + " -p NRestarts --value 2>/dev/null"))
	settled := strings.TrimSpace(state.sshClient.run(
		"sleep 3; state=$(systemctl is-active " + serviceName + " 2>/dev/null); " +
			"printf \"%s %s\" \"$state\" \"$(systemctl show " + serviceName + " -p NRestarts --value 2>/dev/null)\"",
	))
	return settled == "active "+restartCount
}

func (state *setupFlowState) installGoServiceUnitSSH(servicePath string, serviceDocument string) {
	servicePath = strings.TrimSpace(servicePath)
	serviceDocument = strings.TrimSpace(serviceDocument)
	if servicePath == "" || serviceDocument == "" {
		return
	}
	state.sshClient.run(fmt.Sprintf("cat > %s <<'SERVICEEOF'\n%s\nSERVICEEOF\nsystemctl daemon-reload", quoteShellValue(servicePath), serviceDocument))
}

func (state *setupFlowState) installSkillsSSH(context *setup.Context) error {
	if _, errorValue := blueclawworkspace.SkillRootPaths(state.scriptDir); errorValue != nil {
		return errorValue
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
	for _, asset := range deviceassets.All() {
		if asset.DeviceKind != deviceassets.DeviceKindWorkspace {
			continue
		}
		if errorValue := state.installDeviceAssetSSH(asset); errorValue != nil {
			return errorValue
		}
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
	permissionsCommand := strings.Join([]string{
		"chown -R root:root /root/.blueclaw/workspace/skills",
		"chmod -R a+rX,go-w /root/.blueclaw/workspace/skills",
		"chown -R root:root /root/.blueclaw/workspace/tools",
		"chmod -R a+rX,go-w /root/.blueclaw/workspace/tools",
		"chown root:root /root/.blueclaw/workspace/AGENTS.md",
		"chmod 644 /root/.blueclaw/workspace/AGENTS.md",
	}, " && ")
	output, errorValue := state.sshClient.runResult(permissionsCommand)
	if errorValue != nil {
		return fmt.Errorf("set skills workspace ownership and permissions: %s: %w", strings.TrimSpace(output), errorValue)
	}
	return nil
}

// The managed host executables are installed against a python requirements file
// and a font directory that arrive as device assets. They used to arrive with
// the skills, which run four steps later, so a device that had never been set
// up failed here every time: "host-document-requirements-missing".
func (state *setupFlowState) installHostDeviceAssetsSSH() error {
	for _, asset := range deviceassets.All() {
		if asset.DeviceKind != deviceassets.DeviceKindHost {
			continue
		}
		if errorValue := state.installDeviceAssetSSH(asset); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (state *setupFlowState) installDeviceAssetSSH(asset deviceassets.Asset) error {
	assetSourcePaths, errorValue := asset.SourcePaths(state.scriptDir)
	if errorValue != nil {
		return errorValue
	}
	sourceDirectoryPaths := []string{}
	for _, sourceDirectoryPath := range assetSourcePaths {
		if info, errorValue := os.Stat(sourceDirectoryPath); errorValue == nil && info.IsDir() {
			sourceDirectoryPaths = append(sourceDirectoryPaths, sourceDirectoryPath)
		}
	}
	if len(sourceDirectoryPaths) == 0 {
		if asset.DeviceKind == deviceassets.DeviceKindHost {
			return nil
		}
		return fmt.Errorf("device asset %q has no source directory", asset.Name)
	}
	if asset.DeviceKind == deviceassets.DeviceKindHost {
		stagingPath := asset.DevicePath + ".new"
		state.sshClient.run("rm -rf " + quoteShellValue(stagingPath) + " && mkdir -p " + quoteShellValue(stagingPath))
		for _, sourceDirectoryPath := range sourceDirectoryPaths {
			if errorValue := state.sshClient.scpDir(sourceDirectoryPath, stagingPath); errorValue != nil {
				return errorValue
			}
		}
		state.sshClient.run("rm -rf " + quoteShellValue(asset.DevicePath) + " && mv " + quoteShellValue(stagingPath) + " " + quoteShellValue(asset.DevicePath))
		return nil
	}
	state.sshClient.run("rm -rf " + quoteShellValue(asset.DevicePath) + " && mkdir -p " + quoteShellValue(asset.DevicePath))
	for _, sourceDirectoryPath := range sourceDirectoryPaths {
		if errorValue := state.sshClient.scpDir(sourceDirectoryPath, asset.DevicePath); errorValue != nil {
			return errorValue
		}
	}
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
	if _, errorValue := blueclawworkspace.SkillRootPaths(state.scriptDir); errorValue != nil {
		return errorValue
	}
	toolsDirectoryPath := blueclawworkspace.ToolsPath(state.scriptDir)
	if info, errorValue := os.Stat(toolsDirectoryPath); errorValue != nil || !info.IsDir() {
		return fmt.Errorf("tools directory missing: %s", toolsDirectoryPath)
	}
	skillDirectories, errorValue := blueclawworkspace.SkillDirectories(state.scriptDir)
	if errorValue != nil {
		return errorValue
	}
	for _, skillDirectory := range skillDirectories {
		if errorValue := copyDirectoryToStage(skillDirectory.Path, filepath.Join(context.SD.RootPath(), "skills", skillDirectory.Name)); errorValue != nil {
			return errorValue
		}
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
	digest, errorValue := skillRootsDigest(state.scriptDir)
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

func skillRootsDigest(scriptDir string) (string, error) {
	digests := []string{}
	skillRootPaths, errorValue := blueclawworkspace.SkillRootPaths(scriptDir)
	if errorValue != nil {
		return "", errorValue
	}
	for _, rootPath := range skillRootPaths {
		if info, statError := os.Stat(rootPath); statError != nil || !info.IsDir() {
			continue
		}
		digest, errorValue := directoryDigest(rootPath)
		if errorValue != nil {
			return "", errorValue
		}
		digests = append(digests, digest)
	}
	if len(digests) == 0 {
		return "", errors.New("no skill root is present")
	}
	combined := sha256.Sum256([]byte(strings.Join(digests, "\n")))
	return hex.EncodeToString(combined[:]), nil
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
		return fmt.Errorf("blueclaw guest base runtime artifact invalid: %w; run `make prepare-blueclaw-runtime-base` before setup", errorValue)
	}
	fmt.Print("  blueclaw guest base runtime... ")
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
	// guestInitSHA256, prepareScriptSHA256 and baseSourceSHA256 all describe what went into
	// the guest image. The kernel answers for itself through guestKernelConfigurationSHA256,
	// so a kernel-only install must not be refused for the age of an image it does not carry.
	if blueclawRuntimeInstallPlanInstallsRootFilesystem(installPlan) {
		if errorValue := blueclaw.ValidateRuntimeArtifactSource(state.scriptDir, manifest); errorValue != nil {
			fmt.Println("failed")
			return fmt.Errorf("blueclaw guest base runtime artifact invalid: %w", errorValue)
		}
	}
	printBlueclawRuntimeInstallPlan(installPlan)
	if blueclawRuntimeInstallPlanIsCurrent(installPlan) {
		fmt.Println("already current")
		return nil
	}
	blueclawWasActive := strings.TrimSpace(state.sshClient.run("systemctl is-active blueclaw 2>/dev/null || true")) == "active"
	state.sshClient.run("systemctl stop blueclaw 2>/dev/null || true")
	defer startBlueclawIfTheInstallLeftItStopped(state.sshClient, blueclawWasActive)
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
test -x /usr/local/bin/cloud-hypervisor
test -x /usr/local/bin/virtiofsd
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
		return fmt.Errorf("verify blueclaw guest runtime: %s: %w", strings.TrimSpace(output), errorValue)
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

type blueclawServiceRunner interface {
	run(command string) string
	runResult(command string) (string, error)
}

func startBlueclawIfTheInstallLeftItStopped(runner blueclawServiceRunner, blueclawWasActive bool) {
	if !blueclawWasActive {
		return
	}
	if strings.TrimSpace(runner.run("systemctl is-active blueclaw 2>/dev/null || true")) == "active" {
		return
	}
	if _, startError := runner.runResult("systemctl start blueclaw"); startError != nil {
		fmt.Printf("\n  WARNING: blueclaw was stopped for the runtime install and did not start again: %v\n", startError)
	}
}

func blueclawRuntimeInstallPlanInstallsRootFilesystem(plan blueclawRuntimeInstallPlan) bool {
	for _, artifact := range plan.artifacts {
		if artifact.name == "rootfs.ext4" && artifact.shouldInstall {
			return true
		}
	}
	return false
}

func (state *setupFlowState) blueclawRuntimeManifest() string {
	manifestPath := filepath.Join(state.scriptDir, blueclaw.BlueclawRuntimeArtifactPath, "manifest.json")
	document, errorValue := os.ReadFile(manifestPath)
	if errorValue != nil {
		return ""
	}
	if _, errorValue := blueclaw.ParseRuntimeArtifactManifest(document); errorValue != nil {
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
	if outcome, errorValue := state.installBlueclawPayloadHTTPS(artifactDirectoryPath, manifest); errorValue == nil {
		return state.reportBlueclawPayloadHTTPSOutcome(outcome)
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
		if errorValue := state.refreshBlueclawDelivery(); errorValue != nil {
			fmt.Println("failed")
			return fmt.Errorf("refresh the delivery share for the current payload: %w", errorValue)
		}
		fmt.Println("already current (delivery share refreshed)")
		return nil
	}

	temporaryPayloadPath := "/tmp/internkim-blueclaw-payload"
	output, errorValue := state.sshClient.runResult(blueclawStopForPayloadSyncCommand())
	if errorValue != nil {
		fmt.Println("failed")
		return fmt.Errorf("stop blueclaw before payload sync: %s: %w", strings.TrimSpace(output), errorValue)
	}
	defer state.startBlueclawAfterPayloadSync()
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

	if errorValue := state.refreshBlueclawDelivery(); errorValue != nil {
		fmt.Println("failed")
		return fmt.Errorf("refresh the delivery share for the payload: %w", errorValue)
	}
	remoteWorkspaceManifestDocument = state.sshClient.run(blueclawWorkspaceManifestCommand())
	if manifestDocument != remoteWorkspaceManifestDocument {
		fmt.Println("failed")
		return fmt.Errorf("refresh the delivery share for the payload: delivered manifest mismatch")
	}
	output, errorValue = state.sshClient.runResult("install -m 0644 " + temporaryPayloadPath + "/manifest.json " + blueclaw.BlueclawPayloadManifestPath)
	if errorValue != nil {
		fmt.Println("failed")
		return fmt.Errorf("install blueclaw payload manifest: %s: %w", strings.TrimSpace(output), errorValue)
	}
	fmt.Println("installed")
	return nil
}

func (state *setupFlowState) startBlueclawAfterPayloadSync() {
	output, errorValue := state.sshClient.runResult(blueclawStartAfterPayloadSyncCommand())
	if errorValue == nil {
		return
	}
	fmt.Printf("\n  WARNING: blueclaw is stopped and did not start: %s\n", strings.TrimSpace(output))
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
	outcome, errorValue := state.installBlueclawPayloadHTTPS(artifactDirectoryPath, manifest)
	if errorValue != nil {
		fmt.Println("failed")
		return errorValue
	}
	return state.reportBlueclawPayloadHTTPSOutcome(outcome)
}

func (state *setupFlowState) reportBlueclawPayloadHTTPSOutcome(outcome blueclawPayloadInstallOutcome) error {
	if !outcome.AlreadyCurrent || outcome.DeliveryShareRefreshed {
		fmt.Println(outcome.Summary)
		return nil
	}
	if state.sshClient == nil {
		fmt.Println(outcome.Summary)
		fmt.Println("  WARNING: the device did not refresh its delivery share; the payload and skills the guest reads may be stale until the device admind is updated or an SSH deploy runs")
		return nil
	}
	if errorValue := state.refreshBlueclawDelivery(); errorValue != nil {
		fmt.Println("failed")
		return fmt.Errorf("refresh the delivery share for the current payload: %w", errorValue)
	}
	fmt.Println("already current (delivery share refreshed)")
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

type remoteCommandRunner interface {
	runResult(command string) (string, error)
}

func (state *setupFlowState) refreshBlueclawDelivery() error {
	return refreshBlueclawDeliveryOn(state.sshClient)
}

func refreshBlueclawDeliveryOn(runner remoteCommandRunner) error {
	refreshOutput, errorValue := runner.runResult("set -e" + blueclaw.BlueclawDeliveryRefreshCommand())
	if errorValue != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(refreshOutput), errorValue)
	}
	return nil
}

func blueclawWorkspaceManifestCommand() string {
	return "cat " + quoteShellValue(blueclaw.BlueclawDeliveryRuntimePath+"/manifest.json") + " 2>/dev/null || true"
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

// Whether the artifact is well formed and whether its guest image is current are
// different questions. A rebuild answers the first; the second only matters when the
// guest image is about to ship, and rebuilding for it would replace an image the device
// is running well with one nobody has booted.
func (state *setupFlowState) ensureBlueclawRuntimeBaseArtifact() (string, error) {
	artifactDirectoryPath := filepath.Join(state.scriptDir, blueclaw.BlueclawRuntimeArtifactPath)
	_, errorValue := blueclaw.ValidateRuntimeArtifactDirectory(artifactDirectoryPath)
	if errorValue == nil {
		return artifactDirectoryPath, nil
	}
	if reuseError := state.reuseBlueclawRuntimeBaseArtifact(artifactDirectoryPath); reuseError == nil {
		if _, errorValue = blueclaw.ValidateRuntimeArtifactDirectory(artifactDirectoryPath); errorValue == nil {
			return artifactDirectoryPath, nil
		}
	}
	if makeError := state.runMakeTarget("prepare-blueclaw-runtime-base"); makeError != nil {
		return "", fmt.Errorf("blueclaw guest base runtime artifact invalid: %w; automatic preparation failed: %w", errorValue, makeError)
	}
	if _, errorValue = blueclaw.ValidateRuntimeArtifactDirectory(artifactDirectoryPath); errorValue != nil {
		return "", fmt.Errorf("blueclaw guest base runtime artifact invalid after automatic preparation: %w", errorValue)
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

func localLLMIsPlanned(context *setup.Context) bool {
	return context.BoardType != setup.BoardSimulation && context.PlannedSteps["local-llm"]
}

func (state *setupFlowState) installLocalLLMRuntimeSSH(_ *setup.Context) error {
	return state.installLocalLLMSSH()
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
		return missingLocalLLMArtifactError(displayName, buildTool, cacheKey, cacheDirectory)
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

func missingLocalLLMArtifactError(displayName, buildTool, cacheKey, cacheDirectory string) error {
	return fmt.Errorf(
		"%s artifact %s not found in %s; publish it with tools/%s --host <device ip> --user <device user> before running setup",
		displayName, cacheKey, cacheDirectory, buildTool,
	)
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
	fmt.Print("  device browser service... ")
	output, errorValue := state.sshClient.runResult(deviceBrowserRuntimeInstallScript())
	if errorValue != nil {
		fmt.Println("failed")
		diagnostic := strings.TrimSpace(output + "\n" + state.sshClient.run("journalctl -u "+browserruntime.DeviceBrowserServiceName+" -n 40 --no-pager 2>/dev/null"))
		if diagnostic == "" {
			diagnostic = errorValue.Error()
		}
		return fmt.Errorf("device browser service unavailable: %s", diagnostic)
	}
	fmt.Println("running")
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
  echo "agent-browser close log:"
  tail -80 /tmp/internkim-agent-browser-device-close.log 2>/dev/null || true
  echo "device browser version log:"
  tail -20 /tmp/internkim-device-browser-version.log 2>/dev/null || true
  echo "agent-browser device open log:"
  tail -80 /tmp/internkim-agent-browser-device-open.log 2>/dev/null || true
  echo "agent-browser device snapshot log:"
  tail -80 /tmp/internkim-agent-browser-device-snapshot.log 2>/dev/null || true
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

func (state *setupFlowState) writeWorkspaceDocumentsSSH(workspaceDocuments workspaceDocuments) {
	state.sshClient.run("cat > /root/.blueclaw/workspace/AGENTS.md <<'EOF'\n" +
		workspaceDocuments.Agents +
		"\nEOF\nchown blueclaw:blueclaw /root/.blueclaw/workspace/AGENTS.md")
	state.sshClient.run("rm -f /root/.blueclaw/workspace/IDENTITY.md /root/.blueclaw/workspace/SOUL.md /root/.blueclaw/workspace/BOT_PROFILE.yaml /root/.blueclaw/workspace/BOT_PROFILE.md")
}

type workspaceDocuments struct {
	Agents string
}

func loadWorkspaceDocuments(scriptDir string) (workspaceDocuments, error) {
	agentsContent, errorValue := readWorkspaceMarkdown(blueclawworkspace.AgentsPath(scriptDir))
	if errorValue != nil {
		return workspaceDocuments{}, errorValue
	}
	return workspaceDocuments{Agents: agentsContent}, nil
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

	fmt.Printf("  %s\n", state.messenger.t("바이너리 준비 완료", "Binaries staged"))
	return nil
}

func (state *setupFlowState) ensureFleetRegistration(force bool) error {
	if state.registrationResolved && !force {
		return nil
	}

	state.fleetID = loadOrCreateFleetID(state.stateDir)
	state.nodeID = loadNodeID(state.stateDir)
	state.adminEmail = state.parameters.AdminEmail
	state.tunnelToken = loadState(state.stateDir, "tunnel_token")
	state.nodeTunnelToken = loadState(state.stateDir, "node_tunnel_token")
	state.tlsCertificateStatus = loadState(state.stateDir, "tls_certificate_status")
	state.deviceURL = loadState(state.stateDir, "device_url")

	fmt.Printf("  %s: %s\n", state.messenger.t("플릿 ID", "Fleet ID"), state.fleetID)
	fmt.Printf("  %s: %s\n", state.messenger.t("노드 ID", "Node ID"), firstNonEmptyString(state.nodeID, "auto"))

	if force || state.fleetID == "" || state.deviceURL == "" {
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

		saveState(state.stateDir, "tunnel_token", state.tunnelToken)
		saveState(state.stateDir, "node_tunnel_token", state.nodeTunnelToken)
		saveState(state.stateDir, "device_url", state.deviceURL)
		saveState(state.stateDir, "ssh_hostname", sshHostname)
		saveDefaultFleetNode(state.stateDir, registrationResponse)
		if state.adminEmail != "" {
			saveState(state.stateDir, "admin_email", state.adminEmail)
		}
	} else {
		fmt.Printf("  %s\n", state.messenger.t("이미 등록됨", "Already registered"))
		state.fleetRole = firstNonEmptyString(loadState(state.stateDir, "fleet_role"), "active")
		state.fleetActiveCount = defaultIntString(loadState(state.stateDir, "fleet_active_count"), 1)
		state.fleetPendingCount = defaultIntString(loadState(state.stateDir, "fleet_pending_count"), 0)
		state.fleetQuorumSize = defaultIntString(loadState(state.stateDir, "fleet_quorum_size"), 1)
		state.nodeTunnelToken = firstNonEmptyString(loadState(state.stateDir, "node_tunnel_token"), state.tunnelToken)
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
	if err := context.SD.WriteFile("tunnel-origin", []byte(setup.AdminGatewayTunnelOrigin), 0o644); err != nil {
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
	if err := state.ensureFleetRegistration(false); err != nil {
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

	if err := context.SD.WriteFile(
		"internkim-firstboot.sh",
		[]byte(generateFirstbootScript(localLLMIsPlanned(context))),
		0o755,
	); err != nil {
		return err
	}

	stagedSkillDirectories, skillsError := blueclawworkspace.SkillDirectories(state.scriptDir)
	if skillsError != nil {
		return skillsError
	}
	for _, skillDirectory := range stagedSkillDirectories {
		if err := copyDirectoryContents(skillDirectory.Path, filepath.Join(context.SD.RootPath(), "skills", skillDirectory.Name)); err != nil {
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
	blueclawDatabaseTarget := filepath.Join(stageRoot, "blueclaw-db.sql")

	_ = os.RemoveAll(workspaceTarget)
	_ = os.Remove(blueclawDatabaseTarget)

	workspaceSource := filepath.Join(state.stateDir, "backup", "workspace")
	if info, err := os.Stat(workspaceSource); err == nil && info.IsDir() {
		if err := copyDirectoryContents(workspaceSource, workspaceTarget); err != nil {
			return err
		}
		fmt.Printf("  %s\n", state.messenger.t("워크스페이스 백업 복원", "Workspace restored from backup"))
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
