package admind

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPushCalendarOutboxRetriesBlockedRowAfterRecoveryDelay(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("blocked-retry", "Retry after block")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("rows=%d error=%v", len(rows), errorValue)
	}
	for index := 0; index < calendarOutboxMaxAttempts; index++ {
		if errorValue := service.markCalendarOutboxBatchAttempt(ctx, rows[0], "simulated failure"); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, &fakeCalDAVPushClient{}); errorValue != nil {
		t.Fatal(errorValue)
	}
	blockedRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(blockedRows) != 1 || blockedRows[0].Status != calendarOutboxStatusBlocked {
		t.Fatalf("blocked rows=%+v error=%v", blockedRows, errorValue)
	}

	expectedPath := account.DefaultCalendarURL + event.UID + ".ics"
	client := &fakeCalDAVPushClient{putETags: map[string]string{expectedPath: `"etag-recovered"`}}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(client.putCalls) != 0 {
		t.Fatalf("immediate retry put calls=%d want 0", len(client.putCalls))
	}
	blockedAt := time.Now().UTC().Add(-time.Minute - time.Second).Format(time.RFC3339Nano)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_outbox SET last_attempted_at = ?, failed_at = ? WHERE account_id = ?`, blockedAt, blockedAt, account.ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	failingClient := &fakeCalDAVPushClient{putErrors: map[string]error{expectedPath: errCalDAVPreconditionFailed}}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, failingClient); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, failingClient); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(failingClient.putCalls) != 1 {
		t.Fatalf("repeated failed retry put calls=%d want 1", len(failingClient.putCalls))
	}
	database, errorValue = service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_outbox SET last_attempted_at = ?, failed_at = ? WHERE account_id = ?`, blockedAt, blockedAt, account.ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(client.putCalls) != 1 {
		t.Fatalf("recovered retry put calls=%d want 1", len(client.putCalls))
	}
	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(remainingRows) != 0 {
		t.Fatalf("remaining rows=%d want 0", len(remainingRows))
	}
}

func TestBlockedCalendarOutboxFutureFailureTimeUsesCurrentRetryWindow(t *testing.T) {
	currentTime := time.Date(2026, 7, 15, 5, 0, 0, 0, time.UTC)
	row := calendarOutboxRow{
		Status:   calendarOutboxStatusBlocked,
		FailedAt: currentTime.Add(24 * time.Hour).Format(time.RFC3339Nano),
	}
	if !canRetryBlockedCalendarOutbox(row, currentTime) {
		t.Fatal("future failure time should not suspend retries")
	}
	row.LastAttemptedAt = currentTime.Format(time.RFC3339Nano)
	if canRetryBlockedCalendarOutbox(row, currentTime) {
		t.Fatal("current attempt time should start the retry delay")
	}
	if !canRetryBlockedCalendarOutbox(row, currentTime.Add(calendarOutboxBlockedRetryDelay)) {
		t.Fatal("current attempt time should retry after the retry window")
	}
}

func TestPushCalendarOutboxReturnsAttemptPersistenceFailure(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("attempt-persistence-failure", "Attempt failure")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `CREATE TRIGGER fail_calendar_outbox_attempt BEFORE UPDATE ON calendar_outbox BEGIN SELECT RAISE(ABORT, 'attempt persistence blocked'); END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedPath := account.DefaultCalendarURL + event.UID + ".ics"
	client := &fakeCalDAVPushClient{putErrors: map[string]error{expectedPath: errors.New("remote write failed")}}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue == nil {
		t.Fatal("expected attempt persistence failure")
	}
}

func TestPushCalendarOutboxReturnsCleanupPersistenceFailure(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("cleanup-persistence-failure", "Cleanup failure")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `CREATE TRIGGER fail_calendar_outbox_cleanup BEFORE DELETE ON calendar_outbox BEGIN SELECT RAISE(ABORT, 'cleanup persistence blocked'); END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedPath := account.DefaultCalendarURL + event.UID + ".ics"
	client := &fakeCalDAVPushClient{putETags: map[string]string{expectedPath: `"etag-written"`}}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue == nil {
		t.Fatal("expected cleanup persistence failure")
	}
}
