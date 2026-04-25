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

	"github.com/anthropic-lab/internkim/internal/board"
	setup "github.com/anthropic-lab/internkim/internal/provisioning/steps"
	"github.com/anthropic-lab/internkim/internal/runtime/blueclaw"
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
}

type localBinaryAsset struct {
	name         string
	localPath    string
	remotePath   string
	downloadURL  string
	archiveEntry string
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
		Translate:              func(korean, english string) string { return state.messenger.t(korean, english) },
		LoadState:              func(key string) string { return loadState(state.stateDir, key) },
		SaveState:              func(key, value string) { saveState(state.stateDir, key, value) },
		GoogleAuth:             state.googleAuth,
		ResolveGoogleProject:   resolveGoogleProject,
		EnableGoogleAPIs:       enableGoogleAPIs,
		CreateGoogleSA:         createGoogleServiceAccount,
		GetOpenRouterKey:       buildOpenRouterKeyCallback(state.stateDir, state.messenger, state.parameters.OpenRouterAPIKey, state.nonInteractive),
		GetLiteRTModelPath:     buildLiteRTModelPathCallback(state.parameters.LiteRTModelPath),
		GetGasWebhookURL:       state.provisionGasWebhook,
		GwsSkillsInstallScript: gwsSkillsInstallScript,
		BinariesVersion:        state.binariesVersion,
		InstallBinariesSSH:     state.installBinariesSSH,
		StageBinariesSD:        state.stageBinariesSD,
		ConfigureWifiSSH:       state.configureWifiSSH,
		StageWifiSD:            state.stageWifiSD,
		ProvisionTunnelSSH:     state.provisionTunnelSSH,
		StageTunnelSD:          state.stageTunnelSD,
		ConfigureSlackTokenSSH: state.configureSlackTokenSSH,
		StageSlackTokenSD:      state.stageSlackTokenSD,
		InstallUsersSyncSSH:    state.installUsersSyncSSH,
		StageUsersSyncSD:       state.stageUsersSyncSD,
		StageBootstrapSD:       state.stageBootstrapSD,
		InstallMattermost: func(context *setup.Context) error {
			if state.sshClient == nil {
				return nil
			}
			installMattermost(state.messenger, state.sshClient, context.Force)
			return nil
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

func (state *setupFlowState) googleAuth() (*setup.GoogleAuth, error) {
	if state.parameters.GoogleAccessToken != "" {
		return &setup.GoogleAuth{
			AccessToken: state.parameters.GoogleAccessToken,
			Email:       fetchGoogleEmail(state.parameters.GoogleAccessToken),
		}, nil
	}
	if accessToken := strings.TrimSpace(os.Getenv("INTERNKIM_GOOGLE_ACCESS_TOKEN")); accessToken != "" {
		return &setup.GoogleAuth{
			AccessToken: accessToken,
			Email:       fetchGoogleEmail(accessToken),
		}, nil
	}
	if accessToken := strings.TrimSpace(os.Getenv("GOOGLE_ACCESS_TOKEN")); accessToken != "" {
		return &setup.GoogleAuth{
			AccessToken: accessToken,
			Email:       fetchGoogleEmail(accessToken),
		}, nil
	}
	if state.nonInteractive {
		return nil, fmt.Errorf("google access token is empty; pass --google-access-token, set INTERNKIM_GOOGLE_ACCESS_TOKEN, run interactive setup, or skip google")
	}
	return googleAuth()
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
	if state.nonInteractive {
		return "", fmt.Errorf("GAS webhook URL is empty; pass --gas-webhook-url, set INTERNKIM_GAS_WEBHOOK_URL, or run interactive setup once")
	}
	return provisionGasWebhook(accessToken)
}

func (state *setupFlowState) resolveSlackBotToken() string {
	if state.parameters.SlackBotToken != "" {
		return state.parameters.SlackBotToken
	}
	return strings.TrimSpace(os.Getenv("INTERNKIM_SLACK_BOT_TOKEN"))
}

func (state *setupFlowState) ensureWiFiCredentials() error {
	if state.wifiResolved {
		return nil
	}

	savedSSID := loadState(state.stateDir, "wifi_ssid")
	savedPassword := loadState(state.stateDir, "wifi_pass")
	currentSSID := detectSSID(state.getSSIDPath)

	state.wifiChanged = currentSSID != "" && currentSSID != savedSSID
	state.wifiSSID = savedSSID
	state.wifiPassword = savedPassword

	switch {
	case state.wifiChanged:
		state.wifiSSID = currentSSID
		fmt.Printf("  SSID: %s (%s)\n", state.wifiSSID, state.messenger.t("변경 감지", "changed"))
	case state.wifiSSID != "":
		fmt.Printf("  SSID: %s\n", state.wifiSSID)
	case currentSSID != "":
		state.wifiSSID = currentSSID
		fmt.Printf("  SSID: %s\n", state.wifiSSID)
	default:
		if state.nonInteractive {
			return fmt.Errorf("wifi ssid is empty; set up Wi-Fi state interactively once or skip wifi")
		}
		state.wifiSSID = readLine(state.messenger.t("  Wi-Fi SSID 입력: ", "  Enter Wi-Fi SSID: "))
	}

	if state.wifiSSID == "" {
		return fmt.Errorf("wifi ssid is empty")
	}

	if state.wifiPassword == "" || state.wifiChanged {
		keychainPassword := getKeychainPassword(state.wifiSSID)
		if keychainPassword != "" {
			state.wifiPassword = keychainPassword
			fmt.Printf("  %s\n", state.messenger.t("키체인에서 비밀번호 추출 완료", "Password retrieved from keychain"))
		} else {
			if state.nonInteractive {
				return fmt.Errorf("wifi password is empty; save it in state interactively once or skip wifi")
			}
			state.wifiPassword = readSecret(state.messenger.t("  Wi-Fi 비밀번호 입력: ", "  Enter Wi-Fi password: "))
		}
	}

	saveState(state.stateDir, "wifi_ssid", state.wifiSSID)
	saveState(state.stateDir, "wifi_pass", state.wifiPassword)
	state.wifiResolved = true
	return nil
}

func (state *setupFlowState) configureWifiSSH(context *setup.Context) error {
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
			"ctrl_interface=/var/run/wpa_supplicant\nap_scan=1\nnetwork={\n  ssid=\"%s\"\n  key_mgmt=NONE\n}\n",
			ssid,
		)
	}
	return fmt.Sprintf(
		"ctrl_interface=/var/run/wpa_supplicant\nap_scan=1\nnetwork={\n  ssid=\"%s\"\n  key_mgmt=WPA-PSK\n  psk=\"%s\"\n}\n",
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
			name:         "gws",
			localPath:    filepath.Join(state.boardBinDir, "gws"),
			remotePath:   "/usr/local/bin/gws",
			downloadURL:  "https://github.com/googleworkspace/cli/releases/latest/download/google-workspace-cli-aarch64-unknown-linux-gnu.tar.gz",
			archiveEntry: "gws",
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
			name:       blueclaw.LiteRTWrapperName,
			localPath:  filepath.Join(state.boardBinDir, blueclaw.LiteRTWrapperName),
			remotePath: blueclaw.LiteRTWrapperBinaryPath,
		},
		{
			name:       "send-file",
			localPath:  board.SendFilePath(state.scriptDir),
			remotePath: "/usr/local/bin/send-file",
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

		if asset.name == blueclaw.CapabilitydName || asset.name == blueclaw.LiteRTWrapperName {
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
		case "send-file":
			return nil, fmt.Errorf("missing board script: %s", asset.localPath)
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
		filepath.Join(state.scriptDir, "cmd", blueclaw.LiteRTWrapperName),
		filepath.Join(state.scriptDir, "internal", "capabilityd"),
	} {
		state.writeDirectoryHash(hash, path)
	}
	blueclawRevision := strings.TrimSpace(runCmd("git", "-C", blueclaw.BlueclawSubmoduleRoot(state.scriptDir), "rev-parse", "HEAD"))
	_, _ = hash.Write([]byte("blueclaw:" + blueclawRevision + "\n"))
	return hex.EncodeToString(hash.Sum(nil))
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
		state.sshClient.scp(asset.localPath, asset.remotePath)
		state.sshClient.run("chmod +x " + quoteShellValue(asset.remotePath))
		fmt.Printf("  %s %s\n", asset.name, state.messenger.t("설치 완료", "installed"))
	}

	state.sshClient.run(`
NOLOGIN_BIN=$(command -v nologin || echo /usr/sbin/nologin)
getent group gws >/dev/null 2>&1 || groupadd --system gws
id gws &>/dev/null || useradd -r -g gws -m -d /home/gws -s "$NOLOGIN_BIN" gws
getent group blueclaw >/dev/null 2>&1 || groupadd --system blueclaw
id blueclaw &>/dev/null || useradd -r -g blueclaw -m -d /home/blueclaw -s "$NOLOGIN_BIN" blueclaw
install -d -o gws -g gws -m 750 /home/gws /home/gws/.cache /home/gws/.config
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

	state.writeWorkspaceDocumentsSSH(loadWorkspaceAgentsMarkdown(state.scriptDir))

	state.sshClient.run(`mkdir -p /etc/sudoers.d
cat > /usr/local/bin/gws-bot <<'EOF'
#!/bin/sh
GOOGLE_APPLICATION_CREDENTIALS=/root/.internkim/secrets/google-sa.json \
GOOGLE_WORKSPACE_CLI_CREDENTIALS_FILE=/root/.internkim/secrets/google-sa.json \
exec /usr/local/bin/gws "$@"
EOF
chmod 755 /usr/local/bin/gws-bot
rm -f /usr/local/bin/gws-mcp /etc/sudoers.d/blueclaw-gws /usr/local/bin/role-memory-mcp /usr/local/bin/role-memory
cat > /etc/sudoers.d/blueclaw-mcp <<'EOF'
blueclaw ALL=(gws) NOPASSWD: /usr/local/bin/gws
EOF
chmod 440 /etc/sudoers.d/blueclaw-mcp`)

	for _, binaryName := range []string{"download", "send-file"} {
		state.sshClient.run(
			"cp /usr/local/bin/" + binaryName + " " + blueclaw.BlueclawWorkspaceBinaryPath(binaryName) + " && chmod 755 " + blueclaw.BlueclawWorkspaceBinaryPath(binaryName),
		)
	}

	if version := state.binariesVersion(); version != "" {
		state.sshClient.run("mkdir -p /root/.internkim/state && printf '%s' " + quoteShellValue(version) + " > /root/.internkim/state/binaries-version")
	}

	skillsDir := board.SkillsPath(state.scriptDir)
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
			state.sshClient.scpDir(filepath.Join(skillsDir, entry.Name()), remoteSkillDir)
		}
		state.sshClient.run(`for skill in calendar create-gws-file simple-slides; do
  filePath="/root/.blueclaw/workspace/skills/$skill/scripts/gas-call"
  [ -f "$filePath" ] && chmod +x "$filePath"
done
chown -R blueclaw:blueclaw /root/.blueclaw/workspace/skills 2>/dev/null || true`)
	}

	return nil
}

func (state *setupFlowState) writeWorkspaceDocumentsSSH(agentsContent string) {
	state.sshClient.run("cat > /root/.blueclaw/workspace/AGENTS.md <<'EOF'\n" +
		agentsContent +
		"\nEOF\nchown blueclaw:blueclaw /root/.blueclaw/workspace/AGENTS.md")
	state.sshClient.run("cat > /root/.blueclaw/workspace/SOUL.md <<'EOF'\n" +
		identityMarkdown +
		"\nEOF\nchown blueclaw:blueclaw /root/.blueclaw/workspace/SOUL.md")
	state.sshClient.run("cat > /root/.blueclaw/workspace/IDENTITY.md <<'EOF'\n" +
		identityMarkdown +
		"\nEOF\nchown blueclaw:blueclaw /root/.blueclaw/workspace/IDENTITY.md")
}

func loadWorkspaceAgentsMarkdown(scriptDir string) string {
	agentsPath := filepath.Join(scriptDir, "AGENTS.md")
	agentsBytes, err := os.ReadFile(agentsPath)
	if err != nil {
		return agentsMarkdown
	}
	return strings.TrimSpace(string(agentsBytes))
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

func (state *setupFlowState) ensureDeviceRegistration(force bool) error {
	if state.registrationResolved && !force {
		return nil
	}

	state.deviceID = loadOrCreateDeviceID(state.stateDir)
	state.adminEmail = loadState(state.stateDir, "google_email")
	if state.parameters.AdminEmail != "" {
		state.adminEmail = state.parameters.AdminEmail
	}
	if state.adminEmail == "" {
		state.adminEmail = strings.TrimSpace(os.Getenv("INTERNKIM_ADMIN_EMAIL"))
	}
	state.tunnelToken = loadState(state.stateDir, "tunnel_token")
	state.deviceURL = loadState(state.stateDir, "device_url")
	tunnelOrigin := loadState(state.stateDir, "tunnel_origin")

	fmt.Printf("  %s: %s\n", state.messenger.t("기기 ID", "Device ID"), state.deviceID)

	if force || state.tunnelToken == "" || tunnelOrigin != setup.MattermostTunnelOrigin {
		if state.adminEmail == "" {
			if state.nonInteractive {
				return fmt.Errorf("admin email is empty; set INTERNKIM_ADMIN_EMAIL or run interactive setup once")
			}
			state.adminEmail = readLine(state.messenger.t("  관리자 이메일 (구글 계정): ", "  Admin email (Google account): "))
		}
		if state.adminEmail == "" {
			return fmt.Errorf("admin email is empty")
		}

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
		saveState(state.stateDir, "google_email", state.adminEmail)
	} else {
		fmt.Printf("  %s\n", state.messenger.t("이미 등록됨", "Already registered"))
	}

	if state.adminEmail == "" {
		if state.nonInteractive {
			return fmt.Errorf("admin email is empty; set INTERNKIM_ADMIN_EMAIL or run interactive setup once")
		}
		state.adminEmail = readLine(state.messenger.t("  관리자 이메일 (구글 계정): ", "  Admin email (Google account): "))
		if state.adminEmail == "" {
			return fmt.Errorf("admin email is empty")
		}
		saveState(state.stateDir, "google_email", state.adminEmail)
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
printf '%%s' %s > /root/.internkim/admin-email
chmod 644 /root/.internkim/admin-email
chown root:blueclaw /root/.internkim/env/device-url /root/.internkim/env/mattermost-url /root/.internkim/env/tunnel-origin
chmod 640 /root/.internkim/env/device-url /root/.internkim/env/mattermost-url /root/.internkim/env/tunnel-origin`,
		quoteShellValue(state.tunnelToken),
		quoteShellValue(state.deviceURL),
		quoteShellValue(state.deviceURL),
		quoteShellValue(setup.MattermostTunnelOrigin),
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
ExecStart=/bin/sh -c '/usr/local/bin/cloudflared tunnel run --token "$(cat /root/.internkim/secrets/tunnel-token)"'
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SVCEOF
rm -f /etc/init.d/S98cloudflared 2>/dev/null
killall cloudflared 2>/dev/null || true
systemctl daemon-reload
systemctl enable --now cloudflared
for i in $(seq 1 15); do
  [ "$(systemctl is-active cloudflared 2>/dev/null)" = "active" ] && break
  sleep 2
done`)

	if strings.TrimSpace(state.sshClient.run("systemctl is-active cloudflared")) == "active" {
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
	if slackBotToken == "" {
		fmt.Printf("  %s\n", state.messenger.t("Slack 토큰 없음 — Slack connector 비활성", "No Slack token — Slack connector disabled"))
		return nil
	}

	state.sshClient.run(fmt.Sprintf(`mkdir -p /root/.internkim/secrets
printf '%%s' %s > /root/.internkim/secrets/slack-bot-token
chown root:root /root/.internkim/secrets/slack-bot-token
chmod 600 /root/.internkim/secrets/slack-bot-token`,
		quoteShellValue(slackBotToken),
	))
	fmt.Printf("  %s\n", state.messenger.t("Slack 토큰 저장 완료", "Slack token installed"))
	return nil
}

func (state *setupFlowState) stageSlackTokenSD(context *setup.Context) error {
	slackBotToken := state.resolveSlackBotToken()
	if slackBotToken == "" {
		fmt.Printf("  %s\n", state.messenger.t("Slack 토큰 없음 — Slack connector 비활성", "No Slack token — Slack connector disabled"))
		return nil
	}
	if err := context.SD.WriteFile("secrets/slack-bot-token", []byte(slackBotToken), 0o644); err != nil {
		return err
	}
	fmt.Printf("  %s\n", state.messenger.t("Slack 토큰 준비 완료", "Slack token staged"))
	return nil
}

func (state *setupFlowState) installUsersSyncSSH(context *setup.Context) error {
	if err := state.ensureDeviceRegistration(false); err != nil {
		return err
	}
	state.writeDeviceAuthFilesSSH()
	state.sshClient.run(fmt.Sprintf(`cat > %s <<'SYNCEOF'
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
	))
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

	runtimeConfiguration, err := blueclaw.BlueclawRuntimeConfigDocument(blueclaw.BlueclawDefaultModelName)
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

	if err := context.SD.WriteFile("AGENTS.md", []byte(loadWorkspaceAgentsMarkdown(state.scriptDir)), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("IDENTITY.md", []byte(identityMarkdown), 0o644); err != nil {
		return err
	}
	if err := context.SD.WriteFile("SOUL.md", []byte(identityMarkdown), 0o644); err != nil {
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

	localSkillsPath := board.SkillsPath(state.scriptDir)
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

func (state *setupFlowState) stageBackupArtifacts(stageRoot string) error {
	workspaceTarget := filepath.Join(stageRoot, "workspace-restore")
	databaseTarget := filepath.Join(stageRoot, "mattermost-db.sql")

	_ = os.RemoveAll(workspaceTarget)
	_ = os.Remove(databaseTarget)

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
