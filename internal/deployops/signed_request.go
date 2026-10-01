package deployops

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	if target.FleetID == "" || target.FleetSecret == "" {
		return targetFleetIdentity{}, fmt.Errorf("the vault names no %s and %s for %s", FleetIDVariable, FleetSecretVariable, target.Name)
	}
	return targetFleetIdentity{DeviceID: target.FleetID, Secret: target.FleetSecret}, nil
}

func randomSignedRequestNonce() string {
	document := make([]byte, 16)
	rand.Read(document)
	return hex.EncodeToString(document)
}

func signAdminRequest(secret string, action string, deviceID string, nonce string, timestamp string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.Join([]string{action, deviceID, nonce, timestamp}, "\n")))
	return hex.EncodeToString(mac.Sum(nil))
}
