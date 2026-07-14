package admind

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
)

func TestEnsureAttendanceSchemaCreatesSummaryCacheTables(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()

	for _, tableName := range []string{
		"attendance_summary_cache_revisions",
		"attendance_summary_cache_entries",
	} {
		var actualName string
		errorValue := database.QueryRow(
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
			tableName,
		).Scan(&actualName)
		if errorValue != nil {
			t.Fatalf("read cache table %s: %v", tableName, errorValue)
		}
		if actualName != tableName {
			t.Fatalf("cache table = %q, want %q", actualName, tableName)
		}
	}
}

func TestEnsureAttendanceSummaryCacheSchemaRebuildsIntermediateCacheWithoutChangingAttendanceSource(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.Exec(`
	DROP TABLE attendance_summary_cache_entries;
	DROP TABLE attendance_summary_cache_revisions;
	CREATE TABLE attendance_summary_cache_revisions (
		cache_kind TEXT NOT NULL,
		month TEXT NOT NULL,
		target_email TEXT NOT NULL,
		revision INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (cache_kind, month, target_email)
	);
	CREATE TABLE attendance_summary_cache_entries (
		cache_kind TEXT NOT NULL,
		month TEXT NOT NULL,
		target_email TEXT NOT NULL,
		generation TEXT NOT NULL DEFAULT '',
		source_revision INTEGER NOT NULL,
		schema_version INTEGER NOT NULL,
		payload_json TEXT NOT NULL,
		cached_at TEXT NOT NULL,
		PRIMARY KEY (cache_kind, month, target_email)
	);
	INSERT INTO attendance_summary_cache_revisions (cache_kind, month, target_email, revision)
	VALUES ('events', '2026-07', '', 1), ('events', '2026-07', 'staff@example.com', 2);
	INSERT INTO attendance_summary_cache_entries (
		cache_kind, month, target_email, generation, source_revision, schema_version, payload_json, cached_at
	) VALUES
		('events', '2026-07', '', 'old-generation', 1, 1, '{"events":[]}', '2026-07-13T09:00:00Z'),
		('events', '2026-07', 'staff@example.com', 'old-generation', 2, 1, '{"events":[]}', '2026-07-13T09:00:00Z');
	INSERT INTO attendance_events (
		id, mattermost_user_id, mattermost_username, email, display_name, kind,
		occurred_at, local_date, local_time, time_zone_at_event, source, team_id,
		channel_id, action_post_id, result_post_id, location_id, location_name,
		canceled_at, cancel_reason, repeated_click_at
	) VALUES (
		'event-source-1', 'user-1', 'staff', 'staff@example.com', 'Staff', 'clock_in',
		'2026-07-13T09:00:00Z', '2026-07-13', '18:00', 'Asia/Seoul', 'mattermost_button', 'team-1',
		'channel-1', 'action-1', 'result-1', 'office', 'Office', '', '', ''
	);
	INSERT INTO attendance_absence_ranges (
		id, email, kind, start_date, end_date, reason, created_by, created_at,
		updated_at, canceled_at, replaced_by
	) VALUES (
		'range-source-1', 'staff@example.com', 'annual', '2026-07-14', '2026-07-14',
		'Vacation', 'staff@example.com', '2026-07-13T09:00:00Z', '2026-07-13T09:00:00Z', '', ''
	);
	INSERT INTO attendance_absence_occurrences (
		id, range_id, email, date, created_at, canceled_at
	) VALUES (
		'occurrence-source-1', 'range-source-1', 'staff@example.com', '2026-07-14', '2026-07-13T09:00:00Z', ''
	)`); errorValue != nil {
		t.Fatal(errorValue)
	}
	wantSourceRows := readAttendanceSourceRowsForCacheSchemaTest(t, database)

	if errorValue := ensureAttendanceSummaryCacheSchema(context.Background(), database); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertAttendanceSummaryCacheTableDefinitionForTest(
		t,
		database,
		"attendance_summary_cache_revisions",
		[]string{"cache_kind", "month", "revision"},
		[]string{"cache_kind", "month"},
	)
	assertAttendanceSummaryCacheTableDefinitionForTest(
		t,
		database,
		"attendance_summary_cache_entries",
		[]string{"cache_kind", "month", "generation", "source_revision", "schema_version", "payload_json", "cached_at"},
		[]string{"cache_kind", "month"},
	)
	var cachedAtIndexCount int
	if errorValue := database.QueryRow(`
	SELECT COUNT(*) FROM sqlite_master
	WHERE type = 'index' AND name = 'attendance_summary_cache_entries_cached_at_idx'`).Scan(&cachedAtIndexCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if cachedAtIndexCount != 0 {
		t.Fatalf("cached_at index count = %d, want 0", cachedAtIndexCount)
	}
	assertAttendanceSummaryCacheRowsDiscardedForTest(t, database)
	if gotSourceRows := readAttendanceSourceRowsForCacheSchemaTest(t, database); !reflect.DeepEqual(gotSourceRows, wantSourceRows) {
		t.Fatalf("attendance source rows = %#v, want %#v", gotSourceRows, wantSourceRows)
	}
	if _, errorValue := database.Exec(`
	INSERT INTO attendance_summary_cache_revisions (cache_kind, month, revision)
	VALUES ('events', '2026-08', 7);
	INSERT INTO attendance_summary_cache_entries (
		cache_kind, month, generation, source_revision, schema_version, payload_json, cached_at
	) VALUES (
		'events', '2026-08', 'final-generation', 7, 1, '{"events":[{"id":"cached-event"}]}', '2026-07-14T00:00:00Z'
	)`); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := ensureAttendanceSummaryCacheSchema(context.Background(), database); errorValue != nil {
		t.Fatal(errorValue)
	}
	var revision int64
	var payload string
	if errorValue := database.QueryRow(`
	SELECT revisions.revision, entries.payload_json
	FROM attendance_summary_cache_revisions AS revisions
	JOIN attendance_summary_cache_entries AS entries USING (cache_kind, month)
	WHERE revisions.cache_kind = 'events' AND revisions.month = '2026-08'`).Scan(&revision, &payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if revision != 7 || payload != `{"events":[{"id":"cached-event"}]}` {
		t.Fatalf("cache row after second ensure = revision %d payload %s", revision, payload)
	}
	if gotSourceRows := readAttendanceSourceRowsForCacheSchemaTest(t, database); !reflect.DeepEqual(gotSourceRows, wantSourceRows) {
		t.Fatalf("attendance source rows after second ensure = %#v, want %#v", gotSourceRows, wantSourceRows)
	}
}

func TestEnsureAttendanceSummaryCacheSchemaRebuildsMismatchedFinalShape(t *testing.T) {
	finalRevisionDefinition := `
		cache_kind TEXT NOT NULL,
		month TEXT NOT NULL,
		revision INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (cache_kind, month)`
	finalEntryDefinition := `
		cache_kind TEXT NOT NULL,
		month TEXT NOT NULL,
		generation TEXT NOT NULL,
		source_revision INTEGER NOT NULL,
		schema_version INTEGER NOT NULL,
		payload_json TEXT NOT NULL,
		cached_at TEXT NOT NULL,
		PRIMARY KEY (cache_kind, month)`
	testCases := []struct {
		name               string
		revisionDefinition string
		entryDefinition    string
	}{
		{
			name:               "missing column",
			revisionDefinition: finalRevisionDefinition,
			entryDefinition: `
				cache_kind TEXT NOT NULL,
				month TEXT NOT NULL,
				generation TEXT NOT NULL,
				source_revision INTEGER NOT NULL,
				schema_version INTEGER NOT NULL,
				payload_json TEXT NOT NULL,
				PRIMARY KEY (cache_kind, month)`,
		},
		{
			name:               "extra column",
			revisionDefinition: finalRevisionDefinition,
			entryDefinition: `
				cache_kind TEXT NOT NULL,
				month TEXT NOT NULL,
				generation TEXT NOT NULL,
				source_revision INTEGER NOT NULL,
				schema_version INTEGER NOT NULL,
				payload_json TEXT NOT NULL,
				cached_at TEXT NOT NULL,
				legacy_scope TEXT NOT NULL,
				PRIMARY KEY (cache_kind, month)`,
		},
		{
			name: "wrong primary key",
			revisionDefinition: `
				cache_kind TEXT NOT NULL,
				month TEXT NOT NULL,
				revision INTEGER NOT NULL DEFAULT 0,
				PRIMARY KEY (month, cache_kind)`,
			entryDefinition: finalEntryDefinition,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service, _ := newAttendanceActionTestService(t)
			database, errorValue := service.openAttendanceDatabase(context.Background())
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			defer database.Close()
			replaceAttendanceSummaryCacheSchemaForTest(t, database, testCase.revisionDefinition, testCase.entryDefinition)
			if _, errorValue := database.Exec(`
			INSERT INTO attendance_summary_cache_revisions (cache_kind, month, revision)
			VALUES ('events', '2026-07', 4)`); errorValue != nil {
				t.Fatal(errorValue)
			}
			if errorValue := ensureAttendanceSummaryCacheSchema(context.Background(), database); errorValue != nil {
				t.Fatal(errorValue)
			}
			assertAttendanceSummaryCacheTableDefinitionForTest(
				t,
				database,
				"attendance_summary_cache_revisions",
				[]string{"cache_kind", "month", "revision"},
				[]string{"cache_kind", "month"},
			)
			assertAttendanceSummaryCacheTableDefinitionForTest(
				t,
				database,
				"attendance_summary_cache_entries",
				[]string{"cache_kind", "month", "generation", "source_revision", "schema_version", "payload_json", "cached_at"},
				[]string{"cache_kind", "month"},
			)
			assertAttendanceSummaryCacheRowsDiscardedForTest(t, database)
		})
	}
}

func replaceAttendanceSummaryCacheSchemaForTest(
	t *testing.T,
	database *sql.DB,
	revisionDefinition string,
	entryDefinition string,
) {
	t.Helper()
	if _, errorValue := database.Exec(`
	DROP TABLE attendance_summary_cache_entries;
	DROP TABLE attendance_summary_cache_revisions`); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.Exec(
		"CREATE TABLE attendance_summary_cache_revisions (" + revisionDefinition + ");" +
			"CREATE TABLE attendance_summary_cache_entries (" + entryDefinition + ")",
	); errorValue != nil {
		t.Fatal(errorValue)
	}
}

type attendanceSourceRowsForCacheSchemaTest struct {
	eventEmail     string
	absenceReason  string
	occurrenceDate string
}

func readAttendanceSourceRowsForCacheSchemaTest(t *testing.T, database *sql.DB) attendanceSourceRowsForCacheSchemaTest {
	t.Helper()
	var rows attendanceSourceRowsForCacheSchemaTest
	if errorValue := database.QueryRow(`
	SELECT
		(SELECT email FROM attendance_events WHERE id = 'event-source-1'),
		(SELECT reason FROM attendance_absence_ranges WHERE id = 'range-source-1'),
		(SELECT date FROM attendance_absence_occurrences WHERE id = 'occurrence-source-1')`).Scan(
		&rows.eventEmail,
		&rows.absenceReason,
		&rows.occurrenceDate,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	return rows
}

func assertAttendanceSummaryCacheTableDefinitionForTest(
	t *testing.T,
	database *sql.DB,
	tableName string,
	wantColumns []string,
	wantPrimaryKey []string,
) {
	t.Helper()
	rows, errorValue := database.Query("SELECT name, pk FROM pragma_table_info(?) ORDER BY cid", tableName)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer rows.Close()
	columns := []string{}
	primaryKeyColumns := map[int]string{}
	for rows.Next() {
		var columnName string
		var primaryKeyPosition int
		if errorValue := rows.Scan(&columnName, &primaryKeyPosition); errorValue != nil {
			t.Fatal(errorValue)
		}
		columns = append(columns, columnName)
		if primaryKeyPosition > 0 {
			primaryKeyColumns[primaryKeyPosition] = columnName
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !reflect.DeepEqual(columns, wantColumns) {
		t.Fatalf("%s columns = %#v, want %#v", tableName, columns, wantColumns)
	}
	primaryKey := make([]string, 0, len(primaryKeyColumns))
	for position := 1; position <= len(primaryKeyColumns); position++ {
		primaryKey = append(primaryKey, primaryKeyColumns[position])
	}
	if !reflect.DeepEqual(primaryKey, wantPrimaryKey) {
		t.Fatalf("%s primary key = %#v, want %#v", tableName, primaryKey, wantPrimaryKey)
	}
}

func assertAttendanceSummaryCacheRowsDiscardedForTest(t *testing.T, database *sql.DB) {
	t.Helper()
	var cacheRowCount int
	if errorValue := database.QueryRow(`
	SELECT
		(SELECT COUNT(*) FROM attendance_summary_cache_entries) +
		(SELECT COUNT(*) FROM attendance_summary_cache_revisions)`).Scan(&cacheRowCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if cacheRowCount != 0 {
		t.Fatalf("cache row count = %d, want 0", cacheRowCount)
	}
}
