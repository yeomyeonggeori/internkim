package admind

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const (
	calendarRemoteSyncLastSuccessSettingKey = "remote_sync_last_success_at"
	calendarRemoteSyncLeaseSettingKey       = "remote_sync_lease_until"
	calendarRemoteSyncSuccessCacheDuration  = time.Minute
	calendarRemoteSyncLeaseDuration         = 2 * time.Minute
)

type calendarRemoteSyncDecision struct {
	Acquired       bool
	SkippedByCache bool
	SkippedByLease bool
	leaseValue     string
}

func (service *Service) acquireCalendarRemoteSync(ctx context.Context, now time.Time) (calendarRemoteSyncDecision, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return calendarRemoteSyncDecision{}, errorValue
	}
	defer database.Close()
	if errorValue := ensureCalendarRemoteSyncSettings(ctx, database); errorValue != nil {
		return calendarRemoteSyncDecision{}, errorValue
	}
	lastSuccessValue, errorValue := readCalendarRemoteSyncSetting(ctx, database, calendarRemoteSyncLastSuccessSettingKey)
	if errorValue != nil {
		return calendarRemoteSyncDecision{}, errorValue
	}
	if isCalendarRemoteSyncSuccessFresh(lastSuccessValue, now) {
		return calendarRemoteSyncDecision{SkippedByCache: true}, nil
	}
	leaseValue, errorValue := readCalendarRemoteSyncSetting(ctx, database, calendarRemoteSyncLeaseSettingKey)
	if errorValue != nil {
		return calendarRemoteSyncDecision{}, errorValue
	}
	if isCalendarRemoteSyncLeaseActive(leaseValue, now) {
		return calendarRemoteSyncDecision{SkippedByLease: true}, nil
	}
	nextLeaseValue := now.Add(calendarRemoteSyncLeaseDuration).UTC().Format(time.RFC3339Nano)
	result, errorValue := database.ExecContext(
		ctx,
		"UPDATE calendar_settings SET value = ? WHERE key = ? AND value = ?",
		nextLeaseValue,
		calendarRemoteSyncLeaseSettingKey,
		leaseValue,
	)
	if errorValue != nil {
		return calendarRemoteSyncDecision{}, errorValue
	}
	affectedRows, errorValue := result.RowsAffected()
	if errorValue != nil {
		return calendarRemoteSyncDecision{}, errorValue
	}
	if affectedRows == 0 {
		return calendarRemoteSyncDecision{SkippedByLease: true}, nil
	}
	return calendarRemoteSyncDecision{Acquired: true, leaseValue: nextLeaseValue}, nil
}

func (service *Service) finishCalendarRemoteSync(ctx context.Context, decision calendarRemoteSyncDecision, completedAt time.Time, succeeded bool) error {
	if !decision.Acquired || decision.leaseValue == "" {
		return nil
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	if succeeded {
		_, errorValue = database.ExecContext(
			ctx,
			"INSERT INTO calendar_settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
			calendarRemoteSyncLastSuccessSettingKey,
			completedAt.UTC().Format(time.RFC3339Nano),
		)
		if errorValue != nil {
			return errorValue
		}
	}
	_, errorValue = database.ExecContext(
		ctx,
		"UPDATE calendar_settings SET value = '' WHERE key = ? AND value = ?",
		calendarRemoteSyncLeaseSettingKey,
		decision.leaseValue,
	)
	return errorValue
}

func ensureCalendarRemoteSyncSettings(ctx context.Context, database *sql.DB) error {
	for _, key := range []string{calendarRemoteSyncLastSuccessSettingKey, calendarRemoteSyncLeaseSettingKey} {
		if _, errorValue := database.ExecContext(ctx, "INSERT OR IGNORE INTO calendar_settings(key, value) VALUES(?, '')", key); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func readCalendarRemoteSyncSetting(ctx context.Context, database *sql.DB, key string) (string, error) {
	var value string
	errorValue := database.QueryRowContext(ctx, "SELECT value FROM calendar_settings WHERE key = ?", key).Scan(&value)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return "", nil
	}
	return value, errorValue
}

func isCalendarRemoteSyncSuccessFresh(value string, now time.Time) bool {
	successAt, ok := parseCalendarRemoteSyncTime(value)
	return ok && now.Before(successAt.Add(calendarRemoteSyncSuccessCacheDuration))
}

func isCalendarRemoteSyncLeaseActive(value string, now time.Time) bool {
	leaseUntil, ok := parseCalendarRemoteSyncTime(value)
	return ok && leaseUntil.After(now)
}

func parseCalendarRemoteSyncTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	parsedTime, errorValue := time.Parse(time.RFC3339Nano, value)
	if errorValue != nil {
		return time.Time{}, false
	}
	return parsedTime, true
}
