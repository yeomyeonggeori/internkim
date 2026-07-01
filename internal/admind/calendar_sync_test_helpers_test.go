package admind

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/emersion/go-ical"
)

func seedRemoteCalendarAccountForPull(t *testing.T, service *Service, email string) remoteCalendarAccount {
	t.Helper()
	account := remoteCalendarAccount{
		ID:           "google-test",
		Provider:     remoteCalendarProviderGoogle,
		AccountEmail: email,
	}
	saved, errorValue := service.upsertRemoteCalendarAccount(context.Background(), account)
	if errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}
	return saved
}

func seedAccountWithDiscovery(t *testing.T, service *Service) remoteCalendarAccount {
	t.Helper()
	account := seedRemoteCalendarAccountForPull(t, service, "user@example.com")
	account.PrincipalURL = "/principal"
	account.HomeSetURL = "/home/"
	account.DefaultCalendarURL = "/calendars/me/"
	saved, errorValue := service.upsertRemoteCalendarAccount(context.Background(), account)
	if errorValue != nil {
		t.Fatalf("seed account discovery: %v", errorValue)
	}
	return saved
}

func fakeRemoteObject(t *testing.T, uid string, etag string, title string) calDAVCalendarObject {
	t.Helper()
	start := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	end := start.Add(time.Hour)
	event := calendarEvent{
		ID:                "remote-" + uid,
		UID:               uid,
		Title:             title,
		StartISO:          start.Format(time.RFC3339),
		EndISO:            end.Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#10b981",
		ReminderLeadHours: 24,
		CreatedByEmail:    "ignored@example.com",
	}
	calendar, errorValue := calendarObjectForEvent(event)
	if errorValue != nil {
		t.Fatalf("calendarObjectForEvent: %v", errorValue)
	}
	var buffer bytes.Buffer
	if errorValue := ical.NewEncoder(&buffer).Encode(calendar); errorValue != nil {
		t.Fatalf("encode: %v", errorValue)
	}
	return calDAVCalendarObject{
		Path: "/calendars/user@example.com/events/" + uid + ".ics",
		ETag: etag,
		Data: buffer.Bytes(),
	}
}

func newLocalTestCalendarEvent(idSuffix string, title string) calendarEvent {
	start := time.Now().UTC().Add(3 * time.Hour).Truncate(time.Second)
	end := start.Add(time.Hour)
	return calendarEvent{
		ID:                idSuffix,
		UID:               idSuffix + "@internkim",
		Title:             title,
		StartISO:          start.Format(time.RFC3339),
		EndISO:            end.Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#2563eb",
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
	}
}

type fakeCalDAVPushClient struct {
	putETags       map[string]string
	putErrors      map[string]error
	putErrorsQueue map[string][]error
	deleteErrors   map[string]error
	getObjects     map[string]calDAVCalendarObject
	getErrors      map[string]error

	putCalls    []fakePutCall
	deleteCalls []fakeDeleteCall
	getCalls    []string
}

type fakePutCall struct {
	Path        string
	IfMatch     string
	IfNoneMatch string
	Data        []byte
}

type fakeDeleteCall struct {
	Path    string
	IfMatch string
}

func (client *fakeCalDAVPushClient) putCalendarObject(ctx context.Context, objectPath string, ics []byte, ifMatch string, ifNoneMatch string) (string, error) {
	client.putCalls = append(client.putCalls, fakePutCall{
		Path:        objectPath,
		IfMatch:     ifMatch,
		IfNoneMatch: ifNoneMatch,
		Data:        append([]byte(nil), ics...),
	})
	if client.putErrorsQueue != nil {
		if queue, present := client.putErrorsQueue[objectPath]; present && len(queue) > 0 {
			next := queue[0]
			client.putErrorsQueue[objectPath] = queue[1:]
			if next != nil {
				return "", next
			}
		}
	}
	if client.putErrors != nil {
		if errorValue, present := client.putErrors[objectPath]; present {
			return "", errorValue
		}
	}
	if client.putETags != nil {
		if etag, present := client.putETags[objectPath]; present {
			return etag, nil
		}
	}
	return "", nil
}

func (client *fakeCalDAVPushClient) deleteCalendarObject(ctx context.Context, objectPath string, ifMatch string) error {
	client.deleteCalls = append(client.deleteCalls, fakeDeleteCall{Path: objectPath, IfMatch: ifMatch})
	if client.deleteErrors != nil {
		if errorValue, present := client.deleteErrors[objectPath]; present {
			return errorValue
		}
	}
	return nil
}

func (client *fakeCalDAVPushClient) getCalendarObject(ctx context.Context, objectPath string) (calDAVCalendarObject, error) {
	client.getCalls = append(client.getCalls, objectPath)
	if client.getErrors != nil {
		if errorValue, present := client.getErrors[objectPath]; present {
			return calDAVCalendarObject{}, errorValue
		}
	}
	if client.getObjects != nil {
		if object, present := client.getObjects[objectPath]; present {
			return object, nil
		}
	}
	return calDAVCalendarObject{}, errCalDAVPreconditionFailed
}

type fakeCalDAVPullClient struct {
	principalURL           string
	homeSetURL             string
	calendars              []calDAVCalendarInfo
	objects                []calDAVCalendarObject
	ctag                   string
	discoverPrincipalError error

	principalCalls int
	homeSetCalls   int
	listCalls      int
	ctagCalls      int
	queryCalls     int
	ctagPaths      []string
	queryPaths     []string
}

func (client *fakeCalDAVPullClient) fetchCalendarCTag(ctx context.Context, calendarPath string) (string, error) {
	client.ctagCalls++
	client.ctagPaths = append(client.ctagPaths, calendarPath)
	return client.ctag, nil
}

func (client *fakeCalDAVPullClient) discoverPrincipalURL(ctx context.Context) (string, error) {
	client.principalCalls++
	if client.discoverPrincipalError != nil {
		return "", client.discoverPrincipalError
	}
	return client.principalURL, nil
}

func (client *fakeCalDAVPullClient) discoverHomeSetURL(ctx context.Context, principalURL string) (string, error) {
	client.homeSetCalls++
	return client.homeSetURL, nil
}

func (client *fakeCalDAVPullClient) listCalendars(ctx context.Context, homeSetURL string) ([]calDAVCalendarInfo, error) {
	client.listCalls++
	return client.calendars, nil
}

func (client *fakeCalDAVPullClient) queryAllCalendarEvents(ctx context.Context, calendarPath string) ([]calDAVCalendarObject, error) {
	client.queryCalls++
	client.queryPaths = append(client.queryPaths, calendarPath)
	return client.objects, nil
}
