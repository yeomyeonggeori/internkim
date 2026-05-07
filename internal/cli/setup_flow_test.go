package cli

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	setup "gitlab.com/eastriver/internkim/internal/provisioning/steps"
)

func TestUsersSyncDependencyInstallScriptInstallsJQ(t *testing.T) {
	script := usersSyncDependencyInstallScript()
	for _, fragment := range []string{"command -v jq", "apt-get install", "jq curl ca-certificates"} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected users sync dependency script to include %q, got:\n%s", fragment, script)
		}
	}
}

func TestDeviceToolPackagesIncludeSitePublishingBasics(t *testing.T) {
	packages := strings.Join(baseDeviceToolPackages(), " ")
	for _, packageName := range []string{"git", "curl", "unzip", "ca-certificates"} {
		if !strings.Contains(packages, packageName) {
			t.Fatalf("expected base device tools to include %q, got %s", packageName, packages)
		}
	}
}

func TestExtractFromZipWritesArchiveEntry(t *testing.T) {
	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	fileWriter, errorValue := zipWriter.Create("pocketbase")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := fileWriter.Write([]byte("binary")); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := zipWriter.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	path := filepath.Join(t.TempDir(), "pocketbase")

	errorValue = extractFromZip(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()), path, "pocketbase")

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(document) != "binary" {
		t.Fatalf("unexpected extracted document: %q", document)
	}
}

func TestRequiredBinaryAssetsIncludeLightpandaFallback(t *testing.T) {
	state := &setupFlowState{boardBinDir: "/tmp/internkim-board-bin"}
	assets := state.requiredBinaryAssets()

	if !containsBinaryAsset(assets, "lightpanda", browserruntime.DeviceBrowserExecutablePath) {
		t.Fatalf("expected setup to install Lightpanda fallback binary, got %+v", assets)
	}
}

func TestRequiredBinaryAssetsIncludePocketBase(t *testing.T) {
	state := &setupFlowState{boardBinDir: "/tmp/internkim-board-bin"}
	assets := state.requiredBinaryAssets()

	if !containsBinaryAsset(assets, "pocketbase", "/usr/local/bin/pocketbase") {
		t.Fatalf("expected setup to install PocketBase binary, got %+v", assets)
	}
}

func TestDeviceBrowserVersionCommandUsesLightpandaSubcommand(t *testing.T) {
	command := deviceBrowserVersionShellCommand(browserruntime.DeviceBrowserExecutablePath)
	if !strings.Contains(command, quoteShellValue(browserruntime.DeviceBrowserExecutablePath)+" version") {
		t.Fatalf("expected Lightpanda version subcommand, got %s", command)
	}
	if strings.Contains(command, "--version") {
		t.Fatalf("device browser version command must not use --version, got %s", command)
	}
}

func containsBinaryAsset(assets []localBinaryAsset, name string, remotePath string) bool {
	for _, asset := range assets {
		if asset.name == name && asset.remotePath == remotePath {
			return true
		}
	}
	return false
}

func TestDefaultJetsonSetupSkipsGoogle(t *testing.T) {
	selector := applySetupBoardDefaults(setup.BoardJetsonOrinNano, false, setup.Selector{})
	if !containsName(selector.Skip, "google") {
		t.Fatalf("expected default Jetson setup to skip Google, got %+v", selector.Skip)
	}
}

func TestJetsonSetupWithGoogleIncludesGoogle(t *testing.T) {
	selector := applySetupBoardDefaults(setup.BoardJetsonOrinNano, true, setup.Selector{})
	if containsName(selector.Skip, "google") {
		t.Fatalf("expected --with-google to avoid Google skip, got %+v", selector.Skip)
	}
}

func TestJetsonOnlyGoogleStillWorks(t *testing.T) {
	selector := applySetupBoardDefaults(setup.BoardJetsonOrinNano, false, setup.Selector{Only: []string{"google"}})
	if containsName(selector.Skip, "google") {
		t.Fatalf("expected --only google to avoid Google skip, got %+v", selector.Skip)
	}
}

func TestJetsonSetupDefaultsSSHCredentials(t *testing.T) {
	username, password := resolveSetupSSHCredentials(setup.BoardJetsonOrinNano, "", "")
	if username != jetsonDefaultUser {
		t.Fatalf("expected default Jetson user %q, got %q", jetsonDefaultUser, username)
	}
	if password != jetsonDefaultPassword {
		t.Fatalf("expected default Jetson password %q, got %q", jetsonDefaultPassword, password)
	}
}

func TestNonJetsonSetupKeepsLegacySSHUser(t *testing.T) {
	username, password := resolveSetupSSHCredentials("rpi", "", "")
	if username != boardUser {
		t.Fatalf("expected legacy board user %q, got %q", boardUser, username)
	}
	if password != "" {
		t.Fatalf("expected empty legacy board password, got %q", password)
	}
}

func TestSSHPrivilegedCommandSuppressesSudoPrompt(t *testing.T) {
	client := newSSH("sshpass", "internkim", "blueclaw", "192.0.2.1")
	command := client.privilegedCommand("echo ok")
	if !strings.Contains(command, "sudo -S -p '' bash -lc") {
		t.Fatalf("expected sudo command to suppress password prompt, got %s", command)
	}
}

func TestCloudflareSSHUsesAccessProxyCommand(t *testing.T) {
	client := newCloudflareSSH("sshpass", "internkim", "blueclaw", "ssh.device.example.test")
	sshArguments := strings.Join(client.sshArgs("internkim@ssh.device.example.test", "true"), "\n")
	scpArguments := strings.Join(client.scpArgs("local", "internkim@ssh.device.example.test:/tmp/file"), "\n")
	rsyncCommand := client.rsyncSSHCommand("ssh")

	for _, value := range []string{sshArguments, scpArguments, rsyncCommand} {
		if !strings.Contains(value, "ProxyCommand=env GODEBUG=netdns=go TUNNEL_EDGE_IP_VERSION=4 cloudflared --edge-ip-version 4 --edge-bind-address 0.0.0.0 access ssh --hostname %h") {
			t.Fatalf("expected Cloudflare Access ProxyCommand, got %s", value)
		}
	}
}

func TestCloudflareSSHHostnameFromDeviceURL(t *testing.T) {
	hostname := cloudflareSSHHostnameFromDeviceURL("https://device-1.intern.kim/admin")
	if hostname != "ssh-device-1.intern.kim" {
		t.Fatalf("expected SSH hostname from device URL, got %q", hostname)
	}
}

func TestRsyncSparseArgumentsVerifyAppendedData(t *testing.T) {
	arguments := strings.Join(rsyncSparseArguments("ssh", "rootfs.ext4", "host:/tmp/rootfs.ext4"), "\n")
	if !strings.Contains(arguments, "--append-verify") {
		t.Fatalf("expected resumable sparse rsync to verify appended data, got %s", arguments)
	}
	if strings.Contains(arguments, "\n--append\n") {
		t.Fatalf("expected resumable sparse rsync not to use unchecked append, got %s", arguments)
	}
}

func TestRetryableSSHFailureIncludesNetworkRouteFailure(t *testing.T) {
	output := "dial tcp [2606:4700:3031::ac43:d168]:443: connect: no route to host"
	if !isRetryableSSHFailure(output) {
		t.Fatalf("expected network route failure to be retryable")
	}
}

func TestResolveCloudflareSSHHostnameIgnoresLegacyNestedHostname(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	stateDirectory := setupStateDir(filepath.Join(homeDirectory, ".internkim"), setup.BoardJetsonOrinNano)
	saveState(stateDirectory, "ssh_hostname", "ssh.device-1.intern.kim")
	saveState(stateDirectory, "device_id", "device-1")
	target := commandTarget{
		stateDir:     stateDirectory,
		sshHostname:  "ssh.device-1.intern.kim",
		useRemoteSSH: true,
	}

	hostname := resolveCloudflareSSHHostname(config{CFDomain: "intern.kim"}, target)

	if hostname != "ssh-device-1.intern.kim" {
		t.Fatalf("expected flat SSH hostname, got %q", hostname)
	}
}

func TestCommandRemoteArgumentsStartAfterSeparator(t *testing.T) {
	arguments := commandRemoteArguments([]string{"--host", "192.0.2.1", "--", "uptime", "-p"})
	if strings.Join(arguments, " ") != "uptime -p" {
		t.Fatalf("expected remote command arguments, got %+v", arguments)
	}
}

func TestCommandControlArgumentsStopAtSeparator(t *testing.T) {
	arguments := commandControlArguments([]string{"--host", "192.0.2.1", "--", "--not-a-control-flag"})
	if strings.Join(arguments, " ") != "--host 192.0.2.1" {
		t.Fatalf("expected control arguments only, got %+v", arguments)
	}
}

func TestMattermostSetupConnectCommandPayloadTargetsAdmind(t *testing.T) {
	payload := mattermostSetupConnectCommandPayload("team-1", "command-1")
	if payload.Trigger != "connect" || payload.Method != "P" || !payload.Autocomplete {
		t.Fatalf("unexpected /connect command payload: %+v", payload)
	}
	if payload.URL != "http://127.0.0.1:18080/_internkim/mattermost/commands" {
		t.Fatalf("unexpected /connect command url: %s", payload.URL)
	}
}

func TestMattermostSetupAllowsLocalSlashCommandCallback(t *testing.T) {
	configurationPatch := mattermostSetupConfigurationPatch()
	serviceSettings, ok := configurationPatch["ServiceSettings"].(map[string]any)
	if !ok {
		t.Fatalf("expected ServiceSettings in Mattermost setup patch, got %+v", configurationPatch)
	}
	allowedConnections, ok := serviceSettings["AllowedUntrustedInternalConnections"].(string)
	if !ok || !strings.Contains(allowedConnections, "127.0.0.1") {
		t.Fatalf("expected Mattermost setup to allow local slash command callback, got %+v", serviceSettings)
	}
}

func TestFindMattermostSetupConnectCommand(t *testing.T) {
	mmAPI := func(method string, path string, body []byte, token string) (int, []byte) {
		if method != "GET" || path != "/api/v4/commands?team_id=team-1" || token != "admin-token" {
			t.Fatalf("unexpected Mattermost API call: %s %s token=%s", method, path, token)
		}
		return 200, []byte(`[{"id":"command-1","team_id":"team-1","trigger":"connect"},{"id":"other","team_id":"team-1","trigger":"deploy"}]`)
	}

	commandRecord, found := findMattermostSetupConnectCommand(mmAPI, "admin-token", "team-1")
	if !found || commandRecord.ID != "command-1" {
		t.Fatalf("expected /connect command, got found=%v record=%+v", found, commandRecord)
	}
}

func TestLocalLLMBuildArgumentsUseCurrentSSHTarget(t *testing.T) {
	arguments := localLLMBuildArguments(newSSH("sshpass", "internkim", "ssh-password", "172.30.1.46"))
	expectedArguments := []string{"--host", "172.30.1.46", "--user", "internkim"}
	if strings.Join(arguments, "\n") != strings.Join(expectedArguments, "\n") {
		t.Fatalf("expected build arguments %+v, got %+v", expectedArguments, arguments)
	}
}

func TestLocalLLMBuildArgumentsOmitEmptyPassword(t *testing.T) {
	arguments := localLLMBuildArguments(newSSH("sshpass", "root", "", "172.30.1.46"))
	if strings.Contains(strings.Join(arguments, "\n"), "--password") {
		t.Fatalf("expected empty password to be omitted, got %+v", arguments)
	}
}

func TestLocalLLMBuildEnvironmentPassesPasswordOutsideArguments(t *testing.T) {
	environment := localLLMBuildEnvironment([]string{"PATH=/usr/bin"}, newSSH("sshpass", "internkim", "ssh-password", "172.30.1.46"))
	joinedEnvironment := strings.Join(environment, "\n")
	for _, expectedValue := range []string{"LLAMA_CPP_BUILD_PASSWORD=ssh-password", "LITERT_LM_BUILD_PASSWORD=ssh-password"} {
		if !strings.Contains(joinedEnvironment, expectedValue) {
			t.Fatalf("expected environment to include %s, got %+v", expectedValue, environment)
		}
	}
}

func TestSimulationBinariesSkipLocalLLMInstall(t *testing.T) {
	context := &setup.Context{BoardType: setup.BoardSimulation}

	if shouldInstallLocalLLMSSH(context) {
		t.Fatal("expected simulation binaries to skip local LLM install")
	}
}

func TestJetsonBinariesInstallLocalLLM(t *testing.T) {
	context := &setup.Context{BoardType: setup.BoardJetsonOrinNano}

	if !shouldInstallLocalLLMSSH(context) {
		t.Fatal("expected Jetson binaries to install local LLM")
	}
}

func TestPruneLocalLLMBuildCachesKeepsCurrentCache(t *testing.T) {
	temporaryDirectory := t.TempDir()
	cacheRoot := filepath.Join(temporaryDirectory, ".dependency", "llama-cpp")
	for _, cacheName := range []string{"old-aarch64", "b8995-aarch64", "models"} {
		if errorValue := os.MkdirAll(filepath.Join(cacheRoot, cacheName), 0o755); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	state := &setupFlowState{scriptDir: temporaryDirectory}
	if errorValue := state.pruneLocalLLMBuildCaches(".dependency/llama-cpp", "b8995-aarch64"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := os.Stat(filepath.Join(cacheRoot, "old-aarch64")); !os.IsNotExist(errorValue) {
		t.Fatalf("expected old cache to be removed, got %v", errorValue)
	}
	for _, cacheName := range []string{"b8995-aarch64", "models"} {
		if _, errorValue := os.Stat(filepath.Join(cacheRoot, cacheName)); errorValue != nil {
			t.Fatalf("expected %s to remain: %v", cacheName, errorValue)
		}
	}
}

func TestSetupStateDirSeparatesJetsonAndLabIdentity(t *testing.T) {
	baseStateDir := t.TempDir()
	saveState(baseStateDir, "device_id", "shared-device")
	saveState(baseStateDir, "board_ip", "192.168.0.248")
	saveState(baseStateDir, "subnet", "192.168.0")

	jetsonStateDir := setupStateDir(baseStateDir, setup.BoardJetsonOrinNano)
	labStateDir := setupStateDir(baseStateDir, "lab")

	if jetsonStateDir == labStateDir {
		t.Fatalf("expected Jetson and lab state directories to differ")
	}
	if loadState(jetsonStateDir, "device_id") != "" {
		t.Fatalf("expected Jetson state not to inherit shared device_id")
	}
	if loadState(labStateDir, "device_id") != "" {
		t.Fatalf("expected lab state not to inherit shared device_id")
	}
	if loadState(jetsonStateDir, "board_ip") != "" {
		t.Fatalf("expected Jetson state not to inherit shared board_ip")
	}
	if loadState(labStateDir, "board_ip") != "" {
		t.Fatalf("expected lab state not to inherit shared board_ip")
	}
	if loadState(jetsonStateDir, "subnet") != "192.168.0" {
		t.Fatalf("expected Jetson state to inherit network subnet hint")
	}
}

func TestWiFiProfilesMigrateLegacyAndAddCurrent(t *testing.T) {
	stateDirectory := t.TempDir()
	saveState(stateDirectory, "wifi_ssid", "OfficeWiFi")
	saveState(stateDirectory, "wifi_pass", "office-secret")
	getSSIDPath := createExecutableFixture(t, "StudioWiFi\n")
	withArguments(t, "internkim", "setup", "--yes", "--wifi-password", "studio-secret")

	profiles, errorValue := resolveWiFiProfiles(newMsg("en"), stateDirectory, getSSIDPath)
	if errorValue != nil {
		t.Fatalf("expected Wi-Fi profiles: %v", errorValue)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected two Wi-Fi profiles, got %+v", profiles)
	}
	if profiles[0].SSID != "OfficeWiFi" || profiles[1].SSID != "StudioWiFi" {
		t.Fatalf("expected sorted preserved profiles, got %+v", profiles)
	}
	document, errorValue := os.ReadFile(filepath.Join(stateDirectory, wifiProfilesStateFile))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, ssid := range []string{"OfficeWiFi", "StudioWiFi"} {
		if !strings.Contains(string(document), ssid) {
			t.Fatalf("expected persisted Wi-Fi profiles to include %s, got:\n%s", ssid, string(document))
		}
	}
}

func TestJetsonWiFiUpsertScriptPreservesOtherProfiles(t *testing.T) {
	script := buildJetsonWiFiUpsertScript([]resolvedWiFiProfile{
		{SSID: "OfficeWiFi", Password: "office-secret"},
		{SSID: "CafeWiFi", IsOpen: true},
	})
	for _, ssid := range []string{"OfficeWiFi", "CafeWiFi"} {
		if !strings.Contains(script, jetsonWiFiConnectionID(ssid)) {
			t.Fatalf("expected upsert script to include %s, got:\n%s", ssid, script)
		}
	}
	if strings.Contains(script, "connection delete internkim-wifi ") {
		t.Fatalf("upsert script must not delete the legacy umbrella connection name, got:\n%s", script)
	}
	if strings.Contains(script, "wifi-sec.psk ''") {
		t.Fatalf("open Wi-Fi profile must not write an empty PSK, got:\n%s", script)
	}
}

func TestJetsonWiFiApplyCommandStartsSelectorInBackground(t *testing.T) {
	script := buildJetsonWiFiApplyCommand([]resolvedWiFiProfile{
		{SSID: "OfficeWiFi", Password: "office-secret"},
	}, true, false)

	for _, fragment := range []string{
		"nmcli radio wifi on",
		jetsonWiFiConnectionID("OfficeWiFi"),
		"cat > /usr/local/bin/internkim-wifi-select",
		"cat > /etc/systemd/system/internkim-wifi-recovery.service",
		"cat > /etc/systemd/system/internkim-wifi-recovery.timer",
		"systemctl daemon-reload",
		"systemctl enable --now internkim-wifi-recovery.timer",
		"nohup /usr/local/bin/internkim-wifi-select",
		"echo installed",
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected apply command to include %q, got:\n%s", fragment, script)
		}
	}
}

func TestJetsonWiFiApplyCommandCanWaitForWirelessAddress(t *testing.T) {
	script := buildJetsonWiFiApplyCommand([]resolvedWiFiProfile{
		{SSID: "OfficeWiFi", Password: "office-secret"},
	}, true, true)

	for _, fragment := range []string{
		"ip -o -4 addr show scope global",
		"$2 ~ /^(wl|wlan)/",
		"tail -20 /tmp/internkim-wifi-select.log",
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected wait command to include %q, got:\n%s", fragment, script)
		}
	}
}

func TestJetsonWiFiCommandHostCandidatesPreferUSB(t *testing.T) {
	stateDirectory := t.TempDir()
	saveState(stateDirectory, "board_ip", "192.168.0.20")

	candidates := jetsonWiFiCommandHostCandidates(stateDirectory)
	if len(candidates) == 0 || candidates[0] != jetsonUSBHostAddress {
		t.Fatalf("expected USB host first, got %+v", candidates)
	}
}
