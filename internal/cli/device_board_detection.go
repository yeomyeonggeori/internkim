package cli

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

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

	// 2. Subnet SSH scan across every local IPv4 subnet
	subnets := localScanSubnets(stateDir)
	if len(subnets) > 0 {
		type result struct {
			ip  string
			ssh bool
		}
		found := make(chan result, 256*len(subnets))
		var wg sync.WaitGroup
		alreadyTried := make(map[string]bool, len(candidates))
		for _, candidate := range candidates {
			alreadyTried[candidate] = true
		}
		for _, subnet := range subnets {
			for i := 2; i <= 254; i++ {
				ip := fmt.Sprintf("%s.%d", subnet, i)
				if alreadyTried[ip] {
					continue
				}
				alreadyTried[ip] = true
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
		}
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		select {
		case r := <-found:
			saveState(stateDir, "board_ip", r.ip)
			return r.ip, r.ssh
		case <-done:
		case <-time.After(20 * time.Second):
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
