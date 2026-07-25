package admind

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func buzzTestService(t *testing.T) *Service {
	t.Helper()
	stateDirectory := t.TempDir()
	inviteKeyPath := filepath.Join(stateDirectory, "invite-key")
	if errorValue := os.WriteFile(inviteKeyPath, []byte(strings.Repeat("ab", 32)), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	service := NewService(DefaultConfiguration())
	service.Configuration.StateDirectory = stateDirectory
	service.Configuration.BuzzInviteKeyPath = inviteKeyPath
	service.Configuration.BuzzCommunityID = "62f4c6cd-2938-4079-a3bc-376597020816"
	service.Configuration.BuzzRelayURL = "ws://localhost:3000"
	return service
}

func TestMintBuzzInviteProducesVerifiableCode(t *testing.T) {
	service := buzzTestService(t)

	code, expiresAt, errorValue := service.mintBuzzInvite()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	payloadPart, macPart, isSplit := strings.Cut(code, ".")
	if !isSplit {
		t.Fatalf("expected payload.mac code, got %q", code)
	}
	payloadBytes, errorValue := base64.RawURLEncoding.DecodeString(payloadPart)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	macBytes, errorValue := base64.RawURLEncoding.DecodeString(macPart)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	inviteKey, _ := hex.DecodeString(strings.Repeat("ab", 32))
	expectedMac := hmac.New(sha256.New, inviteKey)
	expectedMac.Write(payloadBytes)
	if !hmac.Equal(macBytes, expectedMac.Sum(nil)) {
		t.Fatal("invite MAC does not verify against the derived key")
	}
	var payload struct {
		Community string `json:"c"`
		Role      string `json:"r"`
		ExpiresAt int64  `json:"e"`
		Nonce     string `json:"n"`
	}
	if errorValue := json.Unmarshal(payloadBytes, &payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload.Community != "62f4c6cd-2938-4079-a3bc-376597020816" || payload.Role != "member" || payload.Nonce == "" {
		t.Fatalf("unexpected invite payload: %+v", payload)
	}
	if payload.ExpiresAt != expiresAt.Unix() {
		t.Fatalf("expiry mismatch: %d != %d", payload.ExpiresAt, expiresAt.Unix())
	}
}

func TestParseBuzzMemberList(t *testing.T) {
	output := strings.Join([]string{
		"pubkey                                                             role     added_by                                                           created_at",
		"----",
		strings.Repeat("a", 64) + "   member   -                                                                  2026-07-24T05:25:20Z",
		strings.Repeat("b", 64) + "   member   invite                                                             2026-07-24T06:41:26Z",
		"not-a-row",
	}, "\n")

	records := parseBuzzMemberList(output)

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %+v", records)
	}
	if records[0].AddedBy != "-" || records[1].AddedBy != "invite" {
		t.Fatalf("unexpected addedBy values: %+v", records)
	}
}

func TestSingleOutstandingInviteRequiresExactlyOne(t *testing.T) {
	future := time.Now().UTC().Add(time.Hour)
	past := time.Now().UTC().Add(-time.Hour)

	if invite := singleOutstandingInvite([]buzzInvite{{Email: "a@x", ExpiresAt: future}}); invite == nil || invite.Email != "a@x" {
		t.Fatalf("expected the single outstanding invite, got %+v", invite)
	}
	if invite := singleOutstandingInvite([]buzzInvite{{Email: "a@x", ExpiresAt: future}, {Email: "b@x", ExpiresAt: future}}); invite != nil {
		t.Fatalf("expected ambiguity to fail closed, got %+v", invite)
	}
	if invite := singleOutstandingInvite([]buzzInvite{{Email: "a@x", ExpiresAt: past}, {Email: "b@x", ExpiresAt: future}}); invite == nil || invite.Email != "b@x" {
		t.Fatalf("expected expired invites to be skipped, got %+v", invite)
	}
	if invite := singleOutstandingInvite([]buzzInvite{{Email: "a@x", ExpiresAt: future, ClaimedPubkey: "x"}, {Email: "b@x", ExpiresAt: future}}); invite == nil || invite.Email != "b@x" {
		t.Fatalf("expected claimed invites to be skipped, got %+v", invite)
	}
}

func TestBuzzStorePersistsLinksForAcpd(t *testing.T) {
	service := buzzTestService(t)
	linksPath := filepath.Join(service.Configuration.StateDirectory, "links.json")
	service.Configuration.BuzzAccountLinksPath = linksPath

	store := service.buzzStore()
	store.mutex.Lock()
	store.state.Links[strings.Repeat("c", 64)] = "lee@example.com"
	store.save()
	store.mutex.Unlock()
	service.writeBuzzAccountLinksFile()

	document, errorValue := os.ReadFile(linksPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var links map[string]string
	if errorValue := json.Unmarshal(document, &links); errorValue != nil {
		t.Fatal(errorValue)
	}
	if links[strings.Repeat("c", 64)] != "lee@example.com" {
		t.Fatalf("expected persisted link, got %+v", links)
	}
}

func TestParseBuzzGeneratedIdentity(t *testing.T) {
	output := "Public key:  " + strings.Repeat("a", 64) + "\nSecret key:  " + strings.Repeat("b", 64) + "\n\nSet BUZZ_PRIVATE_KEY to the secret key to use this identity."

	identity, errorValue := parseBuzzGeneratedIdentity(output)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if identity.publicKey != strings.Repeat("a", 64) || identity.secretKey != strings.Repeat("b", 64) {
		t.Fatalf("unexpected identity: %+v", identity)
	}

	if _, errorValue := parseBuzzGeneratedIdentity("no keys here"); errorValue == nil {
		t.Fatal("expected parse failure for missing keys")
	}
}

func TestLinkerPrefersPregeneratedIdentityAndExcludesItFromCorrelation(t *testing.T) {
	future := time.Now().UTC().Add(time.Hour)
	pregenerated := strings.Repeat("e", 64)
	invites := []buzzInvite{
		{Email: "known@x", ExpiresAt: future, IdentityPubkey: pregenerated},
		{Email: "freeform@x", ExpiresAt: future},
	}

	if invite := inviteByIdentityPubkey(invites, pregenerated); invite == nil || invite.Email != "known@x" {
		t.Fatalf("expected pregenerated invite match, got %+v", invite)
	}
	if invite := singleOutstandingInvite(invites); invite == nil || invite.Email != "freeform@x" {
		t.Fatalf("expected pregenerated invites excluded from correlation, got %+v", invite)
	}
}

func TestEncodeBuzzNsecMatchesKnownVector(t *testing.T) {
	nsec, errorValue := encodeBuzzNsec("1178851e7a60684098157ea8fd4ef624c4fd094b41e274194843de1cd39c5aa8")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if nsec != "nsec1z9ug28n6vp5ypxq406506nhkynz06z2tg838gx2gg00pe5uut25qr353t3" {
		t.Fatalf("unexpected nsec encoding: %s", nsec)
	}

	if _, errorValue := encodeBuzzNsec("deadbeef"); errorValue == nil {
		t.Fatal("expected short keys to be rejected")
	}
}
