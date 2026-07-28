package admind

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	nostr "github.com/nbd-wtf/go-nostr"
)

const (
	buzzKeyLoginKind      = 27235
	keyLoginChallengeTTL  = 2 * time.Minute
	keyLoginChallengeType = "buzz-key-login"
)

type keyLoginChallengeResponse struct {
	Challenge string `json:"challenge"`
	ExpiresAt int64  `json:"expiresAt"`
}

type keyLoginRequest struct {
	Email string          `json:"email"`
	Event json.RawMessage `json:"event"`
}

// handleAuthVault returns a person's opaque sealed vault so their browser can
// unlock the signing key with the password or passkey. The blob is useless
// without the factor, so it is served without a session — the returning-login
// flow needs it before any session exists.
func (service *Service) handleAuthVault(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.NotFound(responseWriter, request)
		return
	}
	email := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("email")))
	if email == "" {
		http.Error(responseWriter, "email is required", http.StatusBadRequest)
		return
	}
	subject := service.buzzVaultSubject(request.Context(), email)
	if !isSafeVaultSubject(subject) {
		http.Error(responseWriter, "not found", http.StatusNotFound)
		return
	}
	blob, found, errorValue := service.readBuzzClientVault(subject)
	if errorValue != nil {
		http.Error(responseWriter, "vault_read_failed", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(responseWriter, "not found", http.StatusNotFound)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_, _ = responseWriter.Write(blob)
}

// handleKeyLoginChallenge hands out a short-lived, stateless nonce the browser
// must sign with the unlocked signing key to prove possession.
func (service *Service) handleKeyLoginChallenge(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.NotFound(responseWriter, request)
		return
	}
	challenge, expiresAt, errorValue := service.issueKeyLoginChallenge()
	if errorValue != nil {
		http.Error(responseWriter, "challenge_failed", http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, keyLoginChallengeResponse{Challenge: challenge, ExpiresAt: expiresAt})
}

// handleKeyLogin authenticates a returning member by the signature their browser
// makes with the unlocked Buzz key. The key is a deterministic function of the
// email, so a valid signature over our challenge from the matching public key
// proves the person unlocked their own vault. Cloudflare (signup/email
// verification) is not needed after the first enrollment.
func (service *Service) handleKeyLogin(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	var payload keyLoginRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(payload.Email))
	if email == "" {
		http.Error(responseWriter, "email is required", http.StatusBadRequest)
		return
	}

	var event nostr.Event
	if errorValue := json.Unmarshal(payload.Event, &event); errorValue != nil {
		http.Error(responseWriter, "invalid signature", http.StatusBadRequest)
		return
	}
	if event.Kind != buzzKeyLoginKind {
		http.Error(responseWriter, "invalid signature", http.StatusUnauthorized)
		return
	}
	validSignature, errorValue := event.CheckSignature()
	if errorValue != nil || !validSignature {
		http.Error(responseWriter, "invalid signature", http.StatusUnauthorized)
		return
	}
	if errorValue := service.verifyKeyLoginChallenge(challengeTagValue(event)); errorValue != nil {
		http.Error(responseWriter, "challenge rejected", http.StatusUnauthorized)
		return
	}

	seed := service.buzzKeySeed()
	if seed == "" {
		http.Error(responseWriter, "buzz key seed is not configured", http.StatusNotImplemented)
		return
	}
	subject := service.buzzVaultSubject(request.Context(), email)
	if _, found, _ := service.readBuzzClientVault(subject); !found {
		http.Error(responseWriter, "no identity for this email", http.StatusUnauthorized)
		return
	}
	version := service.buzzIdentityVersion(subject)
	expectedPublicKey, errorValue := buzzPublicKey(buzzKeyForVersion(seed, email, version))
	if errorValue != nil {
		http.Error(responseWriter, "key_login_failed", http.StatusInternalServerError)
		return
	}
	if !strings.EqualFold(event.PubKey, expectedPublicKey) {
		http.Error(responseWriter, "signature does not match this email", http.StatusUnauthorized)
		return
	}
	if !service.isFlowStaffActor(request.Context(), email) {
		http.Error(responseWriter, "account not invited", http.StatusForbidden)
		return
	}
	if errorValue := service.issueWebSessionCookie(responseWriter, request, mattermostUserRecord{Email: email}); errorValue != nil {
		http.Error(responseWriter, "session_failed", http.StatusInternalServerError)
		return
	}
	logAuditEvent("key login success")
	service.writeJSON(responseWriter, map[string]bool{"ok": true})
}

func challengeTagValue(event nostr.Event) string {
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "challenge" {
			return tag[1]
		}
	}
	return ""
}

func (service *Service) issueKeyLoginChallenge() (string, int64, error) {
	key, errorValue := service.webSessionSigningKey()
	if errorValue != nil {
		return "", 0, errorValue
	}
	nonce := make([]byte, 16)
	if _, errorValue := rand.Read(nonce); errorValue != nil {
		return "", 0, errorValue
	}
	expiresAt := time.Now().Add(keyLoginChallengeTTL).Unix()
	payload := hex.EncodeToString(nonce) + ":" + strconv.FormatInt(expiresAt, 10)
	signature := signChallengePayload(key, payload)
	challenge := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + signature
	return challenge, expiresAt, nil
}

func (service *Service) verifyKeyLoginChallenge(challenge string) error {
	if strings.TrimSpace(challenge) == "" {
		return errors.New("missing challenge")
	}
	key, errorValue := service.webSessionSigningKey()
	if errorValue != nil {
		return errorValue
	}
	encodedPayload, signature, found := strings.Cut(challenge, ".")
	if !found {
		return errors.New("malformed challenge")
	}
	payloadBytes, errorValue := base64.RawURLEncoding.DecodeString(encodedPayload)
	if errorValue != nil {
		return errors.New("bad challenge encoding")
	}
	payload := string(payloadBytes)
	if signChallengePayload(key, payload) != signature {
		return errors.New("bad challenge signature")
	}
	_, expiryText, found := strings.Cut(payload, ":")
	if !found {
		return errors.New("malformed challenge payload")
	}
	expiresAt, errorValue := strconv.ParseInt(expiryText, 10, 64)
	if errorValue != nil {
		return errors.New("bad challenge expiry")
	}
	if time.Now().Unix() > expiresAt {
		return errors.New("challenge expired")
	}
	return nil
}

func signChallengePayload(key []byte, payload string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(keyLoginChallengeType))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
