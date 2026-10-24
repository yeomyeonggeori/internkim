package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

type handingOverGuard struct {
	guardHoldingNothing
	mutex        sync.Mutex
	isHandedOver bool
	digests      []string
}

func (guard *handingOverGuard) HandOver() {
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	guard.isHandedOver = true
}

func (guard *handingOverGuard) TakeBack() {
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	guard.isHandedOver = false
}

func (guard *handingOverGuard) handedOver() bool {
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	return guard.isHandedOver
}

func (guard *handingOverGuard) CookieJarDigest(context.Context, string) (string, error) {
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	digest := guard.digests[0]
	if len(guard.digests) > 1 {
		guard.digests = guard.digests[1:]
	}
	return digest, nil
}

type fakeRelay struct {
	mutex         sync.Mutex
	registrations []relayHandoffRegistration
	status        string
}

func (relay *fakeRelay) serve(writer http.ResponseWriter, request *http.Request) {
	relay.mutex.Lock()
	defer relay.mutex.Unlock()
	if request.Method == http.MethodPost && request.URL.Path == relayHandoffsPath {
		var registration relayHandoffRegistration
		_ = json.NewDecoder(request.Body).Decode(&registration)
		relay.registrations = append(relay.registrations, registration)
		writer.WriteHeader(http.StatusCreated)
		return
	}
	if request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, relayHandoffsPath+"/") {
		_ = json.NewEncoder(writer).Encode(relayHandoffStatus{Status: relay.status, SignInOrigin: "https://portal.example.com"})
		return
	}
	writer.WriteHeader(http.StatusNotFound)
}

func (relay *fakeRelay) setStatus(status string) {
	relay.mutex.Lock()
	defer relay.mutex.Unlock()
	relay.status = status
}

func (relay *fakeRelay) registered(t *testing.T) relayHandoffRegistration {
	t.Helper()
	relay.mutex.Lock()
	defer relay.mutex.Unlock()
	if len(relay.registrations) != 1 {
		t.Fatalf("expected one hand-over registered with the relay, got %+v", relay.registrations)
	}
	return relay.registrations[0]
}

type handoffFixture struct {
	service Service
	guard   *handingOverGuard
	relay   *fakeRelay
	ledger  []heldCallRedemption
}

func newHandoffFixture(t *testing.T) *handoffFixture {
	t.Helper()
	fixture := &handoffFixture{guard: &handingOverGuard{digests: []string{"before", "after"}}, relay: &fakeRelay{status: relayHandoffWaiting}}
	relayServer := httptest.NewServer(http.HandlerFunc(fixture.relay.serve))
	t.Cleanup(relayServer.Close)
	ledgerServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var redemption heldCallRedemption
		_ = json.NewDecoder(request.Body).Decode(&redemption)
		fixture.ledger = append(fixture.ledger, redemption)
	}))
	t.Cleanup(ledgerServer.Close)
	fixture.service = Service{
		Configuration:   Configuration{AgentBrowserPath: "agent-browser-test", RelayBaseURL: relayServer.URL, BlueclawBaseURL: ledgerServer.URL},
		DeviceBrowsers:  fakeDeviceBrowsers(4, time.Now()),
		browserHandoffs: &browserHandoffs{},
		browserRequestGuards: &browserRequestGuards{attach: func(context.Context, string) (pageRequestGuard, error) {
			return fixture.guard, nil
		}},
		RunCommand: func(_ context.Context, _ string, commandArguments []string, _ []byte) ([]byte, error) {
			if slices.Contains(commandArguments, "get") && slices.Contains(commandArguments, "url") {
				return []byte(`{"success":true,"data":{"url":"https://portal.example.com/login?next=%2Finvoices"}}`), nil
			}
			return []byte(`{"success":true,"data":{"snapshot":"","refs":{}}}`), nil
		},
	}
	return fixture
}

func (fixture *handoffFixture) invoke(t *testing.T, toolName string, input string, requestContext map[string]string) capabilities.ToolInvokeResponse {
	t.Helper()
	document, _ := json.Marshal(map[string]any{"input": json.RawMessage(input), "context": requestContext})
	response, errorValue := fixture.service.invokeCapabilityTool(context.Background(), toolName, strings.NewReader(string(document)))
	if errorValue != nil {
		t.Fatalf("%s: %v", toolName, errorValue)
	}
	return response
}

func askedIn(requesterEmail string) map[string]string {
	return map[string]string{"requesterEmail": requesterEmail, "requesterPersonID": "person-sample", "conversationID": "conversation-1"}
}

func handoffTargetOf(t *testing.T, response capabilities.ToolInvokeResponse) capabilities.ApprovalTarget {
	t.Helper()
	var refusal struct {
		ApprovalTarget capabilities.ApprovalTarget `json:"approvalTarget"`
	}
	if response.ErrorCode != "approval_required" || json.Unmarshal(response.Result, &refusal) != nil {
		t.Fatalf("expected the hand-over to wait on the requester, got %+v", response)
	}
	return refusal.ApprovalTarget
}

func TestAHandOverWaitsOnTheRequesterAndKeepsTheModelOutOfTheirBrowser(t *testing.T) {
	fixture := newHandoffFixture(t)

	target := handoffTargetOf(t, fixture.invoke(t, "browser_handoff", `{"step":"sign in to the vendor portal"}`, askedIn("sample@example.com")))

	registration := fixture.relay.registered(t)
	if target.HandoffID == "" || target.HandoffID != registration.HandoffID || target.ID != target.HandoffID || target.InputField != "handoff" {
		t.Fatalf("the hold must name the hand-over the relay opens, got target %+v and registration %+v", target, registration)
	}
	if target.Title != "https://portal.example.com" || registration.SignInOrigin != "https://portal.example.com" || registration.RequesterEmail != "sample@example.com" {
		t.Fatalf("the hand-over names the origin and the requester only, got %+v %+v", target, registration)
	}
	if !fixture.guard.handedOver() {
		t.Fatal("the requester's own sign-in must not be withheld as if the agent sent it")
	}
	for toolName, input := range map[string]string{"browser_snapshot": `{}`, "browser_click": `{"target":"@e1"}`, "browser_handoff": `{"step":"sign in"}`} {
		if response := fixture.invoke(t, toolName, input, askedIn("sample@example.com")); response.Outcome != capabilities.ToolOutcomeFailed || !strings.Contains(response.Content, "in their own hands") {
			t.Fatalf("%s ran in a browser the requester holds: %+v", toolName, response)
		}
	}
	if response := fixture.invoke(t, "browser_snapshot", `{}`, askedIn("other@example.com")); response.Outcome != capabilities.ToolOutcomeSucceeded {
		t.Fatalf("another person's browser is not handed over, got %+v", response)
	}
}

func TestTheHandedBackBrowserReportsHowItEndedAndWhetherTheSignInChangedItsCookies(t *testing.T) {
	fixture := newHandoffFixture(t)
	target := handoffTargetOf(t, fixture.invoke(t, "browser_handoff", `{"step":"sign in to the vendor portal"}`, askedIn("sample@example.com")))
	fixture.relay.setStatus(relayHandoffDone)
	onTheHold := askedIn("sample@example.com")
	onTheHold["holdID"] = "hold-1"
	onTheHold["taskRunID"] = "run-1"

	response := fixture.invoke(t, "browser_handoff", `{"step":"sign in to the vendor portal","handoff":"`+target.HandoffID+`"}`, onTheHold)

	var result browserHandoffResult
	if response.Outcome != capabilities.ToolOutcomeSucceeded || json.Unmarshal(response.Result, &result) != nil {
		t.Fatalf("expected the outcome of the hand-over, got %+v", response)
	}
	if result.Ended != "done" || result.Origin != "https://portal.example.com" || result.SignInOrigin != "https://portal.example.com" || result.SignInCookiesChanged == nil || !*result.SignInCookiesChanged {
		t.Fatalf("the model is told how it ended, the origin and whether the cookies changed, got %+v", result)
	}
	if len(fixture.ledger) != 1 || fixture.ledger[0].HoldID != "hold-1" {
		t.Fatalf("the report is given only once the ledger redeems the hold, got %+v", fixture.ledger)
	}
	if fixture.guard.handedOver() {
		t.Fatal("the guard holds the agent's requests again once the browser is back")
	}
	if response := fixture.invoke(t, "browser_snapshot", `{}`, askedIn("sample@example.com")); response.Outcome != capabilities.ToolOutcomeSucceeded {
		t.Fatalf("the agent drives the browser again once it is handed back, got %+v", response)
	}
}

func TestAHandoffTheModelNamesWithoutAHoldIsRefused(t *testing.T) {
	fixture := newHandoffFixture(t)

	response := fixture.invoke(t, "browser_handoff", `{"step":"sign in","handoff":"invented"}`, askedIn("sample@example.com"))

	if response.Outcome != capabilities.ToolOutcomeFailed || !strings.Contains(response.Content, "set only by the runtime") || len(fixture.ledger) != 0 {
		t.Fatalf("a model-set handoff must be refused before anything is read, got %+v", response)
	}
}

func TestAHandOverOutsideAConversationIsRefused(t *testing.T) {
	fixture := newHandoffFixture(t)

	response := fixture.invoke(t, "browser_handoff", `{"step":"sign in"}`, map[string]string{"requesterEmail": "sample@example.com"})

	if response.Outcome != capabilities.ToolOutcomeFailed || fixture.guard.handedOver() {
		t.Fatalf("nobody can take a browser handed over outside a conversation, got %+v", response)
	}
}

func TestAHandOverTheRequesterDeclinedUnlocksTheBrowserWithoutAnotherCall(t *testing.T) {
	fixture := newHandoffFixture(t)
	handoffTargetOf(t, fixture.invoke(t, "browser_handoff", `{"step":"sign in to the vendor portal"}`, askedIn("sample@example.com")))

	fixture.relay.setStatus(relayHandoffAbandoned)

	deadline := time.Now().Add(3 * handoffWatchInterval)
	for fixture.service.browserHandoffs.isHandedOver("sample@example.com") {
		if time.Now().After(deadline) {
			t.Fatal("a hand-over the relay ended still locks the browser")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if fixture.guard.handedOver() {
		t.Fatal("the guard must hold the agent's requests again")
	}
}
