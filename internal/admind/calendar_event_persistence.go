package admind

import (
	"context"
	"database/sql"
	"time"
)

func (service *Service) writeCalendarEvent(ctx context.Context, event calendarEvent) error {
	candidateUpdatedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, time.Now().UTC())
	if errorValue != nil {
		return errorValue
	}
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	return service.writeCalendarEventLockedWithOriginIfCurrent(ctx, event, candidateUpdatedAt, nil, "")
}

func (service *Service) writeCalendarEventLockedWithOriginIfCurrent(ctx context.Context, event calendarEvent, candidateUpdatedAt time.Time, origin *calendarMutationOrigin, expectedUpdatedAt string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if expectedUpdatedAt != "" {
		var matchingVersions int
		errorValue := transaction.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_events WHERE id = ? AND updated_at = ? AND deleted_at = ''`, event.ID, expectedUpdatedAt).Scan(&matchingVersions)
		if errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
		if matchingVersions != 1 {
			_ = transaction.Rollback()
			return errCalendarEventVersionConflict
		}
	}
	logicalTime, errorValue := service.allocateCalendarConflictTime(ctx, transaction, event.UID, candidateUpdatedAt)
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	updatedAt := logicalTime.Format(time.RFC3339Nano)
	event.UpdatedAt = updatedAt
	if errorValue := service.persistCalendarEventMutationWithOrigin(ctx, transaction, event, updatedAt, origin); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func (service *Service) persistCalendarEventMutationWithOrigin(ctx context.Context, transaction *sql.Tx, event calendarEvent, updatedAt string, origin *calendarMutationOrigin) error {
	previousEvent, previousEventFound, errorValue := readCalendarEventWindowMutationEvent(ctx, transaction, event.ID)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := persistCalendarEventWithTransaction(ctx, transaction, event, updatedAt); errorValue != nil {
		return errorValue
	}
	if errorValue := replaceCalendarMutationOrigin(ctx, transaction, event.ID, updatedAt, origin); errorValue != nil {
		return errorValue
	}
	invalidationEvents := []calendarEvent{event}
	if previousEventFound {
		invalidationEvents = append(invalidationEvents, previousEvent)
	}
	return service.invalidateCalendarEventWindowCache(ctx, transaction, invalidationEvents...)
}

func persistCalendarEventWithTransaction(ctx context.Context, transaction *sql.Tx, event calendarEvent, updatedAt string) error {
	_, errorValue := transaction.ExecContext(ctx, `
	INSERT INTO calendar_events (
		id, uid, title, description, location, start_at, end_at, time_zone, is_all_day, color, raw_ics, reminder_lead_hours, created_by_email, created_by_name, updated_by_email, updated_by_name, updated_by_at, mattermost_post_id, updated_at, deleted_at, remote_source, remote_etag, remote_href
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	uid = excluded.uid,
	title = excluded.title,
	description = excluded.description,
	location = excluded.location,
	start_at = excluded.start_at,
	end_at = excluded.end_at,
	time_zone = excluded.time_zone,
	is_all_day = excluded.is_all_day,
	color = excluded.color,
	raw_ics = excluded.raw_ics,
	reminder_lead_hours = excluded.reminder_lead_hours,
	updated_by_email = excluded.updated_by_email,
	updated_by_name = excluded.updated_by_name,
	updated_by_at = excluded.updated_by_at,
	updated_at = excluded.updated_at,
	remote_source = excluded.remote_source,
	remote_etag = excluded.remote_etag,
	remote_href = excluded.remote_href,
	deleted_at = ''`,
		event.ID,
		event.UID,
		event.Title,
		event.Description,
		event.Location,
		event.StartISO,
		event.EndISO,
		event.TimeZone,
		boolToSQLiteInteger(event.IsAllDay),
		event.Color,
		event.RawICS,
		normalizeCalendarReminderLeadHours(event.ReminderLeadHours),
		event.CreatedByEmail,
		event.CreatedByName,
		event.UpdatedByEmail,
		event.UpdatedByName,
		event.UpdatedByAt,
		event.MattermostPostID,
		updatedAt,
		event.RemoteSource,
		event.RemoteETag,
		event.RemoteHref,
	)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := replaceCalendarEventParticipants(ctx, transaction, event.ID, calendarParticipantIdentities(event.Participants)); errorValue != nil {
		return errorValue
	}
	return nil
}

func (service *Service) softDeleteCalendarEvent(ctx context.Context, eventID string) error {
	candidateDeletedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, time.Now().UTC())
	if errorValue != nil {
		return errorValue
	}
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	return service.softDeleteCalendarEventLocked(ctx, eventID, candidateDeletedAt)
}

func (service *Service) softDeleteCalendarEventLocked(ctx context.Context, eventID string, candidateDeletedAt time.Time) error {
	event, found, errorValue := service.readCalendarEventByID(ctx, eventID)
	if errorValue != nil {
		return errorValue
	}
	if !found {
		return sql.ErrNoRows
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := service.persistCalendarEventDeletionWithTransaction(ctx, transaction, event, candidateDeletedAt); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	service.deletePairedTaskForCalendarEvent(ctx, event.ID)
	return nil
}
