package admind

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"time"
)

func (service *Service) writeCalendarEvent(ctx context.Context, event calendarEvent) error {
	return service.writeCalendarEventWithSource(ctx, event, calendarSourceLocal)
}

func (service *Service) writeCalendarEventWithSource(ctx context.Context, event calendarEvent, source string) error {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	return service.writeCalendarEventWithSourceLocked(ctx, event, source)
}

func (service *Service) writeCalendarEventWithSourceLocked(ctx context.Context, event calendarEvent, source string) error {
	var previousEvent calendarEvent
	if source == calendarSourceLocal {
		existing, found, _ := service.readCalendarEventByID(ctx, event.ID)
		if found {
			previousEvent = existing
		}
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	event.UpdatedAt = updatedAt
	var outboxRow calendarOutboxRow
	shouldSignalSync := false
	if source == calendarSourceLocal {
		changedFields := diffCalendarEventFields(previousEvent, event)
		var errorValue error
		outboxRow, shouldSignalSync, errorValue = service.prepareCalendarOutboxForWrite(ctx, event, changedFields)
		if errorValue != nil {
			return errorValue
		}
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
	if errorValue := persistCalendarEventWithTransaction(ctx, transaction, event, updatedAt); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if shouldSignalSync {
		if errorValue := enqueueCalendarOutboxWithRunner(ctx, transaction, outboxRow, updatedAt); errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	service.runCalendarStoreSideEffectUnlocked(func() {
		event = service.finishCalendarEventPersistence(ctx, event)
	})
	if shouldSignalSync {
		service.signalCalendarSyncWakeUp()
	}
	return nil
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
	if errorValue := enqueueCalendarChannelProjection(ctx, transaction, event.ID); errorValue != nil {
		return errorValue
	}
	return nil
}

func (service *Service) finishCalendarEventPersistence(ctx context.Context, event calendarEvent) calendarEvent {
	service.upsertCalendarNotifications(ctx, event)
	return service.applyCalendarMattermostProjection(ctx, event)
}

func (service *Service) softDeleteCalendarEvent(ctx context.Context, eventID string) error {
	return service.softDeleteCalendarEventWithSource(ctx, eventID, calendarSourceLocal)
}

func (service *Service) softDeleteCalendarEventWithSource(ctx context.Context, eventID string, source string) error {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	return service.softDeleteCalendarEventWithSourceLocked(ctx, eventID, source)
}

func (service *Service) softDeleteCalendarEventWithSourceLocked(ctx context.Context, eventID string, source string) error {
	event, found, errorValue := service.readCalendarEventByID(ctx, eventID)
	if errorValue != nil {
		return errorValue
	}
	if !found {
		return sql.ErrNoRows
	}
	deletedAt := time.Now().UTC().Format(time.RFC3339Nano)
	var outboxRow calendarOutboxRow
	shouldSignalSync := false
	if source == calendarSourceLocal {
		outboxRow, shouldSignalSync, errorValue = service.prepareCalendarOutboxForDelete(ctx, event)
		if errorValue != nil {
			return errorValue
		}
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
	result, errorValue := transaction.ExecContext(ctx, "UPDATE calendar_events SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at = ''", deletedAt, deletedAt, strings.TrimSpace(eventID))
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	affectedRows, errorValue := result.RowsAffected()
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if affectedRows == 0 {
		_ = transaction.Rollback()
		return sql.ErrNoRows
	}
	if errorValue := enqueueCalendarChannelProjection(ctx, transaction, eventID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if shouldSignalSync {
		if errorValue := enqueueCalendarOutboxWithRunner(ctx, transaction, outboxRow, deletedAt); errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	service.runCalendarStoreSideEffectUnlocked(func() {
		if errorValue := service.cancelCalendarNotifications(ctx, eventID); errorValue != nil {
			slog.WarnContext(ctx, "calendar notification cancel failed", "event_id", eventID, "error", errorValue)
		}
		service.applyCalendarMattermostProjectionByID(ctx, event.ID)
	})
	if shouldSignalSync {
		service.signalCalendarSyncWakeUp()
	}
	return nil
}
