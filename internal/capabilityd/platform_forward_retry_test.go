package capabilityd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestForwardPlatformEventRetriesWhileBackupDefersIt(t *testing.T) {
	originalDelay := backupDeferredForwardRetryDelay
	backupDeferredForwardRetryDelay = 5 * time.Millisecond
	t.Cleanup(func() { backupDeferredForwardRetryDelay = originalDelay })

	var requestCount atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if requestCount.Add(1) <= 2 {
			writer.Write([]byte(`{"handled":true,"ignored":true,"reason":"backup_prepare_active"}`))
			return
		}
		writer.Write([]byte(`{"handled":true}`))
	}))
	t.Cleanup(server.Close)

	service := Service{Configuration: Configuration{BlueclawBaseURL: server.URL}, HTTPClient: server.Client()}
	errorValue := service.forwardPlatformEvent(context.Background(), "mattermost", platformInboundEvent{
		ConversationID: "conversation-1",
		MessageID:      "message-1",
	})
	if errorValue != nil {
		t.Fatalf("expected the forward to succeed after retries: %v", errorValue)
	}
	if requestCount.Load() != 3 {
		t.Fatalf("expected 3 forward attempts, got %d", requestCount.Load())
	}
}

func TestForwardPlatformEventGivesUpAfterTheBackupRetryWindow(t *testing.T) {
	originalDelay := backupDeferredForwardRetryDelay
	backupDeferredForwardRetryDelay = time.Millisecond
	t.Cleanup(func() { backupDeferredForwardRetryDelay = originalDelay })

	var requestCount atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte(`{"handled":true,"ignored":true,"reason":"backup_prepare_active"}`))
	}))
	t.Cleanup(server.Close)

	service := Service{Configuration: Configuration{BlueclawBaseURL: server.URL}, HTTPClient: server.Client()}
	errorValue := service.forwardPlatformEvent(context.Background(), "mattermost", platformInboundEvent{
		ConversationID: "conversation-1",
		MessageID:      "message-1",
	})
	if errorValue == nil {
		t.Fatal("expected an error once the retry window is exhausted")
	}
	if requestCount.Load() != int64(backupDeferredForwardRetryLimit)+1 {
		t.Fatalf("expected %d forward attempts, got %d", backupDeferredForwardRetryLimit+1, requestCount.Load())
	}
}
