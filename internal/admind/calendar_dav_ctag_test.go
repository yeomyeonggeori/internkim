package admind

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
)

func TestCalendarCTagChangesAfterEventWrite(t *testing.T) {
	service := newCalendarTestService(t)
	ctagBefore, errorValue := service.computeCalendarCTag(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	backend := calendarDAVBackend{service: service}
	cal := newCalendarDocumentWithEvent("ctag-change@example.com", "Trigger")
	if _, errorValue := backend.PutCalendarObject(context.Background(), calendarCollectionPath+"ctag-change.ics", cal, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	ctagAfter, errorValue := service.computeCalendarCTag(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if ctagBefore == ctagAfter {
		t.Fatalf("ctag did not change after event write: before=%s after=%s", ctagBefore, ctagAfter)
	}
}

func TestCalendarCTagReflectsActiveEventsOnly(t *testing.T) {
	service := newCalendarTestService(t)
	backend := calendarDAVBackend{service: service}
	ctx := context.Background()

	persistent := newCalendarDocumentWithEvent("ctag-active@example.com", "Persistent")
	if _, errorValue := backend.PutCalendarObject(ctx, calendarCollectionPath+"ctag-active.ics", persistent, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	ctagWithOne, errorValue := service.computeCalendarCTag(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	transient := newCalendarDocumentWithEvent("ctag-transient@example.com", "Transient")
	if _, errorValue := backend.PutCalendarObject(ctx, calendarCollectionPath+"ctag-transient.ics", transient, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	ctagWithTwo, errorValue := service.computeCalendarCTag(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if ctagWithOne == ctagWithTwo {
		t.Fatalf("ctag should change when an active event is added: %s", ctagWithOne)
	}

	if errorValue := backend.DeleteCalendarObject(ctx, calendarCollectionPath+"ctag-transient.ics"); errorValue != nil {
		t.Fatal(errorValue)
	}
	ctagAfterDelete, errorValue := service.computeCalendarCTag(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if ctagAfterDelete != ctagWithOne {
		t.Fatalf("ctag should reflect active events only: after-delete=%s want=%s", ctagAfterDelete, ctagWithOne)
	}
}

func TestCalendarWebUpdatePreservesCalDAVUID(t *testing.T) {
	service := newCalendarTestService(t)
	backend := calendarDAVBackend{service: service}

	const appleStyleID = "37552144-BF3D-45A1-BD41-339C97034310"
	calendarDocument := newCalendarDocument()
	caldavEvent := ical.NewEvent()
	caldavEvent.Props.SetText(ical.PropUID, appleStyleID)
	caldavEvent.Props.SetDateTime(ical.PropDateTimeStamp, time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC))
	caldavEvent.Props.SetText(ical.PropSummary, "점심1")
	caldavEvent.Props.SetDateTime(ical.PropDateTimeStart, time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC))
	caldavEvent.Props.SetDateTime(ical.PropDateTimeEnd, time.Date(2026, 5, 15, 1, 0, 0, 0, time.UTC))
	calendarDocument.Children = append(calendarDocument.Children, caldavEvent.Component)
	if _, errorValue := backend.PutCalendarObject(context.Background(), calendarCollectionPath+appleStyleID+".ics", calendarDocument, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	storedAfterPut, found, errorValue := service.readCalendarEventByID(context.Background(), appleStyleID)
	if errorValue != nil || !found {
		t.Fatalf("event lookup after CalDAV PUT: found=%v error=%v", found, errorValue)
	}
	if storedAfterPut.UID != appleStyleID {
		t.Fatalf("uid column after CalDAV PUT = %q want %q", storedAfterPut.UID, appleStyleID)
	}
	if !strings.Contains(storedAfterPut.RawICS, "UID:"+appleStyleID+"\r\n") {
		t.Fatalf("RawICS UID after CalDAV PUT should not have suffix; raw=%s", storedAfterPut.RawICS)
	}

	updatePayload := fmt.Sprintf(`{
		"title":"저녁1",
		"startISO":"2026-05-22T00:00:00Z",
		"endISO":"2026-05-22T01:00:00Z",
		"timeZone":"Asia/Seoul",
		"color":"#3b82f6",
		"expectedUpdatedAt":%q
	}`, storedAfterPut.UpdatedAt)
	updateRequest := httptest.NewRequest(http.MethodPut, "/calendar/api/events/"+appleStyleID, strings.NewReader(updatePayload))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	updateResponse := httptest.NewRecorder()
	service.router().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", updateResponse.Code, updateResponse.Body.String())
	}

	storedAfterUpdate, found, errorValue := service.readCalendarEventByID(context.Background(), appleStyleID)
	if errorValue != nil || !found {
		t.Fatalf("event lookup after web update: found=%v error=%v", found, errorValue)
	}
	if storedAfterUpdate.UID != appleStyleID {
		t.Fatalf("uid column after web update = %q want %q (preserved)", storedAfterUpdate.UID, appleStyleID)
	}
	if !strings.Contains(storedAfterUpdate.RawICS, "UID:"+appleStyleID+"\r\n") {
		t.Fatalf("RawICS UID after web update should equal uid column without suffix\nraw=%s", storedAfterUpdate.RawICS)
	}
	if strings.Contains(storedAfterUpdate.RawICS, "UID:"+appleStyleID+"@internkim") {
		t.Fatalf("RawICS UID after web update must not gain @internkim suffix\nraw=%s", storedAfterUpdate.RawICS)
	}

	caldavObject, errorValue := calendarObjectFromEvent(storedAfterUpdate)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	caldavUID, _ := caldavObject.Data.Events()[0].Props.Text(ical.PropUID)
	icsFeed, errorValue := buildCalendarFeed([]calendarEvent{storedAfterUpdate})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	icsUID, _ := icsFeed.Events()[0].Props.Text(ical.PropUID)
	if caldavUID != icsUID {
		t.Fatalf("CalDAV UID %q must equal ICS feed UID %q after web update", caldavUID, icsUID)
	}
}
