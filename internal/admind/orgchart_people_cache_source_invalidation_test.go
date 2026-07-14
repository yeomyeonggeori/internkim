package admind

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOrgchartPeopleCacheInvalidatesChangedUserFromSourceRevision(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	statePath := filepath.Join(service.Configuration.StateDirectory, "users-sync.json")
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"1","users":["one@example.com"]}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	name := "Old Name"
	loader := func(context.Context) (pagesUsersResponse, error) {
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Name: name, Role: "member"}}}, nil
	}
	users, cachePolicy, errorValue := service.readCachedOrgchartUserList(ctx, loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.applyOrgchartPeople(ctx, users, cachePolicy); errorValue != nil {
		t.Fatal(errorValue)
	}
	name = "New Name"
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2","users":["one@example.com"]}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, _, errorValue := service.readCachedOrgchartUserList(ctx, loader); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertOrgchartPersonCacheFound(t, service, "user-1", false)
}

func TestOrgchartPeopleCacheInvalidatesAddedAndRemovedUsersFromSourceRevision(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	statePath := filepath.Join(service.Configuration.StateDirectory, "users-sync.json")
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"1"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	currentRecords := []adminUserMutation{
		{UserID: "user-removed", Email: "removed@example.com", Role: "member"},
		{UserID: "user-retained", Email: "retained@example.com", Role: "member"},
	}
	loader := func(context.Context) (pagesUsersResponse, error) {
		return pagesUsersResponse{Records: currentRecords}, nil
	}
	users, cachePolicy, errorValue := service.readCachedOrgchartUserList(ctx, loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.applyOrgchartPeople(ctx, users, cachePolicy); errorValue != nil {
		t.Fatal(errorValue)
	}
	currentRecords = []adminUserMutation{
		{UserID: "user-retained", Email: "retained@example.com", Role: "member"},
		{UserID: "user-added", Email: "added@example.com", Role: "member"},
	}
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, _, errorValue := service.readCachedOrgchartUserList(ctx, loader); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertOrgchartPersonCacheFound(t, service, "user-removed", false)
	assertOrgchartPersonCacheFound(t, service, "user-retained", true)
	assertOrgchartPersonCacheFound(t, service, "user-added", false)
}

func TestOrgchartPeopleCacheInvalidatesOnlyUpdatedProfile(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	users := pagesUsersResponse{Records: []adminUserMutation{
		{UserID: "user-1", Email: "one@example.com", Role: "member"},
		{UserID: "user-2", Email: "two@example.com", Role: "member"},
	}}
	if _, errorValue := service.applyCachedOrgchartPeople(ctx, users); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrgchartProfiles(ctx, []orgchartProfile{{
		UserID:            "user-1",
		Email:             "one@example.com",
		JobTitle:          "Engineer",
		EmploymentStatus:  orgchartEmploymentStatusActive,
		IsOrgchartVisible: true,
	}}); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertOrgchartPersonCacheFound(t, service, "user-1", false)
	assertOrgchartPersonCacheFound(t, service, "user-2", true)
}

func TestOrgchartPeopleCacheInvalidatesChangedGroupAndAffectedProfile(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrgchartGroups(ctx, []orgGroupRecord{{ID: "design", Name: "Design"}, {ID: "engineering", Name: "Engineering"}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrgchartProfiles(ctx, []orgchartProfile{
		{UserID: "user-1", Email: "one@example.com", PrimaryGroupID: "design", EmploymentStatus: orgchartEmploymentStatusActive, IsOrgchartVisible: true},
		{UserID: "user-2", Email: "two@example.com", PrimaryGroupID: "engineering", EmploymentStatus: orgchartEmploymentStatusActive, IsOrgchartVisible: true},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	users := pagesUsersResponse{Records: []adminUserMutation{
		{UserID: "user-1", Email: "one@example.com", Role: "member"},
		{UserID: "user-2", Email: "two@example.com", Role: "member"},
	}}
	if _, errorValue := service.applyCachedOrgchartPeople(ctx, users); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrgchartGroups(ctx, []orgGroupRecord{{ID: "design", Name: "Product Design"}, {ID: "engineering", Name: "Engineering"}}); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertOrgchartCacheFound(t, service, orgchartPeopleCacheKey{Kind: orgchartPeopleCacheGroups, Key: orgchartPeopleCacheSingletonKey}, false)
	assertOrgchartPersonCacheFound(t, service, "user-1", false)
	assertOrgchartPersonCacheFound(t, service, "user-2", true)
}

func TestOrgchartPeopleCacheInvalidatesMergedDeletedGroupsAndRewrittenProfiles(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrgchartGroups(ctx, []orgGroupRecord{
		{ID: "engineering", Name: "Engineering"},
		{ID: "engineering-duplicate", Name: "Platform"},
		{ID: "operations", Name: "Operations"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrgchartProfiles(ctx, []orgchartProfile{
		{UserID: "user-1", Email: "one@example.com", PrimaryGroupID: "engineering-duplicate", GroupIDs: []string{"engineering-duplicate"}, EmploymentStatus: orgchartEmploymentStatusActive, IsOrgchartVisible: true},
		{UserID: "user-2", Email: "two@example.com", PrimaryGroupID: "operations", GroupIDs: []string{"operations"}, EmploymentStatus: orgchartEmploymentStatusActive, IsOrgchartVisible: true},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	users := pagesUsersResponse{Records: []adminUserMutation{
		{UserID: "user-1", Email: "one@example.com", Role: "member"},
		{UserID: "user-2", Email: "two@example.com", Role: "member"},
	}}
	if _, errorValue := service.applyCachedOrgchartPeople(ctx, users); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrgchartGroups(ctx, []orgGroupRecord{
		{ID: "engineering", Name: "Engineering"},
		{ID: "engineering-duplicate", Name: "engineering"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertOrgchartCacheFound(t, service, orgchartPeopleCacheKey{Kind: orgchartPeopleCacheGroups, Key: orgchartPeopleCacheSingletonKey}, false)
	assertOrgchartPersonCacheFound(t, service, "user-1", false)
	assertOrgchartPersonCacheFound(t, service, "user-2", false)
	profilesByEmail, errorValue := service.readOrgchartProfilesByEmail(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if profilesByEmail["one@example.com"].PrimaryGroupID != "engineering" {
		t.Fatalf("rewritten profile = %#v", profilesByEmail["one@example.com"])
	}
}
