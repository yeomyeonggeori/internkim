package admind

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

const (
	calendarDigestInterval = time.Minute
	calendarDigestPlatform = "mattermost"
)

func (service *Service) keepTheDayAnnounced(ctx context.Context) {
	for {
		now := time.Now()
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(now.Truncate(calendarDigestInterval).Add(calendarDigestInterval))):
		}
		service.announceTheDayOnce(ctx, time.Now())
	}
}

func (service *Service) announceTheDayOnce(ctx context.Context, now time.Time) {
	client := service.centralPlane()
	if client == nil {
		return
	}
	location, _ := service.workspaceTimeLocation()
	local := now.In(location)
	at := local.Format("15:04")

	externalIDs, errorValue := client.NotifyScheduleAt(ctx, calendarDigestPlatform, at)
	if errorValue != nil {
		log.Printf("calendar digest: the plane did not say whose hour %s is: %v", at, errorValue)
		return
	}
	if len(externalIDs) == 0 {
		return
	}

	events, errorValue := service.calendarEventsOn(ctx, local, location)
	if errorValue != nil {
		log.Printf("calendar digest: the day is unreadable: %v", errorValue)
		return
	}
	service.announceTheDayTo(ctx, client, externalIDs, events, local, location)
}

func (service *Service) announceTheDayTo(
	ctx context.Context,
	client *centralplane.Client,
	externalIDs []string,
	events []calendarEvent,
	local time.Time,
	location *time.Location,
) {
	emailByExternalID := calendarDigestEmails(service.attendanceNotifyDirectory(ctx))
	day := local.Format("2006-01-02")
	for _, externalID := range externalIDs {
		entries := calendarDigestFor(events, emailByExternalID[externalID], location)
		if len(entries) == 0 {
			continue
		}
		notification := centralplane.Notification{
			Platform:    calendarDigestPlatform,
			ExternalIDs: []string{externalID},
			Category:    "calendar",
			Title:       "오늘 일정 " + strconv.Itoa(len(entries)) + "건",
			Body:        calendarDigestBody(entries),
			OpenPath:    "/calendar/?date=" + day,
			Tag:         "calendar-day-" + day,
		}
		if _, errorValue := client.Notify(ctx, notification); errorValue != nil {
			log.Printf("calendar digest: %s was not told about %s: %v", externalID, day, errorValue)
		}
	}
}

func (service *Service) calendarEventsOn(ctx context.Context, local time.Time, location *time.Location) ([]calendarEvent, error) {
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return service.readCalendarEvents(ctx, dayStart, dayStart.AddDate(0, 0, 1))
}

func calendarDigestEmails(records []adminUserMutation) map[string]string {
	emailByExternalID := map[string]string{}
	for _, record := range records {
		if record.MattermostUserID == "" || strings.TrimSpace(record.Email) == "" {
			continue
		}
		emailByExternalID[record.MattermostUserID] = record.Email
	}
	return emailByExternalID
}
