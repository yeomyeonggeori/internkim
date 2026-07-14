package tenantruntime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallHostRuntimeWritesTenantScopedServices(t *testing.T) {
	commandRunner := &fakeCommandRunner{}
	service := Service{
		BasePath:                   t.TempDir(),
		SystemdSystemDirectoryPath: t.TempDir(),
		CommandRunner:              commandRunner,
	}
	manifest := newTestCloudSharedManifest(t)
	manifest.TenantID = "pilot-01"
	manifest.ContainerName = "internkim-pilot-01"
	manifest.PublicURL = "https://pilot-01.example.test"
	manifest.AssignedHost = "mac-studio-a"
	manifest.MattermostInstance = MattermostInstance{
		PublicURL:    "https://pilot-01.example.test",
		InternalURL:  "http://127.0.0.1:18065",
		Port:         18065,
		DatabaseName: "mattermost_pilot_01",
	}
	if _, errorValue := service.CreateTenant(manifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	paths, errorValue := BuildRuntimePaths(service.BasePath, manifest.TenantID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := installTenantCredentials(paths, TenantCredentials{
		OpenRouterAPIKey: "device-token",
		AdminPassword:    "admin-password",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestFileWithMode(t, filepath.Join(paths.InternKimSecretsPath, "mattermost-bot-token"), "bot-token", 0o600)
	rootFilesystemImagePath := writeTestFileWithMode(t, filepath.Join(t.TempDir(), "rootfs.ext4"), "rootfs", 0o600)
	workspaceImagePath := writeTestFileWithMode(t, filepath.Join(t.TempDir(), "workspace.ext4"), "workspace", 0o600)
	graphitiPackagePath := t.TempDir()
	writeTestFileWithMode(t, filepath.Join(graphitiPackagePath, "requirements.txt"), "graphiti-core[kuzu]\n", 0o644)
	graphitiVirtualEnvPath := filepath.Join(t.TempDir(), "graphiti-venv")
	runtimeDirectoryBasePath := t.TempDir()

	status, errorValue := service.InstallHostRuntime(context.Background(), manifest.TenantID, HostRuntimeOptions{
		GatewayURL:                 "https://internkim-llm-gateway.example/api/v1/chat/completions",
		GatewaySharedSecret:        "gateway-secret",
		ReleaseDownloadToken:       "release-token",
		ModelName:                  "x-ai/grok-4.3",
		RootFilesystemTemplatePath: rootFilesystemImagePath,
		WorkspaceImageTemplatePath: workspaceImagePath,
		GraphitiPackagePath:        graphitiPackagePath,
		GraphitiVirtualEnvPath:     graphitiVirtualEnvPath,
		RuntimeDirectoryBasePath:   runtimeDirectoryBasePath,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if status.AdmindURL != "http://127.0.0.1:18180" || status.BlueclawURL != "http://127.0.0.1:18100" {
		t.Fatalf("unexpected host runtime status: %+v", status)
	}
	if !status.MattermostBotTokenReady || !status.LLMDeviceTokenReady || !status.GatewaySharedSecretReady || !status.RuntimeConfigurationReady {
		t.Fatalf("expected host runtime status to report ready files: %+v", status)
	}
	if status.GraphitiVirtualEnvPath != graphitiVirtualEnvPath {
		t.Fatalf("expected Graphiti venv path in status, got %+v", status)
	}
	assertFileContains(t, filepath.Join(service.SystemdSystemDirectoryPath, "internkim-tenant-admind-pilot-01.service"), "--listen 127.0.0.1:18180")
	assertFileContains(t, filepath.Join(service.SystemdSystemDirectoryPath, "internkim-tenant-admind-pilot-01.service"), "--blueclaw-url http://127.0.0.1:18100")
	assertFileContains(t, filepath.Join(service.SystemdSystemDirectoryPath, "internkim-tenant-admind-pilot-01.service"), "--mattermost-admin-password "+filepath.Join(paths.InternKimSecretsPath, "mm-admin-pass"))
	assertFileContains(t, filepath.Join(service.SystemdSystemDirectoryPath, "internkim-tenant-admind-pilot-01.service"), "--fleet-id-path "+filepath.Join(paths.InternKimPath, "env", "fleet-id"))
	assertFileContains(t, filepath.Join(service.SystemdSystemDirectoryPath, "internkim-tenant-capabilityd-pilot-01.service"), "--mattermost-url http://127.0.0.1:18065")
	assertFileContains(t, filepath.Join(service.SystemdSystemDirectoryPath, "internkim-tenant-capabilityd-pilot-01.service"), "--socket "+filepath.Join(paths.InternKimPath, "run", "capability.sock"))
	assertFileContains(t, filepath.Join(service.SystemdSystemDirectoryPath, "internkim-tenant-capabilityd-pilot-01.service"), "--openrouter-key "+filepath.Join(paths.InternKimSecretsPath, "llm-device-token"))
	assertFileContains(t, filepath.Join(service.SystemdSystemDirectoryPath, "internkim-tenant-capabilityd-pilot-01.service"), "--openrouter-web-url https://internkim-llm-gateway.example/api/v1/chat/completions")
	assertFileContains(t, filepath.Join(service.SystemdSystemDirectoryPath, "internkim-tenant-capabilityd-pilot-01.service"), "--openrouter-embedding-url https://internkim-llm-gateway.example/api/v1/embeddings")
	assertFileContains(t, filepath.Join(paths.InternKimSecretsPath, "release-download-token"), "release-token")
	assertFileDoesNotContain(t, filepath.Join(service.SystemdSystemDirectoryPath, "internkim-tenant-capabilityd-pilot-01.service"), "--vsock-port")
	assertFileContains(t, filepath.Join(service.SystemdSystemDirectoryPath, "internkim-tenant-blueclaw-pilot-01.service"), filepath.Join(paths.BlueclawRootPath, "config", "runtime.json"))
	assertTenantRuntimeConfiguration(t, filepath.Join(paths.BlueclawRootPath, "config", "runtime.json"), runtimeDirectoryBasePath, filepath.Join(paths.InternKimPath, "run", "capability.sock"))
	assertTenantRuntimeConfiguration(t, filepath.Join(paths.BlueclawWorkspacePath, ".blueclaw/config/runtime.json"), runtimeDirectoryBasePath, filepath.Join(paths.InternKimPath, "run", "capability.sock"))
	assertFileContains(t, filepath.Join(paths.BlueclawWorkspacePath, ".blueclaw/config/policy.json"), "admin@pilot-01.local")
	assertHostRuntimeCommands(t, commandRunner.commands)
}

func TestInstallHostRuntimeRequiresGatewayURL(t *testing.T) {
	service := Service{BasePath: t.TempDir()}
	manifest := newTestCloudSharedManifest(t)
	if _, errorValue := service.CreateTenant(manifest); errorValue != nil {
		t.Fatal(errorValue)
	}

	_, errorValue := service.InstallHostRuntime(context.Background(), manifest.TenantID, HostRuntimeOptions{})

	if errorValue == nil || !strings.Contains(errorValue.Error(), "gateway URL") {
		t.Fatalf("expected gateway URL validation error, got %v", errorValue)
	}
}

func assertTenantRuntimeConfiguration(t *testing.T, path string, runtimeDirectoryBasePath string, capabilitySocketPath string) {
	t.Helper()
	documentBytes, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal(documentBytes, &runtimeConfiguration); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertRuntimeConfigurationValue(t, runtimeConfiguration, []string{"baseURL"}, "http://127.0.0.1:18100")
	assertRuntimeConfigurationValue(t, runtimeConfiguration, []string{"capabilities", "vsockPort"}, float64(7000))
	assertRuntimeConfigurationValue(t, runtimeConfiguration, []string{"connectors", "mattermost", "baseURL"}, "http://127.0.0.1:18065")
	assertRuntimeConfigurationValue(t, runtimeConfiguration, []string{"memory", "graphitiEndpoint"}, "http://127.0.0.1:7791")
	assertRuntimeConfigurationValue(t, runtimeConfiguration, []string{"firecracker", "outboundNetwork", "hostDeviceName"}, "bctap101")
	assertRuntimeConfigurationValue(t, runtimeConfiguration, []string{"firecracker", "healthPortOrService"}, "8082")
	assertRuntimeConfigurationValue(t, runtimeConfiguration, []string{"firecracker", "guestHTTPPortOrService"}, "8081")
	assertRuntimeConfigurationValue(t, runtimeConfiguration, []string{"firecracker", "runtimeDirectoryPath"}, filepath.Join(runtimeDirectoryBasePath, "pilot-01"))
	assertRuntimeConfigurationValue(t, runtimeConfiguration, []string{"languageModel", "capability", "model"}, "x-ai/grok-4.3")
	guestListenerProxies := runtimeConfiguration["firecracker"].(map[string]any)["guestListenerProxies"].([]any)
	if len(guestListenerProxies) != 1 {
		t.Fatalf("expected tenant capability listener proxy, got %+v", guestListenerProxies)
	}
	firstGuestListenerProxy := guestListenerProxies[0].(map[string]any)
	if firstGuestListenerProxy["guestPort"] != float64(7000) || firstGuestListenerProxy["targetUnixSocketPath"] != capabilitySocketPath {
		t.Fatalf("unexpected tenant capability listener proxy: %+v", firstGuestListenerProxy)
	}
}

func assertRuntimeConfigurationValue(t *testing.T, document map[string]any, path []string, expectedValue any) {
	t.Helper()
	var currentValue any = document
	for _, key := range path {
		currentDocument, isDocument := currentValue.(map[string]any)
		if !isDocument {
			t.Fatalf("expected document at %v, got %+v", path, currentValue)
		}
		currentValue = currentDocument[key]
	}
	if currentValue != expectedValue {
		t.Fatalf("expected %v at %v, got %+v", expectedValue, path, currentValue)
	}
}

func assertHostRuntimeCommands(t *testing.T, commands []ExecutableCommand) {
	t.Helper()
	expectedArguments := [][]string{
		{"-c"},
		{"-c"},
		{"-c"},
		{"-c"},
		{"-c"},
		{"sync-workspace", "--workspace-image"},
		{"daemon-reload"},
		{"enable", "internkim-tenant-graphiti-pilot-01.service"},
		{"restart", "internkim-tenant-graphiti-pilot-01.service"},
		{"enable", "internkim-tenant-capabilityd-pilot-01.service"},
		{"restart", "internkim-tenant-capabilityd-pilot-01.service"},
		{"enable", "internkim-tenant-blueclaw-pilot-01.service"},
		{"restart", "internkim-tenant-blueclaw-pilot-01.service"},
		{"enable", "internkim-tenant-admind-pilot-01.service"},
		{"restart", "internkim-tenant-admind-pilot-01.service"},
	}
	if len(commands) != len(expectedArguments) {
		t.Fatalf("expected %d commands, got %+v", len(expectedArguments), commands)
	}
	for index, expectedArgument := range expectedArguments {
		if strings.Join(commands[index].Arguments[:len(expectedArgument)], "\n") != strings.Join(expectedArgument, "\n") {
			t.Fatalf("unexpected command %d: %+v", index, commands[index])
		}
	}
	if commands[0].ExecutableName != "sh" || !strings.Contains(strings.Join(commands[0].Arguments, "\n"), "systemctl stop") {
		t.Fatalf("expected first command to stop Blueclaw before image changes, got %+v", commands[0])
	}
	if commands[1].ExecutableName != "sh" || !strings.Contains(strings.Join(commands[1].Arguments, "\n"), "mount -o loop") || !strings.Contains(strings.Join(commands[1].Arguments, "\n"), "-target-tcp 127.0.0.1:18100") {
		t.Fatalf("expected second command to patch tenant rootfs guest proxy target, got %+v", commands[1])
	}
	if commands[2].ExecutableName != "sh" || !strings.Contains(strings.Join(commands[2].Arguments, "\n"), "uv pip install") {
		t.Fatalf("expected third command to install Graphiti dependencies, got %+v", commands[2])
	}
	if commands[3].ExecutableName != "sh" || !strings.Contains(strings.Join(commands[3].Arguments, "\n"), "systemctl stop") {
		t.Fatalf("expected fourth command to stop Blueclaw before workspace sync, got %+v", commands[3])
	}
	if commands[4].ExecutableName != "sh" || !strings.Contains(strings.Join(commands[4].Arguments, "\n"), "e2fsck -fy") || !strings.Contains(strings.Join(commands[4].Arguments, "\n"), "mount -o loop") || !strings.Contains(strings.Join(commands[4].Arguments, "\n"), ".blueclaw/postgres/data") || !strings.Contains(strings.Join(commands[4].Arguments, "\n"), ".blueclaw/graphiti/kuzu") {
		t.Fatalf("expected fifth command to repair workspace image, got %+v", commands[4])
	}
	if commands[5].ExecutableName != "/usr/local/bin/blueclaw-supervisor" {
		t.Fatalf("expected sixth command to sync workspace image, got %+v", commands[5])
	}
	for index := 6; index < len(commands); index++ {
		if commands[index].ExecutableName != "systemctl" {
			t.Fatalf("expected systemctl command %d, got %+v", index, commands[index])
		}
	}
}

func assertFileDoesNotContain(t *testing.T, path string, unexpectedText string) {
	t.Helper()
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Contains(string(document), unexpectedText) {
		t.Fatalf("expected %s not to contain %q, got:\n%s", path, unexpectedText, string(document))
	}
}

func writeTestFileWithMode(t *testing.T, path string, document string, mode os.FileMode) string {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(path, []byte(document), mode); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}
