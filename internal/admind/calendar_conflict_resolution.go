package admind

import "time"

type calendarConflictWinner string

const (
	calendarConflictWinnerLocal  calendarConflictWinner = "local"
	calendarConflictWinnerRemote calendarConflictWinner = "remote"
	calendarConflictWinnerMerge  calendarConflictWinner = "merge"
)

func resolveCalendarRemoteDeletion(localChangedAt time.Time, lastSeenAt time.Time, missingDetectedAt time.Time) calendarConflictWinner {
	if localChangedAt.IsZero() || lastSeenAt.IsZero() || missingDetectedAt.IsZero() {
		return calendarConflictWinnerRemote
	}
	if !localChangedAt.After(lastSeenAt) {
		return calendarConflictWinnerRemote
	}
	if localChangedAt.After(missingDetectedAt) {
		return calendarConflictWinnerLocal
	}
	return calendarConflictWinnerRemote
}

func resolveCalendarLocalDeletion(localDeletedAt time.Time, remoteModifiedAt time.Time) calendarConflictWinner {
	if localDeletedAt.IsZero() || remoteModifiedAt.IsZero() {
		return calendarConflictWinnerLocal
	}
	if remoteModifiedAt.After(localDeletedAt) {
		return calendarConflictWinnerRemote
	}
	return calendarConflictWinnerLocal
}

func resolveCalendarSameFieldEdit(localChangedAt time.Time, remoteModifiedAt time.Time) calendarConflictWinner {
	if localChangedAt.IsZero() || remoteModifiedAt.IsZero() {
		return calendarConflictWinnerLocal
	}
	if remoteModifiedAt.After(localChangedAt) {
		return calendarConflictWinnerRemote
	}
	return calendarConflictWinnerLocal
}
