package admind

import (
	"context"
	"reflect"
	"testing"
)

func TestOrganizationGroupCacheHitAndCorruptRebuild(t *testing.T) {
	t.Run("hit", func(t *testing.T) {
		service := newLocalUsersTestService(t)
		ctx := context.Background()
		if errorValue := service.writeOrganizationGroups(ctx, []orgGroupRecord{{ID: "engineering", Name: "Engineering"}}); errorValue != nil {
			t.Fatal(errorValue)
		}
		first, errorValue := service.readCachedOrganizationGroups(ctx)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		database, errorValue := service.openOrganizationDatabase(ctx)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if _, errorValue := database.ExecContext(ctx, `UPDATE organization_groups SET name = 'Changed' WHERE id = 'engineering'`); errorValue != nil {
			database.Close()
			t.Fatal(errorValue)
		}
		if errorValue := database.Close(); errorValue != nil {
			t.Fatal(errorValue)
		}
		second, errorValue := service.readCachedOrganizationGroups(ctx)
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
		if errorValue := service.writeOrganizationGroups(ctx, []orgGroupRecord{{ID: "engineering", Name: "Engineering"}}); errorValue != nil {
			t.Fatal(errorValue)
		}
		key := organizationPeopleCacheKey{Kind: organizationPeopleCacheGroups, Key: organizationPeopleCacheSingletonKey}
		snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{key})
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		corruptPayload := []byte(`[{"id":"","name":"Corrupt"}]`)
		if written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, key, snapshots[key].Revision, "", corruptPayload); errorValue != nil || !written {
			t.Fatalf("write corrupt group cache: written = %t error = %v", written, errorValue)
		}
		groups, errorValue := service.readCachedOrganizationGroups(ctx)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if len(groups) != 1 || groups[0].ID != "engineering" || groups[0].Name != "Engineering" {
			t.Fatalf("groups = %#v", groups)
		}
	})
}
