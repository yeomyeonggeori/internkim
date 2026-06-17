package admind

import (
	"testing"
	"time"
)

func TestCalendarMattermostEventDateTextUsesConfiguredTimezone(t *testing.T) {
	seoul, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Skipf("Asia/Seoul tzdata unavailable: %v", errorValue)
	}
	for _, testCase := range []struct {
		name     string
		timeZone string
	}{
		{name: "empty timezone falls back to workspace", timeZone: ""},
		{name: "UTC timezone falls back to workspace", timeZone: "UTC"},
		{name: "explicit event timezone is honored", timeZone: "Asia/Seoul"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			event := calendarEvent{
				StartISO: "2026-06-18T03:00:00Z",
				EndISO:   "2026-06-18T04:00:00Z",
				TimeZone: testCase.timeZone,
			}
			got := calendarMattermostEventDateText(event, seoul)
			if got != "2026-06-18 12:00 - 13:00" {
				t.Fatalf("expected workspace-local time, got %q", got)
			}
		})
	}
}

func TestCalendarDisplayLocationPrefersEventTimezoneThenFallback(t *testing.T) {
	fallback := time.FixedZone("KST", 9*60*60)
	if location := calendarDisplayLocation("", fallback); location != fallback {
		t.Fatalf("empty timezone should use fallback, got %v", location)
	}
	if location := calendarDisplayLocation("UTC", fallback); location != fallback {
		t.Fatalf("UTC stored default should use fallback, got %v", location)
	}
	if location := calendarDisplayLocation("", nil); location != time.UTC {
		t.Fatalf("nil fallback should default to UTC, got %v", location)
	}
}
