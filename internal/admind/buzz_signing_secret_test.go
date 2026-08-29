package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
)

const buzzSigningSecretTestSeed = "device-identity-seed"

func newBuzzSigningSecretService(t *testing.T) *Service {
	t.Helper()
	service := NewService(Configuration{
		BlueclawBaseURL:            "http://blueclaw.local",
		BlueclawPolicyDeliveryPath: t.TempDir() + "/policy.json",
		BuzzKeySeedPath:            writeTestFile(t, buzzSigningSecretTestSeed),
		FleetIDPath:                t.TempDir() + "/missing-fleet-id",
		FleetSecretPath:            t.TempDir() + "/missing-fleet-secret",
		MattermostBotTokenPath:     t.TempDir() + "/missing-bot-token",
		StateDirectory:             t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "http://blueclaw.local/admin/api/policy" {
			return jsonResponse(http.StatusOK, localUsersPolicyDocument(), nil), nil
		}
		return jsonResponse(http.StatusServiceUnavailable, `{}`, nil), nil
	})}
	return service
}

func signingSecretRequest(t *testing.T, pubkeyHex string, remoteAddress string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/admin/api/buzz/signing-secret",
		strings.NewReader(`{"pubkeyHex":"`+pubkeyHex+`"}`))
	request.RemoteAddr = remoteAddress
	return request
}

func TestBuzzSigningSecretAnswersTheKeyItWasAskedAbout(t *testing.T) {
	service := newBuzzSigningSecretService(t)
	memberSecret, errorValue := service.personBuzzSecret(context.Background(), "member@example.com")
	if errorValue != nil {
		t.Fatalf("derive member secret: %v", errorValue)
	}
	memberPubkey, errorValue := buzzPublicKey(memberSecret)
	if errorValue != nil {
		t.Fatalf("derive member pubkey: %v", errorValue)
	}

	responseRecorder := httptest.NewRecorder()
	service.handleBuzzSigningSecret(responseRecorder, signingSecretRequest(t, memberPubkey, "127.0.0.1:5555"))

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	answeredSecret := decodeSigningSecret(t, responseRecorder.Body.String())
	answeredPubkey, errorValue := buzzPublicKey(answeredSecret)
	if errorValue != nil {
		t.Fatalf("derive answered pubkey: %v", errorValue)
	}
	if answeredPubkey != memberPubkey {
		t.Fatalf("answered a key for %s, asked about %s", answeredPubkey, memberPubkey)
	}
}

func TestBuzzSigningSecretFollowsTheMemberThroughARekey(t *testing.T) {
	service := newBuzzSigningSecretService(t)
	if _, errorValue := service.bumpBuzzIdentityVersion("user-member"); errorValue != nil {
		t.Fatalf("bump identity version: %v", errorValue)
	}
	rekeyedSecret, errorValue := service.personBuzzSecret(context.Background(), "member@example.com")
	if errorValue != nil {
		t.Fatalf("derive rekeyed secret: %v", errorValue)
	}
	if rekeyedSecret == buzzidentity.Secret(buzzSigningSecretTestSeed, "member@example.com") {
		t.Fatal("a rekeyed member must no longer derive from the bare email")
	}
	rekeyedPubkey, errorValue := buzzPublicKey(rekeyedSecret)
	if errorValue != nil {
		t.Fatalf("derive rekeyed pubkey: %v", errorValue)
	}

	responseRecorder := httptest.NewRecorder()
	service.handleBuzzSigningSecret(responseRecorder, signingSecretRequest(t, rekeyedPubkey, "127.0.0.1:5555"))

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if decodeSigningSecret(t, responseRecorder.Body.String()) != rekeyedSecret {
		t.Fatal("a rekeyed member must be answered with the key that member now signs with")
	}
}

func TestBuzzSigningSecretRefusesAKeyNobodySignsWith(t *testing.T) {
	service := newBuzzSigningSecretService(t)

	responseRecorder := httptest.NewRecorder()
	service.handleBuzzSigningSecret(responseRecorder, signingSecretRequest(t, strings.Repeat("ab", 32), "127.0.0.1:5555"))

	if responseRecorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d; body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !strings.Contains(responseRecorder.Body.String(), "signs with that key") {
		t.Fatalf("the refusal must say why: %s", responseRecorder.Body.String())
	}
}

func TestBuzzSigningSecretRefusesARequestFromOffTheDevice(t *testing.T) {
	service := newBuzzSigningSecretService(t)
	memberSecret, errorValue := service.personBuzzSecret(context.Background(), "member@example.com")
	if errorValue != nil {
		t.Fatalf("derive member secret: %v", errorValue)
	}
	memberPubkey, errorValue := buzzPublicKey(memberSecret)
	if errorValue != nil {
		t.Fatalf("derive member pubkey: %v", errorValue)
	}

	responseRecorder := httptest.NewRecorder()
	service.handleBuzzSigningSecret(responseRecorder, signingSecretRequest(t, memberPubkey, "203.0.113.7:5555"))

	if responseRecorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d; body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if strings.Contains(responseRecorder.Body.String(), memberSecret) {
		t.Fatal("a refusal must not carry the secret it refused to hand over")
	}
}

func TestBuzzSigningSecretRejectsAMalformedKey(t *testing.T) {
	service := newBuzzSigningSecretService(t)

	responseRecorder := httptest.NewRecorder()
	service.handleBuzzSigningSecret(responseRecorder, signingSecretRequest(t, "not-a-key", "127.0.0.1:5555"))

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
}

func TestBuzzSigningSecretKeepsTheSecretOutOfTheLog(t *testing.T) {
	service := newBuzzSigningSecretService(t)
	memberSecret, errorValue := service.personBuzzSecret(context.Background(), "member@example.com")
	if errorValue != nil {
		t.Fatalf("derive member secret: %v", errorValue)
	}
	memberPubkey, errorValue := buzzPublicKey(memberSecret)
	if errorValue != nil {
		t.Fatalf("derive member pubkey: %v", errorValue)
	}
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	service.handleBuzzSigningSecret(httptest.NewRecorder(), signingSecretRequest(t, memberPubkey, "127.0.0.1:5555"))

	if strings.Contains(logged.String(), memberSecret) {
		t.Fatal("the signing secret must never be written to the log")
	}
	if strings.Contains(logged.String(), buzzSigningSecretTestSeed) {
		t.Fatal("the identity seed must never be written to the log")
	}
}

func decodeSigningSecret(t *testing.T, body string) string {
	t.Helper()
	var answer buzzSigningSecretResponse
	if errorValue := json.Unmarshal([]byte(body), &answer); errorValue != nil {
		t.Fatalf("decode answer %q: %v", body, errorValue)
	}
	if !isBuzzPubkeyHex(answer.SecretHex) {
		t.Fatalf("answer carried no usable secret: %q", body)
	}
	return answer.SecretHex
}
