package admind

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const secretEncryptionKeyByteSize = 32

func loadOrCreateSecretEncryptionKey(keyPath string) ([]byte, error) {
	existing, errorValue := os.ReadFile(keyPath)
	if errorValue == nil {
		if len(existing) != secretEncryptionKeyByteSize {
			return nil, fmt.Errorf("encryption key at %s has wrong size %d", keyPath, len(existing))
		}
		return existing, nil
	}
	if !errors.Is(errorValue, os.ErrNotExist) {
		return nil, errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(keyPath), 0o700); errorValue != nil {
		return nil, errorValue
	}
	generated := make([]byte, secretEncryptionKeyByteSize)
	if _, errorValue := io.ReadFull(rand.Reader, generated); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := writeFileAtomically(keyPath, generated, 0o600); errorValue != nil {
		return nil, errorValue
	}
	return generated, nil
}

func sealSecret(key []byte, plaintext []byte, associatedData []byte) ([]byte, error) {
	aead, errorValue := secretAEAD(key)
	if errorValue != nil {
		return nil, errorValue
	}
	nonce := make([]byte, aead.NonceSize())
	if _, errorValue := io.ReadFull(rand.Reader, nonce); errorValue != nil {
		return nil, errorValue
	}
	return append(nonce, aead.Seal(nil, nonce, plaintext, associatedData)...), nil
}

func openSecret(key []byte, blob []byte, associatedData []byte) ([]byte, error) {
	aead, errorValue := secretAEAD(key)
	if errorValue != nil {
		return nil, errorValue
	}
	nonceSize := aead.NonceSize()
	if len(blob) < nonceSize+1 {
		return nil, errors.New("sealed secret is too short")
	}
	return aead.Open(nil, blob[:nonceSize], blob[nonceSize:], associatedData)
}

func secretAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != secretEncryptionKeyByteSize {
		return nil, fmt.Errorf("encryption key must be %d bytes, got %d", secretEncryptionKeyByteSize, len(key))
	}
	block, errorValue := aes.NewCipher(key)
	if errorValue != nil {
		return nil, errorValue
	}
	return cipher.NewGCM(block)
}
