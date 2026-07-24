package admind

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/releaseset"
	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

func TestCheckReleaseProtocolIdentityRequiresCapabilitydAndBlueclawAgreement(t *testing.T) {
	expectedIdentity := capabilityprotocol.GeneratedProtocolIdentity()
	capabilitySocketPath := startReleaseCapabilityRegistryServer(t, expectedIdentity)
	blueclawServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(responseWriter).Encode(releaseBlueclawHealth{
			Status: "ok",
			ProtocolIdentity: releaseProtocolIdentityResult{
				Passed:   true,
				Expected: expectedIdentity,
			},
		})
	}))
	defer blueclawServer.Close()
	service := Service{Configuration: Configuration{
		CapabilitySocketPath: capabilitySocketPath,
		BlueclawBaseURL:      blueclawServer.URL,
	}}

	if errorValue := service.checkReleaseProtocolIdentity(context.Background(), expectedIdentity); errorValue != nil {
		t.Fatalf("expected matching release identity: %v", errorValue)
	}
	mismatchedIdentity := expectedIdentity
	mismatchedIdentity.ProtocolVersion += "-mismatch"
	if errorValue := service.checkReleaseProtocolIdentity(context.Background(), mismatchedIdentity); errorValue == nil {
		t.Fatal("expected capabilityd identity mismatch")
	}
}

func TestCheckReleaseProtocolIdentityCapabilitydMismatchCarriesReceivedIdentity(t *testing.T) {
	expectedIdentity := capabilityprotocol.GeneratedProtocolIdentity()
	receivedIdentity := capabilityprotocol.ProtocolIdentity{
		ProtocolVersion:       "v-received",
		AggregateProtocolHash: strings.Repeat("1a", 32),
	}
	capabilitySocketPath := startReleaseCapabilityRegistryServer(t, receivedIdentity)
	service := Service{Configuration: Configuration{CapabilitySocketPath: capabilitySocketPath}}

	errorValue := service.checkReleaseProtocolIdentity(context.Background(), expectedIdentity)

	if errorValue == nil {
		t.Fatal("expected capabilityd identity mismatch")
	}
	for _, expectedText := range []string{
		"capabilityd protocol identity mismatch",
		expectedIdentity.ProtocolVersion,
		expectedIdentity.AggregateProtocolHash,
		receivedIdentity.ProtocolVersion,
		receivedIdentity.AggregateProtocolHash,
	} {
		if !strings.Contains(errorValue.Error(), expectedText) {
			t.Fatalf("expected mismatch error to include %q, got %s", expectedText, errorValue.Error())
		}
	}
}

func TestCheckReleaseProtocolIdentityBlueclawMismatchCarriesReceivedIdentity(t *testing.T) {
	expectedIdentity := capabilityprotocol.GeneratedProtocolIdentity()
	capabilitySocketPath := startReleaseCapabilityRegistryServer(t, expectedIdentity)
	receivedHash := strings.Repeat("2b", 32)
	blueclawServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(responseWriter).Encode(releaseBlueclawHealth{
			Status: "ok",
			ProtocolIdentity: releaseProtocolIdentityResult{
				Passed:   false,
				Expected: expectedIdentity,
				Capabilityd: releaseProtocolEndpointStatus{
					Status:                "ok",
					ProtocolVersion:       expectedIdentity.ProtocolVersion,
					AggregateProtocolHash: receivedHash,
				},
				LLMD: releaseProtocolEndpointStatus{
					Status:                "ok",
					Passed:                true,
					ProtocolVersion:       expectedIdentity.ProtocolVersion,
					AggregateProtocolHash: expectedIdentity.AggregateProtocolHash,
				},
				FailureReasons: []string{"capabilityd aggregate protocol hash mismatch"},
			},
		})
	}))
	defer blueclawServer.Close()
	service := Service{Configuration: Configuration{
		CapabilitySocketPath: capabilitySocketPath,
		BlueclawBaseURL:      blueclawServer.URL,
	}}

	errorValue := service.checkReleaseProtocolIdentity(context.Background(), expectedIdentity)

	if errorValue == nil {
		t.Fatal("expected Blueclaw identity mismatch")
	}
	for _, expectedText := range []string{
		"Blueclaw protocol identity mismatch",
		expectedIdentity.AggregateProtocolHash,
		receivedHash,
		"capabilityd aggregate protocol hash mismatch",
	} {
		if !strings.Contains(errorValue.Error(), expectedText) {
			t.Fatalf("expected mismatch error to include %q, got %s", expectedText, errorValue.Error())
		}
	}
}

func TestReleaseProtocolIdentityGateAppliesOnlyToProtocolComponents(t *testing.T) {
	unrelatedManifest := releaseset.NewManifest("release-1", "stable", map[string]releaseset.Component{
		"web": {Name: "web"},
	})
	if releaseProtocolIdentityComponentsPresent(&unrelatedManifest) {
		t.Fatal("expected unrelated component to skip protocol readiness")
	}
	for _, componentName := range []string{"capabilityd", "blueclawLLMD", "blueclawPayload"} {
		manifest := releaseset.NewManifest("release-1", "stable", map[string]releaseset.Component{
			componentName: {Name: componentName},
		})
		if !releaseProtocolIdentityComponentsPresent(&manifest) {
			t.Fatalf("expected %s to require protocol readiness", componentName)
		}
	}
}

func TestReleaseProtocolIdentityTransitionRequiresCompleteRuntime(t *testing.T) {
	service := Service{Configuration: Configuration{StateDirectory: t.TempDir()}}
	currentManifest := releaseset.NewManifest("release-1", "stable", map[string]releaseset.Component{
		"web": {Name: "web"},
	})
	currentManifest.ProtocolVersion += "-previous"
	if errorValue := service.writeCurrentReleaseManifest(&currentManifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	incompleteManifest := releaseset.NewManifest("release-2", "stable", map[string]releaseset.Component{
		"web": {Name: "web"},
	})

	errorValue := service.validateReleaseProtocolTransition(context.Background(), &incompleteManifest)

	if errorValue == nil || !strings.Contains(errorValue.Error(), "capabilityd, blueclawLLMD, blueclawPayload") {
		t.Fatalf("expected complete protocol runtime requirement, got %v", errorValue)
	}
	for _, componentName := range releaseProtocolIdentityComponentNames() {
		incompleteManifest.Components[componentName] = releaseset.Component{Name: componentName}
	}
	if errorValue := service.validateReleaseProtocolTransition(context.Background(), &incompleteManifest); errorValue != nil {
		t.Fatalf("expected complete protocol runtime transition: %v", errorValue)
	}
}

func TestReleaseProtocolIdentityTransitionAllowsUnrelatedSameIdentityRelease(t *testing.T) {
	service := Service{Configuration: Configuration{StateDirectory: t.TempDir()}}
	currentManifest := releaseset.NewManifest("release-1", "stable", map[string]releaseset.Component{
		"web": {Name: "web"},
	})
	if errorValue := service.writeCurrentReleaseManifest(&currentManifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	nextManifest := releaseset.NewManifest("release-2", "stable", map[string]releaseset.Component{
		"web": {Name: "web"},
	})

	if errorValue := service.validateReleaseProtocolTransition(context.Background(), &nextManifest); errorValue != nil {
		t.Fatalf("expected same-identity unrelated release: %v", errorValue)
	}
}

func startReleaseCapabilityRegistryServer(t *testing.T, identity capabilityprotocol.ProtocolIdentity) string {
	t.Helper()
	directoryPath, errorValue := os.MkdirTemp("/tmp", "ik-release-*")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(directoryPath)
	})
	socketPath := filepath.Join(directoryPath, "capability.sock")
	listener, errorValue := net.Listen("unix", socketPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/capabilities" {
			http.NotFound(responseWriter, request)
			return
		}
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{
			"protocolVersion":       identity.ProtocolVersion,
			"aggregateProtocolHash": identity.AggregateProtocolHash,
			"routingCandidates":     []string{},
		})
	})}
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
	})
	go func() {
		if errorValue := server.Serve(listener); errorValue != nil && !errors.Is(errorValue, http.ErrServerClosed) {
			t.Errorf("capability registry server failed: %v", errorValue)
		}
	}()
	return socketPath
}

func TestReleaseUpdateStateReportsCurrent(t *testing.T) {
	current := testReleaseManifest("release-1")
	latest := testReleaseManifest("release-1")

	if state := releaseUpdateState(current, latest, nil); state != "current" {
		t.Fatalf("expected current, got %q", state)
	}
}

func TestReleaseUpdateStateReportsUpdateAvailable(t *testing.T) {
	current := testReleaseManifest("release-1")
	latest := testReleaseManifest("release-2")

	if state := releaseUpdateState(current, latest, nil); state != "update_available" {
		t.Fatalf("expected update_available, got %q", state)
	}
}

func TestReleaseUpdateStateReportsUpdating(t *testing.T) {
	current := testReleaseManifest("release-1")
	latest := testReleaseManifest("release-2")
	job := &Job{Type: "release-update", Status: "running"}

	if state := releaseUpdateState(current, latest, job); state != "updating" {
		t.Fatalf("expected updating, got %q", state)
	}
}

func TestReleaseUpdateUploadAppliesThroughReleaseJob(t *testing.T) {
	service := newReleaseUpdateUploadTestService(t)
	commands := []string{}
	service.RunCommand = func(_ context.Context, name string, arguments ...string) ([]byte, error) {
		commands = append(commands, strings.Join(append([]string{name}, arguments...), " "))
		return []byte("ok\n"), nil
	}
	bundlePath, manifest := writeTestReleaseBundle(t, service)
	bundleSHA256 := fileSHA256(bundlePath)
	bundleSize := fileSize(t, bundlePath)

	createPayload := releaseUpdateUploadCreateRequest{
		fleetSignedRequest: signedTestFleetRequest(t, service, releaseUpdateUploadAction, "nonce-1"),
		ReleaseID:          manifest.ReleaseID,
		Filename:           "release.tar.gz",
		Size:               bundleSize,
		SHA256:             bundleSHA256,
	}
	createResponse := performReleaseUploadJSON(t, service, http.MethodPost, "/updates/uploads", createPayload, "")
	if createResponse.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", createResponse.Code, createResponse.Body.String())
	}
	var upload releaseUpdateUploadCreateResponse
	if errorValue := json.NewDecoder(createResponse.Body).Decode(&upload); errorValue != nil {
		t.Fatal(errorValue)
	}

	document, errorValue := os.ReadFile(bundlePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	chunkRequest := httptest.NewRequest(http.MethodPut, "/admin/api/updates/uploads/"+upload.UploadID+"/chunks/0", bytes.NewReader(document))
	chunkRequest.Header.Set("X-InternKim-Upload-Token", upload.UploadToken)
	chunkResponse := httptest.NewRecorder()
	service.handleAdmin(chunkResponse, chunkRequest)
	if chunkResponse.Code != http.StatusOK {
		t.Fatalf("chunk status = %d body = %s", chunkResponse.Code, chunkResponse.Body.String())
	}

	completeResponse := performReleaseUploadJSON(t, service, http.MethodPost, "/updates/uploads/"+upload.UploadID+"/complete", releaseUpdateUploadCompleteRequest{Chunks: 1, SHA256: bundleSHA256}, upload.UploadToken)
	if completeResponse.Code != http.StatusOK {
		t.Fatalf("complete status = %d body = %s", completeResponse.Code, completeResponse.Body.String())
	}
	waitForReleaseJob(t, service)
	current := service.readCurrentReleaseManifest()
	if current == nil || current.ReleaseID != manifest.ReleaseID {
		t.Fatalf("current release = %+v, want %s", current, manifest.ReleaseID)
	}
	installedSkillPath := filepath.Join(service.Configuration.BlueclawWorkspacePath, "skills", "test-skill", "SKILL.md")
	if strings.TrimSpace(readTrimmedFile(installedSkillPath)) != "test skill" {
		t.Fatalf("skill was not installed at %s", installedSkillPath)
	}
	joinedCommands := strings.Join(commands, "\n")
	if !strings.Contains(joinedCommands, "blueclaw-supervisor sync-workspace") {
		t.Fatalf("expected skills release to sync Blueclaw workspace, got %s", joinedCommands)
	}
	if !strings.Contains(joinedCommands, "sync-workspace --atomic") || !strings.Contains(joinedCommands, "--relative-target 'skills'") {
		t.Fatalf("expected atomic skills-only workspace sync, got %s", joinedCommands)
	}
	if !strings.Contains(joinedCommands, "resize2fs") || !strings.Contains(joinedCommands, "68719476736") {
		t.Fatalf("expected skills release to ensure Blueclaw workspace image capacity, got %s", joinedCommands)
	}
	if !strings.Contains(joinedCommands, service.Configuration.BlueclawWorkspacePath) {
		t.Fatalf("expected skills sync to use configured workspace path, got %s", joinedCommands)
	}
}

func TestReleaseUpdateUploadRejectsWrongSignedAction(t *testing.T) {
	service := newReleaseUpdateUploadTestService(t)
	payload := releaseUpdateUploadCreateRequest{
		fleetSignedRequest: signedTestFleetRequest(t, service, "release-update-apply", "nonce-1"),
		ReleaseID:          "release-1",
		Filename:           "release.tar.gz",
		Size:               1,
		SHA256:             strings.Repeat("a", 64),
	}
	response := performReleaseUploadJSON(t, service, http.MethodPost, "/updates/uploads", payload, "")
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestReleaseWebComponentRootPrefersWeb(t *testing.T) {
	stagingPath := t.TempDir()
	webRoot := filepath.Join(stagingPath, "web", "board-ui")
	legacyRoot := filepath.Join(stagingPath, "adminWeb", "legacy-board-ui")
	if errorValue := os.MkdirAll(webRoot, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.MkdirAll(legacyRoot, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}

	componentRoot, errorValue := (&Service{}).releaseWebComponentRoot(stagingPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if componentRoot != webRoot {
		t.Fatalf("component root = %q, want %q", componentRoot, webRoot)
	}
}

func TestInstallReleaseMattermostPluginsCopiesBundleDirectory(t *testing.T) {
	directoryPath := t.TempDir()
	stagingPath := filepath.Join(directoryPath, "staging")
	sourcePath := filepath.Join(stagingPath, "mattermostPlugins", "mattermost-plugins")
	if errorValue := os.MkdirAll(sourcePath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(sourcePath, "com.internkim.ephemeral-0.1.0.tar.gz"), "plugin-bundle")
	targetPath := filepath.Join(directoryPath, "opt", "mattermost-plugins", "com.internkim.ephemeral-0.1.0.tar.gz")
	service := NewService(Configuration{MattermostPluginBundlePath: targetPath})

	if errorValue := service.installReleaseMattermostPlugins(stagingPath); errorValue != nil {
		t.Fatal(errorValue)
	}

	if strings.TrimSpace(readTrimmedFile(targetPath)) != "plugin-bundle" {
		t.Fatalf("plugin bundle was not installed at %s", targetPath)
	}
}

func TestInstallReleaseLLMDBinary(t *testing.T) {
	stagingPath := t.TempDir()
	sourceDirectoryPath := filepath.Join(stagingPath, "blueclawLLMD", "blueclaw-llmd")
	if errorValue := os.MkdirAll(sourceDirectoryPath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(sourceDirectoryPath, "blueclaw-llmd"), "llmd binary")
	targetPath := filepath.Join(t.TempDir(), "blueclaw-llmd")
	service := &Service{}

	if errorValue := service.installReleaseBinary(stagingPath, "blueclawLLMD", targetPath); errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.TrimSpace(readTrimmedFile(targetPath)) != "llmd binary" {
		t.Fatalf("LLMD binary was not installed at %s", targetPath)
	}
}

func TestInstallReleaseLLMDService(t *testing.T) {
	commands := []string{}
	service := &Service{RunCommand: func(_ context.Context, name string, arguments ...string) ([]byte, error) {
		commands = append(commands, strings.Join(append([]string{name}, arguments...), " "))
		return nil, nil
	}}
	manifest := testReleaseManifest("release-1")
	manifest.Components["blueclawLLMD"] = releaseset.Component{Name: "blueclawLLMD"}

	if errorValue := service.installReleaseLLMDService(context.Background(), manifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedCommands := strings.Join(commands, "\n")
	for _, expectedValue := range []string{blueclawruntime.LLMDServicePath, blueclawruntime.LLMDAuthKeyPath, blueclawruntime.LLMDServiceCredentialDirectoryPath, blueclawruntime.LLMDServiceAuthKeyPath, "head -c 32 /dev/urandom", "chmod 600", "DynamicUser=yes", "systemctl daemon-reload", "systemctl enable " + blueclawruntime.LLMDServiceName} {
		if !strings.Contains(joinedCommands, expectedValue) {
			t.Fatalf("expected LLMD service installation to contain %q, got %s", expectedValue, joinedCommands)
		}
	}
}

func TestInstallReleaseLLMDServiceWithholdsLlamaEnvironmentWhenLocalLlamaIsNotInstalled(t *testing.T) {
	commands := []string{}
	service := &Service{RunCommand: func(_ context.Context, name string, arguments ...string) ([]byte, error) {
		commands = append(commands, strings.Join(append([]string{name}, arguments...), " "))
		if name == "systemctl" && len(arguments) == 2 && arguments[0] == "cat" && arguments[1] == locallm.LlamaCppServiceName {
			return nil, errors.New("unit not found")
		}
		return nil, nil
	}}
	manifest := testReleaseManifest("release-no-local-llama")
	manifest.Components["blueclawLLMD"] = releaseset.Component{Name: "blueclawLLMD"}

	if errorValue := service.installReleaseLLMDService(context.Background(), manifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedCommands := strings.Join(commands, "\n")
	for _, unexpectedValue := range []string{
		"Environment=BLUECLAW_LLMD_LLAMA_BASE_URL=",
		"Environment=BLUECLAW_LLMD_LLAMA_MODEL=",
		"Environment=BLUECLAW_LLMD_LLAMA_STRUCTURED_OUTPUTS_ENABLED=",
	} {
		if strings.Contains(joinedCommands, unexpectedValue) {
			t.Fatalf("expected LLMD install without a provisioned local llama service to omit %q, got %s", unexpectedValue, joinedCommands)
		}
	}
}

func TestInstallReleaseLLMDServiceEmitsLlamaEnvironmentWhenLocalLlamaIsInstalled(t *testing.T) {
	commands := []string{}
	service := &Service{RunCommand: func(_ context.Context, name string, arguments ...string) ([]byte, error) {
		commands = append(commands, strings.Join(append([]string{name}, arguments...), " "))
		if name == "systemctl" && len(arguments) == 2 && arguments[0] == "cat" && arguments[1] == locallm.LlamaCppServiceName {
			return []byte("ExecStart=" + locallm.LlamaCppBinaryPath + " -m /root/.internkim/models/model.gguf"), nil
		}
		return nil, nil
	}}
	manifest := testReleaseManifest("release-with-local-llama")
	manifest.Components["blueclawLLMD"] = releaseset.Component{Name: "blueclawLLMD"}

	if errorValue := service.installReleaseLLMDService(context.Background(), manifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedCommands := strings.Join(commands, "\n")
	for _, expectedValue := range []string{
		"Environment=BLUECLAW_LLMD_LLAMA_BASE_URL=",
		"Environment=BLUECLAW_LLMD_LLAMA_MODEL=",
		"Environment=BLUECLAW_LLMD_LLAMA_STRUCTURED_OUTPUTS_ENABLED=true",
	} {
		if !strings.Contains(joinedCommands, expectedValue) {
			t.Fatalf("expected LLMD install with a provisioned local llama service to contain %q, got %s", expectedValue, joinedCommands)
		}
	}
}

func TestInstallReleaseLLMDServicePreservesLocalOnlyPolicy(t *testing.T) {
	runtimeConfigurationPath := filepath.Join(t.TempDir(), "runtime.json")
	writeFile(t, runtimeConfigurationPath, `{"languageModel":{"llmd":{"localOnly":true}}}`)
	commands := []string{}
	service := &Service{
		Configuration: Configuration{BlueclawRuntimeConfigPath: runtimeConfigurationPath},
		RunCommand: func(_ context.Context, name string, arguments ...string) ([]byte, error) {
			commands = append(commands, strings.Join(append([]string{name}, arguments...), " "))
			return nil, nil
		},
	}
	manifest := testReleaseManifest("release-local-only")
	manifest.Components["blueclawLLMD"] = releaseset.Component{Name: "blueclawLLMD"}

	if errorValue := service.installReleaseLLMDService(context.Background(), manifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedCommands := strings.Join(commands, "\n")
	for _, expectedValue := range []string{"Environment=BLUECLAW_LLMD_LOCAL_ONLY=1", "IPAddressDeny=any", "IPAddressAllow=localhost", "rm -f " + blueclawruntime.LLMDServiceOpenRouterKeyPath} {
		if !strings.Contains(joinedCommands, expectedValue) {
			t.Fatalf("expected local-only LLMD install to contain %q, got %s", expectedValue, joinedCommands)
		}
	}
	if strings.Contains(joinedCommands, "LoadCredential=-openrouter-api-key:") || strings.Contains(joinedCommands, "install -o root -g root -m 600 "+blueclawruntime.OpenRouterKeyPath) {
		t.Fatalf("expected local-only LLMD install to withhold remote credentials, got %s", joinedCommands)
	}
}

func TestInstallReleaseLLMDServiceUsesPersistedRoutingLocalOnlyPolicy(t *testing.T) {
	runtimeConfigurationPath := filepath.Join(t.TempDir(), "runtime.json")
	writeFile(t, runtimeConfigurationPath, `{"capabilities":{"routing":{"localOnly":true}}}`)
	commands := []string{}
	service := &Service{
		Configuration: Configuration{BlueclawRuntimeConfigPath: runtimeConfigurationPath},
		RunCommand: func(_ context.Context, name string, arguments ...string) ([]byte, error) {
			commands = append(commands, strings.Join(append([]string{name}, arguments...), " "))
			return nil, nil
		},
	}
	manifest := testReleaseManifest("release-routing-local-only")
	manifest.Components["blueclawLLMD"] = releaseset.Component{Name: "blueclawLLMD"}

	if errorValue := service.installReleaseLLMDService(context.Background(), manifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedCommands := strings.Join(commands, "\n")
	if !strings.Contains(joinedCommands, "Environment=BLUECLAW_LLMD_LOCAL_ONLY=1") || strings.Contains(joinedCommands, "LoadCredential=-openrouter-api-key:") {
		t.Fatalf("expected persisted routing policy to keep OTA LLMD local-only, got %s", joinedCommands)
	}
}

func TestReconcileReleaseLLMDBootstrapInstallsComponentIgnoredByOldAdmind(t *testing.T) {
	stateDirectoryPath := t.TempDir()
	stagingPath := filepath.Join(stateDirectoryPath, "release-updates", "staging", "old-admind-job")
	componentPath := filepath.Join(stagingPath, "blueclawLLMD")
	if errorValue := os.MkdirAll(filepath.Join(componentPath, "blueclaw-llmd"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(componentPath, "blueclaw-llmd", "blueclaw-llmd"), "llmd binary")
	archivePath := filepath.Join(componentPath, "component.tar.gz")
	writeFile(t, archivePath, "downloaded LLMD archive")
	runtimeConfigurationPath := filepath.Join(t.TempDir(), "runtime.json")
	writeFile(t, runtimeConfigurationPath, `{"languageModel":{"llmd":{"localOnly":true}}}`)
	manifest := testReleaseManifest("release-from-old-admind")
	manifest.Components["blueclawLLMD"] = releaseset.Component{Name: "blueclawLLMD", SHA256: fileSHA256(archivePath)}
	manifest.Components["capabilityd"] = releaseset.Component{Name: "capabilityd"}
	commands := []string{}
	service := &Service{
		Configuration: Configuration{StateDirectory: stateDirectoryPath, BlueclawRuntimeConfigPath: runtimeConfigurationPath},
		RunCommand: func(_ context.Context, name string, arguments ...string) ([]byte, error) {
			command := strings.Join(append([]string{name}, arguments...), " ")
			commands = append(commands, command)
			if name == "systemctl" && len(arguments) > 0 && arguments[0] == "is-active" {
				return []byte("inactive\n"), nil
			}
			if name == "sh" && strings.Contains(command, blueclawruntime.LLMDHealthCheckCommand()) {
				return []byte("ok\n"), nil
			}
			return nil, nil
		},
	}
	if errorValue := service.writeCurrentReleaseManifest(manifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	targetPath := filepath.Join(t.TempDir(), "blueclaw-llmd")

	if errorValue := service.reconcileReleaseLLMDBootstrapAtPath(context.Background(), targetPath); errorValue != nil {
		t.Fatal(errorValue)
	}
	if readTrimmedFile(targetPath) != "llmd binary" {
		t.Fatalf("expected ignored LLMD component to be installed at %s", targetPath)
	}
	if readTrimmedFile(service.installedReleaseLLMDPath()) != manifest.ReleaseID {
		t.Fatalf("expected LLMD release receipt for %s", manifest.ReleaseID)
	}
	joinedCommands := strings.Join(commands, "\n")
	for _, expectedValue := range []string{"systemctl restart " + blueclawruntime.LLMDServiceName, "systemctl restart " + blueclawruntime.CapabilitydServiceName, "Environment=BLUECLAW_LLMD_LOCAL_ONLY=1", blueclawruntime.LLMDServiceAuthKeyPath} {
		if !strings.Contains(joinedCommands, expectedValue) {
			t.Fatalf("expected old-admind LLMD reconciliation to contain %q, got %s", expectedValue, joinedCommands)
		}
	}
	credentialInstallIndex := strings.Index(joinedCommands, "install -o root -g root -m 600 "+blueclawruntime.LLMDAuthKeyPath+" "+blueclawruntime.LLMDServiceAuthKeyPath)
	serviceRestartIndex := strings.Index(joinedCommands, "systemctl restart "+blueclawruntime.LLMDServiceName)
	if credentialInstallIndex < 0 || serviceRestartIndex < 0 || credentialInstallIndex > serviceRestartIndex {
		t.Fatalf("expected OTA LLMD credentials to be staged before service activation, got %s", joinedCommands)
	}
}

func TestRestartReleaseLLMDChecksHealth(t *testing.T) {
	commands := []string{}
	service := &Service{RunCommand: func(_ context.Context, name string, arguments ...string) ([]byte, error) {
		command := strings.Join(append([]string{name}, arguments...), " ")
		commands = append(commands, command)
		if name == "sh" {
			return []byte("ok\n"), nil
		}
		return nil, nil
	}}

	if errorValue := service.restartReleaseLLMD(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedCommands := strings.Join(commands, "\n")
	if !strings.Contains(joinedCommands, "systemctl restart "+blueclawruntime.LLMDServiceName) {
		t.Fatalf("expected LLMD restart, got %s", joinedCommands)
	}
	if !strings.Contains(joinedCommands, blueclawruntime.LLMDHealthCheckCommand()) {
		t.Fatalf("expected LLMD health check, got %s", joinedCommands)
	}
}

func TestReleaseWebComponentRootFallsBackToAdminWeb(t *testing.T) {
	stagingPath := t.TempDir()
	legacyRoot := filepath.Join(stagingPath, "adminWeb", "legacy-board-ui")
	if errorValue := os.MkdirAll(legacyRoot, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}

	componentRoot, errorValue := (&Service{}).releaseWebComponentRoot(stagingPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if componentRoot != legacyRoot {
		t.Fatalf("component root = %q, want %q", componentRoot, legacyRoot)
	}
}

func TestReleaseCapabilitydServiceNamesUseTenantServices(t *testing.T) {
	tenantBasePath := t.TempDir()
	writeReleaseTenantRuntimeConfiguration(t, tenantBasePath, "pilot-02")
	writeReleaseTenantRuntimeConfiguration(t, tenantBasePath, "pilot-01")

	serviceNames := releaseCapabilitydServiceNames(tenantBasePath)
	expectedServiceNames := []string{
		"internkim-capabilityd",
		"internkim-tenant-capabilityd-pilot-01.service",
		"internkim-tenant-capabilityd-pilot-02.service",
	}
	if strings.Join(serviceNames, "\n") != strings.Join(expectedServiceNames, "\n") {
		t.Fatalf("service names = %+v, want %+v", serviceNames, expectedServiceNames)
	}
}

func TestReleaseCapabilitydServiceNamesFallbackToDeviceService(t *testing.T) {
	serviceNames := releaseCapabilitydServiceNames(t.TempDir())
	expectedServiceNames := []string{"internkim-capabilityd"}
	if strings.Join(serviceNames, "\n") != strings.Join(expectedServiceNames, "\n") {
		t.Fatalf("service names = %+v, want %+v", serviceNames, expectedServiceNames)
	}
}

func TestReleaseAdmindServiceNamesUseTenantServices(t *testing.T) {
	tenantBasePath := t.TempDir()
	writeReleaseTenantRuntimeConfiguration(t, tenantBasePath, "pilot-02")
	writeReleaseTenantRuntimeConfiguration(t, tenantBasePath, "pilot-01")

	serviceNames := releaseAdmindServiceNames(tenantBasePath)
	expectedServiceNames := []string{
		"internkim-admind",
		"internkim-tenant-admind-pilot-01.service",
		"internkim-tenant-admind-pilot-02.service",
	}
	if strings.Join(serviceNames, "\n") != strings.Join(expectedServiceNames, "\n") {
		t.Fatalf("service names = %+v, want %+v", serviceNames, expectedServiceNames)
	}
}

func TestReleaseAdmindServiceNamesFallbackToDeviceService(t *testing.T) {
	serviceNames := releaseAdmindServiceNames(t.TempDir())
	expectedServiceNames := []string{"internkim-admind"}
	if strings.Join(serviceNames, "\n") != strings.Join(expectedServiceNames, "\n") {
		t.Fatalf("service names = %+v, want %+v", serviceNames, expectedServiceNames)
	}
}

func TestFetchReleaseStablePointerUsesDownloadToken(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "release-download-token")
	writeFile(t, tokenPath, "download-token")
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-InternKim-Release-Token") != "download-token" {
			t.Fatalf("release token header = %q", request.Header.Get("X-InternKim-Release-Token"))
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"releaseID":"release-1","manifestURL":"https://updates.test/releases/release-1/manifest.json","updatedAt":"2026-06-09T00:00:00Z"}`))
	}))
	defer server.Close()
	service := NewService(Configuration{
		ReleaseRegistryURL:          server.URL,
		ReleaseDownloadTokenPath:    tokenPath,
		ReleaseSigningKeyPath:       writeTestFile(t, ""),
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		MattermostAdminPasswordPath: writeTestFile(t, "password"),
		MattermostTokenPath:         writeTestFile(t, "bot-token"),
		MattermostOAuthClientPath:   writeTestFile(t, "{}"),
		OpenRouterKeyPath:           writeTestFile(t, "openrouter"),
		MattermostBotTokenPath:      writeTestFile(t, "bot-token"),
		FleetIDPath:                 writeTestFile(t, "fleet-1"),
		DeviceURLPath:               writeTestFile(t, "https://fleet-1.example.test"),
		FleetSecretPath:             writeTestFile(t, "secret"),
		BlueclawRuntimeConfigPath:   writeTestFile(t, "{}"),
		CalendarSecretsDirectory:    t.TempDir(),
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		FlowDatabasePath:            filepath.Join(t.TempDir(), "flow.sqlite"),
		CalendarDatabasePath:        filepath.Join(t.TempDir(), "calendar.sqlite"),
		MailDatabasePath:            filepath.Join(t.TempDir(), "mail.sqlite"),
		AttendanceDatabasePath:      filepath.Join(t.TempDir(), "attendance.sqlite"),
		AdminUIPath:                 t.TempDir(),
		CompanionFileDirectory:      t.TempDir(),
		SitesRoot:                   t.TempDir(),
		SiteSecretDirectory:         t.TempDir(),
		BotProfilePath:              writeTestFile(t, "{}"),
		BotProfileImagePath:         writeTestFile(t, "image"),
		BlueclawWorkspacePath:       t.TempDir(),
	})

	pointer, errorValue := service.fetchReleaseStablePointer(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if pointer.ReleaseID != "release-1" {
		t.Fatalf("release id = %q", pointer.ReleaseID)
	}
}

func TestReleaseHistoryEndpointFallsBackToStablePointer(t *testing.T) {
	service := NewService(Configuration{
		ReleaseRegistryURL:     "https://updates.test",
		StateDirectory:         t.TempDir(),
		ReleaseSigningKeyPath:  writeTestFile(t, ""),
		AdminEmailPath:         writeTestFile(t, "admin@example.com"),
		BlueclawWorkspacePath:  t.TempDir(),
		CompanionJobPath:       filepath.Join(t.TempDir(), "jobs.json"),
		CompanionFileDirectory: t.TempDir(),
	})
	currentManifest := testReleaseManifest("release-1")
	if errorValue := service.writeCurrentReleaseManifest(currentManifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/channels/stable-history.json" {
			return testHTTPResponse(http.StatusNotFound, nil), nil
		}
		if request.URL.Path == "/channels/stable.json" {
			return testJSONHTTPResponse(t, releaseset.StablePointer{
				ReleaseID:   "release-1",
				ManifestURL: "https://updates.test/releases/release-1/manifest.json",
				UpdatedAt:   "2026-06-12T00:00:00Z",
			}), nil
		}
		return testHTTPResponse(http.StatusNotFound, nil), nil
	})}

	request := httptest.NewRequest(http.MethodGet, "/admin/api/updates/releases", nil)
	response := httptest.NewRecorder()
	service.handleAdmin(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	var payload releaseHistoryResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(payload.Entries) != 1 || payload.Entries[0].ReleaseID != "release-1" || !payload.Entries[0].IsCurrent {
		t.Fatalf("entries = %+v", payload.Entries)
	}
}

func TestApplyReleaseUpdateWithReleaseIDUsesHistoryManifest(t *testing.T) {
	blobDocument, blobSHA256, blobSize := testReleaseBlobDocument(t)
	service := NewService(Configuration{
		ReleaseRegistryURL:     "https://updates.test",
		StateDirectory:         t.TempDir(),
		ReleaseSigningKeyPath:  writeTestFile(t, ""),
		AdminEmailPath:         writeTestFile(t, "admin@example.com"),
		BlueclawWorkspacePath:  t.TempDir(),
		CompanionJobPath:       filepath.Join(t.TempDir(), "jobs.json"),
		CompanionFileDirectory: t.TempDir(),
	})
	releaseOne := testReleaseManifestWithBlob("release-1", blobSHA256, blobSize)
	releaseTwo := testReleaseManifestWithBlob("release-2", blobSHA256, blobSize)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/channels/stable-history.json":
			return testJSONHTTPResponse(t, releaseset.ChannelHistory{
				Channel: "stable",
				Entries: []releaseset.ChannelHistoryEntry{
					{ReleaseID: "release-2", ManifestURL: "https://updates.test/releases/release-2/manifest.json", CreatedAt: "2026-06-12T00:00:00Z"},
					{ReleaseID: "release-1", ManifestURL: "https://updates.test/releases/release-1/manifest.json", CreatedAt: "2026-06-11T00:00:00Z"},
				},
				UpdatedAt: "2026-06-12T00:00:00Z",
			}), nil
		case "/releases/release-1/manifest.json":
			return testJSONHTTPResponse(t, releaseOne), nil
		case "/releases/release-2/manifest.json":
			return testJSONHTTPResponse(t, releaseTwo), nil
		case "/blobs/sha256/" + blobSHA256:
			return testHTTPResponse(http.StatusOK, blobDocument), nil
		default:
			return testHTTPResponse(http.StatusNotFound, nil), nil
		}
	})}
	if errorValue := service.writeCurrentReleaseManifest(&releaseTwo); errorValue != nil {
		t.Fatal(errorValue)
	}

	document, errorValue := json.Marshal(map[string]string{"releaseID": "release-1"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/admin/api/updates/apply", bytes.NewReader(document))
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	service.handleAdmin(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	waitForReleaseJob(t, service)
	current := service.readCurrentReleaseManifest()
	if current == nil || current.ReleaseID != "release-1" {
		t.Fatalf("current release = %+v", current)
	}
}

func writeReleaseTenantRuntimeConfiguration(t *testing.T, tenantBasePath string, tenantID string) {
	t.Helper()
	runtimeConfigurationPath := filepath.Join(tenantBasePath, tenantID, "blueclaw", "config", "runtime.json")
	if errorValue := os.MkdirAll(filepath.Dir(runtimeConfigurationPath), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, runtimeConfigurationPath, "{}")
}

func testReleaseManifest(releaseID string) *releaseset.Manifest {
	manifest := releaseset.NewManifest(releaseID, "stable", map[string]releaseset.Component{
		"blueclawPayload": {
			Name:         "blueclawPayload",
			Revision:     releaseID,
			SHA256:       "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Size:         1,
			BlobPath:     "blobs/sha256/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			RestartGroup: "blueclaw",
			HealthCheck:  "blueclaw",
		},
	})
	return &manifest
}

func testReleaseManifestWithBlob(releaseID string, blobSHA256 string, blobSize int64) releaseset.Manifest {
	return releaseset.NewManifest(releaseID, "stable", map[string]releaseset.Component{
		"artifact": {
			Name:         "artifact",
			Revision:     releaseID,
			SHA256:       blobSHA256,
			Size:         blobSize,
			BlobPath:     "blobs/sha256/" + blobSHA256,
			RestartGroup: "admind",
			HealthCheck:  "binary",
		},
	})
}

func testReleaseBlobDocument(t *testing.T) ([]byte, string, int64) {
	t.Helper()
	directoryPath := t.TempDir()
	sourcePath := filepath.Join(directoryPath, "artifact")
	if errorValue := os.MkdirAll(sourcePath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(sourcePath, "payload.txt"), "release payload\n")
	archivePath := filepath.Join(directoryPath, "artifact.tar.gz")
	writeTestTarGzipDirectory(t, archivePath, sourcePath)
	document, errorValue := os.ReadFile(archivePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return document, fileSHA256(archivePath), fileSize(t, archivePath)
}

func testJSONHTTPResponse(t *testing.T, payload any) *http.Response {
	t.Helper()
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return testHTTPResponse(http.StatusOK, document)
}

func testHTTPResponse(statusCode int, document []byte) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(document)),
	}
}

func newReleaseUpdateUploadTestService(t *testing.T) *Service {
	t.Helper()
	directoryPath := t.TempDir()
	fleetIDPath := filepath.Join(directoryPath, "fleet-id")
	fleetSecretPath := filepath.Join(directoryPath, "fleet-secret")
	releaseSigningKeyPath := filepath.Join(directoryPath, "release-signing-key")
	writeFile(t, fleetIDPath, "fleet-1")
	writeFile(t, fleetSecretPath, "fleet-secret-1")
	writeFile(t, releaseSigningKeyPath, "release-secret-1")
	service := NewService(Configuration{
		StateDirectory:            filepath.Join(directoryPath, "state"),
		FleetIDPath:               fleetIDPath,
		FleetSecretPath:           fleetSecretPath,
		ReleaseSigningKeyPath:     releaseSigningKeyPath,
		BlueclawWorkspacePath:     filepath.Join(directoryPath, "blueclaw-workspace"),
		AdminEmailPath:            writeTestFile(t, "admin@example.com"),
		BlueclawRuntimeConfigPath: filepath.Join(directoryPath, "runtime.json"),
	})
	service.RunCommand = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("ok\n"), nil
	}
	currentManifest := releaseset.NewManifest("release-current", "stable", map[string]releaseset.Component{
		"skills": {Name: "skills"},
	})
	if errorValue := service.writeCurrentReleaseManifest(&currentManifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	return service
}

func writeTestReleaseBundle(t *testing.T, service *Service) (string, releaseset.Manifest) {
	t.Helper()
	directoryPath := t.TempDir()
	skillsSourcePath := filepath.Join(directoryPath, "skills")
	skillSourcePath := filepath.Join(skillsSourcePath, "test-skill")
	if errorValue := os.MkdirAll(skillSourcePath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(skillSourcePath, "SKILL.md"), "test skill\n")
	componentArchivePath := filepath.Join(directoryPath, "skills.tar.gz")
	writeTestTarGzipDirectory(t, componentArchivePath, skillsSourcePath)
	componentSHA256 := fileSHA256(componentArchivePath)
	componentSize := fileSize(t, componentArchivePath)
	manifest, errorValue := releaseset.NewManifest("release-upload-test", "stable", map[string]releaseset.Component{
		"skills": {
			Name:         "skills",
			Revision:     "test",
			SHA256:       componentSHA256,
			Size:         componentSize,
			BlobPath:     "blobs/skills.tar.gz",
			RestartGroup: "blueclaw",
			HealthCheck:  "skills",
		},
	}).Sign(strings.TrimSpace(readTrimmedFile(service.Configuration.ReleaseSigningKeyPath)))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	bundlePath := filepath.Join(directoryPath, "release.tar.gz")
	writeTestReleaseBundleArchive(t, bundlePath, manifest, componentArchivePath)
	return bundlePath, manifest
}

func writeTestReleaseBundleArchive(t *testing.T, bundlePath string, manifest releaseset.Manifest, componentArchivePath string) {
	t.Helper()
	file, errorValue := os.Create(bundlePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	manifestDocument, errorValue := json.Marshal(manifest)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := writeTestTarBytes(tarWriter, "manifest.json", manifestDocument); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := writeTestTarFile(tarWriter, "blobs/skills.tar.gz", componentArchivePath); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, closeError := range []error{tarWriter.Close(), gzipWriter.Close(), file.Close()} {
		if closeError != nil {
			t.Fatal(closeError)
		}
	}
}

func writeTestTarGzipDirectory(t *testing.T, archivePath string, sourcePath string) {
	t.Helper()
	file, errorValue := os.Create(archivePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	if errorValue := filepath.WalkDir(sourcePath, func(path string, entry os.DirEntry, errorValue error) error {
		if errorValue != nil {
			return errorValue
		}
		if path == sourcePath {
			return nil
		}
		relativePath, errorValue := filepath.Rel(filepath.Dir(sourcePath), path)
		if errorValue != nil {
			return errorValue
		}
		if entry.IsDir() {
			return writeTestTarBytes(tarWriter, filepath.ToSlash(relativePath)+"/", nil)
		}
		return writeTestTarFile(tarWriter, filepath.ToSlash(relativePath), path)
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, closeError := range []error{tarWriter.Close(), gzipWriter.Close(), file.Close()} {
		if closeError != nil {
			t.Fatal(closeError)
		}
	}
}

func writeTestTarBytes(writer *tar.Writer, name string, document []byte) error {
	header := &tar.Header{Name: name, Mode: 0o600, Size: int64(len(document))}
	if strings.HasSuffix(name, "/") {
		header.Typeflag = tar.TypeDir
		header.Mode = 0o700
	}
	if errorValue := writer.WriteHeader(header); errorValue != nil {
		return errorValue
	}
	if len(document) == 0 {
		return nil
	}
	_, errorValue := writer.Write(document)
	return errorValue
}

func writeTestTarFile(writer *tar.Writer, name string, path string) error {
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		return errorValue
	}
	header, errorValue := tar.FileInfoHeader(information, "")
	if errorValue != nil {
		return errorValue
	}
	header.Name = name
	if errorValue := writer.WriteHeader(header); errorValue != nil {
		return errorValue
	}
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	_, errorValue = file.WriteTo(writer)
	return errorValue
}

func TestReleaseUpdateUploadResumesReceivedChunks(t *testing.T) {
	service := newReleaseUpdateUploadTestService(t)
	bundlePath, manifest := writeTestReleaseBundle(t, service)
	bundleSHA256 := fileSHA256(bundlePath)
	bundleSize := fileSize(t, bundlePath)

	createPayload := releaseUpdateUploadCreateRequest{
		fleetSignedRequest: signedTestFleetRequest(t, service, releaseUpdateUploadAction, "nonce-1"),
		ReleaseID:          manifest.ReleaseID,
		Filename:           "release.tar.gz",
		Size:               bundleSize,
		SHA256:             bundleSHA256,
	}
	var firstUpload releaseUpdateUploadCreateResponse
	firstResponse := performReleaseUploadJSON(t, service, http.MethodPost, "/updates/uploads", createPayload, "")
	if errorValue := json.NewDecoder(firstResponse.Body).Decode(&firstUpload); errorValue != nil {
		t.Fatal(errorValue)
	}

	document, errorValue := os.ReadFile(bundlePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	chunkRequest := httptest.NewRequest(http.MethodPut, "/admin/api/updates/uploads/"+firstUpload.UploadID+"/chunks/0", bytes.NewReader(document))
	chunkRequest.Header.Set("X-InternKim-Upload-Token", firstUpload.UploadToken)
	service.handleAdmin(httptest.NewRecorder(), chunkRequest)

	resumePayload := createPayload
	resumePayload.fleetSignedRequest = signedTestFleetRequest(t, service, releaseUpdateUploadAction, "nonce-resume")
	var resumeUpload releaseUpdateUploadCreateResponse
	resumeResponse := performReleaseUploadJSON(t, service, http.MethodPost, "/updates/uploads", resumePayload, "")
	if errorValue := json.NewDecoder(resumeResponse.Body).Decode(&resumeUpload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if resumeUpload.UploadID != firstUpload.UploadID {
		t.Fatalf("resume returned new session %q, want reuse of %q", resumeUpload.UploadID, firstUpload.UploadID)
	}
	if len(resumeUpload.ReceivedChunks) != 1 || resumeUpload.ReceivedChunks[0] != 0 {
		t.Fatalf("resume received chunks = %v, want [0]", resumeUpload.ReceivedChunks)
	}
}

func performReleaseUploadJSON(t *testing.T, service *Service, method string, path string, payload any, uploadToken string) *httptest.ResponseRecorder {
	t.Helper()
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(method, "/admin/api"+path, bytes.NewReader(document))
	request.Header.Set("Content-Type", "application/json")
	if uploadToken != "" {
		request.Header.Set("X-InternKim-Upload-Token", uploadToken)
	}
	response := httptest.NewRecorder()
	service.handleAdmin(response, request)
	return response
}

func signedTestFleetRequest(t *testing.T, service *Service, action string, nonce string) fleetSignedRequest {
	t.Helper()
	timestamp := time.Now().UTC().Format(time.RFC3339)
	deviceID := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath))
	secret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	return fleetSignedRequest{
		Action:    action,
		DeviceID:  deviceID,
		Nonce:     nonce,
		Timestamp: timestamp,
		Signature: signFleetPayload(secret, action, deviceID, nonce, timestamp),
	}
}

func waitForReleaseJob(t *testing.T, service *Service) {
	t.Helper()
	for attempt := 0; attempt < 100; attempt++ {
		service.mutex.Lock()
		for _, job := range service.jobs {
			if job.Type == "release-update" && (job.Status == "completed" || job.Status == "failed") {
				service.mutex.Unlock()
				if job.Status == "failed" {
					t.Fatalf("release job failed at %s: %s", job.Phase, job.Error)
				}
				return
			}
		}
		service.mutex.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("release job did not finish")
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return information.Size()
}
