package admind

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
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
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	members := []flowMember{{ID: "member-1", Name: "Member One"}}
	task := flowTask{
		ID:               "source-task",
		WeekCode:         "26W28",
		OwnerID:          members[0].ID,
		OwnerName:        members[0].Name,
		ParticipantIDs:   []string{members[0].ID},
		ParticipantNames: []string{members[0].Name},
		Business:         "Development",
		Type:             "Implementation",
		Content:          "Build from source",
		Goal:             "Keep summaries available",
		Size:             "S",
		Status:           flowStatusInProgress,
		StatusRank:       1024,
		StartDate:        "2026-07-06",
		EndDate:          "2026-07-07",
	}
	if errorValue := service.writeFlowTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, "DROP TABLE flow_summary_cache_entries"); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	weekStart := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	readModel, errorValue := service.readCachedFlowSummaryReadModel(ctx, "26W28", flowSummaryDependencyKeysForWeek("26W28", weekStart), "members-v1", func(ctx context.Context) (flowSummaryReadModel, error) {
		return service.buildFlowSummaryReadModel(ctx, "26W28", weekStart, members)
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(readModel.WeeklyTasks) != 1 || readModel.WeeklyTasks[0].ID != task.ID {
		t.Fatalf("read model = %+v", readModel)
	}
}

func TestFlowSummaryCacheLogsCleanupFailureSeparatelyFromWrite(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	keys := flowSummaryCacheTestKeys()
	if _, _, _, errorValue := service.readFlowSummaryCacheSnapshot(ctx, "missing", keys, "members-v1"); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
		INSERT INTO flow_summary_cache_entries(
			week_code, requested_week_revision, previous_week_revision, current_month_revision,
			previous_month_revision, definitions_revision, member_fingerprint, schema_version, payload_json, cached_at
		) VALUES ('26W27', 0, 0, 0, 0, 0, 'members-v1', 1, '{}', '2020-01-01T00:00:00Z');
		CREATE TRIGGER reject_flow_summary_cache_cleanup
		BEFORE DELETE ON flow_summary_cache_entries
		BEGIN
			SELECT RAISE(FAIL, 'cleanup blocked');
		END`)
	if errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	var logOutput bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logOutput, nil)))
	defer slog.SetDefault(previousLogger)
	if _, errorValue := service.readCachedFlowSummaryReadModel(ctx, "26W28", keys, "members-v1", func(context.Context) (flowSummaryReadModel, error) {
		return flowSummaryReadModel{WeeklyTasks: []flowTask{{ID: "fresh"}}}, nil
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(logOutput.String(), `"operation":"cleanup"`) {
		t.Fatalf("log output = %s", logOutput.String())
	}
	if _, _, found, errorValue := service.readFlowSummaryCacheSnapshot(ctx, "26W28", keys, "members-v1"); errorValue != nil || !found {
		t.Fatalf("stored cache found = %v error = %v", found, errorValue)
	}
}

func TestFlowSummaryCacheReturnsSourceBuildFailure(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	expectedError := errors.New("source failed")
	_, errorValue := service.readCachedFlowSummaryReadModel(context.Background(), "26W28", flowSummaryCacheTestKeys(), "members-v1", func(context.Context) (flowSummaryReadModel, error) {
		return flowSummaryReadModel{}, expectedError
	})
	if !errors.Is(errorValue, expectedError) {
		t.Fatalf("error = %v", errorValue)
	}
}
