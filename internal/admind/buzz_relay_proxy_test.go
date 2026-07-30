package admind

import (
	"net/http/httptest"
	"testing"
)

func TestPublicRelayURLPrefersConfiguredPublicURL(t *testing.T) {
	service := &Service{}
	service.Configuration.BuzzRelayURL = "ws://127.0.0.1:3000"
	service.Configuration.BuzzRelayPublicURL = "wss://zd2df6qt6jmc-relay.intern.kim"

	request := httptest.NewRequest("GET", "https://zd2df6qt6jmc.intern.kim/agent/api/buzz-relay-config", nil)
	if actual := service.publicRelayURL(request); actual != "wss://zd2df6qt6jmc-relay.intern.kim" {
		t.Fatalf("publicRelayURL = %q, want the configured public URL", actual)
	}
}

func TestPublicRelayURLFallsBackToRequestHost(t *testing.T) {
	service := &Service{}
	service.Configuration.BuzzRelayURL = "ws://127.0.0.1:3000"

	request := httptest.NewRequest("GET", "https://zd2df6qt6jmc.intern.kim/agent/api/buzz-relay-config", nil)
	if actual := service.publicRelayURL(request); actual != "wss://zd2df6qt6jmc.intern.kim/relay" {
		t.Fatalf("publicRelayURL = %q, want the gateway relay path fallback", actual)
	}
}

func TestBuzzRelayPublicHostStripsScheme(t *testing.T) {
	service := &Service{}
	service.Configuration.BuzzRelayPublicURL = "wss://zd2df6qt6jmc-relay.intern.kim"
	if host := service.buzzRelayPublicHost(); host != "zd2df6qt6jmc-relay.intern.kim" {
		t.Fatalf("buzzRelayPublicHost = %q", host)
	}
	if service.buzzRelayEffectiveURL() != "wss://zd2df6qt6jmc-relay.intern.kim" {
		t.Fatalf("buzzRelayEffectiveURL should return the public URL when set")
	}

	loopbackOnly := &Service{}
	loopbackOnly.Configuration.BuzzRelayURL = "ws://127.0.0.1:3000"
	if loopbackOnly.buzzRelayPublicHost() != "" {
		t.Fatalf("buzzRelayPublicHost must be empty without a public URL")
	}
	if loopbackOnly.buzzRelayEffectiveURL() != "ws://127.0.0.1:3000" {
		t.Fatalf("buzzRelayEffectiveURL should fall back to the loopback URL")
	}
}
