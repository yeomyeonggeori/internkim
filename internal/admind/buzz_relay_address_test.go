package admind

import "testing"

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
