package admind

import (
	"context"
	"errors"
	"testing"
)

func TestPushCalendarOutboxStopsAfterAccountAuthenticationError(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	firstEvent := newLocalTestCalendarEvent("push-auth-first", "First")
	secondEvent := newLocalTestCalendarEvent("push-auth-second", "Second")
	if errorValue := service.writeCalendarEvent(ctx, firstEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeCalendarEvent(ctx, secondEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	firstPath := account.DefaultCalendarURL + firstEvent.UID + ".ics"
	secondPath := account.DefaultCalendarURL + secondEvent.UID + ".ics"
	authenticationError := errors.New("caldav put status 401: Unauthorized")
	client := &fakeCalDAVPushClient{putErrors: map[string]error{
		firstPath:  authenticationError,
		secondPath: authenticationError,
	}}

	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(client.putCalls) != 1 {
		t.Fatalf("PUT calls=%d want 1", len(client.putCalls))
	}
	rows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(rows) != 2 {
		t.Fatalf("outbox rows=%d want 2", len(rows))
	}
	if rows[0].AttemptCount != 1 {
		t.Fatalf("first attempt count=%d want 1", rows[0].AttemptCount)
	}
	if rows[1].AttemptCount != 0 {
		t.Fatalf("second attempt count=%d want 0", rows[1].AttemptCount)
	}
}
