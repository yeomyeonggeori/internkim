package admind

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	cloudflareAccessJwtHeader = "Cf-Access-Jwt-Assertion"
	cloudflareAccessJwksTTL   = 15 * time.Minute
	cloudflareAccessClockSkew = 60 * time.Second
)

type cloudflareAccessKey struct {
	KeyID     string `json:"kid"`
	Algorithm string `json:"alg"`
	Modulus   string `json:"n"`
	Exponent  string `json:"e"`
}

type cloudflareAccessKeySet struct {
	Keys []cloudflareAccessKey `json:"keys"`
}

type cloudflareAccessClaims struct {
	Issuer    string `json:"iss"`
	Email     string `json:"email"`
	Audience  audienceList
	ExpiresAt int64 `json:"exp"`
	NotBefore int64 `json:"nbf"`
}

type audienceList []string

func (list *audienceList) UnmarshalJSON(document []byte) error {
	var single string
	if json.Unmarshal(document, &single) == nil {
		*list = audienceList{single}
		return nil
	}
	var many []string
	if json.Unmarshal(document, &many) == nil {
		*list = many
		return nil
	}
	return errors.New("audience is neither a string nor an array")
}

type cloudflareAccessVerifier struct {
	teamCertsURL     string
	allowedAudiences map[string]bool
	expectedIssuer   string
	httpClient       *http.Client

	mutex     sync.Mutex
	keysByID  map[string]*rsa.PublicKey
	fetchedAt time.Time
}

func newCloudflareAccessVerifier(teamDomain string, audiences []string, httpClient *http.Client) *cloudflareAccessVerifier {
	domain := strings.TrimSuffix(strings.TrimSpace(teamDomain), "/")
	allowed := make(map[string]bool, len(audiences))
	for _, audience := range audiences {
		if trimmed := strings.TrimSpace(audience); trimmed != "" {
			allowed[trimmed] = true
		}
	}
	return &cloudflareAccessVerifier{
		teamCertsURL:     "https://" + domain + "/cdn-cgi/access/certs",
		allowedAudiences: allowed,
		expectedIssuer:   "https://" + domain,
		httpClient:       httpClient,
	}
}

func (verifier *cloudflareAccessVerifier) isConfigured() bool {
	return strings.Contains(verifier.teamCertsURL, ".") && len(verifier.allowedAudiences) > 0
}

// verifiedEmail validates the Cloudflare Access JWT the edge injects and returns
// the authenticated email. It checks the RS256 signature against the team's
// published keys, the audience against the configured application, the issuer,
// and the expiry. An absent or invalid token yields an empty email.
func (verifier *cloudflareAccessVerifier) verifiedEmail(ctx context.Context, request *http.Request) string {
	if !verifier.isConfigured() {
		return ""
	}
	token := strings.TrimSpace(request.Header.Get(cloudflareAccessJwtHeader))
	if token == "" {
		return ""
	}
	claims, errorValue := verifier.verifyToken(ctx, token)
	if errorValue != nil {
		logAuditEvent("cloudflare access jwt rejected: " + errorValue.Error())
		return ""
	}
	return strings.ToLower(strings.TrimSpace(claims.Email))
}

func (verifier *cloudflareAccessVerifier) verifyToken(ctx context.Context, token string) (cloudflareAccessClaims, error) {
	headerSegment, payloadSegment, signatureSegment, errorValue := splitJWT(token)
	if errorValue != nil {
		return cloudflareAccessClaims{}, errorValue
	}
	keyID, errorValue := jwtKeyID(headerSegment)
	if errorValue != nil {
		return cloudflareAccessClaims{}, errorValue
	}
	publicKey, errorValue := verifier.publicKey(ctx, keyID)
	if errorValue != nil {
		return cloudflareAccessClaims{}, errorValue
	}
	if errorValue := verifyRS256(publicKey, headerSegment+"."+payloadSegment, signatureSegment); errorValue != nil {
		return cloudflareAccessClaims{}, errorValue
	}
	claims, errorValue := decodeAccessClaims(payloadSegment)
	if errorValue != nil {
		return cloudflareAccessClaims{}, errorValue
	}
	if errorValue := verifier.validateClaims(claims); errorValue != nil {
		return cloudflareAccessClaims{}, errorValue
	}
	return claims, nil
}

func (verifier *cloudflareAccessVerifier) validateClaims(claims cloudflareAccessClaims) error {
	now := time.Now()
	if claims.ExpiresAt != 0 && now.After(time.Unix(claims.ExpiresAt, 0).Add(cloudflareAccessClockSkew)) {
		return errors.New("token expired")
	}
	if claims.NotBefore != 0 && now.Before(time.Unix(claims.NotBefore, 0).Add(-cloudflareAccessClockSkew)) {
		return errors.New("token not yet valid")
	}
	if claims.Issuer != verifier.expectedIssuer {
		return errors.New("unexpected issuer")
	}
	if strings.TrimSpace(claims.Email) == "" {
		return errors.New("token has no email")
	}
	for _, audience := range claims.Audience {
		if verifier.allowedAudiences[audience] {
			return nil
		}
	}
	return errors.New("audience not allowed")
}

func (verifier *cloudflareAccessVerifier) publicKey(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	verifier.mutex.Lock()
	fresh := time.Since(verifier.fetchedAt) < cloudflareAccessJwksTTL
	key := verifier.keysByID[keyID]
	verifier.mutex.Unlock()
	if fresh && key != nil {
		return key, nil
	}
	if errorValue := verifier.refreshKeys(ctx); errorValue != nil {
		return nil, errorValue
	}
	verifier.mutex.Lock()
	defer verifier.mutex.Unlock()
	if key := verifier.keysByID[keyID]; key != nil {
		return key, nil
	}
	return nil, errors.New("no matching signing key")
}

func (verifier *cloudflareAccessVerifier) refreshKeys(ctx context.Context) error {
	requestContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	httpRequest, errorValue := http.NewRequestWithContext(requestContext, http.MethodGet, verifier.teamCertsURL, nil)
	if errorValue != nil {
		return errorValue
	}
	response, errorValue := verifier.httpClient.Do(httpRequest)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("certs endpoint returned " + response.Status)
	}
	var keySet cloudflareAccessKeySet
	if errorValue := json.NewDecoder(response.Body).Decode(&keySet); errorValue != nil {
		return errorValue
	}
	parsed := make(map[string]*rsa.PublicKey, len(keySet.Keys))
	for _, key := range keySet.Keys {
		publicKey, errorValue := rsaPublicKeyFromJWK(key)
		if errorValue != nil {
			continue
		}
		parsed[key.KeyID] = publicKey
	}
	if len(parsed) == 0 {
		return errors.New("no usable signing keys")
	}
	verifier.mutex.Lock()
	verifier.keysByID = parsed
	verifier.fetchedAt = time.Now()
	verifier.mutex.Unlock()
	return nil
}

func splitJWT(token string) (string, string, string, error) {
	segments := strings.Split(token, ".")
	if len(segments) != 3 || segments[0] == "" || segments[1] == "" || segments[2] == "" {
		return "", "", "", errors.New("malformed token")
	}
	return segments[0], segments[1], segments[2], nil
}

func jwtKeyID(headerSegment string) (string, error) {
	document, errorValue := base64.RawURLEncoding.DecodeString(headerSegment)
	if errorValue != nil {
		return "", errors.New("bad header encoding")
	}
	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
	}
	if errorValue := json.Unmarshal(document, &header); errorValue != nil {
		return "", errors.New("bad header")
	}
	if header.Algorithm != "RS256" {
		return "", errors.New("unsupported algorithm")
	}
	if strings.TrimSpace(header.KeyID) == "" {
		return "", errors.New("missing key id")
	}
	return header.KeyID, nil
}

func verifyRS256(publicKey *rsa.PublicKey, signingInput string, signatureSegment string) error {
	signature, errorValue := base64.RawURLEncoding.DecodeString(signatureSegment)
	if errorValue != nil {
		return errors.New("bad signature encoding")
	}
	digest := sha256.Sum256([]byte(signingInput))
	return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature)
}

func decodeAccessClaims(payloadSegment string) (cloudflareAccessClaims, error) {
	document, errorValue := base64.RawURLEncoding.DecodeString(payloadSegment)
	if errorValue != nil {
		return cloudflareAccessClaims{}, errors.New("bad payload encoding")
	}
	var raw struct {
		Issuer    string       `json:"iss"`
		Email     string       `json:"email"`
		Audience  audienceList `json:"aud"`
		ExpiresAt int64        `json:"exp"`
		NotBefore int64        `json:"nbf"`
	}
	if errorValue := json.Unmarshal(document, &raw); errorValue != nil {
		return cloudflareAccessClaims{}, errors.New("bad payload")
	}
	return cloudflareAccessClaims{
		Issuer:    raw.Issuer,
		Email:     raw.Email,
		Audience:  raw.Audience,
		ExpiresAt: raw.ExpiresAt,
		NotBefore: raw.NotBefore,
	}, nil
}

func rsaPublicKeyFromJWK(key cloudflareAccessKey) (*rsa.PublicKey, error) {
	if !strings.EqualFold(key.Algorithm, "RS256") {
		return nil, errors.New("unsupported key algorithm")
	}
	modulusBytes, errorValue := base64.RawURLEncoding.DecodeString(key.Modulus)
	if errorValue != nil {
		return nil, errorValue
	}
	exponentBytes, errorValue := base64.RawURLEncoding.DecodeString(key.Exponent)
	if errorValue != nil {
		return nil, errorValue
	}
	exponent := 0
	for _, part := range exponentBytes {
		exponent = exponent<<8 | int(part)
	}
	if exponent == 0 {
		exponent = int(binary.BigEndian.Uint32(leftPad(exponentBytes, 4)))
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulusBytes),
		E: exponent,
	}, nil
}

func leftPad(value []byte, size int) []byte {
	if len(value) >= size {
		return value
	}
	padded := make([]byte, size)
	copy(padded[size-len(value):], value)
	return padded
}
