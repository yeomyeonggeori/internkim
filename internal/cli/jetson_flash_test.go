package cli

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareJetsonImageAcceptsRawImage(t *testing.T) {
	imagePath := filepath.Join(t.TempDir(), "sd-blob.img")
	if errorValue := os.WriteFile(imagePath, []byte("image"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	preparedPath, errorValue := prepareJetsonImage(imagePath, t.TempDir())
	if errorValue != nil {
		t.Fatalf("expected raw image: %v", errorValue)
	}
	if preparedPath != imagePath {
		t.Fatalf("expected raw image path, got %s", preparedPath)
	}
}

func TestPrepareJetsonImageExtractsImageFromZip(t *testing.T) {
	temporaryDirectory := t.TempDir()
	zipPath := filepath.Join(temporaryDirectory, "jetson.zip")
	createZipFixture(t, zipPath, "nested/sd-blob.img", []byte("jetson image"))

	preparedPath, errorValue := prepareJetsonImage(zipPath, temporaryDirectory)
	if errorValue != nil {
		t.Fatalf("expected zip extraction: %v", errorValue)
	}

	document, errorValue := os.ReadFile(preparedPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(document) != "jetson image" {
		t.Fatalf("expected extracted image content, got %q", string(document))
	}
	if filepath.Base(preparedPath) != "sd-blob.img" {
		t.Fatalf("expected normalized image name, got %s", preparedPath)
	}
}

func TestPrepareJetsonImageRejectsUnsupportedFormat(t *testing.T) {
	_, errorValue := prepareJetsonImage("jetson.iso", t.TempDir())
	if errorValue == nil || !strings.Contains(errorValue.Error(), "unsupported") {
		t.Fatalf("expected unsupported format error, got %v", errorValue)
	}
}

func TestInvalidJetsonImageCacheDetectsPartialZip(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "jetson.zip")
	if errorValue := os.WriteFile(zipPath, []byte("partial"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	if !isInvalidJetsonImageCache(zipPath) {
		t.Fatal("expected partial zip cache to be invalid")
	}
}

func TestEnsureJetsonAccountFilesCreatesSudoUser(t *testing.T) {
	passwd := "root:x:0:0:root:/root:/bin/bash\n"
	group := "root:x:0:\nsudo:x:27:\nvideo:x:44:nvidia\n"
	shadow := "root:*:19000:0:99999:7:::\n"
	gshadow := "root:*::\nsudo:*::\nvideo:*::nvidia\n"

	userID := nextLinuxUserID(passwd)
	passwd = ensurePasswdUser(passwd, "internkim", userID, userID)
	group = ensureGroupUser(group, "internkim", userID)
	shadow = ensureShadowUser(shadow, "internkim", "$6$hash")
	gshadow = ensureGShadowUser(gshadow, "internkim")

	for _, fragment := range []string{
		"internkim:x:1000:1000:Intern Kim:/home/internkim:/bin/bash",
		"internkim:$6$hash:",
		"internkim:x:1000:",
		"sudo:x:27:internkim",
		"video:x:44:nvidia,internkim",
		"sudo:*::internkim",
	} {
		document := passwd + shadow + group + gshadow
		if !strings.Contains(document, fragment) {
			t.Fatalf("expected account document to include %q, got:\n%s", fragment, document)
		}
	}
}

func TestHumanBytesFormatsGibibytes(t *testing.T) {
	if formattedValue := humanBytes(3 * 1024 * 1024 * 1024); formattedValue != "3.0GiB" {
		t.Fatalf("expected GiB formatting, got %s", formattedValue)
	}
}

func TestResolveJetsonFlashWiFiPrefersCurrentSSID(t *testing.T) {
	temporaryDirectory := t.TempDir()
	saveState(temporaryDirectory, "wifi_ssid", "OldWiFi")
	saveState(temporaryDirectory, "wifi_pass", "old-secret")
	getSSIDPath := createExecutableFixture(t, "dlee5G\n")
	withArguments(t, "internkim", "flash", "--fix-oem-user", "--yes", "--wifi-password", "new-secret")

	wifiSSID, wifiPassword, errorValue := resolveJetsonFlashWiFi(newMsg("en"), temporaryDirectory, getSSIDPath)
	if errorValue != nil {
		t.Fatalf("expected Wi-Fi resolution: %v", errorValue)
	}
	if wifiSSID != "dlee5G" {
		t.Fatalf("expected current SSID, got %q", wifiSSID)
	}
	if wifiPassword != "new-secret" {
		t.Fatalf("expected explicit password, got %q", wifiPassword)
	}
	profiles := loadWiFiProfiles(temporaryDirectory)
	if len(profiles) != 2 {
		t.Fatalf("expected legacy and current Wi-Fi profiles to be preserved, got %+v", profiles)
	}
}

func TestResolveJetsonFlashWiFiYesFailsWithoutSSID(t *testing.T) {
	temporaryDirectory := t.TempDir()
	getSSIDPath := createExecutableFixture(t, "Unknown\n")
	withArguments(t, "internkim", "flash", "--fix-oem-user", "--yes")

	_, _, errorValue := resolveJetsonFlashWiFi(newMsg("en"), temporaryDirectory, getSSIDPath)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "SSID") {
		t.Fatalf("expected missing SSID error, got %v", errorValue)
	}
}

func TestBuildJetsonNetworkManagerWiFiConnectionIncludesCredentials(t *testing.T) {
	document := buildJetsonNetworkManagerWiFiConnection("Office WiFi", "secret", false)
	for _, fragment := range []string{"[connection]", "id=" + jetsonWiFiConnectionID("Office WiFi"), "uuid=" + jetsonWiFiConnectionUUID("Office WiFi"), "type=wifi", "ssid=Office WiFi", "key-mgmt=wpa-psk", "psk=secret", "hidden=false"} {
		if !strings.Contains(document, fragment) {
			t.Fatalf("expected NetworkManager document to include %q, got:\n%s", fragment, document)
		}
	}
	for _, forbiddenFragment := range []string{"wlan0", "interface-name", "network:"} {
		if strings.Contains(document, forbiddenFragment) {
			t.Fatalf("NetworkManager document must not include %q, got:\n%s", forbiddenFragment, document)
		}
	}
}

func TestBuildJetsonNetworkManagerWiFiConnectionMarksHiddenSSID(t *testing.T) {
	document := buildJetsonNetworkManagerWiFiConnection("Office WiFi", "secret", true)
	if !strings.Contains(document, "hidden=true") {
		t.Fatalf("hidden SSID document must include hidden=true, got:\n%s", document)
	}
}

func TestBuildJetsonNetworkManagerWiFiConnectionSupportsOpenNetwork(t *testing.T) {
	document := buildJetsonNetworkManagerWiFiConnection("Office WiFi", "", false)
	if strings.Contains(document, "[wifi-security]") || strings.Contains(document, "psk=") {
		t.Fatalf("open NetworkManager document must not include Wi-Fi security, got:\n%s", document)
	}
	if !strings.Contains(document, "ssid=Office WiFi") {
		t.Fatalf("expected SSID, got:\n%s", document)
	}
}

func TestJetsonWiFiConnectionIDIncludesSSIDSlug(t *testing.T) {
	cases := map[string]string{
		"didimdol1":         "internkim-wifi-didimdol1-",
		"KT_GiGA_5G_41BF":   "internkim-wifi-kt-giga-5g-41bf-",
		"  Office  Wi-Fi  ": "internkim-wifi-office-wi-fi-",
		"!@#$%":             "internkim-wifi-",
	}
	for ssid, prefix := range cases {
		identifier := jetsonWiFiConnectionID(ssid)
		if !strings.HasPrefix(identifier, prefix) {
			t.Fatalf("ssid %q produced id %q, expected prefix %q", ssid, identifier, prefix)
		}
		if !strings.HasPrefix(identifier, "internkim-wifi-") {
			t.Fatalf("ssid %q produced id %q without expected wifi prefix", ssid, identifier)
		}
	}
}

func TestBuildJetsonWiFiSelectorPrefersSecureBeforeOpen(t *testing.T) {
	document := buildJetsonWiFiSelectorScript()
	for _, fragment := range []string{`record["isOpen"]`, `-record["signal"]`, `records.sort`} {
		if !strings.Contains(document, fragment) {
			t.Fatalf("expected Wi-Fi selector to include %q, got:\n%s", fragment, document)
		}
	}
}

func TestBuildJetsonWiFiSelectorUsesWirelessAddressOnly(t *testing.T) {
	document := buildJetsonWiFiSelectorScript()
	for _, fragment := range []string{`def wireless_address()`, `parts[1].startswith("wl")`, `return ""`, `if wireless_address():`} {
		if !strings.Contains(document, fragment) {
			t.Fatalf("expected Wi-Fi selector to include %q, got:\n%s", fragment, document)
		}
	}
	if strings.Contains(document, `if output(["ip", "-o", "-4", "addr", "show", "scope", "global"])`) {
		t.Fatalf("Wi-Fi selector must not treat non-wireless IPv4 as success, got:\n%s", document)
	}
}

func TestBuildJetsonWiFiRecoveryUnits(t *testing.T) {
	serviceDocument := buildJetsonWiFiRecoveryService()
	for _, fragment := range []string{"After=NetworkManager.service", "Wants=NetworkManager.service", "Type=oneshot", "ExecStart=/usr/local/bin/internkim-wifi-select", "TimeoutStartSec=90"} {
		if !strings.Contains(serviceDocument, fragment) {
			t.Fatalf("expected Wi-Fi recovery service to include %q, got:\n%s", fragment, serviceDocument)
		}
	}

	timerDocument := buildJetsonWiFiRecoveryTimer()
	for _, fragment := range []string{"OnBootSec=20s", "OnUnitActiveSec=2min", "Unit=internkim-wifi-recovery.service", "WantedBy=timers.target"} {
		if !strings.Contains(timerDocument, fragment) {
			t.Fatalf("expected Wi-Fi recovery timer to include %q, got:\n%s", fragment, timerDocument)
		}
	}

	snapshotDocument := buildJetsonNetworkSnapshotScript()
	for _, fragment := range []string{"network-snapshots", "ps -eo pcpu,pmem,rss,pid,comm", "journalctl -u cloudflared", "Link detected"} {
		if !strings.Contains(snapshotDocument, fragment) {
			t.Fatalf("expected network snapshot script to include %q, got:\n%s", fragment, snapshotDocument)
		}
	}
	if strings.Contains(snapshotDocument, "ps -eo args") {
		t.Fatalf("network snapshot script must avoid process arguments, got:\n%s", snapshotDocument)
	}

	snapshotTimerDocument := buildJetsonNetworkSnapshotTimer()
	for _, fragment := range []string{"OnBootSec=1min", "OnUnitActiveSec=5min", "Unit=internkim-network-snapshot.service", "WantedBy=timers.target"} {
		if !strings.Contains(snapshotTimerDocument, fragment) {
			t.Fatalf("expected network snapshot timer to include %q, got:\n%s", fragment, snapshotTimerDocument)
		}
	}

	journalDocument := buildJetsonPersistentJournalConfiguration()
	for _, fragment := range []string{"Storage=persistent", "SystemMaxUse=512M"} {
		if !strings.Contains(journalDocument, fragment) {
			t.Fatalf("expected persistent journal configuration to include %q, got:\n%s", fragment, journalDocument)
		}
	}
}

func TestBuildJetsonFirstbootScriptStartsNetworkAndSSH(t *testing.T) {
	document := buildJetsonFirstbootScript()
	for _, fragment := range []string{
		"internkim-jetson-firstboot.log",
		"expand_rootfs",
		"resize2fs \"$rootSource\"",
		"systemctl start NetworkManager.service",
		"internkim-ethernet",
		"ipv4.route-metric 100",
		"internkim-wifi-select",
		"internkim-network-snapshot.timer",
		"Storage=persistent",
		"systemctl restart ssh.service",
		"/var/lib/internkim/board-ip",
		"jetson-firstboot.done",
		"Jetson firstboot complete",
	} {
		if !strings.Contains(document, fragment) {
			t.Fatalf("expected firstboot script to include %q, got:\n%s", fragment, document)
		}
	}
}

func TestBuildJetsonFirstbootScriptStartsSSHBeforeRootExpansion(t *testing.T) {
	document := buildJetsonFirstbootScript()
	sshIndex := strings.Index(document, "systemctl restart ssh.service")
	expandIndex := strings.Index(document, "expand_rootfs\n")
	if sshIndex < 0 || expandIndex < 0 {
		t.Fatalf("expected SSH restart and rootfs expansion in firstboot script, got:\n%s", document)
	}
	if sshIndex > expandIndex {
		t.Fatalf("expected SSH to start before rootfs expansion, got:\n%s", document)
	}
}

func TestBuildJetsonFirstbootServiceRunsBeforeSetup(t *testing.T) {
	document := buildJetsonFirstbootService()
	for _, fragment := range []string{"After=local-fs.target NetworkManager.service", "Wants=NetworkManager.service", "ExecStart=/usr/local/bin/internkim-jetson-firstboot.sh", "WantedBy=multi-user.target"} {
		if !strings.Contains(document, fragment) {
			t.Fatalf("expected firstboot service to include %q, got:\n%s", fragment, document)
		}
	}
	if strings.Contains(document, "After=multi-user.target") {
		t.Fatalf("firstboot service must not wait for multi-user.target, got:\n%s", document)
	}
}

func TestValidateJetsonRootPatchDocumentsAcceptsCompletePatch(t *testing.T) {
	errorValue := validateJetsonRootPatchDocuments(
		completeJetsonAccountFilesFixture(),
		completeJetsonPatchDocumentsFixture(),
		jetsonDefaultUser,
		[]resolvedWiFiProfile{{SSID: "Office WiFi", Password: "secret"}},
	)
	if errorValue != nil {
		t.Fatalf("expected complete patch to validate: %v", errorValue)
	}
}

func TestValidateJetsonRootPatchDocumentsRejectsMissingWiFi(t *testing.T) {
	documents := completeJetsonPatchDocumentsFixture()
	documents.wifiConnection = ""
	errorValue := validateJetsonRootPatchDocuments(
		completeJetsonAccountFilesFixture(),
		documents,
		jetsonDefaultUser,
		[]resolvedWiFiProfile{{SSID: "Office WiFi", Password: "secret"}},
	)
	if errorValue == nil || !strings.Contains(errorValue.Error(), jetsonWiFiConnectionPath("Office WiFi")) {
		t.Fatalf("expected missing Wi-Fi verification error, got %v", errorValue)
	}
}

func createZipFixture(t *testing.T, zipPath string, entryName string, document []byte) {
	t.Helper()
	zipFile, errorValue := os.Create(zipPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer zipFile.Close()

	writer := zip.NewWriter(zipFile)
	defer writer.Close()

	entry, errorValue := writer.Create(entryName)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := entry.Write(document); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func createExecutableFixture(t *testing.T, output string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "get-ssid")
	content := "#!/bin/sh\nprintf '%s' " + quoteShellValue(output) + "\n"
	if errorValue := os.WriteFile(path, []byte(content), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}

func withArguments(t *testing.T, arguments ...string) {
	t.Helper()
	previousArguments := os.Args
	os.Args = arguments
	t.Cleanup(func() {
		os.Args = previousArguments
	})
}

func completeJetsonAccountFilesFixture() jetsonAccountFiles {
	return jetsonAccountFiles{
		passwd: "root:x:0:0:root:/root:/bin/bash\ninternkim:x:1000:1000:Intern Kim:/home/internkim:/bin/bash\n",
		shadow: "root:*:19000:0:99999:7:::\ninternkim:$6$hash:19000:0:99999:7:::\n",
		group:  "root:x:0:\nsudo:x:27:internkim\ninternkim:x:1000:\n",
	}
}

func completeJetsonPatchDocumentsFixture() jetsonPatchDocuments {
	return jetsonPatchDocuments{
		hostname:         "internkim\n",
		sshConfig:        "PasswordAuthentication yes\nPubkeyAuthentication yes\n",
		oemMarker:        "1\n",
		wifiConnection:   buildJetsonNetworkManagerWiFiConnection("Office WiFi", "secret", false),
		wifiSelector:     buildJetsonWiFiSelectorScript(),
		wifiRecovery:     buildJetsonWiFiRecoveryService(),
		wifiTimer:        buildJetsonWiFiRecoveryTimer(),
		networkSnapshot:  buildJetsonNetworkSnapshotScript(),
		networkTimer:     buildJetsonNetworkSnapshotTimer(),
		journalConfig:    buildJetsonPersistentJournalConfiguration(),
		firstbootScript:  buildJetsonFirstbootScript(),
		firstbootService: buildJetsonFirstbootService(),
	}
}
