package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

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
		{blueclaw.BlueclawServiceName, "Blueclaw"},
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

	fmt.Println()
	fmt.Printf("  %-20s %s\n", m.t("적용된 릴리스", "Applied release"), appliedReleaseID(sshCmd))

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

const deviceCurrentReleaseManifestPath = "/root/.internkim/state/admin/release-updates/current.json"

func appliedReleaseID(sshCmd func(string) string) string {
	document := sshCmd("cat " + deviceCurrentReleaseManifestPath + " 2>/dev/null")
	if strings.TrimSpace(document) == "" {
		return "none recorded"
	}
	var manifest struct {
		ReleaseID string `json:"releaseID"`
	}
	if errorValue := json.Unmarshal([]byte(document), &manifest); errorValue != nil {
		return "unreadable"
	}
	if strings.TrimSpace(manifest.ReleaseID) == "" {
		return "none recorded"
	}
	return manifest.ReleaseID
}
