package admind

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
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
		SiteID:      site.SiteID,
		RequestedBy: "owner@example.com",
		Message:     "Publish demo prototype",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.Status != SiteStatusPublished {
		t.Fatalf("published status = %q", site.Status)
	}
	if site.PublishedURL != "https://demo.device.intern.kim" {
		t.Fatalf("published url = %q", site.PublishedURL)
	}
	if site.WorkspacePath == "" || site.HostSourcePath == "" || site.LastPublishedCommit == "" {
		t.Fatalf("site workspace metadata missing: %+v", site)
	}

	response := serveSiteRequest(service, "demo.device.intern.kim", "/")
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
	response = serveSiteRequest(service, "demo.device.intern.kim", "/")
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
	response = serveSiteRequest(service, "demo.device.intern.kim", "/dashboard")
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
	response = serveSiteRequest(service, "demo.device.intern.kim", "/")
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
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:      site.SiteID,
		RequestedBy: "owner@example.com",
		Message:     "Publish default prototype",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response := serveSiteRequest(service, "default-build.device.intern.kim", "/")
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

func TestSiteGatewayProxiesPocketBasePaths(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "api-demo"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "frontend")
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{SiteID: site.SiteID})
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

	response := serveSiteRequest(service, "api-demo.device.intern.kim", "/api/collections/posts/records")
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
	assertPathPermission(t, filepath.Dir(site.HostSourcePath), 0o777)
	assertPathPermission(t, site.HostSourcePath, 0o777)
	assertPathPermission(t, filepath.Join(site.HostSourcePath, "app"), 0o777)
	assertPathPermission(t, filepath.Join(site.HostSourcePath, "app", "src"), 0o777)
	assertPathPermission(t, filepath.Join(site.HostSourcePath, "app", "package.json"), 0o666)
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

func TestSitePublishRejectsUnapprovedPocketBaseHooks(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "hook-demo"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "workspace")
	writeFile(t, filepath.Join(site.HostSourcePath, "pocketbase", "pb_hooks", "main.pb.js"), "routerAdd('GET', '/x', () => {})")
	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{SiteID: site.SiteID})
	if errorValue == nil {
		t.Fatal("expected unapproved PocketBase hook rejection")
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
	writeFile(t, deviceURLPath, "https://device.intern.kim")
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
	if errorValue := os.MkdirAll(filepath.Join(site.HostSourcePath, "app", "dist", "assets"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(site.HostSourcePath, "app", "dist", "index.html"), "<!doctype html><html><body>"+body+"</body></html>")
	writeFile(t, filepath.Join(site.HostSourcePath, "app", "dist", "assets", "app.js"), "console.log('ok')")
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
