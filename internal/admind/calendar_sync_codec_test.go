package admind

import "testing"

func TestDecodeRemoteCalendarObjectReadsLastModified(t *testing.T) {
	object := calDAVCalendarObject{
		Path: "/calendars/me/event-1.ics",
		ETag: `"etag-1"`,
		Data: []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//InternKim//Calendar//EN\r\nBEGIN:VEVENT\r\nUID:event-1@example.com\r\nDTSTAMP:20260715T000000Z\r\nLAST-MODIFIED:20260715T010203Z\r\nDTSTART:20260716T010000Z\r\nDTEND:20260716T020000Z\r\nSUMMARY:Planning\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"),
	}
	event, errorValue := decodeRemoteCalendarObject(object, "user@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if event.RemoteModifiedAt != "2026-07-15T01:02:03Z" {
		t.Fatalf("remote modified at=%q", event.RemoteModifiedAt)
	}
}
