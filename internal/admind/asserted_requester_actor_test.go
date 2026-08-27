package admind

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func arrivingOnTheRequesterSocket(request *http.Request) *http.Request {
	var markedRequest *http.Request
	markRequestsAsAssertedByTheListener(http.HandlerFunc(func(_ http.ResponseWriter, marked *http.Request) {
		markedRequest = marked
	})).ServeHTTP(httptest.NewRecorder(), request)
	return markedRequest
}

func aRequestFrom(remoteAddress string, headerEmail string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/files/api/roots", nil)
	request.RemoteAddr = remoteAddress
	if headerEmail != "" {
		request.Header.Set(requesterEmailHeader, headerEmail)
	}
	return request
}

func TestARequesterSocketCallerNamesThePersonItActsFor(t *testing.T) {
	service := &Service{}
	request := arrivingOnTheRequesterSocket(aRequestFrom("@", "Sample@Example.test"))

	if actorEmail := service.actorEmailAllowingAssertedRequester(request); actorEmail != "sample@example.test" {
		t.Fatalf("a requester socket caller could not name its person, got %q", actorEmail)
	}
}

func TestTheTCPListenerCannotNameAPerson(t *testing.T) {
	service := &Service{}

	for _, remoteAddress := range []string{"127.0.0.1:5000", "203.0.113.9:5000", "192.168.0.50:5000", "[2001:db8::1]:5000"} {
		if actorEmail := service.actorEmailAllowingAssertedRequester(aRequestFrom(remoteAddress, "sample@example.test")); actorEmail != "" {
			t.Errorf("%s named a person it has no right to: %q", remoteAddress, actorEmail)
		}
	}
}

func TestARequesterSocketCallerNamingNobodyIsNobody(t *testing.T) {
	service := &Service{}
	request := arrivingOnTheRequesterSocket(aRequestFrom("@", ""))

	if actorEmail := service.actorEmailAllowingAssertedRequester(request); actorEmail != "" {
		t.Fatalf("a requester socket caller with no header became %q", actorEmail)
	}
}
