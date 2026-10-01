package admind

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestForwardedIdentityIsTrustedOnlyBehindALoopbackBinding(t *testing.T) {
	for _, listenAddress := range []string{"127.0.0.1:18080", "localhost:18080", "[::1]:18080"} {
		if !trustsForwardedIdentity(listenAddress) {
			t.Fatalf("loopback binding %q must keep trusting the proxy identity header", listenAddress)
		}
	}
	for _, listenAddress := range []string{"0.0.0.0:18080", "192.168.0.248:18080", ":18080"} {
		if trustsForwardedIdentity(listenAddress) {
			t.Fatalf("routable binding %q must not trust a client-supplied identity header", listenAddress)
		}
	}
}

func TestAuthenticatedCallerEmailIgnoresHeadersOnARoutableBinding(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/admin/api/session", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "attacker@example.test")

	loopbackService := &Service{Configuration: Configuration{ListenAddress: "127.0.0.1:18080"}}
	if email := loopbackService.authenticatedCallerEmail(request); email != "attacker@example.test" {
		t.Fatalf("loopback binding should read the proxy header, got %q", email)
	}

	routableService := &Service{Configuration: Configuration{ListenAddress: "0.0.0.0:18080"}}
	if email := routableService.authenticatedCallerEmail(request); email != "" {
		t.Fatalf("routable binding must not accept a forged identity header, got %q", email)
	}
}

func TestAuthenticatedCallerEmailIgnoresHeadersOnATunneledRequest(t *testing.T) {
	loopbackService := &Service{Configuration: Configuration{ListenAddress: "127.0.0.1:18080"}}
	request := tunneledRequest(http.MethodGet, "/agent/api/channels", "")
	request.Header.Set("Cf-Access-Authenticated-User-Email", "attacker@example.test")
	request.Header.Set("X-Forwarded-Email", "attacker@example.test")
	if email := loopbackService.authenticatedCallerEmail(request); email != "" {
		t.Fatalf("a tunneled request must not name its own caller, got %q", email)
	}
}
