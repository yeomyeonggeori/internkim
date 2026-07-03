package admind

import (
	"context"
	"log"
	"strings"
	"time"
)

const mattermostChannelPostRetentionDuration = 14 * 24 * time.Hour

func (service *Service) sweepExpiredCalendarMattermostLogs(ctx context.Context) {
	eventIDs, errorValue := service.readExpiredCalendarMattermostPostEventIDs(ctx, time.Now().UTC().Add(-mattermostChannelPostRetentionDuration))
	if errorValue != nil {
		log.Printf("calendar Mattermost expiry sweep query failed: %v", errorValue)
		return
	}
	for _, eventID := range eventIDs {
		event, found, errorValue := service.readCalendarEventByID(ctx, eventID)
		if errorValue != nil || !found {
			continue
		}
		if errorValue := service.tryDeleteCalendarMattermostLog(ctx, event); errorValue != nil {
			log.Printf("calendar Mattermost expiry sweep delete failed for %s: %v", eventID, errorValue)
			continue
		}
		if errorValue := service.updateCalendarEventMattermostPostID(ctx, eventID, ""); errorValue != nil {
			log.Printf("calendar Mattermost expiry sweep post id clear failed for %s: %v", eventID, errorValue)
		}
	}
}

func (service *Service) readExpiredCalendarMattermostPostEventIDs(ctx context.Context, cutoff time.Time) ([]string, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id
FROM calendar_events
WHERE mattermost_post_id != '' AND mattermost_post_created_at != '' AND mattermost_post_created_at < ?`,
		cutoff.Format(time.RFC3339))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	eventIDs := []string{}
	for rows.Next() {
		var eventID string
		if errorValue := rows.Scan(&eventID); errorValue != nil {
			return nil, errorValue
		}
		eventIDs = append(eventIDs, strings.TrimSpace(eventID))
	}
	return eventIDs, rows.Err()
}
