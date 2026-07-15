package admind

import (
	"context"
	"reflect"
	"testing"
)

func TestOrgchartGroupCacheHitAndCorruptRebuild(t *testing.T) {
	t.Run("hit", func(t *testing.T) {
		service := newLocalUsersTestService(t)
		ctx := context.Background()
		if errorValue := service.writeOrgchartGroups(ctx, []orgGroupRecord{{ID: "engineering", Name: "Engineering"}}); errorValue != nil {
			t.Fatal(errorValue)
		}
		first, errorValue := service.readCachedOrgchartGroups(ctx, nil)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		database, errorValue := service.openOrgchartDatabase(ctx)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if _, errorValue := database.ExecContext(ctx, `UPDATE orgchart_groups SET name = 'Changed' WHERE id = 'engineering'`); errorValue != nil {
			database.Close()
			t.Fatal(errorValue)
		}
		if errorValue := database.Close(); errorValue != nil {
			t.Fatal(errorValue)
		}
		second, errorValue := service.readCachedOrgchartGroups(ctx, nil)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if !reflect.DeepEqual(first, second) || second[0].Name != "Engineering" {
			t.Fatalf("first = %#v second = %#v", first, second)
		}
	})

	t.Run("corrupt", func(t *testing.T) {
		service := newLocalUsersTestService(t)
		ctx := context.Background()
		if errorValue := service.writeOrgchartGroups(ctx, []orgGroupRecord{{ID: "engineering", Name: "Engineering"}}); errorValue != nil {
			t.Fatal(errorValue)
		}
		key := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheGroups, Key: orgchartPeopleCacheSingletonKey}
		snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		corruptPayload := []byte(`[{"id":"","name":"Corrupt"}]`)
		if written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, key, snapshots[key].Revision, "", corruptPayload); errorValue != nil || !written {
			t.Fatalf("write corrupt group cache: written = %t error = %v", written, errorValue)
		}
		groups, errorValue := service.readCachedOrgchartGroups(ctx, nil)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if len(groups) != 1 || groups[0].ID != "engineering" || groups[0].Name != "Engineering" {
			t.Fatalf("groups = %#v", groups)
		}
	})
}
