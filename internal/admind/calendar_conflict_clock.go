package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func allocateCalendarConflictTime(ctx context.Context, transaction *sql.Tx, eventUID string, candidate time.Time) (time.Time, error) {
	trimmedEventUID := strings.TrimSpace(eventUID)
	if trimmedEventUID == "" {
		return time.Time{}, fmt.Errorf("allocate calendar conflict time: event UID is empty")
	}
	logicalTimeUnixNano, errorValue := advanceCalendarConflictClock(ctx, transaction, trimmedEventUID, candidate.UTC().UnixNano())
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	legacyTime, errorValue := readLatestLegacyCalendarConflictTime(ctx, transaction, trimmedEventUID)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	logicalTime := time.Unix(0, logicalTimeUnixNano).UTC()
	if legacyTime.IsZero() || logicalTime.After(legacyTime) {
		return logicalTime, nil
	}
	logicalTime = legacyTime.Add(time.Nanosecond)
	if _, errorValue := transaction.ExecContext(ctx, `UPDATE calendar_event_logical_clocks SET logical_time_unix_nano = ? WHERE event_uid = ?`, logicalTime.UnixNano(), trimmedEventUID); errorValue != nil {
		return time.Time{}, errorValue
	}
	return logicalTime, nil
}

func advanceCalendarConflictClock(ctx context.Context, transaction *sql.Tx, eventUID string, candidateUnixNano int64) (int64, error) {
	var logicalTimeUnixNano int64
	errorValue := transaction.QueryRowContext(ctx, `
INSERT INTO calendar_event_logical_clocks(event_uid, logical_time_unix_nano)
VALUES(?, ?)
ON CONFLICT(event_uid) DO UPDATE SET
	logical_time_unix_nano = CASE
		WHEN excluded.logical_time_unix_nano > calendar_event_logical_clocks.logical_time_unix_nano THEN excluded.logical_time_unix_nano
		ELSE calendar_event_logical_clocks.logical_time_unix_nano + 1
	END
RETURNING logical_time_unix_nano`, eventUID, candidateUnixNano).Scan(&logicalTimeUnixNano)
	return logicalTimeUnixNano, errorValue
}

func readLatestLegacyCalendarConflictTime(ctx context.Context, transaction *sql.Tx, eventUID string) (time.Time, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT updated_at FROM calendar_events WHERE uid = ?
UNION ALL SELECT deleted_at FROM calendar_events WHERE uid = ?
UNION ALL SELECT created_at FROM calendar_outbox WHERE event_uid = ?
UNION ALL SELECT last_seen_at FROM calendar_remote_event_sync_state WHERE event_uid = ?
UNION ALL SELECT missing_detected_at FROM calendar_remote_event_sync_state WHERE event_uid = ?`, eventUID, eventUID, eventUID, eventUID, eventUID)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	defer rows.Close()
	latestTime := time.Time{}
	for rows.Next() {
		var rawTime string
		if errorValue := rows.Scan(&rawTime); errorValue != nil {
			return time.Time{}, errorValue
		}
		parsedTime := parseCalendarConflictTime(rawTime)
		if parsedTime.After(latestTime) {
			latestTime = parsedTime
		}
	}
	return latestTime, rows.Err()
}
