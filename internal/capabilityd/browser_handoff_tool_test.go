package capabilityd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/capabilities"
)

const browserHandoffFromAConversation = `{
	"input": {"message": "홈택스에 로그인해 주세요"},
	"context": {
		"requesterEmail": "Sample@Example.test",
		"requesterName": "이샘플",
		"platform": "buzz",
		"conversationID": "conversation-1",
		"conversationType": "direct",
		"responseLanguage": "ko"
	}
}`

func deviceBrowserShowing(address string, title string, text string) func(context.Context, string, []string, []byte) ([]byte, error) {
	return func(_ context.Context, _ string, arguments []string, _ []byte) ([]byte, error) {
		command := strings.Join(arguments, " ")
		switch {
		case strings.Contains(command, " snapshot "):
			return json.Marshal(map[string]any{"success": true, "data": map[string]any{"origin": address, "title": title, "snapshot": "- link [ref=e1]"}})
		case strings.HasSuffix(command, " get url"):
			return []byte(address + "\n"), nil
		case strings.Contains(command, " get text body"):
			answer, errorValue := json.Marshal(map[string]any{"success": true, "data": map[string]any{"origin": address, "text": text}})
			return append([]byte("[agent-browser] restore: loaded; save: saved\n"), answer...), errorValue
		}
		return nil, nil
	}
}

func relayTakingHandoffs() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"handoffID":"handoff-1","openURL":"https://intern.kim/handoff/handoff-1","expiresAt":"2026-09-17T00:15:00.000Z"}`))
	}))
}

func TestBrowserHandoffIsBegunOnTheRelayForTheRequester(t *testing.T) {
	var relayed relayHandoffRequest
	relay := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/browser-handoffs" {
			t.Errorf("relay path = %s", request.URL.Path)
		}
		body, _ := io.ReadAll(request.Body)
		if errorValue := json.Unmarshal(body, &relayed); errorValue != nil {
			t.Errorf("relay body is not a handoff request: %v", errorValue)
		}
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"handoffID":"handoff-1","openURL":"https://intern.kim/handoff/handoff-1","expiresAt":"2026-09-17T00:15:00.000Z"}`))
	}))
	defer relay.Close()
	now := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	browsers := fakeDeviceBrowsers(1, now)
	service := Service{
		Configuration:  Configuration{RelayBaseURL: relay.URL},
		DeviceBrowsers: browsers,
		RunCommand:     deviceBrowserShowing("https://hometax.go.kr/login", "홈택스", "로그인"),
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser_handoff", strings.NewReader(browserHandoffFromAConversation))

	if errorValue != nil {
		t.Fatalf("browser_handoff failed: %v", errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded {
		t.Fatalf("outcome = %s, response = %+v", response.Outcome, response)
	}
	var result browserHandoffResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result != (browserHandoffResult{
		HandoffID: "handoff-1",
		Status:    "waiting",
		OpenURL:   "https://intern.kim/handoff/handoff-1",
		ExpiresAt: "2026-09-17T00:15:00.000Z",
		Page:      handoffPage{URL: "https://hometax.go.kr/login", Title: "홈택스", Text: "로그인"},
	}) {
		t.Fatalf("result = %+v", result)
	}
	expected := relayHandoffRequest{
		Message:   "홈택스에 로그인해 주세요",
		Requester: relayHandoffRequester{Email: "sample@example.test", Name: "이샘플"},
		Addressing: relayHandoffAddressing{
			Platform:         "buzz",
			ConversationID:   "conversation-1",
			ConversationType: "direct",
			ResponseLanguage: "ko",
		},
		DevtoolsURL: "http://127.0.0.1:9230",
	}
	if relayed != expected {
		t.Fatalf("relayed = %+v", relayed)
	}
	if _, errorValue := browsers.BrowserFor(context.Background(), "someone-else@example.test"); !errors.Is(errorValue, browserruntime.ErrDeviceBrowsersFull) {
		t.Fatalf("expected the requester's browser to be held for the handoff, got %v", errorValue)
	}
}

func TestBrowserHandoffOutsideAConversationFailsWithoutReachingTheRelay(t *testing.T) {
	relayWasCalled := false
	relay := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		relayWasCalled = true
	}))
	defer relay.Close()
	service := Service{Configuration: Configuration{RelayBaseURL: relay.URL}}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser_handoff", strings.NewReader(`{"input":{"message":"sign in"}}`))

	if errorValue != nil {
		t.Fatalf("expected a failed response, got error %v", errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !strings.Contains(response.Content, "conversation") {
		t.Fatalf("response = %+v", response)
	}
	if relayWasCalled {
		t.Fatal("a handoff nobody can resume must not reach the relay")
	}
}

func TestBrowserHandoffOpensTheAddressBeforeHandingOver(t *testing.T) {
	relayWasCalled := false
	relay := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		relayWasCalled = true
	}))
	defer relay.Close()
	var browserArguments []string
	service := Service{
		Configuration:  Configuration{RelayBaseURL: relay.URL},
		DeviceBrowsers: fakeDeviceBrowsers(1, time.Now()),
		RunCommand: func(_ context.Context, _ string, arguments []string, _ []byte) ([]byte, error) {
			browserArguments = append(browserArguments, arguments...)
			return nil, errors.New("the device browser is down")
		},
	}
	withAddress := strings.Replace(browserHandoffFromAConversation, `"message": "홈택스에 로그인해 주세요"`, `"message": "sign in", "url": "https://hometax.go.kr/"`, 1)

	_, errorValue := service.invokeCapabilityTool(context.Background(), "browser_handoff", strings.NewReader(withAddress))

	if errorValue == nil {
		t.Fatal("expected the unopened address to stop the handoff")
	}
	if !strings.Contains(strings.Join(browserArguments, " "), "https://hometax.go.kr/") {
		t.Fatalf("device browser arguments = %v", browserArguments)
	}
	if relayWasCalled {
		t.Fatal("a page the device browser never opened must not be handed over")
	}
}

func TestBrowserHandoffTellsTheAgentWhatTheRequesterWillSee(t *testing.T) {
	relay := relayTakingHandoffs()
	defer relay.Close()
	notFound := "페이지를 찾을 수 없습니다.\n" + strings.Repeat("주소를 확인해 주세요. ", 200)
	service := Service{
		Configuration:  Configuration{RelayBaseURL: relay.URL},
		DeviceBrowsers: fakeDeviceBrowsers(1, time.Now()),
		RunCommand:     deviceBrowserShowing("https://nid.naver.com/no-such-page", "네이버", notFound),
	}
	withAddress := strings.Replace(browserHandoffFromAConversation, `"message": "홈택스에 로그인해 주세요"`, `"message": "sign in", "url": "https://nid.naver.com/no-such-page"`, 1)

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser_handoff", strings.NewReader(withAddress))

	if errorValue != nil {
		t.Fatalf("browser_handoff failed: %v", errorValue)
	}
	var result browserHandoffResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.Page.URL != "https://nid.naver.com/no-such-page" || !strings.HasPrefix(result.Page.Text, "페이지를 찾을 수 없습니다.") {
		t.Fatalf("page = %+v", result.Page)
	}
	if len([]rune(result.Page.Text)) != handoffPageTextLimit+1 {
		t.Fatalf("expected the page text cut to %d characters, got %d", handoffPageTextLimit, len([]rune(result.Page.Text)))
	}
}

func TestBrowserHandoffWithNoPageOpenIsNotHandedOver(t *testing.T) {
	relayWasCalled := false
	relay := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		relayWasCalled = true
	}))
	defer relay.Close()
	service := Service{
		Configuration:  Configuration{RelayBaseURL: relay.URL},
		DeviceBrowsers: fakeDeviceBrowsers(1, time.Now()),
		RunCommand:     deviceBrowserShowing("about:blank", "", ""),
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser_handoff", strings.NewReader(browserHandoffFromAConversation))

	if errorValue != nil {
		t.Fatalf("expected a failed response, got error %v", errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !strings.Contains(response.Content, "no web page open") {
		t.Fatalf("response = %+v", response)
	}
	if relayWasCalled {
		t.Fatal("a blank browser must not be handed over")
	}
}

func TestBrowserHandoffReportsARelayThatRefuses(t *testing.T) {
	relay := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte("a browser handoff names its requester and conversation"))
	}))
	defer relay.Close()
	service := Service{
		Configuration:  Configuration{RelayBaseURL: relay.URL},
		DeviceBrowsers: fakeDeviceBrowsers(1, time.Now()),
		RunCommand:     deviceBrowserShowing("https://hometax.go.kr/", "홈택스", "로그인"),
	}

	_, errorValue := service.invokeCapabilityTool(context.Background(), "browser_handoff", strings.NewReader(browserHandoffFromAConversation))

	if errorValue == nil || !strings.Contains(errorValue.Error(), "answered 400") {
		t.Fatalf("error = %v", errorValue)
	}
}
