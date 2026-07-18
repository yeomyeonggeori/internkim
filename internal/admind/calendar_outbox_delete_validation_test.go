package admind

import (
	"context"
	"strings"
	"testing"
)

func TestPushCalendarOutboxRejectsCanonicalDeleteObjectWithDifferentUID(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	eventUID := "canonical-uid-check@internkim"
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID: account.ID,
		EventID:   "canonical-uid-check",
		EventUID:  eventUID,
		Operation: calendarOutboxOperationDelete,
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	seedCalendarPushObservationFenceForTest(t, service, account.ID, account.DefaultCalendarURL, eventUID)
	canonicalPath := account.DefaultCalendarURL + eventUID + ".ics"
	wrongObject := fakeRemoteObject(t, "different-uid@internkim", `"etag-wrong"`, "Wrong object")
	wrongObject.Path = canonicalPath
	client := &fakeCalDAVPushClient{getObjects: map[string]calDAVCalendarObject{canonicalPath: wrongObject}}

	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(client.deleteCalls) != 0 {
		t.Fatalf("DELETE calls=%+v want none", client.deleteCalls)
	}
	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(remainingRows) != 1 {
		t.Fatalf("remaining rows=%+v error=%v", remainingRows, errorValue)
	}
	if remainingRows[0].AttemptCount != 1 || !strings.Contains(remainingRows[0].LastError, "UID") {
		t.Fatalf("retryable row=%+v", remainingRows[0])
	}
	fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
	if _, found := fencedUIDs[eventUID]; !found {
		t.Fatal("UID mismatch cleared PUT observation fence")
	}
}

func TestPushCalendarOutboxRejectsRecoveredDeleteObjectWithoutStrongETag(t *testing.T) {
	recoveryCases := []struct {
		name        string
		isUIDQuery  bool
		buildClient func(canonicalPath string, remoteObject calDAVCalendarObject) (calDAVPushClient, *fakeCalDAVPushClient)
	}{
		{
			name: "canonical GET",
			buildClient: func(canonicalPath string, remoteObject calDAVCalendarObject) (calDAVPushClient, *fakeCalDAVPushClient) {
				baseClient := &fakeCalDAVPushClient{getObjects: map[string]calDAVCalendarObject{canonicalPath: remoteObject}}
				return baseClient, baseClient
			},
		},
		{
			name:       "UID REPORT",
			isUIDQuery: true,
			buildClient: func(canonicalPath string, remoteObject calDAVCalendarObject) (calDAVPushClient, *fakeCalDAVPushClient) {
				baseClient := &fakeCalDAVPushClient{getErrors: map[string]error{canonicalPath: errCalDAVObjectNotFound}}
				return &fakeCalDAVUIDQueryPushClient{
					fakeCalDAVPushClient: baseClient,
					queryObjects:         []calDAVCalendarObject{remoteObject},
				}, baseClient
			},
		},
	}
	etagCases := []struct {
		name string
		etag string
	}{
		{name: "empty", etag: ""},
		{name: "wildcard", etag: "*"},
		{name: "weak", etag: `W/"etag-weak"`},
	}
	for _, recoveryCase := range recoveryCases {
		for _, etagCase := range etagCases {
			t.Run(recoveryCase.name+"/"+etagCase.name, func(t *testing.T) {
				service := newCalendarTestService(t)
				ctx := context.Background()
				account := seedAccountWithDiscovery(t, service)
				eventUID := "invalid-delete-etag@internkim"
				if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
					AccountID: account.ID,
					EventID:   "invalid-delete-etag",
					EventUID:  eventUID,
					Operation: calendarOutboxOperationDelete,
				}); errorValue != nil {
					t.Fatal(errorValue)
				}
				seedCalendarPushObservationFenceForTest(t, service, account.ID, account.DefaultCalendarURL, eventUID)
				canonicalPath := account.DefaultCalendarURL + eventUID + ".ics"
				remoteObject := fakeRemoteObject(t, eventUID, etagCase.etag, "Invalid delete ETag")
				remoteObject.Path = canonicalPath
				if recoveryCase.isUIDQuery {
					remoteObject.Path = account.DefaultCalendarURL + "noncanonical-invalid-etag.ics"
				}
				client, callRecorder := recoveryCase.buildClient(canonicalPath, remoteObject)

				if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
					t.Fatal(errorValue)
				}
				if len(callRecorder.deleteCalls) != 0 {
					t.Errorf("DELETE calls=%+v want none", callRecorder.deleteCalls)
				}
				remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
				if errorValue != nil || len(remainingRows) != 1 {
					t.Fatalf("remaining rows=%+v error=%v", remainingRows, errorValue)
				}
				if remainingRows[0].AttemptCount != 1 || !strings.Contains(remainingRows[0].LastError, "strong ETag") {
					t.Fatalf("retryable row=%+v", remainingRows[0])
				}
				fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
				if _, found := fencedUIDs[eventUID]; !found {
					t.Fatal("invalid recovered ETag cleared PUT observation fence")
				}
			})
		}
	}
}

func TestCalDAVCalendarObjectMatchesUIDValidatesAllVEVENTUIDs(t *testing.T) {
	testCases := []struct {
		name      string
		eventUIDs []string
		wantMatch bool
		wantError bool
	}{
		{
			name:      "mixed UIDs are invalid",
			eventUIDs: []string{"target@internkim", "different@internkim"},
			wantError: true,
		},
		{
			name:      "repeated target UID matches",
			eventUIDs: []string{"target@internkim", "target@internkim"},
			wantMatch: true,
		},
		{
			name:      "repeated non-target UID does not match",
			eventUIDs: []string{"different@internkim", "different@internkim"},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			object := calendarObjectWithEventUIDs(testCase.eventUIDs...)
			matches, errorValue := calDAVCalendarObjectMatchesUID(object, "target@internkim")
			if testCase.wantError {
				if errorValue == nil || !strings.Contains(errorValue.Error(), "mixed VEVENT UIDs") {
					t.Fatalf("matches=%v error=%v", matches, errorValue)
				}
				return
			}
			if errorValue != nil || matches != testCase.wantMatch {
				t.Fatalf("matches=%v error=%v want match=%v", matches, errorValue, testCase.wantMatch)
			}
		})
	}
}
