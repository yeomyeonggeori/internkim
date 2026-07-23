package admind

import (
	"testing"
	"time"
)

func TestFlowBusinessTimezoneOwnsTheCurrentWeek(t *testing.T) {
	sundayUTC := time.Date(2026, time.July, 19, 15, 30, 0, 0, time.UTC)
	mondaySeoul := sundayUTC.In(flowDateLocation())

	if mondaySeoul.Weekday() != time.Monday {
		t.Fatalf("expected Monday in Seoul, got %s", mondaySeoul.Weekday())
	}
	if weekCodeForDate(sundayUTC) != "26W29" {
		t.Fatalf("expected UTC instant in W29, got %s", weekCodeForDate(sundayUTC))
	}
	if weekCodeForDate(mondaySeoul) != "26W30" {
		t.Fatalf("expected Seoul business date in W30, got %s", weekCodeForDate(mondaySeoul))
	}
}
