package admind

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newIdentityVaultService(t *testing.T) *Service {
	t.Helper()
	return &Service{Configuration: Configuration{StateDirectory: t.TempDir()}}
}

func TestBuzzIdentitySecretRoundTrips(t *testing.T) {
	service := newIdentityVaultService(t)
	secretHex := strings.Repeat("a1", 32)
	if errorValue := service.storeBuzzIdentitySecret("Lee@Dawn.kim", secretHex); errorValue != nil {
		t.Fatalf("store identity secret: %v", errorValue)
	}
	if !service.hasBuzzIdentitySecret("lee@dawn.kim") {
		t.Fatal("expected stored secret to be reported as present")
	}
	stored, errorValue := service.readBuzzIdentitySecret("lee@dawn.kim")
	if errorValue != nil {
		t.Fatalf("read identity secret: %v", errorValue)
	}
	if stored != secretHex {
		t.Fatalf("expected %q, got %q", secretHex, stored)
	}
}

func TestBuzzIdentitySecretIsNotStoredInPlaintext(t *testing.T) {
	service := newIdentityVaultService(t)
	secretHex := strings.Repeat("b2", 32)
	if errorValue := service.storeBuzzIdentitySecret("lee@dawn.kim", secretHex); errorValue != nil {
		t.Fatalf("store identity secret: %v", errorValue)
	}
	encryptionKey, errorValue := loadOrCreateSecretEncryptionKey(service.buzzIdentityVaultKeyPath())
	if errorValue != nil {
		t.Fatalf("load vault key: %v", errorValue)
	}
	sealed, errorValue := os.ReadFile(service.buzzIdentitySecretPath(encryptionKey, "lee@dawn.kim"))
	if errorValue != nil {
		t.Fatalf("read sealed file: %v", errorValue)
	}
	if strings.Contains(string(sealed), secretHex) {
		t.Fatal("sealed file must not contain the plaintext secret")
	}
	fileInfo, errorValue := os.Stat(service.buzzIdentitySecretPath(encryptionKey, "lee@dawn.kim"))
	if errorValue != nil {
		t.Fatalf("stat sealed file: %v", errorValue)
	}
	if mode := fileInfo.Mode().Perm(); mode != 0o600 {
		t.Fatalf("expected 0600 permissions, got %o", mode)
	}
}

func TestBuzzIdentitySecretPathIsNotGuessableFromTheEmailAlone(t *testing.T) {
	service := newIdentityVaultService(t)
	encryptionKey, errorValue := loadOrCreateSecretEncryptionKey(service.buzzIdentityVaultKeyPath())
	if errorValue != nil {
		t.Fatalf("load vault key: %v", errorValue)
	}
	fileName := filepath.Base(service.buzzIdentitySecretPath(encryptionKey, "lee@dawn.kim"))
	if strings.Contains(fileName, "lee") || strings.Contains(fileName, "dawn") {
		t.Fatalf("vault file name must not embed the email, got %q", fileName)
	}
	unkeyedDigest := sha256.Sum256([]byte("lee@dawn.kim"))
	if strings.Contains(fileName, hex.EncodeToString(unkeyedDigest[:])) {
		t.Fatal("vault file name must not be a plain digest of the email, which anyone can precompute")
	}
	otherService := newIdentityVaultService(t)
	otherKey, errorValue := loadOrCreateSecretEncryptionKey(otherService.buzzIdentityVaultKeyPath())
	if errorValue != nil {
		t.Fatalf("load other vault key: %v", errorValue)
	}
	otherFileName := filepath.Base(otherService.buzzIdentitySecretPath(otherKey, "lee@dawn.kim"))
	if fileName == otherFileName {
		t.Fatal("vault file names must differ between devices with different keys")
	}
}

func TestBuzzIdentitySecretRejectsAFileMovedToAnotherPerson(t *testing.T) {
	service := newIdentityVaultService(t)
	secretHex := strings.Repeat("e5", 32)
	if errorValue := service.storeBuzzIdentitySecret("lee@dawn.kim", secretHex); errorValue != nil {
		t.Fatalf("store identity secret: %v", errorValue)
	}
	encryptionKey, errorValue := loadOrCreateSecretEncryptionKey(service.buzzIdentityVaultKeyPath())
	if errorValue != nil {
		t.Fatalf("load vault key: %v", errorValue)
	}
	sealed, errorValue := os.ReadFile(service.buzzIdentitySecretPath(encryptionKey, "lee@dawn.kim"))
	if errorValue != nil {
		t.Fatalf("read sealed file: %v", errorValue)
	}
	victimPath := service.buzzIdentitySecretPath(encryptionKey, "kwak@dawn.kim")
	if errorValue := os.WriteFile(victimPath, sealed, 0o600); errorValue != nil {
		t.Fatalf("plant sealed file: %v", errorValue)
	}
	if _, errorValue := service.readBuzzIdentitySecret("kwak@dawn.kim"); errorValue == nil {
		t.Fatal("a secret sealed for one person must not open under another person")
	}
}

func TestBuzzIdentitySecretRejectsMalformedInput(t *testing.T) {
	service := newIdentityVaultService(t)
	if errorValue := service.storeBuzzIdentitySecret("lee@dawn.kim", "not-hex"); errorValue == nil {
		t.Fatal("expected malformed secret to be rejected")
	}
	if errorValue := service.storeBuzzIdentitySecret("", strings.Repeat("c3", 32)); errorValue == nil {
		t.Fatal("expected missing email to be rejected")
	}
}

func TestBuzzIdentitySecretMissingIsDistinguishable(t *testing.T) {
	service := newIdentityVaultService(t)
	if _, errorValue := service.readBuzzIdentitySecret("nobody@dawn.kim"); errorValue != errBuzzIdentitySecretMissing {
		t.Fatalf("expected missing-secret error, got %v", errorValue)
	}
}

func TestBuzzIdentitySecretDeletes(t *testing.T) {
	service := newIdentityVaultService(t)
	secretHex := strings.Repeat("d4", 32)
	if errorValue := service.storeBuzzIdentitySecret("lee@dawn.kim", secretHex); errorValue != nil {
		t.Fatalf("store identity secret: %v", errorValue)
	}
	if errorValue := service.deleteBuzzIdentitySecret("lee@dawn.kim"); errorValue != nil {
		t.Fatalf("delete identity secret: %v", errorValue)
	}
	if service.hasBuzzIdentitySecret("lee@dawn.kim") {
		t.Fatal("expected secret to be gone")
	}
	if errorValue := service.deleteBuzzIdentitySecret("lee@dawn.kim"); errorValue != nil {
		t.Fatalf("deleting a missing secret should be a no-op, got %v", errorValue)
	}
}
