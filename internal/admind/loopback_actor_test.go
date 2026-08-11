package admind

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func aRequestFrom(remoteAddress string, headerEmail string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/files/api/roots", nil)
	request.RemoteAddr = remoteAddress
	if headerEmail != "" {
		request.Header.Set(flowRequesterEmailHeader, headerEmail)
	}
	return request
}

func TestALoopbackCallerNamesThePersonItActsFor(t *testing.T) {
	service := &Service{}

	if actorEmail := service.actorEmailAllowingLoopback(aRequestFrom("127.0.0.1:5000", "Sample@Example.test")); actorEmail != "sample@example.test" {
		t.Fatalf("a loopback caller could not name its person, got %q", actorEmail)
	}
}

func TestSomewhereElseCannotNameAPerson(t *testing.T) {
	service := &Service{}

	for _, remoteAddress := range []string{"203.0.113.9:5000", "192.168.0.50:5000", "[2001:db8::1]:5000"} {
		if actorEmail := service.actorEmailAllowingLoopback(aRequestFrom(remoteAddress, "sample@example.test")); actorEmail != "" {
			t.Errorf("%s named a person it has no right to: %q", remoteAddress, actorEmail)
		}
	}
}

func TestALoopbackCallerNamingNobodyIsNobody(t *testing.T) {
	service := &Service{}

	if actorEmail := service.actorEmailAllowingLoopback(aRequestFrom("127.0.0.1:5000", "")); actorEmail != "" {
		t.Fatalf("a loopback caller with no header became %q", actorEmail)
	}
}
