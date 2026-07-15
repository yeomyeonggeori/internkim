package admind

import "time"

const calendarEventWindowCacheSchemaVersion = 1
const calendarEventWindowCacheMaximumEntries = 64

type calendarEventWindowCacheRange struct {
	Key      string
	StartISO string
	EndISO   string
}

type calendarEventWindowCachePayload struct {
	Version int             `json:"version"`
	Events  []calendarEvent `json:"events"`
}

func calendarEventWindowCacheRangeFor(startTime time.Time, endTime time.Time) (calendarEventWindowCacheRange, bool) {
	if startTime.IsZero() || endTime.IsZero() || !startTime.Before(endTime) {
		return calendarEventWindowCacheRange{}, false
	}
	startISO := startTime.UTC().Format(time.RFC3339Nano)
	endISO := endTime.UTC().Format(time.RFC3339Nano)
	return calendarEventWindowCacheRange{
		Key:      startISO + "/" + endISO,
		StartISO: startISO,
		EndISO:   endISO,
	}, true
}
