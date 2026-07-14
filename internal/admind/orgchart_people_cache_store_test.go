package admind

import (
	"context"
	"testing"
)

func TestOrgchartPeopleCacheSchemaCreatesEntriesAndStates(t *testing.T) {
	service := newLocalUsersTestService(t)
	database, errorValue := service.openOrgchartDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()

	for _, tableName := range []string{"orgchart_people_cache_entries", "orgchart_people_cache_states"} {
		var count int
		if errorValue := database.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", tableName).Scan(&count); errorValue != nil {
			t.Fatal(errorValue)
		}
		if count != 1 {
			t.Fatalf("expected %s table", tableName)
		}
	}
}

func TestOrgchartPeopleCacheStoreRejectsStaleConditionalWrite(t *testing.T) {
	ctx := context.Background()
	service := newLocalUsersTestService(t)
	key := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "user-1"}
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.beginOrgchartPeopleCacheMutation(ctx, []orgchartPeopleCacheKey{key}); errorValue != nil {
		t.Fatal(errorValue)
	}

	written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, key, snapshots[key].Revision, "", []byte(`{"userID":"user-1"}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if written {
		t.Fatal("expected stale write rejection")
	}
}

func TestOrgchartPeopleCacheStoreWritesAfterMutationCompletes(t *testing.T) {
	ctx := context.Background()
	service := newLocalUsersTestService(t)
	key := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "user-1"}
	if errorValue := service.beginOrgchartPeopleCacheMutation(ctx, []orgchartPeopleCacheKey{key}); errorValue != nil {
		t.Fatal(errorValue)
	}
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !snapshots[key].IsDirty {
		t.Fatal("expected dirty cache state")
	}
	if errorValue := service.completeOrgchartPeopleCacheMutation(ctx, []orgchartPeopleCacheKey{key}); errorValue != nil {
		t.Fatal(errorValue)
	}

	written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, key, snapshots[key].Revision, "source-1", []byte(`{"userID":"user-1"}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !written {
		t.Fatal("expected current write")
	}
	reloaded, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !reloaded[key].Found || string(reloaded[key].PayloadJSON) != `{"userID":"user-1"}` {
		t.Fatalf("snapshot = %#v", reloaded[key])
	}
}
