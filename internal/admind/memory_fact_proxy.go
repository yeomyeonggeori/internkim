package admind

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const memoryAssertionHeader = "X-Blueclaw-Memory-Assertion"
const memoryAssertionLifetime = 30 * time.Second

func (service *Service) blueclawMemoryFactRequest(ctx context.Context, path string, body map[string]any, responseValue any) error {
	if path != "/admin/api/memory/facts/update" && path != "/admin/api/memory/facts/delete" {
		return fmt.Errorf("memory fact path is not allowed: %s", path)
	}
	key := strings.TrimSpace(readTrimmedFile(service.Configuration.CentralPlaneAgentKeyPath))
	if key == "" {
		return fmt.Errorf("central plane agent key is missing")
	}
	payload, errorValue := json.Marshal(body)
	if errorValue != nil {
		return errorValue
	}
	readerPersonID, isString := body["readerPersonID"].(string)
	if !isString || strings.TrimSpace(readerPersonID) == "" {
		return fmt.Errorf("memory fact reader is missing")
	}
	header, errorValue := signMemoryAssertion(payload, readerPersonID, time.Now().Add(memoryAssertionLifetime).Unix(), path, key)
	if errorValue != nil {
		return errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(service.Configuration.BlueclawBaseURL, "/")+path, strings.NewReader(string(payload)))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(memoryAssertionHeader, header)
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("Blueclaw %s %s returned %d: %s", http.MethodPost, path, response.StatusCode, strings.TrimSpace(string(message)))
	}
	if responseValue == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(responseValue)
}

func signMemoryAssertion(payload []byte, readerPersonID string, expiresAt int64, path string, key string) (string, error) {
	return signRequestAssertion(http.MethodPost, path, payload, readerPersonID, expiresAt, key)
}

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
