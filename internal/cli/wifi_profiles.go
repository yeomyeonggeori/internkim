package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const wifiProfilesStateFile = "wifi_profiles.json"
const jetsonWiFiConnectionPrefix = "internkim-wifi-"
const jetsonWiFiConnectionDirectory = "/etc/NetworkManager/system-connections"
const jetsonWiFiSelectorScriptPath = "/usr/local/bin/internkim-wifi-select"
const jetsonWiFiRecoveryServicePath = "/etc/systemd/system/internkim-wifi-recovery.service"
const jetsonWiFiRecoveryTimerPath = "/etc/systemd/system/internkim-wifi-recovery.timer"
const jetsonNetworkSnapshotScriptPath = "/usr/local/lib/internkim/network-snapshot.sh"
const jetsonNetworkSnapshotServicePath = "/etc/systemd/system/internkim-network-snapshot.service"
const jetsonNetworkSnapshotTimerPath = "/etc/systemd/system/internkim-network-snapshot.timer"
const jetsonTunnelRecoveryScriptPath = "/usr/local/lib/internkim/tunnel-recovery.sh"
const jetsonTunnelRecoveryServicePath = "/etc/systemd/system/internkim-tunnel-recovery.service"
const jetsonTunnelRecoveryTimerPath = "/etc/systemd/system/internkim-tunnel-recovery.timer"
const jetsonPersistentJournalConfigurationPath = "/etc/systemd/journald.conf.d/internkim-persistent.conf"
const jetsonEthernetConnectionID = "internkim-ethernet"

type wifiProfile struct {
	SSID       string `json:"ssid"`
	IsOpen     bool   `json:"isOpen"`
	IsHidden   bool   `json:"isHidden,omitempty"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
	Source     string `json:"source,omitempty"`
}

type resolvedWiFiProfile struct {
	SSID     string
	Password string
	IsOpen   bool
	IsHidden bool
}

func resolveWiFiProfiles(messenger *msg, stateDirectory string) ([]resolvedWiFiProfile, error) {
	profiles := loadWiFiProfiles(stateDirectory)
	selectedSSID := strings.TrimSpace(argString("--wifi-ssid", ""))
	explicitPassword := argString("--wifi-password", "")
	isOpenWiFi := containsArg("--wifi-open") || containsArg("--open-wifi")
	isHiddenWiFi := containsArg("--wifi-hidden") || containsArg("--hidden-wifi")

	if selectedSSID == "" && len(profiles) == 0 {
		if containsArg("--no-wifi") {
			return nil, nil
		}
		if isNonInteractiveWiFiResolution() {
			return nil, errors.New(messenger.t("저장된 Wi-Fi SSID가 없습니다. --wifi-ssid 를 지정하세요.", "No Wi-Fi SSID is saved. Pass --wifi-ssid."))
		}
		selectedSSID = strings.TrimSpace(readLine(messenger.t("  Wi-Fi SSID 입력: ", "  Enter Wi-Fi SSID: ")))
	}
	if selectedSSID != "" {
		if !isOpenWiFi {
			isOpenWiFi = isSavedOpenWiFi(profiles, selectedSSID)
		}
		if !isHiddenWiFi {
			isHiddenWiFi = isSavedHiddenWiFi(profiles, selectedSSID)
		}
		profiles = upsertWiFiProfile(profiles, wifiProfile{
			SSID:       selectedSSID,
			IsOpen:     isOpenWiFi,
			IsHidden:   isHiddenWiFi,
			LastUsedAt: time.Now().UTC().Format(time.RFC3339),
			Source:     "manual",
		})
		saveWiFiProfiles(stateDirectory, profiles)
		printSelectedWiFi(selectedSSID, len(profiles))
	}
	return resolveWiFiProfilePasswords(messenger, stateDirectory, profiles, selectedSSID, explicitPassword)
}

func resolveWiFiProfilePasswords(messenger *msg, stateDirectory string, profiles []wifiProfile, selectedSSID string, explicitPassword string) ([]resolvedWiFiProfile, error) {
	resolvedProfiles := make([]resolvedWiFiProfile, 0, len(profiles))
	for _, profile := range profiles {
		password := ""
		if !profile.IsOpen {
			password = resolveWiFiPassword(profile.SSID, selectedSSID, explicitPassword)
		}
		if !profile.IsOpen && password == "" {
			if strings.EqualFold(profile.SSID, selectedSSID) {
				if isNonInteractiveWiFiResolution() {
					return nil, fmt.Errorf(messenger.t("Wi-Fi 비밀번호를 자동으로 찾지 못했습니다. macOS 키체인에 %s 비밀번호를 저장하거나 --wifi-password 를 지정하세요.", "Wi-Fi password was not found automatically. Save the %s password in macOS Keychain or pass --wifi-password."), profile.SSID)
				}
				password = readSecret(messenger.t("  Wi-Fi 비밀번호 입력: ", "  Enter Wi-Fi password: "))
			}
			if password == "" {
				continue
			}
		}
		resolvedProfiles = append(resolvedProfiles, resolvedWiFiProfile{SSID: profile.SSID, Password: password, IsOpen: profile.IsOpen, IsHidden: profile.IsHidden})
	}
	if selectedSSID != "" && !containsResolvedWiFiProfile(resolvedProfiles, selectedSSID) {
		return nil, fmt.Errorf("wifi password is empty for %s", selectedSSID)
	}
	return resolvedProfiles, nil
}

func isNonInteractiveWiFiResolution() bool {
	return containsArg("--yes") || containsArg("--non-interactive")
}

func resolveWiFiPassword(ssid string, selectedSSID string, explicitPassword string) string {
	if explicitPassword != "" && strings.EqualFold(ssid, selectedSSID) {
		return explicitPassword
	}
	if keychainPassword := getKeychainPassword(ssid); keychainPassword != "" {
		return keychainPassword
	}
	return ""
}

func printSelectedWiFi(selectedSSID string, profileCount int) {
	fmt.Printf("  SSID: %s\n", selectedSSID)
	if profileCount > 1 {
		fmt.Printf("  Wi-Fi profiles: %d\n", profileCount)
	}
}

func loadWiFiProfiles(stateDirectory string) []wifiProfile {
	return sortWiFiProfiles(readWiFiProfilesFile(stateDirectory))
}

func readWiFiProfilesFile(stateDirectory string) []wifiProfile {
	document, errorValue := os.ReadFile(filepath.Join(stateDirectory, wifiProfilesStateFile))
	if errorValue != nil {
		return nil
	}
	var profiles []wifiProfile
	if errorValue := json.Unmarshal(document, &profiles); errorValue != nil {
		return nil
	}
	normalizedProfiles := make([]wifiProfile, 0, len(profiles))
	for _, profile := range profiles {
		if normalizedProfile, ok := normalizeWiFiProfile(profile); ok {
			normalizedProfiles = upsertWiFiProfile(normalizedProfiles, normalizedProfile)
		}
	}
	return normalizedProfiles
}

func saveWiFiProfiles(stateDirectory string, profiles []wifiProfile) {
	_ = os.MkdirAll(stateDirectory, 0o700)
	document, errorValue := json.MarshalIndent(sortWiFiProfiles(profiles), "", "  ")
	if errorValue != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(stateDirectory, wifiProfilesStateFile), append(document, '\n'), 0o600)
}

func upsertWiFiProfile(profiles []wifiProfile, profile wifiProfile) []wifiProfile {
	normalizedProfile, ok := normalizeWiFiProfile(profile)
	if !ok {
		return profiles
	}
	nextProfiles := make([]wifiProfile, 0, len(profiles)+1)
	isInserted := false
	for _, existingProfile := range profiles {
		existingProfile, ok := normalizeWiFiProfile(existingProfile)
		if !ok {
			continue
		}
		if strings.EqualFold(existingProfile.SSID, normalizedProfile.SSID) {
			nextProfiles = append(nextProfiles, mergeWiFiProfile(existingProfile, normalizedProfile))
			isInserted = true
			continue
		}
		nextProfiles = append(nextProfiles, existingProfile)
	}
	if !isInserted {
		nextProfiles = append(nextProfiles, normalizedProfile)
	}
	return sortWiFiProfiles(nextProfiles)
}

func mergeWiFiProfile(existingProfile wifiProfile, profile wifiProfile) wifiProfile {
	if profile.LastUsedAt == "" {
		profile.LastUsedAt = existingProfile.LastUsedAt
	}
	if profile.Source == "" {
		profile.Source = existingProfile.Source
	}
	return profile
}

func normalizeWiFiProfile(profile wifiProfile) (wifiProfile, bool) {
	profile.SSID = strings.TrimSpace(profile.SSID)
	profile.Source = strings.TrimSpace(profile.Source)
	profile.LastUsedAt = strings.TrimSpace(profile.LastUsedAt)
	return profile, profile.SSID != ""
}

func sortWiFiProfiles(profiles []wifiProfile) []wifiProfile {
	sortedProfiles := append([]wifiProfile{}, profiles...)
	sort.SliceStable(sortedProfiles, func(leftIndex int, rightIndex int) bool {
		left := sortedProfiles[leftIndex]
		right := sortedProfiles[rightIndex]
		if left.IsOpen != right.IsOpen {
			return !left.IsOpen
		}
		return strings.ToLower(left.SSID) < strings.ToLower(right.SSID)
	})
	return sortedProfiles
}

func isSavedOpenWiFi(profiles []wifiProfile, ssid string) bool {
	for _, profile := range profiles {
		if strings.EqualFold(profile.SSID, ssid) {
			return profile.IsOpen
		}
	}
	return false
}

func isSavedHiddenWiFi(profiles []wifiProfile, ssid string) bool {
	for _, profile := range profiles {
		if strings.EqualFold(profile.SSID, ssid) {
			return profile.IsHidden
		}
	}
	return false
}

func containsResolvedWiFiProfile(profiles []resolvedWiFiProfile, ssid string) bool {
	for _, profile := range profiles {
		if strings.EqualFold(profile.SSID, ssid) {
			return true
		}
	}
	return false
}

func jetsonWiFiConnectionID(ssid string) string {
	normalizedSSID := strings.ToLower(strings.TrimSpace(ssid))
	hash := sha256.Sum256([]byte(normalizedSSID))
	suffix := hex.EncodeToString(hash[:4])
	slug := slugifyWiFiSSID(normalizedSSID)
	if slug == "" {
		return jetsonWiFiConnectionPrefix + suffix
	}
	return jetsonWiFiConnectionPrefix + slug + "-" + suffix
}

func slugifyWiFiSSID(ssid string) string {
	var builder strings.Builder
	previousWasDash := false
	for _, character := range ssid {
		switch {
		case character >= 'a' && character <= 'z',
			character >= '0' && character <= '9':
			builder.WriteRune(character)
			previousWasDash = false
		case character == '-' || character == '_' || character == ' ' || character == '.':
			if !previousWasDash && builder.Len() > 0 {
				builder.WriteRune('-')
				previousWasDash = true
			}
		}
	}
	slug := strings.Trim(builder.String(), "-")
	const maximumSlugLength = 24
	if len(slug) > maximumSlugLength {
		slug = strings.TrimRight(slug[:maximumSlugLength], "-")
	}
	return slug
}

func jetsonWiFiConnectionPath(ssid string) string {
	return jetsonWiFiConnectionDirectory + "/" + jetsonWiFiConnectionID(ssid) + ".nmconnection"
}

func jetsonWiFiConnectionUUID(ssid string) string {
	hash := sha256.Sum256([]byte("internkim-wifi:" + strings.ToLower(strings.TrimSpace(ssid))))
	hexValue := hex.EncodeToString(hash[:16])
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexValue[0:8], hexValue[8:12], hexValue[12:16], hexValue[16:20], hexValue[20:32])
}

func buildJetsonWiFiUpsertScript(profiles []resolvedWiFiProfile) string {
	var builder strings.Builder
	for _, profile := range profiles {
		connectionID := jetsonWiFiConnectionID(profile.SSID)
		hiddenValue := "no"
		if profile.IsHidden {
			hiddenValue = "yes"
		}
		builder.WriteString("nmcli connection delete " + quoteShellValue(connectionID) + " >/dev/null 2>&1 || true\n")
		builder.WriteString("nmcli connection add type wifi ifname '*' con-name " + quoteShellValue(connectionID) + " ssid " + quoteShellValue(profile.SSID) + "\n")
		builder.WriteString("nmcli connection modify " + quoteShellValue(connectionID) + " connection.autoconnect yes connection.autoconnect-priority 0 wifi.hidden " + hiddenValue + " 802-11-wireless.powersave 2 ipv4.method auto ipv4.dns \"1.1.1.1 8.8.8.8\" ipv4.ignore-auto-dns yes ipv4.route-metric 600 ipv6.method auto ipv6.route-metric 600\n")
		if !profile.IsOpen {
			builder.WriteString("nmcli connection modify " + quoteShellValue(connectionID) + " wifi-sec.key-mgmt wpa-psk wifi-sec.psk " + quoteShellValue(profile.Password) + "\n")
		}
	}
	return strings.TrimSpace(builder.String())
}

func buildJetsonWiFiInstallScript(profiles []resolvedWiFiProfile) string {
	upsertScript := buildJetsonWiFiUpsertScript(profiles)
	ethernetPriorityScript := buildJetsonEthernetPriorityScript()
	selectorScript := buildJetsonWiFiSelectorScript()
	recoveryService := buildJetsonWiFiRecoveryService()
	recoveryTimer := buildJetsonWiFiRecoveryTimer()
	networkSnapshotFilesScript := buildJetsonNetworkSnapshotFilesScript()
	tunnelRecoveryFilesScript := buildJetsonTunnelRecoveryFilesScript()
	return fmt.Sprintf(`systemctl unmask NetworkManager.service 2>/dev/null || true
systemctl enable --now NetworkManager.service 2>/dev/null || true
nmcli radio wifi on
%s
%s
%s
%s
cat > %s <<'WIFIEOF'
%s
WIFIEOF
chmod 755 %s
cat > %s <<'SERVICEEOF'
%s
SERVICEEOF
cat > %s <<'TIMEREOF'
%s
TIMEREOF
systemctl restart systemd-journald 2>/dev/null || true
systemctl daemon-reload
systemctl enable --now internkim-wifi-recovery.timer internkim-network-snapshot.timer internkim-tunnel-recovery.timer
systemctl start internkim-network-snapshot.service 2>/dev/null || true`,
		upsertScript,
		ethernetPriorityScript,
		networkSnapshotFilesScript,
		tunnelRecoveryFilesScript,
		jetsonWiFiSelectorScriptPath,
		selectorScript,
		jetsonWiFiSelectorScriptPath,
		jetsonWiFiRecoveryServicePath,
		recoveryService,
		jetsonWiFiRecoveryTimerPath,
		recoveryTimer,
	)
}

func buildJetsonEthernetPriorityScript() string {
	return strings.TrimSpace(`
ethernet_connection=` + quoteShellValue(jetsonEthernetConnectionID) + `
ethernet_device="$(nmcli -t -f DEVICE,TYPE device status | awk -F: '$2 == "ethernet" && $1 !~ /^(usb|l4tbr|bctap|lo)/ {print $1; exit}')"
if [ -n "$ethernet_device" ]; then
  nmcli connection delete "Wired connection 1" >/dev/null 2>&1 || true
  if ! nmcli -t -f NAME connection show | grep -Fxq "$ethernet_connection"; then
    nmcli connection add type ethernet ifname "$ethernet_device" con-name "$ethernet_connection"
  fi
  nmcli connection modify "$ethernet_connection" connection.autoconnect yes connection.autoconnect-priority 100 ipv4.method auto ipv4.dns "1.1.1.1 8.8.8.8" ipv4.ignore-auto-dns yes ipv4.route-metric 100 ipv6.method auto ipv6.route-metric 100
fi
for connection in $(nmcli -t -f NAME,TYPE connection show | awk -F: '$2 ~ /^(802-11-wireless|wifi)$/ && $1 ~ /^internkim-wifi-/ {print $1}'); do
  nmcli connection modify "$connection" connection.autoconnect yes connection.autoconnect-priority 0 802-11-wireless.powersave 2 ipv4.dns "1.1.1.1 8.8.8.8" ipv4.ignore-auto-dns yes ipv4.route-metric 600 ipv6.route-metric 600
done`)
}

func buildJetsonNetworkSnapshotFilesScript() string {
	return fmt.Sprintf(`mkdir -p /usr/local/lib/internkim /var/log/internkim/network-snapshots /var/log/journal /etc/systemd/journald.conf.d
cat > %s <<'JOURNALEOF'
%s
JOURNALEOF
cat > %s <<'SNAPSHOEOF'
%s
SNAPSHOEOF
chmod 755 %s
cat > %s <<'SNAPSHOTSERVICEEOF'
%s
SNAPSHOTSERVICEEOF
cat > %s <<'SNAPSHOTTIMEREOF'
%s
SNAPSHOTTIMEREOF`,
		jetsonPersistentJournalConfigurationPath,
		buildJetsonPersistentJournalConfiguration(),
		jetsonNetworkSnapshotScriptPath,
		buildJetsonNetworkSnapshotScript(),
		jetsonNetworkSnapshotScriptPath,
		jetsonNetworkSnapshotServicePath,
		buildJetsonNetworkSnapshotService(),
		jetsonNetworkSnapshotTimerPath,
		buildJetsonNetworkSnapshotTimer(),
	)
}

func buildJetsonWiFiSelectorScript() string {
	return strings.TrimSpace(`#!/bin/sh
set -eu
python3 - <<'PY'
import subprocess
import sys
import time

def output(arguments):
    completed = subprocess.run(arguments, text=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    return completed.stdout.strip()

def wireless_address():
    for line in output(["ip", "-o", "-4", "addr", "show", "scope", "global"]).splitlines():
        parts = line.split()
        if len(parts) >= 4 and (parts[1].startswith("wl") or parts[1].startswith("wlan")):
            return parts[3].split("/", 1)[0]
    return ""

def connection_names():
    names = []
    for line in output(["nmcli", "-t", "-f", "NAME,TYPE", "connection", "show"]).splitlines():
        parts = line.split(":", 1)
        if len(parts) == 2 and parts[0].startswith("internkim-wifi-") and parts[1] in ("802-11-wireless", "wifi"):
            names.append(parts[0])
    return names

def connection_record(name):
    ssid = output(["nmcli", "-g", "802-11-wireless.ssid", "connection", "show", name])
    keyManagement = output(["nmcli", "-g", "802-11-wireless-security.key-mgmt", "connection", "show", name])
    return {"name": name, "ssid": ssid, "isOpen": keyManagement in ("", "--")}

def scan_signals():
    signals = {}
    for line in output(["nmcli", "-t", "-f", "SSID,SIGNAL", "device", "wifi", "list"]).splitlines():
        parts = line.rsplit(":", 1)
        if len(parts) != 2 or not parts[0]:
            continue
        try:
            signal = int(parts[1])
        except ValueError:
            continue
        signals[parts[0]] = max(signals.get(parts[0], 0), signal)
    return signals

if wireless_address():
    sys.exit(0)

subprocess.run(["nmcli", "radio", "wifi", "on"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
subprocess.run(["nmcli", "device", "wifi", "rescan"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

signals = scan_signals()
records = []
for name in connection_names():
    record = connection_record(name)
    if not record["ssid"]:
        continue
    record["signal"] = signals.get(record["ssid"], 0)
    records.append(record)

records.sort(key=lambda record: (record["isOpen"], -record["signal"], record["ssid"].lower()))
for record in records:
    subprocess.run(["nmcli", "connection", "up", record["name"]], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(2)
    if wireless_address():
        print(record["name"])
        sys.exit(0)

sys.exit(1)
PY`)
}

func buildJetsonWiFiRecoveryService() string {
	return `[Unit]
Description=Intern Kim Wi-Fi Recovery
After=NetworkManager.service
Wants=NetworkManager.service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/internkim-wifi-select
TimeoutStartSec=90

`
}

func buildJetsonWiFiRecoveryTimer() string {
	return `[Unit]
Description=Intern Kim Wi-Fi Recovery Timer

[Timer]
OnBootSec=20s
OnUnitActiveSec=2min
Unit=internkim-wifi-recovery.service

[Install]
WantedBy=timers.target
`
}

func buildJetsonPersistentJournalConfiguration() string {
	return `[Journal]
Storage=persistent
SystemMaxUse=512M
`
}

func buildJetsonNetworkSnapshotScript() string {
	return strings.TrimSpace(`#!/bin/sh
set +e
output_dir=/var/log/internkim/network-snapshots
mkdir -p "$output_dir"
output_file="$output_dir/$(date -u +%Y%m%dT%H%M%SZ).log"
section() {
  printf "\n== %s ==\n" "$1"
}
{
  section time
  date -u
  uptime
  section routes
  ip route show default
  ip -brief address show
  nmcli -t -f DEVICE,TYPE,STATE,CONNECTION device status
  section ethernet
  for device in /sys/class/net/en*; do
    [ -e "$device" ] || continue
    name=$(basename "$device")
    printf "%s carrier=" "$name"
    cat "$device/carrier" 2>/dev/null || true
    printf "%s operstate=" "$name"
    cat "$device/operstate" 2>/dev/null || true
    ethtool "$name" 2>/dev/null | grep -E "Speed|Duplex|Auto-negotiation|Link detected" || true
  done
  section wifi
  iw dev 2>/dev/null | sed -n "1,80p"
  section pressure
  cat /proc/pressure/cpu /proc/pressure/memory /proc/pressure/io 2>/dev/null
  section memory
  free -h
  swapon --show 2>/dev/null
  section top
  top -b -n 1 | head -30
  section processes
  ps -eo pcpu,pmem,rss,pid,comm --sort=-pcpu | head -25
  section failed-services
  systemctl --no-pager --failed
  section service-states
  systemctl is-active ssh cloudflared cloudflared-node-ssh NetworkManager 2>/dev/null
  section cloudflared-journal
  journalctl -u cloudflared -u cloudflared-node-ssh -n 120 --no-pager 2>/dev/null
  section networkmanager-journal
  journalctl -u NetworkManager -n 80 --no-pager 2>/dev/null
  section kernel-network
  journalctl -k -n 120 --no-pager 2>/dev/null | grep -i -E "oom|killed process|eth|wifi|wlan|carrier|link|r816|network|timeout|tls|dns|error|fail|reset" | tail -80
} > "$output_file"
find "$output_dir" -type f -name "*.log" -mtime +14 -delete
ls -1t "$output_dir"/*.log 2>/dev/null | tail -n +289 | xargs -r rm -f
printf "%s\n" "$output_file"`)
}

func buildJetsonNetworkSnapshotService() string {
	return `[Unit]
Description=Intern Kim Network Snapshot
After=NetworkManager.service
Wants=NetworkManager.service

[Service]
Type=oneshot
ExecStart=/usr/local/lib/internkim/network-snapshot.sh
TimeoutStartSec=60
StandardOutput=journal
StandardError=journal

`
}

func buildJetsonNetworkSnapshotTimer() string {
	return `[Unit]
Description=Intern Kim Network Snapshot Timer

[Timer]
OnBootSec=1min
OnUnitActiveSec=5min
Unit=internkim-network-snapshot.service

[Install]
WantedBy=timers.target
`
}

func buildJetsonTunnelRecoveryFilesScript() string {
	return fmt.Sprintf(`mkdir -p /usr/local/lib/internkim
cat > %s <<'TUNNELRECOVERYEOF'
%s
TUNNELRECOVERYEOF
chmod 755 %s
cat > %s <<'TUNNELRECOVERYSERVICEEOF'
%s
TUNNELRECOVERYSERVICEEOF
cat > %s <<'TUNNELRECOVERYTIMEREOF'
%s
TUNNELRECOVERYTIMEREOF`,
		jetsonTunnelRecoveryScriptPath,
		buildJetsonTunnelRecoveryScript(),
		jetsonTunnelRecoveryScriptPath,
		jetsonTunnelRecoveryServicePath,
		buildJetsonTunnelRecoveryService(),
		jetsonTunnelRecoveryTimerPath,
		buildJetsonTunnelRecoveryTimer(),
	)
}

func buildJetsonTunnelRecoveryScript() string {
	return strings.TrimSpace(`#!/bin/sh
set +e
window=-75s
registered=$(journalctl -u cloudflared --since="$window" --no-pager 2>/dev/null | grep -c "Registered tunnel connection")
disconnected=$(journalctl -u cloudflared --since="$window" --no-pager 2>/dev/null | grep -cE "Lost connection|Serve tunnel error|Connection terminated|i/o timeout|Unregistered tunnel")
if [ "$registered" -gt 0 ]; then
  exit 0
fi
if [ "$disconnected" -eq 0 ]; then
  exit 0
fi
logger -t internkim-tunnel-recovery "cloudflared stuck disconnected ($disconnected events, no re-registration in 75s); restarting"
systemctl restart cloudflared`)
}

func buildJetsonTunnelRecoveryService() string {
	return `[Unit]
Description=Intern Kim Tunnel Recovery
After=cloudflared.service
Wants=cloudflared.service

[Service]
Type=oneshot
ExecStart=/usr/local/lib/internkim/tunnel-recovery.sh
TimeoutStartSec=45

`
}

func buildJetsonTunnelRecoveryTimer() string {
	return `[Unit]
Description=Intern Kim Tunnel Recovery Timer

[Timer]
OnBootSec=90s
OnUnitActiveSec=60s
Unit=internkim-tunnel-recovery.service

[Install]
WantedBy=timers.target
`
}
