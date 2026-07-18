package admind

import (
	"context"
	"strings"
)

type fakeCalDAVUIDQueryCall struct {
	CalendarPath string
	EventUID     string
}

type fakeCalDAVUIDQueryPushClient struct {
	*fakeCalDAVPushClient
	queryObjects []calDAVCalendarObject
	queryError   error
	queryCalls   []fakeCalDAVUIDQueryCall
}

func (client *fakeCalDAVUIDQueryPushClient) queryCalendarObjectsByUID(ctx context.Context, calendarPath string, eventUID string) ([]calDAVCalendarObject, error) {
	client.queryCalls = append(client.queryCalls, fakeCalDAVUIDQueryCall{CalendarPath: calendarPath, EventUID: eventUID})
	return append([]calDAVCalendarObject(nil), client.queryObjects...), client.queryError
}

func calendarObjectWithEventUIDs(eventUIDs ...string) calDAVCalendarObject {
	var builder strings.Builder
	builder.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//InternKim//Calendar//EN\r\n")
	for _, eventUID := range eventUIDs {
		builder.WriteString("BEGIN:VEVENT\r\nUID:")
		builder.WriteString(eventUID)
		builder.WriteString("\r\nDTSTAMP:20260716T000000Z\r\nDTSTART:20260716T010000Z\r\nDTEND:20260716T020000Z\r\nEND:VEVENT\r\n")
	}
	builder.WriteString("END:VCALENDAR\r\n")
	return calDAVCalendarObject{
		Path: "/calendars/me/multiple-events.ics",
		ETag: `"etag-multiple-events"`,
		Data: []byte(builder.String()),
	}
}
