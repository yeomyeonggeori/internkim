package cli

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
	"strings"
	"sync"
	"time"

	"golang.org/x/term"

	"github.com/anthropic-lab/internkim/internal/blueclawworkspace"
	auth "github.com/anthropic-lab/internkim/internal/google/browser"
	internkimlab "github.com/anthropic-lab/internkim/internal/lab"
	setup "github.com/anthropic-lab/internkim/internal/provisioning/steps"
	"github.com/anthropic-lab/internkim/internal/runtime/blueclaw"
)

var (
	boardUser = "root"
	boardPass = ""

	// Armbian Trixie Minimal images per board
	armbianImages = map[string]string{
		"rpi":       "https://dl.armbian.com/rpi4b/Trixie_current_minimal",     // RPi 3/4/5
		"orangepi5": "https://dl.armbian.com/orangepi5/Trixie_current_minimal", // Orange Pi 5 (RK3588S)
	}
)

const jetsonDefaultUser = "internkim"
const jetsonDefaultPassword = "blueclaw"

type config struct {
	APIBaseURL     string
	RegisterSecret string
	CFDomain       string
}

func loadConfig() config {
	loadEnvFile()
	return config{
		APIBaseURL:     envOr("INTERNKIM_API_URL", "https://api.intern.kim"),
		RegisterSecret: envOr("INTERNKIM_REGISTER_SECRET", ""),
		CFDomain:       envOr("INTERNKIM_DOMAIN", "intern.kim"),
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
		case "model":
			runModel()
		case "invite":
			runInvite()
		case "users":
			runUsers()
		case "reset":
			runReset()
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
	fmt.Println("  model    Manage LLM model (current/set/list)")
	fmt.Println("  invite   Generate invite QR code")
	fmt.Println("  users    Manage allowed users")
	fmt.Println("  reset    Reset board runtime data")
	fmt.Println("  status   Check board and tunnel status")
	fmt.Println("  update   OTA update")
	fmt.Println("  deploy   Build and deploy web UI + board-bridge to board")
	fmt.Println("  doctor   Check host dependencies")
	fmt.Println("  verify   Run API, Mattermost, and browser verification")
	fmt.Println("  lab      Run Tart-based Blueclaw-aligned lab workflows")
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
	fmt.Println("  --only <steps>       Run only selected setup steps, for example web or binaries,services")
	fmt.Println("  --force              Re-run selected steps even when state says they are complete")
	fmt.Println("  --force-all          Re-run every selected setup step")
	fmt.Println("  --host <ip>          Override the saved board IP")
	fmt.Println("  --user <name>        Override the SSH user")
	fmt.Println("  --password <value>   Override the SSH password")
	fmt.Println("  --plan               Print the selected setup plan")
	fmt.Println("  --list-steps         Print available setup steps")
	fmt.Println("  --sim                Run the Tart simulation flow")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  internkim setup --only web")
	fmt.Println("  internkim setup --only web,binaries,services --force")
}

// --- Model management ---

func runModel() {
	sub := ""
	if len(os.Args) > 2 {
		sub = os.Args[2]
	}

	scriptDir, _ := os.Getwd()
	sshpassBin := filepath.Join(scriptDir, "bin", "sshpass")

	stateDir := internkimHomeDir()
	boardIP, _ := detectBoardRPi(sshpassBin, stateDir)
	if boardIP == "" {
		fatal("Board not found. Run 'internkim setup' first or ensure the board is on the network.")
	}

	ssh := newSSH(sshpassBin, boardUser, "", boardIP)

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
	raw := strings.TrimSpace(ssh.run("cat " + blueclaw.BlueclawRuntimeConfigPath + " 2>/dev/null"))
	var document map[string]any
	_ = json.Unmarshal([]byte(raw), &document)
	model := ""
	if languageModel, ok := document["languageModel"].(map[string]any); ok {
		if capabilityModel, ok := languageModel["capability"].(map[string]any); ok {
			if value, ok := capabilityModel["model"].(string); ok {
				model = strings.TrimSpace(value)
			}
		}
	}
	if model == "" {
		fmt.Println("No model configured.")
		return
	}
	fmt.Printf("Model: %s\n", model)
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
	ssh.scp(temporaryPath, blueclaw.BlueclawRuntimeConfigPath)
	ssh.run("chown root:" + blueclaw.BlueclawUser + " " + blueclaw.BlueclawRuntimeConfigPath + " && chmod 640 " + blueclaw.BlueclawRuntimeConfigPath)
	ssh.run("systemctl restart " + blueclaw.BlueclawServiceName + " 2>/dev/null")
	fmt.Printf("Model changed to: %s\n", modelID)
	fmt.Println("blueclaw restarted.")
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
	stateDir := internkimHomeDir()
	scriptDir, _ := os.Getwd()
	binDir := filepath.Join(scriptDir, "bin")
	sshpassBin := filepath.Join(binDir, "sshpass")
	boardBinDir := filepath.Join(scriptDir, "build", "board-bin")

	var ssh *sshClient
	boardIP := argString("--host", "")
	if boardIP == "" {
		boardIP = findBoardIP(sshpassBin, stateDir)
		if boardIP == "" {
			fatal("Board not reachable. Check USB or Wi-Fi connection.")
		}
	}
	ssh = newSSH(sshpassBin, boardUser, "", boardIP)
	fmt.Printf("Board: %s\n", boardIP)

	boardTools := []string{"download"}

	fmt.Print("Installing skill dependencies... ")
	ssh.run("pip3 install --quiet fpdf2 pypdf 2>&1 | tail -1")
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
	ssh.run(`for skill in calendar create-gws-file simple-slides; do
  filePath="/root/.blueclaw/workspace/skills/$skill/scripts/gas-call"
  [ -f "$filePath" ] && chmod +x "$filePath"
done
chown -R blueclaw:blueclaw /root/.blueclaw/workspace/skills 2>/dev/null || true`)
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
	ssh.run("rm -f /usr/local/bin/send-file /root/.blueclaw/workspace/bin/send-file && rm -rf /root/.blueclaw/workspace/skills/share-file")

	fmt.Println("Deploy complete.")
}

func findBoardIP(sshpassBin, stateDir string) string {
	ip, _ := detectBoardRPi(sshpassBin, stateDir)
	return ip
}

func runInvite() { fmt.Println("TODO: invite") }
func runUsers()  { fmt.Println("TODO: users") }
func runStatus() {
	stateDir := internkimHomeDir()
	lang := "ko"
	if containsArg("--en") {
		lang = "en"
	}
	m := newMsg(lang)

	labVirtualMachineIPAddress := resolveLabVirtualMachineIPAddress()
	if labVirtualMachineIPAddress != "" {
		fmt.Printf("=== %s (Tart VM: %s) ===\n\n", m.t("기기 상태", "Device Status"), labVirtualMachineIPAddress)
		printBoardStatus(m, labVirtualMachineIPAddress, stateDir)
		return
	}

	// Try RPi5
	fmt.Printf("  %s", m.t("보드 검색 중...", "Scanning for board..."))
	boardIP, sshOK := detectBoardRPi("", stateDir)
	if boardIP != "" && sshOK {
		fmt.Printf("\r=== %s (RPi5: %s) ===\n\n", m.t("기기 상태", "Device Status"), boardIP)
		printBoardStatus(m, boardIP, stateDir)
		return
	}
	if boardIP != "" && !sshOK {
		fmt.Printf("\r  %s %s\n", m.t("보드 발견 (SSH 연결 불가):", "Board found (SSH unreachable):"), boardIP)
		fmt.Printf("  %s\n", m.t("LED 상태로 의미가 달라집니다:", "Meaning depends on the LED:"))
		fmt.Printf("    %s: %s\n", m.t("빠른 점멸", "Fast blink"), m.t("프로비저닝 진행 중 — 잠시 후 다시 시도", "provisioning in progress — retry shortly"))
		fmt.Printf("    %s: %s\n", m.t("하트비트", "Heartbeat"), m.t("프로비저닝 완료 — IP가 바뀌었거나 Wi-Fi/방화벽 문제일 수 있습니다", "provisioning complete — IP may have changed, or Wi-Fi/firewall blocking SSH"))
		fmt.Printf("    %s: %s\n", m.t("느린 점멸", "Slow blink"), m.t("프로비저닝 실패 — /var/log/internkim-firstboot.log 확인", "provisioning failed — check /var/log/internkim-firstboot.log"))
		fmt.Printf("  %s\n", m.t("하트비트라면: `internkim lab status` 로 VM 상태를 확인하거나, SD에서 board-ip 파일을 확인하고 재시도하세요.", "If heartbeat: check the Tart VM with `internkim lab status`, or re-check the board-ip file on the SD card and retry."))
		return
	}

	fmt.Printf("\r  %s\n", m.t(
		"기기를 찾을 수 없습니다.\n  - RPi5: Wi-Fi 연결 확인\n  - Tart Lab: internkim lab vm-up",
		"Device not found.\n  - RPi5: Check Wi-Fi\n  - Tart Lab: start with `internkim lab vm-up`",
	))
}

func printBoardStatus(m *msg, ip string, stateDir string) {
	saveState(stateDir, "board_ip", ip)
	sshCmd := func(cmd string) string {
		out, _ := exec.Command("ssh",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "ConnectTimeout=5",
			"-o", "LogLevel=ERROR",
			"root@"+ip, cmd).CombinedOutput()
		return strings.TrimSpace(string(out))
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
		{"cloudflared", "Cloudflared"},
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
}
func runUpdate() { fmt.Println("TODO: update") }

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

// gwsSkillsInstallScript sparse-clones (or updates) googleworkspace/cli into
// /opt/gws-cli and symlinks skills/gws-* into the blueclaw workspace skills dir.
// Matches the OpenClaw setup pattern documented in the gws-cli README.
const gwsSkillsInstallScript = `mkdir -p /opt /root/.blueclaw/workspace/skills
COUNT=0
if ! command -v git >/dev/null 2>&1; then
  apt-get update -qq >/dev/null 2>&1 && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq git >/dev/null 2>&1 || true
fi
if ! command -v git >/dev/null 2>&1; then
  echo "FATAL: git missing for gws-skills install" >&2
  exit 1
fi
if [ -d /opt/gws-cli/.git ]; then
  git -C /opt/gws-cli fetch --depth 1 origin main >/dev/null 2>&1 && \
    git -C /opt/gws-cli reset --hard origin/main >/dev/null 2>&1 || {
      echo "FATAL: gws-skills update failed" >&2
      exit 1
    }
else
  rm -rf /opt/gws-cli
  git clone --depth 1 --filter=blob:none --sparse \
    https://github.com/googleworkspace/cli.git /opt/gws-cli >/dev/null 2>&1 || {
      echo "FATAL: gws-skills clone failed" >&2
      exit 1
    }
  git -C /opt/gws-cli sparse-checkout set skills >/dev/null 2>&1 || {
    echo "FATAL: gws-skills sparse checkout failed" >&2
    exit 1
  }
fi
for d in /opt/gws-cli/skills/gws-*; do
  [ -d "$d" ] || continue
  ln -sfn "$d" "/root/.blueclaw/workspace/skills/$(basename "$d")"
  COUNT=$((COUNT + 1))
done
if [ "$COUNT" -eq 0 ]; then
  echo "FATAL: gws-skills symlinked 0 directories" >&2
  exit 1
fi
for skill in calendar create-gws-file simple-slides; do
  filePath="/root/.blueclaw/workspace/skills/$skill/scripts/gas-call"
  [ -f "$filePath" ] && chmod +x "$filePath"
done
chown -R blueclaw:blueclaw /root/.blueclaw/workspace/skills 2>/dev/null || true
echo "gws-skills: $COUNT symlinked"
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

	// Extract specific file from tar.gz
	return extractFromTarGz(resp.Body, localPath, tarEntry)
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
which pg_isready 2>/dev/null && echo already || {
  apt-get update -qq
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq postgresql postgresql-contrib python3 || echo apt_install_failed
}`)
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

	// Download and install Mattermost (arm64 tarball)
	fmt.Printf("  %s\n", m.t("Mattermost 버전 확인 중...", "Checking Mattermost version..."))
	installOut := ssh.run(`
cd /tmp
MMVER=$(curl -s https://api.github.com/repos/mattermost/mattermost/releases/latest 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"v//;s/".*//')
[ -z "$MMVER" ] && MMVER="10.9.1"
echo "MMVER=${MMVER}"
URL="https://releases.mattermost.com/${MMVER}/mattermost-${MMVER}-linux-arm64.tar.gz"
echo "Downloading Mattermost ${MMVER}..."
curl -fsSL -o /tmp/mattermost.tar.gz "$URL" 2>&1 | tail -1 && echo "download_ok" || echo "download_failed"`)
	mmver := ""
	for _, line := range strings.Split(installOut, "\n") {
		if strings.HasPrefix(line, "MMVER=") {
			mmver = strings.TrimPrefix(strings.TrimSpace(line), "MMVER=")
		}
	}
	if mmver != "" {
		fmt.Printf("  %s: %s\n", m.t("Mattermost 버전", "Mattermost version"), mmver)
	}
	if strings.Contains(installOut, "download_failed") {
		fmt.Printf("  ERROR: %s\n", m.t("Mattermost 다운로드 실패 — 건너뜀", "Mattermost download failed — skipping"))
		return fmt.Errorf("Mattermost download failed")
	}
	fmt.Printf("  %s\n", m.t("Mattermost 다운로드 완료, 설치 중...", "Download complete, installing..."))

	installResult := ssh.run(fmt.Sprintf(`
cd /tmp && tar -xzf mattermost.tar.gz 2>&1 && echo "tar_ok" || echo "tar_failed"
rm -rf /opt/mattermost
mv /tmp/mattermost /opt/mattermost
mkdir -p /opt/mattermost/data
id mattermost &>/dev/null || useradd --system --user-group mattermost
chown -R mattermost:mattermost /opt/mattermost
chmod -R g+w /opt/mattermost

# Write config
cd /opt/mattermost
cp config/config.defaults.json config/config.json 2>/dev/null && echo "defaults_used" || echo "defaults_missing"
DB_PASS="%s"
MATTERMOST_DB_PASS="$DB_PASS" MATTERMOST_SITE_URL="http://localhost:8065" python3 - <<'PY' && chown mattermost:mattermost config/config.json && echo "config_ok" || echo "config_failed"
import json
import os
from pathlib import Path

path = Path("config/config.json")
document = json.loads(path.read_text())
document.setdefault("SqlSettings", {})
document.setdefault("ServiceSettings", {})
document.setdefault("TeamSettings", {})
document["SqlSettings"]["DriverName"] = "postgres"
document["SqlSettings"]["DataSource"] = "postgres://mmuser:%%s@localhost/mattermost?sslmode=disable&connect_timeout=10" %% os.environ["MATTERMOST_DB_PASS"]
document["ServiceSettings"]["SiteURL"] = os.environ["MATTERMOST_SITE_URL"]
document["TeamSettings"]["TeammateNameDisplay"] = "full_name"
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
systemctl start mattermost 2>&1 && echo "start_ok" || echo "start_failed"`, mmDBPass))

	if strings.Contains(installResult, "tar_failed") {
		fmt.Printf("  ERROR: %s\n", m.t("압축 해제 실패", "Failed to extract tarball"))
		return fmt.Errorf("failed to extract Mattermost tarball")
	}
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

	// 3. Enable personal access tokens + bot accounts in config
	configBody, _ := json.Marshal(map[string]any{
		"ServiceSettings": map[string]any{
			"EnableUserAccessTokens":   true,
			"EnableBotAccountCreation": true,
		},
		"TeamSettings": map[string]any{
			"TeammateNameDisplay": "full_name",
		},
		"EmailSettings": map[string]string{
			"PushNotificationContents": "id_loaded",
		},
	})
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
			"first_name": "김인턴",
			"nickname":   "Intern Kim",
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
		for attempt := 0; attempt < 10; attempt++ {
			chCode, chResp := mmAPI("GET", "/api/v4/teams/"+teamResult.ID+"/channels/name/town-square", nil, adminToken)
			if chCode >= 200 && chCode < 300 {
				var chResult struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal(chResp, &chResult); err == nil && chResult.ID != "" {
					channelID = chResult.ID
					break
				}
			}
			time.Sleep(1 * time.Second)
		}
		if channelID == "" {
			fatal("town-square channel lookup failed after retries")
		}

		// Add bot to team
		if botResult.UserID != "" {
			botMemberBody, _ := json.Marshal(map[string]string{"team_id": teamResult.ID, "user_id": botResult.UserID})
			mmAPI("POST", "/api/v4/teams/"+teamResult.ID+"/members", botMemberBody, adminToken)
		}
	}

	// 8. Store credentials
	deviceURL := loadState(stateDir, "device_url")
	if deviceURL == "" {
		deviceURL = localURL
	}
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

func runLab() {
	if errorValue := runLabArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runSim() {
	labArguments := []string{"scenario-e2e"}
	if len(os.Args) > 2 {
		switch os.Args[2] {
		case "reset":
			if errorValue := runLabArguments([]string{"vm-down"}); errorValue != nil {
				fmt.Printf("lab reset cleanup warning: %v\n", errorValue)
			}
			labArguments = []string{"scenario-e2e"}
		case "stop":
			labArguments = []string{"vm-down"}
		case "ssh":
			labArguments = append([]string{"vm-ssh"}, os.Args[3:]...)
		case "status":
			labArguments = []string{"status"}
		default:
			labArguments = append([]string{"scenario-e2e"}, os.Args[2:]...)
		}
	}

	if errorValue := runLabArguments(labArguments); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runSetupSimulation(setupArguments []string) {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		fatal(errorValue.Error())
	}

	configurationPath := internkimlab.DefaultConfigurationPath(repositoryRootPath)
	configuration, errorValue := internkimlab.LoadConfiguration(configurationPath)
	if errorValue != nil {
		fatal(errorValue.Error())
	}

	if errorValue := ensureSimulationDependencies(configuration); errorValue != nil {
		fatal(errorValue.Error())
	}

	service := internkimlab.NewService(
		configuration,
		internkimlab.OperatingSystemCommandRunner{},
		repositoryRootPath,
	)

	executablePath, errorValue := currentExecutablePath()
	if errorValue != nil {
		fatal(errorValue.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	if containsSetupPlan(setupArguments) {
		if errorValue := service.PrintSimulationPlan(ctx, executablePath, setupArguments); errorValue != nil {
			fatal(errorValue.Error())
		}
		return
	}

	filteredSetupArguments, shouldVerify, shouldVerifyBrowser := splitSimulationVerifyArguments(setupArguments)
	setupArguments = filteredSetupArguments

	if errorValue := service.Setup(ctx, executablePath, setupArguments); errorValue != nil {
		fatal(errorValue.Error())
	}

	if !shouldVerify && !shouldVerifyBrowser {
		return
	}
	virtualMachineIPAddress, errorValue := service.VirtualMachineIPAddress(ctx)
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	verifyArguments := []string{"api", "--host", virtualMachineIPAddress, "--user", configuration.VirtualMachine.SSHUsername, "--password", configuration.VirtualMachine.SSHPassword}
	if errorValue := runVerifyArguments(verifyArguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	verifyArguments[0] = "mattermost"
	if errorValue := runVerifyArguments(verifyArguments); errorValue != nil {
		fatal(errorValue.Error())
	}
	if shouldVerifyBrowser {
		if errorValue := runVerifyArguments([]string{"browser", "--local", "--host", virtualMachineIPAddress, "--user", configuration.VirtualMachine.SSHUsername, "--password", configuration.VirtualMachine.SSHPassword}); errorValue != nil {
			fatal(errorValue.Error())
		}
		if errorValue := runVerifyArguments([]string{"browser", "--public", "--host", virtualMachineIPAddress, "--user", configuration.VirtualMachine.SSHUsername, "--password", configuration.VirtualMachine.SSHPassword}); errorValue != nil {
			fatal(errorValue.Error())
		}
	}
}

func containsSetupPlan(arguments []string) bool {
	for _, argument := range arguments {
		if argument == "--plan" {
			return true
		}
	}

	return false
}

func splitSimulationVerifyArguments(arguments []string) ([]string, bool, bool) {
	var filteredArguments []string
	shouldVerify := false
	shouldVerifyBrowser := false
	for _, argument := range arguments {
		switch argument {
		case "--verify":
			shouldVerify = true
		case "--verify-browser":
			shouldVerify = true
			shouldVerifyBrowser = true
		default:
			filteredArguments = append(filteredArguments, argument)
		}
	}
	return filteredArguments, shouldVerify, shouldVerifyBrowser
}

type hostDependency struct {
	name        string
	purpose     string
	installHint string
}

func simulationDependencies(configuration internkimlab.Configuration) []hostDependency {
	return []hostDependency{
		{
			name:        configuration.VirtualMachine.Tart.BinaryPath,
			purpose:     "Tart VM simulation",
			installHint: "brew install cirruslabs/cli/tart",
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
		case "--force", "--force-all", "--plan", "--list-steps", "--en", "--non-interactive", "--verify", "--verify-browser", "--with-google":
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

// --- Device registration ---

type registerResponse struct {
	DeviceID      string `json:"device_id"`
	TunnelToken   string `json:"tunnel_token"`
	URL           string `json:"url"`
	MattermostURL string `json:"mattermost_url"`
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

func registerDevice(configuration config, deviceID, deviceSecret, adminEmail string) (*registerResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"device_id":     deviceID,
		"device_secret": deviceSecret,
		"admin_email":   adminEmail,
	})

	req, _ := http.NewRequest("POST", configuration.APIBaseURL+"/api/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+configuration.RegisterSecret)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
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

func registerDeviceWithCollisionRetry(configuration config, stateDir, deviceID, adminEmail string) (*registerResponse, error) {
	deviceSecret := loadOrCreateDeviceSecret(stateDir)

	for attempt := 0; attempt < 5; attempt++ {
		response, registerError := registerDevice(configuration, deviceID, deviceSecret, adminEmail)
		if registerError == nil {
			saveState(stateDir, "device_id", response.DeviceID)
			return response, nil
		}

		var httpError *registrationHTTPError
		if !errors.As(registerError, &httpError) || httpError.statusCode != http.StatusConflict {
			return nil, registerError
		}

		deviceID, deviceSecret = resetDeviceIdentity(stateDir)
		fmt.Printf("  Device ID collision detected; retrying with %s\n", deviceID)
	}

	return nil, errors.New("device_id collision retry limit reached")
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
		"board_ip",
		"board_wifi_ip",
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

func loadOrCreateDeviceID(stateDir string) string {
	id := loadState(stateDir, "device_id")
	if id != "" {
		return id
	}
	id = randomHexString(16)
	saveState(stateDir, "device_id", id)
	return id
}

func loadOrCreateDeviceSecret(stateDir string) string {
	secret := loadState(stateDir, "device_secret")
	if secret != "" {
		return secret
	}
	secret = randomHexString(32)
	saveState(stateDir, "device_secret", secret)
	return secret
}

func resetDeviceIdentity(stateDir string) (string, string) {
	deviceID := randomHexString(16)
	deviceSecret := randomHexString(32)
	saveState(stateDir, "device_id", deviceID)
	saveState(stateDir, "device_secret", deviceSecret)
	saveState(stateDir, "device_url", "")
	saveState(stateDir, "tunnel_token", "")
	return deviceID, deviceSecret
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

	// 2. Subnet SSH scan
	subnet := loadState(stateDir, "subnet")
	if subnet == "" {
		// Try to detect from default route
		if out, err := exec.Command("sh", "-c", "route get default 2>/dev/null | awk '/gateway/{print $2}'").Output(); err == nil {
			gw := strings.TrimSpace(string(out))
			if parts := strings.Split(gw, "."); len(parts) == 4 {
				subnet = strings.Join(parts[:3], ".")
			}
		}
	}
	if subnet != "" {
		type result struct {
			ip  string
			ssh bool
		}
		found := make(chan result, 254)
		var wg sync.WaitGroup
		for i := 2; i <= 254; i++ {
			ip := fmt.Sprintf("%s.%d", subnet, i)
			// Skip already-tried candidates
			skip := false
			for _, c := range candidates {
				if c == ip {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
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
		// Wait with timeout
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		select {
		case r := <-found:
			saveState(stateDir, "board_ip", r.ip)
			return r.ip, r.ssh
		case <-done:
		case <-time.After(15 * time.Second):
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

	// Stop the Tart lab VM if it is running so the same Cloudflare tunnel
	// token cannot race between the VM and the real device.
	if labVirtualMachineIPAddress := resolveLabVirtualMachineIPAddress(); labVirtualMachineIPAddress != "" {
		fmt.Printf("  %s\n", m.t("Tart Lab VM 중지 중 (터널 충돌 방지)...", "Stopping Tart lab VM (tunnel conflict)..."))
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
chroot /mnt/armbian sh -c 'export DEBIAN_FRONTEND=noninteractive; apt-get update -qq >/dev/null 2>&1; . /etc/os-release; runtimePackages="%s"; case "${VERSION_ID:-}" in 22.*) runtimePackages="%s" ;; 24.*|25.*|26.*) runtimePackages="%s" ;; esac; apt-get install -y -d -qq postgresql postgresql-contrib jq avahi-daemon git curl ca-certificates $runtimePackages >/dev/null 2>&1'
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
	cfService := "[Unit]\nDescription=Cloudflare Tunnel\nAfter=network-online.target time-sync.target\nWants=network-online.target time-sync.target\n\n[Service]\nType=simple\nExecStart=/bin/sh -c '/usr/local/bin/cloudflared tunnel run --protocol http2 --token \"$(cat /root/.internkim/secrets/tunnel-token)\"'\nRestart=always\nRestartSec=5\n\n[Install]\nWantedBy=multi-user.target\n"
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
	sshpassBin string
	user       string
	pass       string
	host       string
	port       string
}

// ensureInternkimGcpProject returns the GCP project id usable for `gws
// auth setup --project`. Deterministic per device: derives the id from
// the stored device_id so re-runs always land on the same project. Only
// falls back to a random suffix when device_id isn't assigned yet.
func ensureInternkimGcpProject() string {
	if out, err := exec.Command("gcloud", "config", "get-value", "project", "--quiet").Output(); err == nil {
		if projectID := strings.TrimSpace(string(out)); projectID != "" && projectID != "(unset)" {
			return projectID
		}
	}
	stateDir := internkimHomeDir()
	deviceID := strings.TrimSpace(loadState(stateDir, "device_id"))
	var projectID string
	if deviceID != "" {
		projectID = googleProjectID(deviceID)
	} else {
		randomBytes := make([]byte, 4)
		if _, err := rand.Read(randomBytes); err != nil {
			return ""
		}
		projectID = "internkim-" + hex.EncodeToString(randomBytes)
	}

	// Reuse the project if it already exists; otherwise create it.
	if out, err := exec.Command("gcloud", "projects", "describe", projectID, "--quiet", "--format=value(projectId)").Output(); err == nil && strings.TrimSpace(string(out)) == projectID {
		exec.Command("gcloud", "config", "set", "project", projectID, "--quiet").Run()
		return projectID
	}
	fmt.Printf("  새 GCP 프로젝트 생성: %s\n", projectID)
	create := exec.Command("gcloud", "projects", "create", projectID,
		"--name=internkim", "--quiet")
	create.Stdout = os.Stdout
	create.Stderr = os.Stderr
	if err := create.Run(); err != nil {
		return ""
	}
	exec.Command("gcloud", "config", "set", "project", projectID, "--quiet").Run()
	return projectID
}

// ensureGcloudOnPathSim prepends the Google Cloud SDK bin directory to
// PATH when gcloud isn't already discoverable. Mirrors setup.ensureGcloudOnPath
// for the sim / pre-setup code paths.
func ensureGcloudOnPathSim() {
	if _, err := exec.LookPath("gcloud"); err == nil {
		return
	}
	home, _ := os.UserHomeDir()
	for _, candidate := range []string{
		filepath.Join(home, "google-cloud-sdk", "bin"),
		"/opt/homebrew/share/google-cloud-sdk/bin",
		"/usr/local/share/google-cloud-sdk/bin",
	} {
		if info, err := os.Stat(filepath.Join(candidate, "gcloud")); err == nil && !info.IsDir() {
			os.Setenv("PATH", candidate+string(os.PathListSeparator)+os.Getenv("PATH"))
			return
		}
	}
}

// acquireGwsCredentialsJSON returns a portable gws credentials JSON,
// creating one via the chromedp-driven Cloud Console flow on first run
// and re-exporting the cached copy on subsequent runs.
func acquireGwsCredentialsJSON() ([]byte, error) {
	if _, err := exec.LookPath("gws"); err != nil {
		return nil, errors.New("gws CLI not on PATH — install with: brew install googleworkspace-cli")
	}
	ensureGcloudOnPathSim()

	exportOut, exportErr := exec.Command("gws", "auth", "export", "--unmasked").Output()
	if exportErr == nil && strings.Contains(string(exportOut), `"refresh_token"`) {
		return exportOut, nil
	}

	if _, err := exec.LookPath("gcloud"); err != nil {
		exec.Command("open", "https://cloud.google.com/sdk/docs/install").Start()
		return nil, errors.New("gcloud CLI required. Install: brew install --cask google-cloud-sdk, then re-run this step.")
	}
	if out, _ := exec.Command("gcloud", "auth", "list",
		"--filter=status:ACTIVE", "--format=value(account)").Output(); strings.TrimSpace(string(out)) == "" {
		fmt.Println("  gcloud 인증 필요 — 브라우저에서 Google 계정으로 로그인해주세요.")
		login := exec.Command("gcloud", "auth", "login")
		login.Stdin = os.Stdin
		login.Stdout = os.Stdout
		login.Stderr = os.Stderr
		if err := login.Run(); err != nil {
			return nil, fmt.Errorf("gcloud auth login failed: %w", err)
		}
	}
	projectID := strings.TrimSpace(ensureInternkimGcpProject())
	if projectID == "" {
		return nil, errors.New("could not determine or create a GCP project for Intern Kim")
	}
	enableArgs := append([]string{"services", "enable", "--project", projectID, "--quiet"},
		"drive.googleapis.com", "sheets.googleapis.com", "gmail.googleapis.com",
		"calendar-json.googleapis.com", "docs.googleapis.com", "slides.googleapis.com",
		"tasks.googleapis.com", "people.googleapis.com", "forms.googleapis.com",
		"keep.googleapis.com", "meet.googleapis.com", "chat.googleapis.com",
		"script.googleapis.com")
	exec.Command("gcloud", enableArgs...).Run()

	consoleURL := "https://console.cloud.google.com/apis/credentials?project=" + projectID + "&hl=en"
	if err := auth.LaunchChrome(consoleURL); err != nil {
		return nil, fmt.Errorf("launch chrome: %w", err)
	}
	fmt.Println("  Chrome 창이 열립니다. 처음이라면 Google 계정으로 로그인해주세요 (한 번만).")
	clientID, clientSecret, err := auth.EnsureUserOAuthClient(projectID)
	if err != nil {
		return nil, err
	}

	fmt.Println("  브라우저에서 Allow만 클릭해주세요.")
	login := exec.Command("gws", "auth", "login", "--full")
	login.Env = append(os.Environ(),
		"GOOGLE_WORKSPACE_CLI_CLIENT_ID="+clientID,
		"GOOGLE_WORKSPACE_CLI_CLIENT_SECRET="+clientSecret,
	)
	login.Stdin = os.Stdin
	login.Stdout = os.Stdout
	login.Stderr = os.Stderr
	if err := login.Run(); err != nil {
		return nil, fmt.Errorf("gws auth login failed: %w", err)
	}

	exportOut, exportErr = exec.Command("gws", "auth", "export", "--unmasked").Output()
	if exportErr != nil {
		return nil, fmt.Errorf("gws auth export failed: %w", exportErr)
	}
	if !strings.Contains(string(exportOut), `"refresh_token"`) {
		return nil, errors.New("gws auth export output missing refresh_token")
	}
	return exportOut, nil
}

// installGwsCredentials obtains Google Workspace credentials for the local
// maintainer and copies the portable JSON to the runtime user's
// ~/.config/gws/credentials.json on the board.
func installGwsCredentials(ssh *sshClient) error {
	exportOut, err := acquireGwsCredentialsJSON()
	if err != nil {
		return err
	}

	tmpPath := filepath.Join(os.TempDir(), "internkim-gws-credentials.json")
	if err := os.WriteFile(tmpPath, exportOut, 0o600); err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	ssh.run(`mkdir -p /home/blueclaw/.config/gws
chown -R blueclaw:blueclaw /home/blueclaw/.config`)
	ssh.scp(tmpPath, "/home/blueclaw/.config/gws/credentials.json")
	ssh.run(`chown blueclaw:blueclaw /home/blueclaw/.config/gws/credentials.json
chmod 600 /home/blueclaw/.config/gws/credentials.json`)
	return nil
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
	out, _ := s.runResult(cmd)
	return out
}

func (s *sshClient) runResult(cmd string) (string, error) {
	var args []string
	remoteCommand := s.privilegedCommand(cmd)
	if s.pass == "" {
		args = s.sshArgs(fmt.Sprintf("%s@%s", s.user, s.host), remoteCommand)
		return runSSHCommandWithRetry("ssh", args)
	}

	args = append([]string{"-p", s.pass, "ssh"}, s.sshArgs(fmt.Sprintf("%s@%s", s.user, s.host), remoteCommand)...)
	return runSSHCommandWithRetry(s.sshpassBin, args)
}

func runSSHCommandWithRetry(commandName string, arguments []string) (string, error) {
	var output []byte
	var errorValue error
	for attemptIndex := 0; attemptIndex < 3; attemptIndex++ {
		output, errorValue = exec.Command(commandName, arguments...).CombinedOutput()
		if errorValue == nil || !isRetryableSSHFailure(string(output)) {
			return string(output), errorValue
		}
		time.Sleep(time.Duration(attemptIndex+1) * time.Second)
	}
	return string(output), errorValue
}

func isRetryableSSHFailure(output string) bool {
	for _, phrase := range []string{
		"Connection refused",
		"Operation timed out",
		"Connection timed out",
		"Permission denied, please try again.",
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
	return append(base, extra...)
}

func (s *sshClient) scp(localPath, remotePath string) error {
	if s.user != "root" && strings.HasPrefix(remotePath, "/") {
		temporaryRemotePath := "/tmp/internkim-upload-" + filepath.Base(remotePath)
		if err := s.scpDirect(localPath, temporaryRemotePath); err != nil {
			return err
		}
		output, err := s.runResult(fmt.Sprintf(
			"mkdir -p %s && mv %s %s",
			quoteShellValue(filepath.Dir(remotePath)),
			quoteShellValue(temporaryRemotePath),
			quoteShellValue(remotePath),
		))
		if err != nil {
			return fmt.Errorf("move uploaded file to %s: %s: %w", remotePath, strings.TrimSpace(output), err)
		}
		return nil
	}
	return s.scpDirect(localPath, remotePath)
}

func (s *sshClient) scpDirect(localPath, remotePath string) error {
	target := fmt.Sprintf("%s@%s:%s", s.user, s.host, remotePath)
	if s.pass != "" {
		output, err := runSSHCommandWithRetry(s.sshpassBin, append([]string{"-p", s.pass, "scp"}, s.scpArgs(localPath, target)...))
		if err != nil {
			return fmt.Errorf("scp %s to %s failed: %s: %w", localPath, remotePath, strings.TrimSpace(output), err)
		}
		return nil
	}
	output, err := runSSHCommandWithRetry("scp", s.scpArgs(localPath, target))
	if err != nil {
		return fmt.Errorf("scp %s to %s failed: %s: %w", localPath, remotePath, strings.TrimSpace(output), err)
	}
	return nil
}

func (s *sshClient) scpDir(localDir, remoteDir string) error {
	if s.user != "root" && strings.HasPrefix(remoteDir, "/") {
		temporaryRemoteDirectory := "/tmp/internkim-upload-" + filepath.Base(remoteDir)
		output, err := s.runResult("rm -rf " + quoteShellValue(temporaryRemoteDirectory) + " && mkdir -p " + quoteShellValue(temporaryRemoteDirectory) + " && chmod 777 " + quoteShellValue(temporaryRemoteDirectory))
		if err != nil {
			return fmt.Errorf("prepare remote directory %s: %s: %w", temporaryRemoteDirectory, strings.TrimSpace(output), err)
		}
		if err := s.scpDirDirect(localDir, temporaryRemoteDirectory); err != nil {
			return err
		}
		output, err = s.runResult(fmt.Sprintf(
			"mkdir -p %s && cp -a %s/. %s/",
			quoteShellValue(remoteDir),
			quoteShellValue(temporaryRemoteDirectory),
			quoteShellValue(remoteDir),
		))
		if err != nil {
			return fmt.Errorf("move uploaded directory to %s: %s: %w", remoteDir, strings.TrimSpace(output), err)
		}
		return nil
	}
	return s.scpDirDirect(localDir, remoteDir)
}

func (s *sshClient) scpDirDirect(localDir, remoteDir string) error {
	target := fmt.Sprintf("%s@%s:%s", s.user, s.host, remoteDir)
	if s.pass != "" {
		output, err := runSSHCommandWithRetry(s.sshpassBin, append([]string{"-p", s.pass, "scp", "-r"}, s.scpArgs(localDir+"/.", target)...))
		if err != nil {
			return fmt.Errorf("scp directory %s to %s failed: %s: %w", localDir, remoteDir, strings.TrimSpace(output), err)
		}
		return nil
	}
	output, err := runSSHCommandWithRetry("scp", append([]string{"-r"}, s.scpArgs(localDir+"/.", target)...))
	if err != nil {
		return fmt.Errorf("scp directory %s to %s failed: %s: %w", localDir, remoteDir, strings.TrimSpace(output), err)
	}
	return nil
}

// --- Config builders ---

func generateFirstbootScript(deviceURL, adminEmail string) string {
	return buildFirstbootScript(deviceURL, adminEmail)
}

func googleResourceSuffix(deviceID string) string {
	normalizedDeviceID := strings.ToLower(strings.TrimSpace(deviceID))
	if len(normalizedDeviceID) > 20 {
		return normalizedDeviceID[:20]
	}
	return normalizedDeviceID
}

func googleProjectID(deviceID string) string {
	return "internkim-" + googleResourceSuffix(deviceID)
}

func googleServiceAccountName(deviceID string) string {
	return "internkim-" + googleResourceSuffix(deviceID)
}

func googleServiceAccountEmail(deviceID string) string {
	return googleServiceAccountName(deviceID) + "@" + googleProjectID(deviceID) + ".iam.gserviceaccount.com"
}

func createGoogleServiceAccount(deviceID, accessToken string) (string, error) {
	serviceAccountName := googleServiceAccountName(deviceID)
	fmt.Printf("  Service account name: %s\n", serviceAccountName)

	if accessToken == "" {
		googleAuthState, err := googleAuth()
		if err != nil {
			return "", fmt.Errorf("oauth failed: %w", err)
		}
		accessToken = googleAuthState.AccessToken
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	projectID, err := resolveGoogleProject(httpClient, accessToken, deviceID)
	if err != nil {
		return "", fmt.Errorf("project setup failed: %w", err)
	}

	serviceAccountEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", serviceAccountName, projectID)

	createBody, _ := json.Marshal(map[string]any{
		"accountId": serviceAccountName,
		"serviceAccount": map[string]string{
			"displayName": "Intern Kim " + deviceID,
		},
	})
	createRequest, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("https://iam.googleapis.com/v1/projects/%s/serviceAccounts", projectID),
		bytes.NewReader(createBody),
	)
	createRequest.Header.Set("Authorization", "Bearer "+accessToken)
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse, err := httpClient.Do(createRequest)
	if err != nil {
		return "", fmt.Errorf("create SA request failed: %w", err)
	}
	defer createResponse.Body.Close()
	if createResponse.StatusCode != 200 && createResponse.StatusCode != 409 {
		body, _ := io.ReadAll(createResponse.Body)
		return "", fmt.Errorf("create SA HTTP %d: %s", createResponse.StatusCode, string(body))
	}

	if err := enableGoogleAPIs(httpClient, accessToken, projectID); err != nil {
		fmt.Printf("  API 활성화 실패 (무시하고 계속): %v\n", err)
	}

	serviceAccountKey, err := createSAKey(httpClient, accessToken, projectID, serviceAccountEmail)
	if err != nil {
		fmt.Printf("  Key creation blocked — deleting existing keys and retrying...\n")
		deleteExistingSAKeys(httpClient, accessToken, projectID, serviceAccountEmail)
		time.Sleep(3 * time.Second)
		serviceAccountKey, err = createSAKey(httpClient, accessToken, projectID, serviceAccountEmail)
	}
	if err != nil {
		return "", err
	}
	return serviceAccountKey, nil
}

func enableGoogleAPIs(httpClient *http.Client, accessToken, projectID string) error {
	serviceIDs := []string{
		"drive.googleapis.com",
		"docs.googleapis.com",
		"sheets.googleapis.com",
		"slides.googleapis.com",
		"gmail.googleapis.com",
		"calendar-json.googleapis.com",
		"tasks.googleapis.com",
		"people.googleapis.com",
		"chat.googleapis.com",
		"forms.googleapis.com",
		"script.googleapis.com",
		"classroom.googleapis.com",
		"meet.googleapis.com",
		"keep.googleapis.com",
	}
	requestBody, _ := json.Marshal(map[string]any{
		"serviceIds": serviceIDs,
	})
	request, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("https://serviceusage.googleapis.com/v1/projects/%s/services:batchEnable", projectID),
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("API enable request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("API enable HTTP %d: %s", response.StatusCode, string(body))
	}
	fmt.Printf("  Google Workspace API 활성화 완료 (%d개)\n", len(serviceIDs))
	return nil
}

func createSAKey(httpClient *http.Client, accessToken, projectID, serviceAccountEmail string) (string, error) {
	request, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("https://iam.googleapis.com/v1/projects/%s/serviceAccounts/%s/keys", projectID, serviceAccountEmail),
		strings.NewReader(`{"keyAlgorithm":"KEY_ALG_RSA_2048","privateKeyType":"TYPE_GOOGLE_CREDENTIALS_FILE"}`),
	)
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("create key request failed: %w", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != 200 {
		return "", fmt.Errorf("create key HTTP %d: %s", response.StatusCode, string(body))
	}

	var parsed struct {
		PrivateKeyData string `json:"privateKeyData"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse key response: %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(parsed.PrivateKeyData)
	if err != nil {
		return "", fmt.Errorf("decode key: %w", err)
	}
	return string(decoded), nil
}

func deleteExistingSAKeys(httpClient *http.Client, accessToken, projectID, serviceAccountEmail string) {
	request, _ := http.NewRequest(
		"GET",
		fmt.Sprintf("https://iam.googleapis.com/v1/projects/%s/serviceAccounts/%s/keys?keyTypes=USER_MANAGED", projectID, serviceAccountEmail),
		nil,
	)
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := httpClient.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()

	var parsed struct {
		Keys []struct {
			Name string `json:"name"`
		} `json:"keys"`
	}
	body, _ := io.ReadAll(response.Body)
	if json.Unmarshal(body, &parsed) != nil {
		return
	}

	for _, key := range parsed.Keys {
		deleteRequest, _ := http.NewRequest("DELETE", "https://iam.googleapis.com/v1/"+key.Name, nil)
		deleteRequest.Header.Set("Authorization", "Bearer "+accessToken)
		deleteResponse, err := httpClient.Do(deleteRequest)
		if err == nil {
			deleteResponse.Body.Close()
			fmt.Printf("  Deleted existing key: %s\n", key.Name)
		}
	}
}

func resolveGoogleProject(httpClient *http.Client, accessToken, deviceID string) (string, error) {
	projectID := googleProjectID(deviceID)

	request, _ := http.NewRequest("GET", "https://cloudresourcemanager.googleapis.com/v1/projects/"+projectID, nil)
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := httpClient.Do(request)
	if err != nil {
		return "", err
	}
	response.Body.Close()

	if response.StatusCode == 200 {
		fmt.Printf("  Google Cloud project: %s (existing)\n", projectID)
		return projectID, nil
	}

	fmt.Printf("  Creating Google Cloud project: %s...\n", projectID)
	requestBody, _ := json.Marshal(map[string]string{
		"projectId": projectID,
		"name":      "Intern Kim",
	})
	createRequest, _ := http.NewRequest("POST", "https://cloudresourcemanager.googleapis.com/v1/projects", bytes.NewReader(requestBody))
	createRequest.Header.Set("Authorization", "Bearer "+accessToken)
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse, err := httpClient.Do(createRequest)
	if err != nil {
		return "", err
	}
	defer createResponse.Body.Close()
	if createResponse.StatusCode == 409 {
		fmt.Printf("  Google Cloud project: %s (existing)\n", projectID)
		return projectID, nil
	}
	if createResponse.StatusCode != 200 {
		body, _ := io.ReadAll(createResponse.Body)
		return "", fmt.Errorf("create project HTTP %d: %s", createResponse.StatusCode, string(body))
	}

	for attempt := 0; attempt < 10; attempt++ {
		time.Sleep(3 * time.Second)
		checkRequest, _ := http.NewRequest("GET", "https://cloudresourcemanager.googleapis.com/v1/projects/"+projectID, nil)
		checkRequest.Header.Set("Authorization", "Bearer "+accessToken)
		checkResponse, err := httpClient.Do(checkRequest)
		if err == nil && checkResponse.StatusCode == 200 {
			checkResponse.Body.Close()
			fmt.Printf("  Google Cloud project created: %s\n", projectID)
			return projectID, nil
		}
		if checkResponse != nil {
			checkResponse.Body.Close()
		}
		fmt.Print(".")
	}
	return "", fmt.Errorf("project creation timed out")
}

var googleAuthScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
}

func googleAuth() (*setup.GoogleAuth, error) {
	clientID := "764086051850-6qr4p6gpi6hn506pt8ejuq83di341hur.apps.googleusercontent.com"
	clientSecret := "d-FL95Q19q7MQmFpd7hHD0Ty"

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("open local port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d", port)

	authURL := "https://accounts.google.com/o/oauth2/auth" +
		"?client_id=" + clientID +
		"&redirect_uri=" + url.QueryEscape(redirectURI) +
		"&response_type=code" +
		"&scope=" + url.QueryEscape(strings.Join(googleAuthScopes, " ")) +
		"&access_type=offline"

	fmt.Println()
	fmt.Println("  Opening Chrome for Google login...")
	if err := auth.OpenChromeURL(authURL); err != nil {
		return nil, fmt.Errorf("open chrome: %w", err)
	}
	fmt.Printf("  If the window did not open, visit:\n  %s\n\n", authURL)

	codeChannel := make(chan string, 1)
	callbackMux := http.NewServeMux()
	callbackServer := &http.Server{Handler: callbackMux}
	callbackMux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		code := request.URL.Query().Get("code")
		if code == "" {
			return
		}
		writer.Header().Set("Location", "https://script.google.com/home?hl=en")
		writer.WriteHeader(http.StatusFound)
		codeChannel <- code
	})
	go callbackServer.Serve(listener)
	defer callbackServer.Close()

	var authorizationCode string
	select {
	case authorizationCode = <-codeChannel:
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("timed out waiting for Google authorization")
	}

	tokenResponse, err := http.PostForm("https://oauth2.googleapis.com/token", map[string][]string{
		"code":          {authorizationCode},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	defer tokenResponse.Body.Close()

	body, _ := io.ReadAll(tokenResponse.Body)
	var parsed struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if parsed.Error != "" {
		return nil, fmt.Errorf("token endpoint: %s", parsed.Error)
	}

	return &setup.GoogleAuth{
		AccessToken: parsed.AccessToken,
		Email:       fetchGoogleEmail(parsed.AccessToken),
	}, nil
}

func fetchGoogleEmail(accessToken string) string {
	request, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()

	var userInfo struct {
		Email string `json:"email"`
	}
	body, _ := io.ReadAll(response.Body)
	if json.Unmarshal(body, &userInfo) != nil {
		return ""
	}
	return userInfo.Email
}

const gasBridgeManifest = `{
  "timeZone": "Etc/UTC",
  "dependencies": {},
  "exceptionLogging": "STACKDRIVER",
  "runtimeVersion": "V8",
  "oauthScopes": [
    "https://www.googleapis.com/auth/drive.file",
    "https://www.googleapis.com/auth/presentations",
    "https://www.googleapis.com/auth/documents",
    "https://www.googleapis.com/auth/spreadsheets",
    "https://www.googleapis.com/auth/calendar",
    "https://www.googleapis.com/auth/gmail.send",
    "https://www.googleapis.com/auth/script.external_request"
  ],
  "webapp": {
    "executeAs": "USER_DEPLOYING",
    "access": "ANYONE_ANONYMOUS"
  }
}`

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

	scriptDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	gasBridgeCode, err := blueclawworkspace.ReadGasBridgeCode(scriptDir)
	if err != nil {
		return "", err
	}
	webhookURL, err := auth.DeployAppsScriptViaBrowser(gasBridgeCode, gasBridgeManifest)
	if err != nil {
		return "", fmt.Errorf("browser deploy: %w", err)
	}
	if err := os.MkdirAll(internkimHomeDir(), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(gasWebhookURLPath(), []byte(webhookURL+"\n"), 0o600); err != nil {
		return "", err
	}
	fmt.Printf("  Webhook URL saved to %s\n", gasWebhookURLPath())
	return webhookURL, nil
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
		requestedPassword = jetsonDefaultPassword
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

func runSetupLive(messenger *msg) {
	configuration := loadConfig()
	baseStateDir := internkimHomeDir()
	stateDir := baseStateDir
	scriptDir, _ := os.Getwd()
	sshpassBin := filepath.Join(scriptDir, "bin", "sshpass")
	setupBuildID := currentExecutableFingerprint()

	requestedSSH := containsArg("--ssh")
	requestedSD := containsArg("--sd")
	if requestedSSH && requestedSD {
		fatal("--ssh and --sd are mutually exclusive")
	}
	if containsArg("--sim") {
		runSetupSimulation(setupControlArguments(os.Args[2:]))
		return
	}
	hostOverride := argString("--host", "")
	boardType := argString("--board", setup.BoardJetsonOrinNano)
	stateDir = setupStateDir(baseStateDir, boardType)
	sshUser, sshPassword := resolveSetupSSHCredentials(
		boardType,
		argString("--user", ""),
		argString("--password", ""),
	)
	nonInteractive := containsArg("--non-interactive")
	if boardType == setup.BoardJetsonOrinNano {
		requestedSSH = true
	}

	var (
		selectedBackend setup.Backend
		sshConnection   *sshClient
		stagingRoot     string
		boardIP         string
	)

	// Board detection over Wi-Fi has a short TCP dial timeout (3s per
	// saved IP) and occasionally loses the first round — the ARP cache may
	// be cold or the router can hold a half-open path after the board goes
	// idle. Retry detection once after a short pause before falling back
	// to the next backend.
	attemptBackend := func(kind setup.Backend) bool {
		for attempt := 0; attempt < 3; attempt++ {
			switch kind {
			case setup.BackendSSH:
				if hostOverride != "" {
					boardIP = hostOverride
				} else {
					boardIP = findBoardIPForCredentials(sshpassBin, stateDir, sshUser, sshPassword)
				}
				if boardIP != "" {
					sshConnection = newSSH(sshpassBin, sshUser, sshPassword, boardIP)
					return true
				}
			case setup.BackendSD:
				stagingRoot = findSDStagingRoot()
				if stagingRoot != "" {
					return true
				}
			}
			if attempt < 2 {
				time.Sleep(2 * time.Second)
			}
		}
		return false
	}

	switch {
	case requestedSSH:
		if !attemptBackend(setup.BackendSSH) {
			if boardType == setup.BoardJetsonOrinNano {
				failureDetails := describeJetsonSSHFailure(sshpassBin, stateDir, sshUser, sshPassword)
				fatal(messenger.t(
					"Jetson을 SSH로 찾을 수 없습니다.\n"+failureDetails+"\nJetson 콘솔에서 `ip addr`, `nmcli device status`, `systemctl status ssh --no-pager`, `tail /var/log/internkim-jetson-firstboot.log`를 확인하세요. IP를 알면 --host <ip>를 지정하면 됩니다.",
					"Jetson was not found over SSH.\n"+failureDetails+"\nOn the Jetson console, check `ip addr`, `nmcli device status`, `systemctl status ssh --no-pager`, and `tail /var/log/internkim-jetson-firstboot.log`. If you know the IP, pass --host <ip>.",
				))
			}
			fatal(messenger.t("보드를 찾을 수 없습니다 (SSH).", "Board not reachable (SSH)."))
		}
		selectedBackend = setup.BackendSSH
	case requestedSD:
		if !attemptBackend(setup.BackendSD) {
			fatal(messenger.t("SD 카드가 꽂혀있지 않거나 internkim 디렉토리가 없습니다.", "No SD card mounted with an internkim staging dir."))
		}
		selectedBackend = setup.BackendSD
	default:
		if attemptBackend(setup.BackendSSH) {
			selectedBackend = setup.BackendSSH
		} else if attemptBackend(setup.BackendSD) {
			selectedBackend = setup.BackendSD
		} else {
			fatal(messenger.t(
				"타겟을 찾을 수 없습니다 — 보드에 SSH도 안 되고, SD 카드도 없습니다.\n  --ssh 또는 --sd 를 명시하거나, 대상을 준비해 주세요.",
				"No target — board unreachable via SSH and no SD mounted.\n  Pass --ssh or --sd explicitly, or prepare a target.",
			))
		}
	}

	switch selectedBackend {
	case setup.BackendSSH:
		fmt.Printf("Target: %s (ssh)\n", boardIP)
	case setup.BackendSD:
		fmt.Printf("Target: %s (sd staging)\n", stagingRoot)
	}

	flowState := newSetupFlowState(
		messenger,
		configuration,
		collectSetupParameterValues(),
		stateDir,
		scriptDir,
		setupBuildID,
		sshConnection,
		nonInteractive,
	)

	shouldForce := containsArg("--force") || containsArg("--force-all")
	if selectedBackend == setup.BackendSD && setupBuildID != "" {
		stageBuildIDPath := filepath.Join(stagingRoot, "setup-build-id")
		if stageBuildIDBytes, err := os.ReadFile(stageBuildIDPath); err == nil {
			if strings.TrimSpace(string(stageBuildIDBytes)) != setupBuildID {
				shouldForce = true
			}
		}
	}

	pipelineContext := &setup.Context{
		Backend:   selectedBackend,
		Language:  messenger.lang,
		StateDir:  stateDir,
		ScriptDir: scriptDir,
		BoardType: boardType,
		BoardIP:   boardIP,
		Force:     shouldForce,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		Callbacks: flowState.callbacks(),
	}
	if selectedBackend == setup.BackendSSH {
		pipelineContext.SSH = sshBoardConnection{client: sshConnection}
	} else {
		pipelineContext.SD = sdStagingTarget{stagingRoot: stagingRoot}
	}

	selector := setup.Selector{
		Only:     setup.ParseNames(argString("--only", "")),
		From:     argString("--from", ""),
		Skip:     setup.ParseNames(argString("--skip", "")),
		Force:    pipelineContext.Force,
		ForceAll: containsArg("--force-all"),
		DryRun:   containsArg("--plan"),
	}
	selector = applySetupBoardDefaults(boardType, containsArg("--with-google"), selector)

	registry := setup.DefaultRegistry()
	if boardType == setup.BoardJetsonOrinNano {
		registry = setup.JetsonRegistry()
	}
	if err := registry.Run(pipelineContext, selector); err != nil {
		fatal(err.Error())
	}
}

func applySetupBoardDefaults(boardType string, withGoogle bool, selector setup.Selector) setup.Selector {
	if boardType == setup.BoardJetsonOrinNano && !withGoogle && !containsName(selector.Only, "google") {
		selector.Skip = appendMissingName(selector.Skip, "google")
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
		return strings.TrimSpace(os.Getenv("INTERNKIM_LITERT_MODEL_PATH")), nil
	}
}
