package admind

import "testing"

func aTranslation() linkTranslation {
	return linkTranslation{
		appURL:     "https://example.test",
		deviceHost: "samplefleet0001.example.test",
		recordCalendarID: func(given string) string {
			if given == "device-event" {
				return "record-event"
			}
			return ""
		},
		recordTaskID: func(given string) string {
			if given == "device-task" {
				return "record-task"
			}
			return ""
		},
	}
}

func TestALinkIsRewrittenWithTheRecordsIdentifier(t *testing.T) {
	rewritten := aTranslation().rewrite(
		"내일 일정입니다 https://samplefleet0001.example.test/calendar/?date=2026-09-22&event=device-event 확인해주세요")

	expected := "내일 일정입니다 https://example.test/calendar/?date=2026-09-22&event=record-event 확인해주세요"
	if rewritten != expected {
		t.Errorf("rewrote to %q", rewritten)
	}
}

// The record has not taken every event this device holds. Carrying the device's
// identifier to the record opens nothing; the day it is on still does.
func TestAnIdentifierTheRecordNeverTookIsLeftOut(t *testing.T) {
	rewritten := aTranslation().rewrite("https://samplefleet0001.example.test/calendar/?date=2026-09-22&event=unknown")

	if rewritten != "https://example.test/calendar/?date=2026-09-22" {
		t.Errorf("rewrote to %q", rewritten)
	}
}

func TestABoardLinkTakesTheRecordsTaskIdentifier(t *testing.T) {
	rewritten := aTranslation().rewrite("https://samplefleet0001.example.test/flow/?week=26W18&task=device-task")

	if rewritten != "https://example.test/flow/?task=record-task&week=26W18" {
		t.Errorf("rewrote to %q", rewritten)
	}
}

// A message can name somewhere else entirely, and a rewrite that reaches into
// one is a rewrite nobody asked for.
func TestALinkToSomewhereElseIsLeftAlone(t *testing.T) {
	original := "https://example.com/calendar/?event=device-event"
	if rewritten := aTranslation().rewrite(original); rewritten != original {
		t.Errorf("rewrote to %q", rewritten)
	}
}

func TestAMessageWithNoLinkIsUnchanged(t *testing.T) {
	original := "오늘 회의 잘 부탁드립니다"
	if rewritten := aTranslation().rewrite(original); rewritten != original {
		t.Errorf("rewrote to %q", rewritten)
	}
}

func TestALinkWithNoIdentifierMovesByItsHost(t *testing.T) {
	translation := linkTranslation{
		appURL:           "https://example.test",
		deviceHost:       "device.example.test",
		recordCalendarID: func(string) string { return "" },
		recordTaskID:     func(string) string { return "" },
	}

	for given, wanted := range map[string]string{
		"https://device.example.test/attendance/": "https://example.test/attendance/",
		"https://device.example.test/flow/":       "https://example.test/flow/",
		"https://device.example.test/memory/":     "https://example.test/memory/",
		"https://device.example.test/":            "https://example.test/",
	} {
		if rewritten := translation.rewrite(given); rewritten != wanted {
			t.Fatalf("expected %q, got %q", wanted, rewritten)
		}
	}
}

func TestALinkToAnotherHostStillMovesNowhere(t *testing.T) {
	translation := linkTranslation{
		appURL:           "https://example.test",
		deviceHost:       "device.example.test",
		recordCalendarID: func(string) string { return "" },
		recordTaskID:     func(string) string { return "" },
	}

	for _, elsewhere := range []string{"https://news.example.com/flow/?task=1", "http://localhost:5173/attendance/"} {
		if rewritten := translation.rewrite(elsewhere); rewritten != elsewhere {
			t.Fatalf("expected %q to be left alone, got %q", elsewhere, rewritten)
		}
	}
}
