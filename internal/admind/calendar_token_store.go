package admind

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	calendarTokenEncryptionKeyFileName  = "calendar-token.key"
	calendarTokenEncryptionKeyByteSize  = 32
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
	keyPath := service.calendarTokenEncryptionKeyPath()
	existing, errorValue := os.ReadFile(keyPath)
	if errorValue == nil {
		if len(existing) != calendarTokenEncryptionKeyByteSize {
			return nil, fmt.Errorf("calendar token key at %s has wrong size %d", keyPath, len(existing))
		}
		return existing, nil
	}
	if !errors.Is(errorValue, os.ErrNotExist) {
		return nil, errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(keyPath), 0o700); errorValue != nil {
		return nil, errorValue
	}
	generated := make([]byte, calendarTokenEncryptionKeyByteSize)
	if _, errorValue := io.ReadFull(rand.Reader, generated); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := writeFileAtomically(keyPath, generated, 0o600); errorValue != nil {
		return nil, errorValue
	}
	return generated, nil
}

func encryptCalendarTokenPayload(key []byte, payload oauthTokenPayload) ([]byte, error) {
	if len(key) != calendarTokenEncryptionKeyByteSize {
		return nil, fmt.Errorf("calendar token key must be %d bytes, got %d", calendarTokenEncryptionKeyByteSize, len(key))
	}
	plaintext, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return nil, errorValue
	}
	block, errorValue := aes.NewCipher(key)
	if errorValue != nil {
		return nil, errorValue
	}
	aead, errorValue := cipher.NewGCM(block)
	if errorValue != nil {
		return nil, errorValue
	}
	nonce := make([]byte, aead.NonceSize())
	if _, errorValue := io.ReadFull(rand.Reader, nonce); errorValue != nil {
		return nil, errorValue
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}

func decryptCalendarTokenPayload(key []byte, blob []byte) (oauthTokenPayload, error) {
	if len(key) != calendarTokenEncryptionKeyByteSize {
		return oauthTokenPayload{}, fmt.Errorf("calendar token key must be %d bytes, got %d", calendarTokenEncryptionKeyByteSize, len(key))
	}
	block, errorValue := aes.NewCipher(key)
	if errorValue != nil {
		return oauthTokenPayload{}, errorValue
	}
	aead, errorValue := cipher.NewGCM(block)
	if errorValue != nil {
		return oauthTokenPayload{}, errorValue
	}
	nonceSize := aead.NonceSize()
	if len(blob) < nonceSize+1 {
		return oauthTokenPayload{}, errors.New("calendar token blob too short")
	}
	nonce := blob[:nonceSize]
	ciphertext := blob[nonceSize:]
	plaintext, errorValue := aead.Open(nil, nonce, ciphertext, nil)
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
