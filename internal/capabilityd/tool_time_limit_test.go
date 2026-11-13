package capabilityd

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const desktopReadOverASecond = `{
	"toolName": "desktop_read",
	"input": {"arguments": ["snapshot", "--app", "Finder", "-i"]},
	"context": {"requesterEmail": "sample@example.com"},
	"timeoutSecond": 1
}`

func serviceWithARelayThatNeverAnswers(t *testing.T) Service {
	t.Helper()
	released := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-released
	}))
	t.Cleanup(func() {
		close(released)
		server.Close()
	})
	tokenPath := filepath.Join(t.TempDir(), "service-token")
	if errorValue := os.WriteFile(tokenPath, []byte("host-service-token\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return Service{Configuration: Configuration{RelayBaseURL: server.URL, ServiceTokenPath: tokenPath}}
}

func TestAnInvocationEndsAtTheLimitItsRequestNamesAndSaysSo(t *testing.T) {
	service := serviceWithARelayThatNeverAnswers(t)
	startedAt := time.Now()

	response, errorValue := service.invokeCapabilityTool(context.Background(), "desktop_read", strings.NewReader(desktopReadOverASecond))

	if errorValue != nil {
		t.Fatalf("the caller would see a dropped call instead of a tool result: %v", errorValue)
	}
	if elapsed := time.Since(startedAt); elapsed > 3*time.Second {
		t.Fatalf("the invocation answered after %s and the request allowed 1s", elapsed)
	}
	if !response.IsError || response.Outcome != capabilities.ToolOutcomeFailed || response.ErrorCode != "timeout" || response.ToolName != "desktop_read" {
		t.Fatalf("answered %+v", response)
	}
	if response.Message != "desktop_read did not finish within 1 seconds, and the work it started may still have taken effect" {
		t.Fatalf("the model reads %q", response.Message)
	}
	if response.Retryable || response.SafeRetry {
		t.Fatalf("a call whose work may have taken effect was offered as retryable: %+v", response)
	}
}

func TestAHandlerThatIgnoresItsContextIsStillAnsweredAtTheLimit(t *testing.T) {
	ignoresItsContext := func(Service, context.Context, capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
		time.Sleep(3 * time.Second)
		return capabilities.ToolInvokeResponse{ToolName: "image_generate", Status: "ok"}, nil
	}
	startedAt := time.Now()

	response, errorValue := Service{}.answerWithinTheRequestedTime(context.Background(), capabilities.ToolInvokeRequest{ToolName: "image_generate", TimeoutSecond: 1}, ignoresItsContext)

	if errorValue != nil || response.ErrorCode != "timeout" {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
	if elapsed := time.Since(startedAt); elapsed > 2*time.Second {
		t.Fatalf("the invocation waited %s for a handler that ignores its context", elapsed)
	}
}

func TestARequestWithNoLimitRunsUnderTheCallersOwnContext(t *testing.T) {
	callerContext, release := context.WithCancel(context.Background())
	defer release()
	failure := errors.New("the record is unreachable")
	seesTheCallersContext := func(_ Service, ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
		if _, hasDeadline := ctx.Deadline(); hasDeadline || ctx != callerContext {
			t.Errorf("a request with no limit was given a bounded context")
		}
		return capabilities.ToolInvokeResponse{}, failure
	}

	_, errorValue := Service{}.answerWithinTheRequestedTime(callerContext, capabilities.ToolInvokeRequest{ToolName: "task_add"}, seesTheCallersContext)

	if !errors.Is(errorValue, failure) {
		t.Fatalf("a request with no limit came back %v instead of its handler's own error", errorValue)
	}
}

func TestAFinishedAnswerStandsWhateverTheLimit(t *testing.T) {
	answers := func(Service, context.Context, capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
		return capabilities.ToolInvokeResponse{ToolName: "task_add", Status: "ok", Outcome: capabilities.ToolOutcomeSucceeded}, nil
	}

	response, errorValue := Service{}.answerWithinTheRequestedTime(context.Background(), capabilities.ToolInvokeRequest{ToolName: "task_add", TimeoutSecond: 300}, answers)

	if errorValue != nil || response.Status != "ok" {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
}

func TestACallerThatGoesAwayIsNotToldTheToolRanOutOfTime(t *testing.T) {
	callerContext, cancelCaller := context.WithCancel(context.Background())
	time.AfterFunc(10*time.Millisecond, cancelCaller)
	waitsForTheEnd := func(_ Service, ctx context.Context, _ capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
		<-ctx.Done()
		return capabilities.ToolInvokeResponse{}, ctx.Err()
	}

	response, errorValue := Service{}.answerWithinTheRequestedTime(callerContext, capabilities.ToolInvokeRequest{ToolName: "desktop_act", TimeoutSecond: 120}, waitsForTheEnd)

	if !errors.Is(errorValue, context.Canceled) || response.ErrorCode == "timeout" {
		t.Fatalf("a cancelled call came back %+v, %v", response, errorValue)
	}
}

func TestAHandlerThatPanicsUnderALimitIsAnsweredAsAFailedCall(t *testing.T) {
	panics := func(Service, context.Context, capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
		panic("index out of range")
	}

	response, errorValue := Service{}.answerWithinTheRequestedTime(context.Background(), capabilities.ToolInvokeRequest{ToolName: "desktop_read", TimeoutSecond: 120}, panics)

	if errorValue != nil || !response.IsError || response.ErrorCode != "tool_crashed" || response.ToolName != "desktop_read" {
		t.Fatalf("a panicking handler was answered %+v, %v", response, errorValue)
	}
	if response.Message != "the tool crashed while running: index out of range" {
		t.Fatalf("the model reads %q", response.Message)
	}
}

func TestAnAnswerAlreadyGivenWhenTheLimitArrivesStands(t *testing.T) {
	boundedContext, release := context.WithTimeout(context.Background(), time.Millisecond)
	defer release()
	<-boundedContext.Done()
	answered := make(chan toolAnswer, 1)
	answered <- toolAnswer{response: capabilities.ToolInvokeResponse{ToolName: "task_add", Status: "ok", Outcome: capabilities.ToolOutcomeSucceeded}}

	response, errorValue := answerAtTheLimit(context.Background(), boundedContext, capabilities.ToolInvokeRequest{ToolName: "task_add", TimeoutSecond: 1}, answered)

	if errorValue != nil || response.Status != "ok" {
		t.Fatalf("a finished answer lost to the limit it beat: %+v, %v", response, errorValue)
	}
}

func TestEveryServedDescriptorCarriesATimeLimit(t *testing.T) {
	for _, descriptor := range capabilities.RegistryDescriptors(servedCapabilityRegistry(t)) {
		declared, _ := capabilityToolDescriptorFor(descriptor.CanonicalName)
		expected := declared.TimeoutSecond
		if expected <= 0 {
			expected = defaultToolTimeoutSecond
		}
		if descriptor.TimeoutSecond != expected {
			t.Errorf("%s is served with timeoutSecond %d, expected %d", descriptor.Name, descriptor.TimeoutSecond, expected)
		}
	}
}
