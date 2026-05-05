package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	setup "gitlab.com/eastriver/internkim/internal/provisioning/steps"
)

const jetsonUSBHostAddress = "192.168.55.1"

func runWiFi() {
	if containsArg("--help") || containsArg("-h") {
		printWiFiUsage()
		return
	}
	if errorValue := runWiFiAdd(); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func printWiFiUsage() {
	fmt.Println("Usage: internkim wifi [add] [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --wifi-ssid <ssid>       Wi-Fi SSID to add. Defaults to the current Mac Wi-Fi")
	fmt.Println("  --wifi-password <value>  Wi-Fi password. Defaults to macOS Keychain when available")
	fmt.Println("  --wifi-open              Add an open Wi-Fi network")
	fmt.Println("  --wifi-hidden            Mark the Wi-Fi network as hidden")
	fmt.Println("  --no-connect             Install the profile without asking Jetson to connect now")
	fmt.Println("  --wait                   Wait for a wireless IP after applying")
	fmt.Println("  --host <ip>              Override Jetson SSH host, for example 192.168.55.1 over USB")
	fmt.Println("  --user <name>            Override Jetson SSH user")
	fmt.Println("  --password <value>       Override Jetson SSH password")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  internkim wifi add --wifi-ssid OfficeWiFi --wifi-password secret")
	fmt.Println("  internkim wifi add --host 192.168.55.1")
}

func runWiFiAdd() error {
	subcommand := ""
	if len(os.Args) > 2 && !strings.HasPrefix(os.Args[2], "-") {
		subcommand = os.Args[2]
	}
	if subcommand != "" && subcommand != "add" {
		return errors.New("usage: internkim wifi [add] [options]")
	}

	messenger := newMsg(wiFiCommandLanguage())
	boardType := argString("--board", setup.BoardJetsonOrinNano)
	if boardType != setup.BoardJetsonOrinNano {
		return errors.New("wifi command currently supports --board jetson-orin-nano")
	}

	target, errorValue := resolveJetsonWiFiCommandTarget(boardType)
	if errorValue != nil {
		return errorValue
	}

	profiles, errorValue := resolveWiFiProfiles(messenger, target.stateDirectory, target.getSSIDPath)
	if errorValue != nil {
		return errorValue
	}
	if len(profiles) == 0 {
		return errors.New("wifi profile is empty")
	}

	shouldConnect := !containsArg("--no-connect")
	shouldWait := shouldConnect && (containsArg("--wait") || target.hostAddress == jetsonUSBHostAddress)
	sshConnection := newSSH(target.sshpassPath, target.username, target.password, target.hostAddress)
	fmt.Printf("Target: %s (ssh)\n", target.hostAddress)

	output, errorValue := sshConnection.runResult(buildJetsonWiFiApplyCommand(profiles, shouldConnect, shouldWait))
	if errorValue != nil {
		return fmt.Errorf("Jetson Wi-Fi update failed: %s", strings.TrimSpace(output))
	}

	wirelessAddress := lastNonEmptyLine(output)
	if shouldWait && wirelessAddress != "" {
		saveState(target.stateDirectory, "board_wifi_ip", wirelessAddress)
		fmt.Printf("  %s: %s\n", messenger.t("Wi-Fi 연결 성공", "Wi-Fi connected"), wirelessAddress)
		return nil
	}
	if shouldConnect {
		fmt.Println(messenger.t("  Wi-Fi 프로필 설치 완료. Jetson이 백그라운드에서 연결을 시도합니다.", "  Wi-Fi profile installed. Jetson is connecting in the background."))
		return nil
	}
	fmt.Println(messenger.t("  Wi-Fi 프로필 설치 완료", "  Wi-Fi profile installed"))
	return nil
}

type jetsonWiFiCommandTarget struct {
	hostAddress    string
	username       string
	password       string
	sshpassPath    string
	stateDirectory string
	getSSIDPath    string
}

func resolveJetsonWiFiCommandTarget(boardType string) (jetsonWiFiCommandTarget, error) {
	baseStateDirectory := internkimHomeDir()
	stateDirectory := setupStateDir(baseStateDirectory, boardType)
	scriptDirectory, _ := os.Getwd()
	sshpassPath := filepath.Join(scriptDirectory, "bin", "sshpass")
	username, password := resolveSetupSSHCredentials(boardType, argString("--user", ""), argString("--password", ""))

	hostAddress := strings.TrimSpace(argString("--host", ""))
	if hostAddress == "" {
		hostAddress = findJetsonWiFiCommandHost(sshpassPath, stateDirectory, username, password)
	}
	if hostAddress == "" {
		return jetsonWiFiCommandTarget{}, errors.New("Jetson을 SSH로 찾을 수 없습니다. USB-C로 연결한 뒤 잠시 기다리거나 --host 192.168.55.1 를 지정하세요.")
	}

	return jetsonWiFiCommandTarget{
		hostAddress:    hostAddress,
		username:       username,
		password:       password,
		sshpassPath:    sshpassPath,
		stateDirectory: stateDirectory,
		getSSIDPath:    filepath.Join(scriptDirectory, "bin", "get-ssid"),
	}, nil
}

func findJetsonWiFiCommandHost(sshpassPath string, stateDirectory string, username string, password string) string {
	for _, hostAddress := range jetsonWiFiCommandHostCandidates(stateDirectory) {
		if sshCheckHostnameForCredentials(sshpassPath, hostAddress, username, password) {
			return hostAddress
		}
	}
	return findBoardIPForCredentials(sshpassPath, stateDirectory, username, password)
}

func jetsonWiFiCommandHostCandidates(stateDirectory string) []string {
	return uniqueNonEmptyStrings([]string{
		jetsonUSBHostAddress,
		loadState(stateDirectory, "board_ip"),
		loadState(stateDirectory, "board_wifi_ip"),
	})
}

func buildJetsonWiFiApplyCommand(profiles []resolvedWiFiProfile, shouldConnect bool, shouldWait bool) string {
	connectCommand := "true"
	if shouldConnect && shouldWait {
		connectCommand = "/usr/local/bin/internkim-wifi-select >/tmp/internkim-wifi-select.log 2>&1 || true"
	} else if shouldConnect {
		connectCommand = "nohup /usr/local/bin/internkim-wifi-select >/tmp/internkim-wifi-select.log 2>&1 &"
	}
	waitCommand := "echo installed"
	if shouldWait {
		waitCommand = jetsonWiFiWaitCommand()
	}
	return strings.Join([]string{
		"set -eu",
		buildJetsonWiFiInstallScript(profiles),
		connectCommand,
		waitCommand,
	}, "\n")
}

func jetsonWiFiWaitCommand() string {
	return strings.TrimSpace(`for attempt in $(seq 1 12); do
  address="$(ip -o -4 addr show scope global 2>/dev/null | awk '$2 ~ /^(wl|wlan)/ {print $4; exit}' | cut -d/ -f1)"
  if [ -n "$address" ]; then
    echo "$address"
    exit 0
  fi
  sleep 5
done
tail -20 /tmp/internkim-wifi-select.log 2>/dev/null || true
exit 1`)
}

func wiFiCommandLanguage() string {
	if containsArg("--en") {
		return "en"
	}
	return "ko"
}

func lastNonEmptyLine(value string) string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	for lineIndex := len(lines) - 1; lineIndex >= 0; lineIndex-- {
		line := strings.TrimSpace(lines[lineIndex])
		if line != "" {
			return line
		}
	}
	return ""
}
