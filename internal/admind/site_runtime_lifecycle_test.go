package admind

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func listenOnEphemeralPort(t *testing.T) (net.Listener, int) {
	listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return listener, listener.Addr().(*net.TCPAddr).Port
}

func TestEnsureSitePocketBaseRunningStartsUnitOnce(t *testing.T) {
	listener, port := listenOnEphemeralPort(t)
	defer listener.Close()
	service := &Service{}
	commands := []string{}
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		commands = append(commands, strings.Join(append([]string{name}, arguments...), " "))
		return nil, nil
	}
	site := &SiteRecord{SiteID: "site-1", Port: port}

	if errorValue := service.ensureSitePocketBaseRunning(context.Background(), site); errorValue != nil {
		t.Fatalf("first ensure failed: %v", errorValue)
	}
	if errorValue := service.ensureSitePocketBaseRunning(context.Background(), site); errorValue != nil {
		t.Fatalf("second ensure failed: %v", errorValue)
	}

	startCount := 0
	for _, command := range commands {
		if strings.Contains(command, "systemctl start") {
			startCount++
		}
	}
	if startCount != 1 {
		t.Fatalf("expected one systemctl start, got %d: %v", startCount, commands)
	}
}

func TestIdleJanitorStopsOnlyIdleRuntimes(t *testing.T) {
	service := &Service{}
	commands := []string{}
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		commands = append(commands, strings.Join(append([]string{name}, arguments...), " "))
		return nil, nil
	}

	idleActivity := service.siteRuntimeActivityFor("idle-site")
	idleActivity.isRunning = true
	idleActivity.lastRequestAt = time.Now().Add(-sitePocketBaseIdleTimeout - time.Minute)

	busyActivity := service.siteRuntimeActivityFor("busy-site")
	busyActivity.isRunning = true
	busyActivity.inflightCount = 1
	busyActivity.lastRequestAt = time.Now().Add(-sitePocketBaseIdleTimeout - time.Minute)

	freshActivity := service.siteRuntimeActivityFor("fresh-site")
	freshActivity.isRunning = true
	freshActivity.lastRequestAt = time.Now()

	service.stopIdleSitePocketBaseRuntimes(context.Background())

	joined := strings.Join(commands, "\n")
	if !strings.Contains(joined, "idle-site") {
		t.Fatalf("idle site was not stopped: %v", commands)
	}
	if strings.Contains(joined, "busy-site") || strings.Contains(joined, "fresh-site") {
		t.Fatalf("busy or fresh site was stopped: %v", commands)
	}
	if idleActivity.isRunning {
		t.Fatal("idle site still marked running")
	}
}

func TestFinishSitePocketBaseRequestDecrementsInflight(t *testing.T) {
	service := &Service{}
	activity := service.siteRuntimeActivityFor("site-1")
	activity.inflightCount = 2

	service.finishSitePocketBaseRequest("site-1")

	if activity.inflightCount != 1 {
		t.Fatalf("expected inflight 1, got %d", activity.inflightCount)
	}
}

func TestValidateSiteContentPages(t *testing.T) {
	block := siteContentBlock{Variant: "prose", Title: "소개", Body: "본문"}
	cases := []struct {
		name          string
		pages         []siteContentPage
		expectedError string
	}{
		{name: "valid two pages", pages: []siteContentPage{
			{Path: "/", Title: "홈", Blocks: []siteContentBlock{block}},
			{Path: "/about", Title: "소개", Blocks: []siteContentBlock{block}},
		}},
		{name: "reserved api path", pages: []siteContentPage{
			{Path: "/", Title: "홈", Blocks: []siteContentBlock{block}},
			{Path: "/api/things", Title: "목록", Blocks: []siteContentBlock{block}},
		}, expectedError: "reserved"},
		{name: "missing root page", pages: []siteContentPage{
			{Path: "/about", Title: "소개", Blocks: []siteContentBlock{block}},
		}, expectedError: "must include a / page"},
		{name: "duplicate path", pages: []siteContentPage{
			{Path: "/", Title: "홈", Blocks: []siteContentBlock{block}},
			{Path: "/", Title: "홈2", Blocks: []siteContentBlock{block}},
		}, expectedError: "duplicated"},
		{name: "empty blocks", pages: []siteContentPage{
			{Path: "/", Title: "홈", Blocks: nil},
		}, expectedError: "at least one block"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			errorValue := validateSiteContent(&siteContent{SiteName: "테스트", Pages: testCase.pages})
			if testCase.expectedError == "" {
				if errorValue != nil {
					t.Fatalf("expected valid, got %v", errorValue)
				}
				return
			}
			if errorValue == nil || !strings.Contains(errorValue.Error(), testCase.expectedError) {
				t.Fatalf("expected error containing %q, got %v", testCase.expectedError, errorValue)
			}
		})
	}
}

func TestSitePublishTwiceWithPublicImagesStaysContentOnly(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "image-republish", Title: "Image Republish"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.materializeSiteSourceWorkspace(context.Background(), site, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(site.HostSourcePath, "DESIGN.md"), validSiteDesignMarkdownWithColors("#101010", "#fefefe"))
	imagePath := filepath.Join(site.HostSourcePath, "app", "public", "images", "hero.jpg")
	if errorValue := os.MkdirAll(filepath.Dir(imagePath), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, imagePath, "fake-jpeg-bytes")

	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		SourceBundleBase64: testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat: "tar.gz",
	})
	if errorValue != nil {
		t.Fatalf("first publish with public image failed: %v", errorValue)
	}

	writeFile(t, filepath.Join(site.HostSourcePath, "app", "public", "images", "about.jpg"), "fake-jpeg-bytes-2")
	setFileModTime(t, filepath.Join(site.HostSourcePath, "app", "public", "images", "about.jpg"), time.Now().UTC().Add(2*time.Hour))

	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		SourceBundleBase64: testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat: "tar.gz",
	})
	if errorValue != nil {
		t.Fatalf("second publish after adding a public image must stay content-only, got: %v", errorValue)
	}
}

func TestAuthEnabledPublishBootstrapsPocketBaseRuntime(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "auth-bootstrap", Title: "Auth Bootstrap"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.materializeSiteSourceWorkspace(context.Background(), site, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(site.HostSourcePath, "DESIGN.md"), validSiteDesignMarkdownWithColors("#123123", "#fafafa"))
	contentPath := filepath.Join(site.HostSourcePath, "app", "public", "site-content.json")
	document, errorValue := os.ReadFile(contentPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	authDocument := strings.Replace(string(document), "{", `{"auth":{"enabled":true},`, 1)
	writeFile(t, contentPath, authDocument)

	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		SourceBundleBase64: testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat: "tar.gz",
	})
	if errorValue != nil {
		t.Fatalf("auth-enabled publish failed: %v", errorValue)
	}
	migrationPath := filepath.Join(site.HostSourcePath, "pocketbase", "pb_migrations", "1700000002_auth_bootstrap.js")
	if _, errorValue := os.Stat(migrationPath); errorValue != nil {
		t.Fatalf("auth-enabled publish must bootstrap the PocketBase runtime marker: %v", errorValue)
	}
	if !service.siteVersionHasPocketBaseBackend(site, site.CurrentVersionID) {
		t.Fatal("published version must carry a PocketBase backend when auth is enabled")
	}
}
