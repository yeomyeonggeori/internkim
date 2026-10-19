package box

type SealedModelKey struct {
	Version            int    `json:"version,omitempty"`
	Recipient          string `json:"recipient,omitempty"`
	Enc                string `json:"enc,omitempty"`
	Ciphertext         string `json:"ciphertext"`
	EphemeralPublicKey string `json:"ephemeralPublicKey,omitempty"`
	Nonce              string `json:"nonce,omitempty"`
}
