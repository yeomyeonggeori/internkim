package admind

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPushCalendarOutboxRecoversMissingDeleteRemoteStateByUID(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	eventUID := "recover-delete-by-uid@internkim"
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID: account.ID,
		EventID:   "recover-delete-by-uid",
		EventUID:  eventUID,
		Operation: calendarOutboxOperationDelete,
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	seedCalendarPushObservationFenceForTest(t, service, account.ID, account.DefaultCalendarURL, eventUID)

	canonicalPath := account.DefaultCalendarURL + eventUID + ".ics"
	noncanonicalPath := account.DefaultCalendarURL + "server-generated-42.ics"
	remoteObject := fakeRemoteObject(t, eventUID, `"etag-noncanonical"`, "Recovered delete")
	remoteObject.Path = noncanonicalPath
	client := &fakeCalDAVUIDQueryPushClient{
		fakeCalDAVPushClient: &fakeCalDAVPushClient{
			getErrors: map[string]error{canonicalPath: errCalDAVObjectNotFound},
		},
		queryObjects: []calDAVCalendarObject{remoteObject},
	}

	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(client.queryCalls) != 1 || client.queryCalls[0].CalendarPath != account.DefaultCalendarURL || client.queryCalls[0].EventUID != eventUID {
		t.Errorf("UID query calls=%+v", client.queryCalls)
	}
	if len(client.deleteCalls) != 1 || client.deleteCalls[0].Path != noncanonicalPath || client.deleteCalls[0].IfMatch != remoteObject.ETag {
		t.Errorf("DELETE calls=%+v", client.deleteCalls)
	}
	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(remainingRows) != 0 {
		t.Fatalf("remaining rows=%+v error=%v", remainingRows, errorValue)
	}
	fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
	if _, found := fencedUIDs[eventUID]; found {
		t.Fatal("completed DELETE left PUT observation fence")
	}
}

func TestPushCalendarOutboxHandlesUIDQueryRecoveryOutcomes(t *testing.T) {
	testCases := []struct {
		name               string
		queryObjects       func(t *testing.T, eventUID string, calendarURL string) []calDAVCalendarObject
		queryError         error
		wantRemainingRows  int
		wantFence          bool
		wantErrorSubstring string
	}{
		{
			name:              "successful empty query completes delete",
			wantRemainingRows: 0,
		},
		{
			name: "partial text match is authoritative absence",
			queryObjects: func(t *testing.T, eventUID string, calendarURL string) []calDAVCalendarObject {
				object := fakeRemoteObject(t, eventUID+"-partial", `"etag-partial"`, "Partial")
				object.Path = calendarURL + "partial-match.ics"
				return []calDAVCalendarObject{object}
			},
			wantRemainingRows: 0,
		},
		{
			name:               "transient query failure stays retryable",
			queryError:         errors.New("temporary REPORT failure"),
			wantRemainingRows:  1,
			wantFence:          true,
			wantErrorSubstring: "temporary REPORT failure",
		},
		{
			name: "duplicate exact matches stay retryable",
			queryObjects: func(t *testing.T, eventUID string, calendarURL string) []calDAVCalendarObject {
				first := fakeRemoteObject(t, eventUID, `"etag-first"`, "First")
				first.Path = calendarURL + "first.ics"
				second := fakeRemoteObject(t, eventUID, `"etag-second"`, "Second")
				second.Path = calendarURL + "second.ics"
				return []calDAVCalendarObject{first, second}
			},
			wantRemainingRows:  1,
			wantFence:          true,
			wantErrorSubstring: "multiple",
		},
		{
			name: "query result parse failure stays retryable",
			queryObjects: func(t *testing.T, eventUID string, calendarURL string) []calDAVCalendarObject {
				return []calDAVCalendarObject{{
					Path: calendarURL + "invalid.ics",
					ETag: `"etag-invalid"`,
					Data: []byte("not an iCalendar object"),
				}}
			},
			wantRemainingRows:  1,
			wantFence:          true,
			wantErrorSubstring: "decode",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newCalendarTestService(t)
			ctx := context.Background()
			account := seedAccountWithDiscovery(t, service)
			eventUID := "uid-query-outcome@internkim"
			if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
				AccountID: account.ID,
				EventID:   "uid-query-outcome",
				EventUID:  eventUID,
				Operation: calendarOutboxOperationDelete,
			}); errorValue != nil {
				t.Fatal(errorValue)
			}
			seedCalendarPushObservationFenceForTest(t, service, account.ID, account.DefaultCalendarURL, eventUID)
			canonicalPath := account.DefaultCalendarURL + eventUID + ".ics"
			client := &fakeCalDAVUIDQueryPushClient{
				fakeCalDAVPushClient: &fakeCalDAVPushClient{
					getErrors: map[string]error{canonicalPath: errCalDAVObjectNotFound},
				},
				queryError: testCase.queryError,
			}
			if testCase.queryObjects != nil {
				client.queryObjects = testCase.queryObjects(t, eventUID, account.DefaultCalendarURL)
			}

			if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
				t.Fatal(errorValue)
			}
			if len(client.queryCalls) != 1 {
				t.Fatalf("UID query calls=%+v", client.queryCalls)
			}
			if len(client.deleteCalls) != 0 {
				t.Fatalf("DELETE calls=%+v want none", client.deleteCalls)
			}
			remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
			if errorValue != nil || len(remainingRows) != testCase.wantRemainingRows {
				t.Fatalf("remaining rows=%+v error=%v", remainingRows, errorValue)
			}
			if testCase.wantErrorSubstring != "" {
				if remainingRows[0].AttemptCount != 1 || !strings.Contains(remainingRows[0].LastError, testCase.wantErrorSubstring) {
					t.Fatalf("retryable row=%+v", remainingRows[0])
				}
			}
			fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
			_, hasFence := fencedUIDs[eventUID]
			if hasFence != testCase.wantFence {
				t.Fatalf("fence present=%v want %v", hasFence, testCase.wantFence)
			}
		})
	}
}

func TestPushCalendarOutboxPreservesMissingDeleteWhenUIDQueryIsUnsupported(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	eventUID := "unsupported-uid-query@internkim"
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID: account.ID,
		EventID:   "unsupported-uid-query",
		EventUID:  eventUID,
		Operation: calendarOutboxOperationDelete,
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	seedCalendarPushObservationFenceForTest(t, service, account.ID, account.DefaultCalendarURL, eventUID)
	canonicalPath := account.DefaultCalendarURL + eventUID + ".ics"
	client := &fakeCalDAVPushClient{getErrors: map[string]error{canonicalPath: errCalDAVObjectNotFound}}

	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(remainingRows) != 1 {
		t.Fatalf("remaining rows=%+v error=%v", remainingRows, errorValue)
	}
	if remainingRows[0].AttemptCount != 1 || !strings.Contains(remainingRows[0].LastError, "UID query") {
		t.Fatalf("retryable row=%+v", remainingRows[0])
	}
	fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
	if _, found := fencedUIDs[eventUID]; !found {
		t.Fatal("unsupported UID query cleared PUT observation fence")
	}
}

func TestPushCalendarOutboxRecoversDeleteFromSelectedCalendarURL(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account.SelectedCalendarID = "company@example.com"
	account.SelectedCalendarAccessRole = "writer"
	account.SelectedCalendarURL = "/calendars/company/"
	account.InitialSyncCompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, account)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	eventUID := "selected-calendar-delete@internkim"
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID: account.ID,
		EventID:   "selected-calendar-delete",
		EventUID:  eventUID,
		Operation: calendarOutboxOperationDelete,
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedPath := account.SelectedCalendarURL + eventUID + ".ics"
	selectedObject := fakeRemoteObject(t, eventUID, `"etag-selected"`, "Selected delete")
	selectedObject.Path = selectedPath
	client := &fakeCalDAVPushClient{getObjects: map[string]calDAVCalendarObject{
		selectedPath: selectedObject,
	}}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(client.getCalls) != 1 || client.getCalls[0] != selectedPath {
		t.Fatalf("GET calls=%v want [%s]", client.getCalls, selectedPath)
	}
	if len(client.deleteCalls) != 1 || client.deleteCalls[0].Path != selectedPath {
		t.Fatalf("DELETE calls=%+v want selected path", client.deleteCalls)
	}
}
