package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/eastriver/internkim/internal/deployops"
)

func internkimHomeDir() string {
	home, _ := os.UserHomeDir()
	newDir := filepath.Join(home, ".internkim")
	os.MkdirAll(newDir, 0700)
	return newDir
}

func setupStateDir(baseStateDir string, boardType string) string {
	stateName := setupStateName(boardType)
	if stateName == "" {
		return baseStateDir
	}
	stateDir := filepath.Join(baseStateDir, "devices", stateName)
	_ = os.MkdirAll(stateDir, 0o700)
	copySetupStateHints(baseStateDir, stateDir)
	return stateDir
}

func setupStateName(boardType string) string {
	normalizedBoardType := strings.TrimSpace(boardType)
	if normalizedBoardType == "" {
		return ""
	}
	var builder strings.Builder
	for _, character := range strings.ToLower(normalizedBoardType) {
		switch {
		case character >= 'a' && character <= 'z':
			builder.WriteRune(character)
		case character >= '0' && character <= '9':
			builder.WriteRune(character)
		case character == '-' || character == '_':
			builder.WriteRune(character)
		default:
			builder.WriteRune('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}

func copySetupStateHints(sourceDir string, destinationDir string) {
	for _, key := range []string{
		"subnet",
		"wifi_ssid",
		"wifi_pass",
		"wifi_open",
		"openrouter_api_key",
	} {
		if loadState(destinationDir, key) != "" {
			continue
		}
		if value := loadState(sourceDir, key); value != "" {
			saveState(destinationDir, key, value)
		}
	}
}

func loadOrCreateFleetID(stateDir string) string {
	device, _ := deployops.DeviceTargetFromEnvironment()
	id := firstNonEmptyString(device.FleetID, loadState(stateDir, "fleet_id"))
	if id != "" {
		saveState(stateDir, "fleet_id", id)
		return id
	}
	id = randomFleetID()
	saveState(stateDir, "fleet_id", id)
	return id
}

func loadNodeID(stateDir string) string {
	return loadState(stateDir, "node_id")
}

func loadOrCreateNodeKey(stateDir string) string {
	key := loadState(stateDir, "node_key")
	if key != "" {
		return key
	}
	key = "node-" + randomHexString(8)
	saveState(stateDir, "node_key", key)
	return key
}

func isNumericNodeID(nodeID string) bool {
	trimmedNodeID := strings.TrimSpace(nodeID)
	if trimmedNodeID == "" {
		return false
	}
	for index, character := range trimmedNodeID {
		if character < '0' || character > '9' {
			return false
		}
		if index == 0 && character == '0' {
			return false
		}
	}
	return true
}

func loadOrCreateFleetSecret(stateDir string) string {
	device, _ := deployops.DeviceTargetFromEnvironment()
	secret := firstNonEmptyString(device.FleetSecret, loadState(stateDir, "fleet_secret"))
	if secret != "" {
		saveState(stateDir, "fleet_secret", secret)
		return secret
	}
	secret = randomHexString(32)
	saveState(stateDir, "fleet_secret", secret)
	return secret
}

func resetFleetIdentity(stateDir string) (string, string) {
	fleetID := randomFleetID()
	fleetSecret := randomHexString(32)
	saveState(stateDir, "fleet_id", fleetID)
	saveState(stateDir, "fleet_secret", fleetSecret)
	saveState(stateDir, "device_url", "")
	saveState(stateDir, "tunnel_token", "")
	return fleetID, fleetSecret
}

func randomFleetID() string {
	return randomAlphanumericString(12)
}

func randomAlphanumericString(length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	randomBytes := make([]byte, length)
	if _, randomError := rand.Read(randomBytes); randomError != nil {
		panic(fmt.Sprintf("crypto random failed: %v", randomError))
	}
	var builder strings.Builder
	builder.Grow(length)
	for _, randomByte := range randomBytes {
		builder.WriteByte(alphabet[int(randomByte)%len(alphabet)])
	}
	return builder.String()
}

func randomHexString(byteCount int) string {
	randomBytes := make([]byte, byteCount)
	if _, randomError := rand.Read(randomBytes); randomError != nil {
		panic(fmt.Sprintf("crypto random failed: %v", randomError))
	}
	return hex.EncodeToString(randomBytes)
}

const relayDomainStateKey = "relay_domain"

// relayDomainForTarget is the domain a company chose to reach its own Buzz relay
// on, given once with --relay-domain and remembered for later runs. Empty is the
// ordinary answer and leaves the relay on loopback: reaching a relay from
// outside means owning a domain and pointing a tunnel at it, which belongs to
// the company and cannot be worked out from anything here. See
// blueclaw.RelayPublicHost.
func relayDomainForTarget(stateDir string) string {
	chosen := strings.TrimSpace(argString("--relay-domain", ""))
	if chosen == "" {
		return loadState(stateDir, relayDomainStateKey)
	}
	saveState(stateDir, relayDomainStateKey, chosen)
	return chosen
}

func saveState(dir, key, value string) {
	os.WriteFile(filepath.Join(dir, key), []byte(value), 0600)
}

func loadState(dir, key string) string {
	data, err := os.ReadFile(filepath.Join(dir, key))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
