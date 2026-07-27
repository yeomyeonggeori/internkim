package admind

import (
	"strings"
	"testing"
)

func TestIssuedBuzzIdentityIsSealedIntoTheVault(t *testing.T) {
	service := newIdentityVaultService(t)
	secretKey := strings.Repeat("cd", 32)

	if errorValue := service.storeBuzzIdentitySecret("newcomer@dawn.kim", secretKey); errorValue != nil {
		t.Fatalf("seal issued identity: %v", errorValue)
	}

	sealedSecret, errorValue := service.readBuzzIdentitySecret("newcomer@dawn.kim")
	if errorValue != nil {
		t.Fatalf("read issued identity: %v", errorValue)
	}
	if sealedSecret != secretKey {
		t.Fatalf("expected the issued secret back, got %q", sealedSecret)
	}
}

func TestBuzzIdentityVaultLookupIsCaseInsensitiveOnEmail(t *testing.T) {
	service := newIdentityVaultService(t)
	secretHex := strings.Repeat("9f", 32)
	if errorValue := service.storeBuzzIdentitySecret("Newcomer@Dawn.KIM", secretHex); errorValue != nil {
		t.Fatalf("seal issued identity: %v", errorValue)
	}
	if !service.hasBuzzIdentitySecret("newcomer@dawn.kim") {
		t.Fatal("web session email casing must resolve to the same vault entry")
	}
}
