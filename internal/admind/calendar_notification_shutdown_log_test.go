package admind

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func finishReconciliationJobCapturingLog(t *testing.T, ctx context.Context, reconciliationError error) string {
	t.Helper()
	service := &Service{calendarNotificationStates: map[string]*calendarNotificationReconciliationState{}}
	state := &calendarNotificationReconciliationState{running: true}
	service.calendarNotificationStates["event-1"] = state

	var logOutput bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logOutput, nil)))
	defer slog.SetDefault(previousLogger)

	service.finishCalendarNotificationReconciliationJob(ctx, calendarNotificationReconciliationJob{eventID: "event-1", state: state}, reconciliationError)
	return logOutput.String()
}

func TestAWorkerStoppedAtShutdownIsNotReportedAsAFailedReconciliation(t *testing.T) {
	cancelledContext, cancel := context.WithCancel(context.Background())
	cancel()

	logOutput := finishReconciliationJobCapturingLog(t, cancelledContext, context.Canceled)

	if strings.Contains(logOutput, "calendar notification reconciliation failed") {
		t.Fatalf("a worker asked to stop did not fail, got %s", logOutput)
	}
}

func TestAReconciliationThatActuallyFailedIsStillReported(t *testing.T) {
	logOutput := finishReconciliationJobCapturingLog(t, context.Background(), context.DeadlineExceeded)

	if !strings.Contains(logOutput, "calendar notification reconciliation failed") {
		t.Fatalf("a real failure has to stay visible, got %s", logOutput)
	}
}
