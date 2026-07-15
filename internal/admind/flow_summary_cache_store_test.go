package admind

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestFlowSummaryCacheSnapshotReturnsCurrentDependencies(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	keys := flowSummaryCacheTestKeys()
	snapshot := readFlowSummaryDependencySnapshotForTest(t, service, keys, "members-v1")
	if snapshot != (flowSummaryDependencySnapshot{MemberFingerprint: "members-v1"}) {
		t.Fatalf("initial snapshot = %+v", snapshot)
	}
	incrementFlowSummarySourceRevisionsForTest(t, service, keys.RequestedWeek, keys.CurrentMonth)
	_, snapshot, found, errorValue := service.readFlowSummaryCacheSnapshot(ctx, "26W28", keys, "members-v1")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if found || snapshot.RequestedWeekRevision != 1 || snapshot.CurrentMonthRevision != 1 {
		t.Fatalf("found = %v snapshot = %+v", found, snapshot)
	}
}

func TestFlowSummaryCacheSnapshotReturnsMatchingEntry(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	keys := flowSummaryCacheTestKeys()
	snapshot := readFlowSummaryDependencySnapshotForTest(t, service, keys, "members-v1")
	stored, errorValue := service.writeFlowSummaryCacheEntryIfCurrent(ctx, "26W28", keys, snapshot, validFlowSummaryCachePayloadForTest(), time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !stored {
		t.Fatal("cache entry was not stored")
	}
	entry, reloadedSnapshot, found, errorValue := service.readFlowSummaryCacheSnapshot(ctx, "26W28", keys, "members-v1")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found || entry.Payload != string(validFlowSummaryCachePayloadForTest()) || reloadedSnapshot != snapshot {
		t.Fatalf("found = %v entry = %+v snapshot = %+v", found, entry, reloadedSnapshot)
	}
}

func TestFlowSummaryCacheRejectsStaleConditionalWrite(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	keys := flowSummaryCacheTestKeys()
	snapshot := readFlowSummaryDependencySnapshotForTest(t, service, keys, "members-v1")
	incrementFlowSummarySourceRevisionsForTest(t, service, keys.CurrentMonth)
	stored, errorValue := service.writeFlowSummaryCacheEntryIfCurrent(ctx, "26W28", keys, snapshot, validFlowSummaryCachePayloadForTest(), time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if stored {
		t.Fatal("stale cache entry was stored")
	}
}

func TestFlowSummaryCacheTreatsDependencyMismatchAsMiss(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	keys := flowSummaryCacheTestKeys()
	snapshot := readFlowSummaryDependencySnapshotForTest(t, service, keys, "members-v1")
	if stored, errorValue := service.writeFlowSummaryCacheEntryIfCurrent(ctx, "26W28", keys, snapshot, validFlowSummaryCachePayloadForTest(), time.Now()); errorValue != nil || !stored {
		t.Fatalf("stored = %v error = %v", stored, errorValue)
	}
	if _, _, found, errorValue := service.readFlowSummaryCacheSnapshot(ctx, "26W28", keys, "members-v2"); errorValue != nil || found {
		t.Fatalf("member mismatch found = %v error = %v", found, errorValue)
	}
	incrementFlowSummarySourceRevisionsForTest(t, service, keys.PreviousWeek)
	if _, _, found, errorValue := service.readFlowSummaryCacheSnapshot(ctx, "26W28", keys, "members-v1"); errorValue != nil || found {
		t.Fatalf("revision mismatch found = %v error = %v", found, errorValue)
	}
}

func TestFlowSummaryCacheConditionalDeletePreservesReplacement(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	keys := flowSummaryCacheTestKeys()
	snapshot := readFlowSummaryDependencySnapshotForTest(t, service, keys, "members-v1")
	if stored, errorValue := service.writeFlowSummaryCacheEntryIfCurrent(ctx, "26W28", keys, snapshot, validFlowSummaryCachePayloadForTest(), time.Now()); errorValue != nil || !stored {
		t.Fatalf("initial stored = %v error = %v", stored, errorValue)
	}
	initialEntry, _, found, errorValue := service.readFlowSummaryCacheSnapshot(ctx, "26W28", keys, "members-v1")
	if errorValue != nil || !found {
		t.Fatalf("initial found = %v error = %v", found, errorValue)
	}
	replacement := []byte(`{"version":1,"weeklyTasks":[{"id":"replacement"}],"metrics":{},"report":{}}`)
	if stored, errorValue := service.writeFlowSummaryCacheEntryIfCurrent(ctx, "26W28", keys, snapshot, replacement, time.Now()); errorValue != nil || !stored {
		t.Fatalf("replacement stored = %v error = %v", stored, errorValue)
	}
	if errorValue := service.deleteFlowSummaryCacheEntryIfUnchanged(ctx, initialEntry); errorValue != nil {
		t.Fatal(errorValue)
	}
	reloadedEntry, _, found, errorValue := service.readFlowSummaryCacheSnapshot(ctx, "26W28", keys, "members-v1")
	if errorValue != nil || !found || reloadedEntry.Payload != string(replacement) {
		t.Fatalf("replacement found = %v entry = %+v error = %v", found, reloadedEntry, errorValue)
	}
}

func TestFlowSummaryCacheCleansOnlyExpiredEntries(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	keys := flowSummaryCacheTestKeys()
	snapshot := readFlowSummaryDependencySnapshotForTest(t, service, keys, "members-v1")
	oldTime := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	if stored, errorValue := service.writeFlowSummaryCacheEntryIfCurrent(ctx, "26W28", keys, snapshot, validFlowSummaryCachePayloadForTest(), oldTime); errorValue != nil || !stored {
		t.Fatalf("old stored = %v error = %v", stored, errorValue)
	}
	otherKeys := flowSummaryDependencyKeysForWeek("26W29", time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC))
	otherSnapshot := readFlowSummaryDependencySnapshotForTest(t, service, otherKeys, "members-v1")
	if stored, errorValue := service.writeFlowSummaryCacheEntryIfCurrent(ctx, "26W29", otherKeys, otherSnapshot, validFlowSummaryCachePayloadForTest(), time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)); errorValue != nil || !stored {
		t.Fatalf("recent stored = %v error = %v", stored, errorValue)
	}
	if errorValue := service.cleanupFlowSummaryCacheEntries(ctx, time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)); errorValue != nil {
		t.Fatal(errorValue)
	}
	if count := flowSummaryCacheEntryCountForTest(t, service); count != 1 {
		t.Fatalf("entry count = %d, want 1", count)
	}
	if _, _, found, errorValue := service.readFlowSummaryCacheSnapshot(ctx, "26W29", otherKeys, "members-v1"); errorValue != nil || !found {
		t.Fatalf("recent found = %v error = %v", found, errorValue)
	}
}

func newFlowSummaryCacheTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(Configuration{FlowDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
}

func flowSummaryCacheTestKeys() flowSummaryDependencyKeys {
	return flowSummaryDependencyKeysForWeek("26W28", time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC))
}

func readFlowSummaryDependencySnapshotForTest(t *testing.T, service *Service, keys flowSummaryDependencyKeys, memberFingerprint string) flowSummaryDependencySnapshot {
	t.Helper()
	_, snapshot, found, errorValue := service.readFlowSummaryCacheSnapshot(context.Background(), "missing", keys, memberFingerprint)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if found {
		t.Fatal("unexpected cache entry")
	}
	return snapshot
}

func incrementFlowSummarySourceRevisionsForTest(t *testing.T, service *Service, keys ...flowSummarySourceKey) {
	t.Helper()
	database, errorValue := service.openFlowDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(context.Background(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := incrementFlowSummarySourceRevisions(context.Background(), transaction, keys); errorValue != nil {
		transaction.Rollback()
		t.Fatal(errorValue)
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func flowSummaryCacheEntryCountForTest(t *testing.T, service *Service) int {
	t.Helper()
	database, errorValue := service.openFlowDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var count int
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM flow_summary_cache_entries").Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	return count
}

func validFlowSummaryCachePayloadForTest() []byte {
	return []byte(`{"version":1,"weeklyTasks":[],"metrics":{},"report":{}}`)
}
