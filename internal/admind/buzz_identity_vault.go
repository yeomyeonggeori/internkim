package admind

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	buzzIdentityVaultDirectoryName  = "buzz-identities"
	buzzIdentityVaultKeyFileName    = "buzz-identity.key"
	buzzIdentitySecretFileExtension = ".secret.enc"
	buzzIdentitySecretHexLength     = 64
	buzzIdentityNameDomain          = "buzz-identity-name-v1"
)

var errBuzzIdentitySecretMissing = errors.New("buzz identity secret is not stored")

func (service *Service) buzzIdentityVaultDirectory() string {
	return filepath.Join(service.Configuration.StateDirectory, buzzIdentityVaultDirectoryName)
}

func (service *Service) buzzIdentityVaultKeyPath() string {
	return filepath.Join(service.buzzIdentityVaultDirectory(), buzzIdentityVaultKeyFileName)
}

func normalizedIdentityEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (service *Service) buzzIdentitySecretPath(encryptionKey []byte, email string) string {
	nameMAC := hmac.New(sha256.New, encryptionKey)
	nameMAC.Write([]byte(buzzIdentityNameDomain))
	nameMAC.Write([]byte(normalizedIdentityEmail(email)))
	return filepath.Join(service.buzzIdentityVaultDirectory(), hex.EncodeToString(nameMAC.Sum(nil))+buzzIdentitySecretFileExtension)
}

func (service *Service) storeBuzzIdentitySecret(email string, secretHex string) error {
	normalizedEmail := normalizedIdentityEmail(email)
	secretHex = strings.ToLower(strings.TrimSpace(secretHex))
	if normalizedEmail == "" {
		return errors.New("identity email is required")
	}
	if len(secretHex) != buzzIdentitySecretHexLength {
		return fmt.Errorf("identity secret must be %d hex characters", buzzIdentitySecretHexLength)
	}
	if _, errorValue := hex.DecodeString(secretHex); errorValue != nil {
		return errorValue
	}
	encryptionKey, errorValue := loadOrCreateSecretEncryptionKey(service.buzzIdentityVaultKeyPath())
	if errorValue != nil {
		return errorValue
	}
	sealed, errorValue := sealSecret(encryptionKey, []byte(secretHex), []byte(normalizedEmail))
	if errorValue != nil {
		return errorValue
	}
	return writeFileAtomically(service.buzzIdentitySecretPath(encryptionKey, normalizedEmail), sealed, 0o600)
}

func (service *Service) readBuzzIdentitySecret(email string) (string, error) {
	normalizedEmail := normalizedIdentityEmail(email)
	encryptionKey, errorValue := loadOrCreateSecretEncryptionKey(service.buzzIdentityVaultKeyPath())
	if errorValue != nil {
		return "", errorValue
	}
	sealed, errorValue := os.ReadFile(service.buzzIdentitySecretPath(encryptionKey, normalizedEmail))
	if errors.Is(errorValue, os.ErrNotExist) {
		return "", errBuzzIdentitySecretMissing
	}
	if errorValue != nil {
		return "", errorValue
	}
	plaintext, errorValue := openSecret(encryptionKey, sealed, []byte(normalizedEmail))
	if errorValue != nil {
		return "", errorValue
	}
	return string(plaintext), nil
}

func (service *Service) hasBuzzIdentitySecret(email string) bool {
	encryptionKey, errorValue := loadOrCreateSecretEncryptionKey(service.buzzIdentityVaultKeyPath())
	if errorValue != nil {
		return false
	}
	_, errorValue = os.Stat(service.buzzIdentitySecretPath(encryptionKey, email))
	return errorValue == nil
}

func (service *Service) deleteBuzzIdentitySecret(email string) error {
	encryptionKey, errorValue := loadOrCreateSecretEncryptionKey(service.buzzIdentityVaultKeyPath())
	if errorValue != nil {
		return errorValue
	}
	errorValue = os.Remove(service.buzzIdentitySecretPath(encryptionKey, email))
	if errorValue != nil && !errors.Is(errorValue, os.ErrNotExist) {
		return errorValue
	}
	return nil
}
