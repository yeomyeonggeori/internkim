package admind

import (
	"context"
	"testing"
)

func TestOrgchartGroupCacheInitializesWithNoUserOrganizations(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	key := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheGroups, Key: orgchartPeopleCacheSingletonKey}

	groups, errorValue := service.readCachedOrgchartGroups(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(groups) != 0 {
		t.Fatalf("groups = %#v; want empty groups", groups)
	}
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !isReusableOrgchartPeopleCacheSnapshot(snapshots[key]) {
		t.Fatalf("cache snapshot = %#v; want reusable initialized entry", snapshots[key])
	}

	groups, errorValue = service.readCachedOrgchartGroups(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(groups) != 0 {
		t.Fatalf("groups = %#v; want fallback groups excluded", groups)
	}
}

func TestOrgchartGroupCacheReusesInitializedEmptyGroups(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	key := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheGroups, Key: orgchartPeopleCacheSingletonKey}
	if errorValue := service.writeOrgchartGroups(ctx, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	groups, errorValue := service.readCachedOrgchartGroups(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(groups) != 0 {
		t.Fatalf("groups = %#v; want empty groups", groups)
	}
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !isReusableOrgchartPeopleCacheSnapshot(snapshots[key]) {
		t.Fatalf("cache snapshot = %#v; want reusable initialized entry", snapshots[key])
	}
}

func TestOrgchartGroupCacheInvalidatesWhenOnlyParentChanges(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	groups := []orgGroupRecord{
		{ID: "product", Name: "Product"},
		{ID: "engineering", Name: "Engineering"},
		{ID: "design", Name: "Design"},
	}
	if errorValue := service.writeOrgchartGroups(ctx, groups); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.readCachedOrgchartGroups(ctx); errorValue != nil {
		t.Fatal(errorValue)
	}

	groups[1].ParentID = "product"
	if errorValue := service.writeOrgchartGroups(ctx, groups); errorValue != nil {
		t.Fatal(errorValue)
	}
	updatedGroups, errorValue := service.readCachedOrgchartGroups(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if updatedGroups[1].ParentID != "product" {
		t.Fatalf("engineering parent ID = %q; want product", updatedGroups[1].ParentID)
	}
}
