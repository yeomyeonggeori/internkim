package admind

import (
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCalendarTokenEncryptDecryptRoundTrip(t *testing.T) {
	key := makeRandomCalendarTokenKey(t)
	payload := oauthTokenPayload{
		AccessToken:      "access-token-1",
		RefreshToken:     "refresh-token-1",
		TokenType:        "Bearer",
		ExpiresAtRFC3339: "2026-05-15T13:00:00Z",
	}
	blob, errorValue := encryptCalendarTokenPayload(key, payload)
	if errorValue != nil {
		t.Fatalf("encrypt: %v", errorValue)
	}
	decoded, errorValue := decryptCalendarTokenPayload(key, blob)
	if errorValue != nil {
		t.Fatalf("decrypt: %v", errorValue)
	}
	if decoded != payload {
		t.Errorf("payload mismatch: got %+v, want %+v", decoded, payload)
	}
}

func TestCalendarTokenEncryptUsesFreshNonce(t *testing.T) {
	key := makeRandomCalendarTokenKey(t)
	payload := oauthTokenPayload{AccessToken: "a", RefreshToken: "r"}
	first, errorValue := encryptCalendarTokenPayload(key, payload)
	if errorValue != nil {
		t.Fatalf("encrypt 1: %v", errorValue)
	}
	second, errorValue := encryptCalendarTokenPayload(key, payload)
	if errorValue != nil {
		t.Fatalf("encrypt 2: %v", errorValue)
	}
	if string(first) == string(second) {
		t.Fatal("expected distinct ciphertexts due to fresh nonce per call")
	}
}

func TestCalendarTokenFileWriteThenRead(t *testing.T) {
	directory := t.TempDir()
	tokenPath := filepath.Join(directory, "google-acct-1.token.enc")
	key := makeRandomCalendarTokenKey(t)
	payload := oauthTokenPayload{
		AccessToken:      "access",
		RefreshToken:     "refresh",
		TokenType:        "Bearer",
		ExpiresAtRFC3339: "2026-06-01T00:00:00Z",
	}
	if errorValue := writeCalendarTokenFile(tokenPath, key, payload); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	info, errorValue := os.Stat(tokenPath)
	if errorValue != nil {
		t.Fatalf("stat: %v", errorValue)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("permission: got %o, want 0600", mode)
	}
	loaded, errorValue := readCalendarTokenFile(tokenPath, key)
	if errorValue != nil {
		t.Fatalf("read: %v", errorValue)
	}
	if loaded != payload {
		t.Errorf("payload mismatch: got %+v, want %+v", loaded, payload)
	}
}

func TestCalendarTokenFileRejectsWrongKey(t *testing.T) {
	directory := t.TempDir()
	tokenPath := filepath.Join(directory, "google-acct-wrongkey.token.enc")
	rightKey := makeRandomCalendarTokenKey(t)
	wrongKey := makeRandomCalendarTokenKey(t)
	payload := oauthTokenPayload{AccessToken: "a", RefreshToken: "r"}
	if errorValue := writeCalendarTokenFile(tokenPath, rightKey, payload); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	if _, errorValue := readCalendarTokenFile(tokenPath, wrongKey); errorValue == nil {
		t.Fatal("expected decrypt error with wrong key, got nil")
	}
}

func TestLoadOrCreateCalendarTokenEncryptionKeyIsStable(t *testing.T) {
	service := newCalendarTestService(t)
	first, errorValue := service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		t.Fatalf("first load: %v", errorValue)
	}
	if length := len(first); length != secretEncryptionKeyByteSize {
		t.Fatalf("key length: got %d, want %d", length, secretEncryptionKeyByteSize)
	}
	info, errorValue := os.Stat(service.calendarTokenEncryptionKeyPath())
	if errorValue != nil {
		t.Fatalf("stat: %v", errorValue)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("permission: got %o, want 0600", mode)
	}
	second, errorValue := service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		t.Fatalf("second load: %v", errorValue)
	}
	if string(first) != string(second) {
		t.Fatal("expected stable key across calls, got different bytes")
	}
}

func TestCalendarSecretsDirectoryHonoursEnvironmentOverride(t *testing.T) {
	service := newCalendarTestService(t)
	override := t.TempDir()
	t.Setenv(calendarSecretsDirectoryEnvironment, override)
	if got := service.calendarSecretsDirectory(); got != override {
		t.Errorf("override: got %q, want %q", got, override)
	}
}

func TestDeleteCalendarTokenFileIsIdempotent(t *testing.T) {
	directory := t.TempDir()
	tokenPath := filepath.Join(directory, "missing.token.enc")
	if errorValue := deleteCalendarTokenFile(tokenPath); errorValue != nil {
		t.Fatalf("delete missing: %v", errorValue)
	}
	key := makeRandomCalendarTokenKey(t)
	if errorValue := writeCalendarTokenFile(tokenPath, key, oauthTokenPayload{AccessToken: "x"}); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	if errorValue := deleteCalendarTokenFile(tokenPath); errorValue != nil {
		t.Fatalf("delete existing: %v", errorValue)
	}
	if _, errorValue := os.Stat(tokenPath); !os.IsNotExist(errorValue) {
		t.Fatalf("expected file removed, stat returned %v", errorValue)
	}
}

func TestSanitizeCalendarSecretComponentFallsBackOnEmpty(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"", "default"},
		{"   ", "default"},
		{"user@example.com", "user_example_com"},
		{"abc-XYZ_123", "abc-XYZ_123"},
	}
	for _, testCase := range cases {
		if got := sanitizeCalendarSecretComponent(testCase.input); got != testCase.expected {
			t.Errorf("sanitize(%q): got %q, want %q", testCase.input, got, testCase.expected)
		}
	}
}

func makeRandomCalendarTokenKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, secretEncryptionKeyByteSize)
	if _, errorValue := io.ReadFull(rand.Reader, key); errorValue != nil {
		t.Fatalf("rand: %v", errorValue)
	}
	return key
}
