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

func TestReconcileSiteSourcesToMemberCircle(t *testing.T) {
	service, _ := newTestSiteService(t)
	workspaceRoot := service.Configuration.BlueclawWorkspacePath

	writeSiteSourceFile(t, filepath.Join(workspaceRoot, "sites", "oldest", "app", "src", "App.tsx"), "oldest source")
	writeSiteSourceFile(t, filepath.Join(workspaceRoot, "private", "people", "person-1", "sites", "newer", "draft", "app", "src", "App.tsx"), "newer source")
	writeSiteSourceFile(t, filepath.Join(workspaceRoot, "circles", "member", "sites", "completed", "draft", "app", "src", "App.tsx"), "done source")
	writeSiteSourceFile(t, filepath.Join(workspaceRoot, "circles", "member", "sites", "slugged-id", "draft", "app", "src", "App.tsx"), "slugged source")
	writeSiteSourceFile(t, filepath.Join(workspaceRoot, "circles", "member", "sites", "rootlegacy", "app", "src", "App.tsx"), "root legacy source")
	writeSiteSourceFile(t, filepath.Join(workspaceRoot, "circles", "member", "sites", "preexisting-alias", "draft", "app", "src", "App.tsx"), "preexisting alias source")
	ledgerSourceFilePath := filepath.Join(service.siteSourceLedgerPath("ledgeronly"), "app", "src", "App.tsx")
	writeSiteSourceFile(t, ledgerSourceFilePath, "ledger source")

	service.sites = map[string]*SiteRecord{
		"oldest":     {SiteID: "oldest", Slug: "oldest", Status: SiteStatusPublished, SourceWorkspacePath: "/workspace/sites/oldest", WorkspacePath: "/workspace/sites/oldest"},
		"newer":      {SiteID: "newer", Slug: "newer", Status: SiteStatusDraft, SourceWorkspacePath: "home/sites/newer/draft", WorkspacePath: "home/sites/newer"},
		"completed":  {SiteID: "completed", Slug: "completed", Status: SiteStatusDraft, SourceWorkspacePath: "/workspace/circles/member/sites/done/draft", WorkspacePath: "/workspace/circles/member/sites/done", DraftPath: "/workspace/circles/member/sites/done/draft", AppWorkspacePath: "/workspace/circles/member/sites/done/draft/app"},
		"slugged-id": {SiteID: "slugged-id", Slug: "pretty-gyul", Status: SiteStatusPublished, SourceWorkspacePath: "/workspace/circles/member/sites/slugged-id/draft", WorkspacePath: "/workspace/circles/member/sites/slugged-id"},
		"rootlegacy": {SiteID: "rootlegacy", Slug: "root-legacy", Status: SiteStatusDraft, SourceWorkspacePath: "/workspace/circles/member/sites/rootlegacy", WorkspacePath: "/workspace/circles/member/sites/rootlegacy"},
		"preexisting": {SiteID: "preexisting", Slug: "preexisting-alias", Status: SiteStatusDraft,
			SourceWorkspacePath: "/workspace/circles/member/sites/preexisting-alias/draft",
			WorkspacePath:       "/workspace/circles/member/sites/preexisting-alias"},
		"ledgeronly": {SiteID: "ledgeronly", Slug: "ledgeronly", Status: SiteStatusPublished, SourceWorkspacePath: "home/sites/ledgeronly/draft", WorkspacePath: "home/sites/ledgeronly", HostSourcePath: service.siteSourceLedgerPath("ledgeronly")},
		"missing":    {SiteID: "missing", Slug: "missing-source", Status: SiteStatusDraft, SourceWorkspacePath: "/workspace/circles/member/sites/missing/draft", WorkspacePath: "/workspace/circles/member/sites/missing"},
	}

	service.reconcileSiteSourcesToMemberCircle()

	assertSiteSourceMigrated(t, service, workspaceRoot, "oldest", "oldest source")
	assertSiteSourceMigrated(t, service, workspaceRoot, "newer", "newer source")
	assertSiteSourceMigrated(t, service, workspaceRoot, "completed", "done source")
	assertSiteSourceMigrated(t, service, workspaceRoot, "slugged-id", "slugged source")
	assertSiteSourceMigrated(t, service, workspaceRoot, "rootlegacy", "root legacy source")
	assertSiteSourceMigrated(t, service, workspaceRoot, "preexisting", "preexisting alias source")
	assertSiteSourceMigrated(t, service, workspaceRoot, "ledgeronly", "ledger source")
	assertMissingSiteSourceCanonicalized(t, service, workspaceRoot, "missing")
	if _, errorValue := os.Stat(ledgerSourceFilePath); errorValue != nil {
		t.Fatalf("ledger snapshot must be preserved (copied not moved): %v", errorValue)
	}

	if isExistingDirectory(filepath.Join(workspaceRoot, "sites", "oldest")) {
		t.Fatal("oldest legacy source directory should be removed after migration")
	}
	if isExistingDirectory(filepath.Join(workspaceRoot, "private", "people", "person-1", "sites", "newer")) {
		t.Fatal("newer legacy source directory should be removed after migration")
	}
	if isExistingDirectory(filepath.Join(workspaceRoot, "circles", "member", "sites", "slugged-id")) {
		t.Fatal("slugged legacy source directory should be removed after migration")
	}
	if isExistingDirectory(filepath.Join(workspaceRoot, "circles", "member", "sites", "rootlegacy")) {
		t.Fatal("root legacy source directory should be removed after migration")
	}

	service.reconcileSiteSourcesToMemberCircle()
	assertSiteSourceMigrated(t, service, workspaceRoot, "oldest", "oldest source")
	assertSiteSourceMigrated(t, service, workspaceRoot, "completed", "done source")
}

func assertMissingSiteSourceCanonicalized(t *testing.T, service *Service, workspaceRoot string, siteID string) {
	t.Helper()
	site := service.sites[siteID]
	aliasName := siteWorkspaceAliasName(siteID, site.Slug)
	assertMemberCircleSiteStorageMode(t, workspaceRoot, siteID)
	aliasPath := filepath.Join(workspaceRoot, "circles", "member", "sites", aliasName)
	aliasInformation, errorValue := os.Lstat(aliasPath)
	if errorValue != nil {
		t.Fatalf("expected missing source alias for %s: %v", siteID, errorValue)
	}
	if aliasInformation.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected missing source alias for %s to be symlink, got %v", siteID, aliasInformation.Mode())
	}
	draftPath := filepath.Join(workspaceRoot, "circles", "member", "sites", ".ids", siteID, "draft")
	if _, errorValue := os.Stat(draftPath); !os.IsNotExist(errorValue) {
		t.Fatalf("missing source migration should not create draft files for %s", siteID)
	}
	expectedDraft := "/workspace/circles/member/sites/" + aliasName + "/draft"
	if site.SourceWorkspacePath != expectedDraft || site.DraftPath != expectedDraft {
		t.Fatalf("expected canonical missing source paths for %s, got source=%q draft=%q", siteID, site.SourceWorkspacePath, site.DraftPath)
	}
	if site.WorkspacePath != "/workspace/circles/member/sites/"+aliasName {
		t.Fatalf("expected canonical missing source workspace path for %s, got %q", siteID, site.WorkspacePath)
	}
	if site.AppWorkspacePath != expectedDraft+"/app" {
		t.Fatalf("expected canonical missing source app path for %s, got %q", siteID, site.AppWorkspacePath)
	}
}

func assertSiteSourceMigrated(t *testing.T, service *Service, workspaceRoot string, siteID string, expectedContent string) {
	t.Helper()
	site := service.sites[siteID]
	aliasName := siteWorkspaceAliasName(siteID, site.Slug)
	assertMemberCircleSiteStorageMode(t, workspaceRoot, siteID)
	assertMemberCircleDirectoryMode(t, filepath.Join(workspaceRoot, "circles", "member", "sites", ".ids", siteID, "draft"))
	migratedFilePath := filepath.Join(workspaceRoot, "circles", "member", "sites", ".ids", siteID, "draft", "app", "src", "App.tsx")
	content, errorValue := os.ReadFile(migratedFilePath)
	if errorValue != nil {
		t.Fatalf("expected migrated source for %s: %v", siteID, errorValue)
	}
	if string(content) != expectedContent {
		t.Fatalf("unexpected migrated content for %s: %q", siteID, string(content))
	}
	aliasPath := filepath.Join(workspaceRoot, "circles", "member", "sites", aliasName)
	aliasInformation, errorValue := os.Lstat(aliasPath)
	if errorValue != nil {
		t.Fatalf("expected migrated alias for %s: %v", siteID, errorValue)
	}
	if aliasInformation.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected migrated alias for %s to be symlink, got %v", siteID, aliasInformation.Mode())
	}
	expectedDraft := "/workspace/circles/member/sites/" + aliasName + "/draft"
	if site.SourceWorkspacePath != expectedDraft || site.DraftPath != expectedDraft {
		t.Fatalf("expected canonical draft path for %s, got source=%q draft=%q", siteID, site.SourceWorkspacePath, site.DraftPath)
	}
	if site.WorkspacePath != "/workspace/circles/member/sites/"+aliasName {
		t.Fatalf("expected canonical workspace path for %s, got %q", siteID, site.WorkspacePath)
	}
	if site.AppWorkspacePath != expectedDraft+"/app" {
		t.Fatalf("expected canonical app path for %s, got %q", siteID, site.AppWorkspacePath)
	}
}

func assertMemberCircleSiteStorageMode(t *testing.T, workspaceRoot string, siteID string) {
	t.Helper()
	memberSitesPath := filepath.Join(workspaceRoot, "circles", "member", "sites")
	assertMemberCircleDirectoryMode(t, memberSitesPath)
	assertMemberCircleDirectoryMode(t, filepath.Join(memberSitesPath, ".ids"))
	assertMemberCircleDirectoryMode(t, filepath.Join(memberSitesPath, ".ids", siteID))
}
