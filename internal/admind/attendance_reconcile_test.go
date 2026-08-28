package admind

import (
	"testing"
	"time"
)

func TestMonthsToReconcileCoversThisMonthAndTheOneBefore(t *testing.T) {
	months := monthsToPublish(time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC), 1)

	if len(months) != 2 || months[0] != "2026-07" || months[1] != "2026-08" {
		t.Fatalf("months = %v", months)
	}
}

func TestMonthsToReconcileCrossesTheYear(t *testing.T) {
	months := monthsToPublish(time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), 1)

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

func TestReconciledWorkPolicyCarriesEveryRevisionWithTheDateItTookEffect(t *testing.T) {
	earlier := defaultAttendanceWorkPolicyRevision()
	earlier.NightStartTime = "21:30"
	earlier.NightEndTime = "05:30"
	later := earlier
	later.EffectiveDate = "2026-08-01"
	later.WorkMode = attendanceWorkModeFixed
	later.FixedStartTime = "09:00"
	later.FixedEndTime = "18:00"
	later.CoreTimeEnabled = false
	later.CoreStartTime = ""
	later.CoreEndTime = ""
	policy := defaultAttendanceWorkPolicy()
	policy.Revisions = []attendanceWorkPolicyRevision{earlier, later}

	reconciled := reconciledWorkPolicyOf(policy)

	if reconciled.Version != attendanceWorkPolicyVersion || len(reconciled.Revisions) != 2 {
		t.Fatalf("work policy = %+v", reconciled)
	}
	if reconciled.Revisions[0].EffectiveDate != attendanceWorkPolicyInitialEffectiveDate ||
		reconciled.Revisions[0].NightStartTime != "21:30" ||
		reconciled.Revisions[0].NightEndTime != "05:30" {
		t.Fatalf("the revision covering the past = %+v", reconciled.Revisions[0])
	}
	if reconciled.Revisions[1].EffectiveDate != "2026-08-01" ||
		reconciled.Revisions[1].WorkMode != attendanceWorkModeFixed {
		t.Fatalf("the current revision = %+v", reconciled.Revisions[1])
	}
	if len(reconciled.Revisions[1].WorkingWeekdays) != 5 ||
		len(reconciled.Revisions[1].BreakPeriods) != 1 {
		t.Fatalf("work policy schedule = %+v", reconciled.Revisions[1])
	}
}

func TestReconciledCompanyHolidaysCarryWhatTheDeviceRecorded(t *testing.T) {
	reconciled := reconciledCompanyHolidaysOf([]calendarCompanyHoliday{{
		ID:             "company-holiday-1",
		Title:          "창립기념일",
		Date:           "2027-03-02",
		RecursAnnually: true,
		CreatedAt:      "2026-01-01T00:00:00Z",
		UpdatedAt:      "2026-01-01T00:00:00Z",
	}})

	if len(reconciled) != 1 ||
		reconciled[0].ID != "company-holiday-1" ||
		reconciled[0].Title != "창립기념일" ||
		reconciled[0].Date != "2027-03-02" ||
		!reconciled[0].RecursAnnually {
		t.Fatalf("company holidays = %+v", reconciled)
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
