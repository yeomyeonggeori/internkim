package admind

import (
	"bytes"
	"errors"
	"io"
	pathpkg "path"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"
)

func buildCalendarFeed(events []calendarEvent) (*ical.Calendar, error) {
	calendar := newCalendarFeedDocument()
	for _, event := range events {
		eventCalendar, errorValue := calendarObjectForEvent(event)
		if errorValue != nil {
			return nil, errorValue
		}
		calendar.Children = append(calendar.Children, eventCalendar.Events()[0].Component)
	}
	return calendar, nil
}

func encodeCalendarObject(event calendarEvent) (string, error) {
	calendar, errorValue := calendarObjectForEvent(event)
	if errorValue != nil {
		return "", errorValue
	}
	var buffer bytes.Buffer
	if errorValue := ical.NewEncoder(&buffer).Encode(calendar); errorValue != nil {
		return "", errorValue
	}
	return buffer.String(), nil
}

func calendarObjectForEvent(event calendarEvent) (*ical.Calendar, error) {
	startTime, errorValue := time.Parse(time.RFC3339, event.StartISO)
	if errorValue != nil {
		return nil, errorValue
	}
	endTime, errorValue := time.Parse(time.RFC3339, event.EndISO)
	if errorValue != nil {
		return nil, errorValue
	}
	calendar := newCalendarDocument()
	calendarEventValue := ical.NewEvent()
	calendarEventValue.Props.SetText(ical.PropUID, event.UID)
	calendarEventValue.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	calendarEventValue.Props.SetText(ical.PropSummary, event.Title)
	setOptionalCalendarText(calendarEventValue, ical.PropDescription, event.Description)
	setOptionalCalendarText(calendarEventValue, ical.PropLocation, event.Location)
	calendarEventValue.Props.SetText(ical.PropColor, event.Color)
	if event.IsAllDay {
		calendarEventValue.Props.SetDate(ical.PropDateTimeStart, startTime)
		calendarEventValue.Props.SetDate(ical.PropDateTimeEnd, endTime)
	} else {
		calendarEventValue.Props.SetDateTime(ical.PropDateTimeStart, startTime.UTC())
		calendarEventValue.Props.SetDateTime(ical.PropDateTimeEnd, endTime.UTC())
	}
	calendar.Children = append(calendar.Children, calendarEventValue.Component)
	return calendar, nil
}

func newCalendarDocument() *ical.Calendar {
	calendar := ical.NewCalendar()
	calendar.Props.SetText(ical.PropVersion, "2.0")
	calendar.Props.SetText(ical.PropProductID, calendarProductID)
	calendar.Props.SetText(ical.PropCalendarScale, "GREGORIAN")
	calendar.Props.SetText("X-WR-CALNAME", calendarName)
	calendar.Props.SetText("X-WR-TIMEZONE", "UTC")
	return calendar
}

func newCalendarFeedDocument() *ical.Calendar {
	calendar := newCalendarDocument()
	calendar.Props.SetText(ical.PropMethod, "PUBLISH")
	return calendar
}

func setOptionalCalendarText(event *ical.Event, propertyName string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	event.Props.SetText(propertyName, strings.TrimSpace(value))
}

func calendarEventFromCalendarObject(path string, calendar *ical.Calendar, actorEmail string) (calendarEvent, error) {
	eventValues := calendar.Events()
	if len(eventValues) == 0 {
		return calendarEvent{}, errors.New("VEVENT is required")
	}
	eventValue := eventValues[0]
	uid, errorValue := eventValue.Props.Text(ical.PropUID)
	if errorValue != nil || strings.TrimSpace(uid) == "" {
		return calendarEvent{}, errors.New("VEVENT UID is required")
	}
	startTime, errorValue := eventValue.DateTimeStart(time.UTC)
	if errorValue != nil {
		return calendarEvent{}, errorValue
	}
	endTime, errorValue := eventValue.DateTimeEnd(time.UTC)
	if errorValue != nil {
		return calendarEvent{}, errorValue
	}
	title, _ := eventValue.Props.Text(ical.PropSummary)
	description, _ := eventValue.Props.Text(ical.PropDescription)
	location, _ := eventValue.Props.Text(ical.PropLocation)
	color, _ := eventValue.Props.Text(ical.PropColor)
	remoteModifiedAt := ""
	if modifiedAt, modifiedAtError := eventValue.Props.DateTime(ical.PropLastModified, time.UTC); modifiedAtError == nil && !modifiedAt.IsZero() {
		remoteModifiedAt = modifiedAt.UTC().Format(time.RFC3339Nano)
	}
	startProperty := eventValue.Props.Get(ical.PropDateTimeStart)
	rawICS, errorValue := encodeExistingCalendar(calendar)
	if errorValue != nil {
		return calendarEvent{}, errorValue
	}
	id := strings.TrimSuffix(pathpkg.Base(path), ".ics")
	if id == "" || id == "." || id == "/" {
		id = strings.TrimSuffix(uid, "@internkim")
	}
	return calendarEvent{
		ID:                id,
		UID:               uid,
		Title:             firstNonEmpty(strings.TrimSpace(title), "Untitled event"),
		Description:       strings.TrimSpace(description),
		Location:          strings.TrimSpace(location),
		StartISO:          startTime.UTC().Format(time.RFC3339),
		EndISO:            endTime.UTC().Format(time.RFC3339),
		TimeZone:          "UTC",
		IsAllDay:          startProperty != nil && startProperty.ValueType() == ical.ValueDate,
		Color:             firstNonEmpty(strings.TrimSpace(color), "#2563eb"),
		ReminderLeadHours: calendarDefaultReminderLeadHours,
		CreatedByEmail:    actorEmail,
		CreatedByName:     actorEmail,
		RemoteModifiedAt:  remoteModifiedAt,
		RawICS:            rawICS,
	}, nil
}

func encodeExistingCalendar(calendar *ical.Calendar) (string, error) {
	var buffer bytes.Buffer
	if errorValue := ical.NewEncoder(&buffer).Encode(calendar); errorValue != nil {
		return "", errorValue
	}
	return buffer.String(), nil
}

func calendarObjectFromEvent(event calendarEvent) (caldav.CalendarObject, error) {
	rawICS := event.RawICS
	if strings.TrimSpace(rawICS) == "" {
		encodedICS, errorValue := encodeCalendarObject(event)
		if errorValue != nil {
			return caldav.CalendarObject{}, errorValue
		}
		rawICS = encodedICS
	}
	calendar, errorValue := decodeCalendarObject(rawICS)
	if errorValue != nil {
		return caldav.CalendarObject{}, errorValue
	}
	modTime, errorValue := time.Parse(time.RFC3339Nano, event.UpdatedAt)
	if errorValue != nil {
		modTime = time.Now().UTC()
	}
	return caldav.CalendarObject{
		Path:          calendarCollectionPath + event.ID + ".ics",
		ModTime:       modTime,
		ContentLength: int64(len(rawICS)),
		ETag:          calendarEventETag(event),
		Data:          calendar,
	}, nil
}

func decodeCalendarObject(rawICS string) (*ical.Calendar, error) {
	decoder := ical.NewDecoder(strings.NewReader(rawICS))
	calendar, errorValue := decoder.Decode()
	if errorValue != nil && !errors.Is(errorValue, io.EOF) {
		return nil, errorValue
	}
	return calendar, nil
}

func calendarEventETag(event calendarEvent) string {
	return event.ID + "-" + strings.ReplaceAll(event.UpdatedAt, `"`, "")
}
