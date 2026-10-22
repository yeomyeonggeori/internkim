package admind

import (
	"context"
	"encoding/hex"
	"testing"
)

func TestAMemberTheRelayCannotNameKeepsTheNextPassClose(t *testing.T) {
	service, _ := serviceNamingTheSampleWithNoOpenRoom(t)
	ctx := context.Background()

	if !service.anyoneTheRelayCannotNameYet(ctx) {
		t.Fatal("a member the directory names and the relay does not must keep the next membership pass close")
	}

	pubkey, errorValue := buzzPublicKey(service.buzzSecretForEmail(ctx, "sample@example.com"))
	if errorValue != nil {
		t.Fatalf("sample pubkey: %v", errorValue)
	}
	pubkeyBytes, errorValue := hex.DecodeString(pubkey)
	if errorValue != nil {
		t.Fatalf("decode sample pubkey: %v", errorValue)
	}
	relay, errorValue := service.buzzDatabase()
	if errorValue != nil {
		t.Fatalf("relay database: %v", errorValue)
	}
	if _, errorValue := relay.Exec(`INSERT INTO events (pubkey, kind, content, created_at) VALUES ($1, 0, '{"name":"이샘플"}', 1)`, pubkeyBytes); errorValue != nil {
		t.Fatalf("hold the sample's profile: %v", errorValue)
	}

	if service.anyoneTheRelayCannotNameYet(ctx) {
		t.Fatal("once the relay names everyone the directory names, nobody keeps the next pass close")
	}
}
