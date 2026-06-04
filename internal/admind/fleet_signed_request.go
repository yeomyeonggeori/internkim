package admind

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type fleetSignedRequest struct {
	Action    string `json:"action"`
	DeviceID  string `json:"deviceID"`
	Nonce     string `json:"nonce"`
	Timestamp string `json:"timestamp"`
	Signature string `json:"signature"`
}

const fleetSignedRequestTTL = 5 * time.Minute

func (service *Service) validateFleetSignedRequest(payload fleetSignedRequest, isAllowedAction func(string) bool) error {
	payload.Action = strings.TrimSpace(payload.Action)
	payload.DeviceID = strings.TrimSpace(payload.DeviceID)
	payload.Nonce = strings.TrimSpace(payload.Nonce)
	payload.Timestamp = strings.TrimSpace(payload.Timestamp)
	payload.Signature = strings.TrimSpace(payload.Signature)
	if !isAllowedAction(payload.Action) {
		return fmt.Errorf("unsupported signed action")
	}
	if payload.DeviceID == "" || payload.Nonce == "" || payload.Timestamp == "" || payload.Signature == "" {
		return fmt.Errorf("missing signed request fields")
	}
	if payload.DeviceID != strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)) {
		return fmt.Errorf("signed request device mismatch")
	}
	createdAt, errorValue := time.Parse(time.RFC3339, payload.Timestamp)
	if errorValue != nil {
		return fmt.Errorf("invalid signed request timestamp")
	}
	if time.Since(createdAt) > fleetSignedRequestTTL || time.Until(createdAt) > fleetSignedRequestTTL {
		return fmt.Errorf("stale signed request")
	}
	secret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	if secret == "" {
		return fmt.Errorf("fleet secret is not configured")
	}
	expectedSignature := signFleetPayload(secret, payload.Action, payload.DeviceID, payload.Nonce, payload.Timestamp)
	if !hmac.Equal([]byte(expectedSignature), []byte(payload.Signature)) {
		return fmt.Errorf("invalid signed request signature")
	}
	if errorValue := service.claimFleetSignedNonce(payload.Nonce); errorValue != nil {
		return errorValue
	}
	return nil
}

func (service *Service) claimFleetSignedNonce(nonce string) error {
	if strings.Contains(nonce, "/") || strings.Contains(nonce, "\\") || strings.Contains(nonce, "..") {
		return fmt.Errorf("invalid signed request nonce")
	}
	directoryPath := filepath.Join(service.Configuration.StateDirectory, "signed-request-nonces")
	if errorValue := os.MkdirAll(directoryPath, 0o700); errorValue != nil {
		return fmt.Errorf("failed to prepare signed request nonce store")
	}
	noncePath := filepath.Join(directoryPath, nonce)
	file, errorValue := os.OpenFile(noncePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errorValue != nil {
		if os.IsExist(errorValue) {
			return fmt.Errorf("replayed signed request")
		}
		return fmt.Errorf("failed to record signed request nonce")
	}
	_ = file.Close()
	return nil
}

func signFleetPayload(secret string, action string, deviceID string, nonce string, timestamp string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.Join([]string{action, deviceID, nonce, timestamp}, "\n")))
	return hex.EncodeToString(mac.Sum(nil))
}
