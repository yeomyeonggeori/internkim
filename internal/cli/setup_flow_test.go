package cli

import (
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

func TestRequiredBinaryAssetsIncludeLightpandaFallback(t *testing.T) {
	state := &setupFlowState{boardBinDir: "/tmp/internkim-board-bin"}
	assets := state.requiredBinaryAssets()

	if !containsBinaryAsset(assets, "lightpanda", browserruntime.DeviceBrowserExecutablePath) {
		t.Fatalf("expected setup to install Lightpanda fallback binary, got %+v", assets)
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
