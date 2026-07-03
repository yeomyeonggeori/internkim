package admind

import (
	"crypto/rand"
	"encoding/base64"
)

func generateRandomURLToken(byteCount int) (string, error) {
	buffer := make([]byte, byteCount)
	if _, errorValue := rand.Read(buffer); errorValue != nil {
		return "", errorValue
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
