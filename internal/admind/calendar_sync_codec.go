package admind

import (
	"bytes"
	"errors"
	"strings"

	"github.com/emersion/go-ical"
)

func encodeEventToICS(event calendarEvent) ([]byte, error) {
	calendar, errorValue := calendarObjectForEvent(event)
	if errorValue != nil {
		return nil, errorValue
	}
	var buffer bytes.Buffer
	if errorValue := ical.NewEncoder(&buffer).Encode(calendar); errorValue != nil {
		return nil, errorValue
	}
	return buffer.Bytes(), nil
}

func decodeRemoteCalendarObject(object calDAVCalendarObject, accountEmail string) (calendarEvent, error) {
	if object.ConversionError != nil {
		return calendarEvent{}, object.ConversionError
	}
	if len(object.Data) == 0 {
		return calendarEvent{}, errors.New("empty calendar object data")
	}
	calendar, errorValue := decodeCalendarObject(string(object.Data))
	if errorValue != nil {
		return calendarEvent{}, errorValue
	}
	event, errorValue := calendarEventFromCalendarObject(object.Path, calendar, accountEmail)
	if errorValue != nil {
		return calendarEvent{}, errorValue
	}
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = object.ETag
	event.RemoteHref = object.Path
	event.RawICS = string(object.Data)
	return event, nil
}

func decodeCalendarEventFromRawICS(rawICS string, remoteHref string, accountEmail string) calendarEvent {
	trimmed := strings.TrimSpace(rawICS)
	if trimmed == "" {
		return calendarEvent{}
	}
	decoded, errorValue := decodeCalendarObject(trimmed)
	if errorValue != nil {
		return calendarEvent{}
	}
	event, errorValue := calendarEventFromCalendarObject(remoteHref, decoded, accountEmail)
	if errorValue != nil {
		return calendarEvent{}
	}
	return event
}
