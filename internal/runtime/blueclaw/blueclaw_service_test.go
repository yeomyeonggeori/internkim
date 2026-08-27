package blueclaw

import (
	"strings"
	"testing"
)

// The relay checks a NIP-42 auth event against the url it believes it answers
// at. It used to be installed claiming wss on loopback, where it terminates no
// TLS, so chatd connected over ws, signed ws, and was refused with "relay url
// mismatch" until it gave up. A company without a public host is the default.
func TestARelayWithNoPublicHostNamesTheSchemeItServes(t *testing.T) {
	unit := BuzzRelayServiceUnit("")

	if !strings.Contains(unit, "Environment=RELAY_URL="+BuzzRelayLocalURL) {
		t.Fatalf("expected the loopback url, got:\n%s", unit)
	}
	if strings.Contains(unit, "RELAY_URL=wss://127.0.0.1") {
		t.Fatal("expected no TLS scheme on a loopback relay that terminates none")
	}
}

func TestARelayWithAPublicHostIsNamedByIt(t *testing.T) {
	unit := BuzzRelayServiceUnit(RelayPublicURL("buzz.example.test"))

	if !strings.Contains(unit, "Environment=RELAY_URL=wss://buzz.example.test") {
		t.Fatalf("expected the public url, got:\n%s", unit)
	}
}
