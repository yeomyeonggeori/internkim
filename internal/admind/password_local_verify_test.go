package admind

import (
	"crypto/sha256"
	"encoding/base64"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
)

func TestParseMattermostPasswordHashes(t *testing.T) {
	output := "lee@dawn.kim|$2a$10$abcdef\n|$2a$10$orphan\nnohash@x.com|plain\ngood@x.com|$pbkdf2$f=SHA256,w=600000,l=32$c2FsdA$aGFzaA\n"
	hashes := parseMattermostPasswordHashes(output)
	if hashes["lee@dawn.kim"] != "$2a$10$abcdef" {
		t.Errorf("missing bcrypt row: %v", hashes)
	}
	if _, present := hashes["nohash@x.com"]; present {
		t.Errorf("non-hash password must be skipped: %v", hashes)
	}
	if len(hashes) != 2 {
		t.Errorf("expected 2 valid rows (bcrypt + pbkdf2), got %d: %v", len(hashes), hashes)
	}
}

func TestVerifyPBKDF2PasswordHash(t *testing.T) {
	salt := []byte("sixteen-byte-slt")
	derived := pbkdf2.Key([]byte("hunter2"), salt, 600000, 32, sha256.New)
	encoded := "$pbkdf2$f=SHA256,w=600000,l=32$" +
		base64.RawStdEncoding.EncodeToString(salt) + "$" +
		base64.RawStdEncoding.EncodeToString(derived)
	if !verifyMattermostPasswordHash(encoded, "hunter2") {
		t.Error("correct password must verify against MM pbkdf2 format")
	}
	if verifyMattermostPasswordHash(encoded, "wrong") {
		t.Error("wrong password must not verify")
	}
}

func TestVerifyLocalMattermostPassword(t *testing.T) {
	hash, errorValue := bcrypt.GenerateFromPassword([]byte("hunter2"), bcrypt.DefaultCost)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service := &Service{}
	service.Configuration.StateDirectory = t.TempDir()
	blob := []byte(`{"lee@dawn.kim":"` + string(hash) + `"}`)
	if errorValue := writeFileAtomically(filepath.Join(service.Configuration.StateDirectory, mattermostPasswordHashFileName), blob, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !service.verifyLocalMattermostPassword("Lee@Dawn.kim", "hunter2") {
		t.Error("correct password should verify (case-insensitive email)")
	}
	if service.verifyLocalMattermostPassword("lee@dawn.kim", "wrong") {
		t.Error("wrong password must not verify")
	}
	if service.verifyLocalMattermostPassword("unknown@x.com", "hunter2") {
		t.Error("unknown email must not verify")
	}
}
