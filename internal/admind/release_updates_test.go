package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

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
