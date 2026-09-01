package cli

import (
	"archive/zip"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gitlab.com/eastriver/internkim/internal/deviceassets"
	"testing"
	"time"

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

func TestBlueclawPayloadUpdateHTTPClientWaitsForApply(t *testing.T) {
	if blueclawUpdateHTTPClient.Timeout < 2*time.Minute {
		t.Fatalf("expected Blueclaw update client to allow payload apply latency, got %s", blueclawUpdateHTTPClient.Timeout)
	}
	if releaseUpdateHTTPClient.Timeout < 2*time.Minute {
		t.Fatalf("expected release update client to allow payload apply latency, got %s", releaseUpdateHTTPClient.Timeout)
	}
	if statusHTTPClient.Timeout >= blueclawUpdateHTTPClient.Timeout {
		t.Fatalf("expected general status client to stay shorter than payload update client")
	}
}

func TestDeviceToolPackagesIncludeSitePublishingBasics(t *testing.T) {
	packages := strings.Join(baseDeviceToolPackages(), " ")
	for _, packageName := range []string{"bc", "git", "curl", "unzip", "ca-certificates", "iproute2", "iptables", "procps"} {
		if !strings.Contains(packages, packageName) {
			t.Fatalf("expected base device tools to include %q, got %s", packageName, packages)
		}
	}
}

func TestSkillDependencySetupOnlyVerifiesRuntimeBaseEnvironment(t *testing.T) {
	command := installSkillPythonDependenciesCommand()
	for _, forbiddenText := range []string{"uv venv", "uv pip install", "curl -LsSf", "UV_UNMANAGED_INSTALL"} {
		if strings.Contains(command, forbiddenText) {
			t.Fatalf("skills setup must not install dependencies during deployment: found %q in\n%s", forbiddenText, command)
		}
	}
	for _, expectedText := range []string{"/opt/internkim/blueclaw-runtime/rootfs.ext4", "rootfs-builtin-skills-python-missing", `if [ "$contract_output" != "ok" ]`} {
		if !strings.Contains(command, expectedText) {
			t.Fatalf("skills setup must verify %q, got:\n%s", expectedText, command)
		}
	}
}

func TestManagedHostExecutablesScriptInstallsCanonicalRuntimeTools(t *testing.T) {
	script := managedHostExecutablesScript()
	for _, expectedText := range []string{
		"/opt/internkim/managed-bin/bun",
		"/usr/local/bin/bun",
		"/usr/local/bin/bunx",
		"/usr/local/bin/marp",
		"apt-get install -y --no-install-recommends unzip",
		"https://github.com/oven-sh/bun/releases/download/bun-v1.3.10/bun-linux-aarch64.zip",
		"sha256sum -c -",
		"@marp-team/marp-cli",
		"UV_UNMANAGED_INSTALL=/usr/local/bin",
		"host-$managed_executable-owner-drift",
		"host-$managed_executable-mode-drift",
	} {
		if !strings.Contains(script, expectedText) {
			t.Fatalf("expected managed host executable script to include %q, got:\n%s", expectedText, script)
		}
	}
	if strings.Contains(script, "sudo -u blueclaw") {
		t.Fatalf("managed host executable script must not depend on sudo account validation, got:\n%s", script)
	}
}

func TestSetupContextSkipsStepParsesSkipSelection(t *testing.T) {
	context := &setup.Context{SetupSteps: "--skip=wifi,local-llm,google,slack"}
	if !setupContextSkipsStep(context, "local-llm") {
		t.Fatal("expected local-llm to be skipped")
	}
	if setupContextSkipsStep(context, "web") {
		t.Fatal("expected web not to be skipped")
	}
}

func TestParseGitWorktreePaths(t *testing.T) {
	output := "worktree /repo/main\nHEAD abc123\nbranch refs/heads/main\n\nworktree /repo/wt1\nHEAD def456\ndetached\n"
	paths := parseGitWorktreePaths(output)
	if strings.Join(paths, ",") != "/repo/main,/repo/wt1" {
		t.Fatalf("unexpected worktree paths: %v", paths)
	}
}

func TestBlueclawDeliveredManifestIsReadFromTheShare(t *testing.T) {
	command := blueclawWorkspaceManifestCommand()
	for _, expectedText := range []string{
		"cat ",
		"/var/lib/blueclaw/delivery/runtime/current/manifest.json",
	} {
		if !strings.Contains(command, expectedText) {
			t.Fatalf("the payload manifest is on the share now, expected %q, got:\n%s", expectedText, command)
		}
	}
}

func TestBlueclawStopForPayloadSyncWaitsForGuestProcesses(t *testing.T) {
	command := blueclawStopForPayloadSyncCommand()
	for _, expectedText := range []string{
		"systemctl stop blueclaw",
		"systemctl kill blueclaw",
		"systemctl is-active --quiet blueclaw",
	} {
		if !strings.Contains(command, expectedText) {
			t.Fatalf("expected stop command to include %q, got:\n%s", expectedText, command)
		}
	}
}

func TestBlueclawStartAfterPayloadSyncWaitsForService(t *testing.T) {
	command := blueclawStartAfterPayloadSyncCommand()
	for _, expectedText := range []string{
		"systemctl cat blueclaw",
		"systemctl start blueclaw",
		"systemctl is-active --quiet blueclaw",
		"systemctl status blueclaw",
	} {
		if !strings.Contains(command, expectedText) {
			t.Fatalf("expected start command to include %q, got:\n%s", expectedText, command)
		}
	}
}

func TestBlueclawHostWorkspacePayloadSyncCommandUpdatesCanonicalWorkspaceRuntime(t *testing.T) {
	command := blueclawHostWorkspacePayloadSyncCommand("/tmp/internkim-blueclaw-payload")
	for _, expectedText := range []string{
		"/tmp/internkim-blueclaw-payload/workspace/.blueclaw/runtime/",
		"/root/.blueclaw/workspace/.blueclaw/runtime/",
		"rsync -a --delete",
		"chown -R blueclaw:blueclaw",
	} {
		if !strings.Contains(command, expectedText) {
			t.Fatalf("expected host workspace payload sync command to include %q, got:\n%s", expectedText, command)
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

func TestCreateAdminUIArchiveIncludesBoardFiles(t *testing.T) {
	sourceDirectory := t.TempDir()
	versionDirectory := filepath.Join(sourceDirectory, "_app")
	if errorValue := os.MkdirAll(versionDirectory, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(versionDirectory, "version.json"), []byte(`{"version":"test-build"}`), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(sourceDirectory, "index.html"), []byte("<html></html>"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	archivePath, errorValue := createAdminUIArchive(sourceDirectory)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer os.Remove(archivePath)

	targetDirectory := t.TempDir()
	command := exec.Command("tar", "-C", targetDirectory, "-xf", archivePath)
	if output, errorValue := command.CombinedOutput(); errorValue != nil {
		t.Fatalf("extract admin UI archive: %s: %v", strings.TrimSpace(string(output)), errorValue)
	}
	document, errorValue := os.ReadFile(filepath.Join(targetDirectory, "_app", "version.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(document) != `{"version":"test-build"}` {
		t.Fatalf("unexpected deployed version: %s", document)
	}
}

func TestTheVersionInAStampIsTrimmed(t *testing.T) {
	boardUIPath := t.TempDir()
	versionDirectory := filepath.Join(boardUIPath, "_app")
	if errorValue := os.MkdirAll(versionDirectory, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(versionDirectory, "version.json"), []byte("{\"version\": \" version-1 \"}\n"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	version, errorValue := readAdminUIVersion(boardUIPath)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if version != "version-1" {
		t.Fatalf("expected trimmed version, got %q", version)
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

func TestCloudSharedSetupSkipsHardwareAndOptionalProviderSteps(t *testing.T) {
	selector := applySetupBoardDefaults(setup.BoardCloudShared, false, setup.Selector{})
	for _, expectedName := range []string{"wifi", "local-llm", "google"} {
		if !containsName(selector.Skip, expectedName) {
			t.Fatalf("expected cloud-shared setup to skip %s, got %+v", expectedName, selector.Skip)
		}
	}
}

func TestCloudSharedOnlyLocalLLMCanExplicitlySelectLocalLLM(t *testing.T) {
	selector := applySetupBoardDefaults(setup.BoardCloudShared, false, setup.Selector{Only: []string{"local-llm"}})
	if containsName(selector.Skip, "local-llm") {
		t.Fatalf("expected --only local-llm to avoid local-llm skip, got %+v", selector.Skip)
	}
	if !containsName(selector.Skip, "wifi") {
		t.Fatalf("expected cloud-shared setup to keep skipping wifi, got %+v", selector.Skip)
	}
}

func TestJetsonSetupDefaultsSSHCredentials(t *testing.T) {
	username, password := resolveSetupSSHCredentials(setup.BoardJetsonOrinNano, "", "")
	if username != jetsonDefaultUser {
		t.Fatalf("expected default Jetson user %q, got %q", jetsonDefaultUser, username)
	}
	if password != "" {
		t.Fatalf("expected empty Jetson password when INTERNKIM_CONSOLE_PASSWORD unset, got %q", password)
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

func TestUploadMoveCommandIsNotPreWrappedWithSudo(t *testing.T) {
	command := moveUploadedPathCommand("/tmp/internkim-upload-file", "/etc/systemd/system/file.service")
	if strings.Contains(command, "sudo") {
		t.Fatalf("expected upload move command to be wrapped only by runResult, got %s", command)
	}
	for _, expectedText := range []string{
		"mkdir -p '/etc/systemd/system'",
		"mv '/tmp/internkim-upload-file' '/etc/systemd/system/file.service'",
	} {
		if !strings.Contains(command, expectedText) {
			t.Fatalf("expected upload move command to include %q, got %s", expectedText, command)
		}
	}
}

func TestRemoteSSHNamesNoTransport(t *testing.T) {
	client := newSSH("sshpass", "internkim", "blueclaw", "ssh.device.example.test")
	sshArguments := strings.Join(client.sshArgs("internkim@ssh.device.example.test", "true"), "\n")
	scpArguments := strings.Join(client.scpArgs("local", "internkim@ssh.device.example.test:/tmp/file"), "\n")
	rsyncCommand := client.rsyncSSHCommand("ssh")

	for _, value := range []string{sshArguments, scpArguments, rsyncCommand} {
		for _, forbidden := range []string{"ProxyCommand", "cloudflared", "TUNNEL_EDGE_IP_VERSION"} {
			if strings.Contains(value, forbidden) {
				t.Fatalf("ssh must name no transport, found %q in %s", forbidden, value)
			}
		}
	}
	for _, value := range []string{sshArguments, scpArguments} {
		if !strings.Contains(value, "ssh.device.example.test") {
			t.Fatalf("expected the hostname to reach ssh, got %s", value)
		}
	}
}

func TestBlueclawPayloadDirectOnlySetupUsesHTTPMaintenancePath(t *testing.T) {
	if !isBlueclawPayloadDirectOnlySetup([]string{"--only", "blueclaw-payload-direct"}) {
		t.Fatalf("expected blueclaw-payload-direct only setup to use HTTP maintenance path")
	}
	if isBlueclawPayloadDirectOnlySetup([]string{"--only", "blueclaw-payload-direct,capabilityd"}) {
		t.Fatalf("expected mixed setup slices to keep normal backend selection")
	}
	if isBlueclawPayloadDirectOnlySetup([]string{"--only", "capabilityd"}) {
		t.Fatalf("expected capabilityd setup to keep normal backend selection")
	}
}

func TestRsyncSparseArgumentsAvoidUncheckedAppend(t *testing.T) {
	arguments := strings.Join(rsyncSparseArguments("ssh", "rootfs.ext4", "host:/tmp/rootfs.ext4"), "\n")
	if strings.Contains(arguments, "\n--append\n") || strings.Contains(arguments, "\n--append-verify\n") {
		t.Fatalf("expected resumable sparse rsync not to use unchecked append, got %s", arguments)
	}
	if !strings.Contains(arguments, "--partial") {
		t.Fatalf("expected resumable sparse rsync to keep partial files, got %s", arguments)
	}
}

func TestRetryableSSHFailureIncludesNetworkRouteFailure(t *testing.T) {
	output := "dial tcp [2606:4700:3031::ac43:d168]:443: connect: no route to host"
	if !isRetryableSSHFailure(output) {
		t.Fatalf("expected network route failure to be retryable")
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

func TestSimulationHasNoLocalLLMPlanned(t *testing.T) {
	context := &setup.Context{BoardType: setup.BoardSimulation}

	if localLLMIsPlanned(context) {
		t.Fatal("expected simulation to plan no local LLM")
	}
}

func TestLocalLLMIsNotPlannedWhenItsStepIsNot(t *testing.T) {
	context := &setup.Context{
		BoardType:    setup.BoardJetsonOrinNano,
		PlannedSteps: map[string]bool{"binaries": true},
	}

	if localLLMIsPlanned(context) {
		t.Fatal("expected no local LLM when the local-llm step is not planned")
	}
}

func TestLocalLLMIsPlannedWhenItsStepIs(t *testing.T) {
	context := &setup.Context{
		BoardType:    setup.BoardJetsonOrinNano,
		PlannedSteps: map[string]bool{"local-llm": true},
	}

	if !localLLMIsPlanned(context) {
		t.Fatal("expected local LLM when the local-llm step is planned")
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

func TestBinaryVersionSourcesIncludeSharedLLMBackend(t *testing.T) {
	repositoryRoot, errorValue := filepath.Abs("../..")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	state := &setupFlowState{scriptDir: repositoryRoot}
	sourcePaths := state.binaryVersionSourcePaths()

	if !pathsContain(sourcePaths, filepath.Join(repositoryRoot, "internal", "llmbackend")) {
		t.Fatalf("expected binary version sources to include internal/llmbackend, got %+v", sourcePaths)
	}
}

func TestSetupStateDirSeparatesJetsonAndLabIdentity(t *testing.T) {
	baseStateDir := t.TempDir()
	saveState(baseStateDir, "fleet_id", "shared-device")
	saveState(baseStateDir, "board_ip", "192.168.0.248")
	saveState(baseStateDir, "subnet", "192.168.0")

	jetsonStateDir := setupStateDir(baseStateDir, setup.BoardJetsonOrinNano)
	labStateDir := setupStateDir(baseStateDir, "lab")

	if jetsonStateDir == labStateDir {
		t.Fatalf("expected Jetson and lab state directories to differ")
	}
	if loadState(jetsonStateDir, "fleet_id") != "" {
		t.Fatalf("expected Jetson state not to inherit shared fleet_id")
	}
	if loadState(labStateDir, "fleet_id") != "" {
		t.Fatalf("expected lab state not to inherit shared fleet_id")
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

func pathsContain(paths []string, targetPath string) bool {
	cleanTargetPath := filepath.Clean(targetPath)
	for _, path := range paths {
		if filepath.Clean(path) == cleanTargetPath {
			return true
		}
	}
	return false
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
		"cat > /usr/local/lib/internkim/network-snapshot.sh",
		"cat > /etc/systemd/system/internkim-network-snapshot.timer",
		"Storage=persistent",
		"internkim-ethernet",
		"ipv4.route-metric 100",
		"systemctl daemon-reload",
		"systemctl enable --now internkim-wifi-recovery.timer internkim-network-snapshot.timer",
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

func TestSavedRemoteSSHHostnameTakesWhatWasSavedAndInventsNothing(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	stateDirectory := setupStateDir(filepath.Join(homeDirectory, ".internkim"), setup.BoardJetsonOrinNano)
	saveState(stateDirectory, "fleet_id", "device-1")

	if hostname := savedRemoteSSHHostname(commandTarget{stateDir: stateDirectory}); hostname != "" {
		t.Fatalf("a fleet id is not a hostname; nothing should be derived from it, got %q", hostname)
	}

	saveState(stateDirectory, "ssh_hostname", "whatever.the.operator.set")
	if hostname := savedRemoteSSHHostname(commandTarget{stateDir: stateDirectory}); hostname != "whatever.the.operator.set" {
		t.Fatalf("expected the saved hostname, got %q", hostname)
	}
}

func TestSavedRemoteSSHHostnamePrefersAnExplicitHost(t *testing.T) {
	target := commandTarget{host: "192.0.2.10", useRemoteSSH: true, sshHostname: "saved.example.test"}

	if hostname := savedRemoteSSHHostname(target); hostname != "192.0.2.10" {
		t.Fatalf("expected the explicit host, got %q", hostname)
	}
}

func TestDeviceSSHControlFlagsSeparateElevationFromTargeting(t *testing.T) {
	arguments := []string{"--host", "192.168.0.8", "--sudo"}
	if !hasControlFlag(arguments, "--sudo") {
		t.Fatal("--sudo must be recognized as a control flag")
	}
	remaining := withoutControlFlag(arguments, "--sudo")
	for _, argument := range remaining {
		if argument == "--sudo" {
			t.Fatal("--sudo must not reach the target resolver, which rejects flags it does not define")
		}
	}
	if len(remaining) != 2 || remaining[0] != "--host" || remaining[1] != "192.168.0.8" {
		t.Fatalf("targeting flags must survive, got %v", remaining)
	}
}

// The managed host executables are built against a requirements file that
// arrives as a host device asset. While that asset shipped with the skills,
// four steps later, a device set up from scratch died here every time.
func TestManagedHostExecutablesNeedAHostAssetTheBinariesStepInstalls(t *testing.T) {
	script := managedHostExecutablesScript()
	requirementsPath := deviceassets.DocumentConversionDevicePath + "/requirements.txt"

	if !strings.Contains(script, requirementsPath) {
		t.Fatalf("expected the script to read %q, got:\n%s", requirementsPath, script)
	}
	asset, isFound := deviceassets.Find("document-conversion")
	if !isFound || asset.DeviceKind != deviceassets.DeviceKindHost {
		t.Fatalf("expected document-conversion to be a host asset, got %+v", asset)
	}
}
