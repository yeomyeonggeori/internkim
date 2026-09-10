package admind

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"time"
)

const memoryAssertionHeader = "X-Blueclaw-Memory-Assertion"
const memoryAssertionLifetime = 30 * time.Second

func signRequestAssertion(method string, path string, payload []byte, readerPersonID string, expiresAt int64, key string) (string, error) {
	bodyHash := sha256.Sum256(payload)
	assertion, errorValue := json.Marshal(struct {
		ReaderPersonID string `json:"readerPersonID"`
		ExpiresAt      int64  `json:"expiresAt"`
		BodySHA256     string `json:"bodySHA256"`
	}{readerPersonID, expiresAt, hex.EncodeToString(bodyHash[:])})
	if errorValue != nil {
		return "", errorValue
	}
	assertion64 := base64.RawURLEncoding.EncodeToString(assertion)
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(method + "\n" + path + "\n" + assertion64))
	return assertion64 + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
