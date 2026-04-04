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

	picoclawModelName = "qwen-free"
	picoclawModel     = "openrouter/qwen/qwen3.6-plus:free"
	picoclawAPIBase   = "https://openrouter.ai/api/v1"
)

type config struct {
	APIBaseURL     string
	RegisterSecret string
	CFDomain       string
}

func loadConfig() config {
	return config{
		APIBaseURL:     envOr("QC_API_URL", "https://quick-claw.pages.dev"),
		RegisterSecret: envOr("QC_REGISTER_SECRET", ""),
		CFDomain:       envOr("QC_DOMAIN", "dawn.kim"),
	}
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup":
			runSetup()
		case "invite":
			runInvite()
		case "users":
			runUsers()
		case "status":
			runStatus()
		case "update":
			runUpdate()
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
	fmt.Println("  invite   Generate invite QR code")
	fmt.Println("  users    Manage allowed users")
	fmt.Println("  status   Check board and tunnel status")
	fmt.Println("  update   OTA update")
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
	if wifiPass == "" {
		fatal(m.t("키체인에서 비밀번호를 가져올 수 없습니다.", "Failed to retrieve password from keychain."))
	}
	fmt.Printf("  %s: %s\n", m.t("비밀번호", "Password"), maskString(wifiPass))

	// 3. Configure Wi-Fi on board
	step(3, totalSteps, m.t("보드에 Wi-Fi 설정 중...", "Configuring Wi-Fi on board..."))
	wpaConf := fmt.Sprintf(`ctrl_interface=/var/run/wpa_supplicant
ap_scan=1
network={
  ssid="%s"
  scan_ssid=1
  key_mgmt=WPA-PSK
  psk="%s"
}`, ssid, wifiPass)

	ssh.run(fmt.Sprintf(`killall wpa_supplicant 2>/dev/null || true
cat > /etc/wpa_supplicant.conf <<'WPAEOF'
%s
WPAEOF
wpa_supplicant -i wlan0 -c /etc/wpa_supplicant.conf -B
sleep 2
udhcpc -i wlan0 -q -n -t 5 -T 3 2>/dev/null || true
sleep 3`, wpaConf))

	// 4. Verify Wi-Fi
	step(4, totalSteps, m.t("Wi-Fi 연결 확인 중...", "Verifying Wi-Fi connection..."))
	wifiIP := strings.TrimSpace(ssh.run("ip -4 addr show wlan0 2>/dev/null | grep 'inet ' | awk '{print $2}' | cut -d/ -f1"))
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
		configJSON := buildPicoclawConfig(picoclawModelName, picoclawModel, picoclawAPIBase)
		securityYML := buildPicoclawSecurity(picoclawModelName, apiKey)

		ssh.run(fmt.Sprintf(`/usr/local/bin/picoclaw onboard 2>/dev/null || true
cat > /root/.picoclaw/config.json <<'CFGEOF'
%s
CFGEOF
cat > /root/.picoclaw/.security.yml <<'SECEOF'
%s
SECEOF
chmod 600 /root/.picoclaw/.security.yml`, configJSON, securityYML))
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

	// 8. Gateway autostart + swap
	step(8, totalSteps, m.t("picoclaw gateway 시작 중...", "Starting picoclaw gateway..."))

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

// --- Subcommands (stubs) ---

func runInvite()  { fmt.Println("TODO: invite") }
func runUsers()   { fmt.Println("TODO: users") }
func runStatus()  { fmt.Println("TODO: status") }
func runUpdate()  { fmt.Println("TODO: update") }

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

// --- Config builders ---

func buildPicoclawConfig(modelName, model, apiBase string) string {
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
			{"model_name": modelName, "model": model, "api_base": apiBase},
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
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return string(b)
}

func buildPicoclawSecurity(modelName, apiKey string) string {
	return fmt.Sprintf(`model_list:
  %s:
    api_keys:
      - "%s"`, modelName, apiKey)
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
