package admind

import (
	"context"
	"fmt"
	"strings"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"
)

func (client *outboundCalDAVClient) queryCalendarObjectsByUID(ctx context.Context, calendarPath string, eventUID string) ([]calDAVCalendarObject, error) {
	query := &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{
			Name:     "VCALENDAR",
			AllProps: true,
			AllComps: true,
		},
		CompFilter: caldav.CompFilter{
			Name: "VCALENDAR",
			Comps: []caldav.CompFilter{{
				Name: "VEVENT",
				Props: []caldav.PropFilter{{
					Name: "UID",
					TextMatch: &caldav.TextMatch{
						Text: eventUID,
					},
				}},
			}},
		},
	}
	objects, errorValue := client.caldav.QueryCalendar(ctx, calDAVPathOnly(calendarPath), query)
	if errorValue != nil {
		return nil, errorValue
	}
	return convertCalDAVObjects(objects), nil
}

func exactCalDAVCalendarObjectUIDMatches(objects []calDAVCalendarObject, eventUID string) ([]calDAVCalendarObject, error) {
	matches := make([]calDAVCalendarObject, 0, len(objects))
	for _, object := range objects {
		matchesEventUID, errorValue := calDAVCalendarObjectMatchesUID(object, eventUID)
		if errorValue != nil {
			return nil, errorValue
		}
		if matchesEventUID {
			matches = append(matches, object)
		}
	}
	return matches, nil
}

func calDAVCalendarObjectMatchesUID(object calDAVCalendarObject, eventUID string) (bool, error) {
	if object.ConversionError != nil {
		return false, fmt.Errorf("decode CalDAV calendar object %s: %w", object.Path, object.ConversionError)
	}
	calendar, errorValue := decodeCalendarObject(string(object.Data))
	if errorValue != nil {
		return false, fmt.Errorf("decode CalDAV calendar object %s: %w", object.Path, errorValue)
	}
	if calendar == nil {
		return false, fmt.Errorf("decode CalDAV calendar object %s: calendar data is empty", object.Path)
	}
	events := calendar.Events()
	if len(events) == 0 {
		return false, fmt.Errorf("decode CalDAV calendar object %s: VEVENT is required", object.Path)
	}
	objectUID := ""
	for index, event := range events {
		uid, errorValue := event.Props.Text(ical.PropUID)
		if errorValue != nil {
			return false, fmt.Errorf("decode CalDAV calendar object %s VEVENT UID: %w", object.Path, errorValue)
		}
		if index == 0 {
			objectUID = uid
			continue
		}
		if uid != objectUID {
			return false, fmt.Errorf("decode CalDAV calendar object %s: mixed VEVENT UIDs %q and %q", object.Path, objectUID, uid)
		}
	}
	return objectUID == eventUID, nil
}

func strongCalDAVCalendarObjectETag(object calDAVCalendarObject) (string, error) {
	etag := strings.TrimSpace(object.ETag)
	isWeak := len(etag) >= 2 && strings.EqualFold(etag[:2], "W/")
	if etag == "" || etag == caldavWildcardETag || isWeak {
		return "", fmt.Errorf("CalDAV calendar object %s requires a strong ETag", object.Path)
	}
	return etag, nil
}
