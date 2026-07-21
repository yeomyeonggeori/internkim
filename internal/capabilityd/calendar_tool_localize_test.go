package capabilityd

import "testing"

func TestLocalizeCalendarEventTimesRendersEventTimezone(t *testing.T) {
	event := calendarEventForTool{
		TimeZone: "Asia/Seoul",
		StartISO: "2026-07-22T01:00:00Z",
		EndISO:   "2026-07-22T02:00:00Z",
	}

	localized := localizeCalendarEventTimes(event)

	if localized.StartISO != "2026-07-22T10:00:00+09:00" || localized.EndISO != "2026-07-22T11:00:00+09:00" {
		t.Fatalf("expected event-timezone serialization, got %+v", localized)
	}

	unknown := localizeCalendarEventTimes(calendarEventForTool{TimeZone: "Nowhere/Invalid", StartISO: "2026-07-22T01:00:00Z"})
	if unknown.StartISO != "2026-07-22T01:00:00Z" {
		t.Fatalf("expected unknown timezone to keep the original value, got %+v", unknown)
	}
}
