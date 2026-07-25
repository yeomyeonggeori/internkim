package admind

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	calendarTokenEncryptionKeyFileName  = "calendar-token.key"
	calendarSecretsDirectoryEnvironment = "INTERNKIM_CALENDAR_SECRETS_DIR"
	calendarTokenFileExtension          = ".token.enc"
)

type oauthTokenPayload struct {
	AccessToken      string `json:"accessToken"`
	RefreshToken     string `json:"refreshToken"`
	TokenType        string `json:"tokenType"`
	ExpiresAtRFC3339 string `json:"expiresAtRFC3339"`
}

func (service *Service) calendarSecretsDirectory() string {
	override := strings.TrimSpace(os.Getenv(calendarSecretsDirectoryEnvironment))
	if override != "" {
		return override
	}
	return service.Configuration.CalendarSecretsDirectory
}

func (service *Service) calendarTokenFilePath(accountID string) string {
	cleanAccount := sanitizeCalendarSecretComponent(accountID)
	fileName := cleanAccount + calendarTokenFileExtension
	return filepath.Join(service.calendarSecretsDirectory(), fileName)
}

func (service *Service) calendarTokenEncryptionKeyPath() string {
	return filepath.Join(service.calendarSecretsDirectory(), calendarTokenEncryptionKeyFileName)
}

func (service *Service) loadOrCreateCalendarTokenEncryptionKey() ([]byte, error) {
	return loadOrCreateSecretEncryptionKey(service.calendarTokenEncryptionKeyPath())
}

func encryptCalendarTokenPayload(key []byte, payload oauthTokenPayload) ([]byte, error) {
	plaintext, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return nil, errorValue
	}
	return sealSecret(key, plaintext, nil)
}

func decryptCalendarTokenPayload(key []byte, blob []byte) (oauthTokenPayload, error) {
	plaintext, errorValue := openSecret(key, blob, nil)
	if errorValue != nil {
		return oauthTokenPayload{}, errorValue
	}
	var payload oauthTokenPayload
	if errorValue := json.Unmarshal(plaintext, &payload); errorValue != nil {
		return oauthTokenPayload{}, errorValue
	}
	return payload, nil
}

func writeCalendarTokenFile(path string, key []byte, payload oauthTokenPayload) error {
	blob, errorValue := encryptCalendarTokenPayload(key, payload)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return writeFileAtomically(path, blob, 0o600)
}

func readCalendarTokenFile(path string, key []byte) (oauthTokenPayload, error) {
	blob, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return oauthTokenPayload{}, errorValue
	}
	return decryptCalendarTokenPayload(key, blob)
}

func deleteCalendarTokenFile(path string) error {
	if errorValue := os.Remove(path); errorValue != nil {
		if errors.Is(errorValue, os.ErrNotExist) {
			return nil
		}
		return errorValue
	}
	return syncParentDirectory(path)
}

func writeFileAtomically(path string, payload []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	temporary, errorValue := os.CreateTemp(directory, filepath.Base(path)+".tmp-*")
	if errorValue != nil {
		return errorValue
	}
	temporaryPath := temporary.Name()
	cleanup := func() { _ = os.Remove(temporaryPath) }
	if _, errorValue := temporary.Write(payload); errorValue != nil {
		_ = temporary.Close()
		cleanup()
		return errorValue
	}
	if errorValue := temporary.Chmod(mode); errorValue != nil {
		_ = temporary.Close()
		cleanup()
		return errorValue
	}
	if errorValue := temporary.Sync(); errorValue != nil {
		_ = temporary.Close()
		cleanup()
		return errorValue
	}
	if errorValue := temporary.Close(); errorValue != nil {
		cleanup()
		return errorValue
	}
	if errorValue := os.Rename(temporaryPath, path); errorValue != nil {
		cleanup()
		return errorValue
	}
	return syncParentDirectory(path)
}

func renameCalendarTokenFile(sourcePath string, destinationPath string) error {
	if errorValue := os.Rename(sourcePath, destinationPath); errorValue != nil {
		return errorValue
	}
	if errorValue := syncParentDirectory(destinationPath); errorValue != nil {
		rollbackError := os.Rename(destinationPath, sourcePath)
		if rollbackError == nil {
			rollbackError = syncParentDirectory(sourcePath)
		}
		return errors.Join(errorValue, rollbackError)
	}
	if filepath.Dir(sourcePath) == filepath.Dir(destinationPath) {
		return nil
	}
	return syncParentDirectory(sourcePath)
}

func syncParentDirectory(path string) error {
	directory, errorValue := os.Open(filepath.Dir(path))
	if errorValue != nil {
		return errorValue
	}
	defer directory.Close()
	return directory.Sync()
}

func sanitizeCalendarSecretComponent(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "default"
	}
	cleaned := make([]rune, 0, len(trimmed))
	for _, character := range trimmed {
		switch {
		case character >= 'a' && character <= 'z':
			cleaned = append(cleaned, character)
		case character >= 'A' && character <= 'Z':
			cleaned = append(cleaned, character)
		case character >= '0' && character <= '9':
			cleaned = append(cleaned, character)
		case character == '-' || character == '_':
			cleaned = append(cleaned, character)
		default:
			cleaned = append(cleaned, '_')
		}
	}
	if len(cleaned) == 0 {
		return "default"
	}
	return string(cleaned)
}
