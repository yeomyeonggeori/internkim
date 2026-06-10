package deployops

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type signedAdminRequest struct {
	Action    string `json:"action"`
	DeviceID  string `json:"deviceID"`
	Nonce     string `json:"nonce"`
	Timestamp string `json:"timestamp"`
	Signature string `json:"signature"`
}

type targetFleetIdentity struct {
	DeviceID string
	Secret   string
}

func signedAdminRequestPayload(identity targetFleetIdentity, action string) signedAdminRequest {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	nonce := randomSignedRequestNonce()
	return signedAdminRequest{
		Action:    action,
		DeviceID:  identity.DeviceID,
		Nonce:     nonce,
		Timestamp: timestamp,
		Signature: signAdminRequest(identity.Secret, action, identity.DeviceID, nonce, timestamp),
	}
}

func targetIdentity(target Target) (targetFleetIdentity, error) {
	sourcePath := expandedPath(firstNonEmpty(target.SecretSource, target.StatePath))
	if sourcePath == "" {
		return targetFleetIdentity{}, fmt.Errorf("target secret source is not configured")
	}
	fileInfo, errorValue := os.Stat(sourcePath)
	if errorValue != nil {
		return targetFleetIdentity{}, errorValue
	}
	if fileInfo.IsDir() {
		return targetIdentityFromDirectory(sourcePath)
	}
	return targetIdentityFromSecretFile(sourcePath)
}

func targetIdentityFromDirectory(directoryPath string) (targetFleetIdentity, error) {
	return requireTargetIdentity(
		strings.TrimSpace(readStateFile(directoryPath, "fleet_id")),
		strings.TrimSpace(readStateFile(directoryPath, "fleet_secret")),
	)
}

func targetIdentityFromSecretFile(secretPath string) (targetFleetIdentity, error) {
	secretDocument, errorValue := os.ReadFile(secretPath)
	if errorValue != nil {
		return targetFleetIdentity{}, errorValue
	}
	return requireTargetIdentity(
		strings.TrimSpace(readStateFile(filepath.Dir(secretPath), "fleet_id")),
		strings.TrimSpace(string(secretDocument)),
	)
}

func requireTargetIdentity(deviceID string, secret string) (targetFleetIdentity, error) {
	deviceID = strings.TrimSpace(deviceID)
	secret = strings.TrimSpace(secret)
	if deviceID == "" || secret == "" {
		return targetFleetIdentity{}, fmt.Errorf("target fleet id or secret is not configured")
	}
	return targetFleetIdentity{DeviceID: deviceID, Secret: secret}, nil
}

func expandedPath(path string) string {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "~/") {
		homePath, errorValue := os.UserHomeDir()
		if errorValue == nil {
			return filepath.Join(homePath, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func randomSignedRequestNonce() string {
	document := make([]byte, 16)
	if _, errorValue := rand.Read(document); errorValue != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(document)
}

func signAdminRequest(secret string, action string, deviceID string, nonce string, timestamp string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.Join([]string{action, deviceID, nonce, timestamp}, "\n")))
	return hex.EncodeToString(mac.Sum(nil))
}
