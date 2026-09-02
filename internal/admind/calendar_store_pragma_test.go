package admind

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCalendarOpenDatabaseEnablesWAL(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatalf("open: %v", errorValue)
	}
	defer database.Close()
	var journalMode string
	if errorValue := database.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); errorValue != nil {
		t.Fatalf("query journal_mode: %v", errorValue)
	}
	if !strings.EqualFold(journalMode, "wal") {
		t.Errorf("journal_mode: got %q, want %q", journalMode, "wal")
	}
	var busyTimeout int
	if errorValue := database.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); errorValue != nil {
		t.Fatalf("query busy_timeout: %v", errorValue)
	}
	if busyTimeout < 5000 {
		t.Errorf("busy_timeout: got %d, want >= 5000", busyTimeout)
	}
}

func TestCalendarConcurrentWritesDoNotReturnSQLITEBusy(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	const writerCount = 8
	var waitGroup sync.WaitGroup
	errorChannel := make(chan error, writerCount)
	for index := 0; index < writerCount; index++ {
		waitGroup.Add(1)
		go func(eventIndex int) {
			defer waitGroup.Done()
			start := time.Now().UTC().Add(time.Duration(eventIndex) * time.Hour)
			end := start.Add(30 * time.Minute)
			event := calendarEvent{
				ID:                fmt.Sprintf("concurrent-%d", eventIndex),
				UID:               fmt.Sprintf("concurrent-%d@internkim", eventIndex),
				Title:             fmt.Sprintf("Concurrent %d", eventIndex),
				StartISO:          start.Format(time.RFC3339),
				EndISO:            end.Format(time.RFC3339),
				TimeZone:          "UTC",
				Color:             "#10b981",
				ReminderLeadHours: 24,
				CreatedByEmail:    "admin@example.com",
			}
			if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
				errorChannel <- errorValue
			}
		}(index)
	}
	waitGroup.Wait()
	close(errorChannel)
	for errorValue := range errorChannel {
		if strings.Contains(strings.ToLower(errorValue.Error()), "sqlite_busy") || strings.Contains(strings.ToLower(errorValue.Error()), "database is locked") {
			t.Errorf("concurrent write hit SQLITE_BUSY: %v", errorValue)
			continue
		}
		t.Errorf("concurrent write failed: %v", errorValue)
	}
	stored, errorValue := service.readCalendarEvents(ctx, time.Time{}, time.Time{})
	if errorValue != nil {
		t.Fatalf("read: %v", errorValue)
	}
	count := 0
	for _, event := range stored {
		if strings.HasPrefix(event.ID, "concurrent-") {
			count++
		}
	}
	if count != writerCount {
		t.Errorf("stored concurrent events: got %d, want %d", count, writerCount)
	}
}
