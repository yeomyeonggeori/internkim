package admind

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

type flowTransactionBeginResult struct {
	transaction *sql.Tx
	errorValue  error
}

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
	beginResultChannel := make(chan flowTransactionBeginResult, 1)
	go func() {
		transaction, beginError := secondDatabase.BeginTx(ctx, nil)
		beginResultChannel <- flowTransactionBeginResult{transaction: transaction, errorValue: beginError}
	}()
	select {
	case result := <-beginResultChannel:
		if result.transaction != nil {
			_ = result.transaction.Rollback()
		}
		t.Fatalf("second BeginTx returned before first commit: %v", result.errorValue)
	case <-time.After(100 * time.Millisecond):
	}
	if errorValue := incrementFlowSummarySourceRevisions(ctx, firstTransaction, []flowSummarySourceKey{{Kind: flowSummarySourceWeek, Key: "26W28"}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := firstTransaction.Commit(); errorValue != nil {
		t.Fatal(errorValue)
	}
	select {
	case result := <-beginResultChannel:
		if result.errorValue != nil {
			t.Fatal(result.errorValue)
		}
		if errorValue := result.transaction.Rollback(); errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second BeginTx did not proceed after first commit")
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
