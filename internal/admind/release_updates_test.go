package admind

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/releaseset"
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

func TestWaitForCapabilitydProtocolIdentityAcceptsExpectedRegistry(t *testing.T) {
	expectedIdentity := capabilityprotocol.GeneratedProtocolIdentity()
	capabilitySocketPath := startReleaseCapabilityRegistryServer(t, expectedIdentity)
	service := Service{Configuration: Configuration{CapabilitySocketPath: capabilitySocketPath}}

	if errorValue := service.waitForCapabilitydProtocolIdentity(context.Background(), expectedIdentity); errorValue != nil {
		t.Fatalf("expected capabilityd readiness: %v", errorValue)
	}
}

func TestWaitForCapabilitydProtocolIdentityReportsMismatch(t *testing.T) {
	expectedIdentity := capabilityprotocol.GeneratedProtocolIdentity()
	receivedIdentity := expectedIdentity
	receivedIdentity.AggregateProtocolHash = strings.Repeat("1a", 32)
	capabilitySocketPath := startReleaseCapabilityRegistryServer(t, receivedIdentity)
	service := Service{Configuration: Configuration{CapabilitySocketPath: capabilitySocketPath}}

	errorValue := service.checkCapabilitydProtocolIdentity(context.Background(), expectedIdentity)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "capabilityd protocol identity mismatch") {
		t.Fatalf("expected mismatch evidence, got %v", errorValue)
	}
}

func TestWaitForCapabilitydProtocolIdentityRespectsCancellation(t *testing.T) {
	readinessContext, cancel := context.WithCancel(context.Background())
	cancel()
	service := Service{Configuration: Configuration{CapabilitySocketPath: filepath.Join(t.TempDir(), "absent.sock")}}

	errorValue := service.waitForCapabilitydProtocolIdentity(readinessContext, capabilityprotocol.GeneratedProtocolIdentity())
	if !errors.Is(errorValue, context.Canceled) {
		t.Fatalf("expected canceled readiness, got %v", errorValue)
	}
}

func TestInstallReleaseComponentsWaitsForCapabilitydBeforeInstallingWeb(t *testing.T) {
	adminUIPath := filepath.Join(t.TempDir(), "admin-ui")
	stagingPath := t.TempDir()
	webRoot := filepath.Join(stagingPath, "web", "release")
	if errorValue := os.MkdirAll(webRoot, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	markerPath := filepath.Join(webRoot, "ready.txt")
	if errorValue := os.WriteFile(markerPath, []byte("ready"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedIdentity := capabilityprotocol.GeneratedProtocolIdentity()
	var registryRequestCount atomic.Int32
	var webInstalledTooEarly atomic.Bool
	directoryPath, errorValue := os.MkdirTemp("/tmp", "ik-cap-")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directoryPath) })
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
		if registryRequestCount.Add(1) == 1 {
			if _, errorValue := os.Stat(adminUIPath); errorValue == nil {
				webInstalledTooEarly.Store(true)
			}
			http.Error(responseWriter, "capabilityd is starting", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{
			"protocolVersion":       expectedIdentity.ProtocolVersion,
			"aggregateProtocolHash": expectedIdentity.AggregateProtocolHash,
			"routingCandidates":     []string{},
		})
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
	})

	service := Service{Configuration: Configuration{CapabilitySocketPath: socketPath, AdminUIPath: adminUIPath}}
	service.RunCommand = func(context.Context, string, ...string) ([]byte, error) { return nil, nil }
	manifest := &releaseset.Manifest{ProtocolIdentity: expectedIdentity, Components: map[string]releaseset.Component{
		"capabilityd": {Name: "capabilityd"},
		"web":         {Name: "web"},
	}}
	if errorValue := service.installReleaseComponents(context.Background(), "job-1", manifest, stagingPath); errorValue != nil {
		t.Fatalf("install failed: %v", errorValue)
	}
	if registryRequestCount.Load() < 2 {
		t.Fatalf("expected readiness retry after transient registry failure, got %d requests", registryRequestCount.Load())
	}
	if webInstalledTooEarly.Load() {
		t.Fatal("web was installed before capabilityd served the expected registry")
	}
	if _, errorValue := os.Stat(filepath.Join(adminUIPath, "ready.txt")); errorValue != nil {
		t.Fatalf("web was not installed after capabilityd readiness: %v", errorValue)
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
	for _, componentName := range []string{"capabilityd", "blueclawPayload"} {
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

	if errorValue == nil || !strings.Contains(errorValue.Error(), "capabilityd, blueclawPayload") {
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
		if request.Header.Get("X-INTERNKIM-RELEASE-TOKEN") != "download-token" {
			t.Fatalf("release token header = %q", request.Header.Get("X-INTERNKIM-RELEASE-TOKEN"))
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"releaseID":"release-1","manifestURL":"https://updates.test/releases/release-1/manifest.json","updatedAt":"2026-06-09T00:00:00Z"}`))
	}))
	defer server.Close()
	service := NewService(Configuration{
		ReleaseRegistryURL:        server.URL,
		ReleaseDownloadTokenPath:  tokenPath,
		ReleaseSigningKeyPath:     writeTestFile(t, ""),
		AdminEmailPath:            writeTestFile(t, "admin@example.com"),
		OpenRouterKeyPath:         writeTestFile(t, "openrouter"),
		FleetIDPath:               writeTestFile(t, "fleet-1"),
		DeviceURLPath:             writeTestFile(t, "https://fleet-1.example.test"),
		FleetSecretPath:           writeTestFile(t, "secret"),
		BlueclawRuntimeConfigPath: writeTestFile(t, "{}"),
		StateDirectory:            t.TempDir(),
		CompanionJobPath:          filepath.Join(t.TempDir(), "jobs.json"),
		TaskDatabasePath:          filepath.Join(t.TempDir(), "flow.sqlite"),
		CalendarDatabasePath:      filepath.Join(t.TempDir(), "calendar.sqlite"),
		MailDatabasePath:          filepath.Join(t.TempDir(), "mail.sqlite"),
		AttendanceDatabasePath:    filepath.Join(t.TempDir(), "attendance.sqlite"),
		AdminUIPath:               t.TempDir(),
		CompanionFileDirectory:    t.TempDir(),
		SitesRoot:                 t.TempDir(),
		SiteSecretDirectory:       t.TempDir(),
		IdentityDocumentPath:      filepath.Join(t.TempDir(), "identity.json"),
		SoulDocumentPath:          filepath.Join(t.TempDir(), "soul.json"),
		BotProfileImagePath:       writeTestFile(t, "image"),
		BlueclawWorkspacePath:     t.TempDir(),
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
	service.RunCommand = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("ok\n"), nil
	}
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

func performReleaseUploadJSON(t *testing.T, service *Service, method string, path string, payload any, uploadToken string) *httptest.ResponseRecorder {
	t.Helper()
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(method, "/admin/api"+path, bytes.NewReader(document))
	request.Header.Set("Content-Type", "application/json")
	if uploadToken != "" {
		request.Header.Set("X-INTERNKIM-UPLOAD-TOKEN", uploadToken)
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
		Signature: signFleetPayload(secret, action, "", deviceID, nonce, timestamp),
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

func TestReleaseUpdateApplyReadsTheChannelItWasGiven(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/admin/api/updates/apply",
		strings.NewReader(`{"releaseID":"r-1","channel":"direct"}`))

	releaseID, channel, errorValue := decodeReleaseUpdateApplyRequest(request)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if releaseID != "r-1" || channel != "direct" {
		t.Fatalf("release %q channel %q", releaseID, channel)
	}
}

func TestReleaseUpdateApplyFallsBackToStable(t *testing.T) {
	for _, body := range []string{"", `{"releaseID":"r-1"}`} {
		request := httptest.NewRequest(http.MethodPost, "/admin/api/updates/apply", strings.NewReader(body))

		_, channel, errorValue := decodeReleaseUpdateApplyRequest(request)

		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if channel != "stable" {
			t.Fatalf("a request that names no channel takes stable, got %q for body %q", channel, body)
		}
	}
}

func TestSignedReleaseApplyAppliesTheReleaseAndChannelItNames(t *testing.T) {
	requestedPaths := &[]string{}
	service := newSignedReleaseUpdateTestService(t, requestedPaths)

	response := performSignedReleaseApply(t, service, "nonce-names-a-release", "release-1", "direct")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	waitForReleaseJob(t, service)
	current := service.readCurrentReleaseManifest()
	if current == nil || current.ReleaseID != "release-1" {
		t.Fatalf("current release = %+v", current)
	}
}

func TestSignedReleaseApplyCannotNameAReleaseTheChannelDoesNotCarry(t *testing.T) {
	requestedPaths := &[]string{}
	service := newSignedReleaseUpdateTestService(t, requestedPaths)

	response := performSignedReleaseApply(t, service, "nonce-names-a-foreign-release", "release-1", "stable")

	if response.Code != http.StatusBadRequest {
		t.Fatalf("a release the named channel does not list has to be refused, got %d body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "unknown releaseID") {
		t.Fatalf("body = %s", response.Body.String())
	}
	if current := service.readCurrentReleaseManifest(); current == nil || current.ReleaseID != "release-2" {
		t.Fatalf("current release = %+v", current)
	}
}

func TestSignedReleaseApplyRefusesAChannelThatIsNotAChannelName(t *testing.T) {
	for index, channel := range []string{"../stable", "direct/../../secrets", "https://evil.test/channels/direct"} {
		requestedPaths := &[]string{}
		service := newSignedReleaseUpdateTestService(t, requestedPaths)

		response := performSignedReleaseApply(t, service, fmt.Sprintf("nonce-refused-channel-%d", index), "release-1", channel)

		if response.Code != http.StatusBadRequest {
			t.Fatalf("channel %q has to be refused, got %d body = %s", channel, response.Code, response.Body.String())
		}
		for _, path := range *requestedPaths {
			if strings.Contains(path, "channels/") {
				t.Fatalf("channel %q reached the registry at %s", channel, path)
			}
		}
	}
}

func TestSignedReleaseApplyStillRefusesAnUnsignedRelease(t *testing.T) {
	requestedPaths := &[]string{}
	service := newSignedReleaseUpdateTestService(t, requestedPaths)
	document, errorValue := json.Marshal(releaseUpdateApplyRequest{
		fleetSignedRequest: fleetSignedRequest{
			Action:    "release-update-apply",
			DeviceID:  "dc719d8e",
			Nonce:     "nonce-without-a-signature",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Signature: "0000000000000000000000000000000000000000000000000000000000000000",
		},
		ReleaseID: "release-1",
		Channel:   "direct",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	request := httptest.NewRequest(http.MethodPost, "/admin/api/updates/apply", bytes.NewReader(document))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	service.handleAdmin(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func performSignedReleaseApply(t *testing.T, service *Service, nonce string, releaseID string, channel string) *httptest.ResponseRecorder {
	t.Helper()
	document, errorValue := json.Marshal(releaseUpdateApplyRequest{
		fleetSignedRequest: signedTestFleetRequest(t, service, "release-update-apply", nonce),
		ReleaseID:          releaseID,
		Channel:            channel,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/admin/api/updates/apply", bytes.NewReader(document))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	service.handleAdmin(response, request)
	return response
}

func newSignedReleaseUpdateTestService(t *testing.T, requestedPaths *[]string) *Service {
	t.Helper()
	blobDocument, blobSHA256, blobSize := testReleaseBlobDocument(t)
	service := NewService(Configuration{
		ReleaseRegistryURL:     "https://updates.test",
		StateDirectory:         t.TempDir(),
		ReleaseSigningKeyPath:  writeTestFile(t, ""),
		AdminEmailPath:         writeTestFile(t, "admin@example.com"),
		FleetIDPath:            writeTestFile(t, "dc719d8e"),
		FleetSecretPath:        writeTestFile(t, "secret-value"),
		BlueclawWorkspacePath:  t.TempDir(),
		CompanionJobPath:       filepath.Join(t.TempDir(), "jobs.json"),
		CompanionFileDirectory: t.TempDir(),
	})
	service.RunCommand = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("ok\n"), nil
	}
	releaseOne := testReleaseManifestWithBlob("release-1", blobSHA256, blobSize)
	releaseTwo := testReleaseManifestWithBlob("release-2", blobSHA256, blobSize)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		*requestedPaths = append(*requestedPaths, request.URL.Path)
		switch request.URL.Path {
		case "/channels/direct-history.json":
			return testJSONHTTPResponse(t, testReleaseChannelHistory("direct", "release-1")), nil
		case "/channels/stable-history.json":
			return testJSONHTTPResponse(t, testReleaseChannelHistory("stable", "release-2")), nil
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
	return service
}

func testReleaseChannelHistory(channel string, releaseID string) releaseset.ChannelHistory {
	return releaseset.ChannelHistory{
		Channel: channel,
		Entries: []releaseset.ChannelHistoryEntry{{
			ReleaseID:   releaseID,
			ManifestURL: "https://updates.test/releases/" + releaseID + "/manifest.json",
			CreatedAt:   "2026-06-12T00:00:00Z",
		}},
		UpdatedAt: "2026-06-12T00:00:00Z",
	}
}
