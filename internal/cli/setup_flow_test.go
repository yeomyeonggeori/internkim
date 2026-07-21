package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
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
	context := &setup.Context{SetupSteps: "--skip=wifi,local-llm,cloudflare-access,tunnel,google,slack"}
	if !setupContextSkipsStep(context, "tunnel") {
		t.Fatal("expected tunnel to be skipped")
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

func TestBlueclawWorkspaceManifestCommandReadsPayloadManifestInsideImage(t *testing.T) {
	command := blueclawWorkspaceManifestCommand()
	for _, expectedText := range []string{
		"debugfs -R",
		"cat /.blueclaw/runtime/current/manifest.json",
		"/var/lib/blueclaw/workspace.ext4",
	} {
		if !strings.Contains(command, expectedText) {
			t.Fatalf("expected workspace manifest command to include %q, got:\n%s", expectedText, command)
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

func TestReadAdminUIVersionTrimsWhitespace(t *testing.T) {
	boardUIPath := t.TempDir()
	versionDirectory := filepath.Join(boardUIPath, "_app")
	if errorValue := os.MkdirAll(versionDirectory, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(versionDirectory, "version.json"), []byte(" version-1 \n"), 0o644); errorValue != nil {
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

func TestCloudflareSSHUsesAccessProxyCommand(t *testing.T) {
	client := newCloudflareSSH("sshpass", "internkim", "blueclaw", "ssh.device.example.test")
	sshArguments := strings.Join(client.sshArgs("internkim@ssh.device.example.test", "true"), "\n")
	scpArguments := strings.Join(client.scpArgs("local", "internkim@ssh.device.example.test:/tmp/file"), "\n")
	rsyncCommand := client.rsyncSSHCommand("ssh")

	for _, value := range []string{sshArguments, scpArguments, rsyncCommand} {
		if !strings.Contains(value, "ProxyCommand=env GODEBUG=netdns=go TUNNEL_EDGE_IP_VERSION=4 cloudflared --edge-ip-version 4 --edge-bind-address 0.0.0.0 access ssh") {
			t.Fatalf("expected Cloudflare Access ProxyCommand, got %s", value)
		}
		if !strings.Contains(value, "--hostname %h") {
			t.Fatalf("expected hostname placeholder in ProxyCommand, got %s", value)
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

func TestCloudflareSSHHostnameFromDeviceURL(t *testing.T) {
	hostname := cloudflareSSHHostnameFromDeviceURL("https://device-1.intern.kim/admin")
	if hostname != "ssh-device-1.intern.kim" {
		t.Fatalf("expected SSH hostname from device URL, got %q", hostname)
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

func TestResolveCloudflareSSHHostnameIgnoresLegacyNestedHostname(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	stateDirectory := setupStateDir(filepath.Join(homeDirectory, ".internkim"), setup.BoardJetsonOrinNano)
	saveState(stateDirectory, "ssh_hostname", "ssh.device-1.intern.kim")
	saveState(stateDirectory, "fleet_id", "device-1")
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

func TestResolveCloudflareSSHHostnameIgnoresLegacyNodeHostname(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	stateDirectory := setupStateDir(filepath.Join(homeDirectory, ".internkim"), setup.BoardJetsonOrinNano)
	saveState(stateDirectory, "ssh_hostname", "ssh-1.device-1.intern.kim")
	saveState(stateDirectory, "fleet_id", "device-1")
	saveState(stateDirectory, "node_id", "1")
	target := commandTarget{
		stateDir:    stateDirectory,
		nodeID:      "1",
		sshHostname: "ssh-1.device-1.intern.kim",
	}

	hostname := resolveCloudflareSSHHostname(config{CFDomain: "intern.kim"}, target)

	if hostname != "1.ssh.device-1.intern.kim" {
		t.Fatalf("expected node SSH hostname, got %q", hostname)
	}
}

func TestResolveCloudflareSSHHostnameIgnoresLegacyFlatNodeHostname(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	stateDirectory := setupStateDir(filepath.Join(homeDirectory, ".internkim"), setup.BoardJetsonOrinNano)
	saveState(stateDirectory, "ssh_hostname", "ssh-1-device-1.intern.kim")
	saveState(stateDirectory, "fleet_id", "device-1")
	saveState(stateDirectory, "node_id", "1")
	target := commandTarget{
		stateDir:    stateDirectory,
		nodeID:      "1",
		sshHostname: "ssh-1-device-1.intern.kim",
	}

	hostname := resolveCloudflareSSHHostname(config{CFDomain: "intern.kim"}, target)

	if hostname != "1.ssh.device-1.intern.kim" {
		t.Fatalf("expected node SSH hostname, got %q", hostname)
	}
}

func TestCloudflareSSHRegistrationCacheIsReusableWhenSetupIsForced(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	stateDirectory := setupStateDir(filepath.Join(homeDirectory, ".internkim"), setup.BoardJetsonOrinNano)
	saveState(stateDirectory, "node_tunnel_token", "node-token")
	saveState(stateDirectory, "tunnel_revision", setup.TunnelConfigurationRevision)
	savedSSHHostname := "1.ssh.device-1.intern.kim"

	if !canReuseCloudflareSSHRegistration(config{CFDomain: "intern.kim"}, stateDirectory, savedSSHHostname, false) {
		t.Fatalf("expected current SSH registration cache to be reusable")
	}
	if !canReuseCloudflareSSHRegistration(config{CFDomain: "intern.kim"}, stateDirectory, savedSSHHostname, true) {
		t.Fatalf("expected forced setup to reuse valid SSH registration cache")
	}
}

func TestCloudflareSSHRegistrationCacheIsBypassedWhenTLSIsPending(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	stateDirectory := setupStateDir(filepath.Join(homeDirectory, ".internkim"), setup.BoardJetsonOrinNano)
	saveState(stateDirectory, "node_tunnel_token", "node-token")
	saveState(stateDirectory, "tunnel_revision", setup.TunnelConfigurationRevision)
	saveState(stateDirectory, "tls_certificate_status", "initializing")
	savedSSHHostname := "1.ssh.device-1.intern.kim"

	if canReuseCloudflareSSHRegistration(config{CFDomain: "intern.kim"}, stateDirectory, savedSSHHostname, false) {
		t.Fatalf("expected pending TLS state to refresh SSH registration")
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
	payload := mattermostSetupCommandPayload("team-1", "command-1", "connect")
	if payload.Trigger != "connect" || payload.Method != "P" || !payload.Autocomplete {
		t.Fatalf("unexpected /connect command payload: %+v", payload)
	}
	if payload.URL != "http://127.0.0.1:18080/_internkim/mattermost/commands" {
		t.Fatalf("unexpected /connect command url: %s", payload.URL)
	}
}

func TestMattermostSetupAllowsLocalSlashCommandCallback(t *testing.T) {
	configurationPatch := mattermostSetupConfigurationPatch("https://device.example")
	serviceSettings, ok := configurationPatch["ServiceSettings"].(map[string]any)
	if !ok {
		t.Fatalf("expected ServiceSettings in Mattermost setup patch, got %+v", configurationPatch)
	}
	allowedConnections, ok := serviceSettings["AllowedUntrustedInternalConnections"].(string)
	if !ok || !strings.Contains(allowedConnections, "127.0.0.1") {
		t.Fatalf("expected Mattermost setup to allow local slash command callback, got %+v", serviceSettings)
	}
}

func TestMattermostSetupRegistersManagedResourcePaths(t *testing.T) {
	configurationPatch := mattermostSetupConfigurationPatch("https://device.example")
	serviceSettings, ok := configurationPatch["ServiceSettings"].(map[string]any)
	if !ok {
		t.Fatalf("expected ServiceSettings in Mattermost setup patch, got %+v", configurationPatch)
	}
	resourcePaths, ok := serviceSettings["ManagedResourcePaths"].(string)
	if !ok {
		t.Fatalf("expected Mattermost managed resource paths, got %+v", serviceSettings)
	}
	if resourcePaths != mattermostdefaults.ManagedResourcePathSetting() {
		t.Fatalf("unexpected Mattermost managed resource paths: %+v", resourcePaths)
	}
}

func TestMattermostSetupEnablesUserTokensAndBots(t *testing.T) {
	configurationPatch := mattermostSetupConfigurationPatch("https://device.example")
	serviceSettings, ok := configurationPatch["ServiceSettings"].(map[string]any)
	if !ok {
		t.Fatalf("expected ServiceSettings in Mattermost setup patch, got %+v", configurationPatch)
	}
	if serviceSettings["EnableUserAccessTokens"] != true {
		t.Fatalf("expected Mattermost setup to enable user tokens, got %+v", serviceSettings)
	}
	if serviceSettings["EnableBotAccountCreation"] != true {
		t.Fatalf("expected Mattermost setup to enable bot creation, got %+v", serviceSettings)
	}
}

func TestMattermostSetupEnablesMobilePushNotifications(t *testing.T) {
	configurationPatch := mattermostSetupConfigurationPatch("https://device.example")
	emailSettings, ok := configurationPatch["EmailSettings"].(map[string]any)
	if !ok {
		t.Fatalf("expected EmailSettings in Mattermost setup patch, got %+v", configurationPatch)
	}
	if emailSettings["SendPushNotifications"] != true {
		t.Fatalf("expected Mattermost setup to enable push notifications, got %+v", emailSettings)
	}
	if emailSettings["PushNotificationServer"] != mattermostDefaultPushNotificationServer {
		t.Fatalf("expected default push notification server, got %+v", emailSettings)
	}
	if emailSettings["PushNotificationContents"] != "id_loaded" {
		t.Fatalf("expected id-only push notification contents, got %+v", emailSettings)
	}
}

func TestMattermostSetupUsesAbsoluteFileStorageDirectory(t *testing.T) {
	configurationPatch := mattermostSetupConfigurationPatch("https://device.example")
	fileSettings, ok := configurationPatch["FileSettings"].(map[string]any)
	if !ok {
		t.Fatalf("expected FileSettings in Mattermost setup patch, got %+v", configurationPatch)
	}
	if fileSettings["DriverName"] != "local" {
		t.Fatalf("expected local file storage, got %+v", fileSettings)
	}
	if fileSettings["Directory"] != mattermostPersistentFileStorageDirectory {
		t.Fatalf("expected absolute Mattermost data directory, got %+v", fileSettings)
	}
	if fileSettings["EnableFileAttachments"] != true {
		t.Fatalf("expected file attachments to stay enabled, got %+v", fileSettings)
	}
}

func TestMattermostSetupEnsuresDefaultChannels(t *testing.T) {
	var createdChannels []map[string]string
	var patchedChannels []map[string]string
	var deletedSystemPosts []string
	mattermostAPI := func(method string, path string, body []byte, token string) (int, []byte) {
		if token != "admin-token" {
			t.Fatalf("unexpected token: %s", token)
		}
		if method == "GET" && strings.HasPrefix(path, "/api/v4/teams/team-1/channels/name/") {
			return http.StatusNotFound, nil
		}
		if method == "POST" && path == "/api/v4/channels" {
			var channel map[string]string
			if errorValue := json.Unmarshal(body, &channel); errorValue != nil {
				t.Fatal(errorValue)
			}
			createdChannels = append(createdChannels, channel)
			return http.StatusCreated, []byte(`{"id":"channel-` + channel["name"] + `"}`)
		}
		if method == "PUT" && strings.HasPrefix(path, "/api/v4/channels/channel-") && strings.HasSuffix(path, "/patch") {
			var channel map[string]string
			if errorValue := json.Unmarshal(body, &channel); errorValue != nil {
				t.Fatal(errorValue)
			}
			channel["path"] = path
			patchedChannels = append(patchedChannels, channel)
			return http.StatusOK, nil
		}
		if method == "GET" && strings.HasPrefix(path, "/api/v4/channels/channel-") && strings.HasSuffix(path, "/posts?per_page=100") {
			return http.StatusOK, []byte(`{"order":["system-header","user-post"],"posts":{"system-header":{"id":"system-header","type":"system_header_change"},"user-post":{"id":"user-post","type":""}}}`)
		}
		if method == "DELETE" && strings.HasPrefix(path, "/api/v4/posts/system-header") {
			deletedSystemPosts = append(deletedSystemPosts, path)
			return http.StatusOK, nil
		}
		t.Fatalf("unexpected Mattermost API call: %s %s", method, path)
		return http.StatusInternalServerError, nil
	}

	setupMattermostDefaultChannels(mattermostAPI, "admin-token", "", "team-1", "", "ko")

	expectedChannels := mattermostdefaults.PublicChannelsForLanguage("ko")
	if len(createdChannels) != len(expectedChannels) {
		t.Fatalf("created channels = %+v", createdChannels)
	}
	if len(patchedChannels) != len(expectedChannels) {
		t.Fatalf("patched channels = %+v", patchedChannels)
	}
	if len(deletedSystemPosts) != len(expectedChannels) {
		t.Fatalf("deleted system posts = %+v", deletedSystemPosts)
	}
	for channelIndex, channel := range expectedChannels {
		if createdChannels[channelIndex]["name"] != channel.Name {
			t.Fatalf("created channel %d = %+v", channelIndex, createdChannels[channelIndex])
		}
		if patchedChannels[channelIndex]["display_name"] != channel.DisplayName {
			t.Fatalf("patched channel %d = %+v", channelIndex, patchedChannels[channelIndex])
		}
		if patchedChannels[channelIndex]["header"] != channel.Header {
			t.Fatalf("patched channel %d = %+v", channelIndex, patchedChannels[channelIndex])
		}
	}
}

func TestMattermostSetupPatchesSiteURL(t *testing.T) {
	configurationPatch := mattermostSetupConfigurationPatch("https://device.example")
	serviceSettings, ok := configurationPatch["ServiceSettings"].(map[string]any)
	if !ok {
		t.Fatalf("expected ServiceSettings in Mattermost setup patch, got %+v", configurationPatch)
	}
	siteURL, ok := serviceSettings["SiteURL"].(string)
	if !ok || siteURL != "https://device.example" {
		t.Fatalf("expected Mattermost setup to patch SiteURL, got %+v", serviceSettings)
	}
}

func TestMattermostSetupPatchesCorsForSiteURL(t *testing.T) {
	configurationPatch := mattermostSetupConfigurationPatch("https://device.example")
	serviceSettings, ok := configurationPatch["ServiceSettings"].(map[string]any)
	if !ok {
		t.Fatalf("expected ServiceSettings in Mattermost setup patch, got %+v", configurationPatch)
	}
	allowedOrigin, ok := serviceSettings["AllowCorsFrom"].(string)
	if !ok || allowedOrigin != "https://device.example" {
		t.Fatalf("expected Mattermost setup to patch CORS origin, got %+v", serviceSettings)
	}
	allowCredentials, ok := serviceSettings["CorsAllowCredentials"].(bool)
	if !ok || !allowCredentials {
		t.Fatalf("expected Mattermost setup to allow CORS credentials, got %+v", serviceSettings)
	}
}

func TestFindMattermostSetupConnectCommand(t *testing.T) {
	mmAPI := func(method string, path string, body []byte, token string) (int, []byte) {
		if method != "GET" || path != "/api/v4/commands?team_id=team-1" || token != "admin-token" {
			t.Fatalf("unexpected Mattermost API call: %s %s token=%s", method, path, token)
		}
		return 200, []byte(`[{"id":"command-1","team_id":"team-1","trigger":"connect"},{"id":"other","team_id":"team-1","trigger":"deploy"}]`)
	}

	commandRecord, found := findMattermostSetupCommand(mmAPI, "admin-token", "team-1", "connect")
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

func TestJetsonBinariesSkipLocalLLMWhenLocalLLMStepIsNotPlanned(t *testing.T) {
	context := &setup.Context{
		BoardType:    setup.BoardJetsonOrinNano,
		PlannedSteps: map[string]bool{"binaries": true},
	}

	if shouldInstallLocalLLMSSH(context) {
		t.Fatal("expected Jetson binaries to skip local LLM when local-llm is not planned")
	}
}

func TestJetsonBinariesInstallLocalLLMWhenLocalLLMStepIsPlanned(t *testing.T) {
	context := &setup.Context{
		BoardType:    setup.BoardJetsonOrinNano,
		PlannedSteps: map[string]bool{"local-llm": true},
	}

	if !shouldInstallLocalLLMSSH(context) {
		t.Fatal("expected Jetson binaries to install local LLM when local-llm is planned")
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
