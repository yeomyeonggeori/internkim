package admind

import (
	"strings"
	"testing"
)

func TestIssuedBuzzIdentityIsSealedIntoTheVault(t *testing.T) {
	service := newIdentityVaultService(t)
	identity := buzzGeneratedIdentity{
		publicKey: strings.Repeat("ab", 32),
		secretKey: strings.Repeat("cd", 32),
	}

	if errorValue := service.storeBuzzIdentitySecret("newcomer@example.com", identity.secretKey); errorValue != nil {
		t.Fatalf("seal issued identity: %v", errorValue)
	}

	sealedSecret, errorValue := service.readBuzzIdentitySecret("newcomer@example.com")
	if errorValue != nil {
		t.Fatalf("read issued identity: %v", errorValue)
	}
	if sealedSecret != identity.secretKey {
		t.Fatalf("expected the issued secret back, got %q", sealedSecret)
	}
}

func TestBuzzIdentityVaultLookupIsCaseInsensitiveOnEmail(t *testing.T) {
	service := newIdentityVaultService(t)
	secretHex := strings.Repeat("9f", 32)
	if errorValue := service.storeBuzzIdentitySecret("Newcomer@Dawn.KIM", secretHex); errorValue != nil {
		t.Fatalf("seal issued identity: %v", errorValue)
	}
	if !service.hasBuzzIdentitySecret("newcomer@example.com") {
		t.Fatal("web session email casing must resolve to the same vault entry")
	}
}
