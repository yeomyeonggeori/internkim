package admind

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestReadAttendanceEventCacheDatesByResultPostIDReturnsEveryDistinctDate(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	resultPostID := "shared-result-post"
	userRecord := mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com", Nickname: "Staff"}
	for _, occurredAt := range []time.Time{
		time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC),
	} {
		event := service.createAttendanceEvent(
			userRecord,
			attendanceKindClockIn,
			occurredAt,
			"team-1",
			"attendance-channel",
			"action-post-"+occurredAt.Format("20060102"),
			resultPostID,
			attendanceLocation{},
		)
		if errorValue := service.insertAttendanceEvent(ctx, database, event); errorValue != nil {
			t.Fatal(errorValue)
		}
		override := attendanceEventOverride{
			ID:                 "override-" + occurredAt.Format("20060102"),
			EventID:            event.ID,
			EditedBy:           event.Email,
			EditedAt:           "2026-08-01T00:00:00Z",
			Reason:             "Correction",
			OriginalOccurredAt: event.OccurredAt,
			OriginalLocalDate:  event.LocalDate,
			OriginalLocalTime:  event.LocalTime,
			OverrideOccurredAt: "2026-08-01T09:00:00Z",
			OverrideLocalDate:  "2026-08-01",
			OverrideLocalTime:  "09:00:00",
		}
		if errorValue := service.insertAttendanceEventOverride(ctx, database, override); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer transaction.Rollback()
	dates, errorValue := readAttendanceEventCacheDatesByResultPostIDInTransaction(ctx, transaction, resultPostID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	sort.Strings(dates)
	want := []string{"2026-06-30", "2026-07-01", "2026-08-01"}
	if !reflect.DeepEqual(dates, want) {
		t.Fatalf("affected dates = %v, want %v", dates, want)
	}
}
