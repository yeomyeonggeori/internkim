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

func TestOrgchartPeopleCacheSchemaRemovesCachedAtIndex(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS orgchart_people_cache_entries_cached_at_idx ON orgchart_people_cache_entries(cached_at)`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.databaseSchemas = newAdminDatabaseSchemas()
	database, errorValue = service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var indexCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'orgchart_people_cache_entries_cached_at_idx'`).Scan(&indexCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if indexCount != 0 {
		t.Fatalf("cached_at index count = %d", indexCount)
	}
}

func TestOrgchartPeopleCacheSchemaMigratesActiveMutations(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	database, errorValue := service.openSQLiteDatabase(ctx, service.orgchartDatabasePath(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `
		CREATE TABLE orgchart_people_cache_states (
			cache_kind TEXT NOT NULL,
			cache_key TEXT NOT NULL,
			revision INTEGER NOT NULL DEFAULT 0,
			is_dirty INTEGER NOT NULL DEFAULT 0 CHECK(is_dirty IN (0, 1)),
			PRIMARY KEY(cache_kind, cache_key)
		)`); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `INSERT INTO orgchart_people_cache_states(cache_kind, cache_key, revision, is_dirty) VALUES('person', 'user-1', 3, 1)`); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `
		CREATE TABLE orgchart_people_cache_entries (
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
	if _, errorValue := database.ExecContext(ctx, `INSERT INTO orgchart_people_cache_entries(cache_kind, cache_key, source_revision, schema_version, payload_json, cached_at) VALUES('person', 'user-1', '', 1, '{}', '2026-07-14T00:00:00Z')`); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	database, errorValue = service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `PRAGMA table_info(orgchart_people_cache_states)`)
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
	if errorValue := database.QueryRowContext(ctx, `SELECT is_dirty, active_mutations FROM orgchart_people_cache_states WHERE cache_kind = 'person' AND cache_key = 'user-1'`).Scan(&isDirty, &activeMutations); errorValue != nil {
		t.Fatal(errorValue)
	}
	if isDirty != 0 || activeMutations != 0 {
		t.Fatalf("recovered mutation state = dirty %d active %d", isDirty, activeMutations)
	}
	var entryCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM orgchart_people_cache_entries WHERE cache_kind = 'person' AND cache_key = 'user-1'`).Scan(&entryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if entryCount != 0 {
		t.Fatalf("recovered cache entries = %d; want 0", entryCount)
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

func TestOrgchartPeopleCacheStoreKeepsOverlappingMutationDirty(t *testing.T) {
	ctx := context.Background()
	service := newLocalUsersTestService(t)
	key := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "user-1"}
	for range 2 {
		if errorValue := service.beginOrgchartPeopleCacheMutation(ctx, []orgchartPeopleCacheKey{key}); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if errorValue := service.completeOrgchartPeopleCacheMutation(ctx, []orgchartPeopleCacheKey{key}); errorValue != nil {
		t.Fatal(errorValue)
	}

	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !snapshots[key].IsDirty {
		t.Fatal("expected overlapping mutation to remain dirty")
	}
	written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, key, snapshots[key].Revision, "", []byte(`{"userID":"user-1"}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if written {
		t.Fatal("expected write rejection while an overlapping mutation remains active")
	}
	if errorValue := service.completeOrgchartPeopleCacheMutation(ctx, []orgchartPeopleCacheKey{key}); errorValue != nil {
		t.Fatal(errorValue)
	}
	completed, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if completed[key].IsDirty || completed[key].ActiveMutations != 0 {
		t.Fatalf("completed cache state = %#v", completed[key])
	}
}

func TestOrgchartPeopleCacheBatchWriteKeepsPerKeyGuards(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	currentKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "current"}
	staleKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "stale"}
	dirtyKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "dirty"}
	activeKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "active"}
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, state := range []struct {
		key             orgchartPeopleCacheKey
		revision        int
		isDirty         int
		activeMutations int
	}{
		{key: staleKey, revision: 1},
		{key: dirtyKey, isDirty: 1},
		{key: activeKey, activeMutations: 1},
	} {
		if _, errorValue := database.ExecContext(ctx, `INSERT INTO orgchart_people_cache_states(cache_kind, cache_key, revision, is_dirty, active_mutations) VALUES(?, ?, ?, ?, ?)`, string(state.key.Kind), state.key.Key, state.revision, state.isDirty, state.activeMutations); errorValue != nil {
			database.Close()
			t.Fatal(errorValue)
		}
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	writes := []orgchartPeopleCacheWrite{
		{Key: currentKey, Revision: 0, PayloadJSON: []byte(`{"record":{"userID":"current"}}`)},
		{Key: staleKey, Revision: 0, PayloadJSON: []byte(`{"record":{"userID":"stale"}}`)},
		{Key: dirtyKey, Revision: 0, PayloadJSON: []byte(`{"record":{"userID":"dirty"}}`)},
		{Key: activeKey, Revision: 0, PayloadJSON: []byte(`{"record":{"userID":"active"}}`)},
	}
	writtenByKey, errorValue := service.writeOrgchartPeopleCachePayloadsIfCurrent(ctx, writes)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range []orgchartPeopleCacheKey{currentKey, staleKey, dirtyKey, activeKey} {
		expected := key == currentKey
		if writtenByKey[key] != expected {
			t.Fatalf("written[%#v] = %t; want %t", key, writtenByKey[key], expected)
		}
	}
}
