package companion

import "github.com/yeomyeonggeori/internkim/internal/capabilities"

type State struct {
	DeviceURL    string                    `json:"deviceURL"`
	CompanionID  string                    `json:"companionID"`
	Token        string                    `json:"token"`
	PublicKey    string                    `json:"publicKey"`
	PrivateKeyID string                    `json:"privateKeyID,omitempty"`
	PrivateKey   string                    `json:"privateKey,omitempty"`
	LocalOnly    bool                      `json:"localOnly"`
	Capabilities []capabilities.Descriptor `json:"capabilities"`
}
