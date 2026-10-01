package box

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type sealingVector struct {
	ModelKey         string         `json:"modelKey"`
	BoxSecretKey     string         `json:"boxSecretKey"`
	BoxEncryptionKey string         `json:"boxEncryptionKey"`
	Sealed           SealedModelKey `json:"sealed"`
}

func readSealingVector(t *testing.T) sealingVector {
	t.Helper()
	document, errorValue := os.ReadFile(filepath.Join("testdata", "legacy-sealed-model-key.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var vector sealingVector
	if errorValue := json.Unmarshal(document, &vector); errorValue != nil {
		t.Fatal(errorValue)
	}
	return vector
}

func identityWithEncryptionSeed(t *testing.T, encoded string) Identity {
	t.Helper()
	seed, errorValue := decodedKey(encoded)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	encryptionKey, errorValue := ecdh.X25519().NewPrivateKey(seed)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return Identity{signingKey: ed25519.NewKeyFromSeed(make([]byte, seedLength)), encryptionKey: encryptionKey}
}

func TestBoxStillOpensAModelKeySealedBeforeHPKE(t *testing.T) {
	vector := readSealingVector(t)
	identity := identityWithEncryptionSeed(t, vector.BoxSecretKey)
	if identity.EncryptionPublicKey() != vector.BoxEncryptionKey {
		t.Fatalf("encryption key = %s, the browser sealed to %s", identity.EncryptionPublicKey(), vector.BoxEncryptionKey)
	}

	modelKey, errorValue := identity.OpenModelKey(vector.Sealed, sampleCompanyID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if modelKey != vector.ModelKey {
		t.Fatalf("opened %q, sealed %q", modelKey, vector.ModelKey)
	}
}

func TestAnotherBoxCannotOpenAModelKeySealedBeforeHPKE(t *testing.T) {
	vector := readSealingVector(t)
	otherBox := identityWithEncryptionSeed(t, base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("\x44", seedLength))))

	if _, errorValue := otherBox.OpenModelKey(vector.Sealed, sampleCompanyID); errorValue == nil {
		t.Fatal("a box the key was not sealed to opened it")
	}
}

func TestBoxKeepsOneIdentityAcrossRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "box", "identity.json")
	first, errorValue := LoadOrCreateIdentity(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	second, errorValue := LoadOrCreateIdentity(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if first.PublicKey() != second.PublicKey() || first.EncryptionPublicKey() != second.EncryptionPublicKey() {
		t.Fatal("a restart gave the box another identity")
	}
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if information.Mode().Perm() != 0o600 {
		t.Fatalf("identity mode = %o, only its owner may read it", information.Mode().Perm())
	}
}

func TestBoxRefusesAnIdentityItDidNotWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")
	if errorValue := os.WriteFile(path, []byte(`{"signingSeed":"short","encryptionKey":"short"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, errorValue := LoadOrCreateIdentity(path); errorValue == nil {
		t.Fatal("a damaged identity was used, or replaced, instead of refused")
	}
}

func TestAssertionIsSignedByTheKeyItNames(t *testing.T) {
	identity, errorValue := freshIdentity()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	now := time.Unix(1_790_000_000, 0)
	parts := strings.Split(identity.Assertion(now), ".")
	if len(parts) != 3 {
		t.Fatalf("assertion has %d parts", len(parts))
	}

	var claims assertionClaims
	decodeSegment(t, parts[1], &claims)
	if claims.Issuer != identity.PublicKey() || claims.Audience != assertionAudience {
		t.Fatalf("claims = %+v", claims)
	}
	if claims.IssuedAt != now.Unix() || claims.ExpiresAt != now.Add(assertionLifetime).Unix() {
		t.Fatalf("claims = %+v", claims)
	}
	publicKey, errorValue := base64.RawURLEncoding.DecodeString(claims.Issuer)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	signature, errorValue := base64.RawURLEncoding.DecodeString(parts[2])
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !ed25519.Verify(publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		t.Fatal("the named key does not verify the assertion")
	}
}

func decodeSegment(t *testing.T, segment string, into any) {
	t.Helper()
	document, errorValue := base64.RawURLEncoding.DecodeString(segment)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := json.Unmarshal(document, into); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestAnUnclaimedBoxHasNoSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("session asked for without an assertion")
		}
		http.Error(writer, "this box belongs to no company yet", http.StatusNotFound)
	}))
	defer server.Close()
	identity, errorValue := freshIdentity()
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	_, isClaimed, errorValue := Client{AppURL: server.URL}.Session(context.Background(), identity)
	if errorValue != nil || isClaimed {
		t.Fatalf("isClaimed = %v, error = %v", isClaimed, errorValue)
	}
}

func TestAnnouncementNamesTheEncryptionKey(t *testing.T) {
	identity, errorValue := freshIdentity()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&body); errorValue != nil ||
			body["encryptionKey"] != identity.EncryptionPublicKey() || body["wantsPairingCode"] != true {
			t.Errorf("announcement body = %v", body)
		}
		writer.Write([]byte(`{"isClaimed":false,"pairingCode":"ABCD-EFGH","pairingCodeExpiresAt":"2026-10-01T09:15:00.000Z"}`))
	}))
	defer server.Close()

	announcement, errorValue := Client{AppURL: server.URL}.Announce(context.Background(), identity, true)
	if errorValue != nil || announcement.IsClaimed || announcement.PairingCode == nil || announcement.PairingCode.Code != "ABCD-EFGH" {
		t.Fatalf("announcement = %+v, error = %v", announcement, errorValue)
	}
}
