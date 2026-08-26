package capabilityd

import "testing"

func TestLocalizeCalendarEventTimesRendersEventTimezone(t *testing.T) {
	event := calendarEventForTool{
		TimeZone: "Asia/Seoul",
		StartsAt: "2026-07-22T01:00:00Z",
		EndsAt:   "2026-07-22T02:00:00Z",
	}

	localized := localizeCalendarEventTimes(event)

	if localized.StartsAt != "2026-07-22T10:00:00+09:00" || localized.EndsAt != "2026-07-22T11:00:00+09:00" {
		t.Fatalf("expected event-timezone serialization, got %+v", localized)
	}

	unknown := localizeCalendarEventTimes(calendarEventForTool{TimeZone: "Nowhere/Invalid", StartsAt: "2026-07-22T01:00:00Z"})
	if unknown.StartsAt != "2026-07-22T01:00:00Z" {
		t.Fatalf("expected unknown timezone to keep the original value, got %+v", unknown)
	}
}
