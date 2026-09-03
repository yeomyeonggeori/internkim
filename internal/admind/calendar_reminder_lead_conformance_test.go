package admind

import (
	"encoding/json"
	"os"
	"testing"
)

func TestEveryWholeHourReminderLeadTheWebOffersSurvivesThisDevice(t *testing.T) {
	raw, errorValue := os.ReadFile("../../web/src/lib/calendar/event-reminder-leads.json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var offered struct {
		MinutesBeforeStart []int `json:"minutesBeforeStart"`
	}
	if errorValue := json.Unmarshal(raw, &offered); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(offered.MinutesBeforeStart) == 0 {
		t.Fatal("the web offers no reminder lead")
	}
	for _, minutes := range offered.MinutesBeforeStart {
		if minutes%60 != 0 {
			continue
		}
		hours := minutes / 60
		if normalized := normalizeCalendarReminderLeadHours(hours); normalized != hours {
			t.Errorf("a %d minute lead becomes %d hours on this device, want %d", minutes, normalized, hours)
		}
	}
}
