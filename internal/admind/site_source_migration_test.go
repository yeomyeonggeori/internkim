package admind

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSiteSourceFile(t *testing.T, path string, content string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o770); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(path, []byte(content), 0o660); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestReconcileSiteSourcesToStaffCircle(t *testing.T) {
	service, _ := newTestSiteService(t)
	workspaceRoot := service.Configuration.BlueclawWorkspacePath

	writeSiteSourceFile(t, filepath.Join(workspaceRoot, "sites", "oldest", "app", "src", "App.tsx"), "oldest source")
	writeSiteSourceFile(t, filepath.Join(workspaceRoot, "private", "people", "person-1", "sites", "newer", "draft", "app", "src", "App.tsx"), "newer source")
	writeSiteSourceFile(t, filepath.Join(workspaceRoot, "circles", "staff", "sites", "done", "draft", "app", "src", "App.tsx"), "done source")
	ledgerSourceFilePath := filepath.Join(service.siteSourceLedgerPath("ledgeronly"), "app", "src", "App.tsx")
	writeSiteSourceFile(t, ledgerSourceFilePath, "ledger source")

	service.sites = map[string]*SiteRecord{
		"oldest":     {SiteID: "oldest", Status: SiteStatusPublished, SourceWorkspacePath: "/workspace/sites/oldest", WorkspacePath: "/workspace/sites/oldest"},
		"newer":      {SiteID: "newer", Status: SiteStatusDraft, SourceWorkspacePath: "home/sites/newer/draft", WorkspacePath: "home/sites/newer"},
		"done":       {SiteID: "done", Status: SiteStatusDraft, SourceWorkspacePath: "/workspace/circles/staff/sites/done/draft", WorkspacePath: "/workspace/circles/staff/sites/done", DraftPath: "/workspace/circles/staff/sites/done/draft", AppWorkspacePath: "/workspace/circles/staff/sites/done/draft/app"},
		"ledgeronly": {SiteID: "ledgeronly", Status: SiteStatusPublished, SourceWorkspacePath: "home/sites/ledgeronly/draft", WorkspacePath: "home/sites/ledgeronly", HostSourcePath: service.siteSourceLedgerPath("ledgeronly")},
	}

	service.reconcileSiteSourcesToStaffCircle()

	assertSiteSourceMigrated(t, service, workspaceRoot, "oldest", "oldest source")
	assertSiteSourceMigrated(t, service, workspaceRoot, "newer", "newer source")
	assertSiteSourceMigrated(t, service, workspaceRoot, "done", "done source")
	assertSiteSourceMigrated(t, service, workspaceRoot, "ledgeronly", "ledger source")
	if _, errorValue := os.Stat(ledgerSourceFilePath); errorValue != nil {
		t.Fatalf("ledger snapshot must be preserved (copied not moved): %v", errorValue)
	}

	if isExistingDirectory(filepath.Join(workspaceRoot, "sites", "oldest")) {
		t.Fatal("oldest legacy source directory should be removed after migration")
	}
	if isExistingDirectory(filepath.Join(workspaceRoot, "private", "people", "person-1", "sites", "newer")) {
		t.Fatal("newer legacy source directory should be removed after migration")
	}

	service.reconcileSiteSourcesToStaffCircle()
	assertSiteSourceMigrated(t, service, workspaceRoot, "oldest", "oldest source")
	assertSiteSourceMigrated(t, service, workspaceRoot, "done", "done source")
}

func assertSiteSourceMigrated(t *testing.T, service *Service, workspaceRoot string, siteID string, expectedContent string) {
	t.Helper()
	migratedFilePath := filepath.Join(workspaceRoot, "circles", "staff", "sites", siteID, "draft", "app", "src", "App.tsx")
	content, errorValue := os.ReadFile(migratedFilePath)
	if errorValue != nil {
		t.Fatalf("expected migrated source for %s: %v", siteID, errorValue)
	}
	if string(content) != expectedContent {
		t.Fatalf("unexpected migrated content for %s: %q", siteID, string(content))
	}
	site := service.sites[siteID]
	expectedDraft := "/workspace/circles/staff/sites/" + siteID + "/draft"
	if site.SourceWorkspacePath != expectedDraft || site.DraftPath != expectedDraft {
		t.Fatalf("expected canonical draft path for %s, got source=%q draft=%q", siteID, site.SourceWorkspacePath, site.DraftPath)
	}
	if site.WorkspacePath != "/workspace/circles/staff/sites/"+siteID {
		t.Fatalf("expected canonical workspace path for %s, got %q", siteID, site.WorkspacePath)
	}
	if site.AppWorkspacePath != expectedDraft+"/app" {
		t.Fatalf("expected canonical app path for %s, got %q", siteID, site.AppWorkspacePath)
	}
}
