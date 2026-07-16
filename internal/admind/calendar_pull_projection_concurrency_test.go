package admind

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCalendarPullDeferredProjectionQueueDeduplicatesEventIDs(t *testing.T) {
	queue := &calendarPullDeferredProjectionQueue{}
	queue.add("event-1")
	queue.add("event-2")
	queue.add("event-1")
	if len(queue.eventIDs) != 2 || queue.eventIDs[0] != "event-1" || queue.eventIDs[1] != "event-2" {
		t.Fatalf("deferred event IDs=%+v", queue.eventIDs)
	}
}

func TestCalendarPullProjectionDoesNotBlockTargetSwitchOrLocalWrite(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("pull-projection-lock", "Before Pull")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = "/calendars/me/pull-projection-lock.ics"
	event.RemoteETag = `"etag-before"`
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.Configuration.MattermostAdminPasswordPath = writeTestFile(t, "admin-password")
	service.Configuration.MattermostBotTokenPath = writeTestFile(t, "bot-token")
	service.Configuration.MattermostBaseURL = "http://mattermost.local"
	projectionStarted := make(chan struct{})
	projectionRelease := make(chan struct{})
	var requestCount atomic.Int64
	var responseMutex sync.Mutex
	requests := calendarMattermostLogRequests{}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if requestCount.Add(1) == 1 {
			close(projectionStarted)
			<-projectionRelease
		}
		responseMutex.Lock()
		defer responseMutex.Unlock()
		return mattermostCalendarLogLifecycleResponse(t, request, &requests)
	})}
	remoteObject := fakeRemoteObject(t, event.UID, `"etag-after"`, "Remote Pull")
	remoteObject.Path = event.RemoteHref
	pullFinished := make(chan error, 1)
	go func() {
		_, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: `"ctag-after"`, objects: []calDAVCalendarObject{remoteObject}})
		pullFinished <- errorValue
	}()
	select {
	case <-projectionStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("pull projection HTTP did not start")
	}
	selectionFinished := make(chan error, 1)
	go func() {
		_, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
		selectionFinished <- errorValue
	}()
	selectionError, selectionCompletedBeforeRelease := receiveCalendarOperationBeforeTimeout(selectionFinished, 300*time.Millisecond)
	localEvent := event
	localEvent.Title = "Local After Switch"
	localWriteFinished := make(chan error, 1)
	go func() {
		localWriteFinished <- service.writeCalendarEvent(ctx, localEvent)
	}()
	localWriteError, localWriteCompletedBeforeRelease := receiveCalendarOperationBeforeTimeout(localWriteFinished, 300*time.Millisecond)
	close(projectionRelease)
	if !selectionCompletedBeforeRelease {
		selectionError = <-selectionFinished
	}
	if !localWriteCompletedBeforeRelease {
		localWriteError = <-localWriteFinished
	}
	if selectionError != nil {
		t.Fatal(selectionError)
	}
	if localWriteError != nil {
		t.Fatal(localWriteError)
	}
	if errorValue := <-pullFinished; errorValue != nil {
		t.Fatal(errorValue)
	}
	if !selectionCompletedBeforeRelease || !localWriteCompletedBeforeRelease {
		t.Fatalf("projection held calendar lock: selection_completed=%v local_write_completed=%v", selectionCompletedBeforeRelease, localWriteCompletedBeforeRelease)
	}
	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("event found=%v error=%v", found, errorValue)
	}
	if storedEvent.Title != localEvent.Title {
		t.Fatalf("event title=%q want latest local title %q", storedEvent.Title, localEvent.Title)
	}
	if storedEvent.MattermostPostID != "calendar-post-1" {
		t.Fatalf("Mattermost post ID=%q", storedEvent.MattermostPostID)
	}
	responseMutex.Lock()
	createdMessages := append([]string(nil), requests.createdMessages...)
	updatedMessages := append([]string(nil), requests.updatedMessages...)
	responseMutex.Unlock()
	if len(createdMessages) != 1 {
		t.Fatalf("created Mattermost messages=%+v", createdMessages)
	}
	if len(updatedMessages) != 1 || !strings.Contains(updatedMessages[0], localEvent.Title) {
		t.Fatalf("updated Mattermost messages=%+v want latest title %q", updatedMessages, localEvent.Title)
	}
	assertCalendarProjectionOutboxCount(t, service, 0)
}

func TestCalendarLocalProjectionDoesNotHoldStoreMutex(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("local-projection-lock", "Before Local Projection")
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.Configuration.MattermostAdminPasswordPath = writeTestFile(t, "admin-password")
	service.Configuration.MattermostBotTokenPath = writeTestFile(t, "bot-token")
	service.Configuration.MattermostBaseURL = "http://mattermost.local"
	projectionStarted := make(chan struct{})
	projectionRelease := make(chan struct{})
	var requestCount atomic.Int64
	var responseMutex sync.Mutex
	requests := calendarMattermostLogRequests{}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if requestCount.Add(1) == 1 {
			close(projectionStarted)
			<-projectionRelease
		}
		responseMutex.Lock()
		defer responseMutex.Unlock()
		return mattermostCalendarLogLifecycleResponse(t, request, &requests)
	})}
	firstLocalEvent := event
	firstLocalEvent.Title = "First Local Projection"
	firstWriteFinished := make(chan error, 1)
	go func() {
		firstWriteFinished <- service.writeCalendarEvent(ctx, firstLocalEvent)
	}()
	select {
	case <-projectionStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("local projection HTTP did not start")
	}
	selectionFinished := make(chan error, 1)
	go func() {
		_, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
		selectionFinished <- errorValue
	}()
	selectionError, selectionCompletedBeforeRelease := receiveCalendarOperationBeforeTimeout(selectionFinished, 300*time.Millisecond)
	latestLocalEvent := event
	latestLocalEvent.Title = "Latest Local Projection"
	latestWriteFinished := make(chan error, 1)
	go func() {
		latestWriteFinished <- service.writeCalendarEvent(ctx, latestLocalEvent)
	}()
	latestWriteError, latestWriteCompletedBeforeRelease := receiveCalendarOperationBeforeTimeout(latestWriteFinished, 300*time.Millisecond)
	close(projectionRelease)
	if !selectionCompletedBeforeRelease {
		selectionError = <-selectionFinished
	}
	if !latestWriteCompletedBeforeRelease {
		latestWriteError = <-latestWriteFinished
	}
	if selectionError != nil {
		t.Fatal(selectionError)
	}
	if latestWriteError != nil {
		t.Fatal(latestWriteError)
	}
	if errorValue := <-firstWriteFinished; errorValue != nil {
		t.Fatal(errorValue)
	}
	if !selectionCompletedBeforeRelease || !latestWriteCompletedBeforeRelease {
		t.Fatalf("local projection held calendar lock: selection_completed=%v local_write_completed=%v", selectionCompletedBeforeRelease, latestWriteCompletedBeforeRelease)
	}
	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("event found=%v error=%v", found, errorValue)
	}
	if storedEvent.Title != latestLocalEvent.Title || storedEvent.MattermostPostID != "calendar-post-1" {
		t.Fatalf("stored event=%+v", storedEvent)
	}
	responseMutex.Lock()
	createdMessages := append([]string(nil), requests.createdMessages...)
	updatedMessages := append([]string(nil), requests.updatedMessages...)
	responseMutex.Unlock()
	if len(createdMessages) != 1 || len(updatedMessages) != 1 || !strings.Contains(updatedMessages[0], latestLocalEvent.Title) {
		t.Fatalf("created=%+v updated=%+v", createdMessages, updatedMessages)
	}
	assertCalendarProjectionOutboxCount(t, service, 0)
}

func receiveCalendarOperationBeforeTimeout(result <-chan error, timeout time.Duration) (error, bool) {
	select {
	case errorValue := <-result:
		return errorValue, true
	case <-time.After(timeout):
		return nil, false
	}
}
