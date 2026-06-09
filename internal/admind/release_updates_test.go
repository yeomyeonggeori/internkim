package admind

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/releaseset"
)

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
		DeviceURLPath:               writeTestFile(t, "https://fleet-1.intern.kim"),
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
