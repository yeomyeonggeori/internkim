package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"
)

var (
	boardUser = "root"
	boardPass = "root"
	usbNCMIPs = []string{"10.11.60.1"}

	zeroclawModel     = "google/gemini-3.1-flash-lite-preview"
	zeroclawAPIBase   = "https://openrouter.ai/api/v1"
)

type config struct {
	APIBaseURL     string
	RegisterSecret string
	CFDomain       string
}

func loadConfig() config {
	loadEnvFile()
	return config{
		APIBaseURL:     envOr("QC_API_URL", "https://internkim.pages.dev"),
		RegisterSecret: envOr("QC_REGISTER_SECRET", ""),
		CFDomain:       envOr("QC_DOMAIN", "intern.kim"),
	}
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
				v = strings.TrimSpace(v)
				if os.Getenv(k) == "" {
					os.Setenv(k, v)
				}
			}
		}
	}
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup":
			runSetup()
		case "model":
			runModel()
		case "invite":
			runInvite()
		case "users":
			runUsers()
		case "status":
			runStatus()
		case "update":
			runUpdate()
		case "deploy":
			runDeploy()
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
	fmt.Println("  model    Manage LLM model (current/set/list)")
	fmt.Println("  invite   Generate invite QR code")
	fmt.Println("  users    Manage allowed users")
	fmt.Println("  status   Check board and tunnel status")
	fmt.Println("  update   OTA update")
	fmt.Println("  deploy   Build and deploy web UI + board-bridge to board")
	fmt.Println("  sim      Connect to ARM hardware simulator via SSH")
}

func runSetup() {
	lang := "ko"
	if containsArg("--en") {
		lang = "en"
	}
	force := containsArg("--force")
	sim := containsArg("--sim")
	m := newMsg(lang)
	cfg := loadConfig()
	stateDir := quickclawDir()

	scriptDir, _ := os.Getwd()
	binDir := filepath.Join(scriptDir, "bin")
	boardBinDir := filepath.Join(scriptDir, "board-bin")
	sshpassBin := filepath.Join(binDir, "sshpass")
	getSSIDBin := filepath.Join(binDir, "get-ssid")
	zeroclawBin := filepath.Join(boardBinDir, "zeroclaw")
	gwsBin := filepath.Join(boardBinDir, "gws")
	boardUIDir := filepath.Join(scriptDir, "board-ui")

	totalSteps := 10
	fmt.Println("=== Intern Kim Setup ===")
	fmt.Println()

	// 1. Board detection
	step(1, totalSteps, m.t("보드 연결 확인 중...", "Detecting board..."))
	var boardIP string
	var ssh *sshClient
	if sim {
		boardIP = "localhost"
		ssh = newSSHWithPort(sshpassBin, boardUser, "", boardIP, "2222")
	} else {
		boardIP = detectBoard(usbNCMIPs)
		if boardIP == "" {
			fatal(m.t(
				"보드를 찾을 수 없습니다.\nUSB-C 데이터 케이블로 보드와 컴퓨터를 연결하고 30초 기다린 후 다시 시도하세요.",
				"Board not found.\nConnect the board with a USB-C data cable and wait 30 seconds.",
			))
		}
		ssh = newSSH(sshpassBin, boardUser, boardPass, boardIP)
	}
	fmt.Printf("  %s: %s\n", m.t("보드 발견", "Board found"), boardIP)

	// 2. Wi-Fi detection
	step(2, totalSteps, m.t("Wi-Fi 정보 감지 중...", "Detecting Wi-Fi..."))
	ssid := detectSSID(getSSIDBin)
	if ssid == "" {
		fatal(m.t("Wi-Fi에 연결되어 있지 않습니다.", "Not connected to Wi-Fi."))
	}
	fmt.Printf("  SSID: %s\n", ssid)

	boardSSID := strings.TrimSpace(ssh.run(`grep 'ssid="' /etc/wpa_supplicant.conf 2>/dev/null | head -1 | sed 's/.*ssid="//;s/".*//'`))
	wifiIP := strings.TrimSpace(ssh.run("ip -4 addr show wlan0 2>/dev/null | grep 'inet ' | awk '{print $2}' | cut -d/ -f1"))
	skipWifi := !force && boardSSID == ssid && wifiIP != ""

	if skipWifi {
		fmt.Printf("  %s (SSID: %s, IP: %s)\n", m.t("이미 설정됨 — 건너뜀", "Already configured — skipping"), boardSSID, wifiIP)
	} else {
		fmt.Printf("  %s\n", m.t(
			"키체인 접근 팝업이 뜨면 맥 계정/비밀번호를 입력하세요.",
			"Enter your Mac credentials when the keychain popup appears.",
		))
		wifiPass := getKeychainPassword(ssid)

		var wpaConf string
		if wifiPass == "" {
			fmt.Printf("  %s\n", m.t("오픈 네트워크 (비밀번호 없음)", "Open network (no password)"))
			wpaConf = fmt.Sprintf(`ctrl_interface=/var/run/wpa_supplicant
ap_scan=1
network={
  ssid="%s"
  scan_ssid=1
  key_mgmt=NONE
}`, ssid)
		} else {
			fmt.Printf("  %s: %s\n", m.t("비밀번호", "Password"), maskString(wifiPass))
			wpaConf = fmt.Sprintf(`ctrl_interface=/var/run/wpa_supplicant
ap_scan=1
network={
  ssid="%s"
  scan_ssid=1
  key_mgmt=WPA-PSK
  psk="%s"
}`, ssid, wifiPass)
		}

		// 3. Configure Wi-Fi on board
		step(3, totalSteps, m.t("보드에 Wi-Fi 설정 중...", "Configuring Wi-Fi on board..."))

		ssh.run(fmt.Sprintf(`killall wpa_supplicant 2>/dev/null || true
cat > /etc/wpa_supplicant.conf <<'WPAEOF'
%s
WPAEOF
wpa_supplicant -i wlan0 -c /etc/wpa_supplicant.conf -B
sleep 3
udhcpc -i wlan0 -q -n -t 5 -T 3 2>/dev/null || true`, wpaConf))

		// 4. Verify Wi-Fi (retry up to 3 times)
		step(4, totalSteps, m.t("Wi-Fi 연결 확인 중...", "Verifying Wi-Fi connection..."))
		wifiIP = ""
		for attempt := 0; attempt < 3; attempt++ {
			wifiIP = strings.TrimSpace(ssh.run("ip -4 addr show wlan0 2>/dev/null | grep 'inet ' | awk '{print $2}' | cut -d/ -f1"))
			if wifiIP != "" {
				break
			}
			if attempt < 2 {
				fmt.Printf("  %s (%d/3)\n", m.t("재시도 중...", "Retrying..."), attempt+2)
				ssh.run("udhcpc -i wlan0 -q -n -t 5 -T 3 2>/dev/null || true")
			}
		}
		if wifiIP == "" {
			fmt.Printf("  %s\n", m.t("Wi-Fi 연결 실패.", "Wi-Fi connection failed."))
			fmt.Printf("  %s: ssh root@%s\n", m.t("USB로 디버깅", "Debug via USB"), boardIP)
			os.Exit(1)
		}
		fmt.Printf("  %s: %s\n", m.t("Wi-Fi 연결 성공", "Wi-Fi connected"), wifiIP)
	}

	// 5. Install zeroclaw + gws + cloudflared on board
	step(5, totalSteps, m.t("zeroclaw + gws + cloudflared 설치 중...", "Installing zeroclaw + gws + cloudflared..."))
	cloudflaredBin := filepath.Join(boardBinDir, "cloudflared")

	toolBins := []string{"download"}
	for _, tool := range toolBins {
		toolPath := filepath.Join(boardBinDir, tool)
		if _, err := os.Stat(toolPath); os.IsNotExist(err) {
			fmt.Printf("  %s %s... ", m.t("빌드 중", "Building"), tool)
			cmd := exec.Command("go", "build", "-o", toolPath, "./cmd/"+tool+"/")
			cmd.Dir = scriptDir
			cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64")
			if out, err := cmd.CombinedOutput(); err != nil {
				fmt.Printf("FAILED: %s\n", string(out))
			} else {
				fmt.Println("ok")
			}
		}
	}
	for _, bin := range append(
		[]struct{ local, remote, name string }{
			{zeroclawBin, "/usr/local/bin/zeroclaw", "zeroclaw"},
			{gwsBin, "/usr/local/bin/gws", "gws"},
			{cloudflaredBin, "/usr/local/bin/cloudflared", "cloudflared"},
		},
		toolBinEntries(boardBinDir, toolBins)...,
	) {
		if _, err := os.Stat(bin.local); os.IsNotExist(err) {
			fmt.Printf("  %s %s\n", bin.name, m.t("없음 — 건너뜀", "not found — skipping"))
			continue
		}
		existing := strings.TrimSpace(ssh.run(fmt.Sprintf("md5sum %s 2>/dev/null | awk '{print $1}'", bin.remote)))
		localHash := strings.TrimSpace(runCmd("md5", "-q", bin.local))
		if existing != "" && existing == localHash {
			fmt.Printf("  %s %s\n", bin.name, m.t("이미 최신", "up to date"))
		} else {
			ssh.run("mkdir -p /usr/local/bin")
			ssh.scp(bin.local, bin.remote)
			ssh.run(fmt.Sprintf("chmod +x %s", bin.remote))
			fmt.Printf("  %s %s\n", bin.name, m.t("설치 완료", "installed"))
		}
	}

	// 5a. Setup system users and secrets directory
	ssh.run(`
id gws &>/dev/null || useradd -r -s /sbin/nologin gws
id zeroclaw &>/dev/null || useradd -r -s /sbin/nologin zeroclaw
mkdir -p /root/.internkim/secrets
chmod 700 /root/.internkim/secrets
mkdir -p /root/.zeroclaw/workspace/bin /root/.zeroclaw/workspace/downloads
chmod 755 /root/.zeroclaw/workspace/bin /root/.zeroclaw/workspace/downloads`)

	// Create gws-mcp wrapper (runs gws as gws uid, keeps SA key path out of zeroclaw config)
	ssh.run(`cat > /usr/local/bin/gws-mcp <<'WRAPEOF'
#!/bin/bash
export GOOGLE_WORKSPACE_CLI_CREDENTIALS_FILE=/root/.internkim/secrets/google-sa.json
exec sudo -u gws /usr/local/bin/gws mcp
WRAPEOF
chmod 755 /usr/local/bin/gws-mcp`)

	// sudoers: allow zeroclaw to run gws as gws uid without password
	ssh.run(`echo 'zeroclaw ALL=(gws) NOPASSWD: /usr/local/bin/gws' > /etc/sudoers.d/zeroclaw-gws
chmod 440 /etc/sudoers.d/zeroclaw-gws`)

	// 5b. Prepare workspace tools
	for _, tool := range toolBins {
		ssh.run("cp /usr/local/bin/" + tool + " /root/.zeroclaw/workspace/bin/ && chmod 755 /root/.zeroclaw/workspace/bin/" + tool)
	}

	// 5c. Deploy chat UI to board
	if _, err := os.Stat(boardUIDir); err == nil {
		ssh.run("rm -rf /var/www && mkdir -p /var/www")
		ssh.scpDir(boardUIDir, "/var/www")
		fmt.Printf("  %s\n", m.t("채팅 UI 배포 완료", "Chat UI deployed"))
	}

	// 6. OpenRouter API key + zeroclaw config
	step(6, totalSteps, m.t("OpenRouter API 키 설정...", "Configuring OpenRouter API key..."))
	existingKey := strings.TrimSpace(ssh.run("test -f /root/.internkim/secrets/openrouter-api-key && echo yes || echo no"))
	skipAPIKey := !force && existingKey == "yes"
	if skipAPIKey {
		fmt.Printf("  %s\n", m.t("이미 설정됨 — 건너뜀", "Already configured — skipping"))
	} else {
		fmt.Printf("\n  %s\n", m.t("OpenRouter API 키가 필요합니다.", "An OpenRouter API key is required."))
		fmt.Printf("  %s: https://openrouter.ai/keys\n\n", m.t("발급", "Get one at"))
		apiKey := readSecret(m.t("  API 키 입력: ", "  Enter API key: "))
		if apiKey == "" {
			fatal(m.t("API 키가 입력되지 않았습니다.", "No API key provided."))
		}
		fmt.Printf("  %s: %s\n", m.t("API 키", "API key"), maskKey(apiKey))

		// Store API key as KEY=VALUE for systemd EnvironmentFile, owned by zeroclaw uid
		ssh.run(fmt.Sprintf(`printf 'OPENROUTER_API_KEY=%%s' '%s' > /root/.internkim/secrets/openrouter-api-key
chown zeroclaw /root/.internkim/secrets/openrouter-api-key
chmod 640 /root/.internkim/secrets/openrouter-api-key`, apiKey))

		// Write zeroclaw config.toml (no credentials inside)
		zeroclawConfig := buildZeroclawConfig(zeroclawModel, zeroclawAPIBase, nil)
		ssh.run(fmt.Sprintf(`mkdir -p /root/.zeroclaw
cat > /root/.zeroclaw/config.toml <<'CFGEOF'
%s
CFGEOF
chmod 600 /root/.zeroclaw/config.toml`, zeroclawConfig))
	}

	// 7. Register device + cloudflared
	step(7, totalSteps, m.t("기기 등록 + 터널 설정 중...", "Registering device + tunnel setup..."))

	deviceID := loadOrCreateDeviceID(stateDir)
	fmt.Printf("  %s: %s\n", m.t("기기 ID", "Device ID"), deviceID)

	tunnelToken := loadState(stateDir, "tunnel_token")
	skipRegistration := !force && tunnelToken != ""
	if skipRegistration {
		fmt.Printf("  %s\n", m.t("이미 등록됨 — 건너뜀", "Already registered — skipping"))
	} else {
		adminEmail := readLine(m.t("  관리자 이메일 (구글 계정): ", "  Admin email (Google account): "))
		if adminEmail == "" {
			fatal(m.t("이메일이 입력되지 않았습니다.", "No email provided."))
		}

		regResp, err := registerDevice(cfg, deviceID, adminEmail)
		if err != nil {
			if strings.Contains(err.Error(), "409") {
				fmt.Printf("  %s\n", m.t("이미 등록된 기기입니다.", "Device already registered."))
			} else {
				fatal(fmt.Sprintf("%s: %v", m.t("기기 등록 실패", "Registration failed"), err))
			}
		} else {
			saveState(stateDir, "tunnel_token", regResp.TunnelToken)
			saveState(stateDir, "device_url", regResp.URL)
			fmt.Printf("  %s: %s\n", m.t("터널 생성 완료", "Tunnel created"), regResp.URL)
		}
		tunnelToken = loadState(stateDir, "tunnel_token")
	}
	if tunnelToken != "" {
		ssh.run(fmt.Sprintf(`cat > /etc/init.d/S98cloudflared <<'INITEOF'
#!/bin/sh
CLOUDFLARED_BIN="/usr/local/bin/cloudflared"
CLOUDFLARED_LOG="/var/log/cloudflared.log"
CLOUDFLARED_PID="/var/run/cloudflared.pid"
TUNNEL_TOKEN="%s"
case "$1" in
  start)
    if [ -f "$CLOUDFLARED_PID" ] && kill -0 "$(cat $CLOUDFLARED_PID)" 2>/dev/null; then
      echo "cloudflared already running"; exit 0
    fi
    echo "Starting cloudflared tunnel..."
    start-stop-daemon -S -b -m -p "$CLOUDFLARED_PID" -x /bin/sh -- -c "exec $CLOUDFLARED_BIN tunnel run --token $TUNNEL_TOKEN >> $CLOUDFLARED_LOG 2>&1"
    ;;
  stop) start-stop-daemon -K -p "$CLOUDFLARED_PID" 2>/dev/null; rm -f "$CLOUDFLARED_PID" ;;
  restart) $0 stop; sleep 1; $0 start ;;
  status)
    if [ -f "$CLOUDFLARED_PID" ] && kill -0 "$(cat $CLOUDFLARED_PID)" 2>/dev/null; then
      echo "running $(cat $CLOUDFLARED_PID)"
    else echo "stopped"; fi ;;
  *) echo "Usage: $0 {start|stop|restart|status}"; exit 1 ;;
esac
INITEOF
chmod +x /etc/init.d/S98cloudflared
killall cloudflared 2>/dev/null || true
rm -f /var/run/cloudflared.pid
/etc/init.d/S98cloudflared start
sleep 3`, tunnelToken))

		cfStatus := strings.TrimSpace(ssh.run("/etc/init.d/S98cloudflared status"))
		if strings.HasPrefix(cfStatus, "running") {
			fmt.Printf("  %s (%s)\n", m.t("cloudflared 실행 중", "cloudflared running"), cfStatus)
		} else {
			fmt.Printf("  %s\n", m.t("cloudflared 시작 실패 — 로그: ssh root@"+boardIP+" 'cat /var/log/cloudflared.log'",
				"cloudflared failed — log: ssh root@"+boardIP+" 'cat /var/log/cloudflared.log'"))
		}
	}

	// 8. Google Workspace service account setup
	step(8, totalSteps, m.t("Google Workspace 서비스 계정 설정...", "Setting up Google Workspace service account..."))
	existingSAKey := strings.TrimSpace(ssh.run("test -f /root/.internkim/secrets/google-sa.json && echo yes || echo no"))
	if !force && existingSAKey == "yes" {
		fmt.Printf("  %s\n", m.t("이미 설정됨 — 건너뜀", "Already configured — skipping"))
	} else {
		deviceID := loadState(stateDir, "device_id")
		saKey, err := createGoogleServiceAccount(deviceID)
		if err != nil {
			fmt.Printf("  %s: %v\n", m.t("서비스 계정 생성 실패 (나중에 수동 설정 가능)", "SA creation failed (can configure manually later)"), err)
		} else {
			tmpSA := filepath.Join(os.TempDir(), "gsa-"+deviceID+".json")
			os.WriteFile(tmpSA, []byte(saKey), 0600)
			ssh.scp(tmpSA, "/root/.internkim/secrets/google-sa.json")
			os.Remove(tmpSA)
			ssh.run(`chown gws /root/.internkim/secrets/google-sa.json
chmod 640 /root/.internkim/secrets/google-sa.json`)
			fmt.Printf("  %s\n", m.t("서비스 계정 생성 완료", "Service account created"))
		}
	}

	// 9. Mattermost setup
	step(9, totalSteps, m.t("Mattermost 설정...", "Configuring Mattermost..."))
	setupMattermost(m, ssh, stateDir, force)

	// 10. Services autostart + swap
	step(10, totalSteps, m.t("서비스 시작 중...", "Starting services..."))

	// Remove stale httpd init script (board-bridge handles static serving now)
	ssh.run("rm -f /etc/init.d/S97httpd; kill $(ps | grep 'python3 -m http.server' | grep -v grep | awk '{print $1}') 2>/dev/null || true")

	// Remove skills whose required binaries are not available on this device
	ssh.run(`cd /root/.zeroclaw/workspace/skills 2>/dev/null && \
rm -rf agent-browser github summarize skill-creator 2>/dev/null; \
echo "Cleaned unavailable skills"`)

	// zeroclaw systemd service — reads OpenRouter key from secrets file
	ssh.run(`cat > /etc/systemd/system/zeroclaw.service <<'SVCEOF'
[Unit]
Description=ZeroClaw AI Gateway
After=network.target

[Service]
User=zeroclaw
EnvironmentFile=/root/.internkim/secrets/openrouter-api-key
Environment=HOME=/root
ExecStart=/usr/local/bin/zeroclaw gateway
Restart=on-failure

[Install]
WantedBy=multi-user.target
SVCEOF
systemctl daemon-reload
systemctl enable zeroclaw
systemctl restart zeroclaw
sleep 2`)

	gwStatus := strings.TrimSpace(ssh.run("systemctl is-active zeroclaw"))
	if gwStatus == "active" {
		fmt.Printf("  %s\n", m.t("zeroclaw gateway 실행 중", "zeroclaw gateway running"))
	} else {
		fmt.Printf("  %s\n", m.t("gateway 시작 실패", "Gateway failed"))
	}

	ssh.run(`if ! grep -q '/swapfile' /proc/swaps 2>/dev/null; then
  if [ ! -f /swapfile ]; then
    dd if=/dev/zero of=/swapfile bs=1M count=256 2>/dev/null
    chmod 600 /swapfile; mkswap /swapfile >/dev/null 2>&1
  fi
  swapon /swapfile 2>/dev/null || true
  grep -q '/swapfile' /etc/fstab 2>/dev/null || echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi`)

	// 10. Deploy latest build
	step(totalSteps+1, totalSteps+1, m.t("최신 빌드 배포 중...", "Deploying latest build..."))
	runDeploy()

	// Done
	deviceURL := loadState(stateDir, "device_url")
	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("  %s\n", m.t("Intern Kim 설정 완료!", "Intern Kim Setup Complete!"))
	fmt.Println("========================================")
	fmt.Println()
	fmt.Printf("  USB:     %s\n", boardIP)
	fmt.Printf("  Wi-Fi:   %s\n", wifiIP)
	if deviceURL != "" {
		fmt.Printf("  URL:     %s\n", deviceURL)
	}
	fmt.Println()
	fmt.Printf("  %s:\n", m.t("SSH 접속", "SSH access"))
	fmt.Printf("    ssh root@%s\n", wifiIP)
}

// --- Model management ---

func runModel() {
	sub := ""
	if len(os.Args) > 2 {
		sub = os.Args[2]
	}

	scriptDir, _ := os.Getwd()
	sshpassBin := filepath.Join(scriptDir, "bin", "sshpass")

	boardIP := detectBoard(usbNCMIPs)
	if boardIP == "" {
		boardIP = detectBoardWifi(sshpassBin)
	}
	if boardIP == "" {
		fatal("Board not found. Connect via USB or ensure Wi-Fi is reachable.")
	}

	ssh := newSSH(sshpassBin, boardUser, boardPass, boardIP)

	switch sub {
	case "current", "":
		modelCurrentCmd(ssh)
	case "set":
		if len(os.Args) < 4 {
			fmt.Println("Usage: internkim model set <model-id>")
			fmt.Println("Example: internkim model set google/gemini-3.1-flash-lite-preview")
			os.Exit(1)
		}
		modelSetCmd(ssh, os.Args[3])
	case "list":
		modelListCmd(ssh)
	default:
		fmt.Println("Usage: internkim model <current|set|list>")
	}
}

func modelCurrentCmd(ssh *sshClient) {
	raw := ssh.run("grep 'default_model' /root/.zeroclaw/config.toml 2>/dev/null | head -1")
	model := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(raw, "default_model = "), "\""))
	model = strings.Trim(model, "\"")
	if model == "" {
		fmt.Println("No model configured.")
		return
	}
	fmt.Printf("Model: %s\n", model)
}

func modelSetCmd(ssh *sshClient, modelID string) {
	ssh.run(fmt.Sprintf(`sed -i 's|^default_model = .*|default_model = "%s"|' /root/.zeroclaw/config.toml`, modelID))
	ssh.run("systemctl restart zeroclaw 2>/dev/null")
	fmt.Printf("Model changed to: %s\n", modelID)
	fmt.Println("zeroclaw restarted.")
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

func detectBoardWifi(sshpassBin string) string {
	stateDir := quickclawDir()
	// Try to find board via stored Wi-Fi IP or common IPs
	candidates := []string{}
	if wifiIP := loadState(stateDir, "board_wifi_ip"); wifiIP != "" {
		candidates = append(candidates, wifiIP)
	}
	candidates = append(candidates, "192.168.0.141", "192.168.1.141")
	for _, ip := range candidates {
		conn, err := net.DialTimeout("tcp", ip+":22", 2*time.Second)
		if err == nil {
			conn.Close()
			return ip
		}
	}
	return ""
}

// --- Subcommands (stubs) ---

func runDeploy() {
	stateDir := quickclawDir()
	scriptDir, _ := os.Getwd()
	binDir := filepath.Join(scriptDir, "bin")
	sshpassBin := filepath.Join(binDir, "sshpass")
	webDir := filepath.Join(scriptDir, "web")
	boardBinDir := filepath.Join(scriptDir, "board-bin")
	boardUIDir := filepath.Join(scriptDir, "board-ui")

	var ssh *sshClient
	var boardIP string
	if containsArg("--sim") {
		boardIP = "localhost (sim)"
		ssh = newSSHWithPort(sshpassBin, boardUser, "", "localhost", "2222")
	} else {
		boardIP = findBoardIP(sshpassBin, stateDir)
		if boardIP == "" {
			fatal("Board not reachable. Check USB or Wi-Fi connection.")
		}
		ssh = newSSH(sshpassBin, boardUser, boardPass, boardIP)
	}
	fmt.Printf("Board: %s\n", boardIP)

	boardTools := []string{"download"}

	buildUI := true
	buildBridge := true
	buildTools := true
	if len(os.Args) > 2 {
		for _, arg := range os.Args[2:] {
			switch arg {
			case "--ui":
				buildBridge = false
				buildTools = false
			case "--bridge":
				buildUI = false
				buildTools = false
			case "--tools":
				buildUI = false
				buildBridge = false
			}
		}
	}

	if buildUI {
		fmt.Print("Building web UI... ")
		cmd := exec.Command("bun", "run", "build")
		cmd.Dir = webDir
		cmd.Env = append(os.Environ(), "BUILD_TARGET=board")
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Println("FAILED")
			fmt.Println(string(out))
			os.Exit(1)
		}
		fmt.Println("ok")
	}

	if buildBridge {
		fmt.Print("Building board-bridge... ")
		cmd := exec.Command("go", "build", "-o", filepath.Join(boardBinDir, "board-bridge"), "./cmd/board-bridge/")
		cmd.Dir = scriptDir
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64")
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Println("FAILED")
			fmt.Println(string(out))
			os.Exit(1)
		}
		fmt.Println("ok")
	}

	if buildUI {
		fmt.Print("Deploying web UI... ")
		tarPath := filepath.Join(os.TempDir(), "qc-ui.tar")
		cmd := exec.Command("tar", "cf", tarPath, "-C", boardUIDir, ".")
		if err := cmd.Run(); err != nil {
			fatal("Failed to create tar: " + err.Error())
		}
		ssh.scp(tarPath, "/tmp/qc-ui.tar")
		ssh.run("rm -rf /var/www/* && tar xf /tmp/qc-ui.tar -C /var/www/ && rm /tmp/qc-ui.tar")
		os.Remove(tarPath)
		fmt.Println("ok")
	}

	if buildBridge {
		fmt.Print("Deploying board-bridge... ")
		ssh.run("killall board-bridge 2>/dev/null; sleep 1")
		ssh.scp(filepath.Join(boardBinDir, "board-bridge"), "/usr/local/bin/board-bridge")
		ssh.run("chmod +x /usr/local/bin/board-bridge && nohup /usr/local/bin/board-bridge > /var/log/board-bridge.log 2>&1 &")
		fmt.Println("ok")
	}

	if buildTools {
		fmt.Print("Installing skill dependencies... ")
		ssh.run("pip3 install --quiet fpdf2 pypdf 2>&1 | tail -1")
		fmt.Println("ok")

		fmt.Print("Deploying skills... ")
		skillsDir := filepath.Join(scriptDir, "board-scripts", "skills")
		if _, err := os.Stat(skillsDir); err == nil {
			ssh.run("mkdir -p /root/.zeroclaw/workspace/skills")
			entries, _ := os.ReadDir(skillsDir)
			for _, entry := range entries {
				if entry.IsDir() {
					ssh.run("mkdir -p /root/.zeroclaw/workspace/skills/" + entry.Name())
					skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
					if _, err := os.Stat(skillFile); err == nil {
						ssh.scp(skillFile, "/root/.zeroclaw/workspace/skills/"+entry.Name()+"/SKILL.md")
					}
				}
			}
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
			ssh.run("mkdir -p /root/.zeroclaw/workspace/bin /root/.zeroclaw/workspace/downloads && cp /usr/local/bin/" + tool + " /root/.zeroclaw/workspace/bin/ && chmod 755 /root/.zeroclaw/workspace/bin /root/.zeroclaw/workspace/bin/" + tool)
			fmt.Println("ok")
		}
	}

	fmt.Println("Deploy complete.")
}

func findBoardIP(sshpassBin, stateDir string) string {
	candidates := usbNCMIPs
	if wifiIP := loadState(stateDir, "board_wifi_ip"); wifiIP != "" {
		candidates = append(candidates, wifiIP)
	}
	candidates = append(candidates, "192.168.0.141", "192.168.1.141")
	for _, ip := range candidates {
		conn, err := net.DialTimeout("tcp", ip+":22", 2*time.Second)
		if err == nil {
			conn.Close()
			return ip
		}
	}
	return ""
}

func runInvite() { fmt.Println("TODO: invite") }
func runUsers()  { fmt.Println("TODO: users") }
func runStatus() { fmt.Println("TODO: status") }
func runUpdate() { fmt.Println("TODO: update") }

func setupMattermost(m *msg, ssh *sshClient, stateDir string, force bool) {
	existingURL := strings.TrimSpace(ssh.run("cat /root/.internkim/mattermost-url 2>/dev/null"))
	if !force && existingURL != "" {
		fmt.Printf("  %s (%s)\n", m.t("이미 설정됨 — 건너뜀", "Already configured — skipping"), existingURL)
		return
	}

	fmt.Println()
	fmt.Printf("  %s\n", m.t(
		"Mattermost 서버를 보드에 설치한 후 아래 정보를 입력하세요.",
		"Install Mattermost on the board first, then enter the details below.",
	))
	fmt.Printf("  %s\n", m.t(
		"(설정 생략: Enter만 누르세요)",
		"(Skip setup: press Enter)",
	))
	fmt.Println()

	mmURL := readLine(m.t("  Mattermost URL (예: https://chat.intern.kim): ", "  Mattermost URL (e.g. https://chat.intern.kim): "))
	if mmURL == "" {
		fmt.Printf("  %s\n", m.t("건너뜀", "Skipped"))
		return
	}
	mmURL = strings.TrimRight(mmURL, "/")

	adminToken := readSecret(m.t("  관리자 토큰 (System Console → Integrations → Personal Access Tokens): ",
		"  Admin token (System Console → Integrations → Personal Access Tokens): "))
	if adminToken == "" {
		fmt.Printf("  %s\n", m.t("관리자 토큰 없음 — 건너뜀", "No admin token — skipping"))
		return
	}

	botToken := readSecret(m.t("  봇 토큰 (System Console → Integrations → Bot Accounts): ",
		"  Bot token (System Console → Integrations → Bot Accounts): "))
	if botToken == "" {
		fmt.Printf("  %s\n", m.t("봇 토큰 없음 — 건너뜀", "No bot token — skipping"))
		return
	}

	channelID := readLine(m.t("  채널 ID (채널 URL의 마지막 부분): ", "  Channel ID (last part of channel URL): "))
	if channelID == "" {
		fmt.Printf("  %s\n", m.t("채널 ID 없음 — 건너뜀", "No channel ID — skipping"))
		return
	}

	threadReply := promptYN(m.t("스레드로 답장? (권장)", "Reply in thread? (recommended)"))
	mentionOnly := promptYN(m.t("@멘션에만 응답?", "Only respond to @mentions?"))

	// Store credentials in secrets dir
	ssh.run(fmt.Sprintf(`printf '%%s' '%s' > /root/.internkim/mattermost-url
printf '%%s' '%s' > /root/.internkim/mattermost-admin-token
printf '%%s' '%s' > /root/.internkim/secrets/mattermost-bot-token
chmod 600 /root/.internkim/mattermost-url /root/.internkim/mattermost-admin-token
chown zeroclaw /root/.internkim/secrets/mattermost-bot-token
chmod 640 /root/.internkim/secrets/mattermost-bot-token`,
		mmURL, adminToken, botToken))

	// Update zeroclaw config.toml with Mattermost channel
	mm := &mattermostConfig{
		BaseURL:     mmURL,
		BotToken:    botToken,
		ChannelID:   channelID,
		ThreadReply: threadReply,
		MentionOnly: mentionOnly,
	}
	zeroclawConfig := buildZeroclawConfig(zeroclawModel, zeroclawAPIBase, mm)
	ssh.run(fmt.Sprintf(`cat > /root/.zeroclaw/config.toml <<'CFGEOF'
%s
CFGEOF
chmod 600 /root/.zeroclaw/config.toml`, zeroclawConfig))

	fmt.Printf("  %s\n", m.t("Mattermost 채널 설정 완료", "Mattermost channel configured"))

	// Configure push notification privacy
	configureMattermostPush(m, ssh)
}

func configureMattermostPush(m *msg, ssh *sshClient) {
	mmURL := strings.TrimSpace(ssh.run("cat /root/.internkim/mattermost-url 2>/dev/null"))
	mmToken := strings.TrimSpace(ssh.run("cat /root/.internkim/mattermost-admin-token 2>/dev/null"))
	if mmURL == "" || mmToken == "" {
		fmt.Printf("  %s\n", m.t(
			"Mattermost URL/토큰 미설정 — 건너뜀 (나중에 internkim mattermost 커맨드로 설정)",
			"Mattermost URL/token not configured — skipping (use internkim mattermost later)",
		))
		return
	}

	fmt.Println()
	fmt.Printf("  %s\n", m.t("푸시 알림 페이로드 공개 수준을 선택하세요:", "Choose push notification payload visibility:"))
	fmt.Printf("  1. %s\n", m.t("전체 공개  — 알림에 메시지 내용 포함 (push.mattermost.com 경유)", "Full       — message content in notification (via push.mattermost.com)"))
	fmt.Printf("  2. %s\n", m.t("발신자만   — \"ZeroClaw에서 메시지가 왔습니다\" (내용 미포함)", "Sender     — \"Message from ZeroClaw\" (no content)"))
	fmt.Printf("  3. %s\n", m.t("완전 비공개 — 알림은 오지만 앱이 서버에서 직접 내용 가져옴 (권장)", "Private    — notification arrives, app fetches content directly from server (recommended)"))
	fmt.Println()

	choice := readLine(m.t("  선택 (1/2/3) [3]: ", "  Choice (1/2/3) [3]: "))
	if choice == "" {
		choice = "3"
	}

	var contents string
	switch choice {
	case "1":
		contents = "full"
	case "2":
		contents = "generic"
	default:
		contents = "id_loaded"
	}

	body, _ := json.Marshal(map[string]any{
		"EmailSettings": map[string]string{
			"PushNotificationContents": contents,
		},
	})
	req, _ := http.NewRequest("PUT", mmURL+"/api/v4/config/patch", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+mmToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil || resp.StatusCode != 200 {
		fmt.Printf("  %s\n", m.t("설정 실패 — Mattermost 관리자 콘솔에서 수동 설정 필요", "Failed — configure manually in Mattermost admin console"))
		return
	}
	resp.Body.Close()

	label := map[string]string{"full": m.t("전체 공개", "Full"), "generic": m.t("발신자만", "Sender only"), "id_loaded": m.t("완전 비공개", "Private")}[contents]
	fmt.Printf("  %s: %s\n", m.t("설정 완료", "Configured"), label)
}

func runSim() {
	sub := ""
	if len(os.Args) > 2 {
		sub = os.Args[2]
	}

	switch sub {
	case "ssh":
		cmd := exec.Command("ssh",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-p", "2222", "root@localhost")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	default:
		fmt.Println("Usage: internkim sim <ssh>")
		fmt.Println("  ssh  Connect to simulator via SSH (localhost:2222)")
	}
}

// --- Device registration ---

type registerResponse struct {
	DeviceID    string `json:"device_id"`
	TunnelToken string `json:"tunnel_token"`
	URL         string `json:"url"`
}

func registerDevice(cfg config, deviceID, adminEmail string) (*registerResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"device_id":   deviceID,
		"admin_email": adminEmail,
	})

	req, _ := http.NewRequest("POST", cfg.APIBaseURL+"/api/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.RegisterSecret)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result registerResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return &result, nil
}

// --- Command helper ---

func runCmd(name string, args ...string) string {
	out, _ := exec.Command(name, args...).CombinedOutput()
	return string(out)
}

// --- State management (~/.quickclaw/) ---

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

func quickclawDir() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".quickclaw")
	os.MkdirAll(dir, 0700)
	return dir
}

func loadOrCreateDeviceID(stateDir string) string {
	id := loadState(stateDir, "device_id")
	if id != "" {
		return id
	}
	b := make([]byte, 4)
	rand.Read(b)
	id = hex.EncodeToString(b)
	saveState(stateDir, "device_id", id)
	return id
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

// --- Board detection ---

func detectBoard(candidates []string) string {
	for _, ip := range candidates {
		conn, err := net.DialTimeout("tcp", ip+":22", 2*time.Second)
		if err == nil {
			conn.Close()
			return ip
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
	sshpassBin string
	user       string
	pass       string
	host       string
	port       string
}

func newSSH(sshpassBin, user, pass, host string) *sshClient {
	return &sshClient{sshpassBin: sshpassBin, user: user, pass: pass, host: host, port: "22"}
}

func newSSHWithPort(sshpassBin, user, pass, host, port string) *sshClient {
	return &sshClient{sshpassBin: sshpassBin, user: user, pass: pass, host: host, port: port}
}

func (s *sshClient) sshArgs(extra ...string) []string {
	base := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-o", "LogLevel=ERROR",
		"-p", s.port,
	}
	return append(base, extra...)
}

func (s *sshClient) run(cmd string) string {
	var args []string
	if s.pass != "" {
		args = append([]string{"-p", s.pass, "ssh"}, s.sshArgs(fmt.Sprintf("%s@%s", s.user, s.host), cmd)...)
		out, _ := exec.Command(s.sshpassBin, args...).CombinedOutput()
		return string(out)
	}
	out, _ := exec.Command("ssh", append(s.sshArgs(fmt.Sprintf("%s@%s", s.user, s.host), cmd))...).CombinedOutput()
	return string(out)
}

func (s *sshClient) scp(localPath, remotePath string) {
	target := fmt.Sprintf("%s@%s:%s", s.user, s.host, remotePath)
	if s.pass != "" {
		exec.Command(s.sshpassBin, append([]string{"-p", s.pass, "scp"}, s.sshArgs(localPath, target)...)...).Run()
		return
	}
	exec.Command("scp", s.sshArgs(localPath, target)...).Run()
}

func (s *sshClient) scpDir(localDir, remoteDir string) {
	target := fmt.Sprintf("%s@%s:%s", s.user, s.host, remoteDir)
	if s.pass != "" {
		exec.Command(s.sshpassBin, append([]string{"-p", s.pass, "scp", "-r"}, s.sshArgs(localDir+"/.", target)...)...).Run()
		return
	}
	exec.Command("scp", append([]string{"-r"}, s.sshArgs(localDir+"/.", target)...)...).Run()
}

// --- Config builders ---

type mattermostConfig struct {
	BaseURL     string
	BotToken    string
	ChannelID   string
	ThreadReply bool
	MentionOnly bool
}

// buildZeroclawConfig generates zeroclaw config.toml (no credentials inside)
// API key is read at runtime from /root/.internkim/secrets/openrouter-api-key
func buildZeroclawConfig(model, apiBase string, mm *mattermostConfig) string {
	mmSection := ""
	if mm != nil && mm.BaseURL != "" && mm.BotToken != "" && mm.ChannelID != "" {
		mmSection = fmt.Sprintf(`
[channels.mattermost]
enabled = true
base_url = "%s"
bot_token = "%s"
channel_id = "%s"
thread_replies = %v
mention_only = %v
allowed_users = ["*"]
`, mm.BaseURL, mm.BotToken, mm.ChannelID, mm.ThreadReply, mm.MentionOnly)
	}

	return fmt.Sprintf(`# ZeroClaw configuration — credentials are NOT stored here
# API key is injected by systemd from /root/.internkim/secrets/openrouter-api-key

default_provider = "openai-compatible:%s"
default_model = "%s"

[runtime]
kind = "native"

[runtime.native]
sandbox = "landlock"

[gateway]
port = 18790
host = "127.0.0.1"
require_pairing = false

[channels_config]
cli = false

[channels_config.webhook]
secret = "quickclaw"

[[mcp.servers]]
name = "google-workspace"
command = "/usr/local/bin/gws-mcp"
args = []
%s`, apiBase, model, mmSection)
}

// createGoogleServiceAccount creates a service account via Google IAM REST API.
// Opens browser for OAuth consent (once), then creates SA + key, returns SA key JSON.
func createGoogleServiceAccount(deviceID string) (string, error) {
	fmt.Println("  Opening browser for Google Cloud OAuth consent...")
	fmt.Println("  Scope: https://www.googleapis.com/auth/cloud-platform")
	fmt.Printf("  Service account name: internkim-%s\n", deviceID)
	fmt.Println()
	fmt.Println("  NOTE: Automated SA creation requires a Google Cloud project.")
	fmt.Println("  If you don't have one, create it at https://console.cloud.google.com")
	fmt.Println()

	projectID := readLine("  Google Cloud Project ID: ")
	if projectID == "" {
		return "", fmt.Errorf("project ID required")
	}

	// OAuth2 device flow to get access token
	accessToken, err := googleDeviceAuth()
	if err != nil {
		return "", fmt.Errorf("OAuth failed: %w", err)
	}

	saName := fmt.Sprintf("internkim-%s", deviceID)
	saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", saName, projectID)

	client := &http.Client{Timeout: 30 * time.Second}

	// Create service account
	createBody, _ := json.Marshal(map[string]any{
		"accountId": saName,
		"serviceAccount": map[string]string{
			"displayName": "Intern Kim " + deviceID,
		},
	})
	req, _ := http.NewRequest("POST",
		fmt.Sprintf("https://iam.googleapis.com/v1/projects/%s/serviceAccounts", projectID),
		bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("create SA request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		// 409 = already exists, continue to key creation
		if resp.StatusCode != 409 {
			return "", fmt.Errorf("create SA HTTP %d: %s", resp.StatusCode, string(body))
		}
	}

	// Create SA key
	keyReq, _ := http.NewRequest("POST",
		fmt.Sprintf("https://iam.googleapis.com/v1/projects/%s/serviceAccounts/%s/keys", projectID, saEmail),
		strings.NewReader(`{"keyAlgorithm":"KEY_ALG_RSA_2048","privateKeyType":"TYPE_GOOGLE_CREDENTIALS_FILE"}`))
	keyReq.Header.Set("Authorization", "Bearer "+accessToken)
	keyReq.Header.Set("Content-Type", "application/json")
	keyResp, err := client.Do(keyReq)
	if err != nil {
		return "", fmt.Errorf("create key request failed: %w", err)
	}
	defer keyResp.Body.Close()
	keyBody, _ := io.ReadAll(keyResp.Body)
	if keyResp.StatusCode != 200 {
		return "", fmt.Errorf("create key HTTP %d: %s", keyResp.StatusCode, string(keyBody))
	}

	var keyResult struct {
		PrivateKeyData string `json:"privateKeyData"`
	}
	if err := json.Unmarshal(keyBody, &keyResult); err != nil {
		return "", fmt.Errorf("parse key response: %w", err)
	}

	// privateKeyData is base64-encoded JSON
	import64 := keyResult.PrivateKeyData
	decoded := make([]byte, len(import64))
	n, err := decodeBase64(import64, decoded)
	if err != nil {
		return "", fmt.Errorf("decode key: %w", err)
	}
	return string(decoded[:n]), nil
}

// googleDeviceAuth performs OAuth2 device authorization flow and returns an access token.
func googleDeviceAuth() (string, error) {
	clientID := "32555940559.apps.googleusercontent.com" // gcloud SDK public client
	scope := "https://www.googleapis.com/auth/cloud-platform"

	// Step 1: request device code
	resp, err := http.PostForm("https://oauth2.googleapis.com/device/code", map[string][]string{
		"client_id": {clientID},
		"scope":     {scope},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var dc struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURL string `json:"verification_url"`
		Interval        int    `json:"interval"`
	}
	if err := json.Unmarshal(body, &dc); err != nil {
		return "", err
	}

	fmt.Printf("\n  Visit: %s\n", dc.VerificationURL)
	fmt.Printf("  Code:  %s\n\n", dc.UserCode)
	exec.Command("open", dc.VerificationURL).Start()

	interval := dc.Interval
	if interval == 0 {
		interval = 5
	}

	// Step 2: poll for token
	for i := 0; i < 60; i++ {
		time.Sleep(time.Duration(interval) * time.Second)
		pollResp, err := http.PostForm("https://oauth2.googleapis.com/token", map[string][]string{
			"client_id":   {clientID},
			"device_code": {dc.DeviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		})
		if err != nil {
			continue
		}
		pollBody, _ := io.ReadAll(pollResp.Body)
		pollResp.Body.Close()
		var token struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
		}
		json.Unmarshal(pollBody, &token)
		if token.AccessToken != "" {
			return token.AccessToken, nil
		}
		if token.Error != "authorization_pending" && token.Error != "slow_down" {
			return "", fmt.Errorf("auth error: %s", token.Error)
		}
	}
	return "", fmt.Errorf("timed out waiting for authorization")
}

func decodeBase64(src string, dst []byte) (int, error) {
	return base64.StdEncoding.Decode(dst, []byte(src))
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
