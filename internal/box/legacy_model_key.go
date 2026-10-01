package box

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const legacyModelKeySealInformation = "internkim model key v1"

func (identity Identity) openLegacyModelKey(sealed SealedModelKey) (string, error) {
	ephemeralBytes, errorValue := decodedKey(sealed.EphemeralPublicKey)
	if errorValue != nil {
		return "", fmt.Errorf("the sealed model key's ephemeral key: %w", errorValue)
	}
	ephemeralKey, errorValue := ecdh.X25519().NewPublicKey(ephemeralBytes)
	if errorValue != nil {
		return "", fmt.Errorf("the sealed model key's ephemeral key: %w", errorValue)
	}
	sharedSecret, errorValue := identity.encryptionKey.ECDH(ephemeralKey)
	if errorValue != nil {
		return "", fmt.Errorf("agreeing on the model key's sealing key: %w", errorValue)
	}
	sealing, errorValue := legacyModelKeySealing(sharedSecret, ephemeralBytes, identity.encryptionKey.PublicKey().Bytes())
	if errorValue != nil {
		return "", errorValue
	}
	return openedLegacyModelKey(sealing, sealed)
}

func legacyModelKeySealing(sharedSecret, ephemeralPublicKey, boxEncryptionKey []byte) (cipher.AEAD, error) {
	salt := append(append([]byte{}, ephemeralPublicKey...), boxEncryptionKey...)
	key, errorValue := hkdf.Key(sha256.New, sharedSecret, salt, legacyModelKeySealInformation, 32)
	if errorValue != nil {
		return nil, fmt.Errorf("deriving the model key's sealing key: %w", errorValue)
	}
	block, errorValue := aes.NewCipher(key)
	if errorValue != nil {
		return nil, errorValue
	}
	return cipher.NewGCM(block)
}

func openedLegacyModelKey(sealing cipher.AEAD, sealed SealedModelKey) (string, error) {
	nonce, errorValue := base64.RawURLEncoding.DecodeString(sealed.Nonce)
	if errorValue != nil || len(nonce) != sealing.NonceSize() {
		return "", fmt.Errorf("the sealed model key's nonce is not %d bytes", sealing.NonceSize())
	}
	ciphertext, errorValue := base64.RawURLEncoding.DecodeString(sealed.Ciphertext)
	if errorValue != nil {
		return "", fmt.Errorf("the sealed model key's ciphertext: %w", errorValue)
	}
	modelKey, errorValue := sealing.Open(nil, nonce, ciphertext, nil)
	if errorValue != nil {
		return "", fmt.Errorf("the model key was sealed to another box or altered on the way: %w", errorValue)
	}
	return string(modelKey), nil
}
