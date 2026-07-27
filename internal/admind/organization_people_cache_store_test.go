package admind

import (
	"context"
	"fmt"
	"testing"
)

func TestOrganizationPeopleCacheSchemaCreatesEntriesAndStates(t *testing.T) {
	service := newLocalUsersTestService(t)
	database, errorValue := service.openOrganizationDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()

	for _, tableName := range []string{"organization_people_cache_entries", "organization_people_cache_states"} {
		var count int
		if errorValue := database.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", tableName).Scan(&count); errorValue != nil {
			t.Fatal(errorValue)
		}
		if count != 1 {
			t.Fatalf("expected %s table", tableName)
		}
	}
}

func TestOrganizationPeopleCacheSchemaRemovesCachedAtIndex(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS organization_people_cache_entries_cached_at_idx ON organization_people_cache_entries(cached_at)`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.databaseSchemas = newAdminDatabaseSchemas()
	database, errorValue = service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var indexCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'organization_people_cache_entries_cached_at_idx'`).Scan(&indexCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if indexCount != 0 {
		t.Fatalf("cached_at index count = %d", indexCount)
	}
}

func TestOrganizationPeopleCacheSchemaMigratesActiveMutations(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	database, errorValue := service.openSQLiteDatabase(ctx, service.organizationDatabasePath(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `
		CREATE TABLE organization_people_cache_states (
			cache_kind TEXT NOT NULL,
			cache_key TEXT NOT NULL,
			revision INTEGER NOT NULL DEFAULT 0,
			is_dirty INTEGER NOT NULL DEFAULT 0 CHECK(is_dirty IN (0, 1)),
			PRIMARY KEY(cache_kind, cache_key)
		)`); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `INSERT INTO organization_people_cache_states(cache_kind, cache_key, revision, is_dirty) VALUES('person', 'user-1', 3, 1)`); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `
		CREATE TABLE organization_people_cache_entries (
			cache_kind TEXT NOT NULL,
			cache_key TEXT NOT NULL,
			source_revision TEXT NOT NULL,
			schema_version INTEGER NOT NULL,
			payload_json BLOB NOT NULL,
			cached_at TEXT NOT NULL,
			PRIMARY KEY(cache_kind, cache_key)
		)`); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `INSERT INTO organization_people_cache_entries(cache_kind, cache_key, source_revision, schema_version, payload_json, cached_at) VALUES('person', 'user-1', '', 1, '{}', '2026-07-14T00:00:00Z')`); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	database, errorValue = service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `PRAGMA table_info(organization_people_cache_states)`)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer rows.Close()
	hasActiveMutations := false
	for rows.Next() {
		var columnID int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if errorValue := rows.Scan(&columnID, &name, &columnType, &notNull, &defaultValue, &primaryKey); errorValue != nil {
			t.Fatal(errorValue)
		}
		if name == "active_mutations" {
			hasActiveMutations = true
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !hasActiveMutations {
		t.Fatal("expected active_mutations migration")
	}
	var isDirty int
	var activeMutations int
	if errorValue := database.QueryRowContext(ctx, `SELECT is_dirty, active_mutations FROM organization_people_cache_states WHERE cache_kind = 'person' AND cache_key = 'user-1'`).Scan(&isDirty, &activeMutations); errorValue != nil {
		t.Fatal(errorValue)
	}
	if isDirty != 0 || activeMutations != 0 {
		t.Fatalf("recovered mutation state = dirty %d active %d", isDirty, activeMutations)
	}
	var entryCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM organization_people_cache_entries WHERE cache_kind = 'person' AND cache_key = 'user-1'`).Scan(&entryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if entryCount != 0 {
		t.Fatalf("recovered cache entries = %d; want 0", entryCount)
	}
}

func TestOrganizationPeopleCacheStoreRejectsStaleConditionalWrite(t *testing.T) {
	ctx := context.Background()
	service := newLocalUsersTestService(t)
	key := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "user-1"}
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.beginOrganizationPeopleCacheMutation(ctx, []organizationPeopleCacheKey{key}); errorValue != nil {
		t.Fatal(errorValue)
	}

	written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, key, snapshots[key].Revision, "", []byte(`{"userID":"user-1"}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if written {
		t.Fatal("expected stale write rejection")
	}
}

func TestOrganizationPeopleCacheStoreWritesAfterMutationCompletes(t *testing.T) {
	ctx := context.Background()
	service := newLocalUsersTestService(t)
	key := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "user-1"}
	if errorValue := service.beginOrganizationPeopleCacheMutation(ctx, []organizationPeopleCacheKey{key}); errorValue != nil {
		t.Fatal(errorValue)
	}
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !snapshots[key].IsDirty {
		t.Fatal("expected dirty cache state")
	}
	if errorValue := service.completeOrganizationPeopleCacheMutation(ctx, []organizationPeopleCacheKey{key}); errorValue != nil {
		t.Fatal(errorValue)
	}

	written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, key, snapshots[key].Revision, "source-1", []byte(`{"userID":"user-1"}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !written {
		t.Fatal("expected current write")
	}
	reloaded, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !reloaded[key].Found || string(reloaded[key].PayloadJSON) != `{"userID":"user-1"}` {
		t.Fatalf("snapshot = %#v", reloaded[key])
	}
}

func TestOrganizationPeopleCacheStoreKeepsOverlappingMutationDirty(t *testing.T) {
	ctx := context.Background()
	service := newLocalUsersTestService(t)
	key := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "user-1"}
	for range 2 {
		if errorValue := service.beginOrganizationPeopleCacheMutation(ctx, []organizationPeopleCacheKey{key}); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if errorValue := service.completeOrganizationPeopleCacheMutation(ctx, []organizationPeopleCacheKey{key}); errorValue != nil {
		t.Fatal(errorValue)
	}

	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !snapshots[key].IsDirty {
		t.Fatal("expected overlapping mutation to remain dirty")
	}
	written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, key, snapshots[key].Revision, "", []byte(`{"userID":"user-1"}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if written {
		t.Fatal("expected write rejection while an overlapping mutation remains active")
	}
	if errorValue := service.completeOrganizationPeopleCacheMutation(ctx, []organizationPeopleCacheKey{key}); errorValue != nil {
		t.Fatal(errorValue)
	}
	completed, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if completed[key].IsDirty || completed[key].ActiveMutations != 0 {
		t.Fatalf("completed cache state = %#v", completed[key])
	}
}

func TestOrganizationPeopleCacheBatchWriteKeepsPerKeyGuards(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	currentKey := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "current"}
	staleKey := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "stale"}
	dirtyKey := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "dirty"}
	activeKey := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "active"}
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, state := range []struct {
		key             organizationPeopleCacheKey
		revision        int
		isDirty         int
		activeMutations int
	}{
		{key: staleKey, revision: 1},
		{key: dirtyKey, isDirty: 1},
		{key: activeKey, activeMutations: 1},
	} {
		if _, errorValue := database.ExecContext(ctx, `INSERT INTO organization_people_cache_states(cache_kind, cache_key, revision, is_dirty, active_mutations) VALUES(?, ?, ?, ?, ?)`, string(state.key.Kind), state.key.Key, state.revision, state.isDirty, state.activeMutations); errorValue != nil {
			database.Close()
			t.Fatal(errorValue)
		}
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	writes := []organizationPeopleCacheWrite{
		{Key: currentKey, Revision: 0, PayloadJSON: []byte(`{"record":{"userID":"current"}}`)},
		{Key: staleKey, Revision: 0, PayloadJSON: []byte(`{"record":{"userID":"stale"}}`)},
		{Key: dirtyKey, Revision: 0, PayloadJSON: []byte(`{"record":{"userID":"dirty"}}`)},
		{Key: activeKey, Revision: 0, PayloadJSON: []byte(`{"record":{"userID":"active"}}`)},
	}
	writtenByKey, errorValue := service.writeOrganizationPeopleCachePayloadsIfCurrent(ctx, writes)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range []organizationPeopleCacheKey{currentKey, staleKey, dirtyKey, activeKey} {
		expected := key == currentKey
		if writtenByKey[key] != expected {
			t.Fatalf("written[%#v] = %t; want %t", key, writtenByKey[key], expected)
		}
	}
}

func TestOrganizationPeopleCacheWritesMissesInSingleTransaction(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `
		CREATE TRIGGER fail_organization_people_cache_batch
		BEFORE INSERT ON organization_people_cache_entries
		WHEN NEW.cache_kind = 'person' AND NEW.cache_key = 'user-25'
		BEGIN
			SELECT RAISE(ABORT, 'forced person batch failure');
		END
	`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	users := pagesUsersResponse{Records: make([]adminUserMutation, 0, 50)}
	for index := range 50 {
		users.Records = append(users.Records, adminUserMutation{
			UserID: fmt.Sprintf("user-%02d", index),
			Email:  fmt.Sprintf("user-%02d@example.com", index),
		})
	}
	if _, errorValue := service.applyCachedOrganizationPeople(ctx, users); errorValue == nil {
		t.Fatal("expected forced batch write failure")
	}
	database, errorValue = service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var entryCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM organization_people_cache_entries WHERE cache_kind = 'person'`).Scan(&entryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if entryCount != 0 {
		t.Fatalf("person cache entries = %d; want atomic rollback", entryCount)
	}
}
