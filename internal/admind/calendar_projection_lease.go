package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type calendarProjectionLease struct {
	EventID    string
	Owner      string
	Generation int64
}

func (service *Service) acquireCalendarProjectionLease(ctx context.Context, eventID string) (calendarProjectionLease, bool, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return calendarProjectionLease{}, false, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return calendarProjectionLease{}, false, errorValue
	}
	normalizedEventID := strings.TrimSpace(eventID)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO calendar_channel_outbox (event_id, generation, lease_owner, lease_generation, attempt_count, last_error, created_at, updated_at, last_attempted_at)
VALUES (?, 1, '', 0, 0, '', ?, ?, '')
ON CONFLICT(event_id) DO NOTHING`, normalizedEventID, now, now); errorValue != nil {
		_ = transaction.Rollback()
		return calendarProjectionLease{}, false, errorValue
	}
	owner := randomHex(16)
	result, errorValue := transaction.ExecContext(ctx, `
UPDATE calendar_channel_outbox
SET lease_owner = ?, lease_generation = generation
WHERE event_id = ? AND lease_owner = ''`, owner, normalizedEventID)
	if errorValue != nil {
		_ = transaction.Rollback()
		return calendarProjectionLease{}, false, errorValue
	}
	affectedRows, errorValue := result.RowsAffected()
	if errorValue != nil {
		_ = transaction.Rollback()
		return calendarProjectionLease{}, false, errorValue
	}
	if affectedRows == 0 {
		_ = transaction.Rollback()
		return calendarProjectionLease{}, false, nil
	}
	var generation int64
	if errorValue := transaction.QueryRowContext(ctx, `SELECT generation FROM calendar_channel_outbox WHERE event_id = ? AND lease_owner = ?`, normalizedEventID, owner).Scan(&generation); errorValue != nil {
		_ = transaction.Rollback()
		return calendarProjectionLease{}, false, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return calendarProjectionLease{}, false, errorValue
	}
	return calendarProjectionLease{EventID: normalizedEventID, Owner: owner, Generation: generation}, true, nil
}

func (service *Service) completeCalendarProjectionGeneration(ctx context.Context, lease calendarProjectionLease) (calendarProjectionLease, bool, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return calendarProjectionLease{}, false, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return calendarProjectionLease{}, false, errorValue
	}
	currentGeneration, errorValue := readCalendarProjectionLeaseGeneration(ctx, transaction, lease)
	if errorValue != nil {
		_ = transaction.Rollback()
		return calendarProjectionLease{}, false, errorValue
	}
	if currentGeneration == lease.Generation {
		result, errorValue := transaction.ExecContext(ctx, `DELETE FROM calendar_channel_outbox WHERE event_id = ? AND lease_owner = ? AND generation = ?`, lease.EventID, lease.Owner, lease.Generation)
		if errorValue != nil {
			_ = transaction.Rollback()
			return calendarProjectionLease{}, false, errorValue
		}
		affectedRows, errorValue := result.RowsAffected()
		if errorValue != nil || affectedRows != 1 {
			_ = transaction.Rollback()
			if errorValue != nil {
				return calendarProjectionLease{}, false, errorValue
			}
			return calendarProjectionLease{}, false, errors.New("calendar projection lease completion lost ownership")
		}
		return calendarProjectionLease{}, false, transaction.Commit()
	}
	if currentGeneration < lease.Generation {
		_ = transaction.Rollback()
		return calendarProjectionLease{}, false, errors.New("calendar projection generation moved backwards")
	}
	if _, errorValue := transaction.ExecContext(ctx, `UPDATE calendar_channel_outbox SET lease_generation = ? WHERE event_id = ? AND lease_owner = ?`, currentGeneration, lease.EventID, lease.Owner); errorValue != nil {
		_ = transaction.Rollback()
		return calendarProjectionLease{}, false, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return calendarProjectionLease{}, false, errorValue
	}
	lease.Generation = currentGeneration
	return lease, true, nil
}

func (service *Service) releaseCalendarProjectionLeaseAfterFailure(ctx context.Context, lease calendarProjectionLease, cause error) (calendarProjectionLease, bool, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return calendarProjectionLease{}, false, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return calendarProjectionLease{}, false, errorValue
	}
	currentGeneration, errorValue := readCalendarProjectionLeaseGeneration(ctx, transaction, lease)
	if errorValue != nil {
		_ = transaction.Rollback()
		return calendarProjectionLease{}, false, errorValue
	}
	if currentGeneration > lease.Generation {
		if _, errorValue := transaction.ExecContext(ctx, `UPDATE calendar_channel_outbox SET lease_generation = ? WHERE event_id = ? AND lease_owner = ?`, currentGeneration, lease.EventID, lease.Owner); errorValue != nil {
			_ = transaction.Rollback()
			return calendarProjectionLease{}, false, errorValue
		}
		if errorValue := transaction.Commit(); errorValue != nil {
			return calendarProjectionLease{}, false, errorValue
		}
		lease.Generation = currentGeneration
		return lease, true, nil
	}
	_, errorValue = transaction.ExecContext(ctx, `
UPDATE calendar_channel_outbox
SET attempt_count = attempt_count + 1, last_error = ?, last_attempted_at = ?, lease_owner = '', lease_generation = 0
WHERE event_id = ? AND lease_owner = ? AND generation = ?`, cause.Error(), time.Now().UTC().Format(time.RFC3339Nano), lease.EventID, lease.Owner, lease.Generation)
	if errorValue != nil {
		_ = transaction.Rollback()
		return calendarProjectionLease{}, false, errorValue
	}
	return calendarProjectionLease{}, false, transaction.Commit()
}

func readCalendarProjectionLeaseGeneration(ctx context.Context, transaction *sql.Tx, lease calendarProjectionLease) (int64, error) {
	var generation int64
	var owner string
	errorValue := transaction.QueryRowContext(ctx, `SELECT generation, lease_owner FROM calendar_channel_outbox WHERE event_id = ?`, lease.EventID).Scan(&generation, &owner)
	if errorValue != nil {
		return 0, errorValue
	}
	if owner != lease.Owner {
		return 0, fmt.Errorf("calendar projection lease owner changed for event %q", lease.EventID)
	}
	return generation, nil
}
