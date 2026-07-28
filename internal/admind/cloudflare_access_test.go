package admind

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func signTestAccessToken(t *testing.T, key *rsa.PrivateKey, keyID string, claims map[string]any) string {
	t.Helper()
	header := map[string]any{"alg": "RS256", "kid": keyID, "typ": "JWT"}
	encode := func(value any) string {
		document, errorValue := json.Marshal(value)
		if errorValue != nil {
			t.Fatalf("marshal: %v", errorValue)
		}
		return base64.RawURLEncoding.EncodeToString(document)
	}
	signingInput := encode(header) + "." + encode(claims)
	digest := sha256.Sum256([]byte(signingInput))
	signature, errorValue := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if errorValue != nil {
		t.Fatalf("sign: %v", errorValue)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func newTestAccessVerifier(t *testing.T, key *rsa.PrivateKey, keyID string, teamDomain string, audience string) *cloudflareAccessVerifier {
	t.Helper()
	jwks := cloudflareAccessKeySet{Keys: []cloudflareAccessKey{{
		KeyID:     keyID,
		Algorithm: "RS256",
		Modulus:   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		Exponent:  base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
	}}}
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(responseWriter).Encode(jwks)
	}))
	t.Cleanup(server.Close)
	verifier := newCloudflareAccessVerifier(teamDomain, []string{audience}, server.Client())
	verifier.teamCertsURL = server.URL
	return verifier
}

func TestCloudflareAccessVerifierAcceptsValidToken(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	verifier := newTestAccessVerifier(t, key, "kid-1", "team.cloudflareaccess.com", "aud-1")
	token := signTestAccessToken(t, key, "kid-1", map[string]any{
		"iss":   "https://team.cloudflareaccess.com",
		"aud":   []string{"aud-1"},
		"email": "Lee@Dawn.kim",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	request := requestWithAccessToken(token)
	if email := verifier.verifiedEmail(context.Background(), request); email != "lee@dawn.kim" {
		t.Fatalf("expected verified lowercased email, got %q", email)
	}
}

func TestCloudflareAccessVerifierRejectsWrongAudience(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	verifier := newTestAccessVerifier(t, key, "kid-1", "team.cloudflareaccess.com", "aud-1")
	token := signTestAccessToken(t, key, "kid-1", map[string]any{
		"iss":   "https://team.cloudflareaccess.com",
		"aud":   []string{"someone-elses-app"},
		"email": "lee@dawn.kim",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	if email := verifier.verifiedEmail(context.Background(), requestWithAccessToken(token)); email != "" {
		t.Fatalf("token for another application must be rejected, got %q", email)
	}
}

func TestCloudflareAccessVerifierRejectsExpiredToken(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	verifier := newTestAccessVerifier(t, key, "kid-1", "team.cloudflareaccess.com", "aud-1")
	token := signTestAccessToken(t, key, "kid-1", map[string]any{
		"iss":   "https://team.cloudflareaccess.com",
		"aud":   []string{"aud-1"},
		"email": "lee@dawn.kim",
		"exp":   time.Now().Add(-time.Hour).Unix(),
	})
	if email := verifier.verifiedEmail(context.Background(), requestWithAccessToken(token)); email != "" {
		t.Fatalf("expired token must be rejected, got %q", email)
	}
}

func TestCloudflareAccessVerifierRejectsForgedSignature(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	attackerKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	verifier := newTestAccessVerifier(t, key, "kid-1", "team.cloudflareaccess.com", "aud-1")
	token := signTestAccessToken(t, attackerKey, "kid-1", map[string]any{
		"iss":   "https://team.cloudflareaccess.com",
		"aud":   []string{"aud-1"},
		"email": "admin@dawn.kim",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	if email := verifier.verifiedEmail(context.Background(), requestWithAccessToken(token)); email != "" {
		t.Fatalf("token signed by an unknown key must be rejected, got %q", email)
	}
}

func TestCloudflareAccessVerifierRejectsWrongIssuer(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	verifier := newTestAccessVerifier(t, key, "kid-1", "team.cloudflareaccess.com", "aud-1")
	token := signTestAccessToken(t, key, "kid-1", map[string]any{
		"iss":   "https://evil.cloudflareaccess.com",
		"aud":   []string{"aud-1"},
		"email": "lee@dawn.kim",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	if email := verifier.verifiedEmail(context.Background(), requestWithAccessToken(token)); email != "" {
		t.Fatalf("token from a different team must be rejected, got %q", email)
	}
}

func TestCloudflareAccessVerifierIgnoresMissingToken(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	verifier := newTestAccessVerifier(t, key, "kid-1", "team.cloudflareaccess.com", "aud-1")
	if email := verifier.verifiedEmail(context.Background(), httptest.NewRequest(http.MethodGet, "/", nil)); email != "" {
		t.Fatalf("absent token must yield no email, got %q", email)
	}
}

func requestWithAccessToken(token string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(cloudflareAccessJwtHeader, token)
	return request
}
