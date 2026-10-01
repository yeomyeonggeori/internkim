package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPlatformProgressManagerExpiresLeases(t *testing.T) {
	manager := newPlatformProgressManager()
	stopped := make(chan struct{}, 1)

	manager.Start("test", 20*time.Millisecond, func(ctx context.Context) {
		<-ctx.Done()
		stopped <- struct{}{}
	})

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("expected progress lease to expire")
	}
	manager.mutex.Lock()
	_, isActive := manager.leaseByKey["test"]
	manager.mutex.Unlock()
	if isActive {
		t.Fatal("expected expired lease to be removed")
	}
}

func TestPlatformProgressManagerRefreshesLeaseWithoutDuplicateLoop(t *testing.T) {
	manager := newPlatformProgressManager()
	started := make(chan struct{}, 2)
	stopped := make(chan struct{}, 1)

	manager.Start("test", 50*time.Millisecond, func(ctx context.Context) {
		started <- struct{}{}
		<-ctx.Done()
		stopped <- struct{}{}
	})
	manager.Start("test", 150*time.Millisecond, func(ctx context.Context) {
		started <- struct{}{}
		<-ctx.Done()
		stopped <- struct{}{}
	})

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected progress loop to start")
	}
	select {
	case <-started:
		t.Fatal("expected existing lease refresh without duplicate loop")
	case <-time.After(20 * time.Millisecond):
	}
	select {
	case <-stopped:
		t.Fatal("expected refreshed lease to remain active after original ttl")
	case <-time.After(80 * time.Millisecond):
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("expected refreshed lease to expire")
	}
}

func TestPlatformAttachmentRejectsOutsideDevicePath(t *testing.T) {
	companionDirectory := t.TempDir()
	outsidePath := t.TempDir() + "/secret.txt"
	if errorValue := os.WriteFile(outsidePath, []byte("secret"), 0o600); errorValue != nil {
		t.Fatalf("expected outside file: %v", errorValue)
	}
	service := Service{Configuration: Configuration{CompanionFileDirectory: companionDirectory}}

	_, errorValue := service.validatePlatformFiles([]platformFileSpec{{DevicePath: outsidePath}})
	if errorValue == nil {
		t.Fatal("expected outside attachment path to fail")
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	document, errorValue := json.Marshal(value)
	if errorValue != nil {
		t.Fatalf("expected json marshal: %v", errorValue)
	}
	return document
}

func testJSONResponse(statusCode int, response any) *http.Response {
	var responseBody bytes.Buffer
	_ = json.NewEncoder(&responseBody).Encode(response)
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(&responseBody),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestRouterDoesNotRegisterDeprecatedPlatformEndpoints(t *testing.T) {
	service := Service{Configuration: DefaultConfiguration()}
	for _, path := range []string{
		"/v1/platform/buzz/bot.resolve",
		"/v1/platform/buzz/conversation.kind",
		"/v1/platform/buzz/typing.publish",
	} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		responseRecorder := httptest.NewRecorder()
		service.router().ServeHTTP(responseRecorder, request)
		if responseRecorder.Code != http.StatusNotFound {
			t.Fatalf("expected %s to be unregistered, got %d", path, responseRecorder.Code)
		}
	}
}

func TestForwardedPlatformEventDoesNotLeakLegacyFields(t *testing.T) {
	document, errorValue := json.Marshal(platformInboundEvent{
		ConversationID: "thread:channel-1:post-1",
		MessageID:      "post-1",
		SenderID:       "user-1",
		ReplyTargetID:  "reply",
		Prompt:         "hello",
		Context:        platformEventContext{Messages: []platformContextMessage{{Speaker: "admin", Text: "previous"}}},
	})
	if errorValue != nil {
		t.Fatalf("expected event to marshal: %v", errorValue)
	}
	var requestBody map[string]any
	if errorValue := json.Unmarshal(document, &requestBody); errorValue != nil {
		t.Fatalf("expected request body to decode: %v", errorValue)
	}
	forbiddenKeys := []string{"platform", "source", "eventID", "channelType", "rootID", "thread_ts", "post_id", "isBotMessage", "senderUserID", "text"}
	for _, key := range forbiddenKeys {
		if _, isFound := requestBody[key]; isFound {
			t.Fatalf("expected %q to be absent from forwarded body: %v", key, requestBody)
		}
	}
	if requestBody["prompt"] != "hello" {
		t.Fatalf("expected prompt in forwarded body, got %v", requestBody)
	}
}
