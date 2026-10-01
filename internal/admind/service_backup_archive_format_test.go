package admind

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

const owaspPBKDF2SHA256Iterations = 600_000

func TestBackupKeyIsStretchedAtTheOWASPIterationCount(t *testing.T) {
	encryptedPath := encryptTestBackup(t, []byte("backup document"), "a chosen passphrase")
	encryptedDocument, errorValue := os.ReadFile(encryptedPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	format, found := backupFormatOf(encryptedDocument)
	if !found {
		t.Fatalf("a new backup starts with %q, which no format reads", encryptedDocument[:7])
	}
	if format.iterations < owaspPBKDF2SHA256Iterations {
		t.Fatalf("a new backup stretches its passphrase %d times, want at least %d", format.iterations, owaspPBKDF2SHA256Iterations)
	}
}

func TestBackupHeaderIsAuthenticated(t *testing.T) {
	encryptedPath := encryptTestBackup(t, []byte("backup document"), "a chosen passphrase")
	encryptedDocument, errorValue := os.ReadFile(encryptedPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	downgraded := append([]byte("IKBAK1\n"), encryptedDocument[len("IKBAK2\n"):]...)
	if errorValue := os.WriteFile(encryptedPath, downgraded, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if decryptFile(encryptedPath, filepath.Join(t.TempDir(), "decrypted"), "a chosen passphrase") == nil {
		t.Fatal("a backup relabelled as the older format still opened")
	}
}

func TestBackupMadeBeforeTheIterationCountRoseStillOpens(t *testing.T) {
	directoryPath := t.TempDir()
	encryptedPath := filepath.Join(directoryPath, "legacy.ikbak")
	decryptedPath := filepath.Join(directoryPath, "decrypted")
	document := []byte("a backup from the first format")
	if errorValue := os.WriteFile(encryptedPath, legacyBackupDocument(t, document, "a chosen passphrase"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := decryptFile(encryptedPath, decryptedPath, "a chosen passphrase"); errorValue != nil {
		t.Fatal(errorValue)
	}
	decryptedDocument, errorValue := os.ReadFile(decryptedPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !bytes.Equal(decryptedDocument, document) {
		t.Fatalf("legacy backup decrypted to %q", decryptedDocument)
	}
}

func encryptTestBackup(t *testing.T, document []byte, passphrase string) string {
	t.Helper()
	directoryPath := t.TempDir()
	plainPath := filepath.Join(directoryPath, "plain")
	encryptedPath := filepath.Join(directoryPath, "backup.ikbak")
	if errorValue := os.WriteFile(plainPath, document, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := encryptFile(plainPath, encryptedPath, passphrase); errorValue != nil {
		t.Fatal(errorValue)
	}
	return encryptedPath
}

func legacyBackupDocument(t *testing.T, document []byte, passphrase string) []byte {
	t.Helper()
	salt := bytes.Repeat([]byte{7}, 16)
	nonce := bytes.Repeat([]byte{9}, 12)
	key, errorValue := pbkdf2.Key(sha256.New, passphrase, salt, 200_000, 32)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	block, errorValue := aes.NewCipher(key)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	aead, errorValue := cipher.NewGCM(block)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	sealed := aead.Seal(nil, nonce, document, []byte("internkim-backup-v1"))
	legacy := append([]byte("IKBAK1\n"), salt...)
	legacy = append(legacy, nonce...)
	return append(legacy, sealed...)
}
