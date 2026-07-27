package admind

import (
	"testing"
)

func TestBuzzClientVaultRoundTrips(t *testing.T) {
	service := newIdentityVaultService(t)
	blob := []byte(`{"version":1,"kind":"password","ciphertext":"abc"}`)
	if errorValue := service.storeBuzzClientVault("user-0123456789abcdef", blob); errorValue != nil {
		t.Fatalf("store: %v", errorValue)
	}
	stored, found, errorValue := service.readBuzzClientVault("user-0123456789abcdef")
	if errorValue != nil || !found {
		t.Fatalf("read: found=%v err=%v", found, errorValue)
	}
	if string(stored) != string(blob) {
		t.Fatalf("expected %s, got %s", blob, stored)
	}
}

func TestBuzzClientVaultMissingIsNotAnError(t *testing.T) {
	service := newIdentityVaultService(t)
	_, found, errorValue := service.readBuzzClientVault("user-nobody")
	if errorValue != nil {
		t.Fatalf("unexpected error: %v", errorValue)
	}
	if found {
		t.Fatal("expected not found")
	}
}

func TestBuzzClientVaultSubjectSafety(t *testing.T) {
	safe := []string{"user-0123456789abcdef", "lee@example.com", "person_1-2.3"}
	for _, subject := range safe {
		if !isSafeVaultSubject(subject) {
			t.Fatalf("expected %q to be safe", subject)
		}
	}
	unsafe := []string{"", "../etc/passwd", "a/b", "UPPER", "spa ce", "sub/../x"}
	for _, subject := range unsafe {
		if isSafeVaultSubject(subject) {
			t.Fatalf("expected %q to be rejected", subject)
		}
	}
}
