package admind

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestFlowSummaryCacheReusesValidReadModel(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	keys := flowSummaryCacheTestKeys()
	buildCount := 0
	build := func(context.Context) (flowSummaryReadModel, error) {
		buildCount++
		return flowSummaryReadModel{WeeklyTasks: []flowTask{{ID: "cached"}}}, nil
	}
	for iteration := 0; iteration < 2; iteration++ {
		readModel, errorValue := service.readCachedFlowSummaryReadModel(context.Background(), "26W28", keys, "members-v1", build)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if len(readModel.WeeklyTasks) != 1 || readModel.WeeklyTasks[0].ID != "cached" {
			t.Fatalf("read model = %+v", readModel)
		}
	}
	if buildCount != 1 {
		t.Fatalf("build count = %d, want 1", buildCount)
	}
}

func TestFlowSummaryCacheRebuildsForMemberChange(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	keys := flowSummaryCacheTestKeys()
	buildCount := 0
	build := func(context.Context) (flowSummaryReadModel, error) {
		buildCount++
		return flowSummaryReadModel{WeeklyTasks: []flowTask{{ID: "rebuilt"}}}, nil
	}
	for _, fingerprint := range []string{"members-v1", "members-v2", "members-v2"} {
		if _, errorValue := service.readCachedFlowSummaryReadModel(context.Background(), "26W28", keys, fingerprint, build); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if buildCount != 2 {
		t.Fatalf("build count = %d, want 2", buildCount)
	}
}

func TestFlowSummaryCacheRebuildsCorruptPayload(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	keys := flowSummaryCacheTestKeys()
	snapshot := readFlowSummaryDependencySnapshotForTest(t, service, keys, "members-v1")
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
	INSERT INTO flow_summary_cache_entries(
		week_code, requested_week_revision, previous_week_revision, current_month_revision,
		previous_month_revision, definitions_revision, member_fingerprint, schema_version, payload_json, cached_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"26W28",
		snapshot.RequestedWeekRevision,
		snapshot.PreviousWeekRevision,
		snapshot.CurrentMonthRevision,
		snapshot.PreviousMonthRevision,
		snapshot.DefinitionsRevision,
		snapshot.MemberFingerprint,
		flowSummaryCacheSchemaVersion,
		"{",
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	buildCount := 0
	build := func(context.Context) (flowSummaryReadModel, error) {
		buildCount++
		return flowSummaryReadModel{WeeklyTasks: []flowTask{{ID: "fresh"}}}, nil
	}
	for iteration := 0; iteration < 2; iteration++ {
		if _, errorValue := service.readCachedFlowSummaryReadModel(ctx, "26W28", keys, "members-v1", build); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if buildCount != 1 {
		t.Fatalf("build count = %d, want 1", buildCount)
	}
}

func TestFlowSummaryCacheFallsBackWhenCacheStorageFails(t *testing.T) {
	service := NewService(Configuration{FlowDatabasePath: t.TempDir()})
	buildCount := 0
	readModel, errorValue := service.readCachedFlowSummaryReadModel(context.Background(), "26W28", flowSummaryCacheTestKeys(), "members-v1", func(context.Context) (flowSummaryReadModel, error) {
		buildCount++
		return flowSummaryReadModel{WeeklyTasks: []flowTask{{ID: "source"}}}, nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if buildCount != 1 || len(readModel.WeeklyTasks) != 1 || readModel.WeeklyTasks[0].ID != "source" {
		t.Fatalf("build count = %d read model = %+v", buildCount, readModel)
	}
}

func TestFlowSummaryCacheReturnsSourceBuildFailure(t *testing.T) {
	service := NewService(Configuration{FlowDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
	expectedError := errors.New("source failed")
	_, errorValue := service.readCachedFlowSummaryReadModel(context.Background(), "26W28", flowSummaryCacheTestKeys(), "members-v1", func(context.Context) (flowSummaryReadModel, error) {
		return flowSummaryReadModel{}, expectedError
	})
	if !errors.Is(errorValue, expectedError) {
		t.Fatalf("error = %v", errorValue)
	}
}
