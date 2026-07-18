package capabilityprotocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGeneratedProtocolIdentityUsesGeneratedAccessors(t *testing.T) {
	identity := GeneratedProtocolIdentity()
	if errorValue := identity.Validate(); errorValue != nil {
		t.Fatalf("expected generated identity to validate: %v", errorValue)
	}
	if identity.ProtocolVersion != GeneratedProtocolVersion() {
		t.Fatalf("expected generated protocol version %q, got %q", GeneratedProtocolVersion(), identity.ProtocolVersion)
	}
	if identity.AggregateProtocolHash != GeneratedAggregateProtocolHash() {
		t.Fatalf("expected generated aggregate protocol hash %q, got %q", GeneratedAggregateProtocolHash(), identity.AggregateProtocolHash)
	}

	document, errorValue := json.Marshal(RegistryResponse{
		ProtocolIdentity:  identity,
		LocalOnly:         true,
		RoutingCandidates: []string{"device"},
	})
	if errorValue != nil {
		t.Fatalf("expected registry response to encode: %v", errorValue)
	}
	var response map[string]any
	if errorValue := json.Unmarshal(document, &response); errorValue != nil {
		t.Fatalf("expected registry response JSON: %v", errorValue)
	}
	if response["protocolVersion"] != identity.ProtocolVersion || response["aggregateProtocolHash"] != identity.AggregateProtocolHash {
		t.Fatalf("expected top-level protocol identity fields, got %s", document)
	}
}

func TestProtocolIdentityValidationRejectsMissingAndMalformedValues(t *testing.T) {
	for _, identity := range []ProtocolIdentity{
		{},
		{ProtocolVersion: GeneratedProtocolVersion()},
		{ProtocolVersion: GeneratedProtocolVersion(), AggregateProtocolHash: "short"},
		{ProtocolVersion: GeneratedProtocolVersion(), AggregateProtocolHash: strings.Repeat("A", 64)},
		{ProtocolVersion: " " + GeneratedProtocolVersion(), AggregateProtocolHash: GeneratedAggregateProtocolHash()},
		{ProtocolVersion: GeneratedProtocolVersion(), AggregateProtocolHash: GeneratedAggregateProtocolHash() + " "},
	} {
		if errorValue := identity.Validate(); errorValue == nil {
			t.Fatalf("expected invalid identity to fail: %+v", identity)
		}
	}
}
