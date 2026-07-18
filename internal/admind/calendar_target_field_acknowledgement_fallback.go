package admind

import (
	"context"
	"database/sql"
	"time"
)

type calendarTargetFieldAcknowledgementFallbackEvent struct {
	accountID           string
	selectedCalendarID  string
	selectedCalendarURL string
	defaultCalendarURL  string
	eventUID            string
	rawICS              string
	remoteHref          string
	updatedAt           string
}

func seedCurrentCalendarTargetFieldAcknowledgementFallbacks(ctx context.Context, transaction *sql.Tx) error {
	events, errorValue := readCalendarTargetFieldAcknowledgementFallbackEvents(ctx, transaction)
	if errorValue != nil {
		return errorValue
	}
	for _, event := range events {
		if errorValue := seedCurrentCalendarTargetFieldAcknowledgementFallback(ctx, transaction, event); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func readCalendarTargetFieldAcknowledgementFallbackEvents(ctx context.Context, transaction *sql.Tx) ([]calendarTargetFieldAcknowledgementFallbackEvent, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT account.id, account.selected_calendar_id, account.selected_calendar_url, account.default_calendar_url,
	event.uid, event.raw_ics, event.remote_href, event.updated_at
FROM calendar_events event
JOIN calendar_remote_accounts account ON account.provider = event.remote_source
WHERE event.deleted_at = '' AND event.remote_href <> ''`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	events := []calendarTargetFieldAcknowledgementFallbackEvent{}
	for rows.Next() {
		var event calendarTargetFieldAcknowledgementFallbackEvent
		if errorValue := rows.Scan(
			&event.accountID,
			&event.selectedCalendarID,
			&event.selectedCalendarURL,
			&event.defaultCalendarURL,
			&event.eventUID,
			&event.rawICS,
			&event.remoteHref,
			&event.updatedAt,
		); errorValue != nil {
			return nil, errorValue
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func seedCurrentCalendarTargetFieldAcknowledgementFallback(ctx context.Context, transaction *sql.Tx, event calendarTargetFieldAcknowledgementFallbackEvent) error {
	targetCalendarURL := resolveCalendarOutboxTargetURL(event.selectedCalendarID, event.selectedCalendarURL, event.defaultCalendarURL)
	if !calendarRemoteHrefBelongsToTargetURL(event.remoteHref, targetCalendarURL) {
		return nil
	}
	baseline := calendarExistingEventFieldClockBaseline(event.rawICS, event.remoteHref, event.updatedAt)
	if baseline.IsZero() {
		return nil
	}
	existing, errorValue := readCalendarTargetFieldAcknowledgements(ctx, transaction, event.accountID, targetCalendarURL, event.eventUID)
	if errorValue != nil {
		return errorValue
	}
	pendingFields, errorValue := readPendingCalendarTargetFields(ctx, transaction, event.accountID, targetCalendarURL, event.eventUID)
	if errorValue != nil {
		return errorValue
	}
	acknowledgements := map[string]time.Time{}
	for _, field := range calendarAllUserEditableFields() {
		if existing[field].IsZero() && !calendarFieldListIncludes(pendingFields, field) {
			acknowledgements[field] = baseline
		}
	}
	return persistCalendarTargetFieldAcknowledgements(ctx, transaction, event.accountID, targetCalendarURL, event.eventUID, acknowledgements)
}
