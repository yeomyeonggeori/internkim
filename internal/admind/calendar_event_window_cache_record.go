package admind

import "time"

const calendarEventWindowCacheSchemaVersion = 2
const calendarEventWindowCacheMaximumEntries = 64
const calendarEventWindowCacheTimestampLayout = "2006-01-02T15:04:05.000000000Z"
const calendarEventWindowCacheMaximumDuration = 62 * 24 * time.Hour
const calendarEventWindowCacheMaximumPayloadBytes = 4 << 20
const calendarEventWindowCacheTouchInterval = time.Minute

type calendarEventWindowCacheWriteResult int

const (
	calendarEventWindowCacheWriteSkipped calendarEventWindowCacheWriteResult = iota
	calendarEventWindowCacheWriteRevisionChanged
	calendarEventWindowCacheWriteStored
)

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
	if startTime.IsZero() || endTime.IsZero() || !startTime.Before(endTime) || endTime.Sub(startTime) > calendarEventWindowCacheMaximumDuration {
		return calendarEventWindowCacheRange{}, false
	}
	startISO := formatCalendarEventWindowCacheTimestamp(startTime)
	endISO := formatCalendarEventWindowCacheTimestamp(endTime)
	return calendarEventWindowCacheRange{
		Key:      startISO + "/" + endISO,
		StartISO: startISO,
		EndISO:   endISO,
	}, true
}

func formatCalendarEventWindowCacheTimestamp(value time.Time) string {
	return value.UTC().Format(calendarEventWindowCacheTimestampLayout)
}
