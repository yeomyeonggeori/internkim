package admind

import (
	"context"
	"database/sql"
	"log"
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
		existing, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
		if errorValue != nil {
			return errorValue
		}
		if found {
			previousEvent = existing
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
	previousMutationEvent, previousMutationEventFound, errorValue := readCalendarEventWindowMutationEvent(ctx, transaction, event.ID)
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue = transaction.ExecContext(ctx, `
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
	mattermost_post_id = excluded.mattermost_post_id,
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
	event.UpdatedAt = updatedAt
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := replaceCalendarEventParticipants(ctx, transaction, event.ID, calendarParticipantIdentities(event.Participants)); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := enqueueCalendarChannelProjection(ctx, transaction, event.ID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	invalidationEvents := []calendarEvent{event}
	if previousMutationEventFound {
		invalidationEvents = append(invalidationEvents, previousMutationEvent)
	}
	if errorValue := service.invalidateCalendarEventWindowCache(ctx, transaction, invalidationEvents...); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	service.upsertCalendarNotifications(ctx, event)
	event = service.applyCalendarMattermostProjection(ctx, event)
	if source == calendarSourceLocal {
		changedFields := diffCalendarEventFields(previousEvent, event)
		if outboxErr := service.enqueueCalendarOutboxForWrite(ctx, event, changedFields); outboxErr != nil {
			log.Printf("calendar outbox enqueue (write) failed: %v", outboxErr)
		}
	}
	return nil
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
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	result, errorValue := transaction.ExecContext(ctx, "UPDATE calendar_events SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at = ''", time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), strings.TrimSpace(eventID))
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
	if errorValue := service.invalidateCalendarEventWindowCache(ctx, transaction, event); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	if errorValue := service.cancelCalendarNotifications(ctx, eventID); errorValue != nil {
		log.Printf("calendar notification cancel failed: %v", errorValue)
	}
	service.applyCalendarMattermostProjectionByID(ctx, event.ID)
	if source == calendarSourceLocal {
		if outboxErr := service.enqueueCalendarOutboxForDelete(ctx, event); outboxErr != nil {
			log.Printf("calendar outbox enqueue (delete) failed: %v", outboxErr)
		}
	}
	return nil
}
