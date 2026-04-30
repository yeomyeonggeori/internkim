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

type wifiProfile struct {
	SSID       string `json:"ssid"`
	IsOpen     bool   `json:"isOpen"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
	Source     string `json:"source,omitempty"`
}

type resolvedWiFiProfile struct {
	SSID     string
	Password string
	IsOpen   bool
}

func resolveWiFiProfiles(messenger *msg, stateDirectory string, getSSIDPath string) ([]resolvedWiFiProfile, error) {
	profiles := loadWiFiProfiles(stateDirectory)
	selectedSSID := strings.TrimSpace(argString("--wifi-ssid", ""))
	explicitPassword := argString("--wifi-password", "")
	currentSSID := detectSSID(getSSIDPath)
	isOpenWiFi := containsArg("--wifi-open") || containsArg("--open-wifi")

	if selectedSSID == "" {
		selectedSSID = currentSSID
	}
	if selectedSSID == "" && len(profiles) == 0 {
		if containsArg("--no-wifi") {
			return nil, nil
		}
		if isNonInteractiveWiFiResolution() {
			return nil, errors.New(messenger.t("Wi-Fi SSID를 자동으로 찾지 못했습니다. Mac을 대상 Wi-Fi에 연결하거나 --wifi-ssid 를 지정하세요.", "Wi-Fi SSID was not detected automatically. Connect the Mac to the target Wi-Fi or pass --wifi-ssid."))
		}
		selectedSSID = strings.TrimSpace(readLine(messenger.t("  Wi-Fi SSID 입력: ", "  Enter Wi-Fi SSID: ")))
	}
	if selectedSSID != "" {
		if !isOpenWiFi {
			isOpenWiFi = isSavedOpenWiFi(profiles, selectedSSID)
		}
		profiles = upsertWiFiProfile(profiles, wifiProfile{
			SSID:       selectedSSID,
			IsOpen:     isOpenWiFi,
			LastUsedAt: time.Now().UTC().Format(time.RFC3339),
			Source:     wifiProfileSource(selectedSSID, currentSSID),
		})
		saveWiFiProfiles(stateDirectory, profiles)
		printSelectedWiFi(messenger, selectedSSID, currentSSID, len(profiles))
	}
	return resolveWiFiProfilePasswords(messenger, stateDirectory, profiles, selectedSSID, explicitPassword)
}

func resolveWiFiProfilePasswords(messenger *msg, stateDirectory string, profiles []wifiProfile, selectedSSID string, explicitPassword string) ([]resolvedWiFiProfile, error) {
	legacySSID := loadState(stateDirectory, "wifi_ssid")
	legacyPassword := loadState(stateDirectory, "wifi_pass")
	resolvedProfiles := make([]resolvedWiFiProfile, 0, len(profiles))
	for _, profile := range profiles {
		password := ""
		if !profile.IsOpen {
			password = resolveWiFiPassword(profile.SSID, selectedSSID, explicitPassword, legacySSID, legacyPassword)
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
		resolvedProfiles = append(resolvedProfiles, resolvedWiFiProfile{SSID: profile.SSID, Password: password, IsOpen: profile.IsOpen})
	}
	if selectedSSID != "" && !containsResolvedWiFiProfile(resolvedProfiles, selectedSSID) {
		return nil, fmt.Errorf("wifi password is empty for %s", selectedSSID)
	}
	return resolvedProfiles, nil
}

func isNonInteractiveWiFiResolution() bool {
	return containsArg("--yes") || containsArg("--non-interactive")
}

func resolveWiFiPassword(ssid string, selectedSSID string, explicitPassword string, legacySSID string, legacyPassword string) string {
	if explicitPassword != "" && strings.EqualFold(ssid, selectedSSID) {
		return explicitPassword
	}
	if legacyPassword != "" && strings.EqualFold(ssid, legacySSID) {
		return legacyPassword
	}
	if keychainPassword := getKeychainPassword(ssid); keychainPassword != "" {
		return keychainPassword
	}
	return ""
}

func printSelectedWiFi(messenger *msg, selectedSSID string, currentSSID string, profileCount int) {
	switch {
	case currentSSID != "" && selectedSSID == currentSSID:
		fmt.Printf("  SSID: %s (%s)\n", selectedSSID, messenger.t("현재 Mac Wi-Fi", "current Mac Wi-Fi"))
	default:
		fmt.Printf("  SSID: %s\n", selectedSSID)
	}
	if profileCount > 1 {
		fmt.Printf("  Wi-Fi profiles: %d\n", profileCount)
	}
}

func wifiProfileSource(selectedSSID string, currentSSID string) string {
	if currentSSID != "" && selectedSSID == currentSSID {
		return "mac"
	}
	return "manual"
}

func loadWiFiProfiles(stateDirectory string) []wifiProfile {
	profiles := readWiFiProfilesFile(stateDirectory)
	legacySSID := loadState(stateDirectory, "wifi_ssid")
	if legacySSID != "" {
		profiles = upsertWiFiProfile(profiles, wifiProfile{
			SSID:   legacySSID,
			IsOpen: loadState(stateDirectory, "wifi_open") == "true",
			Source: "legacy",
		})
	}
	return sortWiFiProfiles(profiles)
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

func containsResolvedWiFiProfile(profiles []resolvedWiFiProfile, ssid string) bool {
	for _, profile := range profiles {
		if strings.EqualFold(profile.SSID, ssid) {
			return true
		}
	}
	return false
}

func jetsonWiFiConnectionID(ssid string) string {
	hash := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(ssid))))
	return jetsonWiFiConnectionPrefix + hex.EncodeToString(hash[:4])
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
		builder.WriteString("nmcli connection delete " + quoteShellValue(connectionID) + " >/dev/null 2>&1 || true\n")
		builder.WriteString("nmcli connection add type wifi ifname '*' con-name " + quoteShellValue(connectionID) + " ssid " + quoteShellValue(profile.SSID) + "\n")
		builder.WriteString("nmcli connection modify " + quoteShellValue(connectionID) + " connection.autoconnect yes wifi.hidden yes ipv4.method auto ipv6.method auto\n")
		if !profile.IsOpen {
			builder.WriteString("nmcli connection modify " + quoteShellValue(connectionID) + " wifi-sec.key-mgmt wpa-psk wifi-sec.psk " + quoteShellValue(profile.Password) + "\n")
		}
	}
	return strings.TrimSpace(builder.String())
}

func buildJetsonWiFiSelectorScript() string {
	return strings.TrimSpace(`#!/bin/sh
set -eu
nmcli radio wifi on 2>/dev/null || true
nmcli device wifi rescan 2>/dev/null || true
python3 - <<'PY'
import subprocess
import sys
import time

def output(arguments):
    completed = subprocess.run(arguments, text=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    return completed.stdout.strip()

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
    if output(["ip", "-o", "-4", "addr", "show", "scope", "global"]):
        print(record["name"])
        sys.exit(0)

sys.exit(1)
PY`)
}
