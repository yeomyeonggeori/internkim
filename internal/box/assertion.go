package box

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"time"
)

const (
	assertionAudience = "internkim-box"
	assertionLifetime = time.Minute
)

type assertionHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

type assertionClaims struct {
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func (identity Identity) Assertion(now time.Time) string {
	header := encodedJSON(assertionHeader{Algorithm: "EdDSA", Type: "JWT"})
	claims := encodedJSON(assertionClaims{
		Issuer:    identity.PublicKey(),
		Audience:  assertionAudience,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(assertionLifetime).Unix(),
	})
	signingInput := header + "." + claims
	signature := ed25519.Sign(identity.signingKey, []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func encodedJSON(value any) string {
	document, errorValue := json.Marshal(value)
	if errorValue != nil {
		panic(errorValue)
	}
	return base64.RawURLEncoding.EncodeToString(document)
}
