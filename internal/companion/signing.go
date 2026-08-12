package companion

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"
)

const (
	SignatureHeader = "X-INTERNKIM-COMPANION-SIGNATURE"
	TimestampHeader = "X-INTERNKIM-COMPANION-TIMESTAMP"
)

type KeyPair struct {
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
}

func GenerateKeyPair() (KeyPair, error) {
	publicKey, privateKey, errorValue := ed25519.GenerateKey(rand.Reader)
	if errorValue != nil {
		return KeyPair{}, errorValue
	}
	return KeyPair{
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey),
	}, nil
}

func SignRequest(request *http.Request, body []byte, privateKeyDocument string) error {
	privateKey, errorValue := decodePrivateKey(privateKeyDocument)
	if errorValue != nil {
		return errorValue
	}
	timestamp := time.Now().UTC().Format(time.RFC3339)
	signature := ed25519.Sign(privateKey, signingPayload(request.Method, request.URL.RequestURI(), timestamp, body))
	request.Header.Set(TimestampHeader, timestamp)
	request.Header.Set(SignatureHeader, base64.StdEncoding.EncodeToString(signature))
	return nil
}

func VerifyRequestSignature(request *http.Request, body []byte, publicKeyDocument string) bool {
	publicKey, errorValue := decodePublicKey(publicKeyDocument)
	if errorValue != nil {
		return false
	}
	timestamp := strings.TrimSpace(request.Header.Get(TimestampHeader))
	signatureDocument := strings.TrimSpace(request.Header.Get(SignatureHeader))
	if timestamp == "" || signatureDocument == "" {
		return false
	}
	parsedTime, errorValue := time.Parse(time.RFC3339, timestamp)
	if errorValue != nil || time.Since(parsedTime) > 5*time.Minute || time.Until(parsedTime) > 5*time.Minute {
		return false
	}
	signature, errorValue := base64.StdEncoding.DecodeString(signatureDocument)
	if errorValue != nil {
		return false
	}
	return ed25519.Verify(publicKey, signingPayload(request.Method, request.URL.RequestURI(), timestamp, body), signature)
}

func signingPayload(method string, requestURI string, timestamp string, body []byte) []byte {
	hash := sha256.Sum256(body)
	return []byte(strings.ToUpper(method) + "\n" + requestURI + "\n" + timestamp + "\n" + base64.StdEncoding.EncodeToString(hash[:]))
}

func decodePublicKey(document string) (ed25519.PublicKey, error) {
	key, errorValue := base64.StdEncoding.DecodeString(strings.TrimSpace(document))
	if errorValue != nil {
		return nil, errorValue
	}
	if len(key) != ed25519.PublicKeySize {
		return nil, errors.New("invalid public key")
	}
	return ed25519.PublicKey(key), nil
}

func decodePrivateKey(document string) (ed25519.PrivateKey, error) {
	key, errorValue := base64.StdEncoding.DecodeString(strings.TrimSpace(document))
	if errorValue != nil {
		return nil, errorValue
	}
	if len(key) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid private key")
	}
	return ed25519.PrivateKey(key), nil
}
