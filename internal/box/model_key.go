package box

import "fmt"

type SealedModelKey struct {
	Version            int    `json:"version,omitempty"`
	Recipient          string `json:"recipient,omitempty"`
	Enc                string `json:"enc,omitempty"`
	Ciphertext         string `json:"ciphertext"`
	EphemeralPublicKey string `json:"ephemeralPublicKey,omitempty"`
	Nonce              string `json:"nonce,omitempty"`
}

func (identity Identity) OpenModelKey(sealed SealedModelKey) (string, error) {
	if sealed.Version == 0 {
		return identity.openLegacyModelKey(sealed)
	}
	modelKey, errorValue := identity.OpenSecret(SealedSecret{
		Version:    sealed.Version,
		Recipient:  sealed.Recipient,
		Enc:        sealed.Enc,
		Ciphertext: sealed.Ciphertext,
	}, SealPurpose{Information: modelKeySealInformation})
	if errorValue != nil {
		return "", fmt.Errorf("the model key: %w", errorValue)
	}
	return modelKey, nil
}
