package admind

import (
	"context"
	"testing"
	"time"
)

func waitForCalendarNotificationReconciliation(t *testing.T, service *Service, eventID string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		service.calendarNotificationMutex.Lock()
		_, found := service.calendarNotificationStates[eventID]
		service.calendarNotificationMutex.Unlock()
		if !found {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("calendar notification reconciliation did not finish for %q", eventID)
}

func readCalendarNotificationTime(t *testing.T, service *Service, eventID string) string {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var notifyAt string
	if errorValue := database.QueryRowContext(context.Background(), "SELECT notify_at FROM calendar_event_notifications WHERE event_id = ?", eventID).Scan(&notifyAt); errorValue != nil {
		t.Fatal(errorValue)
	}
	return notifyAt
}

func readCalendarNotificationStatus(t *testing.T, service *Service, eventID string) string {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var status string
	if errorValue := database.QueryRowContext(context.Background(), "SELECT status FROM calendar_event_notifications WHERE event_id = ?", eventID).Scan(&status); errorValue != nil {
		t.Fatal(errorValue)
	}
	return status
}

func markCalendarNotificationSent(t *testing.T, service *Service, eventID string) {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(context.Background(), `
UPDATE calendar_event_notifications
SET status = 'sent', sent_at = ?, updated_at = ?
WHERE event_id = ?`, time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), eventID); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func enqueueCalendarNotificationReconciliationForTest(service *Service, eventID string, reconciler calendarNotificationReconciler) {
	service.enqueueCalendarNotificationReconciliationWithContext(context.Background(), eventID, reconciler, true)
}
