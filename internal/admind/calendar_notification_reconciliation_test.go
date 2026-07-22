package admind

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestFinishCalendarEventPersistenceUsesCurrentEventForNotifications(t *testing.T) {
	service := newCalendarTestService(t)
	olderEvent := calendarTestEvent("notification-current-version", "Older", "")
	olderEvent.StartISO = nextDayCalendarHour(10).Format(time.RFC3339)
	olderEvent.EndISO = nextDayCalendarHour(11).Format(time.RFC3339)
	olderEvent.ReminderLeadHours = 1
	if errorValue := service.writeCalendarEvent(context.Background(), olderEvent); errorValue != nil {
		t.Fatal(errorValue)
	}

	newerEvent := olderEvent
	newerEvent.Title = "Newer"
	newerEvent.StartISO = nextDayCalendarHour(14).Format(time.RFC3339)
	newerEvent.EndISO = nextDayCalendarHour(15).Format(time.RFC3339)
	if errorValue := service.writeCalendarEvent(context.Background(), newerEvent); errorValue != nil {
		t.Fatal(errorValue)
	}

	service.finishCalendarEventPersistence(context.Background(), olderEvent)
	waitForCalendarNotificationReconciliation(t, service, olderEvent.ID)

	expectedNotifyAt := expectedCalendarNotifyAt(t, newerEvent)
	if notifyAt := readCalendarNotificationTime(t, service, newerEvent.ID); notifyAt != expectedNotifyAt {
		t.Fatalf("notify_at = %q, expected %q", notifyAt, expectedNotifyAt)
	}
}

func expectedCalendarNotifyAt(t *testing.T, event calendarEvent) string {
	t.Helper()
	startTime, errorValue := time.Parse(time.RFC3339, event.StartISO)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	leadDuration := time.Duration(normalizeCalendarReminderLeadHours(event.ReminderLeadHours)) * time.Hour
	return calendarNotificationDeliveryTime(startTime.Add(-leadDuration), event.TimeZone).Format(time.RFC3339)
}

func nextDayCalendarHour(hour int) time.Time {
	nextDay := time.Now().UTC().AddDate(0, 0, 1)
	return time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), hour, 0, 0, 0, time.UTC)
}

func TestCalendarNotificationReschedulesSentEventAfterFutureMove(t *testing.T) {
	service := newCalendarTestService(t)
	event := calendarTestEvent("notification-rescheduled-sent", "Original", "")
	event.StartISO = nextDayCalendarHour(10).Format(time.RFC3339)
	event.EndISO = nextDayCalendarHour(11).Format(time.RFC3339)
	event.ReminderLeadHours = 1
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	waitForCalendarNotificationReconciliation(t, service, event.ID)
	markCalendarNotificationSent(t, service, event.ID)

	event.Title = "Title only"
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	waitForCalendarNotificationReconciliation(t, service, event.ID)
	if status := readCalendarNotificationStatus(t, service, event.ID); status != "sent" {
		t.Fatalf("status after title update = %q, expected sent", status)
	}

	event.StartISO = nextDayCalendarHour(14).Format(time.RFC3339)
	event.EndISO = nextDayCalendarHour(15).Format(time.RFC3339)
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	waitForCalendarNotificationReconciliation(t, service, event.ID)
	if status := readCalendarNotificationStatus(t, service, event.ID); status != "pending" {
		t.Fatalf("status after future move = %q, expected pending", status)
	}
	expectedNotifyAt := expectedCalendarNotifyAt(t, event)
	if notifyAt := readCalendarNotificationTime(t, service, event.ID); notifyAt != expectedNotifyAt {
		t.Fatalf("notify_at = %q, expected %q", notifyAt, expectedNotifyAt)
	}
}

func TestCancelStaleCalendarNotificationsReleasesReadConnectionBeforeUpdate(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	eventID := "notification-stale-target"
	if _, errorValue := database.ExecContext(context.Background(), `
INSERT INTO calendar_event_notifications (
	event_id, recipient_key, target_type, target_value, target_label, notify_at, status, sent_at, error, updated_at
) VALUES (?, ?, 'dm', 'user-1', 'User 1', ?, 'pending', '', '', ?)`,
		eventID,
		"dm:user-1",
		time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
		time.Now().UTC().Format(time.RFC3339Nano),
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if errorValue := service.cancelStaleCalendarNotifications(ctx, database, eventID, map[string]bool{}, time.Now().UTC().Format(time.RFC3339Nano)); errorValue != nil {
		t.Fatal(errorValue)
	}
	if status := readCalendarNotificationStatus(t, service, eventID); status != "canceled" {
		t.Fatalf("status = %q, expected canceled", status)
	}
}

func TestReconcileCalendarNotificationsPreservesRecreatedEvent(t *testing.T) {
	service := newCalendarTestService(t)
	event := calendarTestEvent("notification-recreated-event", "Original", "")
	event.StartISO = time.Now().UTC().Add(4 * time.Hour).Truncate(time.Second).Format(time.RFC3339)
	event.EndISO = time.Now().UTC().Add(5 * time.Hour).Truncate(time.Second).Format(time.RFC3339)
	event.ReminderLeadHours = 1
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(context.Background(), event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}

	recreatedEvent := event
	recreatedEvent.Title = "Recreated"
	recreatedEvent.StartISO = time.Now().UTC().Add(8 * time.Hour).Truncate(time.Second).Format(time.RFC3339)
	recreatedEvent.EndISO = time.Now().UTC().Add(9 * time.Hour).Truncate(time.Second).Format(time.RFC3339)
	if errorValue := service.writeCalendarEvent(context.Background(), recreatedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}

	service.reconcileCalendarEventNotifications(context.Background(), event.ID)
	waitForCalendarNotificationReconciliation(t, service, event.ID)

	if status := readCalendarNotificationStatus(t, service, event.ID); status != "pending" {
		t.Fatalf("status = %q, expected pending", status)
	}
}

func TestReconcileCalendarNotificationsReleasesStoreLockDuringTargetResolution(t *testing.T) {
	service := newCalendarTestService(t)
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	var requestStartedOnce sync.Once
	var releaseRequestOnce sync.Once
	release := func() { releaseRequestOnce.Do(func() { close(releaseRequest) }) }
	defer release()
	service.Configuration.MattermostAdminPasswordPath = writeTestFile(t, "admin-password")
	service.Configuration.MattermostBaseURL = "http://mattermost.local"
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/v4/users" {
			requestStartedOnce.Do(func() { close(requestStarted) })
			<-releaseRequest
			return jsonResponse(http.StatusOK, `[]`, nil), nil
		}
		if request.URL.Path == "/api/v4/users/login" {
			return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), nil
		}
		return jsonResponse(http.StatusOK, `[]`, nil), nil
	})}
	event := calendarTestEvent("notification-unlocked-target-resolution", "Unlocked", "")
	event.Description = "iam"
	event.StartISO = time.Now().UTC().Add(4 * time.Hour).Truncate(time.Second).Format(time.RFC3339)
	event.EndISO = time.Now().UTC().Add(5 * time.Hour).Truncate(time.Second).Format(time.RFC3339)
	event.ReminderLeadHours = 1
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("calendar notification target resolution did not start")
	}
	lockAcquired := make(chan struct{})
	go func() {
		service.calendarStoreWriteMutex.Lock()
		service.calendarStoreWriteMutex.Unlock()
		close(lockAcquired)
	}()
	select {
	case <-lockAcquired:
	case <-time.After(time.Second):
		t.Fatal("calendar store lock remained held during notification target resolution")
	}
	release()
	waitForCalendarNotificationReconciliation(t, service, event.ID)
}

func TestReconcileCalendarNotificationsProcessesSnapshotOnce(t *testing.T) {
	service := newCalendarTestService(t)
	event := calendarTestEvent("notification-single-snapshot", "Single snapshot", "")
	event.StartISO = time.Now().UTC().Add(4 * time.Hour).Truncate(time.Second).Format(time.RFC3339)
	event.EndISO = time.Now().UTC().Add(5 * time.Hour).Truncate(time.Second).Format(time.RFC3339)
	event.ReminderLeadHours = 1
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	waitForCalendarNotificationReconciliation(t, service, event.ID)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(context.Background(), `
CREATE TABLE notification_reconciliation_count (count INTEGER NOT NULL);
INSERT INTO notification_reconciliation_count (count) VALUES (0);
CREATE TRIGGER count_notification_reconciliation_insert
AFTER INSERT ON calendar_event_notifications
BEGIN
	UPDATE notification_reconciliation_count SET count = count + 1;
	UPDATE calendar_events SET updated_at = updated_at || '-changed' WHERE id = NEW.event_id;
END;
CREATE TRIGGER count_notification_reconciliation_update
AFTER UPDATE ON calendar_event_notifications
BEGIN
	UPDATE notification_reconciliation_count SET count = count + 1;
	UPDATE calendar_events SET updated_at = updated_at || '-changed' WHERE id = NEW.event_id;
END`)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	service.reconcileCalendarEventNotifications(context.Background(), event.ID)
	waitForCalendarNotificationReconciliation(t, service, event.ID)

	database, errorValue = service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var count int
	if errorValue := database.QueryRowContext(context.Background(), "SELECT count FROM notification_reconciliation_count").Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	if count != 1 {
		t.Fatalf("notification reconciliation count = %d, expected 1", count)
	}
}
