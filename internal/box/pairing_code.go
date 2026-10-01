package box

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const pairingCodeFileName = "pairing-code.json"

type PairingCode struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (code PairingCode) isLiveAt(now time.Time) bool {
	return code.Code != "" && now.Before(code.ExpiresAt)
}

func ShownPairingCode(stateDirectoryPath string, now time.Time) (PairingCode, bool, error) {
	document, errorValue := os.ReadFile(filepath.Join(stateDirectoryPath, pairingCodeFileName))
	if errors.Is(errorValue, os.ErrNotExist) {
		return PairingCode{}, false, nil
	}
	if errorValue != nil {
		return PairingCode{}, false, errorValue
	}
	var code PairingCode
	if errorValue := json.Unmarshal(document, &code); errorValue != nil {
		return PairingCode{}, false, fmt.Errorf("the pairing code at %s is not one this box wrote: %w", stateDirectoryPath, errorValue)
	}
	return code, code.isLiveAt(now), nil
}

func showPairingCode(stateDirectoryPath string, code PairingCode) error {
	document, errorValue := json.Marshal(code)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(stateDirectoryPath, 0o700); errorValue != nil {
		return errorValue
	}
	return writeFileAtomically(filepath.Join(stateDirectoryPath, pairingCodeFileName), document, 0o600)
}

func forgetPairingCode(stateDirectoryPath string) error {
	errorValue := os.Remove(filepath.Join(stateDirectoryPath, pairingCodeFileName))
	if errors.Is(errorValue, os.ErrNotExist) {
		return nil
	}
	return errorValue
}
