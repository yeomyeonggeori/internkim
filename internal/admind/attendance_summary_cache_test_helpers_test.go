package admind

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func insertAttendanceCacheTestEvent(t *testing.T, service *Service, occurredAtValue string) (*sql.DB, attendanceEvent) {
	t.Helper()
	occurredAt, errorValue := time.Parse(time.RFC3339, occurredAtValue)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event := service.createAttendanceEvent(
		mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com", Nickname: "Staff"},
		attendanceKindClockIn,
		occurredAt,
		"team-1",
		"attendance-channel",
		"action-post",
		"result-post-"+occurredAt.Format("20060102"),
		attendanceLocation{},
	)
	if errorValue := service.insertAttendanceEvent(context.Background(), database, event); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	return database, event
}

func primeAttendanceSummaryCacheForTest(
	t *testing.T,
	service *Service,
	kind attendanceSummaryCacheKind,
	month string,
) int64 {
	t.Helper()
	_, revision, _, errorValue := service.readAttendanceSummaryCachePayload(context.Background(), kind, month, time.Now().UTC())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	stored, errorValue := service.writeAttendanceSummaryCachePayloadIfCurrent(context.Background(), kind, month, revision, []byte(`{}`), time.Now().UTC())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !stored {
		t.Fatalf("team cache for %s %s was not stored", kind, month)
	}
	return revision
}

func assertAttendanceSummaryCacheInvalidatedForTest(
	t *testing.T,
	service *Service,
	kind attendanceSummaryCacheKind,
	month string,
	previousRevision int64,
) {
	t.Helper()
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var entryCount int
	if errorValue := database.QueryRow(`
	SELECT COUNT(*) FROM attendance_summary_cache_entries
	WHERE cache_kind = ? AND month = ?`, kind, month).Scan(&entryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if entryCount != 0 {
		t.Fatalf("team cache entry count for %s %s = %d, want 0", kind, month, entryCount)
	}
	var revision int64
	if errorValue := database.QueryRow(`
	SELECT revision FROM attendance_summary_cache_revisions
	WHERE cache_kind = ? AND month = ?`, kind, month).Scan(&revision); errorValue != nil {
		t.Fatal(errorValue)
	}
	if revision != previousRevision+1 {
		t.Fatalf("team revision for %s %s = %d, want %d", kind, month, revision, previousRevision+1)
	}
}

func assertAttendanceSummaryCachePreservedForTest(
	t *testing.T,
	service *Service,
	kind attendanceSummaryCacheKind,
	month string,
	previousRevision int64,
) {
	t.Helper()
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var entryCount int
	if errorValue := database.QueryRow(`
	SELECT COUNT(*) FROM attendance_summary_cache_entries
	WHERE cache_kind = ? AND month = ?`, kind, month).Scan(&entryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if entryCount != 1 {
		t.Fatalf("team cache entry count for %s %s = %d, want 1", kind, month, entryCount)
	}
	var revision int64
	if errorValue := database.QueryRow(`
	SELECT COALESCE((
		SELECT revision FROM attendance_summary_cache_revisions
		WHERE cache_kind = ? AND month = ?
	), 0)`, kind, month).Scan(&revision); errorValue != nil {
		t.Fatal(errorValue)
	}
	if revision != previousRevision {
		t.Fatalf("team revision for %s %s = %d, want %d", kind, month, revision, previousRevision)
	}
}

func insertAttendanceSummaryCacheEntryForTest(
	t *testing.T,
	service *Service,
	kind attendanceSummaryCacheKind,
	month string,
	revision int64,
	schemaVersion int,
	payload string,
	cachedAt time.Time,
) {
	t.Helper()
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	_, errorValue = database.Exec(`
	INSERT INTO attendance_summary_cache_entries (
		cache_kind, month, generation, source_revision, schema_version, payload_json, cached_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`, kind, month, service.attendanceSummaryCacheGeneration(), revision, schemaVersion, payload, cachedAt.Format(time.RFC3339Nano))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
}

func attendanceSummaryCacheEntryCountForTest(t *testing.T, service *Service) int {
	t.Helper()
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var count int
	if errorValue := database.QueryRow("SELECT COUNT(*) FROM attendance_summary_cache_entries").Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	return count
}

func attendanceSummaryCacheRevisionForTest(t *testing.T, service *Service, kind attendanceSummaryCacheKind, month string) int64 {
	t.Helper()
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var revision int64
	if errorValue := database.QueryRow(
		"SELECT revision FROM attendance_summary_cache_revisions WHERE cache_kind = ? AND month = ?",
		kind,
		month,
	).Scan(&revision); errorValue != nil {
		t.Fatal(errorValue)
	}
	return revision
}
