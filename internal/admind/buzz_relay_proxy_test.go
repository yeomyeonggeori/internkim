package admind

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPublicRelayURLPrefersConfiguredPublicURL(t *testing.T) {
	service := &Service{}
	service.Configuration.BuzzRelayURL = "ws://127.0.0.1:3000"
	service.Configuration.BuzzRelayPublicURL = "wss://example-device-relay.example.test"

	request := httptest.NewRequest("GET", "https://example-device.example.test/agent/api/buzz-relay-config", nil)
	if actual := service.publicRelayURL(request); actual != "wss://example-device-relay.example.test" {
		t.Fatalf("publicRelayURL = %q, want the configured public URL", actual)
	}
}

func TestPublicRelayURLFallsBackToRequestHost(t *testing.T) {
	service := &Service{}
	service.Configuration.BuzzRelayURL = "ws://127.0.0.1:3000"

	request := httptest.NewRequest("GET", "https://example-device.example.test/agent/api/buzz-relay-config", nil)
	if actual := service.publicRelayURL(request); actual != "wss://example-device.example.test/relay" {
		t.Fatalf("publicRelayURL = %q, want the gateway relay path fallback", actual)
	}
}

func TestPublicRelayURLReadsTheDomainProvisioningRecorded(t *testing.T) {
	recorded := filepath.Join(t.TempDir(), "buzz-relay-public-url")
	if errorValue := os.WriteFile(recorded, []byte("wss://relay.example.test"), 0600); errorValue != nil {
		t.Fatalf("write recorded relay URL: %v", errorValue)
	}
	service := &Service{}
	service.Configuration.BuzzRelayURL = "ws://127.0.0.1:3000"
	service.Configuration.BuzzRelayPublicURLPath = recorded

	if host := service.buzzRelayPublicHost(); host != "relay.example.test" {
		t.Fatalf("buzzRelayPublicHost = %q, want the recorded domain", host)
	}

	request := httptest.NewRequest("GET", "https://a-device.example.test/agent/api/buzz-relay-config", nil)
	if actual := service.publicRelayURL(request); actual != "wss://relay.example.test" {
		t.Fatalf("publicRelayURL = %q, want the recorded domain", actual)
	}
}

func TestBuzzRelayPublicHostStripsScheme(t *testing.T) {
	service := &Service{}
	service.Configuration.BuzzRelayPublicURL = "wss://example-device-relay.example.test"
	if host := service.buzzRelayPublicHost(); host != "example-device-relay.example.test" {
		t.Fatalf("buzzRelayPublicHost = %q", host)
	}
	if service.buzzRelayEffectiveURL() != "wss://example-device-relay.example.test" {
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
