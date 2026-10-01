package box

import (
	"crypto/hpke"
	"encoding/base64"
	"fmt"
)

const (
	modelKeySealInformation    = "internkim model key"
	mailAccountSealInformation = "internkim mail account"

	// Version 1 is RFC 9180 HPKE in base mode with DHKEM(X25519, HKDF-SHA256),
	// HKDF-SHA256 and AES-256-GCM.
	sealedSecretVersion = 1
)

type SealPurpose struct {
	Information    string
	AdditionalData string
}

type SealedSecret struct {
	Version    int    `json:"version"`
	Recipient  string `json:"recipient"`
	Enc        string `json:"enc"`
	Ciphertext string `json:"ciphertext"`
}

func MailPasswordPurpose(companyID, memberID, field string) SealPurpose {
	return SealPurpose{
		Information:    mailAccountSealInformation,
		AdditionalData: companyID + "|" + memberID + "|mail|" + field,
	}
}

func (identity Identity) OpenSecret(sealed SealedSecret, purpose SealPurpose) (string, error) {
	if sealed.Version != sealedSecretVersion {
		return "", fmt.Errorf("the secret is sealed in version %d, and this box opens version %d", sealed.Version, sealedSecretVersion)
	}
	if sealed.Recipient != identity.EncryptionPublicKey() {
		return "", fmt.Errorf("the secret is sealed to the box key %s, and this box holds %s", sealed.Recipient, identity.EncryptionPublicKey())
	}
	opened, errorValue := identity.openSealed(sealed, purpose)
	if errorValue != nil {
		return "", errorValue
	}
	return string(opened), nil
}

func (identity Identity) openSealed(sealed SealedSecret, purpose SealPurpose) ([]byte, error) {
	enc, errorValue := base64.RawURLEncoding.DecodeString(sealed.Enc)
	if errorValue != nil {
		return nil, fmt.Errorf("the sealed encapsulated key: %w", errorValue)
	}
	ciphertext, errorValue := base64.RawURLEncoding.DecodeString(sealed.Ciphertext)
	if errorValue != nil {
		return nil, fmt.Errorf("the sealed ciphertext: %w", errorValue)
	}
	privateKey, errorValue := hpke.NewDHKEMPrivateKey(identity.encryptionKey)
	if errorValue != nil {
		return nil, errorValue
	}
	recipient, errorValue := hpke.NewRecipient(enc, privateKey, hpke.HKDFSHA256(), hpke.AES256GCM(), []byte(purpose.Information))
	if errorValue != nil {
		return nil, fmt.Errorf("the sealed encapsulated key: %w", errorValue)
	}
	opened, errorValue := recipient.Open([]byte(purpose.AdditionalData), ciphertext)
	if errorValue != nil {
		return nil, fmt.Errorf("it was sealed to another box, for another purpose or owner, or altered on the way: %w", errorValue)
	}
	return opened, nil
}
