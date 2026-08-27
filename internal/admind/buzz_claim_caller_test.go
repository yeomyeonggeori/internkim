package admind

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func buzzClaimTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(Configuration{
		ListenAddress:   "127.0.0.1:18080",
		BuzzKeySeedPath: writeTestFile(t, "a-seed-for-this-test"),
	})
}

// The relay is another service on this machine acting for somebody Supabase has
// already identified, and it names them in the header loopback callers use.
func TestTheRelayCanClaimForThePersonItNames(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://internkim/agent/api/buzz-claim", nil)
	request.Header.Set(requesterEmailHeader, "leesample@example.test")
	recorder := httptest.NewRecorder()

	buzzClaimTestService(t).handleBuzzClaim(recorder, arrivingOnTheRequesterSocket(request))

	if recorder.Code != http.StatusOK {
		t.Fatalf("the relay named the person, so the claim has to answer: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestAClaimFromOutsideStillHasToProveWhoItIs(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "https://device.example.test/agent/api/buzz-claim", nil)
	request.RemoteAddr = "203.0.113.7:41000"
	request.Header.Set(requesterEmailHeader, "somebody@example.test")
	recorder := httptest.NewRecorder()

	buzzClaimTestService(t).handleBuzzClaim(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("a header from outside names nobody: %d", recorder.Code)
	}
}

func TestTheSameEmailAlwaysClaimsTheSameKey(t *testing.T) {
	service := buzzClaimTestService(t)
	claim := func() string {
		request := httptest.NewRequest(http.MethodGet, "http://internkim/agent/api/buzz-claim", nil)
		request.Header.Set(requesterEmailHeader, "Lee@Example.test ")
		recorder := httptest.NewRecorder()
		service.handleBuzzClaim(recorder, arrivingOnTheRequesterSocket(request))
		return recorder.Body.String()
	}

	if first, second := claim(), claim(); first != second {
		t.Fatalf("a derived key that moves is a second identity: %q vs %q", first, second)
	}
}
