package admind

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSiteGatewayLifecycle(t *testing.T) {
	service, commandLog := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
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
		SiteID:              site.SiteID,
		RequestedBy:         "owner@example.com",
		Message:             "Publish demo prototype",
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
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
	if !strings.HasPrefix(site.SourceWorkspacePath, "/workspace/circles/staff/sites/") || !strings.HasSuffix(site.SourceWorkspacePath, "/draft") {
		t.Fatalf("site source workspace should be staff-circle draft path, got %q", site.SourceWorkspacePath)
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

func TestSiteResponsesHidePublishedURLUntilPublished(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
		Slug:        "visibility-demo",
		Title:       "Visibility Demo",
		RequestedBy: "owner@example.com",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.PublishedURL == "" {
		t.Fatal("stored site should keep computed published URL")
	}
	assertSiteResponseOmitsPublishedURL(t, writeSiteResponse(service, site.SiteID))
	assertSiteResponseOmitsPublishedURL(t, writeSiteListResponse(service))

	writeTestWorkspaceBuild(t, site, "visible after publish")
	publishResponse := publishSiteResponse(t, service, site)
	assertSiteResponseIncludesPublishedURL(t, publishResponse, "https://visibility-demo.device.intern.kim")

	unpublishRequest := httptest.NewRequest(http.MethodPost, "/admin/api/sites/"+site.SiteID+"/unpublish", strings.NewReader(`{"requestedBy":"owner@example.com"}`))
	unpublishResponse := httptest.NewRecorder()
	service.unpublishSiteFromRequest(unpublishResponse, unpublishRequest, site.SiteID)
	if unpublishResponse.Code != http.StatusOK {
		t.Fatalf("unpublish status = %d body = %q", unpublishResponse.Code, unpublishResponse.Body.String())
	}
	assertSiteResponseOmitsPublishedURL(t, unpublishResponse)
	assertSiteResponseOmitsPublishedURL(t, writeSiteResponse(service, site.SiteID))
	assertSiteResponseOmitsPublishedURL(t, writeSiteListResponse(service))
}

func TestSiteListCanIncludeLiveHTTPStatus(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
		Slug:        "live-status",
		Title:       "Live Status",
		RequestedBy: "owner@example.com",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "live status ok")
	publishSiteResponse(t, service, site)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/api/sites?checkLive=true", nil)
	service.listSites(response, request)

	siteObject := siteJSONObjects(t, response)[0]
	if siteObject["liveHTTPStatus"] != float64(http.StatusOK) {
		t.Fatalf("liveHTTPStatus = %v, response = %s", siteObject["liveHTTPStatus"], response.Body.String())
	}
}

func TestSitePublishFailsWhenReadinessProbeSeesEmptyIndex(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
		Slug:        "empty-index",
		RequestedBy: "owner@example.com",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "temporary")
	writeFile(t, filepath.Join(site.HostSourcePath, "app", "dist", "index.html"), "")

	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		RequestedBy:         "owner@example.com",
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue == nil {
		t.Fatal("expected readiness probe failure")
	}
	if !strings.Contains(errorValue.Error(), "observed status 200") || !strings.Contains(errorValue.Error(), "index body length 0") {
		t.Fatalf("readiness error = %v", errorValue)
	}
	failedSite := service.findSiteByID(site.SiteID)
	if failedSite.Status != SiteStatusFailed || !strings.Contains(failedSite.LastError, "observed status 200") {
		t.Fatalf("failed site state = %+v", failedSite)
	}
}

func TestSitePublishBlockedByPublishedSiteLimit(t *testing.T) {
	service, _ := newTestSiteService(t)
	var firstPublishedSiteID string
	for index := 0; index < publishedSiteLimit; index++ {
		publishedSite, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
			Slug:        "published-" + strconv.Itoa(index),
			RequestedBy: "owner@example.com",
		})
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		publishedSite.Status = SiteStatusPublished
		if errorValue := service.storeSite(publishedSite); errorValue != nil {
			t.Fatal(errorValue)
		}
		if index == 0 {
			firstPublishedSiteID = publishedSite.SiteID
		}
	}
	draftSite, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
		Slug:        "draft-overflow",
		RequestedBy: "owner@example.com",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	_, publishError := service.publishSite(context.Background(), sitePublishRequest{
		SiteID:      draftSite.SiteID,
		RequestedBy: "owner@example.com",
	})
	if publishError == nil {
		t.Fatal("expected publish limit error")
	}
	if !strings.Contains(publishError.Error(), "site publish limit reached") {
		t.Fatalf("publish error = %v", publishError)
	}
	if !strings.Contains(publishError.Error(), firstPublishedSiteID) {
		t.Fatalf("publish error missing published siteID: %v", publishError)
	}

	_, republishError := service.publishSite(context.Background(), sitePublishRequest{
		SiteID:      firstPublishedSiteID,
		RequestedBy: "owner@example.com",
	})
	if republishError != nil && strings.Contains(republishError.Error(), "site publish limit reached") {
		t.Fatalf("republish of an already-published site should not be blocked by the cap: %v", republishError)
	}
}

func TestSitePublishedURLFallsBackToFleetDomainWithoutDeviceURL(t *testing.T) {
	service, _ := newTestSiteService(t)
	rootPath := t.TempDir()
	service.Configuration.DeviceURLPath = filepath.Join(rootPath, "missing-device-url")
	fleetIDPath := filepath.Join(rootPath, "fleet-id")
	writeFile(t, fleetIDPath, "9rrfolb86o61")
	service.Configuration.FleetIDPath = fleetIDPath

	if publishedURL := service.sitePublishedURL("banchan-table-reservation"); publishedURL != "https://banchan-table-reservation.9rrfolb86o61.intern.kim" {
		t.Fatalf("published url = %q", publishedURL)
	}
	if slug := service.siteSlugFromRequestHost("banchan-table-reservation.9rrfolb86o61.intern.kim"); slug != "banchan-table-reservation" {
		t.Fatalf("slug from request host = %q", slug)
	}
}

func TestSitePublishSurfacesPocketBaseRestartError(t *testing.T) {
	service, _ := newTestSiteService(t)
	service.RunCommand = siteRestartErrorCommand
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
		Slug:        "backend-restart",
		RequestedBy: "owner@example.com",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "backend")
	if errorValue := os.MkdirAll(filepath.Join(site.HostSourcePath, "pocketbase", "pb_migrations"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(site.HostSourcePath, "pocketbase", "pb_migrations", "001_init.js"), "migrate(() => {})")

	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		RequestedBy:         "owner@example.com",
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "systemctl restart") || !strings.Contains(errorValue.Error(), "restart unavailable") {
		t.Fatalf("expected restart failure, got %v", errorValue)
	}
	failedSite := service.findSiteByID(site.SiteID)
	if failedSite.Status != SiteStatusFailed || !strings.Contains(failedSite.LastError, "restart unavailable") {
		t.Fatalf("failed site state = %+v", failedSite)
	}
}

func TestSitePublishIgnoresRestartErrorWithoutPocketBaseBackend(t *testing.T) {
	service, _ := newTestSiteService(t)
	service.RunCommand = siteRestartErrorCommand
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
		Slug:        "frontend-restart",
		RequestedBy: "owner@example.com",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "frontend")

	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		RequestedBy:         "owner@example.com",
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.Status != SiteStatusPublished {
		t.Fatalf("published status = %q", site.Status)
	}
}

func TestSitePrototypePublishesDefaultBuild(t *testing.T) {
	service, commandLog := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
		Slug:        "default-build",
		Title:       "Default Build",
		RequestedBy: "owner@example.com",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.HasPrefix(site.SourceWorkspacePath, "/workspace/circles/staff/sites/") || !strings.HasSuffix(site.SourceWorkspacePath, "/draft") {
		t.Fatalf("site source workspace should be staff-circle draft path, got %q", site.SourceWorkspacePath)
	}
	if site.AppWorkspacePath != site.SourceWorkspacePath+"/app" {
		t.Fatalf("site app workspace path = %q, source = %q", site.AppWorkspacePath, site.SourceWorkspacePath)
	}
	if _, statError := os.Stat(filepath.Join(site.HostSourcePath, "DESIGN.md")); statError != nil {
		t.Fatalf("site create should stage editable source before commit: %v", statError)
	}
	writeTestWorkspaceBuild(t, site, "Default Build")
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		RequestedBy:         "owner@example.com",
		Message:             "Publish default prototype",
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
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
	sourceInformation, errorValue := os.Stat(site.HostSourcePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if sourceInformation.Mode().Perm()&0o007 != 0 {
		t.Fatalf("site source ledger must not be world-accessible, got %o", sourceInformation.Mode().Perm())
	}
}

func TestSitePreviewDoesNotChangePublishedURLUntilPublish(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
		Slug:          "preview-flow",
		Title:         "Preview Flow",
		RequestedBy:   "owner@example.com",
		OwnerIdentity: siteIdentity{PersonID: "person-1", DisplayName: "Owner"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "published version")
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		RequestedBy:         "owner@example.com",
		Message:             "Publish stable version",
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "draft preview version")
	site, errorValue = service.previewSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		RequestedBy:         "owner@example.com",
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.PreviewID == "" || !strings.Contains(site.PreviewURL, "/__preview/"+site.PreviewID) {
		t.Fatalf("expected preview URL metadata, got %+v", site)
	}
	publishedResponse := serveSiteRequest(service, "preview-flow.device.intern.kim", "/")
	if !strings.Contains(publishedResponse.Body.String(), "published version") || strings.Contains(publishedResponse.Body.String(), "draft preview version") {
		t.Fatalf("preview should not change public response: %q", publishedResponse.Body.String())
	}
	previewResponse := serveSiteRequest(service, "preview-flow.device.intern.kim", "/__preview/"+site.PreviewID+"/")
	if previewResponse.Code != http.StatusOK || !strings.Contains(previewResponse.Body.String(), "draft preview version") {
		t.Fatalf("expected preview response, status=%d body=%q", previewResponse.Code, previewResponse.Body.String())
	}
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		RequestedBy:         "owner@example.com",
		Message:             "Publish preview version",
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.PreviewURL != "" || site.PreviewID != "" {
		t.Fatalf("expected publish to clear preview metadata, got %+v", site)
	}
	ownerCurrentTarget, errorValue := os.Readlink(filepath.Join(service.sitePublishedRootPath(site), "current"))
	if errorValue != nil || ownerCurrentTarget != service.sitePublishedVersionPath(site, site.CurrentVersionID) {
		t.Fatalf("expected owner-private current symlink to point at live version, target=%q error=%v", ownerCurrentTarget, errorValue)
	}
	runtimeCurrentTarget, errorValue := os.Readlink(filepath.Join(service.sitePath(site.SiteID), "current"))
	if errorValue != nil || runtimeCurrentTarget != service.sitePublishedVersionPath(site, site.CurrentVersionID) {
		t.Fatalf("expected runtime current symlink to point at live version, target=%q error=%v", runtimeCurrentTarget, errorValue)
	}
	publishedResponse = serveSiteRequest(service, "preview-flow.device.intern.kim", "/")
	if !strings.Contains(publishedResponse.Body.String(), "draft preview version") {
		t.Fatalf("expected published response to update after publish: %q", publishedResponse.Body.String())
	}
	previewResponse = serveSiteRequest(service, "preview-flow.device.intern.kim", "/__preview/preview-missing/")
	if previewResponse.Code != http.StatusNotFound {
		t.Fatalf("expected preview to be closed after publish, got %d", previewResponse.Code)
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
		"TODO(design)",
		"primary: \"#111111\"",
		"background: \"#FFFFFF\"",
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
	for _, forbiddenText := range []string{"## Product", "## Audience", "## Prototype Scope", "## Workflows", "## Acceptance Criteria", "warm limestone", "slate text", "green secondary accents", "amber tertiary highlights"} {
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
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "rollback-revision", RequestedBy: "owner@example.com"})
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
	packageJSON := readRepositoryFile(t, "assets", "blueclaw-site-scaffold", "react-vite-ts", "package.json")
	for _, expectedText := range []string{`"react"`, `"vite"`, `"@vitejs/plugin-react"`, `"bun scripts/build.ts"`} {
		if !strings.Contains(packageJSON, expectedText) {
			t.Fatalf("site package manifest must contain %q", expectedText)
		}
	}
	if strings.Contains(packageJSON, "@google/design.md") {
		t.Fatalf("site package manifest must not depend on nested design.md CLI")
	}
	if strings.Contains(packageJSON, `": "^`) {
		t.Fatalf("site package manifest must pin exact dependency versions")
	}
	buildScript := readRepositoryFile(t, "assets", "blueclaw-site-scaffold", "react-vite-ts", "scripts", "build.ts")
	for _, expectedText := range []string{`arguments: ["install", "--prefer-offline"]`, `existsSync("node_modules/vite/bin/vite.js")`, `arguments: ["--bun", "./node_modules/vite/bin/vite.js", "build", "--logLevel", "info"]`, `collectDesignQualityIssues`, `category: "designDocument"`, `await buildVite();`} {
		if !strings.Contains(buildScript, expectedText) {
			t.Fatalf("site build script must contain %q", expectedText)
		}
	}
	if strings.Contains(buildScript, "Bun.execPath") || strings.Contains(buildScript, `name: "bunx"`) {
		t.Fatalf("site build script must rely on canonical runtime PATH, got Bun.execPath/bunx")
	}
	if strings.Contains(buildScript, `import("vite")`) {
		t.Fatalf("site build script must use the installed local Vite binary instead of resolving Vite through Bun's package cache")
	}
	if strings.Contains(buildScript, `name: "./node_modules/.bin/vite"`) {
		t.Fatalf("site build script must not require a node executable through Vite's shebang")
	}
	if strings.Contains(buildScript, `arguments: ["x", "vite", "build"]`) {
		t.Fatalf("site build script must not spawn nested bun x vite")
	}
	if !strings.Contains(buildScript, `PATH: canonicalRuntimePATH`) {
		t.Fatalf("site build script must pass canonical PATH to child commands")
	}
	if strings.Contains(buildScript, `existsSync("node_modules")`) {
		t.Fatalf("site build script must refresh dependencies instead of trusting stale node_modules")
	}
	if strings.Contains(buildScript, "site quality gate failed") {
		t.Fatalf("site build script must not fail solely because quality issues were reported")
	}
	if strings.Contains(buildScript, "DESIGN.md lint failed") {
		t.Fatalf("site build script must report DESIGN.md issues without failing the build")
	}
	if strings.Contains(buildScript, "DESIGN.md is required") {
		t.Fatalf("site build script must not fail solely because DESIGN.md is missing")
	}
	if !strings.Contains(buildScript, "suggestedFix") {
		t.Fatalf("site build script must include actionable quality fixes")
	}
	viteIndex := strings.Index(buildScript, `await buildVite();`)
	qualityIndex := strings.LastIndex(buildScript, "writeBuildQuality(qualityIssues);")
	if viteIndex < 0 || qualityIndex < viteIndex {
		t.Fatalf("site build script must write build-quality.json after vite build")
	}

	indexCSS := readRepositoryFile(t, "assets", "blueclaw-site-scaffold", "react-vite-ts", "src", "index.css")
	for _, expectedText := range []string{`--background: #ffffff`, `--foreground: #111111`, `--primary: #111111`, `--border: #e5e7eb`} {
		if !strings.Contains(indexCSS, expectedText) {
			t.Fatalf("site scaffold must default to black-on-white token %q", expectedText)
		}
	}
	for _, forbiddenText := range []string{"#0f172a", "#f7f5ef", "#2f6b5f", "#d97706"} {
		if strings.Contains(strings.ToLower(indexCSS), forbiddenText) {
			t.Fatalf("site scaffold must not default to slate/navy accent token %q", forbiddenText)
		}
	}
}

func TestSiteScaffoldMirrorsCanonicalAssets(t *testing.T) {
	assertDirectoriesMatch(t,
		repositoryPath("assets", "blueclaw-site-scaffold", "react-vite-ts"),
		repositoryPath("internal", "admind", "site_scaffold", "react-vite-ts"),
	)
	siteSource := readRepositoryFile(t, "internal", "admind", "sites.go")
	for _, forbiddenText := range []string{"func sitePackageJSON", "func siteBuildTS", "func siteAppTSX", "func siteIndexCSS"} {
		if strings.Contains(siteSource, forbiddenText) {
			t.Fatalf("admind must materialize the canonical scaffold instead of keeping duplicate %q", forbiddenText)
		}
	}
}

func TestSiteScaffoldDistMatchesScaffoldSource(t *testing.T) {
	manifestDocument := readRepositoryFile(t, "internal", "admind", "site_scaffold_dist", "react-vite-ts", "manifest.json")
	var manifest struct {
		ScaffoldSourceSHA256 string `json:"scaffoldSourceSHA256"`
	}
	if errorValue := json.Unmarshal([]byte(manifestDocument), &manifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	if manifest.ScaffoldSourceSHA256 == "" {
		t.Fatal("expected scaffoldSourceSHA256 in site_scaffold_dist manifest.json")
	}
	recomputedSHA256 := scaffoldSourceSHA256ForTest(t, repositoryPath("internal", "admind", "site_scaffold", "react-vite-ts"))
	if recomputedSHA256 != manifest.ScaffoldSourceSHA256 {
		t.Fatalf("site_scaffold_dist manifest is stale; re-run tools/build-site-scaffold-dist (recomputed=%s manifest=%s)", recomputedSHA256, manifest.ScaffoldSourceSHA256)
	}
}

func scaffoldSourceSHA256ForTest(t *testing.T, scaffoldSourceDirectory string) string {
	t.Helper()
	type sourceEntry struct {
		relativePath  string
		contentSHA256 string
	}
	entries := []sourceEntry{}
	errorValue := filepath.Walk(scaffoldSourceDirectory, func(path string, information os.FileInfo, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if information.IsDir() {
			if information.Name() == "node_modules" || information.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if information.Name() == ".DS_Store" || strings.HasPrefix(information.Name(), "._") {
			return nil
		}
		relativePath, relativeError := filepath.Rel(scaffoldSourceDirectory, path)
		if relativeError != nil {
			return relativeError
		}
		document, readError := os.ReadFile(path)
		if readError != nil {
			return readError
		}
		entries = append(entries, sourceEntry{relativePath: filepath.ToSlash(relativePath), contentSHA256: sha256Hex(string(document))})
		return nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	sort.Slice(entries, func(leftIndex int, rightIndex int) bool {
		return entries[leftIndex].relativePath < entries[rightIndex].relativePath
	})
	aggregateInput := strings.Builder{}
	for _, entry := range entries {
		aggregateInput.WriteString(entry.relativePath)
		aggregateInput.WriteString("\n")
		aggregateInput.WriteString(entry.contentSHA256)
		aggregateInput.WriteString("\n")
	}
	return sha256Hex(aggregateInput.String())
}

func TestSiteCreateMaterializesScaffoldDistContentAndManifest(t *testing.T) {
	service, _ := newTestSiteService(t)
	content := &siteContent{
		SiteName: "콘텐츠 사이트",
		Tagline:  "환영합니다",
		Sections: []siteContentSection{
			{Title: "소개", Body: "이 사이트는 예시입니다."},
		},
	}
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "content-site", Title: "Content Site"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.materializeSiteSourceWorkspace(context.Background(), site, content); errorValue != nil {
		t.Fatal(errorValue)
	}

	contentDocument := readTrimmedFile(filepath.Join(site.HostSourcePath, "app", "public", "site-content.json"))
	var roundTrippedContent siteContent
	if errorValue := json.Unmarshal([]byte(contentDocument), &roundTrippedContent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if roundTrippedContent.SiteName != content.SiteName || len(roundTrippedContent.Sections) != 1 || roundTrippedContent.Sections[0].Body != content.Sections[0].Body {
		t.Fatalf("content round trip = %+v", roundTrippedContent)
	}

	if !isDirectory(filepath.Join(site.HostSourcePath, "app", "dist")) {
		t.Fatal("expected create to materialize the canonical app/dist")
	}
	indexHTML := readTrimmedFile(filepath.Join(site.HostSourcePath, "app", "dist", "index.html"))
	if !strings.Contains(indexHTML, "Content Site") {
		t.Fatalf("expected dist index.html to be title-substituted, got %q", indexHTML)
	}

	manifestDocument := readTrimmedFile(filepath.Join(site.HostSourcePath, siteScaffoldAppManifestPath))
	var manifest map[string]string
	if errorValue := json.Unmarshal([]byte(manifestDocument), &manifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(manifest) == 0 {
		t.Fatal("expected a non-empty scaffold-app-manifest.json")
	}
	if !siteAppMatchesScaffoldManifest(site.HostSourcePath) {
		t.Fatal("expected a freshly created workspace to match its own scaffold manifest")
	}
}

func TestSitePristinePublishSucceedsWithoutAppDistOrFreshnessCheck(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "pristine-publish", Title: "Pristine Publish"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.materializeSiteSourceWorkspace(context.Background(), site, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(site.HostSourcePath, "DESIGN.md"), validSiteDesignMarkdownWithColors("#101010", "#fefefe"))
	if errorValue := os.RemoveAll(filepath.Join(site.HostSourcePath, "app", "dist")); errorValue != nil {
		t.Fatal(errorValue)
	}

	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.Status != SiteStatusPublished {
		t.Fatalf("published status = %q", site.Status)
	}
	if site.QualityStatus != "passed_prebuilt" {
		t.Fatalf("expected passed_prebuilt quality status, got %q", site.QualityStatus)
	}
	response := serveSiteRequest(service, "pristine-publish.device.intern.kim", "/")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Pristine Publish") {
		t.Fatalf("published body = %d %q", response.Code, response.Body.String())
	}
}

func TestSiteContentOnlyChangeRepublishesWithoutBuild(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "content-only-republish", Title: "Content Only"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.materializeSiteSourceWorkspace(context.Background(), site, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(site.HostSourcePath, "DESIGN.md"), validSiteDesignMarkdownWithColors("#101010", "#fefefe"))
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	updatedContentDocument := siteContentJSONDocument(&siteContent{
		SiteName: "Updated Name",
		Sections: []siteContentSection{{Title: "New", Body: "새 콘텐츠"}},
	})
	contentPath := filepath.Join(site.HostSourcePath, "app", "public", "site-content.json")
	writeFile(t, contentPath, updatedContentDocument)
	setFileModTime(t, contentPath, time.Now().UTC().Add(2*time.Hour))

	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response := serveSiteRequest(service, "content-only-republish.device.intern.kim", "/site-content.json")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "새 콘텐츠") {
		t.Fatalf("expected republished content overlay, status=%d body=%q", response.Code, response.Body.String())
	}
}

// TestSiteDesignDocumentChangeRepublishesWithoutBuild guards against a
// regression where writing app/DESIGN.md alone (no app/src or public/
// changes) falsely tripped the app/dist staleness check. DESIGN.md is
// rendered into theme.css at publish time (applySiteDesignTheme), not baked
// into the Vite build, so it must never require a rebuild — matching
// Blueclaw's own pathIsSiteDesignOrControlFile classification.
func TestSiteDesignDocumentChangeRepublishesWithoutBuild(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "design-only-republish", Title: "Design Only"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.materializeSiteSourceWorkspace(context.Background(), site, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	designPath := filepath.Join(site.HostSourcePath, "DESIGN.md")
	writeFile(t, designPath, validSiteDesignMarkdownWithColors("#101010", "#fefefe"))
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	writeFile(t, designPath, validSiteDesignMarkdownWithColors("#336699", "#ffffff"))
	setFileModTime(t, designPath, time.Now().UTC().Add(2*time.Hour))

	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatalf("expected DESIGN.md-only edit to publish without a rebuild, got %v", errorValue)
	}
}

func TestSiteModifiedSourceWithoutRebuildIsRejected(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "modified-source", Title: "Modified Source"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.materializeSiteSourceWorkspace(context.Background(), site, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	appSourcePath := filepath.Join(site.HostSourcePath, "app", "src", "App.tsx")
	writeFile(t, appSourcePath, "export default function App() { return <main>edited</main> }\n")
	setFileModTime(t, appSourcePath, time.Now().UTC().Add(2*time.Hour))

	if siteAppMatchesScaffoldManifest(site.HostSourcePath) {
		t.Fatal("expected an edited app/src/App.tsx to break the scaffold manifest match")
	}

	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "app/dist is stale") {
		t.Fatalf("expected stale build rejection for edited source without a rebuild, got %v", errorValue)
	}
}

func TestSitePublishRejectsInvalidApplicationContentFile(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "invalid-content", Title: "Invalid Content"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.materializeSiteSourceWorkspace(context.Background(), site, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(site.HostSourcePath, "app", "public", "site-content.json"), `{"siteName":"","sections":[]}`)

	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "app/public/site-content.json is invalid") {
		t.Fatalf("expected invalid content rejection, got %v", errorValue)
	}
}

func TestSitePublishAcceptsBlocksApplicationContentFile(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "blocks-content", Title: "Blocks Content"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.materializeSiteSourceWorkspace(context.Background(), site, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(site.HostSourcePath, "DESIGN.md"), validSiteDesignMarkdownWithColors("#101010", "#fefefe"))
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	blocksContentDocument := siteContentJSONDocument(&siteContent{
		SiteName: "Blocks Content",
		Tagline:  "블록 기반 콘텐츠",
		Blocks: []siteContentBlock{
			{Variant: "hero", Title: "Blocks Content", Body: "블록 기반 콘텐츠", ActionLabel: "자세히 보기", ActionHref: "#block-2"},
			{Variant: "features", Title: "기능", Items: []siteContentBlockItem{
				{Title: "빠름", Body: "빠른 프로토타입 생성"},
				{Title: "안전", Body: "안전한 배포 검증"},
			}},
			{Variant: "faq", Title: "자주 묻는 질문", Items: []siteContentBlockItem{
				{Title: "무료인가요?", Body: "네, 검증용 프로토타입은 무료입니다."},
			}},
		},
	})
	contentPath := filepath.Join(site.HostSourcePath, "app", "public", "site-content.json")
	writeFile(t, contentPath, blocksContentDocument)
	setFileModTime(t, contentPath, time.Now().UTC().Add(2*time.Hour))

	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response := serveSiteRequest(service, "blocks-content.device.intern.kim", "/site-content.json")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"blocks"`) {
		t.Fatalf("expected republished blocks content overlay, status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestSitePublishRejectsApplicationContentFileWithUnknownBlockVariant(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "invalid-block-variant", Title: "Invalid Block Variant"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.materializeSiteSourceWorkspace(context.Background(), site, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(site.HostSourcePath, "app", "public", "site-content.json"), `{"siteName":"Invalid Block Variant","blocks":[{"variant":"testimonial","title":"Bad"}]}`)

	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "app/public/site-content.json is invalid") {
		t.Fatalf("expected invalid block variant rejection, got %v", errorValue)
	}
}

func TestValidateSiteContentAcceptsBlocksOrSectionsButRequiresOne(t *testing.T) {
	if errorValue := validateSiteContent(&siteContent{SiteName: "Sections Only", Sections: []siteContentSection{{Title: "소개", Body: "내용"}}}); errorValue != nil {
		t.Fatalf("expected sections-only content to validate, got %v", errorValue)
	}
	if errorValue := validateSiteContent(&siteContent{SiteName: "Blocks Only", Blocks: []siteContentBlock{{Variant: "hero", Title: "제목"}}}); errorValue != nil {
		t.Fatalf("expected blocks-only content to validate, got %v", errorValue)
	}
	if errorValue := validateSiteContent(&siteContent{SiteName: "Blocks With Items", Blocks: []siteContentBlock{
		{Variant: "faq", Items: []siteContentBlockItem{{Title: "질문", Body: "답변"}}},
	}}); errorValue != nil {
		t.Fatalf("expected blocks with valid items to validate, got %v", errorValue)
	}
	if errorValue := validateSiteContent(&siteContent{SiteName: "Neither"}); errorValue == nil {
		t.Fatal("expected an error when neither sections nor blocks are present")
	}
	if errorValue := validateSiteContent(&siteContent{SiteName: "Unknown Variant", Blocks: []siteContentBlock{{Variant: "testimonial"}}}); errorValue == nil {
		t.Fatal("expected an error for an unknown block variant")
	}
	if errorValue := validateSiteContent(&siteContent{SiteName: "Empty Item", Blocks: []siteContentBlock{
		{Variant: "faq", Items: []siteContentBlockItem{{Title: "", Body: "답변"}}},
	}}); errorValue == nil {
		t.Fatal("expected an error for a block item missing a title")
	}
}

func TestSiteCreateRejectsDuplicateSlug(t *testing.T) {
	service, _ := newTestSiteService(t)
	_, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "portfolio", RequestedBy: "owner@example.com"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "portfolio", RequestedBy: "owner@example.com"})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "site slug already exists") {
		t.Fatalf("expected duplicate slug rejection, got %v", errorValue)
	}
}

func TestSiteCreateRejectsConcurrentDuplicateSlug(t *testing.T) {
	service, _ := newTestSiteService(t)
	errorsByRequest := make(chan error, 2)
	start := make(chan struct{})
	waitGroup := sync.WaitGroup{}
	for range 2 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			_, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "portfolio"})
			errorsByRequest <- errorValue
		}()
	}
	close(start)
	waitGroup.Wait()
	close(errorsByRequest)

	successCount := 0
	duplicateCount := 0
	for errorValue := range errorsByRequest {
		if errorValue == nil {
			successCount++
			continue
		}
		if strings.Contains(errorValue.Error(), "site slug already exists") {
			duplicateCount++
		}
	}
	if successCount != 1 || duplicateCount != 1 {
		t.Fatalf("expected one create and one duplicate rejection, successes=%d duplicates=%d", successCount, duplicateCount)
	}
}

func TestSiteCreateMaterializationFailureLeavesNoRecordAndCanRetry(t *testing.T) {
	service, _ := newTestSiteService(t)
	runCommand := service.RunCommand
	shouldFailMaterialization := true
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		if shouldFailMaterialization && name == "git" {
			return nil, errors.New("git initialization failed")
		}
		return runCommand(ctx, name, arguments...)
	}

	createSite := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/admin/api/sites", strings.NewReader(`{"slug":"retryable-site","title":"Retryable Site"}`))
		response := httptest.NewRecorder()
		service.createSite(response, request)
		return response
	}

	failedResponse := createSite()
	if failedResponse.Code != http.StatusInternalServerError {
		t.Fatalf("failed create status = %d, body = %q", failedResponse.Code, failedResponse.Body.String())
	}
	if site := service.findSiteBySlug("retryable-site"); site != nil {
		t.Fatalf("failed create left a site record: %+v", site)
	}
	if usedPorts := service.usedSitePorts(); len(usedPorts) != 0 {
		t.Fatalf("failed create left reserved ports: %+v", usedPorts)
	}
	if strings.Contains(readTrimmedFile(service.siteRegistryPath()), "retryable-site") {
		t.Fatal("failed create persisted the slug")
	}
	staffSitesPath := filepath.Join(service.Configuration.BlueclawWorkspacePath, "circles", "staff", "sites")
	if directoryHasEntries(filepath.Join(staffSitesPath, siteIDStorageDirectoryName)) {
		t.Fatal("failed create left staged site storage")
	}
	if _, errorValue := os.Lstat(filepath.Join(staffSitesPath, "retryable-site")); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("failed create left the slug alias: %v", errorValue)
	}
	if directoryHasEntries(filepath.Join(filepath.Dir(service.Configuration.SitesRoot), "site-sources")) {
		t.Fatal("failed create left staged source files")
	}

	shouldFailMaterialization = false
	retryResponse := createSite()
	if retryResponse.Code != http.StatusOK {
		t.Fatalf("retry status = %d, body = %q", retryResponse.Code, retryResponse.Body.String())
	}
	if site := service.findSiteBySlug("retryable-site"); site == nil {
		t.Fatal("retry did not create the site")
	}
}

func TestSiteCreateRequiresExplicitSlug(t *testing.T) {
	service, _ := newTestSiteService(t)
	for _, payload := range []siteCreateRequest{
		{Title: "Portfolio", RequestedBy: "owner@example.com"},
		{Slug: "Portfolio", RequestedBy: "owner@example.com"},
	} {
		_, errorValue := service.createSiteRecord(context.Background(), payload)
		if errorValue == nil || !strings.Contains(errorValue.Error(), "site slug is required") {
			t.Fatalf("expected canonical explicit slug requirement, got %v", errorValue)
		}
	}
}

func TestSiteCreateDoesNotReuseConversationSite(t *testing.T) {
	service, _ := newTestSiteService(t)
	firstSite, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "first-site", ConversationID: "thread-1"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	secondSite, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "second-site", ConversationID: "thread-1"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if firstSite.SiteID == secondSite.SiteID {
		t.Fatalf("expected distinct exact site identities, got %s", firstSite.SiteID)
	}
}

func TestSiteCreateIgnoresStaleStaffCircleSourceWorkspacePath(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
		Slug:                "current-site",
		SourceWorkspacePath: "/workspace/circles/staff/sites/other-site/draft",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.SourceWorkspacePath != "/workspace/circles/staff/sites/current-site/draft" {
		t.Fatalf("expected source workspace path to use current site alias, got %q", site.SourceWorkspacePath)
	}
	if site.WorkspacePath != "/workspace/circles/staff/sites/current-site" {
		t.Fatalf("expected workspace path to use current site alias, got %q", site.WorkspacePath)
	}
}

func TestSitePublishRejectsBodyPathSiteIDMismatch(t *testing.T) {
	service := &Service{}
	request := httptest.NewRequest(http.MethodPost, "/admin/api/sites/site-1/publish", strings.NewReader(`{"siteID":"site-2"}`))
	response := httptest.NewRecorder()

	service.publishSiteFromRequest(response, request, "site-1")

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "siteID does not match request path") {
		t.Fatalf("expected exact siteID rejection, status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestSitePublishMaterializesEditableSourceBundle(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
		Slug:        "source-bundle",
		Title:       "Source Bundle",
		RequestedBy: "owner@example.com",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	sourceWorkspacePath := t.TempDir()
	writeTestSourceBuild(t, sourceWorkspacePath, "bundle publish")
	customSourceDesignMarkdown := validSiteDesignMarkdownWithMarker("custom source design")
	writeFile(t, filepath.Join(sourceWorkspacePath, "DESIGN.md"), customSourceDesignMarkdown)
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
	if readTrimmedFile(filepath.Join(site.HostSourcePath, "DESIGN.md")) != strings.TrimSpace(customSourceDesignMarkdown) {
		t.Fatalf("host staging should be materialized from editable source")
	}
	response := serveSiteRequest(service, "source-bundle.device.intern.kim", "/")
	if !strings.Contains(response.Body.String(), "bundle publish") {
		t.Fatalf("published body = %q", response.Body.String())
	}
}

func TestSitePublishAllowsQualityIssuesWithFreshBuild(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
		Slug:        "quality-report",
		Title:       "Quality Report",
		RequestedBy: "owner@example.com",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	sourceWorkspacePath := t.TempDir()
	writeTestSourceBuild(t, sourceWorkspacePath, "quality publish")
	writeFile(t, filepath.Join(sourceWorkspacePath, "DESIGN.md"), siteDesignMD(site))
	writeFile(t, filepath.Join(sourceWorkspacePath, ".internkim", "build-quality.json"), `{
  "blockingIssueCount": 1,
  "issues": [
    {
      "severity": "warning",
      "category": "visualHierarchy",
      "target": "src/App.tsx",
      "message": "Tighten the visual hierarchy.",
      "suggestedFix": "Improve heading and card hierarchy."
    }
  ]
}`)
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		RequestedBy:         "owner@example.com",
		Message:             "Publish with quality report",
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, sourceWorkspacePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.QualityStatus != "needs_improvement" || site.QualityIssueCount != 1 {
		t.Fatalf("expected publish to retain quality warning metadata, got %+v", site)
	}
	if len(site.QualitySummary) == 0 || !strings.Contains(site.QualitySummary[0], "src/App.tsx") {
		t.Fatalf("expected quality summary to name affected source, got %+v", site.QualitySummary)
	}
	response := serveSiteRequest(service, "quality-report.device.intern.kim", "/")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "quality publish") {
		t.Fatalf("expected published site despite quality warnings, status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestSiteCreateStoresMetadataOwnershipAndIdeaMirror(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
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
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
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
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "api-demo"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "frontend")
	if errorValue := os.MkdirAll(filepath.Join(site.HostSourcePath, "pocketbase", "pb_migrations"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(site.HostSourcePath, "pocketbase", "pb_migrations", "001_init.js"), "migrate(() => {})")
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
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

	response := serveSiteRequest(service, "api-demo.device.intern.kim", "/api/collections/posts/records")
	if response.Code != http.StatusAccepted {
		t.Fatalf("pocketbase proxy status = %d", response.Code)
	}
	if response.Body.String() != "pocketbase" {
		t.Fatalf("pocketbase proxy body = %q", response.Body.String())
	}
}

func publishStaticTestSite(t *testing.T, service *Service, slug string) *SiteRecord {
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: slug})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "frontend")
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return site
}

func publishPocketBaseTestSite(t *testing.T, service *Service, slug string) *SiteRecord {
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: slug})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "frontend")
	if errorValue := os.MkdirAll(filepath.Join(site.HostSourcePath, "pocketbase", "pb_migrations"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(site.HostSourcePath, "pocketbase", "pb_migrations", "001_init.js"), "migrate(() => {})")
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return site
}

func TestSitePublishDisablesPocketBaseForStaticSite(t *testing.T) {
	service, commandLog := newTestSiteService(t)
	site := publishStaticTestSite(t, service, "static-pub")
	if !containsCommand(*commandLog, "systemctl disable --now "+siteServiceName(site.SiteID)) {
		t.Fatalf("static publish should disable pocketbase service: %+v", *commandLog)
	}
	if containsCommand(*commandLog, "systemctl enable --now "+siteServiceName(site.SiteID)) {
		t.Fatalf("static publish should not enable pocketbase service: %+v", *commandLog)
	}
	if containsCommand(*commandLog, "systemctl restart "+siteServiceName(site.SiteID)) {
		t.Fatalf("static publish should not restart pocketbase service: %+v", *commandLog)
	}
}

func TestSitePublishEnablesPocketBaseForBackendSite(t *testing.T) {
	service, commandLog := newTestSiteService(t)
	site := publishPocketBaseTestSite(t, service, "db-pub")
	if !containsCommand(*commandLog, "systemctl disable "+siteServiceName(site.SiteID)) {
		t.Fatalf("db publish should leave pocketbase service disabled for lazy start: %+v", *commandLog)
	}
	if !containsCommand(*commandLog, "systemctl restart "+siteServiceName(site.SiteID)) {
		t.Fatalf("db publish should restart pocketbase service: %+v", *commandLog)
	}
	if containsCommand(*commandLog, "systemctl disable --now "+siteServiceName(site.SiteID)) {
		t.Fatalf("db publish should not disable pocketbase service: %+v", *commandLog)
	}
}

func TestStaticSitePocketBasePathReturnsNotFound(t *testing.T) {
	service, _ := newTestSiteService(t)
	publishStaticTestSite(t, service, "static-api")
	response := serveSiteRequest(service, "static-api.device.intern.kim", "/api/collections/posts/records")
	if response.Code != http.StatusNotFound {
		t.Fatalf("static site pocketbase path status = %d, want 404", response.Code)
	}
}

func TestReconcileDisablesStaticPublishedSitesOnly(t *testing.T) {
	service, commandLog := newTestSiteService(t)
	staticSite := publishStaticTestSite(t, service, "recon-static")
	databaseSite := publishPocketBaseTestSite(t, service, "recon-db")
	*commandLog = nil
	service.reconcilePublishedSitePocketBaseRuntimes(context.Background())
	if !containsCommand(*commandLog, "systemctl disable --now "+siteServiceName(staticSite.SiteID)) {
		t.Fatalf("reconcile should disable static published site: %+v", *commandLog)
	}
	if containsCommand(*commandLog, "systemctl disable --now "+siteServiceName(databaseSite.SiteID)) {
		t.Fatalf("reconcile should not disable database-backed site: %+v", *commandLog)
	}
}

func TestReconcileSkipsUnpublishedSites(t *testing.T) {
	service, commandLog := newTestSiteService(t)
	site := publishStaticTestSite(t, service, "recon-unpub")
	if _, errorValue := service.unpublishSite(context.Background(), site.SiteID, siteLifecycleRequest{UserConfirmed: true}); errorValue != nil {
		t.Fatal(errorValue)
	}
	*commandLog = nil
	service.reconcilePublishedSitePocketBaseRuntimes(context.Background())
	if containsCommand(*commandLog, "systemctl disable --now "+siteServiceName(site.SiteID)) {
		t.Fatalf("reconcile should skip non-published site: %+v", *commandLog)
	}
}

func TestReconcileSiteRuntimeTogglesByBackendMarker(t *testing.T) {
	service, commandLog := newTestSiteService(t)
	site := publishStaticTestSite(t, service, "toggle")
	versionID := site.CurrentVersionID
	versionPath := service.sitePublishedVersionPath(site, versionID)

	*commandLog = nil
	if errorValue := service.reconcileSitePocketBaseRuntime(context.Background(), site, versionID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !containsCommand(*commandLog, "systemctl disable --now "+siteServiceName(site.SiteID)) {
		t.Fatalf("static version should disable pocketbase: %+v", *commandLog)
	}
	if containsCommand(*commandLog, "systemctl enable --now "+siteServiceName(site.SiteID)) {
		t.Fatalf("static version should not enable pocketbase: %+v", *commandLog)
	}

	if errorValue := os.MkdirAll(filepath.Join(versionPath, "pb_migrations"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(versionPath, "pb_migrations", "001_init.js"), "migrate(() => {})")
	*commandLog = nil
	if errorValue := service.reconcileSitePocketBaseRuntime(context.Background(), site, versionID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !containsCommand(*commandLog, "systemctl disable "+siteServiceName(site.SiteID)) {
		t.Fatalf("backend version should leave pocketbase disabled for lazy start: %+v", *commandLog)
	}
	if !containsCommand(*commandLog, "systemctl restart "+siteServiceName(site.SiteID)) {
		t.Fatalf("backend version should restart pocketbase: %+v", *commandLog)
	}
	if containsCommand(*commandLog, "systemctl disable --now "+siteServiceName(site.SiteID)) {
		t.Fatalf("backend version should not disable pocketbase: %+v", *commandLog)
	}
}

func TestReconcileSkipsPublishingSites(t *testing.T) {
	service, commandLog := newTestSiteService(t)
	site := publishStaticTestSite(t, service, "recon-publishing")
	site.Status = SiteStatusPublishing
	if errorValue := service.storeSite(site); errorValue != nil {
		t.Fatal(errorValue)
	}
	*commandLog = nil
	service.reconcilePublishedSitePocketBaseRuntimes(context.Background())
	if containsCommand(*commandLog, "systemctl disable --now "+siteServiceName(site.SiteID)) {
		t.Fatalf("reconcile should skip publishing site: %+v", *commandLog)
	}
}

func TestSiteRegistryPersistsAndAllocatesDistinctPorts(t *testing.T) {
	service, _ := newTestSiteService(t)
	firstSite, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "first"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	secondSite, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "second"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if firstSite.Port == secondSite.Port {
		t.Fatalf("expected distinct ports, got %d", firstSite.Port)
	}

	reloadedService := NewService(service.Configuration)
	reloadedSite := reloadedService.findSiteBySlug(firstSite.Slug)
	if reloadedSite == nil {
		t.Fatalf("reloaded site missing")
	}
	if reloadedSite.Port != firstSite.Port {
		t.Fatalf("reloaded port = %d", reloadedSite.Port)
	}
}

func TestLoadSitesReconcilesInterruptedPublishingToFailed(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "interrupted"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service.updateSiteStatus(site.SiteID, SiteStatusPublishing, "")

	reloadedService := NewService(service.Configuration)
	reloadedSite := reloadedService.findSiteByID(site.SiteID)
	if reloadedSite == nil {
		t.Fatal("reloaded site missing")
	}
	if reloadedSite.Status != SiteStatusFailed {
		t.Fatalf("expected interrupted publishing site to load as failed, got %q", reloadedSite.Status)
	}
	if reloadedSite.LastError == "" {
		t.Fatal("expected reconciled site to carry a lastError")
	}
}

func TestFailedNeverPublishedSiteReleasesPort(t *testing.T) {
	service, _ := newTestSiteService(t)
	failedSite, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "failed-never-published"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service.updateSiteStatus(failedSite.SiteID, SiteStatusFailed, "build failed before publish")

	if service.usedSitePorts()[failedSite.Port] {
		t.Fatalf("expected failed never-published site to release port %d", failedSite.Port)
	}

	nextSite, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "reuses-port"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if nextSite.Port != failedSite.Port {
		t.Fatalf("expected reclaimed port %d to be reused, got %d", failedSite.Port, nextSite.Port)
	}
}

func TestPublishedFailedSiteKeepsPort(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "published-then-failed"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	liveSite := service.findSiteByID(site.SiteID)
	liveSite.CurrentVersionID = "v-live"
	liveSite.Status = SiteStatusFailed
	liveSite.LastError = "redeploy failed"
	if errorValue := service.storeSite(liveSite); errorValue != nil {
		t.Fatal(errorValue)
	}

	if !service.usedSitePorts()[site.Port] {
		t.Fatalf("expected failed site with a live version to keep port %d", site.Port)
	}
}

func TestSiteWorkspaceIsWritableByRequesterTerminal(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "terminal-writable"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.SourceWorkspacePath != "/workspace/circles/staff/sites/terminal-writable/draft" {
		t.Fatalf("expected staff-circle draft workspace, got %q", site.SourceWorkspacePath)
	}
	if site.WorkspacePath != "/workspace/circles/staff/sites/terminal-writable" {
		t.Fatalf("workspace path should point at project root: %+v", site)
	}
	if site.AppWorkspacePath != site.SourceWorkspacePath+"/app" {
		t.Fatalf("app workspace path should point at app source: %+v", site)
	}
	storagePath := service.siteProjectStorageHostPath(site.SiteID)
	if information, errorValue := os.Stat(storagePath); errorValue != nil || !information.IsDir() {
		t.Fatalf("expected hidden siteID storage directory at %s: %v", storagePath, errorValue)
	}
	staffSitesPath := filepath.Join(service.Configuration.BlueclawWorkspacePath, "circles", "staff", "sites")
	assertStaffCircleDirectoryMode(t, staffSitesPath)
	assertStaffCircleDirectoryMode(t, filepath.Join(staffSitesPath, siteIDStorageDirectoryName))
	assertStaffCircleDirectoryMode(t, storagePath)
	aliasPath := service.siteProjectAliasHostPath(site)
	aliasInformation, errorValue := os.Lstat(aliasPath)
	if errorValue != nil {
		t.Fatalf("expected slug alias at %s: %v", aliasPath, errorValue)
	}
	if aliasInformation.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected slug alias to be symlink, got mode %v", aliasInformation.Mode())
	}
	targetPath, errorValue := os.Readlink(aliasPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	resolvedTargetPath := filepath.Clean(filepath.Join(filepath.Dir(aliasPath), targetPath))
	if resolvedTargetPath != filepath.Clean(storagePath) {
		t.Fatalf("alias target = %q, want %q", resolvedTargetPath, storagePath)
	}
}

func TestWriteSiteRepairsBrokenSlugAliasDirectory(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "read-repairs-alias"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	breakSiteAliasDirectory(t, service, site, "read repaired")

	response := writeSiteResponse(service, site.SiteID)
	if response.Code != http.StatusOK {
		t.Fatalf("response status = %d body = %q", response.Code, response.Body.String())
	}

	assertSiteAliasSymlinkTarget(t, service, site)
	migratedFilePath := filepath.Join(service.siteProjectStorageHostPath(site.SiteID), "draft", "app", "src", "App.tsx")
	content, errorValue := os.ReadFile(migratedFilePath)
	if errorValue != nil {
		t.Fatalf("expected migrated source: %v", errorValue)
	}
	if string(content) != "read repaired" {
		t.Fatalf("migrated source = %q", string(content))
	}
	expectedSourcePath := "~/sites/" + site.SiteID + "/draft"
	if !strings.Contains(response.Body.String(), expectedSourcePath) {
		t.Fatalf("response should expose the personal source workspace path %q: %s", expectedSourcePath, response.Body.String())
	}
}

func TestSiteListRepairsBrokenSlugAliasDirectory(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "list-repairs-alias"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	breakSiteAliasDirectory(t, service, site, "list repaired")

	response := writeSiteListResponse(service)
	if response.Code != http.StatusOK {
		t.Fatalf("response status = %d body = %q", response.Code, response.Body.String())
	}

	assertSiteAliasSymlinkTarget(t, service, site)
	migratedFilePath := filepath.Join(service.siteProjectStorageHostPath(site.SiteID), "draft", "app", "src", "App.tsx")
	content, errorValue := os.ReadFile(migratedFilePath)
	if errorValue != nil {
		t.Fatalf("expected migrated source: %v", errorValue)
	}
	if string(content) != "list repaired" {
		t.Fatalf("migrated source = %q", string(content))
	}
}

func TestSitePublishRepairsStaffCircleSiteWorkspacePermissions(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "publish-repairs-permissions"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	staffSitesPath := filepath.Join(service.Configuration.BlueclawWorkspacePath, "circles", "staff", "sites")
	storageRootPath := filepath.Join(staffSitesPath, siteIDStorageDirectoryName)
	storagePath := service.siteProjectStorageHostPath(site.SiteID)
	for _, path := range []string{staffSitesPath, storageRootPath, storagePath} {
		if errorValue := os.Chmod(path, 0o700); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	writeTestWorkspaceBuild(t, site, "published after permission repair")

	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	assertStaffCircleDirectoryMode(t, staffSitesPath)
	assertStaffCircleDirectoryMode(t, storageRootPath)
	assertStaffCircleDirectoryMode(t, storagePath)
}

func TestSiteDeleteRequiresExplicitConfirmation(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "delete-me"})
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
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "workspace-only"})
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
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "stale-build"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "first build")
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	sourcePath := filepath.Join(site.HostSourcePath, "app", "src", "App.tsx")
	writeFile(t, sourcePath, "export default function App() {\n  return <main>updated source</main>;\n}\n")
	sourceModTime := time.Now().UTC().Add(2 * time.Hour)
	setFileModTime(t, sourcePath, sourceModTime)
	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "app/dist is stale") {
		t.Fatalf("expected stale build rejection, got %v", errorValue)
	}

	writeTestWorkspaceBuild(t, site, "fresh build")
	setDirectoryFilesModTime(t, filepath.Join(site.HostSourcePath, "app", "dist"), sourceModTime.Add(time.Hour))
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response := serveSiteRequest(service, "stale-build.device.intern.kim", "/")
	if !strings.Contains(response.Body.String(), "fresh build") {
		t.Fatalf("published body = %q", response.Body.String())
	}
}

func TestSitePublishRejectsUnapprovedPocketBaseHooks(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "hook-demo"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "workspace")
	writeFile(t, filepath.Join(site.HostSourcePath, "pocketbase", "pb_hooks", "main.pb.js"), "routerAdd('GET', '/x', () => {})")
	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue == nil {
		t.Fatal("expected unapproved PocketBase hook rejection")
	}
}

func TestSitePublishIgnoresPocketBaseHookMetadataFiles(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "hook-metadata-demo"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeTestWorkspaceBuild(t, site, "workspace")
	writeFile(t, filepath.Join(site.HostSourcePath, "pocketbase", "pb_hooks", ".gitkeep"), "")
	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat:  "tar.gz",
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
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "owned", RequestedBy: "owner@example.com"})
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

func writeSiteResponse(service *Service, siteID string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	service.writeSite(response, siteID)
	return response
}

func assertStaffCircleDirectoryMode(t *testing.T, path string) {
	t.Helper()
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		t.Fatalf("expected staff-circle directory at %s: %v", path, errorValue)
	}
	if !information.IsDir() {
		t.Fatalf("expected staff-circle path to be a directory: %s", path)
	}
	if information.Mode().Perm() != 0o770 || information.Mode()&os.ModeSetgid == 0 {
		t.Fatalf("staff-circle directory mode at %s = %v, want setgid 0770", path, information.Mode())
	}
}

func writeSiteListResponse(service *Service) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/api/sites", nil)
	service.listSites(response, request)
	return response
}

func breakSiteAliasDirectory(t *testing.T, service *Service, site *SiteRecord, content string) {
	t.Helper()
	aliasPath := service.siteProjectAliasHostPath(site)
	if errorValue := os.Remove(aliasPath); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeSiteSourceFile(t, filepath.Join(aliasPath, "draft", "app", "src", "App.tsx"), content)
	if errorValue := os.Chmod(aliasPath, 0); errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() {
		_ = os.Chmod(aliasPath, os.ModeSetgid|0o770)
	})
}

func assertSiteAliasSymlinkTarget(t *testing.T, service *Service, site *SiteRecord) {
	t.Helper()
	storagePath := service.siteProjectStorageHostPath(site.SiteID)
	aliasPath := service.siteProjectAliasHostPath(site)
	aliasInformation, errorValue := os.Lstat(aliasPath)
	if errorValue != nil {
		t.Fatalf("expected slug alias at %s: %v", aliasPath, errorValue)
	}
	if aliasInformation.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected slug alias to be symlink, got mode %v", aliasInformation.Mode())
	}
	targetPath, errorValue := os.Readlink(aliasPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	resolvedTargetPath := filepath.Clean(filepath.Join(filepath.Dir(aliasPath), targetPath))
	if resolvedTargetPath != filepath.Clean(storagePath) {
		t.Fatalf("alias target = %q, want %q", resolvedTargetPath, storagePath)
	}
}

func publishSiteResponse(t *testing.T, service *Service, site *SiteRecord) *httptest.ResponseRecorder {
	t.Helper()
	requestBody := `{"requestedBy":"owner@example.com","sourceWorkspacePath":"` + site.SourceWorkspacePath + `","sourceBundleBase64":"` + testSourceBundleBase64(t, site.HostSourcePath) + `","sourceBundleFormat":"tar.gz"}`
	request := httptest.NewRequest(http.MethodPost, "/admin/api/sites/"+site.SiteID+"/publish", strings.NewReader(requestBody))
	response := httptest.NewRecorder()
	service.publishSiteFromRequest(response, request, site.SiteID)
	if response.Code != http.StatusOK {
		t.Fatalf("publish status = %d body = %q", response.Code, response.Body.String())
	}
	return response
}

func assertSiteResponseOmitsPublishedURL(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("response status = %d body = %q", response.Code, response.Body.String())
	}
	for _, site := range siteJSONObjects(t, response) {
		if _, isPresent := site["publishedURL"]; isPresent {
			t.Fatalf("publishedURL should be omitted from response: %s", response.Body.String())
		}
	}
}

func assertSiteResponseIncludesPublishedURL(t *testing.T, response *httptest.ResponseRecorder, expected string) {
	t.Helper()
	site := siteJSONObjects(t, response)[0]
	if site["publishedURL"] != expected {
		t.Fatalf("publishedURL = %v, expected %q in %s", site["publishedURL"], expected, response.Body.String())
	}
}

func siteJSONObjects(t *testing.T, response *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var object map[string]any
	if errorValue := json.Unmarshal(response.Body.Bytes(), &object); errorValue != nil {
		t.Fatal(errorValue)
	}
	if sites, isList := object["sites"].([]any); isList {
		return siteListJSONObjects(t, sites)
	}
	return []map[string]any{object}
}

func siteListJSONObjects(t *testing.T, sites []any) []map[string]any {
	t.Helper()
	result := []map[string]any{}
	for _, site := range sites {
		siteObject, isObject := site.(map[string]any)
		if !isObject {
			t.Fatalf("site response entry = %#v", site)
		}
		result = append(result, siteObject)
	}
	return result
}

func siteRestartErrorCommand(contextValue context.Context, name string, arguments ...string) ([]byte, error) {
	_ = contextValue
	if name == "systemctl" && strings.HasPrefix(strings.Join(arguments, " "), "restart") {
		return nil, errors.New("restart unavailable")
	}
	return []byte("ok"), nil
}

func writeTestWorkspaceBuild(t *testing.T, site *SiteRecord, body string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Join(site.HostSourcePath, "app", "src"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.RemoveAll(filepath.Join(site.HostSourcePath, "app", "dist")); errorValue != nil {
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
	writeFile(t, filepath.Join(site.HostSourcePath, "DESIGN.md"), validSiteDesignMarkdownWithColors("#101010", "#fefefe"))
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

func repositoryPath(pathParts ...string) string {
	return filepath.Join(append([]string{"..", ".."}, pathParts...)...)
}

func readRepositoryFile(t *testing.T, pathParts ...string) string {
	t.Helper()
	document, errorValue := os.ReadFile(repositoryPath(pathParts...))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(document)
}

func assertDirectoriesMatch(t *testing.T, expectedRootPath string, actualRootPath string) {
	t.Helper()
	expectedFiles := readDirectoryFiles(t, expectedRootPath)
	actualFiles := readDirectoryFiles(t, actualRootPath)
	for relativePath, expectedContent := range expectedFiles {
		actualContent, isFound := actualFiles[relativePath]
		if !isFound {
			t.Fatalf("%s missing mirrored scaffold file %s", actualRootPath, relativePath)
		}
		if actualContent != expectedContent {
			t.Fatalf("%s differs from canonical scaffold file %s", actualRootPath, relativePath)
		}
	}
	for relativePath := range actualFiles {
		if _, isFound := expectedFiles[relativePath]; !isFound {
			t.Fatalf("%s has extra scaffold file %s", actualRootPath, relativePath)
		}
	}
}

func readDirectoryFiles(t *testing.T, rootPath string) map[string]string {
	t.Helper()
	files := map[string]string{}
	errorValue := filepath.Walk(rootPath, func(path string, information os.FileInfo, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if information.IsDir() {
			return nil
		}
		if !information.Mode().IsRegular() {
			return nil
		}
		relativePath, errorValue := filepath.Rel(rootPath, path)
		if errorValue != nil {
			return errorValue
		}
		document, errorValue := os.ReadFile(path)
		if errorValue != nil {
			return errorValue
		}
		files[filepath.ToSlash(relativePath)] = string(document)
		return nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return files
}

func TestSiteSourcePathMatchesSiteAcceptsAdvertisedForms(t *testing.T) {
	site := &SiteRecord{
		SiteID:              "abc123",
		Slug:                "demo",
		SourceWorkspacePath: "/workspace/circles/staff/sites/demo/draft",
	}

	acceptedPaths := []string{
		"/workspace/circles/staff/sites/demo/draft",
		"home/sites/abc123/draft",
		"/workspace/private/people/person-1/sites/abc123/draft",
	}
	for _, path := range acceptedPaths {
		if !siteSourcePathMatchesSite(site, path) {
			t.Fatalf("expected %q to match site", path)
		}
	}
	rejectedPaths := []string{
		"",
		"home/sites/other456/draft",
		"/workspace/private/people/person-1/sites/other456/draft",
		"/workspace/shared/anything",
	}
	for _, path := range rejectedPaths {
		if siteSourcePathMatchesSite(site, path) {
			t.Fatalf("expected %q to be rejected", path)
		}
	}
}

func TestValidateSiteStagingPathsQuarantinesOrphanedAlias(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.newUnpersistedSiteRecord(siteCreateRequest{
		Slug:        "orphan-demo",
		RequestedBy: "owner@example.com",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	orphanPath := service.siteProjectAliasHostPath(site)
	if errorValue := os.MkdirAll(orphanPath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.validateSiteStagingPaths(site); errorValue != nil {
		t.Fatalf("expected orphaned staging path to be quarantined, got %v", errorValue)
	}
	if _, errorValue := os.Lstat(orphanPath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("expected orphan path to be moved aside, got %v", errorValue)
	}
}

func TestValidateSiteStagingPathsRejectsClaimedAlias(t *testing.T) {
	service, _ := newTestSiteService(t)
	existingSite, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{
		Slug:        "claimed-demo",
		RequestedBy: "owner@example.com",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	duplicateSite := &SiteRecord{
		SiteID:         "different-id",
		Slug:           existingSite.Slug,
		HostSourcePath: service.siteSourceLedgerPath("different-id"),
	}

	errorValue = service.validateSiteStagingPaths(duplicateSite)

	if errorValue == nil || !strings.Contains(errorValue.Error(), "already exists") {
		t.Fatalf("expected claimed staging path rejection, got %v", errorValue)
	}
}

func TestFrontendBuildFreshnessIgnoresDesignDocumentEdits(t *testing.T) {
	workspacePath := t.TempDir()
	applicationPath := filepath.Join(workspacePath, "app")
	distPath := filepath.Join(applicationPath, "dist")
	if errorValue := os.MkdirAll(filepath.Join(applicationPath, "src"), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.MkdirAll(distPath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	buildTime := time.Now().Add(-time.Hour)
	sourcePath := filepath.Join(applicationPath, "src", "main.ts")
	if errorValue := os.WriteFile(sourcePath, []byte("export {}"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.Chtimes(sourcePath, buildTime.Add(-time.Minute), buildTime.Add(-time.Minute)); errorValue != nil {
		t.Fatal(errorValue)
	}
	builtPath := filepath.Join(distPath, "index.html")
	if errorValue := os.WriteFile(builtPath, []byte("<html></html>"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.Chtimes(builtPath, buildTime, buildTime); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(applicationPath, siteDesignDocumentPath), []byte("---\nstyle: editorial\n---\n"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := ensureSiteFrontendBuildIsFresh(workspacePath, distPath); errorValue != nil {
		t.Fatalf("expected a DESIGN.md edit after the build to stay fresh, got %v", errorValue)
	}
}
