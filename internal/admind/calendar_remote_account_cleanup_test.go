package admind

import (
	"context"
	"strings"
	"testing"
	"time"
)

type calendarRemoteAccountCleanupFixture struct {
	account     remoteCalendarAccount
	calendarURL string
	event       calendarEvent
}

func TestDeleteRemoteCalendarAccountRemovesOnlyAccountScopedSyncRows(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	targetFixture := seedCalendarRemoteAccountCleanupFixture(t, service, "cleanup-target", "target@example.com")
	otherFixture := seedCalendarRemoteAccountCleanupFixture(t, service, "cleanup-other", "other@example.com")

	if errorValue := service.deleteRemoteCalendarAccount(contextValue, targetFixture.account.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertCalendarRemoteAccountCleanupRowCounts(t, service, targetFixture, 0)
	assertCalendarRemoteAccountCleanupRowCounts(t, service, otherFixture, 1)
	for _, fixture := range []calendarRemoteAccountCleanupFixture{targetFixture, otherFixture} {
		if _, found, errorValue := service.readCalendarEventByUID(contextValue, fixture.event.UID); errorValue != nil || !found {
			t.Fatalf("calendar event %s found=%v error=%v", fixture.event.UID, found, errorValue)
		}
		assertCalendarCleanupPreservedRowCount(t, service, `SELECT COUNT(*) FROM calendar_conflicts WHERE event_uid = ?`, fixture.event.UID)
		assertCalendarCleanupPreservedRowCount(t, service, `SELECT COUNT(*) FROM calendar_event_logical_clocks WHERE event_uid = ?`, fixture.event.UID)
	}
}

func TestDeleteRemoteCalendarAccountRollsBackEveryCleanupWhenAccountDeleteFails(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	targetFixture := seedCalendarRemoteAccountCleanupFixture(t, service, "cleanup-rollback", "rollback@example.com")
	otherFixture := seedCalendarRemoteAccountCleanupFixture(t, service, "cleanup-rollback-other", "rollback-other@example.com")
	database, errorValue := service.openCalendarDatabase(contextValue)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(contextValue, `
CREATE TRIGGER fail_calendar_remote_account_cleanup
BEFORE DELETE ON calendar_remote_accounts
WHEN OLD.id = 'cleanup-rollback'
BEGIN
	SELECT RAISE(ABORT, 'forced calendar account cleanup failure');
END`)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	errorValue = service.deleteRemoteCalendarAccount(contextValue, targetFixture.account.ID)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "forced calendar account cleanup failure") {
		t.Fatalf("delete error=%v", errorValue)
	}
	assertCalendarRemoteAccountCleanupRowCounts(t, service, targetFixture, 1)
	assertCalendarRemoteAccountCleanupRowCounts(t, service, otherFixture, 1)
}

func TestDeleteRemoteCalendarAccountWaitsForRemoteMutation(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	fixture := seedCalendarRemoteAccountCleanupFixture(t, service, "cleanup-remote-lock", "remote-lock@example.com")
	service.calendarRemoteMutex.Lock()
	started := make(chan struct{})
	deleteResult := make(chan error, 1)
	go func() {
		close(started)
		deleteResult <- service.deleteRemoteCalendarAccount(contextValue, fixture.account.ID)
	}()
	<-started
	completedEarly := false
	select {
	case errorValue := <-deleteResult:
		completedEarly = true
		if errorValue != nil {
			service.calendarRemoteMutex.Unlock()
			t.Fatal(errorValue)
		}
	case <-time.After(100 * time.Millisecond):
	}
	service.calendarRemoteMutex.Unlock()
	if completedEarly {
		t.Fatal("account cleanup completed during an in-flight remote mutation")
	}
	if errorValue := <-deleteResult; errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestDeleteRemoteCalendarAccountWaitsForCalendarStoreWrite(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	fixture := seedCalendarRemoteAccountCleanupFixture(t, service, "cleanup-store-lock", "store-lock@example.com")
	service.calendarStoreWriteMutex.Lock()
	started := make(chan struct{})
	deleteResult := make(chan error, 1)
	go func() {
		close(started)
		deleteResult <- service.deleteRemoteCalendarAccount(contextValue, fixture.account.ID)
	}()
	<-started
	completedEarly := false
	select {
	case errorValue := <-deleteResult:
		completedEarly = true
		if errorValue != nil {
			service.calendarStoreWriteMutex.Unlock()
			t.Fatal(errorValue)
		}
	case <-time.After(100 * time.Millisecond):
	}
	service.calendarStoreWriteMutex.Unlock()
	if completedEarly {
		t.Fatal("account cleanup completed during an in-flight calendar store write")
	}
	if errorValue := <-deleteResult; errorValue != nil {
		t.Fatal(errorValue)
	}
}

func seedCalendarRemoteAccountCleanupFixture(t *testing.T, service *Service, accountID string, accountEmail string) calendarRemoteAccountCleanupFixture {
	t.Helper()
	contextValue := context.Background()
	calendarURL := "/calendars/" + accountID + "/"
	account, errorValue := service.upsertRemoteCalendarAccount(contextValue, remoteCalendarAccount{
		ID:                 accountID,
		Provider:           remoteCalendarProviderGoogle,
		AccountEmail:       accountEmail,
		DefaultCalendarURL: calendarURL,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event := newLocalTestCalendarEvent(accountID, accountID)
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-` + accountID + `"`
	event.RemoteHref = calendarURL + accountID + ".ics"
	if errorValue := service.writeCalendarEventWithSource(contextValue, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(contextValue)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	statements := []struct {
		query     string
		arguments []any
	}{
		{
			query:     `INSERT INTO calendar_outbox(account_id, event_id, event_uid, operation, target_calendar_url, created_at) VALUES(?, ?, ?, ?, ?, ?)`,
			arguments: []any{account.ID, event.ID, event.UID, calendarOutboxOperationPut, calendarURL, now},
		},
		{
			query:     `INSERT INTO calendar_sync_state(account_id, calendar_url, last_ctag, last_synced_at) VALUES(?, ?, ?, ?)`,
			arguments: []any{account.ID, calendarURL, "ctag", now},
		},
		{
			query:     `INSERT INTO calendar_remote_event_sync_state(account_id, calendar_url, event_uid, remote_modified_at, last_seen_at, updated_at) VALUES(?, ?, ?, ?, ?, ?)`,
			arguments: []any{account.ID, calendarURL, event.UID, now, now, now},
		},
		{
			query:     `INSERT INTO calendar_target_field_acknowledgements(account_id, calendar_url, event_uid, field, acknowledged_at) VALUES(?, ?, ?, ?, ?)`,
			arguments: []any{account.ID, calendarURL, event.UID, calendarFieldTitle, now},
		},
		{
			query:     `INSERT INTO calendar_push_observation_fences(account_id, calendar_url, event_uid, created_at) VALUES(?, ?, ?, ?)`,
			arguments: []any{account.ID, calendarURL, event.UID, now},
		},
		{
			query:     `INSERT INTO calendar_conflicts(event_id, event_uid, field, local_value, remote_value, detected_at) VALUES(?, ?, ?, ?, ?, ?)`,
			arguments: []any{event.ID, event.UID, calendarFieldTitle, "local", "remote", now},
		},
		{
			query:     `INSERT INTO calendar_event_logical_clocks(event_uid, logical_time_unix_nano) VALUES(?, ?)`,
			arguments: []any{event.UID, time.Now().UTC().UnixNano()},
		},
	}
	for _, statement := range statements {
		if _, errorValue := database.ExecContext(contextValue, statement.query, statement.arguments...); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	return calendarRemoteAccountCleanupFixture{account: account, calendarURL: calendarURL, event: event}
}

func assertCalendarRemoteAccountCleanupRowCounts(t *testing.T, service *Service, fixture calendarRemoteAccountCleanupFixture, expectedCount int) {
	t.Helper()
	checks := []struct {
		query     string
		arguments []any
	}{
		{query: `SELECT COUNT(*) FROM calendar_remote_accounts WHERE id = ?`, arguments: []any{fixture.account.ID}},
		{query: `SELECT COUNT(*) FROM calendar_outbox WHERE account_id = ?`, arguments: []any{fixture.account.ID}},
		{query: `SELECT COUNT(*) FROM calendar_sync_state WHERE account_id = ?`, arguments: []any{fixture.account.ID}},
		{query: `SELECT COUNT(*) FROM calendar_remote_event_sync_state WHERE account_id = ?`, arguments: []any{fixture.account.ID}},
		{query: `SELECT COUNT(*) FROM calendar_target_field_acknowledgements WHERE account_id = ?`, arguments: []any{fixture.account.ID}},
		{query: `SELECT COUNT(*) FROM calendar_push_observation_fences WHERE account_id = ?`, arguments: []any{fixture.account.ID}},
	}
	for _, check := range checks {
		database, errorValue := service.openCalendarDatabase(context.Background())
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		var count int
		errorValue = database.QueryRowContext(context.Background(), check.query, check.arguments...).Scan(&count)
		database.Close()
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if count != expectedCount {
			t.Fatalf("query %q count=%d want %d", check.query, count, expectedCount)
		}
	}
}

func assertCalendarCleanupPreservedRowCount(t *testing.T, service *Service, query string, eventUID string) {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var count int
	if errorValue := database.QueryRowContext(context.Background(), query, eventUID).Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	if count != 1 {
		t.Fatalf("query %q count=%d want 1", query, count)
	}
}
