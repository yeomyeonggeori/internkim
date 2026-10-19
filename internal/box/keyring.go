package box

import (
	"crypto/ecdh"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const companyKeyFileName = "company-key.json"

var ErrCompanyKeyNotHeld = errors.New("this box does not hold the company key yet")

type AwaitingBox struct {
	PublicKey     string `json:"publicKey"`
	EncryptionKey string `json:"encryptionKey"`
}

type SealedCompanyKey struct {
	PublicKey string       `json:"publicKey"`
	Sealed    SealedSecret `json:"sealed"`
}

type Keyring struct {
	Identity   Identity
	companyKey *ecdh.PrivateKey
}

type storedCompanyKey struct {
	CompanyID  string `json:"companyID"`
	PrivateKey string `json:"privateKey"`
}

func (keyring Keyring) HoldsCompanyKey(sealingKey string) bool {
	_, errorValue := keyring.privateKeyFor(sealingKey)
	return errorValue == nil
}

func (keyring Keyring) SealingKey() (string, bool) {
	if keyring.companyKey == nil {
		return "", false
	}
	return encodedKey(keyring.companyKey.PublicKey().Bytes()), true
}

func (keyring Keyring) OpenSecret(sealed SealedSecret, purpose SealPurpose) (string, error) {
	privateKey, errorValue := keyring.privateKeyFor(sealed.Recipient)
	if errorValue != nil {
		return "", errorValue
	}
	return openSecretWith(privateKey, sealed, purpose)
}

func (keyring Keyring) OpenModelKey(sealed SealedModelKey, companyID string) (string, error) {
	if sealed.Version == 0 {
		return keyring.Identity.openLegacyModelKey(sealed)
	}
	modelKey, errorValue := keyring.OpenSecret(SealedSecret{
		Version:    sealed.Version,
		Recipient:  sealed.Recipient,
		Enc:        sealed.Enc,
		Ciphertext: sealed.Ciphertext,
	}, ModelKeyPurpose(companyID, sealed.Recipient))
	if errorValue != nil {
		return "", fmt.Errorf("the model key: %w", errorValue)
	}
	return modelKey, nil
}

func (keyring Keyring) SealCompanyKeyFor(companyID string, awaiting []AwaitingBox) ([]SealedCompanyKey, error) {
	if keyring.companyKey == nil {
		return nil, ErrCompanyKeyNotHeld
	}
	sealed := make([]SealedCompanyKey, 0, len(awaiting))
	for _, box := range awaiting {
		secret, errorValue := SealSecretTo(box.EncryptionKey, keyring.companyKey.Bytes(), CompanyKeyPurpose(companyID, box.EncryptionKey))
		if errorValue != nil {
			return nil, fmt.Errorf("sealing the company key to box %s: %w", box.PublicKey, errorValue)
		}
		sealed = append(sealed, SealedCompanyKey{PublicKey: box.PublicKey, Sealed: secret})
	}
	return sealed, nil
}

func KeyringFor(identity Identity, stateDirectoryPath, companyID, sealingKey string, sealedCompanyKey *SealedSecret) (Keyring, error) {
	keyring := Keyring{Identity: identity}
	if identity.EncryptionPublicKey() == sealingKey {
		keyring.companyKey = identity.encryptionKey
		return keyring, storeCompanyKey(stateDirectoryPath, companyID, identity.encryptionKey)
	}
	held, errorValue := loadCompanyKey(stateDirectoryPath, companyID)
	if errorValue != nil {
		return Keyring{}, errorValue
	}
	if held != nil && encodedKey(held.PublicKey().Bytes()) == sealingKey {
		keyring.companyKey = held
		return keyring, nil
	}
	if sealedCompanyKey == nil {
		return keyring, nil
	}
	opened, errorValue := openCompanyKey(identity, companyID, sealingKey, *sealedCompanyKey)
	if errorValue != nil {
		return Keyring{}, errorValue
	}
	if errorValue := storeCompanyKey(stateDirectoryPath, companyID, opened); errorValue != nil {
		return Keyring{}, errorValue
	}
	keyring.companyKey = opened
	return keyring, nil
}

func LoadKeyring(identity Identity, stateDirectoryPath, companyID string) (Keyring, error) {
	held, errorValue := loadCompanyKey(stateDirectoryPath, companyID)
	if errorValue != nil {
		return Keyring{}, errorValue
	}
	return Keyring{Identity: identity, companyKey: held}, nil
}

func openCompanyKey(identity Identity, companyID, sealingKey string, sealed SealedSecret) (*ecdh.PrivateKey, error) {
	opened, errorValue := identity.OpenSecret(sealed, CompanyKeyPurpose(companyID, identity.EncryptionPublicKey()))
	if errorValue != nil {
		return nil, fmt.Errorf("the company key handed to this box: %w", errorValue)
	}
	privateKey, errorValue := ecdh.X25519().NewPrivateKey([]byte(opened))
	if errorValue != nil {
		return nil, fmt.Errorf("the company key handed to this box: %w", errorValue)
	}
	if encodedKey(privateKey.PublicKey().Bytes()) != sealingKey {
		return nil, fmt.Errorf("the company key handed to this box is not the key the company seals to")
	}
	return privateKey, nil
}

func companyKeyPathIn(stateDirectoryPath string) string {
	return filepath.Join(stateDirectoryPath, companyKeyFileName)
}

func loadCompanyKey(stateDirectoryPath, companyID string) (*ecdh.PrivateKey, error) {
	document, errorValue := os.ReadFile(companyKeyPathIn(stateDirectoryPath))
	if errors.Is(errorValue, os.ErrNotExist) {
		return nil, nil
	}
	if errorValue != nil {
		return nil, errorValue
	}
	var stored storedCompanyKey
	if errorValue := json.Unmarshal(document, &stored); errorValue != nil {
		return nil, fmt.Errorf("the company key at %s is not one this box wrote: %w", companyKeyPathIn(stateDirectoryPath), errorValue)
	}
	if stored.CompanyID != companyID {
		return nil, nil
	}
	seed, errorValue := decodedKey(stored.PrivateKey)
	if errorValue != nil {
		return nil, fmt.Errorf("the company key at %s: %w", companyKeyPathIn(stateDirectoryPath), errorValue)
	}
	return ecdh.X25519().NewPrivateKey(seed)
}

func storeCompanyKey(stateDirectoryPath, companyID string, privateKey *ecdh.PrivateKey) error {
	document, errorValue := json.Marshal(storedCompanyKey{CompanyID: companyID, PrivateKey: encodedKey(privateKey.Bytes())})
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(stateDirectoryPath, 0o700); errorValue != nil {
		return errorValue
	}
	return writeFileAtomically(companyKeyPathIn(stateDirectoryPath), document, 0o600)
}
