package box

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const seedLength = 32

type Identity struct {
	signingKey    ed25519.PrivateKey
	encryptionKey *ecdh.PrivateKey
}

type storedIdentity struct {
	SigningSeed   string `json:"signingSeed"`
	EncryptionKey string `json:"encryptionKey"`
}

func (identity Identity) PublicKey() string {
	return encodedKey(identity.signingKey.Public().(ed25519.PublicKey))
}

func (identity Identity) EncryptionPublicKey() string {
	return encodedKey(identity.encryptionKey.PublicKey().Bytes())
}

func LoadOrCreateIdentity(path string) (Identity, error) {
	identity, errorValue := loadIdentity(path)
	if !errors.Is(errorValue, os.ErrNotExist) {
		return identity, errorValue
	}
	identity, errorValue = freshIdentity()
	if errorValue != nil {
		return Identity{}, errorValue
	}
	if errorValue := storeIdentity(path, identity); errorValue != nil {
		return Identity{}, errorValue
	}
	return identity, nil
}

func freshIdentity() (Identity, error) {
	signingSeed := make([]byte, seedLength)
	if _, errorValue := rand.Read(signingSeed); errorValue != nil {
		return Identity{}, fmt.Errorf("drawing the box signing key: %w", errorValue)
	}
	encryptionKey, errorValue := ecdh.X25519().GenerateKey(rand.Reader)
	if errorValue != nil {
		return Identity{}, fmt.Errorf("drawing the box encryption key: %w", errorValue)
	}
	return Identity{signingKey: ed25519.NewKeyFromSeed(signingSeed), encryptionKey: encryptionKey}, nil
}

func loadIdentity(path string) (Identity, error) {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return Identity{}, errorValue
	}
	var stored storedIdentity
	if errorValue := json.Unmarshal(document, &stored); errorValue != nil {
		return Identity{}, fmt.Errorf("the box identity at %s is not one this box wrote: %w", path, errorValue)
	}
	return identityOfStored(stored, path)
}

func identityOfStored(stored storedIdentity, path string) (Identity, error) {
	signingSeed, errorValue := decodedKey(stored.SigningSeed)
	if errorValue != nil {
		return Identity{}, fmt.Errorf("the box signing key at %s: %w", path, errorValue)
	}
	encryptionSeed, errorValue := decodedKey(stored.EncryptionKey)
	if errorValue != nil {
		return Identity{}, fmt.Errorf("the box encryption key at %s: %w", path, errorValue)
	}
	encryptionKey, errorValue := ecdh.X25519().NewPrivateKey(encryptionSeed)
	if errorValue != nil {
		return Identity{}, fmt.Errorf("the box encryption key at %s: %w", path, errorValue)
	}
	return Identity{signingKey: ed25519.NewKeyFromSeed(signingSeed), encryptionKey: encryptionKey}, nil
}

func storeIdentity(path string, identity Identity) error {
	document, errorValue := json.Marshal(storedIdentity{
		SigningSeed:   encodedKey(identity.signingKey.Seed()),
		EncryptionKey: encodedKey(identity.encryptionKey.Bytes()),
	})
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return writeFileAtomically(path, document, 0o600)
}

func writeFileAtomically(path string, document []byte, mode os.FileMode) error {
	temporary, errorValue := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if errorValue != nil {
		return errorValue
	}
	defer os.Remove(temporary.Name())
	if errorValue := temporary.Chmod(mode); errorValue != nil {
		temporary.Close()
		return errorValue
	}
	if _, errorValue := temporary.Write(document); errorValue != nil {
		temporary.Close()
		return errorValue
	}
	if errorValue := temporary.Sync(); errorValue != nil {
		temporary.Close()
		return errorValue
	}
	if errorValue := temporary.Close(); errorValue != nil {
		return errorValue
	}
	return os.Rename(temporary.Name(), path)
}

func encodedKey(key []byte) string {
	return base64.RawURLEncoding.EncodeToString(key)
}

func decodedKey(encoded string) ([]byte, error) {
	key, errorValue := base64.RawURLEncoding.DecodeString(encoded)
	if errorValue != nil {
		return nil, errorValue
	}
	if len(key) != seedLength {
		return nil, fmt.Errorf("a key is %d bytes, this one is %d", seedLength, len(key))
	}
	return key, nil
}
