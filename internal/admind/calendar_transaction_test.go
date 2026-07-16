package admind

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCalendarWriteTransactionReservesWriterAtBegin(t *testing.T) {
	ctx := context.Background()
	service := NewService(Configuration{CalendarDatabasePath: filepath.Join(t.TempDir(), "calendar.sqlite")})
	firstDatabase, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer firstDatabase.Close()
	secondDatabase, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer secondDatabase.Close()
	firstTransaction, errorValue := firstDatabase.BeginTx(ctx, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer firstTransaction.Rollback()
	var eventCount int
	if errorValue := firstTransaction.QueryRowContext(ctx, "SELECT COUNT(*) FROM calendar_events").Scan(&eventCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := secondDatabase.ExecContext(ctx, "PRAGMA busy_timeout=100"); errorValue != nil {
		t.Fatal(errorValue)
	}
	blockedContext, cancelBlocked := context.WithTimeout(ctx, 500*time.Millisecond)
	blockedAt := time.Now()
	secondTransaction, errorValue := secondDatabase.BeginTx(blockedContext, nil)
	blockedFor := time.Since(blockedAt)
	cancelBlocked()
	if secondTransaction != nil {
		_ = secondTransaction.Rollback()
		t.Fatal("second BeginTx succeeded before first commit")
	}
	if errorValue == nil {
		t.Fatal("second BeginTx returned no transaction and no error")
	}
	if blockedFor < 75*time.Millisecond {
		t.Fatalf("second BeginTx blocked for %s, want at least 75ms", blockedFor)
	}
	if _, errorValue := firstTransaction.ExecContext(ctx, `
INSERT INTO calendar_settings (key, value)
VALUES ('transaction-reservation', 'held')
ON CONFLICT(key) DO UPDATE SET value = excluded.value`); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := firstTransaction.Commit(); errorValue != nil {
		t.Fatal(errorValue)
	}
	successContext, cancelSuccess := context.WithTimeout(ctx, 2*time.Second)
	defer cancelSuccess()
	secondTransaction, errorValue = secondDatabase.BeginTx(successContext, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := secondTransaction.Rollback(); errorValue != nil {
		t.Fatal(errorValue)
	}
}
