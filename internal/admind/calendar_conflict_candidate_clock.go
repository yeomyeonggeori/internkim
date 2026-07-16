package admind

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

type calendarConflictCandidateClock struct {
	mutex    sync.Mutex
	isSeeded bool
	latest   time.Time
}

func (service *Service) reserveCalendarConflictCandidateTime(ctx context.Context, candidate time.Time) (time.Time, error) {
	return service.calendarCandidateClock.reserve(ctx, service, candidate)
}

func (service *Service) allocateCalendarConflictTime(ctx context.Context, transaction *sql.Tx, eventUID string, candidate time.Time) (time.Time, error) {
	return allocateCalendarConflictTimeWithCandidateClock(ctx, transaction, &service.calendarCandidateClock, eventUID, candidate)
}

func allocateCalendarConflictTimeWithCandidateClock(ctx context.Context, transaction *sql.Tx, candidateClock *calendarConflictCandidateClock, eventUID string, candidate time.Time) (time.Time, error) {
	logicalTime, errorValue := allocateCalendarConflictTime(ctx, transaction, eventUID, candidate)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	candidateClock.observe(logicalTime)
	return logicalTime, nil
}

func (clock *calendarConflictCandidateClock) reserve(ctx context.Context, service *Service, candidate time.Time) (time.Time, error) {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	if !clock.isSeeded {
		seed, errorValue := readCalendarConflictCandidateSeed(ctx, service)
		if errorValue != nil {
			return time.Time{}, errorValue
		}
		if seed.After(clock.latest) {
			clock.latest = seed
		}
		clock.isSeeded = true
	}
	reserved := candidate.UTC()
	if !reserved.After(clock.latest) {
		reserved = clock.latest.Add(time.Nanosecond)
	}
	clock.latest = reserved
	return reserved, nil
}

func (clock *calendarConflictCandidateClock) observe(candidate time.Time) {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	candidate = candidate.UTC()
	if candidate.After(clock.latest) {
		clock.latest = candidate
	}
}

func readCalendarConflictCandidateSeed(ctx context.Context, service *Service) (time.Time, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	defer database.Close()
	latest := time.Time{}
	logicalClockRows, errorValue := database.QueryContext(ctx, `SELECT logical_time_unix_nano FROM calendar_event_logical_clocks`)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	for logicalClockRows.Next() {
		var logicalTimeUnixNano int64
		if errorValue := logicalClockRows.Scan(&logicalTimeUnixNano); errorValue != nil {
			logicalClockRows.Close()
			return time.Time{}, errorValue
		}
		logicalTime := time.Unix(0, logicalTimeUnixNano).UTC()
		if logicalTime.After(latest) {
			latest = logicalTime
		}
	}
	if errorValue := logicalClockRows.Err(); errorValue != nil {
		logicalClockRows.Close()
		return time.Time{}, errorValue
	}
	if errorValue := logicalClockRows.Close(); errorValue != nil {
		return time.Time{}, errorValue
	}
	legacyRows, errorValue := database.QueryContext(ctx, `
SELECT updated_at FROM calendar_events
UNION ALL SELECT deleted_at FROM calendar_events
UNION ALL SELECT created_at FROM calendar_outbox
UNION ALL SELECT last_seen_at FROM calendar_remote_event_sync_state
UNION ALL SELECT missing_detected_at FROM calendar_remote_event_sync_state`)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	defer legacyRows.Close()
	for legacyRows.Next() {
		var rawTime string
		if errorValue := legacyRows.Scan(&rawTime); errorValue != nil {
			return time.Time{}, errorValue
		}
		legacyTime := parseCalendarConflictTime(rawTime)
		if legacyTime.After(latest) {
			latest = legacyTime
		}
	}
	return latest, legacyRows.Err()
}
