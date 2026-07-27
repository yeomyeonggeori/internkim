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

func normalizedVaultSubject(subject string) string {
	return strings.ToLower(strings.TrimSpace(subject))
}

func (service *Service) buzzIdentitySecretPath(encryptionKey []byte, subject string) string {
	nameMAC := hmac.New(sha256.New, encryptionKey)
	nameMAC.Write([]byte(buzzIdentityNameDomain))
	nameMAC.Write([]byte(normalizedVaultSubject(subject)))
	return filepath.Join(service.buzzIdentityVaultDirectory(), hex.EncodeToString(nameMAC.Sum(nil))+buzzIdentitySecretFileExtension)
}

func (service *Service) storeBuzzIdentitySecret(subject string, secretHex string) error {
	normalizedSubject := normalizedVaultSubject(subject)
	secretHex = strings.ToLower(strings.TrimSpace(secretHex))
	if normalizedSubject == "" {
		return errors.New("identity subject is required")
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
	sealed, errorValue := sealSecret(encryptionKey, []byte(secretHex), []byte(normalizedSubject))
	if errorValue != nil {
		return errorValue
	}
	return writeFileAtomically(service.buzzIdentitySecretPath(encryptionKey, normalizedSubject), sealed, 0o600)
}

func (service *Service) readBuzzIdentitySecret(subject string) (string, error) {
	normalizedSubject := normalizedVaultSubject(subject)
	encryptionKey, errorValue := loadOrCreateSecretEncryptionKey(service.buzzIdentityVaultKeyPath())
	if errorValue != nil {
		return "", errorValue
	}
	sealed, errorValue := os.ReadFile(service.buzzIdentitySecretPath(encryptionKey, normalizedSubject))
	if errors.Is(errorValue, os.ErrNotExist) {
		return "", errBuzzIdentitySecretMissing
	}
	if errorValue != nil {
		return "", errorValue
	}
	plaintext, errorValue := openSecret(encryptionKey, sealed, []byte(normalizedSubject))
	if errorValue != nil {
		return "", errorValue
	}
	return string(plaintext), nil
}

func (service *Service) hasBuzzIdentitySecret(subject string) bool {
	encryptionKey, errorValue := loadOrCreateSecretEncryptionKey(service.buzzIdentityVaultKeyPath())
	if errorValue != nil {
		return false
	}
	_, errorValue = os.Stat(service.buzzIdentitySecretPath(encryptionKey, subject))
	return errorValue == nil
}

func (service *Service) deleteBuzzIdentitySecret(subject string) error {
	encryptionKey, errorValue := loadOrCreateSecretEncryptionKey(service.buzzIdentityVaultKeyPath())
	if errorValue != nil {
		return errorValue
	}
	errorValue = os.Remove(service.buzzIdentitySecretPath(encryptionKey, subject))
	if errorValue != nil && !errors.Is(errorValue, os.ErrNotExist) {
		return errorValue
	}
	return nil
}
