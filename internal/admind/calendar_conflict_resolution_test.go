package admind

import (
	"testing"
	"time"
)

func TestResolveCalendarRemoteDeletion(t *testing.T) {
	lastSeenAt := time.Date(2026, 7, 15, 1, 0, 0, 0, time.UTC)
	missingDetectedAt := lastSeenAt.Add(time.Hour)
	testCases := []struct {
		name           string
		localChangedAt time.Time
		expected       calendarConflictWinner
	}{
		{name: "before last seen", localChangedAt: lastSeenAt.Add(-time.Minute), expected: calendarConflictWinnerRemote},
		{name: "at last seen", localChangedAt: lastSeenAt, expected: calendarConflictWinnerRemote},
		{name: "ambiguous interval prefers deletion", localChangedAt: lastSeenAt.Add(30 * time.Minute), expected: calendarConflictWinnerRemote},
		{name: "at missing detected", localChangedAt: missingDetectedAt, expected: calendarConflictWinnerRemote},
		{name: "strictly after missing detected", localChangedAt: missingDetectedAt.Add(time.Nanosecond), expected: calendarConflictWinnerLocal},
		{name: "missing evidence prefers deletion", localChangedAt: time.Time{}, expected: calendarConflictWinnerRemote},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual := resolveCalendarRemoteDeletion(testCase.localChangedAt, lastSeenAt, missingDetectedAt)
			if actual != testCase.expected {
				t.Fatalf("winner=%s expected=%s", actual, testCase.expected)
			}
		})
	}
}

func TestResolveCalendarLocalDeletion(t *testing.T) {
	localDeletedAt := time.Date(2026, 7, 15, 2, 0, 0, 0, time.UTC)
	if actual := resolveCalendarLocalDeletion(localDeletedAt, localDeletedAt.Add(time.Minute)); actual != calendarConflictWinnerRemote {
		t.Fatalf("newer remote edit winner=%s", actual)
	}
	if actual := resolveCalendarLocalDeletion(localDeletedAt, localDeletedAt.Add(-time.Minute)); actual != calendarConflictWinnerLocal {
		t.Fatalf("newer local delete winner=%s", actual)
	}
	if actual := resolveCalendarLocalDeletion(localDeletedAt, time.Time{}); actual != calendarConflictWinnerLocal {
		t.Fatalf("missing remote time winner=%s", actual)
	}
}

func TestResolveCalendarSameFieldEdit(t *testing.T) {
	localChangedAt := time.Date(2026, 7, 15, 2, 0, 0, 0, time.UTC)
	if actual := resolveCalendarSameFieldEdit(localChangedAt, localChangedAt.Add(time.Minute)); actual != calendarConflictWinnerRemote {
		t.Fatalf("newer remote edit winner=%s", actual)
	}
	if actual := resolveCalendarSameFieldEdit(localChangedAt, localChangedAt.Add(-time.Minute)); actual != calendarConflictWinnerLocal {
		t.Fatalf("newer local edit winner=%s", actual)
	}
	if actual := resolveCalendarSameFieldEdit(localChangedAt, time.Time{}); actual != calendarConflictWinnerLocal {
		t.Fatalf("missing remote time winner=%s", actual)
	}
}
