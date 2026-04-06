package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
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

	picoclawModelName = "openrouter"
	picoclawModel     = "openrouter/google/gemini-3.1-flash-lite-preview"
	picoclawAPIBase   = "https://openrouter.ai/api/v1"
)

type config struct {
	APIBaseURL     string
	RegisterSecret string
	CFDomain       string
}

func loadConfig() config {
	loadEnvFile()
	return config{
		APIBaseURL:     envOr("QC_API_URL", "https://quick-claw.pages.dev"),
		RegisterSecret: envOr("QC_REGISTER_SECRET", ""),
		CFDomain:       envOr("QC_DOMAIN", "dawn.kim"),
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
		default:
			printUsage()
		}
		return
	}
	runSetup()
}

func printUsage() {
	fmt.Println("Usage: quick-claw <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  setup    Full device provisioning")
	fmt.Println("  model    Manage LLM model (current/set/list)")
	fmt.Println("  invite   Generate invite QR code")
	fmt.Println("  users    Manage allowed users")
	fmt.Println("  status   Check board and tunnel status")
	fmt.Println("  update   OTA update")
	fmt.Println("  deploy   Build and deploy web UI + pico-bridge to board")
}

func runSetup() {
	lang := "ko"
	for _, a := range os.Args {
		if a == "--en" {
			lang = "en"
		}
	}
	m := newMsg(lang)
	cfg := loadConfig()
	stateDir := quickclawDir()

	scriptDir, _ := os.Getwd()
	binDir := filepath.Join(scriptDir, "bin")
	boardBinDir := filepath.Join(scriptDir, "board-bin")
	sshpassBin := filepath.Join(binDir, "sshpass")
	getSSIDBin := filepath.Join(binDir, "get-ssid")
	picoclawBin := filepath.Join(boardBinDir, "picoclaw")
	boardUIDir := filepath.Join(scriptDir, "board-ui")

	totalSteps := 8
	fmt.Println("=== Quick Claw Setup ===")
	fmt.Println()

	// 1. Board detection
	step(1, totalSteps, m.t("보드 연결 확인 중...", "Detecting board..."))
	boardIP := detectBoard(usbNCMIPs)
	if boardIP == "" {
		fatal(m.t(
			"보드를 찾을 수 없습니다.\nUSB-C 데이터 케이블로 보드와 컴퓨터를 연결하고 30초 기다린 후 다시 시도하세요.",
			"Board not found.\nConnect the board with a USB-C data cable and wait 30 seconds.",
		))
	}
	fmt.Printf("  %s: %s\n", m.t("보드 발견", "Board found"), boardIP)

	ssh := newSSH(sshpassBin, boardUser, boardPass, boardIP)

	// 2. Wi-Fi detection
	step(2, totalSteps, m.t("Wi-Fi 정보 감지 중...", "Detecting Wi-Fi..."))
	ssid := detectSSID(getSSIDBin)
	if ssid == "" {
		fatal(m.t("Wi-Fi에 연결되어 있지 않습니다.", "Not connected to Wi-Fi."))
	}
	fmt.Printf("  SSID: %s\n", ssid)

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
	var wifiIP string
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

	// 5. Install picoclaw + cloudflared on board
	step(5, totalSteps, m.t("picoclaw + cloudflared 설치 중...", "Installing picoclaw + cloudflared..."))
	cloudflaredBin := filepath.Join(boardBinDir, "cloudflared")

	for _, bin := range []struct{ local, remote, name string }{
		{picoclawBin, "/usr/local/bin/picoclaw", "picoclaw"},
		{cloudflaredBin, "/usr/local/bin/cloudflared", "cloudflared"},
	} {
		if _, err := os.Stat(bin.local); os.IsNotExist(err) {
			fatal(fmt.Sprintf("%s not found: %s", bin.name, bin.local))
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

	// 5b. Deploy chat UI to board
	if _, err := os.Stat(boardUIDir); err == nil {
		ssh.run("rm -rf /var/www && mkdir -p /var/www")
		ssh.scpDir(boardUIDir, "/var/www")
		fmt.Printf("  %s\n", m.t("채팅 UI 배포 완료", "Chat UI deployed"))
	}

	// 6. OpenRouter API key + picoclaw config
	step(6, totalSteps, m.t("OpenRouter API 키 설정...", "Configuring OpenRouter API key..."))
	apiKey := ""
	existingKey := strings.TrimSpace(ssh.run("grep 'sk-or-' /root/.picoclaw/.security.yml 2>/dev/null | head -1"))
	skipAPIKey := false
	if existingKey != "" {
		fmt.Printf("  %s\n", m.t("API 키가 이미 설정되어 있습니다.", "API key already configured."))
		if !promptYN(m.t("다시 설정하시겠습니까?", "Reconfigure?")) {
			skipAPIKey = true
		}
	}
	if !skipAPIKey {
		fmt.Printf("\n  %s\n", m.t("OpenRouter API 키가 필요합니다.", "An OpenRouter API key is required."))
		fmt.Printf("  %s: https://openrouter.ai/keys\n\n", m.t("발급", "Get one at"))
		apiKey = readSecret(m.t("  API 키 입력: ", "  Enter API key: "))
		if apiKey == "" {
			fatal(m.t("API 키가 입력되지 않았습니다.", "No API key provided."))
		}
		fmt.Printf("  %s: %s\n", m.t("API 키", "API key"), maskKey(apiKey))
	}

	if !skipAPIKey {
		configJSON := buildPicoclawConfig(picoclawModelName, picoclawModel, picoclawAPIBase, apiKey)

		ssh.run(fmt.Sprintf(`/usr/local/bin/picoclaw onboard 2>/dev/null || true
cat > /root/.picoclaw/config.json <<'CFGEOF'
%s
CFGEOF
chmod 600 /root/.picoclaw/config.json
rm -f /root/.picoclaw/.security.yml`, configJSON))
	}

	// 7. Register device + cloudflared
	step(7, totalSteps, m.t("기기 등록 + 터널 설정 중...", "Registering device + tunnel setup..."))

	deviceID := loadOrCreateDeviceID(stateDir)
	fmt.Printf("  %s: %s\n", m.t("기기 ID", "Device ID"), deviceID)

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

	tunnelToken := loadState(stateDir, "tunnel_token")
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

	// 8. Services autostart + swap
	step(8, totalSteps, m.t("서비스 시작 중...", "Starting services..."))

	// httpd for static UI
	ssh.run(`cat > /etc/init.d/S97httpd <<'INITEOF'
#!/bin/sh
case "$1" in
  start) cd /var/www && python3 -m http.server 8080 >> /var/log/httpd.log 2>&1 & ;;
  stop) kill $(ps | grep "python3 -m http.server" | grep -v grep | awk '{print $1}') 2>/dev/null ;;
  restart) $0 stop; sleep 1; $0 start ;;
  *) echo "Usage: $0 {start|stop|restart}"; exit 1 ;;
esac
INITEOF
chmod +x /etc/init.d/S97httpd
/etc/init.d/S97httpd stop 2>/dev/null
/etc/init.d/S97httpd start`)
	fmt.Printf("  %s\n", m.t("httpd 시작됨 (port 8080)", "httpd started (port 8080)"))

	// Remove skills whose required binaries are not available on this device
	ssh.run(`cd /root/.picoclaw/workspace/skills 2>/dev/null && \
rm -rf agent-browser github summarize skill-creator 2>/dev/null; \
echo "Cleaned unavailable skills"`)

	ssh.run(`cat > /etc/init.d/S99picoclaw <<'INITEOF'
#!/bin/sh
PICOCLAW_BIN="/usr/local/bin/picoclaw"
PICOCLAW_LOG="/var/log/picoclaw.log"
PICOCLAW_PID="/var/run/picoclaw.pid"
export PATH="/usr/local/bin:/usr/bin:/usr/sbin:/bin:/sbin"
export HOME="/root"
case "$1" in
  start)
    if [ -f "$PICOCLAW_PID" ] && kill -0 "$(cat $PICOCLAW_PID)" 2>/dev/null; then
      echo "picoclaw already running"; exit 0
    fi
    echo "Starting picoclaw gateway..."
    start-stop-daemon -S -b -m -p "$PICOCLAW_PID" -x /bin/sh -- -c "exec $PICOCLAW_BIN gateway >> $PICOCLAW_LOG 2>&1"
    ;;
  stop) start-stop-daemon -K -p "$PICOCLAW_PID" 2>/dev/null; rm -f "$PICOCLAW_PID" ;;
  restart) $0 stop; sleep 1; $0 start ;;
  status)
    if [ -f "$PICOCLAW_PID" ] && kill -0 "$(cat $PICOCLAW_PID)" 2>/dev/null; then
      echo "running $(cat $PICOCLAW_PID)"
    else echo "stopped"; fi ;;
  *) echo "Usage: $0 {start|stop|restart|status}"; exit 1 ;;
esac
INITEOF
chmod +x /etc/init.d/S99picoclaw
killall picoclaw 2>/dev/null || true
rm -f /var/run/picoclaw.pid
/etc/init.d/S99picoclaw start
sleep 2`)

	gwStatus := strings.TrimSpace(ssh.run("/etc/init.d/S99picoclaw status"))
	if strings.HasPrefix(gwStatus, "running") {
		fmt.Printf("  %s (%s)\n", m.t("gateway 실행 중", "Gateway running"), gwStatus)
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

	// Done
	deviceURL := loadState(stateDir, "device_url")
	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("  %s\n", m.t("Quick Claw 설정 완료!", "Quick Claw Setup Complete!"))
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
			fmt.Println("Usage: quick-claw model set <model-id>")
			fmt.Println("Example: quick-claw model set google/gemini-3.1-flash-lite-preview")
			os.Exit(1)
		}
		modelSetCmd(ssh, ensureOpenRouterPrefix(os.Args[3]))
	case "list":
		modelListCmd(ssh)
	default:
		fmt.Println("Usage: quick-claw model <current|set|list>")
	}
}

func modelCurrentCmd(ssh *sshClient) {
	raw := ssh.run("cat /root/.picoclaw/config.json")
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		fatal("Failed to read config: " + err.Error())
	}
	models, _ := cfg["model_list"].([]any)
	if len(models) == 0 {
		fmt.Println("No model configured.")
		return
	}
	m, _ := models[0].(map[string]any)
	model, _ := m["model"].(string)
	fmt.Printf("Model: %s\n", strings.TrimPrefix(model, "openrouter/"))
}

func modelSetCmd(ssh *sshClient, modelID string) {
	raw := ssh.run("cat /root/.picoclaw/config.json")
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		fatal("Failed to read config: " + err.Error())
	}

	models, _ := cfg["model_list"].([]any)
	if len(models) == 0 {
		fatal("No model_list in config.")
	}
	m, _ := models[0].(map[string]any)
	old := m["model"]
	m["model"] = modelID
	models[0] = m
	cfg["model_list"] = models

	b, _ := json.MarshalIndent(cfg, "", "  ")
	ssh.run(fmt.Sprintf("cat > /root/.picoclaw/config.json <<'EOF'\n%s\nEOF", string(b)))

	// Restart picoclaw
	ssh.run("killall picoclaw 2>/dev/null; rm -f /root/.picoclaw/.picoclaw.pid; sleep 1; /etc/init.d/S99picoclaw start 2>/dev/null")
	fmt.Printf("Model changed: %s -> %s\n", old, modelID)
	fmt.Println("picoclaw restarted.")
}

func ensureOpenRouterPrefix(model string) string {
	if strings.HasPrefix(model, "openrouter/") {
		return model
	}
	return "openrouter/" + model
}

func modelListCmd(ssh *sshClient) {
	// Read current API key from security.yml to query OpenRouter
	keyLine := strings.TrimSpace(ssh.run(`grep 'sk-or-' /root/.picoclaw/.security.yml 2>/dev/null | head -1`))
	apiKey := strings.Trim(strings.TrimSpace(strings.TrimPrefix(keyLine, "- ")), `"`)
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
	fmt.Printf("\n%d models shown. Use 'quick-claw model set <model-id>' to switch.\n", count)
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

	boardIP := findBoardIP(sshpassBin, stateDir)
	if boardIP == "" {
		fatal("Board not reachable. Check USB or Wi-Fi connection.")
	}
	fmt.Printf("Board: %s\n", boardIP)
	ssh := newSSH(sshpassBin, boardUser, boardPass, boardIP)

	buildUI := true
	buildBridge := true
	for _, arg := range os.Args[2:] {
		switch arg {
		case "--ui":
			buildBridge = false
		case "--bridge":
			buildUI = false
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
		fmt.Print("Building pico-bridge... ")
		cmd := exec.Command("go", "build", "-o", filepath.Join(boardBinDir, "pico-bridge"), "./cmd/pico-bridge/")
		cmd.Dir = scriptDir
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=riscv64")
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
		fmt.Print("Deploying pico-bridge... ")
		ssh.run("killall pico-bridge 2>/dev/null; sleep 1")
		ssh.scp(filepath.Join(boardBinDir, "pico-bridge"), "/usr/local/bin/pico-bridge")
		ssh.run("chmod +x /usr/local/bin/pico-bridge && nohup /usr/local/bin/pico-bridge > /var/log/pico-bridge.log 2>&1 &")
		fmt.Println("ok")
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
}

func newSSH(sshpassBin, user, pass, host string) *sshClient {
	return &sshClient{sshpassBin: sshpassBin, user: user, pass: pass, host: host}
}

func (s *sshClient) run(cmd string) string {
	args := []string{"-p", s.pass, "ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-o", "LogLevel=ERROR",
		fmt.Sprintf("%s@%s", s.user, s.host),
		cmd,
	}
	out, _ := exec.Command(s.sshpassBin, args...).CombinedOutput()
	return string(out)
}

func (s *sshClient) scp(localPath, remotePath string) {
	args := []string{"-p", s.pass, "scp",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		localPath,
		fmt.Sprintf("%s@%s:%s", s.user, s.host, remotePath),
	}
	exec.Command(s.sshpassBin, args...).Run()
}

func (s *sshClient) scpDir(localDir, remoteDir string) {
	args := []string{"-p", s.pass, "scp", "-r",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		localDir + "/.",
		fmt.Sprintf("%s@%s:%s", s.user, s.host, remoteDir),
	}
	exec.Command(s.sshpassBin, args...).Run()
}

// --- Config builders ---

func buildPicoclawConfig(modelName, model, apiBase, apiKey string) string {
	cfg := map[string]any{
		"version": 2,
		"agents": map[string]any{
			"defaults": map[string]any{
				"workspace":                   "/root/.picoclaw/workspace",
				"model_name":                  modelName,
				"max_tokens":                  16384,
				"max_tool_iterations":         30,
				"summarize_message_threshold": 15,
			},
		},
		"model_list": []map[string]any{
			{"model_name": modelName, "model": model, "api_base": apiBase, "api_keys": []string{apiKey}},
		},
		"gateway": map[string]any{
			"host":      "0.0.0.0",
			"port":      18790,
			"log_level": "warn",
		},
		"tools": map[string]any{
			"web":  map[string]any{"enabled": true, "duckduckgo": map[string]any{"enabled": true, "max_results": 3}},
			"exec": map[string]any{"enabled": true, "timeout_seconds": 30},
			"cron": map[string]any{"enabled": true},
		},
		"channels": map[string]any{
			"pico": map[string]any{
				"enabled":           true,
				"token":             "quickclaw",
				"allow_token_query": true,
				"allow_origins":     []string{"*"},
				"ping_interval":     30,
				"read_timeout":      60,
				"max_connections":   100,
			},
		},
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return string(b)
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
