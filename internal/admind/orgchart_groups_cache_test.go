package admind

import (
	"context"
	"testing"
)

func TestOrgchartGroupCacheDoesNotReuseUninitializedEmptyGroups(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	key := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheGroups, Key: orgchartPeopleCacheSingletonKey}

	groups, errorValue := service.readCachedOrgchartGroups(ctx, nil)
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
	if snapshots[key].Found {
		t.Fatalf("cache snapshot = %#v; want no reusable entry", snapshots[key])
	}

	fallbackGroups := []orgGroupRecord{{ID: "engineering", Name: "Engineering"}}
	groups, errorValue = service.readCachedOrgchartGroups(ctx, fallbackGroups)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(groups) != 1 || groups[0] != fallbackGroups[0] {
		t.Fatalf("groups = %#v; want imported fallback groups %#v", groups, fallbackGroups)
	}
}

func TestOrgchartGroupCacheReusesInitializedEmptyGroups(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	key := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheGroups, Key: orgchartPeopleCacheSingletonKey}
	if errorValue := service.writeOrgchartGroups(ctx, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	groups, errorValue := service.readCachedOrgchartGroups(ctx, nil)
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
