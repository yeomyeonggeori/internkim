package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestPublicBlueclawUpdateMetadataHidesArchivePath(t *testing.T) {
	metadata := &blueclawUpdateArtifactMetadata{
		Component:        "blueclaw",
		Version:          "version-1",
		BlueclawRevision: "revision-1",
		ArchivePath:      "/root/private/payload.tar.gz",
	}

	publicMetadata := publicBlueclawUpdateMetadata(metadata)

	if publicMetadata.ArchivePath != "" {
		t.Fatalf("archive path was exposed: %q", publicMetadata.ArchivePath)
	}
	if metadata.ArchivePath == "" {
		t.Fatal("source metadata was mutated")
	}
}

func TestWriteLimitedRequestBodyRejectsOversizedInput(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), "chunk")

	errorValue := writeLimitedRequestBody(targetPath, strings.NewReader("12345"), 4)

	if errorValue == nil {
		t.Fatal("expected oversized request body to fail")
	}
}

func TestValidateBlueclawUpdateUploadRequest(t *testing.T) {
	payload := blueclawUpdateUploadCreateRequest{
		fleetSignedRequest: fleetSignedRequest{
			Action:    "blueclaw-update-upload",
			DeviceID:  "device-1",
			Nonce:     "nonce-1",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
		Version:  "revision-1",
		Filename: "payload.tar.gz",
		Size:     10,
		SHA256:   strings.Repeat("a", 64),
	}

	if errorValue := validateBlueclawUpdateUploadRequest(payload); errorValue != nil {
		t.Fatalf("valid upload request failed: %v", errorValue)
	}

	payload.SHA256 = "bad"
	if errorValue := validateBlueclawUpdateUploadRequest(payload); errorValue == nil {
		t.Fatal("invalid sha256 was accepted")
	}

	payload.SHA256 = strings.Repeat("a", 64)
	payload.Size = 0
	if errorValue := validateBlueclawUpdateUploadRequest(payload); errorValue == nil {
		t.Fatal("missing size was accepted")
	}
}

func TestPersistedBlueclawUpdateMetadataKeepsArchivePath(t *testing.T) {
	stateDirectory := t.TempDir()
	path := filepath.Join(stateDirectory, "metadata.json")
	metadata := &blueclawUpdateArtifactMetadata{
		Component:        "blueclaw",
		Version:          "version-1",
		BlueclawRevision: "revision-1",
		ArchivePath:      "/root/private/payload.tar.gz",
	}

	if errorValue := writeBlueclawUpdateMetadata(path, metadata); errorValue != nil {
		t.Fatal(errorValue)
	}

	readMetadata := readBlueclawUpdateMetadata(path)
	if readMetadata == nil || readMetadata.ArchivePath != metadata.ArchivePath {
		document, _ := os.ReadFile(path)
		t.Fatalf("archive path was not persisted: %s", string(document))
	}
}

func TestBlueclawPayloadTenantInstallTargetsReadRuntimeConfiguration(t *testing.T) {
	tenantBasePath := t.TempDir()
	runtimeConfigurationPath := filepath.Join(tenantBasePath, "pilot-01", "blueclaw", "config", "runtime.json")
	if errorValue := os.MkdirAll(filepath.Dir(runtimeConfigurationPath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	runtimeConfiguration := blueclawPayloadRuntimeConfiguration{}
	runtimeConfiguration.Firecracker.HostWorkspacePath = filepath.Join(tenantBasePath, "pilot-01", "blueclaw", "workspace")
	runtimeConfiguration.Firecracker.WorkspaceImagePath = filepath.Join(tenantBasePath, "pilot-01", "blueclaw", "firecracker", "workspace.ext4")
	document, errorValue := json.Marshal(runtimeConfiguration)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(runtimeConfigurationPath, document, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	targets := blueclawPayloadTenantInstallTargets(tenantBasePath)

	if len(targets) != 1 {
		t.Fatalf("expected one tenant target, got %+v", targets)
	}
	target := targets[0]
	if target.Name != "pilot-01" || target.ServiceName != "internkim-tenant-blueclaw-pilot-01.service" {
		t.Fatalf("unexpected tenant target identity: %+v", target)
	}
	if target.HostWorkspacePath != runtimeConfiguration.Firecracker.HostWorkspacePath {
		t.Fatalf("unexpected host workspace path: %+v", target)
	}
	if target.WorkspaceImagePath != runtimeConfiguration.Firecracker.WorkspaceImagePath {
		t.Fatalf("unexpected workspace image path: %+v", target)
	}
	if target.RuntimeConfigurationPath != runtimeConfigurationPath {
		t.Fatalf("unexpected runtime configuration path: %+v", target)
	}
	if target.WorkspaceRuntimeConfigurationPath != filepath.Join(runtimeConfiguration.Firecracker.HostWorkspacePath, ".blueclaw", "config", "runtime.json") {
		t.Fatalf("unexpected workspace runtime configuration path: %+v", target)
	}
	if target.PayloadManifestPath != filepath.Join(tenantBasePath, "pilot-01", "blueclaw", "payload-manifest.json") {
		t.Fatalf("unexpected payload manifest path: %+v", target)
	}
}

func TestBlueclawPayloadInstallTargetsIncludeCanonicalAndTenants(t *testing.T) {
	tenantBasePath := t.TempDir()
	writeBlueclawPayloadTenantRuntimeConfiguration(t, tenantBasePath, "pilot-02")
	writeBlueclawPayloadTenantRuntimeConfiguration(t, tenantBasePath, "pilot-01")

	targets := blueclawPayloadInstallTargets(tenantBasePath)
	targetNames := []string{}
	for _, target := range targets {
		targetNames = append(targetNames, target.Name)
	}

	expectedTargetNames := []string{"blueclaw", "pilot-01", "pilot-02"}
	if strings.Join(targetNames, "\n") != strings.Join(expectedTargetNames, "\n") {
		t.Fatalf("target names = %+v, want %+v", targetNames, expectedTargetNames)
	}
}

func TestDrainBlueclawTasksBeforeStopCompletesWhenActiveTasksReachZero(t *testing.T) {
	runningRequestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/admin/api/task" {
			http.NotFound(responseWriter, request)
			return
		}
		status := request.URL.Query().Get("status")
		if status == "running" {
			runningRequestCount++
		}
		if status == "running" && runningRequestCount == 1 {
			writeBlueclawTaskDrainResponse(t, responseWriter, []blueclawTaskRunListItem{{TaskRunID: "task-1", Status: "running"}})
			return
		}
		writeBlueclawTaskDrainResponse(t, responseWriter, []blueclawTaskRunListItem{})
	}))
	defer server.Close()
	service := NewService(Configuration{BlueclawBaseURL: server.URL})
	service.HTTPClient = server.Client()

	service.drainBlueclawTasksBeforeStopWithPollInterval(context.Background(), blueclawPayloadInstallTarget{Name: "blueclaw"}, 200*time.Millisecond, time.Millisecond)

	if runningRequestCount < 2 {
		t.Fatalf("expected drain to poll until zero tasks, got %d running requests", runningRequestCount)
	}
}

func TestDrainBlueclawTasksBeforeStopTimesOutWithActiveTasks(t *testing.T) {
	logOutput := captureBlueclawTaskDrainLogs(t)
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		writeBlueclawTaskDrainResponse(t, responseWriter, []blueclawTaskRunListItem{{TaskRunID: "task-1", Status: request.URL.Query().Get("status")}})
	}))
	defer server.Close()
	service := NewService(Configuration{BlueclawBaseURL: server.URL})
	service.HTTPClient = server.Client()

	service.drainBlueclawTasksBeforeStopWithPollInterval(context.Background(), blueclawPayloadInstallTarget{Name: "blueclaw"}, 15*time.Millisecond, time.Millisecond)

	if !strings.Contains(logOutput.String(), "timed out") {
		t.Fatalf("expected timeout warning, got %s", logOutput.String())
	}
}

func TestDrainBlueclawTasksBeforeStopReturnsImmediatelyWhenTaskAPIUnavailable(t *testing.T) {
	logOutput := captureBlueclawTaskDrainLogs(t)
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		http.Error(responseWriter, "unavailable", http.StatusBadGateway)
	}))
	server.Close()
	service := NewService(Configuration{BlueclawBaseURL: server.URL})
	service.HTTPClient = server.Client()
	startedAt := time.Now()

	service.drainBlueclawTasksBeforeStopWithPollInterval(context.Background(), blueclawPayloadInstallTarget{Name: "blueclaw"}, 200*time.Millisecond, 50*time.Millisecond)

	if time.Since(startedAt) >= 50*time.Millisecond {
		t.Fatalf("expected immediate return when API is unavailable")
	}
	if !strings.Contains(logOutput.String(), "task API unavailable") {
		t.Fatalf("expected unavailable warning, got %s", logOutput.String())
	}
}

func TestHostWorkspacePayloadSyncCommandUsesTenantTarget(t *testing.T) {
	target := blueclawPayloadInstallTarget{
		HostWorkspacePath: "/srv/internkim/tenants/pilot-01/blueclaw/workspace",
	}

	command := hostWorkspacePayloadSyncCommandForTarget("/tmp/payload", target)

	for _, expectedText := range []string{
		"/tmp/payload/workspace/.blueclaw/runtime/",
		"/srv/internkim/tenants/pilot-01/blueclaw/workspace/.blueclaw/runtime/",
		"rsync -a --delete",
		"chown -R blueclaw:blueclaw",
	} {
		if !strings.Contains(command, expectedText) {
			t.Fatalf("expected tenant sync command to include %q, got:\n%s", expectedText, command)
		}
	}
	if strings.Contains(command, "/root/.blueclaw/workspace") {
		t.Fatalf("tenant sync command must not use canonical workspace, got:\n%s", command)
	}
}

func TestBlueclawPayloadWorkspaceSyncCommandPreservesGuestState(t *testing.T) {
	target := canonicalBlueclawPayloadInstallTarget()
	command := blueclawPayloadWorkspaceSyncCommand(target)
	for _, expectedText := range []string{
		"sync-workspace --atomic --preserve-guest-state",
		"--source '/root/.blueclaw/workspace'",
	} {
		if !strings.Contains(command, expectedText) {
			t.Fatalf("expected %q in %s", expectedText, command)
		}
	}
}

func TestSyncBlueclawRuntimeConfigurationForTargetUpdatesMigrationPath(t *testing.T) {
	directoryPath := t.TempDir()
	runtimeConfigurationPath := filepath.Join(directoryPath, "config", "runtime.json")
	workspaceRuntimeConfigurationPath := filepath.Join(directoryPath, "workspace", ".blueclaw", "config", "runtime.json")
	for _, path := range []string{runtimeConfigurationPath, workspaceRuntimeConfigurationPath} {
		if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
			t.Fatal(errorValue)
		}
		writeFile(t, path, `{"database":{"migrationDirectoryPath":"/workspace/.blueclaw/migrations"}}`)
	}
	target := blueclawPayloadInstallTarget{
		RuntimeConfigurationPath:          runtimeConfigurationPath,
		WorkspaceRuntimeConfigurationPath: workspaceRuntimeConfigurationPath,
	}

	if isBlueclawRuntimeConfigurationCurrentForTarget(target) {
		t.Fatal("expected stale runtime configuration to be detected")
	}
	if errorValue := syncBlueclawRuntimeConfigurationForTarget(target); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !isBlueclawRuntimeConfigurationCurrentForTarget(target) {
		t.Fatal("expected runtime configuration to be current")
	}
	for _, path := range []string{runtimeConfigurationPath, workspaceRuntimeConfigurationPath} {
		document, errorValue := os.ReadFile(path)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if !strings.Contains(string(document), "/workspace/.blueclaw/runtime/current/migrations") {
			t.Fatalf("expected current migration path in %s: %s", path, string(document))
		}
	}
}

func writeBlueclawTaskDrainResponse(t *testing.T, responseWriter http.ResponseWriter, taskRuns []blueclawTaskRunListItem) {
	t.Helper()
	responseWriter.Header().Set("Content-Type", "application/json")
	if errorValue := json.NewEncoder(responseWriter).Encode(taskRuns); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func captureBlueclawTaskDrainLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var output bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
	})
	return &output
}

func writeBlueclawPayloadTenantRuntimeConfiguration(t *testing.T, tenantBasePath string, tenantID string) {
	t.Helper()
	runtimeConfigurationPath := filepath.Join(tenantBasePath, tenantID, "blueclaw", "config", "runtime.json")
	if errorValue := os.MkdirAll(filepath.Dir(runtimeConfigurationPath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	runtimeConfiguration := blueclawPayloadRuntimeConfiguration{}
	runtimeConfiguration.Firecracker.HostWorkspacePath = filepath.Join(tenantBasePath, tenantID, "blueclaw", "workspace")
	runtimeConfiguration.Firecracker.WorkspaceImagePath = filepath.Join(tenantBasePath, tenantID, "blueclaw", "firecracker", "workspace.ext4")
	document, errorValue := json.Marshal(runtimeConfiguration)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(runtimeConfigurationPath, document, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestRefreshBlueclawCapabilityContractReplacesStaleOperationNames(t *testing.T) {
	staleDocument := `{
  "capabilities": {
    "transport": "vsock",
    "protocolVersion": "stale",
    "aggregateProtocolHash": "stale",
    "toolDescriptors": [{"name": "flow.task.add", "version": "1"}],
    "toolNames": ["flow.task.add"],
    "routing": {"candidates": ["flow.task.add"], "localOnly": false}
  },
  "languageModel": {"capability": {"model": "preserve-me"}},
  "firecracker": {"hostWorkspacePath": "/srv/keep/this"}
}`

	refreshed, errorValue := refreshBlueclawCapabilityContract(staleDocument)
	if errorValue != nil {
		t.Fatalf("refresh returned error: %v", errorValue)
	}
	if strings.Contains(refreshed, "flow.task.add") {
		t.Fatalf("expected legacy flow.task.add to be gone, got:\n%s", refreshed)
	}
	if !strings.Contains(refreshed, `"task.add"`) {
		t.Fatalf("expected neutral task.add descriptor, got:\n%s", refreshed)
	}
	currentContract := blueclawruntime.CurrentCapabilityContract()
	if !strings.Contains(refreshed, `"protocolVersion": "`+currentContract.ProtocolVersion+`"`) ||
		!strings.Contains(refreshed, `"aggregateProtocolHash": "`+currentContract.AggregateProtocolHash+`"`) {
		t.Fatalf("expected current protocol identity, got:\n%s", refreshed)
	}
	if !strings.Contains(refreshed, "preserve-me") || !strings.Contains(refreshed, "/srv/keep/this") {
		t.Fatalf("expected host-specific fields preserved, got:\n%s", refreshed)
	}
}

func TestRefreshBlueclawCapabilityContractMigratesPreLLMDLineage(t *testing.T) {
	preLLMDDeviceDocument := `{
  "capabilities": {
    "transport": "vsock",
    "protocolVersion": "stale",
    "aggregateProtocolHash": "stale",
    "toolDescriptors": [],
    "routing": {"candidates": [], "localOnly": false}
  },
  "languageModel": {
    "defaultProvider": "capabilityLLM",
    "capability": {"model": "preserve-me"}
  }
}`

	refreshed, errorValue := refreshBlueclawCapabilityContract(preLLMDDeviceDocument)
	if errorValue != nil {
		t.Fatalf("refresh returned error: %v", errorValue)
	}
	var refreshedDocument map[string]any
	if errorValue := json.Unmarshal([]byte(refreshed), &refreshedDocument); errorValue != nil {
		t.Fatal(errorValue)
	}
	languageModel, ok := refreshedDocument["languageModel"].(map[string]any)
	if !ok {
		t.Fatalf("languageModel section missing:\n%s", refreshed)
	}
	if languageModel["defaultProvider"] != "llmd" {
		t.Fatalf("pre-llmd device must migrate to the authoritative provider, got %v", languageModel["defaultProvider"])
	}
	llmdSection, ok := languageModel["llmd"].(map[string]any)
	if !ok {
		t.Fatalf("llmd section missing:\n%s", refreshed)
	}
	if llmdSection["endpoint"] != blueclawGuestLLMDBridgeEndpoint {
		t.Fatalf("guest bridge endpoint missing, got %v", llmdSection["endpoint"])
	}
	capability, _ := languageModel["capability"].(map[string]any)
	if capability["model"] != "preserve-me" {
		t.Fatalf("existing capability configuration must be preserved:\n%s", refreshed)
	}
}

func TestRefreshBlueclawCapabilityContractKeepsExplicitLLMDConfiguration(t *testing.T) {
	configuredDocument := `{
  "capabilities": {"routing": {"candidates": []}},
  "languageModel": {
    "defaultProvider": "llmd",
    "llmd": {"endpoint": "", "unixSocketPath": "/run/internkim/capability.sock"}
  }
}`

	refreshed, errorValue := refreshBlueclawCapabilityContract(configuredDocument)
	if errorValue != nil {
		t.Fatalf("refresh returned error: %v", errorValue)
	}
	if !strings.Contains(refreshed, `"unixSocketPath": "/run/internkim/capability.sock"`) {
		t.Fatalf("existing socket transport must be preserved:\n%s", refreshed)
	}
	if strings.Contains(refreshed, blueclawGuestLLMDBridgeEndpoint) {
		t.Fatalf("socket-configured llmd must not gain the bridge endpoint:\n%s", refreshed)
	}
}

func refreshedDocumentIsCurrent(t *testing.T, document string) bool {
	t.Helper()
	refreshedAgain, errorValue := refreshedBlueclawRuntimeConfiguration(document)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return refreshedAgain == document
}

func TestRepairWorkspaceImageOnlyRunsOnCorruptProbe(t *testing.T) {
	target := canonicalBlueclawPayloadInstallTarget()
	cases := []struct {
		name         string
		probeOutput  string
		expectRepair bool
	}{
		{name: "healthy", probeOutput: "Inode: 2   Type: directory    Mode:  0755", expectRepair: false},
		{name: "missing image", probeOutput: "/var/lib/blueclaw/workspace.ext4: No such file or directory", expectRepair: false},
		{name: "corrupt bitmap", probeOutput: "Block bitmap checksum does not match bitmap while reading allocation bitmaps\nstat: Filesystem not open", expectRepair: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := &Service{}
			commands := []string{}
			service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
				command := strings.Join(append([]string{name}, arguments...), " ")
				commands = append(commands, command)
				if strings.Contains(command, "debugfs") {
					return []byte(testCase.probeOutput), nil
				}
				return []byte("e2fsck done"), nil
			}
			if errorValue := service.repairWorkspaceImageIfUnhealthy(context.Background(), "job-1", target); errorValue != nil {
				t.Fatalf("unexpected repair error: %v", errorValue)
			}
			ranRepair := false
			for _, command := range commands {
				if strings.Contains(command, "e2fsck") {
					ranRepair = true
				}
			}
			if ranRepair != testCase.expectRepair {
				t.Fatalf("expected repair=%v, commands=%v", testCase.expectRepair, commands)
			}
		})
	}
}
