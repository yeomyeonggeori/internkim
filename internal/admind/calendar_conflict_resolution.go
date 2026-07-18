package admind

import "time"

type calendarConflictWinner string

const (
	calendarConflictWinnerLocal  calendarConflictWinner = "local"
	calendarConflictWinnerRemote calendarConflictWinner = "remote"
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

func selectCalendarLocalWinningFields(localChangedFields []string, remoteChangedFields []string, fieldChangedAt map[string]time.Time, remoteModifiedAt time.Time) []string {
	result := []string{}
	for _, field := range localChangedFields {
		if !calendarFieldListIncludes(remoteChangedFields, field) {
			result = append(result, field)
			continue
		}
		if resolveCalendarSameFieldEdit(fieldChangedAt[field], remoteModifiedAt) == calendarConflictWinnerLocal {
			result = append(result, field)
		}
	}
	return result
}

func parseCalendarConflictTime(value string) time.Time {
	parsed, errorValue := time.Parse(time.RFC3339Nano, value)
	if errorValue == nil {
		return parsed.UTC()
	}
	parsed, errorValue = time.Parse(time.RFC3339, value)
	if errorValue != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func latestCalendarPendingChangeAt(change pendingCalendarLocalChange) time.Time {
	latest := time.Time{}
	for _, changedAt := range change.FieldChangedAt {
		if changedAt.After(latest) {
			latest = changedAt
		}
	}
	return latest
}
