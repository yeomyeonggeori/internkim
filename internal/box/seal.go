package box

import (
	"crypto/ecdh"
	"crypto/hpke"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

const (
	modelKeySealInformation      = "internkim model key"
	mailAccountSealInformation   = "internkim mail account"
	wifiNetworkSealInformation   = "internkim wifi network"
	adminPasswordSealInformation = "internkim admin password"
	companyKeySealInformation    = "internkim company key"

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

type MailConnection struct {
	Host     string
	Port     int
	Security string
	Username string
}

func MailPasswordPurpose(companyID, memberID, field string, connection MailConnection) SealPurpose {
	return SealPurpose{
		Information: mailAccountSealInformation,
		AdditionalData: additionalDataOf(
			companyID, memberID, "mail", field,
			connection.Host, strconv.Itoa(connection.Port), connection.Security, connection.Username,
		),
	}
}

func ModelKeyPurpose(companyID, boxEncryptionKey string) SealPurpose {
	return SealPurpose{
		Information:    modelKeySealInformation,
		AdditionalData: additionalDataOf(companyID, boxEncryptionKey, "model-key"),
	}
}

func WifiNetworkPurpose(companyID, boxEncryptionKey, requestID string) SealPurpose {
	return SealPurpose{
		Information:    wifiNetworkSealInformation,
		AdditionalData: additionalDataOf(companyID, boxEncryptionKey, "wifi", requestID),
	}
}

func AdminPasswordPurpose(companyID, boxEncryptionKey, settingID string) SealPurpose {
	return SealPurpose{
		Information:    adminPasswordSealInformation,
		AdditionalData: additionalDataOf(companyID, boxEncryptionKey, "admin-password", settingID),
	}
}

func CompanyKeyPurpose(companyID, recipientEncryptionKey string) SealPurpose {
	return SealPurpose{
		Information:    companyKeySealInformation,
		AdditionalData: additionalDataOf(companyID, recipientEncryptionKey, "company-key"),
	}
}

func additionalDataOf(parts ...string) string {
	var joined strings.Builder
	for _, part := range parts {
		joined.WriteString(strconv.Itoa(len(part)) + ":" + part)
	}
	return joined.String()
}

func (identity Identity) OpenSecret(sealed SealedSecret, purpose SealPurpose) (string, error) {
	return openSecretWith(identity.encryptionKey, sealed, purpose)
}

func openSecretWith(privateKey *ecdh.PrivateKey, sealed SealedSecret, purpose SealPurpose) (string, error) {
	if sealed.Version != sealedSecretVersion {
		return "", fmt.Errorf("the secret is sealed in version %d, and this box opens version %d", sealed.Version, sealedSecretVersion)
	}
	holding := encodedKey(privateKey.PublicKey().Bytes())
	if sealed.Recipient != holding {
		return "", fmt.Errorf("the secret is sealed to the key %s, and this box holds %s", sealed.Recipient, holding)
	}
	opened, errorValue := openSealed(privateKey, sealed, purpose)
	if errorValue != nil {
		return "", errorValue
	}
	return string(opened), nil
}

func openSealed(privateKey *ecdh.PrivateKey, sealed SealedSecret, purpose SealPurpose) ([]byte, error) {
	enc, errorValue := base64.RawURLEncoding.DecodeString(sealed.Enc)
	if errorValue != nil {
		return nil, fmt.Errorf("the sealed encapsulated key: %w", errorValue)
	}
	ciphertext, errorValue := base64.RawURLEncoding.DecodeString(sealed.Ciphertext)
	if errorValue != nil {
		return nil, fmt.Errorf("the sealed ciphertext: %w", errorValue)
	}
	recipientKey, errorValue := hpke.NewDHKEMPrivateKey(privateKey)
	if errorValue != nil {
		return nil, errorValue
	}
	recipient, errorValue := hpke.NewRecipient(enc, recipientKey, hpke.HKDFSHA256(), hpke.AES256GCM(), []byte(purpose.Information))
	if errorValue != nil {
		return nil, fmt.Errorf("the sealed encapsulated key: %w", errorValue)
	}
	opened, errorValue := recipient.Open([]byte(purpose.AdditionalData), ciphertext)
	if errorValue != nil {
		return nil, fmt.Errorf("it was sealed to another box, for another purpose or owner, or altered on the way: %w", errorValue)
	}
	return opened, nil
}

func SealSecretTo(recipientEncryptionKey string, plaintext []byte, purpose SealPurpose) (SealedSecret, error) {
	recipientBytes, errorValue := decodedKey(recipientEncryptionKey)
	if errorValue != nil {
		return SealedSecret{}, fmt.Errorf("the recipient key %s: %w", recipientEncryptionKey, errorValue)
	}
	recipientKey, errorValue := ecdh.X25519().NewPublicKey(recipientBytes)
	if errorValue != nil {
		return SealedSecret{}, fmt.Errorf("the recipient key %s: %w", recipientEncryptionKey, errorValue)
	}
	publicKey, errorValue := hpke.NewDHKEMPublicKey(recipientKey)
	if errorValue != nil {
		return SealedSecret{}, errorValue
	}
	enc, sender, errorValue := hpke.NewSender(publicKey, hpke.HKDFSHA256(), hpke.AES256GCM(), []byte(purpose.Information))
	if errorValue != nil {
		return SealedSecret{}, errorValue
	}
	ciphertext, errorValue := sender.Seal([]byte(purpose.AdditionalData), plaintext)
	if errorValue != nil {
		return SealedSecret{}, errorValue
	}
	return SealedSecret{
		Version:    sealedSecretVersion,
		Recipient:  recipientEncryptionKey,
		Enc:        encodedKey(enc),
		Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
	}, nil
}
