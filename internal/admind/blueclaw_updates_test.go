package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	blueclawruntime "github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func TestRuntimeRestampIncludesTheDeliveredGuestConfiguration(t *testing.T) {
	target := canonicalBlueclawPayloadInstallTarget()
	if target.DeliveryRuntimeConfigurationPath != filepath.Join(blueclawruntime.BlueclawDeliveryConfigPath, "runtime.json") {
		t.Fatal("the release target omits the configuration the guest reads")
	}
	directory := t.TempDir()
	target.RuntimeConfigurationPath = filepath.Join(directory, "host.json")
	target.WorkspaceRuntimeConfigurationPath = filepath.Join(directory, "workspace.json")
	target.DeliveryRuntimeConfigurationPath = filepath.Join(directory, "delivered.json")
	contract := blueclawruntime.CurrentCapabilityContract()
	staleDocument := `{"capabilities":{"aggregateProtocolHash":"previous"},"agent":{"name":"sample"}}`
	currentDocument, errorValue := refreshedBlueclawRuntimeConfiguration(staleDocument, contract)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, target.RuntimeConfigurationPath, currentDocument)
	writeFile(t, target.WorkspaceRuntimeConfigurationPath, currentDocument)
	writeFile(t, target.DeliveryRuntimeConfigurationPath, staleDocument)
	if isBlueclawRuntimeConfigurationCurrentForTarget(target, contract) {
		t.Fatal("current host copies hid a stale delivered configuration")
	}
	if errorValue := syncBlueclawRuntimeConfigurationForTarget(target, contract); errorValue != nil {
		t.Fatal(errorValue)
	}
	delivered, errorValue := os.ReadFile(target.DeliveryRuntimeConfigurationPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(delivered) != currentDocument {
		t.Fatal("the delivered configuration did not receive the current contract")
	}
}

func TestRuntimeRestampAddsAssertionKeyToEveryCopyWithoutChangingPolicy(t *testing.T) {
	directoryPath := t.TempDir()
	target := blueclawPayloadInstallTarget{
		Name:                              "blueclaw",
		RuntimeConfigurationPath:          filepath.Join(directoryPath, "host", "runtime.json"),
		WorkspaceRuntimeConfigurationPath: filepath.Join(directoryPath, "workspace", "runtime.json"),
		DeliveryRuntimeConfigurationPath:  filepath.Join(directoryPath, "delivery", "runtime.json"),
	}
	policyDocument := "{\"people\":[{\"personID\":\"sample-person\"}],\"retention\":{\"days\":30}}\n"
	for _, path := range []string{target.RuntimeConfigurationPath, target.WorkspaceRuntimeConfigurationPath, target.DeliveryRuntimeConfigurationPath} {
		if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
			t.Fatal(errorValue)
		}
		writeFile(t, path, "{\"memory\":{\"timeoutSecond\":30},\"custom\":{\"preserve\":true}}\n")
		writeFile(t, filepath.Join(filepath.Dir(path), "policy.json"), policyDocument)
	}

	if errorValue := syncBlueclawRuntimeConfigurationForTarget(target, blueclawruntime.CurrentCapabilityContract()); errorValue != nil {
		t.Fatal(errorValue)
	}
	wantedPath := blueclawruntime.BlueclawGuestDeliverySecretsPath + "/" + blueclawruntime.BlueclawAdminAssertionKeyName
	for _, path := range []string{target.RuntimeConfigurationPath, target.WorkspaceRuntimeConfigurationPath, target.DeliveryRuntimeConfigurationPath} {
		var runtimeDocument map[string]any
		document, errorValue := os.ReadFile(path)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if errorValue := json.Unmarshal(document, &runtimeDocument); errorValue != nil {
			t.Fatal(errorValue)
		}
		memorySection, ok := runtimeDocument["memory"].(map[string]any)
		if !ok || memorySection["adminAssertionKeyPath"] != wantedPath {
			t.Fatalf("runtime copy %s has assertion key path %#v", path, memorySection)
		}
		if runtimeDocument["custom"].(map[string]any)["preserve"] != true {
			t.Fatalf("runtime copy %s lost unknown fields", path)
		}
		policy, errorValue := os.ReadFile(filepath.Join(filepath.Dir(path), "policy.json"))
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if string(policy) != policyDocument {
			t.Fatalf("policy changed at %s: %q", path, policy)
		}
	}
}

func TestRuntimeRestampPreservesExplicitAssertionKeyPath(t *testing.T) {
	document := `{"memory":{"adminAssertionKeyPath":"/custom/key"}}`
	refreshed, errorValue := refreshedBlueclawRuntimeConfiguration(document, blueclawruntime.CurrentCapabilityContract())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(refreshed, `"adminAssertionKeyPath": "/custom/key"`) {
		t.Fatalf("explicit assertion key path was replaced: %s", refreshed)
	}
}

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
	runtimeConfiguration.Guest.HostWorkspacePath = filepath.Join(tenantBasePath, "pilot-01", "blueclaw", "workspace")
	runtimeConfiguration.Guest.WorkspaceImagePath = filepath.Join(tenantBasePath, "pilot-01", "blueclaw", "guest", "workspace.ext4")
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
	if target.HostWorkspacePath != runtimeConfiguration.Guest.HostWorkspacePath {
		t.Fatalf("unexpected host workspace path: %+v", target)
	}
	if target.WorkspaceImagePath != runtimeConfiguration.Guest.WorkspaceImagePath {
		t.Fatalf("unexpected workspace image path: %+v", target)
	}
	if target.RuntimeConfigurationPath != runtimeConfigurationPath {
		t.Fatalf("unexpected runtime configuration path: %+v", target)
	}
	if target.WorkspaceRuntimeConfigurationPath != filepath.Join(runtimeConfiguration.Guest.HostWorkspacePath, ".blueclaw", "config", "runtime.json") {
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
		if request.URL.Path != "/admin/api/run" {
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

func TestBlueclawPayloadIsDeliveredWithoutTouchingTheImage(t *testing.T) {
	target := canonicalBlueclawPayloadInstallTarget()
	command := blueclawDeliveryRefreshCommandForTarget(target)
	if !strings.Contains(command, "rsync -a --delete") {
		t.Fatalf("expected the share to be refreshed, got %s", command)
	}
	for _, forbidden := range []string{"sync-workspace", "workspace.ext4"} {
		if strings.Contains(command, forbidden) {
			t.Fatalf("the payload arrives on the share, so %q has no reason to appear in %s", forbidden, command)
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

	if isBlueclawRuntimeConfigurationCurrentForTarget(target, blueclawruntime.CurrentCapabilityContract()) {
		t.Fatal("expected stale runtime configuration to be detected")
	}
	if errorValue := syncBlueclawRuntimeConfigurationForTarget(target, blueclawruntime.CurrentCapabilityContract()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !isBlueclawRuntimeConfigurationCurrentForTarget(target, blueclawruntime.CurrentCapabilityContract()) {
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

func TestSyncBlueclawWorkspaceImageForTargetRefreshesTheShareAndRestarts(t *testing.T) {
	_ = captureBlueclawTaskDrainLogs(t)
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		http.Error(responseWriter, "unavailable", http.StatusBadGateway)
	}))
	server.Close()
	service := NewService(Configuration{BlueclawBaseURL: server.URL})
	service.HTTPClient = server.Client()
	commands := []string{}
	service.RunCommand = func(_ context.Context, name string, arguments ...string) ([]byte, error) {
		commands = append(commands, strings.Join(append([]string{name}, arguments...), " "))
		return []byte("Inode: 2   Type: directory    Mode:  0755"), nil
	}
	target := canonicalBlueclawPayloadInstallTarget()
	jobID := service.newJob("blueclaw-update").JobID

	if errorValue := service.refreshBlueclawDeliveryForTarget(context.Background(), jobID, target); errorValue != nil {
		t.Fatalf("expected the delivery refresh to succeed: %v", errorValue)
	}

	commandIndex := func(expectedText string) int {
		for index, command := range commands {
			if strings.Contains(command, expectedText) {
				return index
			}
		}
		return -1
	}
	refreshIndex := commandIndex("rsync -a --delete")
	restartIndex := commandIndex("systemctl restart ")
	if refreshIndex == -1 || restartIndex == -1 {
		t.Fatalf("expected the share to be refreshed and the guest restarted, got:\n%s", strings.Join(commands, "\n"))
	}
	if refreshIndex > restartIndex {
		t.Fatalf("the guest reads the share when it starts, so the refresh has to come first:\n%s", strings.Join(commands, "\n"))
	}
	if commandIndex("systemctl stop ") != -1 {
		t.Fatalf("nothing writes into the image any more, so the guest need not be stopped:\n%s", strings.Join(commands, "\n"))
	}
}

func TestReconcileBlueclawRuntimeConfigurationForTargetRestampsStaleIdentityAndRestarts(t *testing.T) {
	_ = captureBlueclawTaskDrainLogs(t)
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		http.Error(responseWriter, "unavailable", http.StatusBadGateway)
	}))
	server.Close()
	directoryPath := t.TempDir()
	runtimeConfigurationPath := filepath.Join(directoryPath, "config", "runtime.json")
	workspaceRuntimeConfigurationPath := filepath.Join(directoryPath, "workspace", ".blueclaw", "config", "runtime.json")
	for _, path := range []string{runtimeConfigurationPath, workspaceRuntimeConfigurationPath} {
		if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
			t.Fatal(errorValue)
		}
		writeFile(t, path, `{"capabilities":{"protocolVersion":"0.1.0","aggregateProtocolHash":"stale"}}`)
	}
	target := blueclawPayloadInstallTarget{
		Name:                              "blueclaw",
		ServiceName:                       "blueclaw.service",
		HostWorkspacePath:                 filepath.Join(directoryPath, "workspace"),
		WorkspaceImagePath:                filepath.Join(directoryPath, "workspace.ext4"),
		RuntimeConfigurationPath:          runtimeConfigurationPath,
		WorkspaceRuntimeConfigurationPath: workspaceRuntimeConfigurationPath,
	}
	service := NewService(Configuration{BlueclawBaseURL: server.URL})
	service.HTTPClient = server.Client()
	commands := []string{}
	service.RunCommand = func(_ context.Context, name string, arguments ...string) ([]byte, error) {
		commands = append(commands, strings.Join(append([]string{name}, arguments...), " "))
		return []byte("Inode: 2   Type: directory    Mode:  0755"), nil
	}

	service.reconcileBlueclawRuntimeConfigurationForTarget(context.Background(), target)

	if !isBlueclawRuntimeConfigurationCurrentForTarget(target, blueclawruntime.CurrentCapabilityContract()) {
		t.Fatal("expected stale runtime configuration to be restamped")
	}
	contract := blueclawruntime.CurrentCapabilityContract()
	for _, path := range []string{runtimeConfigurationPath, workspaceRuntimeConfigurationPath} {
		document, errorValue := os.ReadFile(path)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if !strings.Contains(string(document), contract.AggregateProtocolHash) {
			t.Fatalf("expected current aggregate protocol hash in %s: %s", path, string(document))
		}
	}
	if !strings.Contains(strings.Join(commands, "\n"), "rsync -a --delete") {
		t.Fatalf("expected the share to be refreshed after restamp, got:\n%s", strings.Join(commands, "\n"))
	}

	commandsBeforeSecondRun := len(commands)
	service.reconcileBlueclawRuntimeConfigurationForTarget(context.Background(), target)
	if len(commands) != commandsBeforeSecondRun {
		t.Fatalf("expected a current runtime configuration to restart nothing, got:\n%s", strings.Join(commands[commandsBeforeSecondRun:], "\n"))
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
	runtimeConfiguration.Guest.HostWorkspacePath = filepath.Join(tenantBasePath, tenantID, "blueclaw", "workspace")
	runtimeConfiguration.Guest.WorkspaceImagePath = filepath.Join(tenantBasePath, tenantID, "blueclaw", "guest", "workspace.ext4")
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
  "guest": {"hostWorkspacePath": "/srv/keep/this"}
}`

	refreshed, errorValue := refreshBlueclawCapabilityContract(staleDocument, blueclawruntime.CurrentCapabilityContract())
	if errorValue != nil {
		t.Fatalf("refresh returned error: %v", errorValue)
	}
	if strings.Contains(refreshed, "flow.task.add") {
		t.Fatalf("expected legacy flow.task.add to be gone, got:\n%s", refreshed)
	}
	if !strings.Contains(refreshed, `"task_add"`) {
		t.Fatalf("expected neutral task_add descriptor, got:\n%s", refreshed)
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

func TestRefreshBlueclawCapabilityContractRetiresLLMDLineage(t *testing.T) {
	llmdDeviceDocument := `{
  "capabilities": {
    "transport": "vsock",
    "protocolVersion": "stale",
    "aggregateProtocolHash": "stale",
    "toolDescriptors": [],
    "routing": {"candidates": [], "localOnly": false}
  },
  "languageModel": {
    "defaultProvider": "llmd",
    "llmd": {"endpoint": "http://127.0.0.1:18081/_internkim/llmd"},
    "capability": {"model": "preserve-me"}
  }
}`

	refreshed, errorValue := refreshBlueclawCapabilityContract(llmdDeviceDocument, blueclawruntime.CurrentCapabilityContract())
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
	if _, exists := languageModel["defaultProvider"]; exists {
		t.Fatalf("a migrated device must retire the legacy provider selector, got %v", languageModel["defaultProvider"])
	}
	if _, isPresent := languageModel["llmd"]; isPresent {
		t.Fatalf("the retired llmd section must not survive a refresh:\n%s", refreshed)
	}
	capability, _ := languageModel["capability"].(map[string]any)
	if capability["model"] != "preserve-me" {
		t.Fatalf("existing capability configuration must be preserved:\n%s", refreshed)
	}
}

func TestRefreshBlueclawCapabilityContractLeavesAnExplicitProviderAlone(t *testing.T) {
	configuredDocument := `{
  "capabilities": {"routing": {"candidates": []}},
  "languageModel": {
    "defaultProvider": "direct",
    "direct": {"apiKeyPath": "/srv/keep/this"}
  }
}`

	refreshed, errorValue := refreshBlueclawCapabilityContract(configuredDocument, blueclawruntime.CurrentCapabilityContract())
	if errorValue != nil {
		t.Fatalf("refresh returned error: %v", errorValue)
	}
	if !strings.Contains(refreshed, `"defaultProvider": "direct"`) {
		t.Fatalf("an explicitly chosen provider must be preserved:\n%s", refreshed)
	}
	if !strings.Contains(refreshed, `"apiKeyPath": "/srv/keep/this"`) {
		t.Fatalf("that provider's own configuration must be preserved:\n%s", refreshed)
	}
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

func TestWaitForBlueclawWorkspaceImageReleaseWaitsForTheHolderToExit(t *testing.T) {
	originalWait := blueclawWorkspaceImageReleaseWait
	originalProbe := blueclawWorkspaceImageHolderProbe
	blueclawWorkspaceImageReleaseWait = 30 * time.Second
	remainingHeldProbes := 2
	blueclawWorkspaceImageHolderProbe = func(string) (string, bool) {
		if remainingHeldProbes > 0 {
			remainingHeldProbes--
			return "process 123 (cloud-hypervisor)", true
		}
		return "", false
	}
	t.Cleanup(func() {
		blueclawWorkspaceImageReleaseWait = originalWait
		blueclawWorkspaceImageHolderProbe = originalProbe
	})

	service := &Service{}
	if errorValue := service.waitForBlueclawWorkspaceImageRelease(context.Background(), "/tmp/workspace.ext4"); errorValue != nil {
		t.Fatalf("expected the wait to succeed once the holder exits: %v", errorValue)
	}
}

func TestWaitForBlueclawWorkspaceImageReleaseNamesAPersistentHolder(t *testing.T) {
	originalWait := blueclawWorkspaceImageReleaseWait
	originalProbe := blueclawWorkspaceImageHolderProbe
	blueclawWorkspaceImageReleaseWait = 10 * time.Millisecond
	blueclawWorkspaceImageHolderProbe = func(string) (string, bool) {
		return "process 123 (cloud-hypervisor)", true
	}
	t.Cleanup(func() {
		blueclawWorkspaceImageReleaseWait = originalWait
		blueclawWorkspaceImageHolderProbe = originalProbe
	})

	service := &Service{}
	errorValue := service.waitForBlueclawWorkspaceImageRelease(context.Background(), "/tmp/workspace.ext4")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "process 123 (cloud-hypervisor)") {
		t.Fatalf("expected a holder-naming error, got %v", errorValue)
	}
}

func TestDescriptorOpenModeSaysWhetherTheHolderCanWrite(t *testing.T) {
	if descriptorOpenMode("pos:\t0\nflags:\t0100000\nmnt_id:\t29\n") != "read-only" {
		t.Fatalf("read-only = %q", descriptorOpenMode("pos:\t0\nflags:\t0100000\n"))
	}
	if descriptorOpenMode("flags:\t0100002\n") != "for reading and writing" {
		t.Fatalf("read-write = %q", descriptorOpenMode("flags:\t0100002\n"))
	}
	if descriptorOpenMode("flags:\t0100001\n") != "write-only" {
		t.Fatalf("write-only = %q", descriptorOpenMode("flags:\t0100001\n"))
	}
	if descriptorOpenMode("nothing useful") != "unreadably" {
		t.Fatal("a document with no flags line cannot say how it was opened")
	}
}

func TestBlueclawWorkspaceImageHolderIgnoresThisProcess(t *testing.T) {
	workspaceImagePath := filepath.Join(t.TempDir(), "workspace.ext4")
	if errorValue := os.WriteFile(workspaceImagePath, []byte("image"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	heldFile, errorValue := os.Open(workspaceImagePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() { heldFile.Close() })

	if holderDescription, isHeld := blueclawWorkspaceImageHolder(workspaceImagePath); isHeld {
		t.Fatalf("admind must not count its own descriptor as a holder, got %s", holderDescription)
	}
}

func TestStampedGuestProtocolHashReadsTheCapabilitiesBlock(t *testing.T) {
	stamped, errorValue := stampedGuestProtocolHash([]byte(`{"capabilities":{"aggregateProtocolHash":"abc123","protocolVersion":"0.4.0"}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if stamped != "abc123" {
		t.Fatalf("stamped = %q", stamped)
	}
}

func TestVerifyGuestProtocolIdentityStampRejectsAStaleStamp(t *testing.T) {
	expected := blueclawruntime.CurrentCapabilityContract().AggregateProtocolHash
	if strings.TrimSpace(expected) == "" {
		t.Skip("this build carries no capability contract hash")
	}
	service := &Service{RunCommand: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"capabilities":{"aggregateProtocolHash":"0000000000000000000000000000000000000000000000000000000000000000"}}`), nil
	}}

	errorValue := service.verifyGuestProtocolIdentityStamp(context.Background(), canonicalBlueclawPayloadInstallTarget())

	if errorValue == nil || !strings.Contains(errorValue.Error(), expected) {
		t.Fatalf("expected the stale stamp to be named against %s, got %v", expected, errorValue)
	}
}

func TestVerifyGuestProtocolIdentityStampAcceptsTheCurrentStamp(t *testing.T) {
	expected := blueclawruntime.CurrentCapabilityContract().AggregateProtocolHash
	if strings.TrimSpace(expected) == "" {
		t.Skip("this build carries no capability contract hash")
	}
	service := &Service{RunCommand: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"capabilities":{"aggregateProtocolHash":"` + expected + `"}}`), nil
	}}

	if errorValue := service.verifyGuestProtocolIdentityStamp(context.Background(), canonicalBlueclawPayloadInstallTarget()); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestSeedWorkspaceRuntimeConfigurationCreatesTheCopyTheGuestBootsFrom(t *testing.T) {
	rootPath := t.TempDir()
	hostPath := filepath.Join(rootPath, "config", "runtime.json")
	workspacePath := filepath.Join(rootPath, "workspace", ".blueclaw", "config", "runtime.json")
	if errorValue := os.MkdirAll(filepath.Dir(hostPath), 0o750); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(hostPath, []byte(`{"capabilities":{"aggregateProtocolHash":"abc123"}}`), 0o640); errorValue != nil {
		t.Fatal(errorValue)
	}
	target := blueclawPayloadInstallTarget{Name: "blueclaw", RuntimeConfigurationPath: hostPath, WorkspaceRuntimeConfigurationPath: workspacePath}

	if !workspaceRuntimeConfigurationIsMissing(target) {
		t.Fatal("a workspace without the guest copy must not read as current")
	}
	if isBlueclawRuntimeConfigurationCurrentForTarget(target, blueclawruntime.CurrentCapabilityContract()) {
		t.Fatal("a missing guest copy must make the target stale")
	}
	if errorValue := seedWorkspaceRuntimeConfiguration(target); errorValue != nil {
		t.Fatal(errorValue)
	}

	document, errorValue := os.ReadFile(workspacePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(document), "abc123") {
		t.Fatalf("seeded document = %s", string(document))
	}
	if workspaceRuntimeConfigurationIsMissing(target) {
		t.Fatal("the seeded copy must settle the staleness that triggered it")
	}
}

func TestSeedWorkspaceRuntimeConfigurationLeavesAnExistingCopyAlone(t *testing.T) {
	rootPath := t.TempDir()
	hostPath := filepath.Join(rootPath, "runtime.json")
	workspacePath := filepath.Join(rootPath, "workspace-runtime.json")
	if errorValue := os.WriteFile(hostPath, []byte(`{"capabilities":{"aggregateProtocolHash":"host"}}`), 0o640); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(workspacePath, []byte(`{"capabilities":{"aggregateProtocolHash":"guest"}}`), 0o640); errorValue != nil {
		t.Fatal(errorValue)
	}
	target := blueclawPayloadInstallTarget{Name: "blueclaw", RuntimeConfigurationPath: hostPath, WorkspaceRuntimeConfigurationPath: workspacePath}

	if errorValue := seedWorkspaceRuntimeConfiguration(target); errorValue != nil {
		t.Fatal(errorValue)
	}

	document, _ := os.ReadFile(workspacePath)
	if !strings.Contains(string(document), "guest") {
		t.Fatalf("seeding must not overwrite an existing guest copy: %s", string(document))
	}
}

func serveCapabilityRegistryOnSocket(t *testing.T, document string) string {
	t.Helper()
	socketDirectory, errorValue := os.MkdirTemp("/tmp", "ik-capability")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDirectory) })
	socketPath := filepath.Join(socketDirectory, "c.sock")
	listener, errorValue := net.Listen("unix", socketPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(document))
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	return socketPath
}

func TestCapabilityContractForStampingFollowsTheRunningCapabilityd(t *testing.T) {
	socketPath := serveCapabilityRegistryOnSocket(t, `{"protocolVersion":"0.4.0","aggregateProtocolHash":"1111111111111111111111111111111111111111111111111111111111111111","capabilities":[{"name":"task_write"}]}`)
	service := &Service{Configuration: Configuration{CapabilitySocketPath: socketPath}}

	contract := service.capabilityContractForStamping(context.Background())

	if contract.AggregateProtocolHash != "1111111111111111111111111111111111111111111111111111111111111111" {
		t.Fatalf("the stamp must follow the running capabilityd, got %q", contract.AggregateProtocolHash)
	}
	if len(contract.ToolDescriptors) != 1 || contract.ToolDescriptors[0].Name != "task_write" {
		t.Fatalf("descriptors = %+v", contract.ToolDescriptors)
	}
}

func TestCapabilityContractForStampingFallsBackWhenCapabilitydIsUnreachable(t *testing.T) {
	service := &Service{Configuration: Configuration{CapabilitySocketPath: filepath.Join(t.TempDir(), "absent.sock")}}

	contract := service.capabilityContractForStamping(context.Background())

	if contract.AggregateProtocolHash != blueclawruntime.CurrentCapabilityContract().AggregateProtocolHash {
		t.Fatalf("an unreachable capabilityd must leave this admind's own contract in place, got %q", contract.AggregateProtocolHash)
	}
}

func TestThePayloadIsVerifiedWhereItWasDelivered(t *testing.T) {
	commands := []string{}
	service := &Service{}
	service.RunCommand = func(_ context.Context, name string, arguments ...string) ([]byte, error) {
		commands = append(commands, strings.Join(append([]string{name}, arguments...), " "))
		return []byte("{}"), nil
	}
	artifactPath := t.TempDir()
	if errorValue := os.WriteFile(filepath.Join(artifactPath, "manifest.json"), []byte("{}"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	matches, detail := service.blueclawWorkspaceManifestMatchesTarget(artifactPath, canonicalBlueclawPayloadInstallTarget())

	if !matches {
		t.Fatalf("expected the delivered manifest to match, got %s", detail)
	}
	joined := strings.Join(commands, "\n")
	if strings.Contains(joined, "debugfs") || strings.Contains(joined, "workspace.ext4") {
		t.Fatalf("the payload no longer reaches the workspace image, so reading it there compares against something that stopped being written: %s", joined)
	}
	if !strings.Contains(joined, blueclawruntime.BlueclawDeliveryRuntimePath) {
		t.Fatalf("expected the check to read the share the guest runs from: %s", joined)
	}
}

func TestNothingOnThePayloadPathReadsTheWorkspaceImage(t *testing.T) {
	target := canonicalBlueclawPayloadInstallTarget()

	for name, command := range map[string]string{
		"delivery refresh":  blueclawDeliveryRefreshCommandForTarget(target),
		"guest stamp read":  guestRuntimeConfigurationReadCommand(),
		"stop before apply": stopBlueclawPayloadTargetCommand(target),
		"start after apply": startBlueclawPayloadTargetCommand(target),
	} {
		if strings.Contains(command, "debugfs") || strings.Contains(command, target.WorkspaceImagePath) {
			t.Fatalf("the payload and the configuration reach the guest through the share, so %s reading the image compares against a copy nothing writes: %s", name, command)
		}
	}
}

// The admind applying a release is always the one the previous release
// installed, so its own compiled contract is a release behind. Judging the
// guest stamp against it let a device whose capabilityd already served a new
// aggregate hash report itself current, and the guest kept the pin it booted
// with until its connector refused to start.
func TestTheGuestStampIsVerifiedAgainstTheRunningCapabilityd(t *testing.T) {
	const servedHash = "2222222222222222222222222222222222222222222222222222222222222222"
	socketPath := serveCapabilityRegistryOnSocket(t, `{"protocolVersion":"0.4.0","aggregateProtocolHash":"`+servedHash+`","capabilities":[{"name":"task_write"}]}`)
	service := &Service{Configuration: Configuration{CapabilitySocketPath: socketPath}}
	service.RunCommand = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		stampedWithThisBinarysContract := `{"capabilities":{"aggregateProtocolHash":"` +
			blueclawruntime.CurrentCapabilityContract().AggregateProtocolHash + `"}}`
		return []byte(stampedWithThisBinarysContract), nil
	}

	errorValue := service.verifyGuestProtocolIdentityStamp(context.Background(), canonicalBlueclawPayloadInstallTarget())

	if errorValue == nil {
		t.Fatal("a guest stamped with this admind's own contract must fail against the hash capabilityd serves")
	}
	if !strings.Contains(errorValue.Error(), servedHash) {
		t.Fatalf("the failure must name the hash the device actually serves, got %v", errorValue)
	}
}
