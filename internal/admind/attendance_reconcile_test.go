package admind

import (
	"testing"
	"time"
)

func TestMonthsToReconcileCoversThisMonthAndTheOneBefore(t *testing.T) {
	months := monthsToReconcile(time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC), 1)

	if len(months) != 2 || months[0] != "2026-07" || months[1] != "2026-08" {
		t.Fatalf("months = %v", months)
	}
}

func TestMonthsToReconcileCrossesTheYear(t *testing.T) {
	months := monthsToReconcile(time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), 1)

	if months[0] != "2025-12" || months[1] != "2026-01" {
		t.Fatalf("months = %v", months)
	}
}

func TestMonthWindowIsTheMonthAndNotAMinuteMore(t *testing.T) {
	from, to, errorValue := monthWindow("2026-02")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if from.Format(time.RFC3339) != "2026-02-01T00:00:00Z" {
		t.Fatalf("from = %s", from.Format(time.RFC3339))
	}
	if to.Format(time.RFC3339) != "2026-03-01T00:00:00Z" {
		t.Fatalf("to = %s", to.Format(time.RFC3339))
	}
}

func TestReconciledEventsCarryWhoAndWhenAndWhere(t *testing.T) {
	reconciled := reconciledEventsOf([]attendanceEvent{{
		MattermostUserID: "U-first",
		Kind:             "clock_in",
		OccurredAt:       "2026-08-05T08:29:00Z",
		LocationName:     "본사",
	}})

	if len(reconciled) != 1 {
		t.Fatalf("reconciled = %+v", reconciled)
	}
	if reconciled[0].ExternalID != "U-first" || reconciled[0].Location != "본사" {
		t.Fatalf("event = %+v", reconciled[0])
	}
	if reconciled[0].OccurredAt.Format(time.RFC3339) != "2026-08-05T08:29:00Z" {
		t.Fatalf("occurredAt = %s", reconciled[0].OccurredAt)
	}
}

func TestReconciledEventsLeaveOutWhatTheDeviceCancelled(t *testing.T) {
	reconciled := reconciledEventsOf([]attendanceEvent{
		{MattermostUserID: "U-first", Kind: "clock_in", OccurredAt: "2026-08-05T08:29:00Z"},
		{MattermostUserID: "U-first", Kind: "clock_in", OccurredAt: "2026-08-05T08:30:00Z", CanceledAt: "2026-08-05T08:31:00Z"},
	})

	if len(reconciled) != 1 {
		t.Fatalf("a cancelled clock-in must not be sent as truth: %+v", reconciled)
	}
	if reconciled[0].OccurredAt.Format(time.RFC3339) != "2026-08-05T08:29:00Z" {
		t.Fatalf("the wrong one survived: %+v", reconciled[0])
	}
}

func TestReconciledEventsLeaveOutWhatNobodyCanBeMatchedBy(t *testing.T) {
	reconciled := reconciledEventsOf([]attendanceEvent{
		{Kind: "clock_in", OccurredAt: "2026-08-05T08:29:00Z"},
		{MattermostUserID: "U-first", Kind: "clock_in", OccurredAt: "not a time"},
	})

	if len(reconciled) != 0 {
		t.Fatalf("reconciled = %+v", reconciled)
	}
}

func TestBackfillMonthsDefaultsToAYearAndIsBounded(t *testing.T) {
	if backfillMonths("") != 11 {
		t.Fatalf("empty = %d", backfillMonths(""))
	}
	if backfillMonths("not a number") != 11 {
		t.Fatalf("nonsense = %d", backfillMonths("not a number"))
	}
	if backfillMonths("0") != 11 || backfillMonths("-3") != 11 {
		t.Fatal("a window of no months is not a window")
	}
	if backfillMonths("3") != 3 {
		t.Fatalf("three = %d", backfillMonths("3"))
	}
	if backfillMonths("6000") != 60 {
		t.Fatalf("a device cannot ask to walk every month ever: %d", backfillMonths("6000"))
	}
}
