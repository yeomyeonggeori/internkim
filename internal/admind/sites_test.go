package admind

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSiteGatewayLifecycle(t *testing.T) {
	service, commandLog := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{
		Slug:           "demo",
		Title:          "Demo",
		RequestedBy:    "owner@example.com",
		Platform:       "mattermost",
		ConversationID: "thread-1",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "hello dynamic site")
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		RequestedBy:        "owner@example.com",
		Message:            "Publish demo prototype",
		SourceBundleBase64: testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat: "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.Status != SiteStatusPublished {
		t.Fatalf("published status = %q", site.Status)
	}
	if site.PublishedURL != "https://demo.device.example.test" {
		t.Fatalf("published url = %q", site.PublishedURL)
	}
	if site.WorkspacePath == "" || site.HostSourcePath == "" || site.LastPublishedCommit == "" {
		t.Fatalf("site workspace metadata missing: %+v", site)
	}
	if !strings.HasPrefix(site.SourceWorkspacePath, "home/sites/") {
		t.Fatalf("site source workspace should be requester-private virtual path, got %q", site.SourceWorkspacePath)
	}

	response := serveSiteRequest(service, "demo.device.example.test", "/")
	if response.Code != http.StatusOK {
		t.Fatalf("published site status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "hello dynamic site") {
		t.Fatalf("published body = %q", response.Body.String())
	}

	dataPath := filepath.Join(service.sitePath(site.SiteID), "pb_data", "data.db")
	if errorValue := os.MkdirAll(filepath.Dir(dataPath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, dataPath, "site data")
	site, errorValue = service.unpublishSite(context.Background(), site.SiteID, siteLifecycleRequest{RequestedBy: "owner@example.com", Reason: "pause"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.Status != SiteStatusUnpublished {
		t.Fatalf("unpublished status = %q", site.Status)
	}
	response = serveSiteRequest(service, "demo.device.example.test", "/")
	if response.Code != http.StatusGone {
		t.Fatalf("unpublished site status = %d", response.Code)
	}
	if readTrimmedFile(dataPath) != "site data" {
		t.Fatalf("unpublish should preserve data")
	}

	site, errorValue = service.restoreSite(context.Background(), site.SiteID, siteLifecycleRequest{RequestedBy: "owner@example.com"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.Status != SiteStatusPublished {
		t.Fatalf("restored status = %q", site.Status)
	}
	response = serveSiteRequest(service, "demo.device.example.test", "/dashboard")
	if response.Code != http.StatusOK {
		t.Fatalf("restored site status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "hello dynamic site") {
		t.Fatalf("restored body = %q", response.Body.String())
	}

	site, errorValue = service.deleteSite(context.Background(), site.SiteID, siteLifecycleRequest{RequestedBy: "owner@example.com", Confirm: "DELETE", UserConfirmed: true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.Status != SiteStatusDeleted {
		t.Fatalf("deleted status = %q", site.Status)
	}
	if _, statError := os.Stat(service.sitePath(site.SiteID)); !os.IsNotExist(statError) {
		t.Fatalf("deleted site files still exist: %v", statError)
	}
	response = serveSiteRequest(service, "demo.device.example.test", "/")
	if response.Code != http.StatusNotFound {
		t.Fatalf("deleted site status = %d", response.Code)
	}
	if !containsCommand(*commandLog, "systemctl stop "+siteServiceName(site.SiteID)) {
		t.Fatalf("unpublish did not stop site service: %+v", *commandLog)
	}
	if !containsCommand(*commandLog, "systemctl disable --now "+siteServiceName(site.SiteID)) {
		t.Fatalf("delete did not disable site service: %+v", *commandLog)
	}
}

func TestSitePrototypePublishesDefaultBuild(t *testing.T) {
	service, commandLog := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{
		Slug:        "default-build",
		Title:       "Default Build",
		RequestedBy: "owner@example.com",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.HasPrefix(site.SourceWorkspacePath, "home/sites/") {
		t.Fatalf("site source workspace should be requester-private virtual path, got %q", site.SourceWorkspacePath)
	}
	if site.AppWorkspacePath != site.SourceWorkspacePath+"/app" {
		t.Fatalf("site app workspace path = %q, source = %q", site.AppWorkspacePath, site.SourceWorkspacePath)
	}
	if _, statError := os.Stat(filepath.Join(site.HostSourcePath, "DESIGN.md")); !os.IsNotExist(statError) {
		t.Fatalf("site create should not materialize editable source in admind cache: %v", statError)
	}
	writeTestWorkspaceBuild(t, site, "Default Build")
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		RequestedBy:        "owner@example.com",
		Message:            "Publish default prototype",
		SourceBundleBase64: testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat: "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response := serveSiteRequest(service, "default-build.device.example.test", "/")
	if response.Code != http.StatusOK {
		t.Fatalf("default build status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "Default Build") {
		t.Fatalf("default build body = %q", response.Body.String())
	}
	if !containsCommandFragment(*commandLog, "safe.directory="+site.HostSourcePath) {
		t.Fatalf("site git commands should trust the site workspace: %+v", *commandLog)
	}
}

func TestSiteDefaultDesignMDUsesStitchFormat(t *testing.T) {
	document := siteDesignMD(&SiteRecord{Slug: "stitch-demo", Title: "Stitch Demo"})
	for _, expectedText := range []string{
		"---\nversion: alpha",
		"colors:",
		"typography:",
		"rounded:",
		"spacing:",
		"components:",
		"## Overview",
		"## Colors",
		"## Typography",
		"## Layout",
		"## Elevation & Depth",
		"## Shapes",
		"## Components",
		"## Do's and Don'ts",
	} {
		if !strings.Contains(document, expectedText) {
			t.Fatalf("default DESIGN.md must contain %q", expectedText)
		}
	}
	for _, forbiddenText := range []string{"## Product", "## Audience", "## Prototype Scope", "## Workflows", "## Acceptance Criteria"} {
		if strings.Contains(document, forbiddenText) {
			t.Fatalf("default DESIGN.md must not contain legacy section %q", forbiddenText)
		}
	}
}

func TestSiteRevisionEntriesMarkPublishedVersions(t *testing.T) {
	service, _ := newTestSiteService(t)
	site := &SiteRecord{
		SiteID:            "site-1",
		Slug:              "demo",
		CurrentVersionID:  "20260529T010203-abcdef123456",
		PreviousVersionID: "20260528T010203-111111111111",
	}
	if errorValue := os.MkdirAll(service.siteVersionPath(site.SiteID, site.CurrentVersionID), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	output := "abcdef1234567890\x1f1770000000\x1fImprove layout\n2222222222229999\x1f1760000000\x1fDraft only"
	entries := service.siteRevisionEntries(site, output)
	if len(entries) != 2 {
		t.Fatalf("revision entries = %+v", entries)
	}
	if !entries[0].IsCurrent || !entries[0].IsPublishedBuild || entries[0].VersionID != site.CurrentVersionID {
		t.Fatalf("current revision metadata = %+v", entries[0])
	}
	if entries[1].IsPublishedBuild {
		t.Fatalf("draft-only commit should not be marked published: %+v", entries[1])
	}
}

func TestSiteRollbackCanTargetPublishedRevision(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "rollback-revision", RequestedBy: "owner@example.com"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	currentVersionID := "20260529T010203-abcdef123456"
	targetVersionID := "20260528T010203-111111111111"
	site.Status = SiteStatusPublished
	site.CurrentVersionID = currentVersionID
	if errorValue := service.storeSite(site); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, versionID := range []string{currentVersionID, targetVersionID} {
		if errorValue := os.MkdirAll(service.siteVersionPath(site.SiteID, versionID), 0o700); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	rolledBackSite, errorValue := service.rollbackSite(context.Background(), site.SiteID, siteLifecycleRequest{
		RequestedBy: "owner@example.com",
		Revision:    "1111111111119999",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if rolledBackSite.CurrentVersionID != targetVersionID || rolledBackSite.PreviousVersionID != currentVersionID {
		t.Fatalf("rollback version metadata = %+v", rolledBackSite)
	}
}

func TestSiteReactScaffoldIncludesManagedBuildContract(t *testing.T) {
	packageJSON := sitePackageJSON(&SiteRecord{Slug: "react-demo"})
	for _, expectedText := range []string{`"react"`, `"vite"`, `"@google/design.md"`, `"@vitejs/plugin-react"`, `"bun scripts/build.ts"`} {
		if !strings.Contains(packageJSON, expectedText) {
			t.Fatalf("site package manifest must contain %q", expectedText)
		}
	}
	for _, expectedText := range []string{`name: "bun", arguments: ["install"]`, "@google/design.md", "vite"} {
		if !strings.Contains(siteBuildTS(), expectedText) {
			t.Fatalf("site build script must contain %q", expectedText)
		}
	}
}

func TestSitePublishMaterializesEditableSourceBundle(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{
		Slug:        "source-bundle",
		Title:       "Source Bundle",
		RequestedBy: "owner@example.com",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	sourceWorkspacePath := t.TempDir()
	writeTestSourceBuild(t, sourceWorkspacePath, "bundle publish")
	writeFile(t, filepath.Join(sourceWorkspacePath, "DESIGN.md"), "custom source design")
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		RequestedBy:         "owner@example.com",
		Message:             "Publish bundled source",
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, sourceWorkspacePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if readTrimmedFile(filepath.Join(site.HostSourcePath, "DESIGN.md")) != "custom source design" {
		t.Fatalf("host staging should be materialized from editable source")
	}
	response := serveSiteRequest(service, "source-bundle.device.example.test", "/")
	if !strings.Contains(response.Body.String(), "bundle publish") {
		t.Fatalf("published body = %q", response.Body.String())
	}
}

func TestSiteCreateStoresMetadataOwnershipAndIdeaMirror(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{
		Slug:           "portfolio-demo",
		Title:          "Portfolio Demo",
		Prompt:         "김인턴 포트폴리오 사이트를 만들어줘",
		Description:    "김인턴의 업무 자동화 역량을 보여주는 포트폴리오",
		Idea:           "업무를 대신 처리하는 인턴형 AI 포트폴리오",
		Purpose:        "portfolio",
		Audience:       "잠재 사용자",
		Archetype:      "portfolio",
		DomainKeywords: []string{"ai assistant", "portfolio"},
		RequestedBy:    "owner@example.com",
		Requester: siteIdentity{
			PersonID:       "person-1",
			Platform:       "mattermost",
			PlatformUserID: "user-1",
			DisplayName:    "Owner",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.Description == "" || site.Idea == "" || site.Purpose != "portfolio" || site.Archetype != "portfolio" {
		t.Fatalf("expected site metadata to be stored, got %+v", site)
	}
	if site.OwnerIdentity.PersonID != "person-1" || site.CreatedBy.PlatformUserID != "user-1" {
		t.Fatalf("expected structured owner identity, got owner=%+v createdBy=%+v", site.OwnerIdentity, site.CreatedBy)
	}
	metadata := service.siteWorkspaceMetadata(site)
	if !strings.Contains(metadata, `"idea"`) || !strings.Contains(metadata, `"owner"`) {
		t.Fatalf("expected workspace metadata mirror to include idea and owner, got %s", metadata)
	}
	idea := siteIdeaMarkdown(site)
	if !strings.Contains(idea, "업무를 대신 처리") {
		t.Fatalf("expected idea mirror, got %s", idea)
	}
}

func TestSitePublishRequiresOwnerOrEditor(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{
		Slug:        "private-site",
		Title:       "Private Site",
		RequestedBy: "owner@example.com",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	sourceWorkspacePath := t.TempDir()
	writeTestSourceBuild(t, sourceWorkspacePath, "private publish")
	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		RequestedBy:         "other@example.com",
		Message:             "Publish from other user",
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, sourceWorkspacePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "editor permission") {
		t.Fatalf("expected editor permission failure, got %v", errorValue)
	}
}

func TestSiteGatewayProxiesPocketBasePaths(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "api-demo"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "frontend")
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		SourceBundleBase64: testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat: "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "127.0.0.1:"+strconv.Itoa(site.Port) {
			t.Fatalf("proxy host = %q", request.URL.Host)
		}
		if request.URL.Path != "/api/collections/posts/records" {
			t.Fatalf("proxy path = %q", request.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       io.NopCloser(strings.NewReader("pocketbase")),
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Request:    request,
		}, nil
	})}

	response := serveSiteRequest(service, "api-demo.device.example.test", "/api/collections/posts/records")
	if response.Code != http.StatusAccepted {
		t.Fatalf("pocketbase proxy status = %d", response.Code)
	}
	if response.Body.String() != "pocketbase" {
		t.Fatalf("pocketbase proxy body = %q", response.Body.String())
	}
}

func TestSiteRegistryPersistsAndAllocatesDistinctPorts(t *testing.T) {
	service, _ := newTestSiteService(t)
	firstSite, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "first"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	secondSite, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "second"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if firstSite.Port == secondSite.Port {
		t.Fatalf("expected distinct ports, got %d", firstSite.Port)
	}

	reloadedService := NewService(service.Configuration)
	reloadedSite := reloadedService.findSiteBySlug("first")
	if reloadedSite == nil {
		t.Fatalf("reloaded site missing")
	}
	if reloadedSite.Port != firstSite.Port {
		t.Fatalf("reloaded port = %d", reloadedSite.Port)
	}
}

func TestSiteWorkspaceIsWritableByRequesterTerminal(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "terminal-writable"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.HasPrefix(site.SourceWorkspacePath, "home/sites/") {
		t.Fatalf("expected requester-private virtual source workspace, got %q", site.SourceWorkspacePath)
	}
	if site.WorkspacePath != site.SourceWorkspacePath {
		t.Fatalf("workspace path should point at source workspace: %+v", site)
	}
	if site.AppWorkspacePath != site.SourceWorkspacePath+"/app" {
		t.Fatalf("app workspace path should point at app source: %+v", site)
	}
}

func TestSiteDeleteRequiresExplicitConfirmation(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "delete-me"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = service.deleteSite(context.Background(), site.SiteID, siteLifecycleRequest{})
	if errorValue == nil {
		t.Fatal("expected delete confirmation error")
	}
	if service.findSiteByID(site.SiteID).Status == SiteStatusDeleted {
		t.Fatal("site should not be deleted without confirmation")
	}
}

func TestSiteWorkspacePublishRejectsArbitrarySourcePaths(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "workspace-only"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "workspace")
	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		FrontendSourcePath: t.TempDir(),
	})
	if errorValue == nil {
		t.Fatal("expected arbitrary frontend source path rejection")
	}
}

func TestSitePublishRejectsStaleFrontendBuild(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "stale-build"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "first build")
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		SourceBundleBase64: testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat: "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	sourcePath := filepath.Join(site.HostSourcePath, "app", "src", "App.tsx")
	writeFile(t, sourcePath, "export default function App() {\n  return <main>updated source</main>;\n}\n")
	sourceModTime := time.Now().UTC().Add(2 * time.Hour)
	setFileModTime(t, sourcePath, sourceModTime)
	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		SourceBundleBase64: testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat: "tar.gz",
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "app/dist is stale") {
		t.Fatalf("expected stale build rejection, got %v", errorValue)
	}

	writeTestWorkspaceBuild(t, site, "fresh build")
	setDirectoryFilesModTime(t, filepath.Join(site.HostSourcePath, "app", "dist"), sourceModTime.Add(time.Hour))
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		SourceBundleBase64: testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat: "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response := serveSiteRequest(service, "stale-build.device.example.test", "/")
	if !strings.Contains(response.Body.String(), "fresh build") {
		t.Fatalf("published body = %q", response.Body.String())
	}
}

func TestSitePublishRejectsUnapprovedPocketBaseHooks(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "hook-demo"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "workspace")
	writeFile(t, filepath.Join(site.HostSourcePath, "pocketbase", "pb_hooks", "main.pb.js"), "routerAdd('GET', '/x', () => {})")
	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		SourceBundleBase64: testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat: "tar.gz",
	})
	if errorValue == nil {
		t.Fatal("expected unapproved PocketBase hook rejection")
	}
}

func TestSitePublishIgnoresPocketBaseHookMetadataFiles(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "hook-metadata-demo"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "workspace")
	writeFile(t, filepath.Join(site.HostSourcePath, "pocketbase", "pb_hooks", ".gitkeep"), "")
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		SourceBundleBase64: testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat: "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.Status != SiteStatusPublished {
		t.Fatalf("published status = %q", site.Status)
	}
}

func TestSiteLifecycleRequiresOwnerOrConfirmation(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "owned", RequestedBy: "owner@example.com"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = service.unpublishSite(context.Background(), site.SiteID, siteLifecycleRequest{RequestedBy: "other@example.com"})
	if errorValue == nil {
		t.Fatal("expected owner confirmation error")
	}
	_, errorValue = service.unpublishSite(context.Background(), site.SiteID, siteLifecycleRequest{RequestedBy: "other@example.com", Confirm: "CONFIRM", UserConfirmed: true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
}

func newTestSiteService(t *testing.T) (*Service, *[]string) {
	t.Helper()
	rootPath := t.TempDir()
	deviceURLPath := filepath.Join(rootPath, "device-url")
	writeFile(t, deviceURLPath, "https://device.example.test")
	commandLog := []string{}
	service := NewService(Configuration{
		StateDirectory:        filepath.Join(rootPath, "state", "admin"),
		AdminEmailPath:        writeTestFile(t, "admin@example.com"),
		DeviceURLPath:         deviceURLPath,
		SitesRoot:             filepath.Join(rootPath, "sites"),
		SiteSecretDirectory:   filepath.Join(rootPath, "secrets", "sites"),
		SiteSystemdDirectory:  filepath.Join(rootPath, "systemd"),
		CompanionJobPath:      filepath.Join(rootPath, "state", "admin", "jobs.json"),
		FlowDatabasePath:      filepath.Join(rootPath, "state", "admin", "flow.sqlite"),
		MattermostBaseURL:     "http://mattermost.local",
		BlueclawWorkspacePath: filepath.Join(rootPath, "blueclaw"),
	})
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		commandLog = append(commandLog, strings.TrimSpace(name+" "+strings.Join(arguments, " ")))
		return []byte("ok"), nil
	}
	return service, &commandLog
}

func writeTestWorkspaceBuild(t *testing.T, site *SiteRecord, body string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Join(site.HostSourcePath, "app", "src"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.MkdirAll(filepath.Join(site.HostSourcePath, "app", "dist", "assets"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.MkdirAll(filepath.Join(site.HostSourcePath, "pocketbase", "pb_hooks"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(site.HostSourcePath, "app", "src", "App.tsx"), "export default function App() { return <main>ok</main> }\n")
	writeFile(t, filepath.Join(site.HostSourcePath, "app", "dist", "index.html"), "<!doctype html><html><body>"+body+"</body></html>")
	writeFile(t, filepath.Join(site.HostSourcePath, "app", "dist", "assets", "app.js"), "console.log('ok')")
	writeTestBuildQuality(t, site.HostSourcePath)
}

func writeTestSourceBuild(t *testing.T, workspacePath string, body string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Join(workspacePath, "app", "src"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.MkdirAll(filepath.Join(workspacePath, "app", "dist", "assets"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(workspacePath, "app", "package.json"), `{"scripts":{"build":"vite"}}`)
	writeFile(t, filepath.Join(workspacePath, "app", "src", "App.tsx"), "export default function App() { return <main>ok</main> }\n")
	writeFile(t, filepath.Join(workspacePath, "app", "dist", "index.html"), "<!doctype html><html><body>"+body+"</body></html>")
	writeFile(t, filepath.Join(workspacePath, "app", "dist", "assets", "app.js"), "console.log('ok')")
	writeTestBuildQuality(t, workspacePath)
}

func writeTestBuildQuality(t *testing.T, workspacePath string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Join(workspacePath, ".internkim"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(workspacePath, ".internkim", "build-quality.json"), `{"blockingIssueCount":0}`)
}

func testSourceBundleBase64(t *testing.T, sourceWorkspacePath string) string {
	t.Helper()
	buffer := bytes.Buffer{}
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	errorValue := filepath.Walk(sourceWorkspacePath, func(path string, information os.FileInfo, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relativePath, errorValue := filepath.Rel(sourceWorkspacePath, path)
		if errorValue != nil || relativePath == "." {
			return errorValue
		}
		if testSourceBundlePathIsSkipped(relativePath, information) {
			if information.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		header, errorValue := tar.FileInfoHeader(information, "")
		if errorValue != nil {
			return errorValue
		}
		header.Name = filepath.ToSlash(relativePath)
		if errorValue := tarWriter.WriteHeader(header); errorValue != nil {
			return errorValue
		}
		if information.IsDir() {
			return nil
		}
		file, errorValue := os.Open(path)
		if errorValue != nil {
			return errorValue
		}
		defer file.Close()
		_, errorValue = io.Copy(tarWriter, file)
		return errorValue
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := tarWriter.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := gzipWriter.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	return base64.StdEncoding.EncodeToString(buffer.Bytes())
}

func testSourceBundlePathIsSkipped(relativePath string, information os.FileInfo) bool {
	_ = information
	for _, component := range strings.Split(filepath.Clean(relativePath), string(os.PathSeparator)) {
		switch component {
		case ".git", "node_modules":
			return true
		}
	}
	return false
}

func setFileModTime(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	if errorValue := os.Chtimes(path, modTime, modTime); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func setDirectoryFilesModTime(t *testing.T, rootPath string, modTime time.Time) {
	t.Helper()
	errorValue := filepath.Walk(rootPath, func(path string, information os.FileInfo, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if !information.Mode().IsRegular() {
			return nil
		}
		return os.Chtimes(path, modTime, modTime)
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
}

func serveSiteRequest(service *Service, host string, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "https://"+host+path, nil)
	request.Host = host
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	return response
}

func containsCommand(commands []string, expected string) bool {
	for _, command := range commands {
		if command == expected {
			return true
		}
	}
	return false
}

func containsCommandFragment(commands []string, expected string) bool {
	for _, command := range commands {
		if strings.Contains(command, expected) {
			return true
		}
	}
	return false
}

func assertPathPermission(t *testing.T, path string, expected os.FileMode) {
	t.Helper()
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if information.Mode().Perm() != expected {
		t.Fatalf("%s permission = %o, expected %o", path, information.Mode().Perm(), expected)
	}
}
