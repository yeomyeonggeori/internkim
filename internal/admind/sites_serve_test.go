package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func serveSiteViaHTTP(t *testing.T, service *Service, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/admin/api/sites/serve", strings.NewReader(string(document)))
	response := httptest.NewRecorder()
	service.serveSiteFromRequest(response, request)
	return response
}

func stagedSiteSourceForServe(t *testing.T, service *Service) string {
	t.Helper()
	site, errorValue := service.createSiteRecord(context.Background(), siteCreateRequest{Slug: "serve-source-template", Title: "Serve Source Template"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.materializeSiteSourceWorkspace(context.Background(), site, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	return site.HostSourcePath
}

func TestServeSiteAllocatesSlugFromTitleWithDedup(t *testing.T) {
	service, _ := newTestSiteService(t)
	sourcePath := stagedSiteSourceForServe(t, service)

	serveOnce := func() map[string]any {
		response := serveSiteViaHTTP(t, service, map[string]any{
			"title":               "Fleet Status Board",
			"mode":                "publish",
			"sourceWorkspacePath": "~/sites/fleet-status-board",
			"sourceBundleBase64":  testSourceBundleBase64(t, sourcePath),
			"sourceBundleFormat":  "tar.gz",
			"requestedBy":         "owner@example.com",
		})
		if response.Code != http.StatusOK {
			t.Fatalf("serve status = %d body = %q", response.Code, response.Body.String())
		}
		record := map[string]any{}
		if errorValue := json.Unmarshal(response.Body.Bytes(), &record); errorValue != nil {
			t.Fatal(errorValue)
		}
		return record
	}

	firstRecord := serveOnce()
	if firstRecord["slug"] != "fleet-status-board" || firstRecord["status"] != SiteStatusPublished {
		t.Fatalf("unexpected first serve record: %+v", firstRecord)
	}
	if publishedURL, _ := firstRecord["publishedURL"].(string); !strings.Contains(publishedURL, "fleet-status-board") {
		t.Fatalf("unexpected published URL: %+v", firstRecord)
	}

	secondRecord := serveOnce()
	if secondRecord["slug"] != "fleet-status-board-2" {
		t.Fatalf("expected deduplicated slug, got %+v", secondRecord)
	}
	if secondRecord["siteID"] == firstRecord["siteID"] {
		t.Fatalf("expected a new site record for a referenceless serve, got %+v", secondRecord)
	}
}

func TestServeSiteWithReferenceUpdatesExistingSite(t *testing.T) {
	service, _ := newTestSiteService(t)
	sourcePath := stagedSiteSourceForServe(t, service)

	firstResponse := serveSiteViaHTTP(t, service, map[string]any{
		"title":               "Docs Portal",
		"mode":                "publish",
		"sourceWorkspacePath": "~/sites/docs-portal",
		"sourceBundleBase64":  testSourceBundleBase64(t, sourcePath),
		"sourceBundleFormat":  "tar.gz",
		"requestedBy":         "owner@example.com",
	})
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("first serve status = %d body = %q", firstResponse.Code, firstResponse.Body.String())
	}
	firstRecord := map[string]any{}
	if errorValue := json.Unmarshal(firstResponse.Body.Bytes(), &firstRecord); errorValue != nil {
		t.Fatal(errorValue)
	}

	updateResponse := serveSiteViaHTTP(t, service, map[string]any{
		"title":               "Docs Portal",
		"mode":                "publish",
		"siteReference":       "docs-portal",
		"sourceWorkspacePath": "~/sites/docs-portal",
		"sourceBundleBase64":  testSourceBundleBase64(t, sourcePath),
		"sourceBundleFormat":  "tar.gz",
		"requestedBy":         "owner@example.com",
	})
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update serve status = %d body = %q", updateResponse.Code, updateResponse.Body.String())
	}
	updateRecord := map[string]any{}
	if errorValue := json.Unmarshal(updateResponse.Body.Bytes(), &updateRecord); errorValue != nil {
		t.Fatal(errorValue)
	}
	if updateRecord["siteID"] != firstRecord["siteID"] || updateRecord["slug"] != "docs-portal" {
		t.Fatalf("expected the referenced site to be updated in place, got %+v", updateRecord)
	}
}

func TestServeSiteUnknownReferenceReturnsTypedNotFound(t *testing.T) {
	service, _ := newTestSiteService(t)
	sourcePath := stagedSiteSourceForServe(t, service)

	response := serveSiteViaHTTP(t, service, map[string]any{
		"title":               "Ghost Site",
		"mode":                "publish",
		"siteReference":       "missing-site",
		"sourceWorkspacePath": "~/sites/ghost-site",
		"sourceBundleBase64":  testSourceBundleBase64(t, sourcePath),
		"sourceBundleFormat":  "tar.gz",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("serve status = %d body = %q", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, `"status":"not_found"`) || !strings.Contains(body, `"siteReference":"missing-site"`) {
		t.Fatalf("expected typed not_found document, got %q", body)
	}
	if !strings.Contains(body, `"slug":"serve-source-template"`) {
		t.Fatalf("expected existing sites as candidates, got %q", body)
	}
}

func TestServeSitePreviewProducesPreviewURLWithoutPublishing(t *testing.T) {
	service, _ := newTestSiteService(t)
	sourcePath := stagedSiteSourceForServe(t, service)

	response := serveSiteViaHTTP(t, service, map[string]any{
		"title":               "Preview Board",
		"mode":                "preview",
		"sourceWorkspacePath": "~/sites/preview-board",
		"sourceBundleBase64":  testSourceBundleBase64(t, sourcePath),
		"sourceBundleFormat":  "tar.gz",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("serve status = %d body = %q", response.Code, response.Body.String())
	}
	record := map[string]any{}
	if errorValue := json.Unmarshal(response.Body.Bytes(), &record); errorValue != nil {
		t.Fatal(errorValue)
	}
	if record["status"] != SiteStatusDraft {
		t.Fatalf("expected preview serve to keep the site unpublished, got %+v", record)
	}
	previewURL, _ := record["previewURL"].(string)
	if !strings.Contains(previewURL, "/__preview/") {
		t.Fatalf("expected preview URL, got %+v", record)
	}
}

func TestServeSitePublishRejectsInvalidDesignDocument(t *testing.T) {
	service, _ := newTestSiteService(t)
	sourcePath := stagedSiteSourceForServe(t, service)
	writeFile(t, filepath.Join(sourcePath, "DESIGN.md"), "not a design document")

	response := serveSiteViaHTTP(t, service, map[string]any{
		"title":               "Broken Design",
		"mode":                "publish",
		"sourceWorkspacePath": "~/sites/broken-design",
		"sourceBundleBase64":  testSourceBundleBase64(t, sourcePath),
		"sourceBundleFormat":  "tar.gz",
	})
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "DESIGN.md front matter is invalid") {
		t.Fatalf("expected DESIGN.md fail-closed serve publish, got %d %q", response.Code, response.Body.String())
	}
}

func TestServeSiteValidatesModeTitleAndBundle(t *testing.T) {
	service, _ := newTestSiteService(t)

	invalidPayloads := []map[string]any{
		{"title": "A", "mode": "deploy", "sourceBundleBase64": "YQ=="},
		{"title": "A", "mode": "publish"},
		{"mode": "publish", "sourceBundleBase64": "YQ=="},
	}
	for _, payload := range invalidPayloads {
		response := serveSiteViaHTTP(t, service, payload)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("expected serve rejection for %+v, got %d %q", payload, response.Code, response.Body.String())
		}
	}
}

func TestSiteSlugFromTitleNormalizesAndFallsBack(t *testing.T) {
	testCases := map[string]string{
		"Fleet Status Board":       "fleet-status-board",
		"  Fleet   Status  ":       "fleet-status",
		"고객지원 결산":                  "site",
		"고객지원 Dashboard 2026":      "dashboard-2026",
		"Team's #1 (Best) Site!":   "team-s-1-best-site",
		strings.Repeat("very", 40): strings.Repeat("very", 15),
	}
	for title, expectedSlug := range testCases {
		if slug := siteSlugFromTitle(title); slug != expectedSlug {
			t.Fatalf("siteSlugFromTitle(%q) = %q, want %q", title, slug, expectedSlug)
		}
	}
}

func TestUnserveKeepsRequesterWorkspaceProject(t *testing.T) {
	service, _ := newTestSiteService(t)
	sourcePath := stagedSiteSourceForServe(t, service)

	response := serveSiteViaHTTP(t, service, map[string]any{
		"title":               "Ephemeral Site",
		"mode":                "publish",
		"sourceWorkspacePath": "~/sites/ephemeral-site",
		"sourceBundleBase64":  testSourceBundleBase64(t, sourcePath),
		"sourceBundleFormat":  "tar.gz",
		"requestedBy":         "owner@example.com",
		"ownerIdentity":       map[string]any{"personID": "person-1"},
	})
	if response.Code != http.StatusOK {
		t.Fatalf("serve status = %d body = %q", response.Code, response.Body.String())
	}
	record := map[string]any{}
	if errorValue := json.Unmarshal(response.Body.Bytes(), &record); errorValue != nil {
		t.Fatal(errorValue)
	}
	siteID, _ := record["siteID"].(string)
	site := service.findSiteByID(siteID)
	if site == nil {
		t.Fatal("served site record is missing")
	}

	ownerProjectPath := service.siteOwnerProjectPath(site)
	if errorValue := os.MkdirAll(ownerProjectPath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(ownerProjectPath, "draft.txt"), "requester draft")

	deletedSite, errorValue := service.deleteSite(context.Background(), siteID, siteLifecycleRequest{
		RequestedBy:   "owner@example.com",
		Confirm:       "DELETE",
		UserConfirmed: true,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if deletedSite.Status != SiteStatusDeleted {
		t.Fatalf("expected deleted status, got %+v", deletedSite)
	}
	if _, errorValue := os.Stat(filepath.Join(ownerProjectPath, "draft.txt")); errorValue != nil {
		t.Fatalf("unserve must not touch requester workspace files: %v", errorValue)
	}
	if _, errorValue := os.Stat(site.HostSourcePath); !os.IsNotExist(errorValue) {
		t.Fatalf("expected server-side source ledger removal, got %v", errorValue)
	}
	if service.findSiteBySlug("ephemeral-site") != nil {
		t.Fatal("expected the slug to be freed after unserve")
	}
}
