package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
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

func main() {
	lang := "ko"
	if len(os.Args) > 1 && os.Args[1] == "--en" {
		lang = "en"
	}
	m := newMsg(lang)

	scriptDir, _ := os.Executable()
	scriptDir = filepath.Dir(scriptDir)
	if d, err := os.Getwd(); err == nil {
		scriptDir = d
	}
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

	// 5. Install picoclaw
	step(5, totalSteps, m.t("picoclaw 설치 중...", "Installing picoclaw..."))
	if _, err := os.Stat(picoclawBin); os.IsNotExist(err) {
		fatal(m.t("picoclaw 바이너리를 찾을 수 없습니다: "+picoclawBin, "picoclaw binary not found: "+picoclawBin))
	}

	installedVer := strings.TrimSpace(ssh.run("/usr/local/bin/picoclaw version 2>/dev/null | grep -o 'picoclaw [0-9.]*' | awk '{print $2}'"))
	localVer := "0.2.5"
	if installedVer == localVer {
		fmt.Printf("  picoclaw %s %s\n", localVer, m.t("이미 설치됨", "already installed"))
	} else {
		ssh.run("mkdir -p /usr/local/bin")
		ssh.scp(picoclawBin, "/usr/local/bin/picoclaw")
		ssh.run("chmod +x /usr/local/bin/picoclaw")
		fmt.Printf("  picoclaw %s %s\n", localVer, m.t("설치 완료", "installed"))
	}

	// 6. OpenRouter API key
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
		apiKey = readSecret(m.t("API 키 입력: ", "Enter API key: "))
		if apiKey == "" {
			fatal(m.t("API 키가 입력되지 않았습니다.", "No API key provided."))
		}
		fmt.Printf("  %s: %s\n", m.t("API 키", "API key"), maskKey(apiKey))
	}

	// 7. Telegram bot token
	step(7, totalSteps, m.t("텔레그램 봇 설정...", "Configuring Telegram bot..."))
	telegramToken := ""
	existingTG := strings.TrimSpace(ssh.run("grep 'token:' /root/.picoclaw/.security.yml 2>/dev/null | grep -v '{}' | head -1"))
	skipTelegram := false
	if existingTG != "" && !strings.Contains(existingTG, "your-") {
		fmt.Printf("  %s\n", m.t("텔레그램 봇이 이미 설정되어 있습니다.", "Telegram bot already configured."))
		if !promptYN(m.t("다시 설정하시겠습니까?", "Reconfigure?")) {
			skipTelegram = true
		}
	}
	if !skipTelegram {
		fmt.Printf("\n  %s\n", m.t("텔레그램 봇 토큰이 필요합니다.", "A Telegram bot token is required."))
		fmt.Printf("  %s:\n", m.t("발급 방법", "How to get one"))
		fmt.Printf("    1. %s @BotFather %s\n", m.t("텔레그램에서", "Open"), m.t("열기", "@BotFather on Telegram"))
		fmt.Printf("    2. /newbot %s\n", m.t("명령 입력", "command"))
		fmt.Printf("    3. %s\n\n", m.t("토큰 복사", "Copy the token"))
		telegramToken = readSecret(m.t("봇 토큰 입력: ", "Enter bot token: "))
		if telegramToken == "" {
			fmt.Printf("  %s\n", m.t("텔레그램 설정을 건너뜁니다.", "Skipping Telegram setup."))
			skipTelegram = true
		} else {
			fmt.Printf("  %s: %s\n", m.t("봇 토큰", "Bot token"), maskKey(telegramToken))
		}
	}

	// Write config + security to board
	if !skipAPIKey || !skipTelegram {
		configJSON := buildConfig(picoclawModelName, picoclawModel, picoclawAPIBase, !skipTelegram)
		securityYML := buildSecurity(picoclawModelName, apiKey, telegramToken)

		ssh.run(fmt.Sprintf(`/usr/local/bin/picoclaw onboard 2>/dev/null || true
cat > /root/.picoclaw/config.json <<'CFGEOF'
%s
CFGEOF
cat > /root/.picoclaw/.security.yml <<'SECEOF'
%s
SECEOF
chmod 600 /root/.picoclaw/.security.yml`, configJSON, securityYML))
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
		fmt.Printf("  %s\n", m.t("gateway 시작 실패 — 로그 확인: ssh root@"+boardIP+" 'cat /var/log/picoclaw.log'",
			"Gateway failed — check: ssh root@"+boardIP+" 'cat /var/log/picoclaw.log'"))
	}

	// Swap
	ssh.run(`if ! grep -q '/swapfile' /proc/swaps 2>/dev/null; then
  if [ ! -f /swapfile ]; then
    dd if=/dev/zero of=/swapfile bs=1M count=256 2>/dev/null
    chmod 600 /swapfile; mkswap /swapfile >/dev/null 2>&1
  fi
  swapon /swapfile 2>/dev/null || true
  grep -q '/swapfile' /etc/fstab 2>/dev/null || echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi`)

	// Done
	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("  %s\n", m.t("Quick Claw 설정 완료!", "Quick Claw Setup Complete!"))
	fmt.Println("========================================")
	fmt.Println()
	fmt.Printf("  USB:     %s\n", boardIP)
	fmt.Printf("  Wi-Fi:   %s\n", wifiIP)
	fmt.Println()
	if !skipTelegram && telegramToken != "" {
		fmt.Printf("  %s\n", m.t(
			"텔레그램에서 봇에게 메시지를 보내보세요!",
			"Send a message to your bot on Telegram!",
		))
	}
	fmt.Printf("  %s:\n", m.t("SSH 접속", "SSH access"))
	fmt.Printf("    ssh root@%s\n", wifiIP)
	fmt.Println()
	fmt.Printf("  %s:\n", m.t("상태 확인", "Check status"))
	fmt.Printf("    ssh root@%s '/etc/init.d/S99picoclaw status'\n", wifiIP)
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

func buildConfig(modelName, model, apiBase string, telegramEnabled bool) string {
	cfg := map[string]any{
		"version": 2,
		"agents": map[string]any{
			"defaults": map[string]any{
				"workspace":                    "/root/.picoclaw/workspace",
				"model_name":                   modelName,
				"max_tokens":                   16384,
				"max_tool_iterations":          30,
				"summarize_message_threshold":  15,
			},
		},
		"model_list": []map[string]any{
			{"model_name": modelName, "model": model, "api_base": apiBase},
		},
		"channels": map[string]any{
			"telegram": map[string]any{
				"enabled":   telegramEnabled,
				"allow_from": []string{},
				"typing":    map[string]any{"enabled": true},
				"placeholder": map[string]any{
					"enabled": true,
					"text":    []string{"..."},
				},
				"streaming": map[string]any{
					"enabled":          true,
					"throttle_seconds": 3,
					"min_growth_chars": 100,
				},
			},
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

func buildSecurity(modelName, apiKey, telegramToken string) string {
	var lines []string
	if apiKey != "" {
		lines = append(lines, "model_list:")
		lines = append(lines, fmt.Sprintf("  %s:", modelName))
		lines = append(lines, "    api_keys:")
		lines = append(lines, fmt.Sprintf("      - \"%s\"", apiKey))
	}
	if telegramToken != "" {
		lines = append(lines, "channels:")
		lines = append(lines, "  telegram:")
		lines = append(lines, fmt.Sprintf("    token: \"%s\"", telegramToken))
	}
	return strings.Join(lines, "\n")
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
