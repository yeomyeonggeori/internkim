package admind

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFlowWriteTransactionReservesWriterAtBegin(t *testing.T) {
	ctx := context.Background()
	service := NewService(Configuration{FlowDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
	firstDatabase, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer firstDatabase.Close()
	secondDatabase, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer secondDatabase.Close()
	firstTransaction, errorValue := firstDatabase.BeginTx(ctx, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer firstTransaction.Rollback()
	var taskCount int
	if errorValue := firstTransaction.QueryRowContext(ctx, "SELECT COUNT(*) FROM flow_tasks").Scan(&taskCount); errorValue != nil {
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
	if errorValue := incrementFlowSummarySourceRevisions(ctx, firstTransaction, []flowSummarySourceKey{{Kind: flowSummarySourceWeek, Key: "26W28"}}); errorValue != nil {
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

func TestFlowDatabasePreservesQuestionMarkInDatabasePath(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "state?tenant=one", "flow?cache.sqlite")
	service := NewService(Configuration{FlowDatabasePath: databasePath})
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var journalMode string
	var busyTimeout int
	var foreignKeys int
	if errorValue := database.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.EqualFold(journalMode, "wal") || busyTimeout != 5000 || foreignKeys != 1 {
		t.Fatalf("SQLite pragmas = journal_mode %q, busy_timeout %d, foreign_keys %d", journalMode, busyTimeout, foreignKeys)
	}
	if _, errorValue := os.Stat(databasePath); errorValue != nil {
		t.Fatalf("intended database path was not created: %v", errorValue)
	}
	truncatedPath := strings.SplitN(databasePath, "?", 2)[0]
	if _, errorValue := os.Stat(truncatedPath); !os.IsNotExist(errorValue) {
		t.Fatalf("truncated database path exists: %s, error: %v", truncatedPath, errorValue)
	}
}

func TestSQLiteDatabaseDSNConfiguresDriverConnection(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "driver?cache.sqlite")
	database, errorValue := sql.Open("sqlite", sqliteDatabaseDSNWithOptions(databasePath, sqliteDatabaseOptions{transactionLock: "immediate"}))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var journalMode string
	var busyTimeout int
	var foreignKeys int
	if errorValue := database.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.EqualFold(journalMode, "wal") || busyTimeout != 5000 || foreignKeys != 1 {
		t.Fatalf("SQLite pragmas = journal_mode %q, busy_timeout %d, foreign_keys %d", journalMode, busyTimeout, foreignKeys)
	}
	if _, errorValue := os.Stat(databasePath); errorValue != nil {
		t.Fatalf("driver database path was not created: %v", errorValue)
	}
}

func TestOpenSQLiteDatabaseKeepsDeferredTransactions(t *testing.T) {
	ctx := context.Background()
	service := NewService(Configuration{})
	databasePath := filepath.Join(t.TempDir(), "deferred.sqlite")
	firstDatabase, errorValue := service.openSQLiteDatabase(ctx, databasePath, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer firstDatabase.Close()
	secondDatabase, errorValue := service.openSQLiteDatabase(ctx, databasePath, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer secondDatabase.Close()
	firstTransaction, errorValue := firstDatabase.BeginTx(ctx, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer firstTransaction.Rollback()
	var value int
	if errorValue := firstTransaction.QueryRowContext(ctx, "SELECT 1").Scan(&value); errorValue != nil {
		t.Fatal(errorValue)
	}
	secondContext, cancelSecond := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancelSecond()
	secondTransaction, errorValue := secondDatabase.BeginTx(secondContext, nil)
	if errorValue != nil {
		t.Fatalf("deferred second BeginTx failed: %v", errorValue)
	}
	if errorValue := secondTransaction.Rollback(); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestConcurrentFlowTaskWritesPreserveTasksAndRevisions(t *testing.T) {
	ctx := context.Background()
	service := NewService(Configuration{FlowDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	const writerCount = 8
	startChannel := make(chan struct{})
	resultChannel := make(chan error, writerCount)
	for writerIndex := range writerCount {
		go func() {
			<-startChannel
			task := flowSummaryInvalidationTask(
				fmt.Sprintf("concurrent-task-%d", writerIndex),
				"26W28",
				"2026-07-06",
				"2026-07-07",
				flowStatusInProgress,
				(writerIndex+1)*1024,
			)
			resultChannel <- service.writeFlowTask(ctx, task)
		}()
	}
	close(startChannel)
	writeErrors := make([]error, 0)
	for range writerCount {
		if writeError := <-resultChannel; writeError != nil {
			writeErrors = append(writeErrors, writeError)
		}
	}
	if len(writeErrors) != 0 {
		t.Fatalf("concurrent task writes failed: %v", writeErrors)
	}
	database, errorValue = service.openFlowDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var storedTaskCount int
	if errorValue := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM flow_tasks WHERE id LIKE 'concurrent-task-%'").Scan(&storedTaskCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if storedTaskCount != writerCount {
		t.Fatalf("stored task count = %d, want %d", storedTaskCount, writerCount)
	}
	for _, key := range []flowSummarySourceKey{
		{Kind: flowSummarySourceWeek, Key: "26W28"},
		{Kind: flowSummarySourceMonth, Key: "2026-07"},
	} {
		var revision int64
		if errorValue := database.QueryRowContext(ctx, "SELECT revision FROM flow_summary_source_revisions WHERE source_kind = ? AND source_key = ?", key.Kind, key.Key).Scan(&revision); errorValue != nil {
			t.Fatal(errorValue)
		}
		if revision != writerCount {
			t.Fatalf("revision %s/%s = %d, want %d", key.Kind, key.Key, revision, writerCount)
		}
	}
}
