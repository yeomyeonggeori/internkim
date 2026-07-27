package admind

import (
	"context"
	"testing"
)

func TestOrganizationGroupCacheInitializesWithNoUserOrganizations(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	key := organizationPeopleCacheKey{Kind: organizationPeopleCacheGroups, Key: organizationPeopleCacheSingletonKey}

	groups, errorValue := service.readCachedOrganizationGroups(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(groups) != 0 {
		t.Fatalf("groups = %#v; want empty groups", groups)
	}
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !isReusableOrganizationPeopleCacheSnapshot(snapshots[key]) {
		t.Fatalf("cache snapshot = %#v; want reusable initialized entry", snapshots[key])
	}

	groups, errorValue = service.readCachedOrganizationGroups(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(groups) != 0 {
		t.Fatalf("groups = %#v; want fallback groups excluded", groups)
	}
}

func TestOrganizationGroupCacheReusesInitializedEmptyGroups(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	key := organizationPeopleCacheKey{Kind: organizationPeopleCacheGroups, Key: organizationPeopleCacheSingletonKey}
	if errorValue := service.writeOrganizationGroups(ctx, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	groups, errorValue := service.readCachedOrganizationGroups(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(groups) != 0 {
		t.Fatalf("groups = %#v; want empty groups", groups)
	}
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !isReusableOrganizationPeopleCacheSnapshot(snapshots[key]) {
		t.Fatalf("cache snapshot = %#v; want reusable initialized entry", snapshots[key])
	}
}

func TestOrganizationGroupCacheInvalidatesWhenOnlyParentChanges(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	groups := []orgGroupRecord{
		{ID: "product", Name: "Product"},
		{ID: "engineering", Name: "Engineering"},
		{ID: "design", Name: "Design"},
	}
	if errorValue := service.writeOrganizationGroups(ctx, groups); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.readCachedOrganizationGroups(ctx); errorValue != nil {
		t.Fatal(errorValue)
	}

	groups[1].ParentID = "product"
	if errorValue := service.writeOrganizationGroups(ctx, groups); errorValue != nil {
		t.Fatal(errorValue)
	}
	updatedGroups, errorValue := service.readCachedOrganizationGroups(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if updatedGroups[1].ParentID != "product" {
		t.Fatalf("engineering parent ID = %q; want product", updatedGroups[1].ParentID)
	}
}
